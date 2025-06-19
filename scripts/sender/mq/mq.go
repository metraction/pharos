package mq

import (
	"context"
	"fmt"
	"time"

	"github.com/dranikpg/gtrs"
	"github.com/redis/go-redis/v9"
)

type RedisGtrsQueue[T any] struct {
	streamName string

	rdb    *redis.Client
	stream *gtrs.Stream[T]

	timeout time.Duration
}

func NewRedisGtrsQueue[T any](ctx context.Context, redisEndpoint, streamName string, maxlen int64) (*RedisGtrsQueue[T], error) {

	options, err := redis.ParseURL(redisEndpoint)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(options)

	timeout := 60 * time.Second
	stream := gtrs.NewStream[T](rdb, streamName, &gtrs.Options{
		TTL:    30 * time.Second,
		MaxLen: maxlen,
		Approx: true,
	})

	result := RedisGtrsQueue[T]{
		rdb:        rdb,
		stream:     &stream,
		streamName: streamName,
		timeout:    timeout}
	return &result, nil
}

func (rx *RedisGtrsQueue[T]) Connect(ctx context.Context) error {
	if err := rx.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis connect (ping): %v", err)
	}
	return nil
}

// NEW: Stefan
func (rx *RedisGtrsQueue[T]) Close() {
	if rx.rdb != nil {
		rx.rdb.Close()
	}
}
func (rx *RedisGtrsQueue[T]) Publish(ctx context.Context, payload T) (string, error) {
	id, err := rx.stream.Add(ctx, payload)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (rx *RedisGtrsQueue[T]) Subscribe(ctx context.Context, groupName, consumerName, mode string, handlerFunc func(gtrs.Message[T]) error) error {
	// source: https://github.com/dranikpg/gtrs

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
	}
	return fmt.Errorf("timeout waiting for reply")
}
