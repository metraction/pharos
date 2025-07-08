package mappers

import (
	"path/filepath"

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

func NewResultEnricherStream(enricher EnricherConfig) streams.Flow {
	// Create a source that will be used as the initial flow
	// In AppendResultEnricherStream, this is provided as a parameter
	// Here we need to create it ourselves
	var source streams.Flow = flow.NewMap(ToWrappedResult, 1)
	var result streams.Flow
	
	// Apply all enrichers
	for _, mapper := range enricher.Configs {
		config := filepath.Join(enricher.BasePath, mapper.Config)
		switch mapper.Name {
		case "file":
			source = source.Via(flow.NewMap(Wrap(NewAppendFile[map[string]interface{}](config)), 1))
		case "hbs":
			source = source.Via(flow.NewMap(Wrap(NewPureHbs[map[string]interface{}, map[string]interface{}](config)), 1))
		case "debug":
			source = source.Via(flow.NewMap(Wrap(NewDebug(config)), 1))
		}
		result = source
	}
	
	// If no configs were processed, use the initial source
	if result == nil {
		result = source
	}
	
	// Apply final transformation to unwrap the result
	result = result.Via(flow.NewMap(ToUnWrappedResult, 1))
	
	return result
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
