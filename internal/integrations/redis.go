package integrations

import (
	"context"
	"fmt"
	"log/slog" // Added for slog.Default()
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/metraction/pharos/pkg/model"
	"github.com/redis/go-redis/v9"
	"github.com/reugn/go-streams"
	rg_redis "github.com/reugn/go-streams/redis"
)

// NewRedisStreamSource creates a new Redis client and returns a streams.Source
// that emits messages from the specified Redis Stream using a consumer group.
// The context.Context can be used to cancel the operation.
// streamName: The name of the Redis stream (e.g., "mystream").
// groupName: The name of the consumer group.
// consumerName: A unique name for this consumer instance within the group.
// groupStartID: Where the group should start reading if it's created for the first time (e.g., "0" or "$").
// blockTimeout: How long to block for new messages (e.g., 1 * time.Second). Use 0 for non-blocking.
// messageCount: Number of messages to fetch per XREADGROUP command.
func NewRedisStreamSource(ctx context.Context, redisCfg model.Redis, streamName string, groupName string, consumerName string, groupStartID string, blockTimeout time.Duration, messageCount int64) (streams.Source, error) {
	// 1. Create Redis client (using go-redis/redis v6)
	redisAddr := fmt.Sprintf("%s:%d", redisCfg.Host, redisCfg.Port)
	fmt.Println("Connecting to Redis at:", redisAddr)
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		// Other v6 options if needed (e.g., Password, DB)
	})

	// Ping the Redis server to ensure connectivity (context-aware ping for v6)
	if err := rdb.Ping(ctx).Err(); err != nil { // v6 Ping doesn't take context directly, but underlying operations are ctx aware via client
		rdb.Close()
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", redisAddr, err)
	}

	// 2. Create Consumer Group (if it doesn't exist) using v6 client
	// XGroupCreateMkStream creates the stream if it doesn't exist, then the group.
	// If the group already exists, Redis returns a BUSYGROUP error, which we can ignore.
	// For v6, XGroupCreateMkStream is called directly on the client.
	cmd := rdb.XGroupCreateMkStream(ctx, streamName, groupName, groupStartID)
	if err := cmd.Err(); err != nil && !strings.HasPrefix(err.Error(), "BUSYGROUP") {
		rdb.Close()
		return nil, fmt.Errorf("failed to create consumer group '%s' for stream '%s': %w", groupName, streamName, err)
	}

	// 3. Define XReadGroupArgs for reading from the stream as part of a consumer group (using v6 types)
	xReadGroupArgs := &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: consumerName,
		Streams:  []string{streamName, ">"}, // ">" means only new messages not yet delivered to other consumers
		Count:    messageCount,              // Number of messages to fetch per command
		Block:    blockTimeout,
		NoAck:    true, // Default is false, meaning messages need to be acknowledged (XACK)
	}

	// 4. Create and return the Redis Stream source from go-streams/redis.
	// This source will use XREADGROUP to consume messages.
	// The NewRedisSource function handles closing the Redis client (rdb) when the context is done.
	// It expects a redis.UniversalClient from the v6 library.
	redisSource, err := rg_redis.NewStreamSource(ctx, rdb, xReadGroupArgs, nil, slog.Default())
	if err != nil {
		// rdb.Close() is typically handled by NewRedisSource's cleanup on context done or internal error.
		return nil, fmt.Errorf("failed to create Redis Stream consumer group source for stream '%s', group '%s': %w", streamName, groupName, err)
	}

	return redisSource, nil
}

// NewRedisStreamSink creates a new Redis client and returns a streams.Sink
// that publishes messages to the specified Redis Stream.
// The context.Context can be used for cancellation during sink setup.
// streamName: The name of the Redis stream to publish to (e.g., "images_to_scan").
func NewRedisStreamSink(ctx context.Context, redisCfg model.Redis, streamName string) (streams.Sink, error) {
	redisAddr := fmt.Sprintf("%s:%d", redisCfg.Host, redisCfg.Port)
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("failed to connect to Redis at %s for sink: %w", redisAddr, err)
	}

	// The rg_redis.NewStreamSink expects a *redis.Client from go-redis/v9,
	// the target stream name, and an *slog.Logger.
	// The sink itself handles closing the Redis client when its processing is done or context is cancelled.
	streamSink := rg_redis.NewStreamSink(ctx, rdb, streamName, slog.Default())

	return streamSink, nil
}

func NewRedisRemoteMap() {

}

type RedisGtrsClient struct {
}

func sendRequest(ctx context.Context, client *redis.Client, payload map[string]interface{}, streamName string) (error, string) {
	corrID := uuid.New().String()
	err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"corr_id": corrID,
			"payload": payload,
		},
	}).Err()
	return err, corrID
}
