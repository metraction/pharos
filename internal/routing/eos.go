package routing

import (
	"github.com/metraction/pharos/pkg/mappers"
	"github.com/reugn/go-streams"
)

func NewEosEnricher(source streams.Source, basePath string) streams.Flow {
	enrichers := []mappers.EnricherConfig{
		{Name: "file", Config: "eos.yaml"},
		{Name: "hbs", Config: "eos_v2.hbs"},
	}
	return mappers.NewEnricherStream(source, enrichers, basePath)
}
