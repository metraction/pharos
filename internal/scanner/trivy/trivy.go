package trivy

import (
	"fmt"
	"os"
	"time"

	"github.com/metraction/pharos/internal/utils"
	"github.com/rs/zerolog"
)

// grype vulnerability scanner
type TrivyScanner struct {
	Generator   string
	HomeDir     string
	GrypeBin    string
	ScanTimeout time.Duration
	//Version     GrypeVersion      // grype binary version + meta
	//DbState     GrypeLocalDbState // grype local database state

	logger *zerolog.Logger
}

// create new sbom generator using syft
func NewTrivyScanner(scanTimeout time.Duration, logger *zerolog.Logger) (*TrivyScanner, error) {

	// find grype path
	grypeBin, err := utils.OsWhich("trivy")
	if err != nil {
		return nil, fmt.Errorf("trivy not installed")
	}
	// get homedir
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	scanner := TrivyScanner{
		Generator: "trivy",
		HomeDir:   homeDir,
		GrypeBin:  grypeBin,

		ScanTimeout: scanTimeout,
		logger:      logger,
	}
	//scanner.GetGrypeVersion()
	scanner.logger.Info().
		Str("engine", scanner.GrypeBin).
		//Str("grype.version", scanner.Version.GrypeVersion).
		//Str("grype.dbschema", scanner.Version.SupportedDbSchema).
		//Str("grype.platform", scanner.Version.Platform).
		//Any("grype.built", scanner.Version.BuildDate).
		Any("scan.timeout", scanner.ScanTimeout.String()).
		Msg("NewTrivyScanner() ready")

	return &scanner, nil
}
