package routing

import (
	"path/filepath"

	"github.com/metraction/policy-engine/pkg/enricher"
	"github.com/reugn/go-streams"
)

func NewEosEnricher(source streams.Source) streams.Source {
	enrichers := []enricher.EnricherConfig{
		{Name: "file", Config: "eos.yaml"},
		{Name: "hbs", Config: "eos_v2.hbs"},
		{Name: "debug", Config: ""},
	}
	return enricher.NewEnricherStream(source, enrichers, filepath.Join("..", "..", "testdata"))
}
