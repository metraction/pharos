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
	Generator string

	HomeDir  string
	GrypeBin string
	Timeout  time.Duration

	logger *zerolog.Logger
}

// create new sbom generator using syft
func NewGrypeScanner(timeout time.Duration, logger *zerolog.Logger) (*GrypeScanner, error) {

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

	return &GrypeScanner{
		Generator: "grype",
		HomeDir:   homeDir,
		GrypeBin:  grypeBin,

		Timeout: timeout,
		logger:  logger,
	}, nil
}

// check grype local database status
func (rx *GrypeScanner) CheckUpdate() error {

	rx.logger.Info().
		Str("engine", rx.GrypeBin).
		Msg("CheckUpdate")

	var stdout, stderr bytes.Buffer

	cmd := exec.Command(rx.GrypeBin, "db", "check", "-o", "json")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	state := GrypeDatabaseState{}
	state.FromBytes(stdout.Bytes())

	fmt.Println(stdout.String())

	if err != nil {
		return err
	}
	return nil

}

func (rx *GrypeScanner) RunUpdate() error {

	var stdout, stderr bytes.Buffer

	cmd := exec.Command(rx.GrypeBin, "db", "update")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	//err := cmd.Run()
	return nil
}
