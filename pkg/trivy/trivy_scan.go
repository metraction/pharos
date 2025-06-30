package trivy

import (
	"context"
	"errors"

	"github.com/dustin/go-humanize"
	"github.com/metraction/pharos/internal/integrations/cache"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/images"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/pkg/trivytype"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

// execute scan with grype scanner
func ScanImage(task model.PharosScanTask2, scanEngine *TrivyScanner, kvc *cache.PharosCache, logger *zerolog.Logger) (model.PharosScanResult, []byte, []byte, error) {

	logger.Debug().Msg("Trivy.ScanImage()")

	// return sbom cache key for given digest
	CacheKey := func(digest string) string {
		return lo.Substring(digest, 0, 39) + ".trivy.sbom"
	}

	ctx := context.Background()
	var scanData []byte
	var scanProd trivytype.TrivyScanType
	var sbomEngine *TrivySbomCreator

	result := model.PharosScanResult{
		ScanTask: task,
	}
	result.ScanTask.Status = "get-digest"
	result.ScanTask.Engine = scanEngine.Engine

	// get manifestDigest to have a platform unique key for caching
	indexDigest, manifestDigest, rxPlatform, err := images.GetImageDigests(task)
	if err != nil {
		result.ScanTask.SetError(err)
		return result, nil, nil, err
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
	if sbomEngine, err = NewTrivySbomCreator(task.ScanTTL, logger); err != nil {
		logger.Fatal().Err(err).Msg("NewTrivySbomCreator()")
	}

	var sbomData []byte                  // raw data
	var sbomProd trivytype.TrivySbomType // trivy sbom struct = CyconeDX

	cacheState := "n/a"
	key := CacheKey(manifestDigest)

	// try cache, else create
	sbomData, err = kvc.GetExpire(ctx, key, task.CacheTTL)
	if err != nil && !errors.Is(err, cache.ErrKeyNotFound) {
		result.ScanTask.SetError(err)
		return result, nil, nil, err
	}

	if errors.Is(err, cache.ErrKeyNotFound) {
		// cache miss: generate sbom
		cacheState = "cache miss"
		result.ScanTask.Status = cacheState
		if sbomProd, sbomData, err = sbomEngine.CreateSbom(task, "cyclonedx"); err != nil {
			result.ScanTask.SetError(err)
			return result, nil, nil, err
		}
		// cache sbom
		if err := kvc.SetExpire(ctx, key, sbomData, task.CacheTTL); err != nil {
			result.ScanTask.SetError(err)
			return result, nil, nil, err

		}
	} else {
		// cache hit, parse []byte
		cacheState = "cache hit"
		result.ScanTask.Status = cacheState
		if err := sbomProd.FromBytes(sbomData); err != nil {
			result.ScanTask.SetError(err)
			return result, nil, nil, err

		}
	}

	// scan sbom for vulns
	result.ScanTask.Status = "scan"
	if scanProd, scanData, err = scanEngine.VulnScanSbom(sbomData); err != nil {
		logger.Fatal().Err(err).Msg("VulnScanSbom()")
	}
	// map produce result to pharos result type
	result.ScanTask.Status = "scan-parse"
	if err = result.LoadTrivyImageScan(sbomProd, scanProd); err != nil {
		result.ScanTask.SetError(err)
		return result, nil, nil, err
	}
	result.ScanTask.Status = "done"
	logger.Debug().
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
