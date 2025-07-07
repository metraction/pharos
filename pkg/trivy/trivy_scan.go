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

// execute scan with trivy scanner, return engine specific sbom and scan results as []byte
func ScanImage(task model.PharosScanTask2, scanEngine *TrivyScanner, kvc *cache.PharosCache, logger *zerolog.Logger) (model.PharosScanResult, []byte, []byte, error) {

	logger.Debug().Msg("TrivyScan() ..")

	// return sbom cache key for given digest
	CacheKey := func(digest string) string {
		return lo.Substring(digest, 0, 39) + ".trivy.sbom"
	}

	ctx := context.Background()
	elapsed := utils.ElapsedFunc()

	var scanData []byte
	var scanProd trivytype.TrivyScanType
	var sbomEngine *TrivySbomCreator

	result := model.PharosScanResult{
		ScanTask: task,
		ScanMeta: model.PharosScanMeta{
			Engine: scanEngine.Engine,
		},
	}
	result.ScanTask.Engine = scanEngine.Engine // TODO: Remove as covered in result.ScanMeta

	// get manifest and manifest Digest, this also ensures the image still exists, or is updated if image behind tag (latest) has changed
	result.SetTaskStatus("get-digest")
	indexDigest, manifestDigest, rxPlatform, err := images.GetImageDigests(task)
	if err != nil {
		result.SetTaskError(err).SetScanElapsed(elapsed())
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
	result.SetTaskStatus("get-sbom")
	sbomData, err = kvc.GetExpire(ctx, key, task.CacheTTL)
	if err != nil && !errors.Is(err, cache.ErrKeyNotFound) {
		result.SetTaskError(err).SetScanElapsed(elapsed())
		return result, nil, nil, err
	}

	if errors.Is(err, cache.ErrKeyNotFound) {
		// cache miss: generate sbom
		cacheState = "cache miss"
		result.SetTaskStatus(cacheState)
		if sbomProd, sbomData, err = sbomEngine.CreateSbom(task, "cyclonedx"); err != nil {
			result.SetTaskError(err).SetScanElapsed(elapsed())
			return result, nil, nil, err
		}
		// cache sbom
		if err := kvc.SetExpire(ctx, key, sbomData, task.CacheTTL); err != nil {
			result.SetTaskError(err).SetScanElapsed(elapsed())
			return result, nil, nil, err

		}
	} else {
		// cache hit, parse []byte
		cacheState = "cache hit"
		result.SetTaskStatus(cacheState)
		if err := sbomProd.FromBytes(sbomData); err != nil {
			result.SetTaskError(err).SetScanElapsed(elapsed())
			return result, nil, nil, err
		}
	}

	// scan sbom for vulns
	result.SetTaskStatus("scan")
	if scanProd, scanData, err = scanEngine.VulnScanSbom(sbomData); err != nil {
		logger.Fatal().Err(err).Msg("VulnScanSbom()")
	}
	// map produce result to pharos result type
	result.SetTaskStatus("scan-parse")
	if err = result.LoadTrivyImageScan(manifestDigest, task, sbomProd, scanProd); err != nil {
		result.SetTaskError(err).SetScanElapsed(elapsed())
		return result, nil, nil, err
	}
	result.SetTaskStatus("done").SetScanElapsed(elapsed())
	logger.Info().
		Str("cache", cacheState).
		Str("manDigest", utils.ShortDigest(manifestDigest)).
		Any("t.scan_timeout", task.ScanTTL.String()).
		Any("t.cache_expiry", task.CacheTTL.String()).
		Any("i.distro", result.Image.DistroName+" "+result.Image.DistroVersion).
		Any("i.size", humanize.Bytes(result.Image.Size)).
		Any("s.findings", len(result.Findings)).
		Any("s.vulns", len(result.Vulnerabilities)).
		Any("s.packages", len(result.Packages)).
		Msg("TrivyScan() OK")

	return result, sbomData, scanData, nil

}
