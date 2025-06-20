package routing

import (
	"context"
	"os"
	"time"

	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/internal/integrations/cache"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/grype"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

var logger *zerolog.Logger

func NewScannerFlow(ctx context.Context, cfg *model.Config) error {
	logger = logging.NewLogger("info")

	server, err := integrations.NewRedisGtrsServer[model.PharosScanTask, model.PharosScanResult](
		ctx, cfg.Redis, cfg.Scanner.RequestQueue, cfg.Scanner.ResponseQueue)
	if err != nil {
		return err
	}

	// connect redis for key value cache
	kvc, err := cache.NewPharosCache(cfg.Scanner.CacheEndpoint, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Redis cache create")
	}
	// TODO close cache connection on shutdown
	// defer kvc.Close()
	if err = kvc.Connect(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Redis cache connect")
	}
	logger.Info().Str("redis_version", kvc.Version(ctx)).Msg("PharosCache.Connect() OK")

	var scanEngine *grype.GrypeScanner
	// create scanner & update database
	scanTimeout, err := time.ParseDuration(cfg.Scanner.Timeout)
	if err != nil {
		logger.Fatal().Err(err).Msg("time.ParseDuration()")
	}
	if scanEngine, err = grype.NewGrypeScanner(scanTimeout, true, logger); err != nil {
		dbCacheDir := os.Getenv("GRYPE_DB_CACHE_DIR")
		logger.Debug().Str("GRYPE_DB_CACHE_DIR", dbCacheDir).Msg("Grype settings: ")
		logger.Fatal().Err(err).Msg("NewGrypeScanner()")
	}

	go server.ProcessRequest(ctx, func(task model.PharosScanTask) model.PharosScanResult {
		logger.Debug().Msg("Processing scan request: " + task.ImageSpec.Image)
		result, _, _, err := grype.ScanImage(task, scanEngine, kvc, logger)
		if err != nil {
			logger.Error().Err(err).Msg("grype.ScanImage()")
		}

		// Log the number of findings, vulnerabilities, and packages before sending
		logger.Info().Int("findings", len(result.Findings)).Int("vulns", len(result.Vulnerabilities)).Int("pkgs", len(result.Packages)).Msg("Sending scan results")

		// Now we can return the original result since we've fixed the serialization at the model level
		return result
	})
	return nil
}
