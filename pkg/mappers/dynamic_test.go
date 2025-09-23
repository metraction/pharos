package mappers

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/extension"
)

func TestDynamicWrapperStream(t *testing.T) {
	enricher := model.EnricherConfig{
		BasePath: filepath.Join("..", "..", "testdata", "enrichers"),
		Configs: []model.MapperConfig{
			{Name: "file", Config: "eos/eos.yaml"},
			// {Name: "debug", Config: ""},
			{Name: "hbs", Config: "pass_through.hbs"},
		},
	}

	scanResult := model.NewTestScanResult(model.NewTestScanTask(t, "test-1", "test-image-1"), "test-engine-1")
	outChan := make(chan any, 1)
	outChan <- scanResult
	close(outChan)

	source := extension.NewChanSource(outChan)
	//stream := NewResultEnricherStream(source, "eos-passthrough", enricher)
	stream := source.Via(NewEnricherMap("eos-passthrough", enricher, nil))
	result := (<-stream.Out()).(model.PharosScanResult)

	// Assert that the result contains the same scan result that was passed in
	if !reflect.DeepEqual(result.ScanTask.JobId, scanResult.ScanTask.JobId) {
		t.Errorf("Expected result.ScanTask.JobId to be %v, got %v", scanResult.ScanTask.JobId, result.ScanTask.JobId)
	}

	// Assert that the result contains the expected structure
	data, ok := result.Image.ContextRoots[0].Contexts[0].Data["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result.ScanTask.Context to contain 'payload' as map[string]interface{}, got %T", result.ScanTask.Context["data"])
	}
	imageSpec, ok := data["Image"].(map[string]interface{})["ImageSpec"].(string)
	if !ok {
		t.Fatalf("Expected result.ScanTask.Context to contain 'Image.ImageSpec' as map[string]interface{}, got %T", data["Image"])
	}

	if imageSpec != "test-image-1" {
		t.Errorf("Expected imageSpec to be 'test-image-1', got '%s'", imageSpec)
	}

}
