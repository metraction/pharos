package trivy

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/trivytype"
	"github.com/rs/zerolog"
)

// trivy vulnerability scanner
type TrivyScanner struct {
	Engine      string
	HomeDir     string
	ScannerBin  string
	ScanTimeout time.Duration

	// version / status
	ScannerVersion  string
	DatabaseVersion string
	DatabaseUpdated time.Time

	logger *zerolog.Logger
}

func (rx *TrivyScanner) ScannerName() string {
	return "trivy"
}

// create trivy scanner
func NewTrivyScanner(scanTimeout time.Duration, updateDb bool, logger *zerolog.Logger) (*TrivyScanner, error) {

	// find trivy path
	trivyBin, err := utils.OsWhich("trivy")
	if err != nil {
		return nil, fmt.Errorf("trivy not installed")
	}
	// get homedir
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	scanner := TrivyScanner{
		Engine:     "trivy",
		HomeDir:    homeDir,
		ScannerBin: trivyBin,

		ScanTimeout: scanTimeout,
		logger:      logger,
	}
	if err := scanner.GetVersion(); err != nil {
		return nil, err
	}

	scanner.logger.Info().
		Str("engine", scanner.ScannerBin).
		Str("scan.version", scanner.ScannerVersion).
		Any("scan.timeout", scanner.ScanTimeout.String()).
		Msg("NewTrivyScanner() OK")

	// update database
	if updateDb {
		if err = scanner.UpdateDatabase(); err != nil {
			logger.Fatal().Err(err).Msg("UpdateDatabase()")
		}
	}
	return &scanner, nil
}

// check trivy local database status, update DbState
func (rx *TrivyScanner) GetVersion() error {

	var stdout, stderr bytes.Buffer

	cmd := exec.Command(rx.ScannerBin, "version", "-f", "json")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%s", utils.NoColorCodes(stderr.String()))
	}
	result := TrivyVersion{}
	err = result.FromBytes(stdout.Bytes())
	if err != nil {
		return err
	}
	rx.ScannerVersion = result.Version
	rx.DatabaseVersion = strconv.Itoa(result.VulnerabilityDb.Version)
	rx.DatabaseUpdated = result.VulnerabilityDb.UpdatedAt
	return nil
}

// run trivy database update
// check online if an update is available and download it if required
func (rx *TrivyScanner) UpdateDatabase() error {

	var stdout, stderr bytes.Buffer

	rx.logger.Info().Msg("UpdateDatabase() .. ")

	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.ScannerBin, "image", "--download-db-only")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%s", utils.NoColorCodes(stderr.String()))
	}
	result := utils.NoColorCodes(stdout.String())
	if result == "" {
		result = "update OK"
	}
	rx.GetVersion()
	rx.logger.Info().
		Str("result", result).
		Str("db.version", rx.DatabaseVersion).
		Any("db.updated", rx.DatabaseUpdated).
		Str("db.age", utils.HumanDeltaMin(time.Since(rx.DatabaseUpdated))).
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("UpdateDatabase() OK")

	return nil
}

// scan cyclondex sbom with trivy
func (rx *TrivyScanner) VulnScanSbom(sbom []byte) (trivytype.TrivyScanType, []byte, error) {

	rx.logger.Info().
		Any("scan_timeout", rx.ScanTimeout.String()).
		Msg("VulnScanSbom() ..")

	var err error
	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), rx.ScanTimeout)
	defer cancel()

	elapsed := utils.ElapsedFunc()

	// trivy cannot read sbom from stdin, so we create a file
	tmpfile := "trivy-sbom-temp.json"
	fh, err := os.CreateTemp("", tmpfile)
	if err != nil {
		return trivytype.TrivyScanType{}, nil, err
	}
	defer fh.Close()
	defer os.Remove(fh.Name())

	if err = os.WriteFile(fh.Name(), sbom, 0644); err != nil {
		return trivytype.TrivyScanType{}, nil, err
	}

	cmd := exec.Command(rx.ScannerBin, "sbom", fh.Name(), "--scanners", "vuln", "-f", "json")
	cmd.Stdin = bytes.NewReader([]byte(sbom))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	//cmd.Env = append(cmd.Env, "GRYPE_DB_REQUIRE_UPDATE_CHECK=true")

	err = cmd.Run()
	data := stdout.Bytes() // results as []byte

	if ctx.Err() == context.DeadlineExceeded {
		return trivytype.TrivyScanType{}, nil, fmt.Errorf("scan sbom: timeout after %s", rx.ScanTimeout.String())
	} else if err != nil {
		return trivytype.TrivyScanType{}, nil, fmt.Errorf("%s", utils.NoColorCodes(stderr.String()))
	}

	// parse into grype scan model
	scan := trivytype.TrivyScanType{}
	if err := scan.ReadBytes(data); err != nil {
		return trivytype.TrivyScanType{}, nil, err
	}

	rx.logger.Info().
		Str("type", scan.ArtifactType).
		Any("matches", len(scan.ListVulnerabilities())).
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("VulnScanSbom() OK")

	return scan, data, nil
}
