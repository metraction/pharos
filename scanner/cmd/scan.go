/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"errors"
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
	ScanEngine    string // scan engine to use
	Image         string
	Platform      string
	RepoAuth      string // Registry authority dsn
	ScanTimeout   string // sbom & scan execution timeout
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

	scanCmd.Flags().StringVar(&ScanArgs.ScanTimeout, "scan_timeout", EnvOrDefault("scan_timeout", "180s"), "Scan timeout")
	scanCmd.Flags().StringVar(&ScanArgs.CacheExpiry, "cache_expiry", EnvOrDefault("cache_expiry", "90s"), "Redis sbom cache expiry")
	scanCmd.Flags().StringVar(&ScanArgs.CacheEndpoint, "cache_endpoint", EnvOrDefault("cache_endpoint", ""), "Redis cache, e.g. redis://user:pwd@localhost:6379/0")

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
			logger.Fatal().Err(err).Msg("invalid scan_timeout argument")
		}
		cacheExpiry, err := time.ParseDuration(ScanArgs.CacheExpiry)
		if err != nil {
			logger.Fatal().Err(err).Msg("invalid cache_expiry argument")
		}
		ExecuteScan(ScanArgs.ScanEngine, ScanArgs.Image, ScanArgs.Platform, ScanArgs.RepoAuth, scanTimeout, cacheExpiry, ScanArgs.CacheEndpoint, logger)
	},
}

func ExecuteScan(engine, imageRef, platform, repoAuth string, scanTimeout, cacheExpiry time.Duration, cacheEndpoint string, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scanner Scan >-----")
	logger.Info().
		Str("engine", engine).
		Str("image", imageRef).
		Str("platform", platform).
		Str("repo_auth", utils.MaskDsn(repoAuth)).
		Str("scan_timeout", scanTimeout.String()).
		Str("cache_expiry", cacheExpiry.String()).
		Str("cache_endpoint", utils.MaskDsn(cacheEndpoint)).
		Msg("")

	var err error
	var pharosScanResult model.PharosImageScanResult

	var sbomData *[]byte
	var scanData *[]byte

	ctx := context.Background()

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

	logger.Info().
		Str("redis.version", kvc.Version(ctx)).
		Msg("")

	// process repository auth
	auth := repo.RepoAuth{}
	if err = auth.FromDsn(repoAuth); err != nil {
		logger.Error().Err(err).Msg("Load registry authority")
	}

	if engine == "grype" {
		var vulnScanner *grype.GrypeScanner

		// create scanner
		if vulnScanner, err = grype.NewGrypeScanner(scanTimeout, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewGrypeScanner()")
		}
		// update database
		if err = vulnScanner.UpdateDatabase(); err != nil {
			logger.Fatal().Err(err).Msg("UpdateDatabase()")
		}

		// scan image, use cache
		err := ScanAndCacheGrype(imageRef, platform, auth, scanTimeout, cacheExpiry, kvc, logger)
		if err != nil {
			logger.Fatal().Err(err).Msg("ScanAndCacheGrype()")
		}
	}

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

// scan image with grype
func ScanAndCacheGrype(imageRef, platform string, auth repo.RepoAuth, scanTimeout, cacheExpiry time.Duration, kvc *cache.PharosCache, logger *zerolog.Logger) error {

	ctx := context.Background()

	logger.Info().Msg("Scan with Grype")

	// get manifestDigest to have a platform unique key for caching
	indexDigest, manifestDigest, err := repo.GetImageDigests(imageRef, platform, auth)
	if err != nil {
		return err
	}

	logger.Info().
		Str("digest.idx", utils.ShortDigest(indexDigest)).
		Str("digest.man", utils.ShortDigest(manifestDigest)).
		Any("scan_timeout", scanTimeout).
		Any("cache_expiry", cacheExpiry).
		Msg("image digests")

	key := utils.ShortDigest(manifestDigest) + ".sbom"
	cacheState := "cache miss"

	// try cache, else create
	data, err := kvc.GetExpire(ctx, key, scanTimeout)
	if err != nil && !errors.Is(err, cache.ErrKeyNotFound) {
		return err
	}

	if errors.Is(err, cache.ErrKeyNotFound) {
		// cache miss: generate sbom
		data = []byte(imageRef + " " + time.Now().String())
		// cache sbom
		if err := kvc.SetExpire(ctx, key, data, scanTimeout); err != nil {
			return err
		}
	} else {
		cacheState = "cache hit"
	}

	logger.Info().
		Str("key", key).
		Str("cache", cacheState).
		Any("data", data).
		Msg("")

	return nil
}
