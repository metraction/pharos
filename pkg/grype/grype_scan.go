package grype

import (
	"context"
	"errors"
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/metraction/pharos/internal/services/cache"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/grypetype"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/pkg/scanning"
	"github.com/metraction/pharos/pkg/syft"
	"github.com/metraction/pharos/pkg/syfttype"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

// execute scan with grype scanner
func ScanImage(task model.PharosImageScanTask, scanEngine *GrypeScanner, kvc *cache.PharosCache, logger *zerolog.Logger) (model.PharosImageScanResult, []byte, []byte, error) {

	logger.Info().Msg("Grype.ScanImage()")

	// return sbom cache key for given digest
	CacheKey := func(digest string) string {
		return lo.Substring(digest, 0, 39) + ".grype.sbom"
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

	var sbomEngine *syft.SyftSbomCreator

	// create sbom generator
	if sbomEngine, err = syft.NewSyftSbomCreator(task.Timeout, logger); err != nil {
		logger.Fatal().Err(err).Msg("NewSyftSbomCreator()")
	}

	var sbomData []byte                // raw data
	var sbomProd syfttype.SyftSbomType // syft sbom struct

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
		if sbomProd, sbomData, err = sbomEngine.CreateSbom(task, "syft-json"); err != nil {
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
		Any("time.scan_timeout", task.Timeout.String()).
		Any("time.cache_expiry", task.ImageSpec.CacheExpiry.String()).
		Any("img.distro", sbomProd.Distro.Name+" "+sbomProd.Source.Version).
		Any("img.size", humanize.Bytes(sbomProd.Source.Metadata.ImageSize)).
		Any("scan.findings", len(scanResult.Findings)).
		Any("scan.vulns", len(scanResult.Vulnerabilities)).
		Any("scan.packages", len(scanResult.Packages)).
		Msg("")

	return scanResult, sbomData, scanData, nil

}
