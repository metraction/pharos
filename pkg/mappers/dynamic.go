package mappers

import (
	"path/filepath"

	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
)

type MapperConfig struct {
	Name   string
	Config string
}

type EnricherConfig struct {
	BasePath string
	Configs  []MapperConfig
}

func NewEnricherStream(stream streams.Source, enricher EnricherConfig) streams.Flow {
	var result streams.Flow
	for _, mapper := range enricher.Configs {
		filePath := filepath.Join(enricher.BasePath, mapper.Config)
		switch mapper.Name {
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

func NewResultEnricherStream(stream streams.Source, enricher EnricherConfig) streams.Flow {
	var result streams.Flow
	stream = stream.Via(flow.NewMap(ToWrappedResult, 1))
	for _, mapper := range enricher.Configs {
		filePath := filepath.Join(enricher.BasePath, mapper.Config)
		switch mapper.Name {
		case "file":
			stream = stream.Via(flow.NewMap(Wrap(NewAppendFile[map[string]interface{}](filePath)), 1))
		case "hbs":
			stream = stream.Via(flow.NewMap(Wrap(NewPureHbs[map[string]interface{}, map[string]interface{}](filePath)), 1))
		case "debug":
			stream = stream.Via(flow.NewMap(Wrap(NewDebug()), 1))
		}
		result = stream.(streams.Flow)
	}
	result = result.Via(flow.NewMap(ToUnWrappedResult, 1))
	return result
}

func ToWrappedResult(result model.PharosScanResult) WrappedResult {
	return WrappedResult{
		Result:  result,
		Context: ToMap(result),
	}
}

func ToUnWrappedResult(result WrappedResult) model.PharosScanResult {
	scanResult := result.Result
	scanResult.ScanTask.Context = result.Context
	return scanResult
}
