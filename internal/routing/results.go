package routing

import (
	"context"

	pharosstreams "github.com/metraction/pharos/internal/integrations/streams"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
	"github.com/rs/zerolog"
)

func NewScanResultCollectorFlow(
	ctx context.Context,
	config *model.Config,
	source streams.Source,
	log *zerolog.Logger) streams.Flow {
	pharosScanTaskHandler := pharosstreams.NewPharosScanTaskHandler()

	redisFlow := source.
		Via(NewScannerFlow(ctx, config)).
		Via(flow.NewMap(pharosScanTaskHandler.UpdateScanTaskMetrics, 1)).
		Via(flow.NewFilter(pharosScanTaskHandler.FilterFailedTasks, 1)).
		Via(flow.NewMap(pharosScanTaskHandler.UpdateScanTime, 1)).
		Via(flow.NewMap(pharosScanTaskHandler.NotifyReceiver, 1))
	return NewScanResultsInternalFlow(redisFlow)
}

func NewScanResultsInternalFlow(source streams.Source) streams.Flow {
	pharosScanTaskHandler := pharosstreams.NewPharosScanTaskHandler()
	contextFlow := source.
		Via(flow.NewMap(pharosScanTaskHandler.UpdateScanTaskMetrics, 1)).
		Via(flow.NewFilter(pharosScanTaskHandler.FilterFailedTasks, 1)).
		Via(flow.NewMap(pharosScanTaskHandler.CreateRootContext, 1))
	return contextFlow
}
