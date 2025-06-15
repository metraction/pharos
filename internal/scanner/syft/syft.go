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
	"github.com/metraction/pharos/internal/scanner/repo"
	"github.com/metraction/pharos/internal/utils"
	"github.com/rs/zerolog"
)

// Create cyclonedx from artifact

type SyftSbomCreator struct {
	Generator string

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
		Generator: "syft",
		HomeDir:   homeDir,
		SyftBin:   syftBin,
		Timeout:   timeout,
		logger:    logger,
	}

	logger.Info().
		Any("timeout", generator.Timeout.String()).
		Msg("NewSyftSbomCreator() OK")
	return &generator, nil
}

// download image, create sbom in chosen format, e.g. "syft-json", "cyclonedx-json"
func (rx *SyftSbomCreator) CreateSbom(imageRef, platform string, auth repo.RepoAuth, tlsCheck bool, format string) (SyftSbomType, []byte, error) {

	rx.logger.Info().
		Str("image", imageRef).
		Str("platform", platform).
		Bool("tlsCheck", tlsCheck).
		Str("format", format).
		Msg("SyftSbomCreator.CreateSbom() ..")

	var err error
	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), rx.Timeout)
	defer cancel()

	// fail if not imge is provided
	if imageRef == "" {
		return SyftSbomType{}, nil, fmt.Errorf("no image provided")
	}
	// be explicit, set default in app and not here
	if platform == "" {
		return SyftSbomType{}, nil, fmt.Errorf("no platform provided")
	}

	elapsed := utils.ElapsedFunc()
	cmd := exec.Command(rx.SyftBin, "registry:"+imageRef, "--platform", platform, "-o", format)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// prepare environment
	// https://github.com/anchore/syft/wiki/configuration
	cmd.Env = append(cmd.Env, "SYFT_CHECK_FOR_APP_UPDATE=false")
	cmd.Env = append(cmd.Env, "SYFT_PARALLELISM=5")

	// Authentication
	if auth.HasAuth() {
		cmd.Env = append(cmd.Env, "SYFT_REGISTRY_AUTH_AUTHORITY="+auth.Authority)
		if auth.Username != "" {
			rx.logger.Info().
				Str("authority", auth.Authority).
				Str("user", auth.Username).
				Msg("Add user authenication")

			cmd.Env = append(cmd.Env, "SYFT_REGISTRY_AUTH_USERNAME="+auth.Username)
			cmd.Env = append(cmd.Env, "SYFT_REGISTRY_AUTH_PASSWORD="+auth.Password)
		} else if auth.Token != "" {
			rx.logger.Info().
				Str("authority", auth.Authority).
				Str("token", auth.Token).
				Msg("Add token authenication")
			cmd.Env = append(cmd.Env, "SYFT_REGISTRY_AUTH_TOKEN="+auth.Token)
		}
	}
	if !tlsCheck {
		cmd.Env = append(cmd.Env, "SYFT_REGISTRY_INSECURE_SKIP_TLS_VERIFY=true")
	}

	// SYFT_REGISTRY_AUTH_AUTHORITY
	// SYFT_REGISTRY_AUTH_USERNAME
	// SYFT_REGISTRY_AUTH_PASSWORD
	// SYFT_REGISTRY_AUTH_TOKEN

	// SYFT_REGISTRY_AUTH_TLS_CERT
	// SYFT_REGISTRY_AUTH_TLS_KEY

	// TODO: Registry certificates
	// SYFT_REGISTRY_CA_CERT

	// execute, then check success or timeout
	err = cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return SyftSbomType{}, nil, fmt.Errorf("create sbom: timeout after %s", rx.Timeout.String())
	} else if err != nil {
		return SyftSbomType{}, nil, fmt.Errorf(utils.NoColorCodes(stderr.String()))
	}
	data := stdout.Bytes()
	//fmt.Println(stdout.String())

	var sbom SyftSbomType
	if err := json.Unmarshal(data, &sbom); err != nil {
		return SyftSbomType{}, nil, err
	}

	rx.logger.Info().
		Str("image", imageRef).
		Str("platform", platform).
		Str("format", format).
		Str("distro", sbom.Distro.Name).
		Any("size.1", len(data)).
		Any("size.2", humanize.Bytes(uint64(len(stdout.String())))).
		Any("elapsed", utils.HumanDeltaMilisec(elapsed())).
		Msg("CreateSbom() success")

	return sbom, data, nil
}

func safeLen[T any](data *[]T) int {
	if data == nil {
		return 0
	}
	return len(*data)
}
