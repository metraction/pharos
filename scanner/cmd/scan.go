/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"

	"github.com/metraction/pharos/internal/scanner/trivy"
	"github.com/metraction/pharos/internal/services/cache"
	"github.com/metraction/pharos/pkg/grype"
	"github.com/metraction/pharos/pkg/model"

	"github.com/metraction/pharos/internal/utils"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of root command
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

// dump scan results to files (for debug)
func WriteResults(prefix string, sbomData []byte, scanData []byte, result model.PharosImageScanResult) {
	os.WriteFile(fmt.Sprintf("%s-sbom.json", prefix), sbomData, 0644)
	os.WriteFile(fmt.Sprintf("%s-scan.json", prefix), scanData, 0644)
	os.WriteFile(fmt.Sprintf("%sgrype-model.json", prefix), result.ToBytes(), 0644)
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
	var pharosScanResult model.PharosImageScanResult

	var sbomData *[]byte
	var scanData *[]byte

	ctx := context.Background()

	// build scantask from arguments
	auth := model.PharosRepoAuth{}
	if err := auth.FromDsn(repoAuth); err != nil {
		logger.Fatal().Err(err).Msg("PharosRepoAuth.FromDsn()")
	}
	auth.TlsCheck = tlsCheck

	task := model.PharosImageScanTask{
		JobId: "",
		Auth:  auth,
		ImageSpec: model.PharosImageSpec{
			Image:       imageRef,
			Platform:    platform,
			CacheExpiry: cacheExpiry,
		},
		Timeout: scanTimeout,
	}

	logger.Info().Any("task", task).Msg("ScanTask")

	// connect redis for key value cache
	kvc, err := cache.NewPharosCache(cacheEndpoint, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Redis cache create")
	}
	defer kvc.Close()

	err = kvc.Connect(ctx)
	if err != nil {
		logger.Fatal().Err(err).Msg("Redis cache connect")
	}

	logger.Info().Str("redis_version", kvc.Version(ctx)).Msg("PharosCache.Connect() OK")

	if engine == "grype" {
		var scanEngine *grype.GrypeScanner

		// create scanner & update database
		if scanEngine, err = grype.NewGrypeScanner(scanTimeout, true, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewGrypeScanner()")
		}
		// scan image, use cache
		result, sbomData, scanData, err := grype.ScanImage(task, scanEngine, kvc, logger)
		if err != nil {
			logger.Fatal().Err(err).Msg("grype.ScanImage()")
		}
		WriteResults("grype", sbomData, scanData, result)
	}

	os.Exit(1)

	// scan sbom with chosen scanner engine
	if engine == "grype" {
	} else if engine == "trivy" {
		var sbom *cdx.BOM
		var trivyResult *trivy.TrivyScanType
		var vulnScanner *trivy.TrivyScanner
		var trivySbomGenerator *trivy.TrivySbomCreator

		// create sbom generator
		if trivySbomGenerator, err = trivy.NewTrivySbomCreator(scanTimeout, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewTrivySbomCreator()")
		}
		// create scanner
		if vulnScanner, err = trivy.NewTrivyScanner(scanTimeout, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewTrivyScanner()")
		}

		// get image and create sbom
		if sbom, sbomData, err = trivySbomGenerator.CreateSbom(imageRef, platform, "cyclonedx"); err != nil {
			logger.Fatal().Err(err).Msg("CreateSbom()")
		}

		// ensure initial update of vuln database
		if err = vulnScanner.UpdateDatabase(); err != nil {
			logger.Fatal().Err(err).Msg("UpdateDatabase()")
		}
		if trivyResult, scanData, err = vulnScanner.VulnScanSbom(sbomData); err != nil {
			logger.Fatal().Err(err).Msg("VulnScanSbom()")
		}

		// map into model
		if err = pharosScanResult.LoadTrivyImageScan(sbom, trivyResult); err != nil {
			logger.Fatal().Err(err).Msg("scanResult.LoadGrypeScan()")
		}
		//logger.Info().Any("model", pharosScanResult).Msg("")

		os.WriteFile("trivy-sbom.json", *sbomData, 0644)
		os.WriteFile("trivy-scan.json", *scanData, 0644)
	} else {

		logger.Fatal().Str("engine", engine).Msg("unknown engine")
	}
	logger.Info().
		Str("engine", engine).
		Str("image", imageRef).
		Str("platform", platform).
		Any("x.findings", len(pharosScanResult.Findings)).
		Any("x.vulns", len(pharosScanResult.Vulnerabilities)).
		Any("x.packages", len(pharosScanResult.Packages)).
		Msg("success")

	os.WriteFile(engine+"-model.json", pharosScanResult.ToBytes(), 0644)

	logger.Info().Msg("done")

}
