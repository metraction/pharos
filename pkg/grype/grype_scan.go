package grype

import (
	"context"
	"errors"
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/metraction/pharos/internal/integrations/cache"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/grypetype"
	"github.com/metraction/pharos/pkg/images"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/pkg/syft"
	"github.com/metraction/pharos/pkg/syfttype"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

// execute scan with grype scanner
func ScanImage(task model.PharosScanTask2, scanEngine *GrypeScanner, kvc *cache.PharosCache, logger *zerolog.Logger) (model.PharosScanResult, []byte, []byte, error) {

	logger.Debug().Msg("ScanImage() ..")

	// return sbom cache key for given digest
	CacheKey := func(digest string) string {
		return lo.Substring(digest, 0, 39) + ".grype.sbom"
	}

	ctx := context.Background()
	var scanData []byte
	var scanProd grypetype.GrypeScanType
	var sbomEngine *syft.SyftSbomCreator

	result := model.PharosScanResult{
		ScanTask: task,
	}
	result.ScanTask.Status = "get-digest"
	result.ScanTask.Engine = scanEngine.Engine

	// get manifestDigest to have a platform unique key for cachings
	indexDigest, manifestDigest, rxPlatform, err := images.GetImageDigests(task)
	if err != nil {
		result.ScanTask.SetError(err)
		return result, nil, nil, fmt.Errorf("error getting digests: image:%s %w", task.ImageSpec, err)
	}
	result.ScanTask.RxDigest = manifestDigest
	result.ScanTask.RxPlatform = rxPlatform

	logger.Info().
		Str("digest.idx", utils.ShortDigest(indexDigest)).
		Str("digest.man", utils.ShortDigest(manifestDigest)).
		Str("rxPlatform", rxPlatform).
		Str("image", task.ImageSpec).
		Msg("GetDigest()")

	// create sbom generator
	if sbomEngine, err = syft.NewSyftSbomCreator(task.ScanTTL, logger); err != nil {
		logger.Fatal().Err(err).Msg("NewSyftSbomCreator()")
	}

	var sbomData []byte                // raw data
	var sbomProd syfttype.SyftSbomType // syft sbom struct

	cacheState := "n/a"
	key := CacheKey(manifestDigest)

	// try cache, else create
	sbomData, err = kvc.GetExpireUnpack(ctx, key, task.CacheTTL)
	if err != nil && !errors.Is(err, cache.ErrKeyNotFound) {
		result.ScanTask.SetError(err)
		return result, nil, nil, fmt.Errorf("image:%s %w", task.ImageSpec, err)
	}

	if errors.Is(err, cache.ErrKeyNotFound) {
		// cache miss: generate sbom
		cacheState = "cache miss"
		result.ScanTask.Status = cacheState
		if sbomProd, sbomData, err = sbomEngine.CreateSbom(task, "syft-json"); err != nil {
			result.ScanTask.SetError(err)
			return result, nil, nil, fmt.Errorf("image:%s %w", task.ImageSpec, err)
		}
		// cache sbom
		if err := kvc.SetExpirePack(ctx, key, sbomData, task.CacheTTL); err != nil {
			result.ScanTask.SetError(err)
			return result, nil, nil, fmt.Errorf("image:%s %w", task.ImageSpec, err)
		}
	} else {
		// cache hit, parse []byte
		cacheState = "cache hit"
		result.ScanTask.Status = cacheState
		if err := sbomProd.FromBytes(sbomData); err != nil {
			result.ScanTask.SetError(err)
			return result, nil, nil, fmt.Errorf("image:%s %w", task.ImageSpec, err)
		}
	}

	// scan sbom for vulns
	result.ScanTask.Status = "scan"
	if scanProd, scanData, err = scanEngine.VulnScanSbom(sbomData); err != nil {
		logger.Fatal().Err(err).Msg("VulnScanSbom()")
	}
	// map produce result to pharos result type
	result.ScanTask.Status = "parse-scan"
	if err = result.LoadGrypeImageScan(sbomProd, scanProd); err != nil {
		result.ScanTask.SetError(err)
		return result, nil, nil, fmt.Errorf("image:%s %w", task.ImageSpec, err)
	}

	result.ScanTask.Status = "done"
	logger.Info().
		Str("cache", cacheState).
		Any("t.scan_timeout", task.ScanTTL.String()).
		Any("t.cache_expiry", task.CacheTTL.String()).
		Any("i.distro", result.Image.DistroName+" "+result.Image.DistroVersion).
		Any("i.size", humanize.Bytes(result.Image.Size)).
		Any("s.findings", len(result.Findings)).
		Any("s.vulns", len(result.Vulnerabilities)).
		Any("s.packages", len(result.Packages)).
		Msg("ScanImage() OK")

	return result, sbomData, scanData, nil

}
