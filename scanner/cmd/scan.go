/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/metraction/pharos/internal/integrations/cache"
	"github.com/metraction/pharos/pkg/grype"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/pkg/trivy"

	"github.com/metraction/pharos/internal/utils"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of root command
// implemented as type to facilitate testing of command main routine
type ScanArgsType = struct {
	ScanEngine  string // scan engine to use
	Image       string
	Platform    string
	RepoAuth    string // Registry authority dsn
	ScanTimeout string // sbom & scan execution timeout
	TlsCheck    string // Skop TLS cert check when pulling images
	//
	CacheExpiry   string // how log to cache sboms in redis
	CacheEndpoint string // redis://user:pwd@localhost:6379/0

}

var ScanArgs = ScanArgsType{}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringVar(&ScanArgs.ScanEngine, "engine", EnvOrDefault("engine", ""), "Scan engine to use [grype,trivy]")
	scanCmd.Flags().StringVar(&ScanArgs.Image, "image", EnvOrDefault("image", ""), "Image to scan, e.g. docker.io/alpine:3.16")
	scanCmd.Flags().StringVar(&ScanArgs.Platform, "platform", EnvOrDefault("platform", "linux/amd64"), "Image platform")
	scanCmd.Flags().StringVar(&ScanArgs.RepoAuth, "repo_auth", EnvOrDefault("repo_auth", ""), "Registry auth, e.g. registry://user:pwd@docker.io/?type=password")
	scanCmd.Flags().StringVar(&ScanArgs.TlsCheck, "tlscheck", EnvOrDefault("tlscheck", "on"), "Check TLS cert (on), skip check (off)")

	scanCmd.Flags().StringVar(&ScanArgs.ScanTimeout, "scan_timeout", EnvOrDefault("scan_timeout", "180s"), "Scan timeout")
	scanCmd.Flags().StringVar(&ScanArgs.CacheExpiry, "cache_expiry", EnvOrDefault("cache_expiry", "90s"), "Redis sbom cache expiry")
	scanCmd.Flags().StringVar(&ScanArgs.CacheEndpoint, "cache_endpoint", EnvOrDefault("cache_endpoint", ""), "Redis cache, e.g. redis://user:pwd@localhost:6379/0")

	scanCmd.MarkFlagRequired("engine")
	scanCmd.MarkFlagRequired("image")
}

// dump scan results to files (for debugging)
func WriteResults(prefix string, sbomData []byte, scanData []byte, result model.PharosScanResult) {
	os.WriteFile(fmt.Sprintf("%s-sbom.json", prefix), sbomData, 0644)
	os.WriteFile(fmt.Sprintf("%s-scan.json", prefix), scanData, 0644)
	os.WriteFile(fmt.Sprintf("%s-model.json", prefix), result.ToBytes(), 0644)
	return
}

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Run scanner and exit",
	Long:  `Scan one asset then exit`,
	Run: func(cmd *cobra.Command, args []string) {

		tlsCheck := utils.ToBool(ScanArgs.TlsCheck)
		scanTimeout := utils.DurationOr(ScanArgs.ScanTimeout, 90*time.Second)
		cacheExpiry := utils.DurationOr(ScanArgs.CacheExpiry, 90*time.Second)

		ExecuteScan(ScanArgs.ScanEngine, ScanArgs.Image, ScanArgs.Platform, ScanArgs.RepoAuth, tlsCheck, scanTimeout, cacheExpiry, ScanArgs.CacheEndpoint, logger)
	},
}

func ExecuteScan(engine, imageRef, platform, repoAuth string, tlsCheck bool, scanTimeout, cacheExpiry time.Duration, cacheEndpoint string, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scanner Scan >-----")
	logger.Info().
		Str("engine", engine).
		Str("image", imageRef).
		Str("platform", platform).
		Str("repo_auth", utils.MaskDsn(repoAuth)).
		Bool("tlscheck", tlsCheck).
		Any("scan_timeout", scanTimeout.String()).
		Any("cache_expiry", cacheExpiry.String()).
		Str("cache_endpoint", utils.MaskDsn(cacheEndpoint)).
		Msg("")

	var err error
	var sbomData []byte // sbom raw result
	var scanData []byte // scan raw result
	var task model.PharosScanTask
	var auth model.PharosRepoAuth
	var result model.PharosScanResult // Pharos scan result type

	ctx := context.Background()

	// build scantask from arguments
	if auth, err = model.NewPharosRepoAuth(repoAuth, tlsCheck); err != nil {
		logger.Fatal().Err(err).Msg("invalid repo auth definition")
	}
	if task, err = model.NewPharosScanTask("", imageRef, platform, auth, cacheExpiry, scanTimeout); err != nil {
		logger.Fatal().Err(err).Msg("invalid scan task definition")
	}

	// connect redis for key value cache
	kvc, err := cache.NewPharosCache(cacheEndpoint, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Redis cache create")
	}
	defer kvc.Close()

	if err = kvc.Connect(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Redis cache connect")
	}

	logger.Info().Str("redis_version", kvc.Version(ctx)).Msg("PharosCache.Connect() OK")

	// execute scan with respective scanner engine

	if engine == "grype" {
		var scanEngine *grype.GrypeScanner

		// create scanner & update database
		if scanEngine, err = grype.NewGrypeScanner(scanTimeout, true, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewGrypeScanner()")
		}
		// scan image, use cache
		result, sbomData, scanData, err = grype.ScanImage(task, scanEngine, kvc, logger)
		if err != nil {
			logger.Fatal().Err(err).Msg("grype.ScanImage()")
		}
		WriteResults("grype", sbomData, scanData, result)

	} else if engine == "trivy" {
		var scanEngine *trivy.TrivyScanner

		// create scanner & update database
		if scanEngine, err = trivy.NewTrivyScanner(scanTimeout, true, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewTrivyScanner()")
		}
		// scan image, use cache
		result, sbomData, scanData, err = trivy.ScanImage(task, scanEngine, kvc, logger)
		if err != nil {
			logger.Fatal().Err(err).Msg("trivy.ScanImage()")
		}
		WriteResults("trivy", sbomData, scanData, result)

	} else {
		logger.Fatal().Str("engine", engine).Msg("unknown engine")
	}

	logger.Info().
		Str("engine", engine).
		Str("image", imageRef).
		Str("platform", platform).
		Any("img.distro", result.Image.DistroName+" "+result.Image.DistroVersion).
		Any("img.size", humanize.Bytes(result.Image.Size)).
		Any("scan.findings", len(result.Findings)).
		Any("scan.vulns", len(result.Vulnerabilities)).
		Any("scan.packages", len(result.Packages)).
		Msg("scan completed")

	logger.Info().
		Msg("done")

}
