package routing

import (
	"context"

	"github.com/metraction/pharos/integrations"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
)

func NewScannerFlow(ctx context.Context, cfg *model.Config, streamName string) error {
	source, err := integrations.NewRedisStreamSource(ctx, cfg.Redis, streamName, "scanner", "scanner", "0", 0, 1)
	if err != nil {
		return err
	}

	go source.
		Via(flow.NewMap(func(msg any) any {
			// Do some scanning
			return msg
		}, 1)).
		To(extension.NewStdoutSink())

	return nil
}
