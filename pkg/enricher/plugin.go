package enricher

import (
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/mappers"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams"
)

var logger = logging.NewLogger("info", "component", "plugin")

func LoadPlugin(config *model.Config, source streams.Source) streams.Flow {
	mapperConfig, err := mappers.LoadMappersConfig("results", config.EnricherPath+"/enricher.yaml")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load mappers config")
	}
	enricherConfig := model.EnricherConfig{
		BasePath: config.EnricherPath,
		Configs:  mapperConfig,
	}
	return mappers.NewResultEnricherStream(source, enricherConfig)
}
