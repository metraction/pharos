package grype

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
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

	wgDbUpdate sync.WaitGroup
	logger     *zerolog.Logger
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

	if scanner.ScannerVersion, err = GetScannerVersion(scanner.ScannerBin); err != nil {
		return nil, err
	}
	if updateDb {
		if err := scanner.UpdateDatabase(); err != nil {
			return nil, err
		}
	}
	scanner.logger.Info().
		Str("engine", scanner.ScannerBin).
		Str("dir(prod)", scanner.DbProdDir).
		Str("dir(stage)", scanner.DbStageDir).
		Str("scanner(ver)", scanner.ScannerVersion).
		Any("scan(timeout)", scanner.ScanTimeout.String()).
		Msg("NewGrypeScanner() OK")

	return &scanner, nil
}

// run grype database update (stage update first to keep scanner blocking minimal)
func (rx *GrypeScanner) UpdateDatabase() error {

	var err error
	elapsed := utils.ElapsedFunc()

	updProd := GrypeUpdateRequired(rx.ScannerBin, rx.DbProdDir)   // check if prod update is required
	updStage := GrypeUpdateRequired(rx.ScannerBin, rx.DbStageDir) // check if staging update required

	rx.logger.Info().Any("prod", updProd).Any("staged", updStage).Msg("update vulndb ..")
	if updProd {
		if updStage {
			if err := GetGrypeUpdate(rx.ScannerBin, rx.DbStageDir); err != nil {
				return err
			}
		}
		// make scanner wait while update is in progress
		rx.wgDbUpdate.Add(1)
		defer rx.wgDbUpdate.Done()

		// copy staged to production (fast)
		if err := DeployStagedUpdate(rx.DbStageDir, rx.DbProdDir); err != nil {
			return err
		}
		// verify state
		updStage = GrypeUpdateRequired(rx.ScannerBin, rx.DbStageDir)
		updProd = GrypeUpdateRequired(rx.ScannerBin, rx.DbProdDir)
	}

	if rx.DatabaseVersion, rx.DatabaseUpdated, err = GetDatabaseStatus(rx.ScannerBin); err != nil {
		return err
	}
	rx.logger.Info().
		Any("prod", updProd).
		Any("staged", updStage).
		Str("version", rx.DatabaseVersion).
		Str("built", rx.DatabaseUpdated.Format("2006-01-02 15:04:05")).
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("update vulndb OK")

	return nil
}

// scan cyclondex sbom with grype
func (rx *GrypeScanner) VulnScanSbom(sbom []byte) (grypetype.GrypeScanType, []byte, error) {

	rx.logger.Info().
		Any("scan_timeout", rx.ScanTimeout.String()).
		Msg("VulnScanSbom() ..")

	var stdout, stderr bytes.Buffer

	rx.wgDbUpdate.Wait() // wait in case of running db update

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
