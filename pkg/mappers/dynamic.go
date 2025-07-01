package mappers

import (
	"path/filepath"

	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
)

type EnricherConfig struct {
	Name   string
	Config string
}

func NewEnricherStream(stream streams.Source, enrichers []EnricherConfig, basePath string) streams.Flow {
	var result streams.Flow
	for _, enricher := range enrichers {
		filePath := filepath.Join(basePath, enricher.Config)
		switch enricher.Name {
		case "file":
			stream = stream.Via(flow.NewMap(NewAppendFile[map[string]interface{}](filePath), 1))
		case "hbs":
			stream = stream.Via(flow.NewMap(NewPureHbs[map[string]interface{}, map[string]interface{}](filePath), 1))
		case "debug":
			stream = stream.Via(flow.NewMap(NewDebug(), 1))
		}
		result = stream.(streams.Flow)
	}
	return result
}
