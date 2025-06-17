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

// TestMessage is a simple message type for testing
type TestMessage struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

// TestResponse is a simple response type for testing
type TestResponse struct {
	RequestID string `json:"request_id"`
	Result    string `json:"result"`
}

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

func TestNewRedisGtrsClient(t *testing.T) {
	// Setup Redis (mini or real)
	mr, redisCfg := setupRedisTest(t)
	if mr != nil {
		defer mr.Close()
	}

	// Test creating a new client
	ctx := context.Background()
	client, err := NewRedisGtrsClient[TestMessage, TestResponse](ctx, redisCfg, "test_requests", "test_responses")

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "test_requests", client.requestQueue)
	assert.Equal(t, "test_responses", client.replyQueue)

	// Test sending a request
	msg := TestMessage{
		ID:      uuid.New().String(),
		Content: "Test message",
	}

	err, corrID := client.SendRequest(ctx, msg)
	require.NoError(t, err)
	assert.NotEmpty(t, corrID)

}

func TestNewRedisGtrsServer(t *testing.T) {
	// Setup Redis (mini or real)
	mr, redisCfg := setupRedisTest(t)
	if mr != nil {
		defer mr.Close()
	}

	// Test creating a new server
	ctx := context.Background()
	server, err := NewRedisGtrsServer[TestMessage, TestResponse](ctx, redisCfg, "test_requests", "test_responses")

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.Equal(t, "test_requests", server.requestQueue)
	assert.Equal(t, "test_responses", server.replyQueue)

	// Create a mock handler function
	handlerCalled := false
	mockHandler := func(msg TestMessage) TestResponse {
		handlerCalled = true
		return TestResponse{
			RequestID: msg.ID,
			Result:    "Processed: " + msg.Content,
		}
	}

	// Start the server in a goroutine (it will run until context is cancelled)
	serverCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		server.ProcessRequest(serverCtx, server.rdb, mockHandler)
	}()

	// Create a client to send a request
	client, err := NewRedisGtrsClient[TestMessage, TestResponse](ctx, redisCfg, "test_requests", "test_responses")
	require.NoError(t, err)

	// Send a test message
	msg := TestMessage{
		ID:      uuid.New().String(),
		Content: "Test request",
	}

	err, corrID := client.SendRequest(ctx, msg)
	require.NoError(t, err)

	fmt.Println("Sent request with correlation ID:", corrID)

	// Give some time for processing
	time.Sleep(100 * time.Millisecond)

	// Cancel the server context to stop processing
	cancel()

	// Check if the handler was called
	assert.True(t, handlerCalled, "The mock handler should have been called")
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

	ctx := context.Background()

	// Create server
	server, err := NewRedisGtrsServer[TestMessage, TestResponse](ctx, redisCfg, "test_requests", "test_responses")
	require.NoError(t, err)

	// Create a mock handler function
	mockHandler := func(msg TestMessage) TestResponse {
		// Simulate some processing time to test concurrency
		time.Sleep(50 * time.Millisecond)
		return TestResponse{
			RequestID: msg.ID,
			Result:    "Processed: " + msg.Content,
		}
	}

	// Start the server in a goroutine
	serverCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		server.ProcessRequest(serverCtx, server.rdb, mockHandler)
	}()

	// Create client
	client, err := NewRedisGtrsClient[TestMessage, TestResponse](ctx, redisCfg, "test_requests", "test_responses")
	require.NoError(t, err)

	// Number of concurrent requests to send
	numRequests := 5

	// Create a wait group to wait for all goroutines to complete
	wg := sync.WaitGroup{}
	wg.Add(numRequests)

	// Create a mutex to protect access to the results map
	resultsMutex := sync.Mutex{}
	results := make(map[string]TestResponse)
	errors := make([]error, 0)

	// Send multiple requests in parallel
	for i := 0; i < numRequests; i++ {
		go func(index int) {
			defer wg.Done()

			// Create a unique message
			msgID := uuid.New().String()
			msg := TestMessage{
				ID:      msgID,
				Content: fmt.Sprintf("Test parallel request %d", index),
			}

			// Send the request
			err, corrID := client.SendRequest(ctx, msg)
			if err != nil {
				resultsMutex.Lock()
				errors = append(errors, fmt.Errorf("request %d failed: %w", index, err))
				resultsMutex.Unlock()
				return
			}

			// Wait for response with timeout
			response, err := client.ReceiveResponse(ctx, corrID, 5*time.Second)
			if err != nil {
				resultsMutex.Lock()
				errors = append(errors, fmt.Errorf("response %d failed: %w", index, err))
				resultsMutex.Unlock()
				return
			}

			// Store the result
			resultsMutex.Lock()
			results[msgID] = response
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
	for msgID, response := range results {
		assert.Equal(t, msgID, response.RequestID, "Response ID should match request ID")
		assert.Contains(t, response.Result, "Processed: Test parallel request", "Response should contain expected content")
	}
}
