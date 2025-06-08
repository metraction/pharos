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
	GrypeBin    string
	ScanTimeout time.Duration
	Version     GrypeVersion      // grype binary version + meta
	DbState     GrypeLocalDbState // grype local database state

	logger *zerolog.Logger
}

// create new sbom generator using syft
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
		Generator: "grype",
		HomeDir:   homeDir,
		GrypeBin:  grypeBin,

		ScanTimeout: scanTimeout,
		logger:      logger,
	}
	scanner.GetGrypeVersion()
	scanner.logger.Info().
		Str("engine", scanner.GrypeBin).
		Str("grype.version", scanner.Version.GrypeVersion).
		Str("grype.dbschema", scanner.Version.SupportedDbSchema).
		Str("grype.platform", scanner.Version.Platform).
		Any("grype.built", scanner.Version.BuildDate).
		Any("scan.timeout", scanner.ScanTimeout.String()).
		Msg("NewGrypeScanner() ready")

	return &scanner, nil
}

// check grype local database status, update DbState
func (rx *GrypeScanner) GetGrypeVersion() error {

	var stdout, stderr bytes.Buffer

	cmd := exec.Command(rx.GrypeBin, "version", "-o", "json")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	rx.Version.FromBytes(stdout.Bytes())
	if err != nil {
		return fmt.Errorf(utils.NoColorCodes(stderr.String()))
	}
	return nil
}

// check grype local database status, update DbState
func (rx *GrypeScanner) GetDatabaseState() error {

	var stdout, stderr bytes.Buffer

	cmd := exec.Command(rx.GrypeBin, "db", "status", "-o", "json")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	rx.DbState.FromBytes(stdout.Bytes())
	if err != nil {
		return fmt.Errorf(utils.NoColorCodes(stderr.String()))
	}
	return nil
}

// run grype database update
// check online if an update is available and download it if required
func (rx *GrypeScanner) UpdateDatabase() error {

	var stdout, stderr bytes.Buffer

	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.GrypeBin, "db", "update")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	//cmd.Env = append(cmd.Env, "GRYPE_DB_UPDATE_URL=30s")    // mac check time
	//cmd.Env = append(cmd.Env, "UPDATE_DOWNLOAD_TIMEOUT=3m") // max download time

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf(utils.NoColorCodes(stderr.String()))
	}
	rx.GetDatabaseState()
	rx.logger.Info().
		Str("result", utils.NoColorCodes(stdout.String())).
		Str("db.path", rx.DbState.Path).
		Str("db.schema", rx.DbState.SchemaVersion).
		Any("db.built", rx.DbState.Built).
		Str("db.age", utils.HumanDeltaMin(time.Since(rx.DbState.Built))).
		Any("db.valid", rx.DbState.Valid).
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("UpdateDatabase() ready")

	return nil
}

// scan cyclondex sbom with grype
func (rx *GrypeScanner) VulnScanSbom(sbom *[]byte) (*[]byte, error) {

	rx.logger.Info().
		Any("scan_timeout", rx.ScanTimeout.String()).
		Msg("VulnScanSbom()")

	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), rx.ScanTimeout)
	defer cancel()

	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.GrypeBin, "-o", "json") // cyclonedx-json has no "fixed" state ;-(
	cmd.Stdin = bytes.NewReader([]byte(*sbom))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// check https://github.com/anchore/grype
	//cmd.Env = append(cmd.Env, "GRYPE_DB_VALIDATE_AGE=false") // we ensure db is up-to-date
	// X cmd.Env = append(cmd.Env, "GRYPE_DB_AUTO_UPDATE=false") // don't auto update db
	cmd.Env = append(cmd.Env, "GRYPE_CHECK_FOR_APP_UPDATE=false")
	//cmd.Env = append(cmd.Env, "GRYPE_DB_REQUIRE_UPDATE_CHECK=false")

	// GRYPE_ADD_CPES_IF_NONE

	err := cmd.Run()
	data := stdout.Bytes() // results as []byte

	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("scan sbom: timeout after %s", rx.ScanTimeout.String())
	} else if err != nil {
		return nil, fmt.Errorf(utils.NoColorCodes(stderr.String()))
	}

	rx.logger.Info().
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("VulnScanSbom() success")

	return &data, nil
}
