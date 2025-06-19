package integrations

import (
	"context"
	"fmt"
	"strings"
	"time"

	"log/slog" // Added for slog.Default()

	"github.com/dranikpg/gtrs"
	"github.com/metraction/pharos/pkg/model"
	"github.com/redis/go-redis/v9"
	"github.com/reugn/go-streams"
	rg_redis "github.com/reugn/go-streams/redis"
)

func NewRedisStreamSource(ctx context.Context, redisCfg model.Redis, streamName string, groupName string, consumerName, groupStartID string, blockTimeout time.Duration, messageCount int64) (streams.Source, error) {
	// 1. Create Redis client (using go-redis/redis v6)
	fmt.Println("Connecting to Redis at:", redisCfg.DSN)
	rdb := redis.NewClient(&redis.Options{
		Addr: redisCfg.DSN,
		// Other v6 options if needed (e.g., Password, DB)
	})

	// Ping the Redis server to ensure connectivity (context-aware ping for v6)
	if err := rdb.Ping(ctx).Err(); err != nil { // v6 Ping doesn't take context directly, but underlying operations are ctx aware via client
		rdb.Close()
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", redisCfg.DSN, err)
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
	rdb := redis.NewClient(&redis.Options{
		Addr: redisCfg.DSN,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("failed to connect to Redis at %s for sink: %w", redisCfg.DSN, err)
	}

	// The rg_redis.NewStreamSink expects a *redis.Client from go-redis/v9,
	// the target stream name, and an *slog.Logger.
	// The sink itself handles closing the Redis client when its processing is done or context is cancelled.
	streamSink := rg_redis.NewStreamSink(ctx, rdb, streamName, slog.Default())

	return streamSink, nil
}

type RedisGtrsClient[T any, R any] struct {
	rdb           *redis.Client
	requestStream *gtrs.Stream[T]
	requestQueue  string
	replyQueue    string
	streamName    string // NEW Stefan
	zeroValue     R
	timeout       time.Duration
}

// NEW: Stefan
func NewRedisGtrsClientStefan[T any, R any](ctx context.Context, redisEndpoint, streamName string) (*RedisGtrsClient[T, R], error) {

	options, err := redis.ParseURL(redisEndpoint)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(options)

	timeout := 60 * time.Second
	stream := gtrs.NewStream[T](rdb, streamName, nil)

	return &RedisGtrsClient[T, R]{
		rdb:           rdb,
		requestStream: &stream,
		requestQueue:  "none",
		streamName:    streamName,
		replyQueue:    "none",
		timeout:       timeout}, nil
}

// NEW: Stefan
func (rx *RedisGtrsClient[T, R]) Connect(ctx context.Context) error {
	if err := rx.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis connect (ping): %v", err)
	}
	return nil
}

// NEW: Stefan
func (rx *RedisGtrsClient[T, R]) Close() {
	if rx.rdb != nil {
		rx.rdb.Close()
	}
}

// NEW: Stefan
func (rx *RedisGtrsClient[T, R]) ReceiveStefan(ctx context.Context, groupName, consumerName, mode string, handlerFunc func(gtrs.Message[T]) error) error {
	// source: https://github.com/dranikpg/gtrs
	// consumer := gtrs.NewConsumer[T](ctx, rx.rdb, gtrs.StreamIDs{rx.requestQueue: "0"}, gtrs.StreamConsumerConfig{
	// 	Block:      0,
	// 	Count:      0,
	// 	BufferSize: 50,
	// })

	// mode "0" all history, ">" new entries
	block := 0 * time.Second
	if mode == "0" {
		block = 10 * time.Second
	}

	groupConfig := gtrs.GroupConsumerConfig{
		StreamConsumerConfig: gtrs.StreamConsumerConfig{
			Block:      block, // 0 means infinite
			Count:      1,     // maximum number of entries per request
			BufferSize: 1,     // how many entries to prefetch at most
		},
		AckBufferSize: 1, // size of the acknowledgement buffer
	}
	group := gtrs.NewGroupConsumer[T](ctx, rx.rdb, groupName, consumerName, rx.streamName, mode, groupConfig)
	defer group.Close()

	fmt.Printf("Waiting for stream:%s group:%s consumer:%s\n", rx.streamName, groupName, consumerName)
	for msg := range group.Chan() {
		if msg.Err != nil {
			fmt.Println("listener[1]: msg.err", msg.Err)
			continue
		}
		// call handler, acknowledge when no error is thrown
		err := handlerFunc(msg)
		if err == nil {
			group.Ack(msg)
		}
		if err != nil {
			fmt.Println("listener[2]: handler.err", err)
		}
		// act on handler error, like terminate loop
	}
	return fmt.Errorf("timeout waiting for reply")
}

func NewRedisGtrsClient[T any, R any](ctx context.Context, redisCfg *model.Config, requestQueue string, replyQueue string) (*RedisGtrsClient[T, R], error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisCfg.Redis.DSN,
	})

	timeout, err := time.ParseDuration(redisCfg.Publisher.Timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timeout: %w", err)
	}

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("failed to connect to Redis at %s for sink: %w", redisCfg.Redis.DSN, err)
	}
	stream := gtrs.NewStream[T](rdb, requestQueue, nil)
	return &RedisGtrsClient[T, R]{
		rdb:           rdb,
		requestStream: &stream,
		requestQueue:  requestQueue,
		replyQueue:    replyQueue,
		timeout:       timeout}, nil
}

