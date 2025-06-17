package routing

import (
	"context"

	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/services/cache"
	"github.com/metraction/pharos/pkg/grype"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

var logger *zerolog.Logger

func NewScannerFlow(ctx context.Context, cfg *model.Config) error {
	logger = logging.NewLogger("info")

	server, err := integrations.NewRedisGtrsServer[model.PharosScanTask, model.PharosScanResult](ctx, cfg.Redis, cfg.Scanner.RequestQueue, cfg.Scanner.ResponseQueue)
	if err != nil {
		return err
	}

	// connect redis for key value cache
	kvc, err := cache.NewPharosCache(cfg.Scanner.CacheEndpoint, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Redis cache create")
	}
	defer kvc.Close()

	var scanEngine *grype.GrypeScanner

	go server.ProcessRequest(ctx, func(task model.PharosScanTask) model.PharosScanResult {
		logger.Debug().Msg("Processing scan request: " + task.ImageSpec.Image)
		result, _, _, err := grype.ScanImage(task, scanEngine, kvc, logger)
		if err != nil {
			logger.Fatal().Err(err).Msg("grype.ScanImage()")
		}
		return result
	})
	return nil
}
