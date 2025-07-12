package routing

import (
	"context"

	pharosstreams "github.com/metraction/pharos/internal/integrations/streams"
	"github.com/metraction/pharos/pkg/mappers"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
	"github.com/rs/zerolog"
)

func NewScanResultCollectorFlow(
	ctx context.Context,
	config *model.Config,
	enricher model.EnricherConfig,
	source streams.Source,
	log *zerolog.Logger) streams.Flow {
	pharosScanTaskHandler := pharosstreams.NewPharosScanTaskHandler()

	redisFlow := source.
		Via(NewScannerFlow(ctx, config)).
		Via(flow.NewFilter(pharosScanTaskHandler.FilterFailedTasks, 1)).
		Via(flow.NewMap(pharosScanTaskHandler.UpdateScanTime, 1)).
		Via(flow.NewMap(pharosScanTaskHandler.NotifyReceiver, 1))

	return NewScanResultsInternalFlow(redisFlow, enricher)

}

func NewScanResultsInternalFlow(source streams.Source, enricher model.EnricherConfig) streams.Flow {
	pharosScanTaskHandler := pharosstreams.NewPharosScanTaskHandler()
	contextFlow := source.
		Via(flow.NewFilter(pharosScanTaskHandler.FilterFailedTasks, 1)).
		Via(flow.NewMap(pharosScanTaskHandler.CreateRootContext, 1))
	stream := mappers.NewResultEnricherStream(contextFlow, enricher)

	return stream
}
