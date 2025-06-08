package grype

import (
	"bytes"
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

	logger.Info().Msg("NewGrypeScanner()")

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

	rx.logger.Info().Msg("UpdateDatabase()")

	var stdout, stderr bytes.Buffer

	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.GrypeBin, "db", "update")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

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
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).Msg("UpdateDatabase() ready")

	return nil
}
