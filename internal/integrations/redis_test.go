package integrations

import (
	"context"
	"fmt"
	"os"
	"strings"
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

func setupRedisTest(t *testing.T) (*miniredis.Miniredis, *model.Config) {
	// Check if REDIS_DSN environment variable is set
	redisDSN := os.Getenv("REDIS_DSN")
	if redisDSN != "" {
		// Use real Redis instance
		t.Logf("Using real Redis instance at %s", redisDSN)
		config := &model.Config{
			Redis:     model.Redis{DSN: redisDSN},
			Publisher: model.PublisherConfig{Timeout: "5s"},
		}
		return nil, config
	}

	// Start a mini Redis server for testing
	t.Log("Using miniredis for testing")
	mr, err := miniredis.Run()
	require.NoError(t, err)

	// Create Redis config that points to the mini Redis
	redisCfg := model.Redis{
		DSN: mr.Addr(),
	}

	config := &model.Config{
		Redis:     redisCfg,
		Publisher: model.PublisherConfig{Timeout: "5s"},
	}
	return mr, config
}

// newTestScanTask is a test helper that creates a PharosScanTask with standard defaults.
func newTestScanTask(t *testing.T, taskID, image string) model.PharosScanTask {
	t.Helper()
	task, err := model.NewPharosScanTask(
		taskID,
		image,
		"",                     // platform
		model.PharosRepoAuth{}, // auth
		1*time.Hour,            // cache expiry
		30*time.Second,         // scan timeout
	)
	require.NoError(t, err)
	return task
}

// newTestScanResult is a test helper that creates a PharosScanResult for a given task and engine name.
func newTestScanResult(task model.PharosScanTask, engineName string) model.PharosScanResult {
	return model.PharosScanResult{
		Version:  "1.0",
		ScanTask: task,
		ScanEngine: model.PharosScanEngine{
			Name:    engineName,
			Version: "1.0",
		},
		Image: model.PharosImageMeta{
			ImageSpec:   task.ImageSpec.Image,
			ImageId:     "test-image-id",
			IndexDigest: "sha256:test",
		},
		Findings:        []model.PharosScanFinding{},
		Vulnerabilities: []model.PharosVulnerability{},
		Packages:        []model.PharosPackage{},
	}
}

func TestIntegrationClientServer(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup Redis (mini or real)
	mr, config := setupRedisTest(t)
	if mr != nil {
		defer mr.Close()
	}

	ctx := context.Background()

	// Create server
	server, err := NewRedisGtrsServer[model.PharosScanTask, model.PharosScanResult](ctx, config.Redis, "test_requests", "test_responses")
	require.NoError(t, err)

	// Create a mock handler function
	mockHandler := func(task model.PharosScanTask) model.PharosScanResult {
		// Simulate some processing time to test concurrency
		time.Sleep(50 * time.Millisecond)

		// Create a simple scan result
		return newTestScanResult(task, "test-engine")
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
			task := newTestScanTask(t, taskID, fmt.Sprintf("test-image-%d", index))

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

/*
Object under test is RedisGtrsServer to prove that each worker consumes one and only one message

	+-----------+                      +-----------+         +---------+
	|           |  T1, T2, T3, T4, T5  |           |   T1    |         |
	|  Client   |--------------------->|           |-------->| Worker 1|
	|           |          			   |           |         |         |
	+-----------+          			   |           |         +---------+
	                   			       |           |
	                       			   |           |         +---------+
	                     			   |           |   T3    |         |
	                       			   |		   |-------->| Worker 2|
	                      			   |           |         |         |
	                      			   |           |         +---------+
	                      			   |           |
	                      			   |           |         +---------+
	                      			   |           |   T2    |         |
	                      			   |           |-------->| Worker 3|
	                       			   |           |         |         |
	        	  			           +-----------+         +---------+
*/
func TestMessageConsumedOnlyOnce(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup Redis (mini or real)
	mr, config := setupRedisTest(t)
	if mr != nil {
		defer mr.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a unique request/response queue name for this test to avoid interference
	testID := strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
	requestQueue := fmt.Sprintf("test_requests_%s", testID)
	responseQueue := fmt.Sprintf("test_responses_%s", testID)

	// Create a mutex to protect access to the counters
	mutex := sync.Mutex{}
	processedCount := make(map[string]int)

	// Create multiple server instances (consumers) with the same group name but different consumer names
	const numServers = 3
	servers := make([]*RedisGtrsServer[model.PharosScanTask, model.PharosScanResult], numServers)

	// Use a wait group to ensure all consumers are ready before sending the request
	var wg sync.WaitGroup
	wg.Add(numServers)

	// Create servers and start processing
	for i := 0; i < numServers; i++ {
		serverIdx := i // Capture loop variable
		server, err := NewRedisGtrsServer[model.PharosScanTask, model.PharosScanResult](ctx, config.Redis, requestQueue, responseQueue)
		require.NoError(t, err)
		servers[i] = server

		// Create a unique handler for each server that tracks message processing
		handler := func(task model.PharosScanTask) model.PharosScanResult {
			// Record that this server processed the message
			mutex.Lock()
			processedCount[task.JobId] = processedCount[task.JobId] + 1
			serverName := fmt.Sprintf("server-%d", serverIdx)
			t.Logf("Task %s processed by %s", task.JobId, serverName)
			mutex.Unlock()

			// Simulate some processing time
			time.Sleep(50 * time.Millisecond)

			return newTestScanResult(task, fmt.Sprintf("test-engine-%d", serverIdx))
		}

		// Start each server in its own goroutine
		go func() {
			wg.Done() // Signal that this server is ready
			server.ProcessRequest(ctx, handler)
		}()
	}

	// Wait for all servers to be ready
	wg.Wait()

	// Wait a bit to ensure all consumers have properly joined the group
	time.Sleep(500 * time.Millisecond)

	// Create client
	client, err := NewRedisGtrsClient[model.PharosScanTask, model.PharosScanResult](ctx, config, requestQueue, responseQueue)
	require.NoError(t, err)

	// Send multiple requests
	const numRequestsToSend = 5
	results := make(map[string]model.PharosScanResult, numRequestsToSend)
	for i := 0; i < numRequestsToSend; i++ {
		taskID := fmt.Sprintf("%d", i)
		task := newTestScanTask(t, taskID, fmt.Sprintf("test-image-%d", i))
		resp, err := client.RequestReply(ctx, task)
		require.NoError(t, err)
		results[taskID] = resp
	}

	// Allow some time for all consumers to potentially process the messages
	time.Sleep(1 * time.Second)

	// Verify each response is correct
	for taskID, resp := range results {
		assert.Equal(t, taskID, resp.ScanTask.JobId)
	}

	// Check that exactly one server processed each message
	mutex.Lock()
	for taskID := range results {
		assert.Equal(t, 1, processedCount[taskID], "Message should be processed exactly once")
	}
	mutex.Unlock()
}
