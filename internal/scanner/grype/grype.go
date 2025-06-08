package grype

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/metraction/pharos/internal/utils"
	"github.com/rs/zerolog"
)

// grype vulnerability scanner
type GrypeScanner struct {
	Generator   string
	HomeDir     string
	ScannerBin  string
	ScanTimeout time.Duration
	// version / status
	ScannerVersion  string
	DatabaseVersion string
	DatabaseUpdated time.Time

	//grypeVersion GrypeVersion      // grype binary version + meta
	//DbState      GrypeLocalDbState // grype local database state

	logger *zerolog.Logger
}

// create grype scanner
func NewGrypeScanner(scanTimeout time.Duration, logger *zerolog.Logger) (*GrypeScanner, error) {

	// find grype path
	grypeBin, err := utils.OsWhich("grype")
	if err != nil {
		return nil, fmt.Errorf("grype not installed")
	}
	// get homedir
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	scanner := GrypeScanner{
		Generator:  "grype",
		HomeDir:    homeDir,
		ScannerBin: grypeBin,

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
		Msg("NewGrypeScanner() ready")

	return &scanner, nil
}

// check grype local database status, update DbState
func (rx *GrypeScanner) GetVersion() error {

	var stdout, stderr bytes.Buffer

	cmd := exec.Command(rx.ScannerBin, "version", "-o", "json")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := GrypeVersion{}
	err = result.FromBytes(stdout.Bytes())

	if err != nil {
		return fmt.Errorf(utils.NoColorCodes(stderr.String()))
	}
	rx.ScannerVersion = result.GrypeVersion

	if err := rx.GetDatabaseState(); err != nil {
		return err
	}
	return nil
}

// check grype local database status, update DbState
func (rx *GrypeScanner) GetDatabaseState() error {

	var stdout, stderr bytes.Buffer

	cmd := exec.Command(rx.ScannerBin, "db", "status", "-o", "json")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf(utils.NoColorCodes(stderr.String()))
	}

	result := GrypeLocalDbState{}
	if err := result.FromBytes(stdout.Bytes()); err != nil {
		return fmt.Errorf(utils.NoColorCodes(stderr.String()))
	}
	rx.DatabaseVersion = result.SchemaVersion
	rx.DatabaseUpdated = result.Built
	return nil
}

// run grype database update
// check online if an update is available and download it if required
func (rx *GrypeScanner) UpdateDatabase() error {

	var stdout, stderr bytes.Buffer

	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.ScannerBin, "db", "update")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	//cmd.Env = append(cmd.Env, "GRYPE_DB_UPDATE_URL=30s")    // mac check time
	//cmd.Env = append(cmd.Env, "UPDATE_DOWNLOAD_TIMEOUT=3m") // max download time

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf(utils.NoColorCodes(stderr.String()))
	}
	if err := rx.GetDatabaseState(); err != nil {
		return err
	}
	rx.logger.Info().
		Str("result", utils.NoColorCodes(stdout.String())).
		Str("db.version", rx.DatabaseVersion).
		Any("db.updated", rx.DatabaseUpdated).
		Str("db.age", utils.HumanDeltaMin(time.Since(rx.DatabaseUpdated))).
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("UpdateDatabase() ready")

	return nil
}

// scan cyclondex sbom with grype
func (rx *GrypeScanner) VulnScanSbom(sbom *[]byte) (*GrypeScanType, *[]byte, error) {

	rx.logger.Info().
		Any("scan_timeout", rx.ScanTimeout.String()).
		Msg("VulnScanSbom()")

	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), rx.ScanTimeout)
	defer cancel()

	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.ScannerBin, "-o", "json") // cyclonedx-json has no "fixed" state ;-(
	cmd.Stdin = bytes.NewReader([]byte(*sbom))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// check https://github.com/anchore/grype
	cmd.Env = append(cmd.Env, "GRYPE_CHECK_FOR_APP_UPDATE=false")
	cmd.Env = append(cmd.Env, "GRYPE_ADD_CPES_IF_NONE=true")
	//cmd.Env = append(cmd.Env, "GRYPE_DB_REQUIRE_UPDATE_CHECK=true")
	//cmd.Env = append(cmd.Env, "GRYPE_DB_AUTO_UPDATE=false") // don't auto update db
	//cmd.Env = append(cmd.Env, "GRYPE_DB_VALIDATE_AGE=false") // we ensure db is up-to-date

	// GRYPE_ADD_CPES_IF_NONE

	err := cmd.Run()
	data := stdout.Bytes() // results as []byte

	if ctx.Err() == context.DeadlineExceeded {
		return nil, nil, fmt.Errorf("scan sbom: timeout after %s", rx.ScanTimeout.String())
	} else if err != nil {
		return nil, nil, fmt.Errorf(utils.NoColorCodes(stderr.String()))
	}

	// parse into grype scan model
	result := GrypeScanType{}
	if err := result.ReadBytes(data); err != nil {
		return nil, nil, err
	}

	rx.logger.Info().
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Str("type", result.Type).
		Any("matches", len(result.Matches)).
		Any("path", result.Source.TargetPath).
		Msg("VulnScanSbom() success")

	return &result, &data, nil
}
