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

// execute scan with grype scanner, return engine specific sbom and scan results as []byte
func ScanImage(task model.PharosScanTask2, scanEngine *GrypeScanner, kvc *cache.PharosCache, logger *zerolog.Logger) (model.PharosScanResult, []byte, []byte, error) {

	logger.Debug().Msg("GrypeScan() ..")

	// return sbom cache key for given digest
	CacheKey := func(digest string) string {
		return lo.Substring(digest, 0, 39) + ".grype.sbom"
	}

	ctx := context.Background()
	elapsed := utils.ElapsedFunc()

	var scanData []byte
	var scanProd grypetype.GrypeScanType
	var sbomEngine *syft.SyftSbomCreator

	// initialize scan result type
	result := model.PharosScanResult{
		ScanTask: task,
		ScanMeta: model.PharosScanMeta{
			Engine: scanEngine.Engine,
		},
	}

	// TODO: obseolete, Engine is in result.ScanMeta
	result.ScanTask.Engine = scanEngine.Engine

	// get manifest and manifest Digest, this also ensures the image still exists, or is updated if image behind tag (latest) has changed
	result.SetTaskStatus("get-digest")
	indexDigest, manifestDigest, rxPlatform, err := images.GetImageDigests(task)
	if err != nil {
		result.SetTaskError(err).SetScanElapsed(elapsed())
		return result, nil, nil, fmt.Errorf("image:%s %w", task.ImageSpec, err)
	}
	result.ScanTask.RxDigest = manifestDigest
	result.ScanTask.RxPlatform = rxPlatform

	logger.Debug().
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
	result.SetTaskStatus("get-sbom")
	sbomData, err = kvc.GetExpireUnpack(ctx, key, task.CacheTTL)
	if err != nil && !errors.Is(err, cache.ErrKeyNotFound) {
		result.SetTaskError(err).SetScanElapsed(elapsed())
		return result, nil, nil, fmt.Errorf("image:%s %w", task.ImageSpec, err)
	}

	if errors.Is(err, cache.ErrKeyNotFound) {
		// cache miss: generate sbom
		cacheState = "cache miss"
		result.SetTaskStatus(cacheState)
		if sbomProd, sbomData, err = sbomEngine.CreateSbom(task, "syft-json"); err != nil {
			result.SetTaskError(err).SetScanElapsed(elapsed())
			return result, nil, nil, fmt.Errorf("image:%s %w", task.ImageSpec, err)
		}
		// cache sbom
		if err := kvc.SetExpirePack(ctx, key, sbomData, task.CacheTTL); err != nil {
			result.SetTaskError(err).SetScanElapsed(elapsed())
			return result, nil, nil, fmt.Errorf("image:%s %w", task.ImageSpec, err)
		}
	} else {
		// cache hit, parse []byte
		cacheState = "cache hit"
		result.SetTaskStatus(cacheState)
		if err := sbomProd.FromBytes(sbomData); err != nil {
			result.SetTaskError(err).SetScanElapsed(elapsed())
			return result, nil, nil, fmt.Errorf("image:%s %w", task.ImageSpec, err)
		}
	}

	// scan sbom for vulns
	result.SetTaskStatus("scan")
	if scanProd, scanData, err = scanEngine.VulnScanSbom(sbomData); err != nil {
		logger.Fatal().Err(err).Msg("VulnScanSbom()")
	}

	// map engine results to generic PharosResult
	result.SetTaskStatus("parse-scan")
	if err = result.LoadGrypeImageScan(manifestDigest, task, sbomProd, scanProd); err != nil {
		result.SetTaskError(err).SetScanElapsed(elapsed())
		return result, nil, nil, fmt.Errorf("image:%s %w", task.ImageSpec, err)
	}

	result.SetTaskStatus("done").SetScanElapsed(elapsed())
	logger.Info().
		Str("cache", cacheState).
		Str("manDigest1", utils.ShortDigest(manifestDigest)).
		Str("manDigest2", utils.ShortDigest(result.Image.ManifestDigest)).
		Any("t.scan_timeout", task.ScanTTL.String()).
		Any("t.cache_expiry", task.CacheTTL.String()).
		Any("i.distro", result.Image.DistroName+" "+result.Image.DistroVersion).
		Any("i.size", humanize.Bytes(result.Image.Size)).
		Any("s.findings", len(result.Findings)).
		Any("s.vulns", len(result.Vulnerabilities)).
		Any("s.packages", len(result.Packages)).
		Msg("GrypeScan() OK")

	return result, sbomData, scanData, nil

}
