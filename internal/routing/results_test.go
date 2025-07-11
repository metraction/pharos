package routing

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/metraction/pharos/internal/integrations/redis"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/mappers"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/extension"
)

func TestIntegrationScannerFlow(t *testing.T) {
	// if os.Getenv("RUN_SCANNER_TESTS") != "true" {
	// 	t.Skip("skipping scanner test - make sure it runs on right image and then set RUN_SCANNER_TESTS=true")
	// }
	// Setup Redis (mini or real)
	redis, _, config := redis.SetupTestRedis(t)
	if redis != nil {
		defer redis.Close()
	}
	config.Scanner.CacheEndpoint = "redis://" + config.Redis.DSN
	config.Scanner.Timeout = "5s"

	scanTask := model.NewTestScanTask(t, "test-1", "nginx:latest")
	outChan := make(chan any, 1)
	outChan <- scanTask
	close(outChan)

	stream := extension.NewChanSource(outChan).
		Via(NewScannerFlow(context.Background(), config))

	result := (<-stream.Out()).(model.PharosScanResult)

	// Assert that the result contains the same scan result that was passed in
	if !reflect.DeepEqual(result.ScanTask.JobId, scanTask.JobId) {
		t.Errorf("Expected result.ScanTask.JobId to be %v, got %v", scanTask.JobId, result.ScanTask.JobId)
	}
}

func TestIntegrationScanResultCollectorFlow(t *testing.T) {
	logger := logging.NewLogger("info")

	// Setup Redis (mini or real)
	redis, _, config := redis.SetupTestRedis(t)
	if redis != nil {
		defer redis.Close()
	}
	config.Scanner.CacheEndpoint = "redis://" + config.Redis.DSN
	config.Scanner.Timeout = "5s"

	enricher := mappers.EnricherConfig{
		BasePath: filepath.Join("..", "..", "testdata", "enrichers"),
		Configs: []mappers.MapperConfig{
			{Name: "file", Config: "eos.yaml"},
			//	{Name: "debug", Config: "1"},
			{Name: "hbs", Config: "eos_v1.hbs"},
			//	{Name: "debug", Config: "2"},
		},
	}

	scanTask := model.NewTestScanTask(t, "test-1", "nginx:latest")
	outChan := make(chan any, 1)
	outChan <- scanTask
	close(outChan)

	source := extension.NewChanSource(outChan)
	stream := NewScanResultCollectorFlow(context.Background(), config, enricher, source, logger)

	result := (<-stream.Out()).(model.PharosScanResult)

	// Assert that the result contains the same scan result that was passed in
	if !reflect.DeepEqual(result.ScanTask.JobId, scanTask.JobId) {
		t.Errorf("Expected result.ScanTask.JobId to be %v, got %v", scanTask.JobId, result.ScanTask.JobId)
	}
}
