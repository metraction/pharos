package mappers

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/extension"
)

func TestOwnersMapper(t *testing.T) {
	enricher := EnricherConfig{
		BasePath: filepath.Join("..", "..", "testdata", "enrichers"),
		Configs: []MapperConfig{
			{Name: "file", Config: "owners.yaml"},
			{Name: "hbs", Config: "owners_v1.hbs"},
			//	{Name: "debug", Config: ""},
		},
	}

	scanResult := model.NewTestScanResult(model.NewTestScanTask(t, "test-1", "test-image-1"), "test-engine-1")
	outChan := make(chan any, 1)
	outChan <- scanResult
	close(outChan)

	source := extension.NewChanSource(outChan)
	stream := NewResultEnricherStream(source, enricher)
	result := (<-stream.Out()).(model.PharosScanResult)

	fmt.Println(result)
}
