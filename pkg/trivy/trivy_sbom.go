package trivy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/pkg/trivytype"
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
		Generator:    "trivy",
		HomeDir:      homeDir,
		GeneratorBin: trivyBin,
		Timeout:      timeout,
		logger:       logger,
	}

	logger.Debug().
		Any("timeout", generator.Timeout.String()).
		Msg("NewTrivySbomCreator() ready")
	return &generator, nil
}

// download image, create sbom in chosen format, e.g. "cyclonedx"
func (rx *TrivySbomCreator) CreateSbom(task model.PharosScanTask2, format string) (trivytype.TrivySbomType, []byte, error) {

	//auth := task.Auth
	//imageRef := task.ImageSpec.Image
	//platform := task.Platform //lo.CoalesceOrEmpty(task.ImageSpec.Platform, "linux/amd64")

	tlsCheck := utils.DsnParaBoolOr(task.AuthDsn, "tlscheck", true)

	rx.logger.Debug().
		Str("image", task.ImageSpec).
		Str("platform", task.Platform).
		Bool("tlsCheck", tlsCheck).
		Str("format", format).
		Msg("TrivySbomCreator.CreateSbom() ..")

	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), rx.Timeout)
	defer cancel()

	// fail if image is not provided
	if task.ImageSpec == "" {
		return trivytype.TrivySbomType{}, nil, fmt.Errorf("no image provided")
	}

	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.GeneratorBin, "image", "--platform", task.Platform, "--format", format, task.ImageSpec)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// prepare environment: auth
	if utils.DsnUserOr(task.AuthDsn, "") != "" {
		cmd.Env = append(cmd.Env, "TRIVY_USERNAME="+utils.DsnUserOr(task.AuthDsn, ""))
		cmd.Env = append(cmd.Env, "TRIVY_PASSWORD="+utils.DsnPasswordOr(task.AuthDsn, ""))

		rx.logger.Debug().
			Str("authority", utils.DsnHostPortOr(task.AuthDsn, "")).
			Str("user", utils.DsnUserOr(task.AuthDsn, "")).
			Msg("MakeSbom() user auth")

	}
	if !tlsCheck {
		cmd.Env = append(cmd.Env, "TRIVY_INSECURE=true")
	}

	// execute, then check success or timeout
	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return trivytype.TrivySbomType{}, nil, fmt.Errorf("create sbom: timeout after %s", rx.Timeout.String())
	} else if err != nil {
		return trivytype.TrivySbomType{}, nil, fmt.Errorf("%s", utils.NoColorCodes(stderr.String()))
	}

	// get and parse sbom
	data := stdout.Bytes()
	var sbom trivytype.TrivySbomType
	if err := json.Unmarshal(data, &sbom); err != nil {
		return trivytype.TrivySbomType{}, nil, err
	}

	rx.logger.Info().
		Str("image", task.ImageSpec).
		Str("platform", task.Platform).
		Str("format", format).
		Str("distro", "N/A").
		Any("size", humanize.Bytes(uint64(len(data)))).
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("MakeSbom() OK")

	return sbom, data, nil
}
