package routing

import (
	"context"

	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/internal/integrations/streams"
	"github.com/metraction/pharos/pkg/model"
	"github.com/redis/go-redis/v9"
	"github.com/reugn/go-streams/flow"
	"github.com/rs/zerolog"
)

func NewScanResultCollectorFlow(
	ctx context.Context,
	rdb *redis.Client,
	databaseContext *model.DatabaseContext,
	config *model.ResultCollectorConfig,
	log *zerolog.Logger) {
	pharosScanTaskHandler := streams.NewPharosScanTaskHandler()
	imageDbSink := streams.NewImageDbSink(databaseContext)
	integrations.NewRedisConsumerGroupSource[model.PharosScanResult](ctx, rdb, config.QueueName, config.GroupName, config.ConsumerName, "0", config.BlockTimeout, 1).
		Via(flow.NewPassThrough()).
		Via(flow.NewFilter(pharosScanTaskHandler.FilterFailedTasks, 1)).
		Via(flow.NewMap(pharosScanTaskHandler.CreateRootContext, 1)).
		To(imageDbSink)
}
