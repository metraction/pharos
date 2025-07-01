package mappers

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
)

func TestDynamicStream(t *testing.T) {
	enrichers := []EnricherConfig{
		{Name: "file", Config: "eos.yaml"},
		{Name: "hbs", Config: "eos_v1.hbs"},
		//	{Name: "debug", Config: ""},
	}

	img := Image{
		Vulnerabilities: []Vulnerability{
			{L1: 5},
			{L1: 7},
		},
		Namespace: "portal",
		Distro:    "alpine",
		Version:   "3.16",
	}

	outChan := make(chan any)
	go func() {
		outChan <- img
		close(outChan)
	}()
	source := extension.NewChanSource(outChan).
		Via(flow.NewMap(NewMapOfMaps(), 1))

	enrichersPath := filepath.Join("..", "..", "testdata", "enrichers")
	stream := NewEnricherStream(source, enrichers, enrichersPath)
	result := (<-stream.Out()).(map[string]interface{})

	// Assert that the result contains the expected structure
	spec, ok := result["spec"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to contain 'spec' as map[string]interface{}, got %T", result["spec"])
	}

	// Assert that spec contains the eos field
	eos, ok := spec["eos"]
	if !ok {
		t.Fatalf("Expected spec to contain 'eos' field, got keys: %v", getMapKeys(spec))
	}

	// Check if eos contains the expected date
	eosStr := fmt.Sprintf("%v", eos)

	// Check if the date string contains the expected date format
	if !strings.Contains(eosStr, "2024-05-23") {
		t.Errorf("Expected eos to contain '2024-05-23', got '%s'", eosStr)
	}
}
