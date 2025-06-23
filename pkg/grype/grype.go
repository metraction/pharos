package grype

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/grypetype"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

// grype vulnerability scanner
type GrypeScanner struct {
	Engine     string
	HomeDir    string
	DbProdDir  string // vuln db dir for production scanner
	DbStageDir string // vuln db dir for staging new updates

	ScannerBin  string
	ScanTimeout time.Duration
	// version / status
	ScannerVersion  string
	DatabaseVersion string
	DatabaseUpdated time.Time

	logger *zerolog.Logger
}

// create grype scanner
func NewGrypeScanner(scanTimeout time.Duration, updateDb bool, logger *zerolog.Logger) (*GrypeScanner, error) {

	var err error
	var grypeBin string
	var homeDir string

	// find grype path
	if grypeBin, err = utils.OsWhich("grype"); err != nil {
		return nil, fmt.Errorf("grype not installed")
	}
	// get homedir
	if homeDir, err = os.UserHomeDir(); err != nil {
		return nil, err
	}
	// get vuln db prod directory
	dbProdDir := os.Getenv("GRYPE_DB_CACHE_DIR")
	dbProdDir = lo.Ternary(dbProdDir != "", dbProdDir, filepath.Join(homeDir, ".cache", "grype", "db"))

	// get vuln db staging directory (create if required to ensure all works at startup)
	dbStageDir := filepath.Join(os.TempDir(), "grype-db-stage")
	if err := os.Mkdir(dbStageDir, 0755); err != nil {
		if !os.IsExist(err) {
			return nil, fmt.Errorf("unable to create stage dir %v: %v", dbStageDir, err)
		}
	}

	scanner := GrypeScanner{
		Engine:      "grype",
		HomeDir:     homeDir,
		DbProdDir:   dbProdDir,
		DbStageDir:  dbStageDir,
		ScannerBin:  grypeBin,
		ScanTimeout: scanTimeout,
		logger:      logger,
	}

	if scanner.ScannerVersion, err = GetVersion(scanner.ScannerBin); err != nil {
		return nil, err
	}
	scanner.logger.Info().
		Str("engine", scanner.ScannerBin).
		Str("db_prod_dir", scanner.DbProdDir).
		Str("db_stage_dir", scanner.DbStageDir).
		Str("scan_version", scanner.ScannerVersion).
		Any("scan_timeout", scanner.ScanTimeout.String()).
		Msg("NewGrypeScanner() OK")

	// ensure valid vuln-db is present upon startup
	updProd := GrypeUpdateRequired(scanner.ScannerBin, scanner.DbProdDir)   // check if prod update is required
	updStage := GrypeUpdateRequired(scanner.ScannerBin, scanner.DbStageDir) // check if staging update required

	elapsed := utils.ElapsedFunc()
	if updateDb {
		scanner.logger.Info().Msg("update vulndb ..")
		if updProd {
			if updStage {
				if err := GetGrypeUpdate(scanner.ScannerBin, scanner.DbStageDir); err != nil {
					return nil, err
				}
			}
			// copy staged to production (fast)
			if err := DeployStagedUpdate(scanner.DbStageDir, scanner.DbProdDir); err != nil {
				return nil, err
			}
		}
		updStage = GrypeUpdateRequired(scanner.ScannerBin, scanner.DbStageDir)
		updProd = GrypeUpdateRequired(scanner.ScannerBin, scanner.DbProdDir)

		scanner.logger.Info().
			Any("stage db", updStage).
			Any("prod db", updProd).
			Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
			Msg("update vulndb OK")
	}

	return &scanner, nil
}

// check grype local database status, update DbState
func (rx *GrypeScanner) GetDatabaseState() error {

	var stdout, stderr bytes.Buffer

	cmd := exec.Command(rx.ScannerBin, "db", "status", "-o", "json")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%s", utils.NoColorCodes(stderr.String()))
	}

	result := GrypeLocalDbState{}
	if err := result.FromBytes(stdout.Bytes()); err != nil {
		msg := TranslateMessage(stderr.String())
		return fmt.Errorf("%s", utils.NoColorCodes(msg))
	}
	rx.DatabaseVersion = result.SchemaVersion
	rx.DatabaseUpdated = result.Built
	return nil
}

// run grype database update
// check online if an update is available and download it if required
func (rx *GrypeScanner) UpdateDatabase() error {

	var stdout, stderr bytes.Buffer

	rx.logger.Info().Msg("UpdateDatabase() .. ")

	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.ScannerBin, "db", "update")
	cmd.Env = append(cmd.Env, "GRYPE_DB_CACHE_DIR="+rx.DbProdDir)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	//cmd.Env = append(cmd.Env, "GRYPE_DB_UPDATE_URL=30s")    // mac check time
	//cmd.Env = append(cmd.Env, "UPDATE_DOWNLOAD_TIMEOUT=3m") // max download time

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%s", utils.NoColorCodes(stderr.String()))
	}
	if err := rx.GetDatabaseState(); err != nil {
		return err
	}
	msg := TranslateMessage(stdout.String())

	rx.logger.Info().
		Str("result", utils.NoColorCodes(msg)).
		Str("db.version", rx.DatabaseVersion).
		Any("db.updated", rx.DatabaseUpdated).
		Str("db.age", utils.HumanDeltaMin(time.Since(rx.DatabaseUpdated))).
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("UpdateDatabase() OK")

	return nil
}

// scan cyclondex sbom with grype
func (rx *GrypeScanner) VulnScanSbom(sbom []byte) (grypetype.GrypeScanType, []byte, error) {

	rx.logger.Info().
		Any("scan_timeout", rx.ScanTimeout.String()).
		Msg("VulnScanSbom() ..")

	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), rx.ScanTimeout)
	defer cancel()

	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.ScannerBin, "-o", "json") // cyclonedx-json has no "fixed" state ;-(
	//cmd.Stdin = bytes.NewReader([]byte(sbom))
	cmd.Stdin = bytes.NewReader(sbom)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// check https://github.com/anchore/grype
	cmd.Env = append(cmd.Env, "GRYPE_CHECK_FOR_APP_UPDATE=false")
	cmd.Env = append(cmd.Env, "GRYPE_ADD_CPES_IF_NONE=true")
	cmd.Env = append(cmd.Env, "GRYPE_DB_CACHE_DIR="+rx.DbProdDir)

	//cmd.Env = append(cmd.Env, "GRYPE_DB_REQUIRE_UPDATE_CHECK=true")
	//cmd.Env = append(cmd.Env, "GRYPE_DB_AUTO_UPDATE=false") // don't auto update db
	//cmd.Env = append(cmd.Env, "GRYPE_DB_VALIDATE_AGE=false") // we ensure db is up-to-date
	// GRYPE_ADD_CPES_IF_NONE

	err := cmd.Run()
	data := stdout.Bytes() // results as []byte

	if ctx.Err() == context.DeadlineExceeded {
		return grypetype.GrypeScanType{}, nil, fmt.Errorf("scan sbom: timeout after %s", rx.ScanTimeout.String())
	} else if err != nil {
		return grypetype.GrypeScanType{}, nil, fmt.Errorf("%s", utils.NoColorCodes(stderr.String()))
	}

	// parse into grype scan model
	result := grypetype.GrypeScanType{}
	if err := result.ReadBytes(data); err != nil {
		return grypetype.GrypeScanType{}, nil, err
	}

	rx.logger.Info().
		Str("type", result.Type).
		Any("matches", len(result.Matches)).
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("VulnScanSbom() OK")

	return result, data, nil
}
