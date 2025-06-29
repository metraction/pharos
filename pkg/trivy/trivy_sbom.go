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
func (rx *TrivySbomCreator) CreateSbom(task model.PharosScanTask, format string) (trivytype.TrivySbomType, []byte, error) {

	auth := task.Auth
	imageRef := task.ImageSpec.Image
	platform := task.ImageSpec.Platform //lo.CoalesceOrEmpty(task.ImageSpec.Platform, "linux/amd64")

	rx.logger.Debug().
		Str("image", imageRef).
		Str("platform", platform).
		Bool("tlsCheck", auth.TlsCheck).
		Str("format", format).
		Msg("TrivySbomCreator.CreateSbom() ..")

	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), rx.Timeout)
	defer cancel()

	// fail if image is not provided
	if imageRef == "" {
		return trivytype.TrivySbomType{}, nil, fmt.Errorf("no image provided")
	}
	// be explicit, set default in app and not here
	// if platform == "" {
	// 	return trivytype.TrivySbomType{}, nil, fmt.Errorf("no platform provided")
	// }

	// auth
	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.GeneratorBin, "image", "--platform", platform, "--format", format, imageRef)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// prepare environment
	// Authentication
	if auth.HasAuth(imageRef) {
		if auth.Username != "" {
			rx.logger.Debug().
				Str("authority", auth.Authority).
				Str("user", auth.Username).
				Msg("Add user authenication")

			cmd.Env = append(cmd.Env, "TRIVY_USERNAME="+auth.Username)
			cmd.Env = append(cmd.Env, "TRIVY_PASSWORD="+auth.Password)
		} else if auth.Token != "" {
			return trivytype.TrivySbomType{}, nil, fmt.Errorf("token authentication not supported")
		}
	}
	if !auth.TlsCheck {
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
		Str("image", imageRef).
		Str("platform", platform).
		Str("format", format).
		Str("distro", "N/A").
		Any("size", humanize.Bytes(uint64(len(data)))).
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("MakeSbom() OK")

	return sbom, data, nil
}
