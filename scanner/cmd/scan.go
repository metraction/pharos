/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/dustin/go-humanize"

	"github.com/metraction/pharos/internal/scanner/trivy"
	"github.com/metraction/pharos/internal/services/cache"
	"github.com/metraction/pharos/pkg/grype"
	"github.com/metraction/pharos/pkg/grypetype"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/pkg/scanning"
	"github.com/metraction/pharos/pkg/syft"
	"github.com/metraction/pharos/pkg/syfttype"

	"github.com/samber/lo"

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
		Any("scan_timeout", scanTimeout).
		Any("cache_expiry", cacheExpiry).
		Str("cache_endpoint", utils.MaskDsn(cacheEndpoint)).
		Msg("")

	var err error
	var pharosScanResult model.PharosImageScanResult

	var sbomData *[]byte
	var scanData *[]byte

	ctx := context.Background()

	// build scantask
	auth := model.PharosRepoAuth{}
	if err := auth.FromDsn(repoAuth); err != nil {
		logger.Fatal().Err(err).Msg("PharosRepoAuth.FromDsn()")
	}

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
	logger.Info().
		Str("redis_version", kvc.Version(ctx)).
		Msg("PharosCache.Connect() OK")

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

		// if imageRef is a file, scan all images in file
		file, err := os.Open(imageRef)
		if err != nil {
			// scan one image
			result, sbomData, scanData, err := ScanAndCacheGrype(imageRef, platform, auth, tlsCheck, scanTimeout, cacheExpiry, vulnScanner, kvc, logger)
			if err != nil {
				logger.Fatal().Err(err).Msg("ScanAndCacheGrype()")
			}
			os.WriteFile("grype-sbom.json", sbomData, 0644)
			os.WriteFile("grype-scan.json", scanData, 0644)
			os.WriteFile("grype-model.json", result.ToBytes(), 0644)
			os.Exit(0)
		}

		// scan all images in file
		defer file.Close()
		k := 0
		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			k += 1
			fileBase := filepath.Join("_output", fmt.Sprintf("%v", k))
			image := scanner.Text()
			logger.Info().Any("#", k).Str("image", image).Str("base", fileBase).Msg("")
			result, sbomData, scanData, err := ScanAndCacheGrype(image, platform, auth, tlsCheck, scanTimeout, cacheExpiry, vulnScanner, kvc, logger)
			if err != nil {
				logger.Error().Err(err).Msg("ScanAndCacheGrype")
				continue
			}

			os.WriteFile(fileBase+"-grype-sbom.json", sbomData, 0644)
			os.WriteFile(fileBase+"-grype-scan.json", scanData, 0644)
			os.WriteFile(fileBase+"-grype-model.json", result.ToBytes(), 0644)

		}
		if err := scanner.Err(); err != nil {
			logger.Fatal().Err(err).Msg("File Scanner")
		}

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

// scan image with grype
func ScanAndCacheGrype(imageRef, platform string, auth model.PharosRepoAuth, tlsCheck bool, scanTimeout, cacheExpiry time.Duration, scanEngine *grype.GrypeScanner, kvc *cache.PharosCache, logger *zerolog.Logger) (model.PharosImageScanResult, []byte, []byte, error) {

	// return sbom cache key for given digest
	CacheKey := func(digest string) string {
		return lo.Substring(digest, 0, 39) + ".grype.sbom"
	}
	noResult := model.PharosImageScanResult{}

	ctx := context.Background()

	logger.Info().Msg("Scan with Grype")

	// get manifestDigest to have a platform unique key for caching
	indexDigest, manifestDigest, err := scanning.GetImageDigests(imageRef, platform, auth, tlsCheck)
	if err != nil {
		return noResult, nil, nil, fmt.Errorf("get image digest: %v", err)
	}

	logger.Info().
		Str("digest.idx", utils.ShortDigest(indexDigest)).
		Str("digest.man", utils.ShortDigest(manifestDigest)).
		Msg("image digests")

	var sbomEngine *syft.SyftSbomCreator

	// create sbom generator
	if sbomEngine, err = syft.NewSyftSbomCreator(scanTimeout, logger); err != nil {
		logger.Fatal().Err(err).Msg("NewSyftSbomCreator()")
	}

	var sbomData []byte                // raw data
	var sbomProd syfttype.SyftSbomType // syft sbom struct

	cacheState := "n/a"
	key := CacheKey(manifestDigest)

	// try cache, else create
	sbomData, err = kvc.GetExpire(ctx, key, scanTimeout)
	if err != nil && !errors.Is(err, cache.ErrKeyNotFound) {
		return noResult, nil, nil, err
	}

	if errors.Is(err, cache.ErrKeyNotFound) {
		// cache miss: generate sbom
		cacheState = "cache miss"
		if sbomProd, sbomData, err = sbomEngine.CreateSbom(imageRef, platform, auth, tlsCheck, "syft-json"); err != nil {
			return noResult, nil, nil, err
		}
		// cache sbom
		if err := kvc.SetExpire(ctx, key, sbomData, scanTimeout); err != nil {
			return noResult, nil, nil, err
		}
	} else {
		// cache hit, parse []byte
		cacheState = "cache hit"
		if err := sbomProd.FromBytes(sbomData); err != nil {
			return noResult, nil, nil, err
		}
	}
	var scanResult model.PharosImageScanResult
	var scanProd grypetype.GrypeScanType
	var scanData []byte

	// scan sbom
	if scanProd, scanData, err = scanEngine.VulnScanSbom(sbomData); err != nil {
		logger.Fatal().Err(err).Msg("VulnScanSbom()")
	}
	if err = scanResult.LoadGrypeImageScan(sbomProd, scanProd); err != nil {
		logger.Fatal().Err(err).Msg("scanResult.LoadGrypeScan()")
	}

	logger.Info().
		Str("key", key).
		Str("cache", cacheState).
		Any("time.scan_timeout", scanTimeout.String()).
		Any("time.cache_expiry", cacheExpiry.String()).
		Any("img.distro", sbomProd.Distro.Name+" "+sbomProd.Source.Version).
		Any("img.size", humanize.Bytes(sbomProd.Source.Metadata.ImageSize)).
		Any("scan.findings", len(scanResult.Findings)).
		Any("scan.vulns", len(scanResult.Vulnerabilities)).
		Any("scan.packages", len(scanResult.Packages)).
		Msg("")

	return scanResult, sbomData, scanData, nil
}
