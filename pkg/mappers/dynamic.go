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
		switch mapper.Name {
		case "file":
			stream = stream.Via(flow.NewMap(NewAppendFile[map[string]interface{}](filepath.Join(enricher.BasePath, mapper.Config)), 1))
		case "hbs":
			stream = stream.Via(flow.NewMap(NewPureHbs[map[string]interface{}, map[string]interface{}](filepath.Join(enricher.BasePath, mapper.Config)), 1))
		case "debug":
			stream = stream.Via(flow.NewMap(NewDebug(mapper.Config), 1))
		}
		result = stream.(streams.Flow)
	}
	return result
}

func NewResultEnricherMap(enricher EnricherConfig) streams.Flow {
	// Create a single functions composition of functions resulting in
	// WrappedResult and passing it to next function
	return flow.NewMap(func(scanResult model.PharosScanResult) model.PharosScanResult {
		// Step 1: Wrap the result
		wrapped := ToWrappedResult(scanResult)

		// Step 2: Apply all enrichers in sequence
		for _, mapper := range enricher.Configs {
			config := filepath.Join(enricher.BasePath, mapper.Config)
			switch mapper.Name {
			case "file":
				wrapped = Wrap(NewAppendFile[map[string]interface{}](config))(wrapped)
			case "hbs":
				wrapped = Wrap(NewPureHbs[map[string]interface{}, map[string]interface{}](config))(wrapped)
			case "debug":
				wrapped = Wrap(NewDebug(config))(wrapped)
			}
		}

		// Step 3: Unwrap the result
		return ToUnWrappedResult(wrapped)
	}, 1)
}

func AppendResultEnricherStream(stream streams.Source, enricher EnricherConfig) streams.Flow {
	var result streams.Flow
	stream = stream.Via(flow.NewMap(ToWrappedResult, 1))
	for _, mapper := range enricher.Configs {
		config := filepath.Join(enricher.BasePath, mapper.Config)
		switch mapper.Name {
		case "file":
			stream = stream.Via(flow.NewMap(Wrap(NewAppendFile[map[string]interface{}](config)), 1))
		case "hbs":
			stream = stream.Via(flow.NewMap(Wrap(NewPureHbs[map[string]interface{}, map[string]interface{}](config)), 1))
		case "debug":
			stream = stream.Via(flow.NewMap(Wrap(NewDebug(config)), 1))
		}
		result = stream.(streams.Flow)
	}
	result = result.Via(flow.NewMap(ToUnWrappedResult, 1))
	return result
}
