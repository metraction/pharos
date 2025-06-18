package integrations

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/metraction/pharos/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// We'll use the actual model types for testing
// PharosScanTask and PharosScanResult are defined in pkg/model

func setupRedisTest(t *testing.T) (*miniredis.Miniredis, model.Redis) {
	// Check if REDIS_DSN environment variable is set
	redisDSN := os.Getenv("REDIS_DSN")
	if redisDSN != "" {
		// Use real Redis instance
		t.Logf("Using real Redis instance at %s", redisDSN)
		return nil, model.Redis{
			DSN: redisDSN,
		}
	}

	// Start a mini Redis server for testing
	t.Log("Using miniredis for testing")
	mr, err := miniredis.Run()
	require.NoError(t, err)

	// Create Redis config that points to the mini Redis
	redisCfg := model.Redis{
		DSN: mr.Addr(),
	}

	return mr, redisCfg
}

func TestIntegrationClientServer(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup Redis (mini or real)
	mr, redisCfg := setupRedisTest(t)
	if mr != nil {
		defer mr.Close()
	}
	config := &model.Config{
		Redis: redisCfg,
		Publisher: model.PublisherConfig{
			Timeout: "5s",
		},
	}

	ctx := context.Background()

	// Create server
	server, err := NewRedisGtrsServer[model.PharosScanTask, model.PharosScanResult](ctx, redisCfg, "test_requests", "test_responses")
	require.NoError(t, err)

	// Create a mock handler function
	mockHandler := func(task model.PharosScanTask) model.PharosScanResult {
		// Simulate some processing time to test concurrency
		time.Sleep(50 * time.Millisecond)

		// Create a simple scan result
		return model.PharosScanResult{
			Version:  "1.0",
			ScanTask: task,
			ScanEngine: model.PharosScanEngine{
				Name:    "test-engine",
				Version: "1.0",
			},
			Image: model.PharosImageMeta{
				ImageSpec: task.ImageSpec.Image,
				ImageId:   "test-image-id",
				Digest:    "sha256:test",
			},
			Findings:        []model.PharosScanFinding{},
			Vulnerabilities: []model.PharosVulnerability{},
			Packages:        []model.PharosPackage{},
		}
	}

	// Start the server in a goroutine
	serverCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		server.ProcessRequest(serverCtx, mockHandler)
	}()

	// Create client
	client, err := NewRedisGtrsClient[model.PharosScanTask, model.PharosScanResult](ctx, config, "test_requests", "test_responses")
	require.NoError(t, err)

	// Number of concurrent requests to send
	numRequests := 5

	// Create a wait group to wait for all goroutines to complete
	wg := sync.WaitGroup{}
	wg.Add(numRequests)

	// Create a mutex to protect access to the results map
	resultsMutex := sync.Mutex{}
	results := make(map[string]model.PharosScanResult)
	errors := make([]error, 0)

	// Send multiple requests in parallel
	for i := 0; i < numRequests; i++ {
		go func(index int) {
			defer wg.Done()

			// Create a unique task ID
			taskID := uuid.New().String()

			// Create a scan task
			task, err := model.NewPharosScanTask(
				taskID,
				fmt.Sprintf("test-image-%d", index),
				"",                     // platform
				model.PharosRepoAuth{}, // auth
				1*time.Hour,            // cache expiry
				30*time.Second,         // scan timeout
			)
			if err != nil {
				resultsMutex.Lock()
				errors = append(errors, fmt.Errorf("failed to create task %d: %w", index, err))
				resultsMutex.Unlock()
				return
			}

			// Send the request
			response, err := client.RequestReply(ctx, task)
			if err != nil {
				resultsMutex.Lock()
				errors = append(errors, fmt.Errorf("request %d failed: %w", index, err))
				resultsMutex.Unlock()
				return
			}

			// Store the result
			resultsMutex.Lock()
			results[taskID] = response
			resultsMutex.Unlock()
		}(i)
	}

	// Wait for all requests to complete
	wg.Wait()

	// Check for errors
	require.Empty(t, errors, "Errors occurred during parallel requests")

	// Verify we got all responses
	assert.Equal(t, numRequests, len(results), "Should receive exactly %d responses", numRequests)

	// Verify each response matches its request
	for taskID, response := range results {
		assert.Equal(t, taskID, response.ScanTask.JobId, "Response task ID should match request ID")
		assert.Equal(t, "test-engine", response.ScanEngine.Name, "Response should contain expected engine name")
	}
}
