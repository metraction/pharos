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

func NewEnricherMap(name string, enricher model.EnricherConfig, enricherCommon *model.EnricherCommonConfig) streams.Flow {
	// Create a single functions composition of functions resulting in
	logger.Info().Str("enricher", enricher.BasePath).Str("name", name).Msg("Creating enricher map")
	// WrappedResult and passing it to next function
	if enricher.Enricher == nil {
		return flow.NewMap(func(scanResult model.PharosScanResult) model.PharosScanResult {
			// Step 1: Wrap the result
			wrapped := ToWrappedResult(scanResult)
			processed := false

			// Step 2: Apply all enrichers in sequence
			for _, mapper := range enricher.Configs {
				config := filepath.Join(enricher.BasePath, mapper.Config)

				switch mapper.Name {
				case "file":
					wrapped = Wrap(NewAppendFile(config, mapper.Ref))(wrapped)
					processed = true
				case "hbs":
					wrapped = Wrap(NewPureHbs[map[string]interface{}, map[string]interface{}](config))(wrapped)
					processed = true
				case "starlark":
					wrapped = Wrap(NewStarlark(config))(wrapped)
					processed = true
				case "debug":
					wrapped = Wrap(NewDebug(config))(wrapped)
					processed = true
				}
			}

			if !processed {
				logger.Warn().Str("name", name).Msg("No valid enricher found, passing through")
				return scanResult
			}

			// Step 4: Unwrap the result
			return ToUnWrappedResult(name)(wrapped)
		}, 1)
	} else {
		return flow.NewMap(func(scanResult model.PharosScanResult) model.PharosScanResult {
			wrapped := ToWrappedResult(scanResult)
			logger.Info().Str("name", enricher.Enricher.Name).Msg("Applying database enricher")
			if enricher.Enricher.Type == "visual" {
				wrapped = Wrap(NewVisual(enricher.Enricher, enricherCommon))(wrapped)
			}
			return ToUnWrappedResult(enricher.Enricher.Name)(wrapped)
		}, 1)
	}
}
