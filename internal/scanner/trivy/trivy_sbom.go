package trivy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/dustin/go-humanize"
	"github.com/metraction/pharos/internal/utils"
	"github.com/rs/zerolog"
)

// Create cyclonedx from artifact

type TrivySbomCreator struct {
	Generator string

	HomeDir      string
	GeneratorBin string
	Timeout      time.Duration

	logger *zerolog.Logger
}

// create new sbom generator using syft
func NewTrivySbomCreator(timeout time.Duration, logger *zerolog.Logger) (*TrivySbomCreator, error) {

	// find syft path
	trivyBin, err := utils.OsWhich("trivy")
	if err != nil {
		return nil, fmt.Errorf("syft not installed")
	}
	// get homedir
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	generator := TrivySbomCreator{
		Generator:    "syft",
		HomeDir:      homeDir,
		GeneratorBin: trivyBin,
		Timeout:      timeout,
		logger:       logger,
	}

	logger.Info().
		Any("timeout", generator.Timeout.String()).
		Msg("NewTrivySbomCreator() ready")
	return &generator, nil
}

// download image, create sbom in chosen format, e.g. "cyclonedx"
func (rx *TrivySbomCreator) CreateSbom(imageUri, platform, format string) (*cdx.BOM, *[]byte, error) {

	rx.logger.Info().
		Str("image", imageUri).
		Str("platform", platform).
		Str("format", format).
		Msg("CreateSbom() ..")

	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), rx.Timeout)
	defer cancel()

	// fail if not imge is provided
	if imageUri == "" {
		return nil, nil, fmt.Errorf("no image provided")
	}
	// be explicit, set default in app and not here
	if platform == "" {
		return nil, nil, fmt.Errorf("no platform provided")
	}

	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.GeneratorBin, "image", "--platform", platform, "--format", format, imageUri)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// prepare environment
	// cmd.Env = append(cmd.Env, "SYFT_CHECK_FOR_APP_UPDATE=false")

	// execute, then check success or timeout
	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return nil, nil, fmt.Errorf("create sbom: timeout after %s", rx.Timeout.String())
	} else if err != nil {
		return nil, nil, fmt.Errorf(utils.NoColorCodes(stderr.String()))
	}
	data := stdout.Bytes()

	// parse sbom
	var sbom cdx.BOM
	if err := json.Unmarshal(data, &sbom); err != nil {
		return nil, nil, err
	}

	rx.logger.Info().
		Str("image", imageUri).
		Str("platform", platform).
		Str("format", format).
		Any("size", humanize.Bytes(uint64(len(stdout.String())))).
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("CreateSbom() OK")

	return &sbom, &data, nil
}
