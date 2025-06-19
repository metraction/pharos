package mq

import (
	"context"
	"fmt"
	"time"

	"github.com/dranikpg/gtrs"
	"github.com/redis/go-redis/v9"
)

type RedisGtrsQueue[T any] struct {
	StreamName string
	timeout    time.Duration
	stream     *gtrs.Stream[T]
	rdb        *redis.Client
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
		StreamName: streamName,
		rdb:        rdb,
		stream:     &stream,

		timeout: timeout}
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

// delete messages left unacknowleged or failed for longer thant timeout
func (rx *RedisGtrsQueue[T]) DeleteStale(ctx context.Context, groupName string, batch int64, timeout time.Duration) (int64, error) {

	pending, err := rx.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: rx.StreamName,
		Group:  groupName,
		Start:  "-",
		End:    "+",
		Count:  batch, // batch size
	}).Result()
	if err != nil {
		return 0, err
	}
	staleIds := []string{}
	for _, msg := range pending {
		if msg.Idle > timeout {
			staleIds = append(staleIds, msg.ID)
			if err := rx.rdb.XAck(ctx, rx.StreamName, groupName, msg.ID).Err(); err != nil {
				return 0, err
			}

			fmt.Println("delete", msg.ID, msg.Idle)
		}
	}
	// delete in one go
	if len(staleIds) > 0 {
		fmt.Println("delete", rx.StreamName, staleIds)
		_, err := rx.rdb.XDel(ctx, rx.StreamName, staleIds...).Result()
		if err != nil {
			return 0, err
		}
	}
	return int64(len(staleIds)), nil
}

// return strream length, unprocessed, unconfirmed messages
func (rx *RedisGtrsQueue[T]) GetState(ctx context.Context) (int64, int64, int64, error) {
	var unprocessed int64
	var unconfirmed int64

	// total length
	length, err := rx.rdb.XLen(ctx, rx.StreamName).Result()
	if err != nil {
		return 0, 0, 0, err
	}

	// queued, not yet processed
	groups, err := rx.rdb.XInfoGroups(ctx, rx.StreamName).Result()
	if err != nil {
		return 0, 0, 0, err
	}
	for _, group := range groups {
		// processed, but not acknowledged
		pending, err := rx.rdb.XPending(ctx, rx.StreamName, group.Name).Result()
		if err != nil {
			return 0, 0, 0, err
		}
		unconfirmed += pending.Count
		unprocessed += group.Lag
	}
	return length, unprocessed, unconfirmed, err
}

func (rx *RedisGtrsQueue[T]) XLen(ctx context.Context, streamName string) (int64, error) {
	length, err := rx.rdb.XLen(ctx, streamName).Result()
	if err != nil {
		return 0, err
	}

	return length, nil
}

func (rx *RedisGtrsQueue[T]) XPending(ctx context.Context, streamName, groupName string) (*redis.XPending, error) {
	// Get number of pending entries for the group
	pending, err := rx.rdb.XPending(ctx, streamName, groupName).Result()
	if err != nil {
		return nil, err
	}
	return pending, nil
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
	group := gtrs.NewGroupConsumer[T](ctx, rx.rdb, groupName, consumerName, rx.StreamName, mode, groupConfig)
	defer group.Close()

	fmt.Printf("Waiting for stream:%s group:%s consumer:%s\n", rx.StreamName, groupName, consumerName)
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
