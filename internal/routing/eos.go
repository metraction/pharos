package routing

import (
	"github.com/metraction/policy-engine/pkg/enricher"
	"github.com/reugn/go-streams"
)

func NewEosEnricher(source streams.Source, basePath string) streams.Source {
	enrichers := []enricher.EnricherConfig{
		{Name: "file", Config: "eos.yaml"},
		{Name: "hbs", Config: "eos_v2.hbs"},
	}
	return enricher.NewEnricherStream(source, enrichers, basePath)
}
