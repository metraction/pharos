package syft

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
	"github.com/metraction/pharos/pkg/syfttype"
	"github.com/rs/zerolog"
)

// Create cyclonedx from artifact

type SyftSbomCreator struct {
	Engine string

	HomeDir string
	SyftBin string
	Timeout time.Duration

	logger *zerolog.Logger
}

// create new sbom generator using syft
func NewSyftSbomCreator(timeout time.Duration, logger *zerolog.Logger) (*SyftSbomCreator, error) {

	// find syft path
	syftBin, err := utils.OsWhich("syft")
	if err != nil {
		return nil, fmt.Errorf("syft not installed")
	}
	// get homedir
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	generator := SyftSbomCreator{
		Engine:  "syft",
		HomeDir: homeDir,
		SyftBin: syftBin,
		Timeout: timeout,
		logger:  logger,
	}

	logger.Debug().
		Any("timeout", generator.Timeout.String()).
		Msg("NewSyftSbomCreator() OK")
	return &generator, nil
}

// download image, create sbom in chosen format, e.g. "syft-json", "cyclonedx-json"
// func (rx *SyftSbomCreator) CreateSbom(imageRef, platform string, auth model.PharosRepoAuth, tlsCheck bool, format string) (syfttype.SyftSbomType, []byte, error) {
func (rx *SyftSbomCreator) CreateSbom(task model.PharosScanTask2, format string) (syfttype.SyftSbomType, []byte, error) {

	// auth := task.Auth
	// imageRef := task.ImageSpec.Image
	//platform := task.Platform

	tlsCheck := utils.DsnParaBoolOr(task.AuthDsn, "tlscheck", true)

	rx.logger.Debug().
		Str("image", task.ImageSpec).
		Str("platform", task.Platform).
		Bool("tlsCheck", tlsCheck).
		Str("format", format).
		Msg("MakeSbom() ..")

	var err error
	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), rx.Timeout)
	defer cancel()

	// fail if image is not provided
	if task.ImageSpec == "" {
		return syfttype.SyftSbomType{}, nil, fmt.Errorf("no image provided")
	}
	// note: empty platform is OK
	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.SyftBin, "scan", "registry:"+task.ImageSpec, "--platform", task.Platform, "-o", format)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// prepare environment
	// https://github.com/anchore/syft/wiki/configuration
	cmd.Env = append(cmd.Env, "SYFT_CHECK_FOR_APP_UPDATE=false")
	cmd.Env = append(cmd.Env, "SYFT_PARALLELISM=5")
	cmd.Env = append(cmd.Env, "HOME=/tmp") // see: https://github.com/anchore/syft/issues/2984

	// Authentication
	if utils.DsnUserOr(task.AuthDsn, "") != "" {
		cmd.Env = append(cmd.Env, "SYFT_REGISTRY_AUTH_AUTHORITY="+utils.DsnHostPortOr(task.AuthDsn, ""))
		cmd.Env = append(cmd.Env, "SYFT_REGISTRY_AUTH_USERNAME="+utils.DsnUserOr(task.AuthDsn, ""))
		cmd.Env = append(cmd.Env, "SYFT_REGISTRY_AUTH_PASSWORD="+utils.DsnPasswordOr(task.AuthDsn, ""))
		rx.logger.Debug().
			Str("authority", utils.DsnHostPortOr(task.AuthDsn, "")).
			Str("user", utils.DsnUserOr(task.AuthDsn, "")).
			Msg("MakeSbom() user auth")

	}
	if !tlsCheck {
		cmd.Env = append(cmd.Env, "SYFT_REGISTRY_INSECURE_SKIP_TLS_VERIFY=true")
	}
	// SYFT_REGISTRY_AUTH_TLS_CERT
	// SYFT_REGISTRY_AUTH_TLS_KEY
	// SYFT_REGISTRY_CA_CERT

	// execute, then check success or timeout
	err = cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return syfttype.SyftSbomType{}, nil, fmt.Errorf("create sbom: timeout after %s", rx.Timeout.String())
	} else if err != nil {
		return syfttype.SyftSbomType{}, nil, fmt.Errorf("%s", utils.NoColorCodes(stderr.String()))
	}

	// get and parse sbom
	data := stdout.Bytes()
	var sbom syfttype.SyftSbomType
	if err := json.Unmarshal(data, &sbom); err != nil {
		return syfttype.SyftSbomType{}, nil, err
	}

	rx.logger.Info().
		Str("image", task.ImageSpec).
		Str("platform", task.Platform).
		Str("format", format).
		Str("distro", sbom.Distro.Name).
		Any("size", humanize.Bytes(uint64(len(data)))).
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("MakeSbom() OK")

	return sbom, data, nil
}

func safeLen[T any](data *[]T) int {
	if data == nil {
		return 0
	}
	return len(*data)
}
