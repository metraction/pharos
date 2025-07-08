package redis

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
	"github.com/redis/go-redis/v9"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// We'll use the actual model types for testing
// PharosScanTask and PharosScanResult are defined in pkg/model

func setupRedisTest(t *testing.T) (*miniredis.Miniredis, *redis.Client, *model.Config) {
	// Check if REDIS_DSN environment variable is set
	redisDSN := os.Getenv("REDIS_DSN")
	if redisDSN != "" {
		// Use real Redis instance
		t.Logf("Using real Redis instance at %s", redisDSN)
		config := &model.Config{
			Redis:     model.Redis{DSN: redisDSN},
			Publisher: model.PublisherConfig{Timeout: "5s"},
		}

		// Create Redis client
		rdb := redis.NewClient(&redis.Options{
			Addr: redisDSN,
		})

		// Test the connection
		err := rdb.Ping(context.Background()).Err()
		require.NoError(t, err, "Failed to connect to Redis")

		return nil, rdb, config
	}

	// Start a mini Redis server for testing
	t.Log("Using miniredis for testing")
	mr, err := miniredis.Run()
	require.NoError(t, err)

	// Create Redis config that points to the mini Redis
	redisCfg := model.Redis{
		DSN: mr.Addr(),
	}

	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	config := &model.Config{
		Redis:     redisCfg,
		Publisher: model.PublisherConfig{Timeout: "5s"},
	}
	return mr, rdb, config
}

// newTestScanTask is a test helper that creates a PharosScanTask with standard defaults.
func newTestScanTask(t *testing.T, taskID, image string) model.PharosScanTask2 {
	t.Helper()
	task := model.PharosScanTask2{JobId: taskID, ImageSpec: image, ScanTTL: 30 * time.Second, CacheTTL: 1 * time.Hour}
	return task
}

