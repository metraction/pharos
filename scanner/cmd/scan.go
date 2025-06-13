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
	"github.com/metraction/pharos/internal/scanner/grype"
	"github.com/metraction/pharos/internal/scanner/repo"
	"github.com/metraction/pharos/internal/scanner/syft"
	"github.com/metraction/pharos/internal/scanner/trivy"
	"github.com/metraction/pharos/internal/services/cache"

	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of root command
type ScanArgsType = struct {
	ScanEngine  string // scan engine to use
	ScanTimeout string // sbom & scan execution timeout
	RepoAuth    string // Registry authority dsn
	Cache       string // redis://user:pwd@localhost:6379/0
	Image       string
	Platform    string
}

var ScanArgs = ScanArgsType{}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringVar(&ScanArgs.ScanEngine, "engine", EnvOrDefault("engine", ""), "Scan engine to use [grype,trivy]")
	scanCmd.Flags().StringVar(&ScanArgs.ScanTimeout, "scan_timeout", EnvOrDefault("scan_timeout", "180s"), "Scan timeout")

	scanCmd.Flags().StringVar(&ScanArgs.Image, "image", EnvOrDefault("image", ""), "Image to scan, e.g. docker.io/alpine:3.16")
	scanCmd.Flags().StringVar(&ScanArgs.Platform, "platform", EnvOrDefault("platform", "linux/amd64"), "Image platform")
	scanCmd.Flags().StringVar(&ScanArgs.RepoAuth, "repoauth", EnvOrDefault("repoauth", ""), "Registry auth, e.g. registry://user:pwd@docker.io/?type=password")
	scanCmd.Flags().StringVar(&ScanArgs.Cache, "cache", EnvOrDefault("cache", ""), "Redis cache, e.g. redis://user:pwd@localhost:6379/0")

	scanCmd.MarkFlagRequired("engine")
}

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Run scanner and exit",
	Long:  `Scan one asset then exit`,
	Run: func(cmd *cobra.Command, args []string) {

		scanTimeout, err := time.ParseDuration(ScanArgs.ScanTimeout)
		if err != nil {
			logger.Fatal().Err(err).Msg("invalid argument")
		}

		ExecuteScan(ScanArgs.ScanEngine, ScanArgs.Image, ScanArgs.Platform, ScanArgs.RepoAuth, ScanArgs.Cache, scanTimeout, logger)
	},
}

func ExecuteScan(engine, imageRef, platform, regAuth, cacheEndpoint string, scanTimeout time.Duration, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scanner Scan >-----")
	logger.Info().
		Str("engine", engine).
		Str("image", imageRef).
		Str("platform", platform).
		Str("cache", utils.MaskDsn(cacheEndpoint)).
		Any("scan_timeout", scanTimeout.String()).
		Msg("")

	var err error
	var pharosScanResult model.PharosImageScanResult

	var sbomData *[]byte
	var scanData *[]byte

	ctx := context.Background()

	// connect redis cache
	cache, err := cache.NewPharosCache(cacheEndpoint, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Redis cache create")
	}
	defer cache.Close()

	err = cache.Connect(ctx)
	if err != nil {
		logger.Fatal().Err(err).Msg("Redis cache connect")
	}

	logger.Info().
		Str("redis.version", cache.Version(ctx)).
		Msg("")

	// process repository auth
	auth := repo.RepoAuth{}
	if err = auth.FromDsn(regAuth); err != nil {
		logger.Error().Err(err).Msg("Load registry authority")
	}

	err = ScanWithGrype(imageRef, platform, auth, cache, logger)
	os.Exit(1)
	// scan sbom with chosen scanner engine
	if engine == "grype" {
		var grypeResult *grype.GrypeScanType
		var syftSbom *syft.SyftSbomType
		var vulnScanner *grype.GrypeScanner
		var syftSbomGenerator *syft.SyftSbomCreator

		// create sbom generator
		if syftSbomGenerator, err = syft.NewSyftSbomCreator(scanTimeout, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewSyftSbomCreator()")
		}
		// create scanner
		if vulnScanner, err = grype.NewGrypeScanner(scanTimeout, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewGrypeScanner()")
		}
		// get image and create sbom
		if syftSbom, sbomData, err = syftSbomGenerator.CreateSbom(imageRef, platform, "syft-json", auth); err != nil {
			logger.Fatal().Err(err).Msg("CreateSbom()")
		}
		// check/update scanner database
		if err = vulnScanner.UpdateDatabase(); err != nil {
			logger.Fatal().Err(err).Msg("UpdateDatabase()")
		}
		// scan sbom
		if grypeResult, scanData, err = vulnScanner.VulnScanSbom(sbomData); err != nil {
			logger.Fatal().Err(err).Msg("VulnScanSbom()")
		}
		if err = pharosScanResult.LoadGrypeImageScan(syftSbom, grypeResult); err != nil {
			logger.Fatal().Err(err).Msg("scanResult.LoadGrypeScan()")
		}
		//logger.Info().Any("model", pharosScanResult).Msg("")

		os.WriteFile("grype-sbom.json", *sbomData, 0644)
		os.WriteFile("grype-sbom-model.json", syftSbom.ToBytes(), 0644)
		os.WriteFile("grype-scan.json", *scanData, 0644)

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

// Scan with Grype and cache
func ScanWithGrype(imageRef, platform string, auth repo.RepoAuth, cache *cache.PharosCache, logger *zerolog.Logger) error {

	ctx := context.Background()
	sbomTTL := 60 * time.Second // sbom cache expiry

	logger.Info().Msg("Scan with Grype")

	// get digest
	indexDigest, manifestDigest, err := repo.GetImageDigests(imageRef, platform, auth)
	if err != nil {
		return err
	}

	logger.Info().
		Str("digest.idx", utils.ShortDigest(indexDigest)).
		Str("digest.man", utils.ShortDigest(manifestDigest)).
		Any("sbomTTL", sbomTTL).
		Msg("image digests")

	what := "cached"
	key := utils.ShortDigest(manifestDigest) + ".sbom"

	data, err := cache.GetExpire(ctx, key, sbomTTL)
	fmt.Println("11.d:", string(data))
	fmt.Println("11.e:", err)
	if err != nil {
		fmt.Println("22: cache miss")
		// cache miss: generate sbom
		data = []byte(imageRef + " " + time.Now().String())
		// cache sbom
		cache.SetExpire(ctx, key, data, sbomTTL)
		what = "new"
	} else {
		fmt.Println("22: cache HIT")
	}

	logger.Info().
		Str("key", key).
		Str("what", what).
		Any("data", data).
		Msg("")

	return nil
}
