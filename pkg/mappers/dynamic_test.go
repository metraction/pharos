package mappers

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/metraction/pharos/pkg/model"
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
		Via(flow.NewMap(ToMap, 1))

	enrichersPath := filepath.Join("..", "..", "testdata", "enrichers")
	stream := NewEnricherStream(source, enrichers, enrichersPath)
	result := (<-stream.Out()).((map[string]interface{}))

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

func TestDynamicWrapperStream(t *testing.T) {
	enrichers := []EnricherConfig{
		{Name: "file", Config: "eos.yaml"},
		{Name: "hbs", Config: "pass_through.hbs"},
		//{Name: "debug", Config: ""},
	}

	scanResult := model.NewTestScanResult(model.NewTestScanTask(t, "test-1", "test-image-1"), "test-engine-1")
	outChan := make(chan any, 1)
	outChan <- WrappedResult{Result: scanResult, Context: ToMap(scanResult)}
	close(outChan)

	source := extension.NewChanSource(outChan)

	enrichersPath := filepath.Join("..", "..", "testdata", "enrichers")
	stream := NewResultEnricherStream(source, enrichers, enrichersPath)
	result := (<-stream.Out()).(WrappedResult)

	// Assert that the result contains the same scan result that was passed in
	if !reflect.DeepEqual(result.Result, scanResult) {
		t.Errorf("Expected result.Result to be %v, got %v", scanResult, result.Result)
	}

	// Assert that the result contains the expected structure
	data, ok := result.Context["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to contain 'data' as map[string]interface{}, got %T", result.Context["data"])
	}
	imageSpec, ok := data["Image"].(map[string]interface{})["ImageSpec"].(string)
	if !ok {
		t.Fatalf("Expected result to contain 'Image.ImageSpec' as map[string]interface{}, got %T", data["Image"])
	}

	if imageSpec != "test-image-1" {
		t.Errorf("Expected imageSpec to be 'test-image-1', got '%s'", imageSpec)
	}

}