// newTestScanResult is a test helper that creates a PharosScanResult for a given task and engine name.
func newTestScanResult(task model.PharosScanTask2, engineName string) model.PharosScanResult {
	task.Engine = engineName
	task.Status = "done"
	return model.PharosScanResult{
		Version:  "1.0",
		ScanTask: task,
		Image: model.PharosImageMeta{
			ImageSpec:   task.ImageSpec,
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
	mr, _, config := setupRedisTest(t)
	if mr != nil {
		defer mr.Close()
	}

	ctx := context.Background()

	// Create server
	server, err := NewRedisGtrsServer[model.PharosScanTask2, model.PharosScanResult](ctx, config.Redis, "test_requests", "test_responses")
	require.NoError(t, err)

	// Create a mock handler function
	mockHandler := func(task model.PharosScanTask2) model.PharosScanResult {
		// Simulate some processing time to test concurrency
		time.Sleep(50 * time.Millisecond)

		// Create a simple scan result
		return model.NewTestScanResult(task, "test-engine")
	}

	// Start the server in a goroutine
	serverCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		server.ProcessRequest(serverCtx, mockHandler)
	}()

	// Create client
	client, err := NewRedisGtrsClient[model.PharosScanTask2, model.PharosScanResult](ctx, config, "test_requests", "test_responses")
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
			task := model.NewTestScanTask(t, taskID, fmt.Sprintf("test-image-%d", index))

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
		assert.Equal(t, "test-engine", response.ScanTask.Engine, "Response should contain expected engine name")
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
	mr, _, config := setupRedisTest(t)
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
	servers := make([]*RedisGtrsServer[model.PharosScanTask2, model.PharosScanResult], numServers)

	// Use a wait group to ensure all consumers are ready before sending the request
	var wg sync.WaitGroup
	wg.Add(numServers)

	// Create servers and start processing
	for i := 0; i < numServers; i++ {
		serverIdx := i // Capture loop variable
		server, err := NewRedisGtrsServer[model.PharosScanTask2, model.PharosScanResult](ctx, config.Redis, requestQueue, responseQueue)
		require.NoError(t, err)
		servers[i] = server

		// Create a unique handler for each server that tracks message processing
		handler := func(task model.PharosScanTask2) model.PharosScanResult {
			// Record that this server processed the message
			mutex.Lock()
			processedCount[task.JobId] = processedCount[task.JobId] + 1
			serverName := fmt.Sprintf("server-%d", serverIdx)
			t.Logf("Task %s processed by %s", task.JobId, serverName)
			mutex.Unlock()

			// Simulate some processing time
			time.Sleep(50 * time.Millisecond)

			return model.NewTestScanResult(task, fmt.Sprintf("test-engine-%d", serverIdx))
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
	client, err := NewRedisGtrsClient[model.PharosScanTask2, model.PharosScanResult](ctx, config, requestQueue, responseQueue)
	require.NoError(t, err)

	// Send multiple requests
	const numRequestsToSend = 5
	results := make(map[string]model.PharosScanResult, numRequestsToSend)
	for i := 0; i < numRequestsToSend; i++ {
		taskID := fmt.Sprintf("%d", i)
		task := model.NewTestScanTask(t, taskID, fmt.Sprintf("test-image-%d", i))
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

func TestRedisConsumerGroupSource(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup Redis (mini or real)
	mr, rdb, _ := setupRedisTest(t)
	if mr != nil {
		defer mr.Close()
	}

	// Create a context with timeout for the test
	testCtx, testCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer testCancel()

	// Create a unique stream name for this test
	testID := strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
	streamName := fmt.Sprintf("test_stream_%s", testID)
	groupName := "test_group"

	// Number of consumers and messages
	const numConsumers = 3
	const numMessages = 6 // Should be at least 2x numConsumers to test distribution

	// Create a wait group to synchronize test completion
	wg := sync.WaitGroup{}
	wg.Add(numMessages) // We expect each message to be processed exactly once

	// Track which consumer processed which message
	mutex := sync.Mutex{}
	processedMessages := make(map[string]string) // message ID -> consumer name
	messageCount := make(map[string]int)         // message ID -> count of times processed
	consumerStats := make(map[string]int)        // consumer name -> count of messages processed

	// Step 1: Create a Redis stream sink to publish messages
	streamSink := NewRedisStreamSink[model.PharosScanTask2](testCtx, rdb, streamName)

	// Step 2: Create a channel source to feed messages to the sink
	messageChan := make(chan any, numMessages)
	chanSource := extension.NewChanSource(messageChan)

	// Step 3: Connect the source to the sink
	go chanSource.
		Via(flow.NewMap(func(msg any) any {
			//fmt.Println("Sending message:", msg)
			return msg
		}, 1)).
		To(streamSink)

	// Step 4: Create multiple consumer sources with the same group but different consumer names
	//fmt.Println("Creating consumer sources...")
	consumers := make([]streams.Source, numConsumers)
	for i := 0; i < numConsumers; i++ {
		consumerName := fmt.Sprintf("consumer-%d", i)

		// Create a consumer group source with separate Redis context
		source := NewRedisConsumerGroupSource[model.PharosScanTask2](
			testCtx,
			rdb,
			streamName,
			groupName,
			consumerName,
			"0",                  // Start from beginning
			100*time.Millisecond, // Block timeout
			1,                    // Process one message at a time
		)
		consumers[i] = source

		// Start a goroutine to process messages from this consumer
		func(consumerIdx int, src streams.Source) {
			consumerID := fmt.Sprintf("consumer-%d", consumerIdx)
			t.Logf("Started consumer: %s", consumerID)
			// Process messages until context is done
			go func() {
				src.
					Via(flow.NewMap(func(msg any) any {
						scanTask := msg.(model.PharosScanTask2)
						t.Logf("Consumer %s processing task %s for image %s", consumerID, scanTask.JobId, scanTask.ImageSpec)

						// Record that this consumer processed this message
						mutex.Lock()
						processedMessages[scanTask.JobId] = consumerID
						messageCount[scanTask.JobId]++
						consumerStats[consumerID]++
						mutex.Unlock()

						// Create a scan result from the task
						result := model.NewTestScanResult(scanTask, consumerID)

						// Mark message as processed
						wg.Done()
						return result
					}, 1)).
					To(extension.NewStdoutSink())
			}()
		}(i, consumers[i])
	}

	// Step 5: Send messages to the channel source
	for i := 0; i < numMessages; i++ {
		// Create a PharosScanTask with a unique identifier
		taskID := fmt.Sprintf("task-%d", i)
		imageRef := fmt.Sprintf("test-image:%d", i)

		scanTask := model.NewTestScanTask(t, taskID, imageRef)
		// Send the task to the channel
		messageChan <- scanTask
		t.Logf("Published scan task: %s for image %s", taskID, imageRef)

		// Small delay to ensure messages are processed in order
		time.Sleep(50 * time.Millisecond)
	}

	// Close the channel to signal no more messages
	close(messageChan)

	// Wait for all messages to be processed or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("All messages processed successfully")
	case <-time.After(10 * time.Second):
		// If we time out, log the current state for debugging
		mutex.Lock()
		t.Logf("Timeout: Processed %d/%d messages", len(processedMessages), numMessages)
		for consumer, count := range consumerStats {
			t.Logf("Consumer %s processed %d messages", consumer, count)
		}
		mutex.Unlock()
		t.Fatal("Timeout waiting for messages to be processed")
	}

	// Give a short time for consumers to shut down gracefully
	time.Sleep(200 * time.Millisecond)

	// Verify each message was processed exactly once
	mutex.Lock()
	defer mutex.Unlock()

	t.Log("Message processing statistics:")
	for msgID, count := range messageCount {
		consumerID := processedMessages[msgID]
		t.Logf("Message %s processed by %s", msgID, consumerID)
		assert.Equal(t, 1, count, "Message %s should be processed exactly once", msgID)
	}

	t.Log("Consumer statistics:")
	for consumer, count := range consumerStats {
		t.Logf("%s processed %d messages", consumer, count)
	}

	// Verify we processed all messages
	assert.Equal(t, numMessages, len(processedMessages), "Should have processed all messages")
}

func TestRedisQueueLimit(t *testing.T) {
	limit := 5

	// Create redis for test
	mr, rdb, _ := setupRedisTest(t)
	if mr != nil {
		defer mr.Close()
	}

	ctx := context.Background()
	queueName := "test_queue_" + uuid.New().String()[:8]
	wg := sync.WaitGroup{}
	wg.Add(limit) // We expect each message to be processed exactly once

	rejectedCounter := func(in any) {
		fmt.Println("Rejected message:", in.(model.PharosScanTask2).JobId)
		wg.Done()
	}

	// Setup steams to write to redis
	messageChan := make(chan any)
	go extension.NewChanSource(messageChan).
		Via(flow.NewFilter(NewQueueLimit(ctx, rdb, queueName, int64(limit), rejectedCounter), 1)).
		To(NewRedisStreamSink[model.PharosScanTask2](ctx, rdb, queueName))

	// Send messages to the channel
	for i := 0; i < 10; i++ {
		taskID := fmt.Sprintf("%d", i)
		task := model.NewTestScanTask(t, taskID, fmt.Sprintf("test-image-%d", i))
		fmt.Println("Sending message:", task.JobId)
		time.Sleep(50 * time.Millisecond)
		messageChan <- task
	}
	// Close the channel to signal no more messages

	close(messageChan)

	queueLen := rdb.XLen(ctx, queueName).Val()
	assert.Equal(t, int64(limit), queueLen)

	go NewRedisConsumerGroupSource[model.PharosScanResult](ctx, rdb, queueName, "g1", "c1", "0", 100*time.Millisecond, 1).
		Via(flow.NewMap(func(msg any) any {
			return msg
		}, 1)).
		To(extension.NewStdoutSink())

	wg.Wait()

}
