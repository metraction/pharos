package routing

import (
	"context"

	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/internal/integrations/db"
	pharosstreams "github.com/metraction/pharos/internal/integrations/streams"
	"github.com/metraction/pharos/pkg/mappers"
	"github.com/metraction/pharos/pkg/model"
	"github.com/redis/go-redis/v9"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
	"github.com/rs/zerolog"
)

func NewScanResultCollectorSink(
	ctx context.Context,
	rdb *redis.Client,
	databaseContext *model.DatabaseContext,
	config *model.ResultCollectorConfig,
	enricher mappers.EnricherConfig,
	log *zerolog.Logger) {
	pharosScanTaskHandler := pharosstreams.NewPharosScanTaskHandler()

	integrations.NewRedisConsumerGroupSource[model.PharosScanResult](ctx, rdb, config.QueueName, config.GroupName, config.ConsumerName, "0", config.BlockTimeout, 1).
		Via(flow.NewPassThrough()).
		Via(flow.NewFilter(pharosScanTaskHandler.FilterFailedTasks, 1)).
		Via(NewScanResultsInternalFlow(enricher)).
		To(db.NewImageDbSink(databaseContext))
}

func NewScanResultsInternalFlow(enricher mappers.EnricherConfig) streams.Flow {
	pharosScanTaskHandler := pharosstreams.NewPharosScanTaskHandler()
	return flow.NewFilter(pharosScanTaskHandler.FilterFailedTasks, 1).
		Via(flow.NewMap(pharosScanTaskHandler.CreateRootContext, 1)).
		Via(mappers.NewResultEnricherMap(enricher))
}
