package trivy

import (
	"context"
	"errors"
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/metraction/pharos/internal/services/cache"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/pkg/scanning"
	"github.com/metraction/pharos/pkg/trivytype"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

// execute scan with grype scanner
func ScanImage(task model.PharosImageScanTask, scanEngine *TrivyScanner, kvc *cache.PharosCache, logger *zerolog.Logger) (model.PharosImageScanResult, []byte, []byte, error) {

	logger.Info().Msg("Trivy.ScanImage()")

	// return sbom cache key for given digest
	CacheKey := func(digest string) string {
		return lo.Substring(digest, 0, 39) + ".trivy.sbom"
	}

	ctx := context.Background()
	noResult := model.PharosImageScanResult{}

	// get manifestDigest to have a platform unique key for caching
	indexDigest, manifestDigest, err := scanning.GetImageDigests(task)
	if err != nil {
		return noResult, nil, nil, fmt.Errorf("get image digest: %v", err)
	}

	logger.Info().
		Str("digest.idx", utils.ShortDigest(indexDigest)).
		Str("digest.man", utils.ShortDigest(manifestDigest)).
		Msg("image digests")

	var sbomEngine *TrivySbomCreator

	// create sbom generator
	if sbomEngine, err = NewTrivySbomCreator(task.Timeout, logger); err != nil {
		logger.Fatal().Err(err).Msg("NewTrivySbomCreator()")
	}

	var sbomData []byte                  // raw data
	var sbomProd trivytype.TrivySbomType // trivy sbom struct = CyconeDX

	cacheState := "n/a"
	key := CacheKey(manifestDigest)

	// try cache, else create
	sbomData, err = kvc.GetExpire(ctx, key, task.Timeout)
	if err != nil && !errors.Is(err, cache.ErrKeyNotFound) {
		return noResult, nil, nil, err
	}

	if errors.Is(err, cache.ErrKeyNotFound) {
		// cache miss: generate sbom
		cacheState = "cache miss"
		if sbomProd, sbomData, err = sbomEngine.CreateSbom(task, "cyclonedx"); err != nil {
			return noResult, nil, nil, err
		}
		// cache sbom
		if err := kvc.SetExpire(ctx, key, sbomData, task.ImageSpec.CacheExpiry); err != nil {
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
	var scanProd trivytype.TrivyScanType
	var scanData []byte

	// scan sbom
	if scanProd, scanData, err = scanEngine.VulnScanSbom(sbomData); err != nil {
		logger.Fatal().Err(err).Msg("VulnScanSbom()")
	}
	if err = scanResult.LoadTrivyImageScan(sbomProd, scanProd); err != nil {
		logger.Fatal().Err(err).Msg("scanResult.LoadTrivyScan()")
	}

	logger.Info().
		Str("key", key).
		Str("cache", cacheState).
		Any("time.scan_timeout", task.Timeout.String()).
		Any("time.cache_expiry", task.ImageSpec.CacheExpiry.String()).
		Any("img.distro", scanResult.Image.DistroName+" "+scanResult.Image.DistroVersion).
		Any("img.size", humanize.Bytes(scanResult.Image.Size)).
		Any("scan.findings", len(scanResult.Findings)).
		Any("scan.vulns", len(scanResult.Vulnerabilities)).
		Any("scan.packages", len(scanResult.Packages)).
		Msg("")

	return scanResult, sbomData, scanData, nil

}
