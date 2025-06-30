package routing

import (
	"context"

	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/pkg/model"
	"github.com/redis/go-redis/v9"
	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
	"github.com/rs/zerolog"
)

func NewScanResultCollectorFlow(
	ctx context.Context,
	rdb *redis.Client,
	databaseContext *model.DatabaseContext,
	config *model.ResultCollectorConfig,
	log *zerolog.Logger) {

	integrations.NewRedisConsumerGroupSource[model.PharosScanResult](ctx, rdb, config.QueueName, config.GroupName, config.ConsumerName, "0", config.BlockTimeout, 1).
		Via(flow.NewPassThrough()).
		Via(flow.NewMap(func(item model.PharosScanResult) model.PharosScanResult {
			if item.ScanTask.Error != "" {
				log.Warn().Str("JobId", item.ScanTask.JobId).Str("error", item.ScanTask.Error).Msg("Scan task failed during async scan")
			} else {
				if err := integrations.SaveScanResult(databaseContext, &item); err != nil {
					log.Error().Msg("Async result saving error")
				}
			}

			//time.Sleep(10 * time.Second) // Simulate waiting for the scan to complete
			log.Info().Str("image", item.ScanTask.ImageSpec).Msg("Async scan completed")
			return item
		}, 1)).
		To(extension.NewStdoutSink())
}
