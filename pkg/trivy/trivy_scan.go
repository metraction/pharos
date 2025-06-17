package trivy

import (
	"context"
	"errors"

	"github.com/dustin/go-humanize"
	"github.com/metraction/pharos/internal/integrations/cache"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/pkg/scanning"
	"github.com/metraction/pharos/pkg/trivytype"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

// execute scan with grype scanner
func ScanImage(task model.PharosScanTask, scanEngine *TrivyScanner, kvc *cache.PharosCache, logger *zerolog.Logger) (model.PharosScanResult, []byte, []byte, error) {

	logger.Info().Msg("Trivy.ScanImage()")

	// return sbom cache key for given digest
	CacheKey := func(digest string) string {
		return lo.Substring(digest, 0, 39) + ".trivy.sbom"
	}

	ctx := context.Background()

	var scanData []byte
	var scanProd trivytype.TrivyScanType
	var sbomEngine *TrivySbomCreator

	task.SbomEngine = "trivy"
	task.ScanEngine = scanEngine.Engine + " " + scanEngine.ScannerVersion

	result := model.PharosScanResult{
		ScanTask: task,
	}

	// get manifestDigest to have a platform unique key for caching
	result.SetStatus("get-digest")
	indexDigest, manifestDigest, err := scanning.GetImageDigests(task)
	if err != nil {
		return result.SetError(err), nil, nil, err
	}

	logger.Info().
		Str("digest.idx", utils.ShortDigest(indexDigest)).
		Str("digest.man", utils.ShortDigest(manifestDigest)).
		Msg("image digests")

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
		return result.SetError(err), nil, nil, err
	}

	if errors.Is(err, cache.ErrKeyNotFound) {
		// cache miss: generate sbom
		cacheState = "cache miss"
		result.SetStatus("cache-miss")
		if sbomProd, sbomData, err = sbomEngine.CreateSbom(task, "cyclonedx"); err != nil {
			return result.SetError(err), nil, nil, err
		}
		// cache sbom
		if err := kvc.SetExpire(ctx, key, sbomData, task.ImageSpec.CacheExpiry); err != nil {
			return result.SetError(err), nil, nil, err
		}
	} else {
		// cache hit, parse []byte
		cacheState = "cache hit"
		result.SetStatus("cache-hit")
		if err := sbomProd.FromBytes(sbomData); err != nil {
			return result.SetError(err), nil, nil, err
		}
	}

	// scan sbom for vulns
	result.SetStatus("scan")
	if scanProd, scanData, err = scanEngine.VulnScanSbom(sbomData); err != nil {
		logger.Fatal().Err(err).Msg("VulnScanSbom()")
	}
	// map produce result to pharos result type
	result.SetStatus("parse-scan")
	if err = result.LoadTrivyImageScan(sbomProd, scanProd); err != nil {
		return result.SetError(err), nil, nil, err

	}
	result.SetStatus("done")
	logger.Info().
		Str("key", key).
		Str("cache", cacheState).
		Any("time.scan_timeout", task.Timeout.String()).
		Any("time.cache_expiry", task.ImageSpec.CacheExpiry.String()).
		Any("img.distro", result.Image.DistroName+" "+result.Image.DistroVersion).
		Any("img.size", humanize.Bytes(result.Image.Size)).
		Any("scan.findings", len(result.Findings)).
		Any("scan.vulns", len(result.Vulnerabilities)).
		Any("scan.packages", len(result.Packages)).
		Msg("")

	return result, sbomData, scanData, nil

}
