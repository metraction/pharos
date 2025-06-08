package trivy

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/metraction/pharos/internal/utils"
	"github.com/rs/zerolog"
)

// grype vulnerability scanner
type TrivyScanner struct {
	Generator   string
	HomeDir     string
	ScannerBin  string
	ScanTimeout time.Duration
	// version / status
	ScannerVersion  string
	DatabaseVersion string
	DatabaseUpdated time.Time

	//Version     GrypeVersion      // grype binary version + meta
	//DbState     GrypeLocalDbState // grype local database state

	logger *zerolog.Logger
}

// create trivy scanner
func NewTrivyScanner(scanTimeout time.Duration, logger *zerolog.Logger) (*TrivyScanner, error) {

	// find grype path
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
		Generator:  "trivy",
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
		Str("db.version", scanner.DatabaseVersion).
		Any("scan.timeout", scanner.ScanTimeout.String()).
		Msg("NewTrivyScanner() ready")

	return &scanner, nil
}

// check grype local database status, update DbState
func (rx *TrivyScanner) GetVersion() error {

	var stdout, stderr bytes.Buffer

	cmd := exec.Command(rx.ScannerBin, "version", "-f", "json")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf(utils.NoColorCodes(stderr.String()))
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

	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.ScannerBin, "image", "--download-db-only")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	//cmd.Env = append(cmd.Env, "GRYPE_DB_UPDATE_URL=30s")    // mac check time
	//cmd.Env = append(cmd.Env, "UPDATE_DOWNLOAD_TIMEOUT=3m") // max download time

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf(utils.NoColorCodes(stderr.String()))
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
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("UpdateDatabase() ready")

	return nil
}