/*
TODO: As simple consumer doesn't do ACK reply queue might fillup and cause long read on client side.
It should start waiting for response and read only new messages
*/
func (c *RedisGtrsClient[T, R]) RequestReply(ctx context.Context, payload T) (R, error) {
	err, corrID := c.SendRequest(ctx, payload)
	if err != nil {
		return c.zeroValue, err
	}
	return c.ReceiveResponse(ctx, corrID, c.timeout)
}

func (c *RedisGtrsClient[T, R]) SendRequest(ctx context.Context, payload T) (error, string) {
	corrID, err := c.requestStream.Add(ctx, payload)
	return err, corrID
}

func (c *RedisGtrsClient[T, R]) ReceiveResponse(ctx context.Context, corrID string, timeout time.Duration) (R, error) {
	// Read reply queue from the begining
	replyConsumer := gtrs.NewConsumer[R](ctx, c.rdb, gtrs.StreamIDs{c.replyQueue: "0"}, gtrs.StreamConsumerConfig{
		Block:      timeout,
		Count:      0,
		BufferSize: 50,
	})
	defer replyConsumer.Close()
	fmt.Println("Waiting for reply on:", c.replyQueue, corrID)
	for msg := range replyConsumer.Chan() {
		if msg.Err != nil {
			continue
		}
		var reply R = msg.Data
		if msg.ID == corrID {
			// This is the reply for our request
			return reply, nil
		}
	}
	// Create a zero value for type R in case of error
	return c.zeroValue, fmt.Errorf("timeout waiting for reply")
}

type RedisGtrsServer[T any, R any] struct {
	rdb          *redis.Client
	requestQueue string
	replyQueue   string
	replyStream  *gtrs.Stream[R]
}

func NewRedisGtrsServer[T any, R any](ctx context.Context, redisCfg model.Redis, requestQueue string, replyQueue string) (*RedisGtrsServer[T, R], error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisCfg.DSN,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("failed to connect to Redis at %s for sink: %w", redisCfg.DSN, err)
	}
	fmt.Println("Connected to Redis at:", redisCfg.DSN, requestQueue, replyQueue)
	replyStream := gtrs.NewStream[R](rdb, replyQueue, nil)

	return &RedisGtrsServer[T, R]{
		rdb:          rdb,
		requestQueue: requestQueue,
		replyQueue:   replyQueue,
		replyStream:  &replyStream}, nil
}

func (c *RedisGtrsServer[T, R]) ProcessRequest(ctx context.Context, handler func(T) R) {
	consumer := gtrs.NewGroupConsumer[T](ctx, c.rdb, "g1", "c1", c.requestQueue, "0-0", gtrs.GroupConsumerConfig{
		StreamConsumerConfig: gtrs.StreamConsumerConfig{
			Block:      0,   // 0 means infinite
			Count:      100, // maximum number of entries per request
			BufferSize: 20,  // how many entries to prefetch at most
		},
		AckBufferSize: 10, // size of the acknowledgement buffer
	})

	for msg := range consumer.Chan() {
		if msg.Err != nil {
			continue
		}
		req := msg.Data
		// Process the request and produce a reply
		result := handler(req)

		// Try to add the response to the reply stream
		replyID, err := c.replyStream.Add(ctx, result, msg.ID)
		if err != nil {
			fmt.Printf("ERROR sending reply: %v\n", err)
		} else {
			fmt.Printf("Successfully sent reply with ID: %s\n", replyID)
		}

		// Once it is in response queue, take it out of request queue
		consumer.Ack(msg)
	}
}
