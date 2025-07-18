package mappers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
	"gopkg.in/yaml.v3"
)

// LoadMappersConfig reads a YAML file from the given path and returns a slice of MapperConfig.
// The YAML file can either contain a direct list of mappers or a map with a key (e.g., "results")
// that contains a list of mappers with name and config properties.
func LoadMappersConfig(name string, configPath string) ([]model.MapperConfig, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path is empty")
	}

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read config file: %w", err)
	}

	// First try to parse as a direct list of MapperConfig
	var configs []model.MapperConfig
	err = yaml.Unmarshal(data, &configs)
	if err != nil || len(configs) == 0 {
		// If that fails, try to parse as a map with a key containing the list
		var configMap map[string][]model.MapperConfig
		err = yaml.Unmarshal(data, &configMap)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse config file: %w", err)
		}
		// Look for any key that contains a non-empty list of configs
		for _, value := range configMap {
			if len(value) > 0 {
				configs = value
				break
			}
		}
		// If no valid configs found, return an error
		if len(configs) == 0 {
			return nil, fmt.Errorf("no valid mapper configurations found in config file")
		}
		return configMap[name], nil
	}
	return configs, nil
}

func NewEnricherStream(stream streams.Source, enricher model.EnricherConfig) streams.Flow {
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

func NewResultEnricherStream(stream streams.Source, name string, enricher model.EnricherConfig) streams.Flow {
	var result streams.Flow

	// In case no enrichers return stream converted to flow
	if len(enricher.Configs) == 0 {
		return stream.Via(flow.NewPassThrough())
	}
	stream = stream.Via(flow.NewMap(ToWrappedResult, 1))
	for _, mapper := range enricher.Configs {
		config := filepath.Join(enricher.BasePath, mapper.Config)
		switch mapper.Name {
		case "file":
			stream = stream.Via(flow.NewMap(Wrap(NewAppendFile[map[string]interface{}](config)), 1))
		case "hbs":
			stream = stream.Via(flow.NewMap(Wrap(NewPureHbs[map[string]interface{}, map[string]interface{}](config)), 1))
		case "starlark":
			stream = stream.Via(flow.NewMap(Wrap(NewStarlark(config)), 1))
		case "debug":
			stream = stream.Via(flow.NewMap(Wrap(NewDebug(config)), 1))
		}
		result = stream.(streams.Flow)
	}
	result = result.Via(flow.NewMap(ToUnWrappedResult(name), 1))
	return result
}
