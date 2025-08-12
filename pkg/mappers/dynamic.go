package mappers

import (
	"fmt"
	"path/filepath"

	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
	"gopkg.in/yaml.v3"
)

// LoadMappersConfig reads a YAML file from the given path and returns a slice of MapperConfig.
// The YAML file can either contain a direct list of mappers or a map with a key (e.g., "results")
// that contains a list of mappers with name and config properties.
func LoadMappersConfig(data []byte) (map[string][]model.MapperConfig, error) {

	// If that fails, try to parse as a map with a key containing the list
	var configMap map[string][]model.MapperConfig
	err := yaml.Unmarshal(data, &configMap)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse config file: %w", err)
	}
	return configMap, nil
}

func NewEnricherMap(name string, enricher model.EnricherConfig) streams.Flow {
	// Create a single functions composition of functions resulting in
	logger.Info().Interface("enricher", enricher).Msg("Creating enricher map")
	// WrappedResult and passing it to next function
	return flow.NewMap(func(scanResult model.PharosScanResult) model.PharosScanResult {
		// Step 1: Wrap the result
		wrapped := ToWrappedResult(scanResult)

		// Step 2: Apply all enrichers in sequence
		for _, mapper := range enricher.Configs {
			config := filepath.Join(enricher.BasePath, mapper.Config)
			switch mapper.Name {
			case "file":
				wrapped = Wrap(NewAppendFile(config))(wrapped)
			case "hbs":
				wrapped = Wrap(NewPureHbs[map[string]interface{}, map[string]interface{}](config))(wrapped)
			case "starlark":
				wrapped = Wrap(NewStarlark(config))(wrapped)
			case "debug":
				wrapped = Wrap(NewDebug(config))(wrapped)
			}
		}

		// Step 3: Unwrap the result
		return ToUnWrappedResult(name)(wrapped)
	}, 1)
}
