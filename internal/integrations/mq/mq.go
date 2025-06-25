package mq

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/metraction/pharos/internal/gtrsconvert"
	"github.com/redis/go-redis/v9"
)

// gtrsconvert: https://dev.to/danielgtaylor/reducing-go-dependencies-dec

// error messages
var ErrTaskqueueTimeout = errors.New("taskqueue timeout")
var ErrMsgDelete = errors.New("[handler] message delete")

// main task queue object
type RedisWorkerGroup[T any] struct {
	RedisEndpoint string
	StreamName    string
	GroupName     string
	Mode          string
	MaxLen        int64 // stream max length

	rdb *redis.Client
}

// return new task queue object with arguments set
func NewRedisWorkerGroup[T any](ctx context.Context, redisEndpoint, mode, streamName, groupName string, maxStreamLen int64) (*RedisWorkerGroup[T], error) {

	// create redis client
	options, err := redis.ParseURL(redisEndpoint)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(options)

	// setup object parameter
	// mode: "$" read from last processed message
	// mode: "0" read from start of queue

	result := RedisWorkerGroup[T]{
		RedisEndpoint: redisEndpoint,
		StreamName:    streamName,
		GroupName:     groupName,
		Mode:          mode,
		MaxLen:        maxStreamLen, // max number of messages in stream (Redis trims to this automatically)
		rdb:           rdb,
	}

	return &result, nil
}

// connect
func (rx *RedisWorkerGroup[T]) Connect(ctx context.Context) error {

	if err := rx.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis connect (ping): %v", err)
	}

	// NOTE: The stream is created with either automatically with XAdd or with the below
	if err := rx.rdb.XGroupCreateMkStream(ctx, rx.StreamName, rx.GroupName, rx.Mode).Err(); err != nil {
		// don't thorow error if stream/group already exists
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			return err
		}
	}
	return nil
}

// safely close connection
func (rx *RedisWorkerGroup[T]) Close() {
	if rx.rdb != nil {
		rx.rdb.Close()
	}
}

// publish message
// TODO: priority queue
func (rx *RedisWorkerGroup[T]) Publish(ctx context.Context, priority int, payload T) (string, error) {

	var err error
	var values map[string]any

	if values, err = gtrsconvert.StructToMap(payload); err != nil {
		return "", err
	}
	id, err := rx.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: rx.StreamName,
		MaxLen: rx.MaxLen,
		Approx: true,
		Values: values, // data
	}).Result()
	if err != nil {
		return "", err
	}
	return id, nil
}

// subscribe worker to tasks
func (rx *RedisWorkerGroup[T]) Subscribe(ctx context.Context, consumerName string, pendingBlock int64, blockTime time.Duration, handlerFunc WorkerFunc[T]) error {

	k1 := 0
	k2 := 0
	// read pending first (once)
	pending, err := rx.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    rx.GroupName,
		Consumer: consumerName,
		Streams:  []string{rx.StreamName, "0"},
		Count:    pendingBlock,
		Block:    0,
		NoAck:    false,
	}).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("error reading pending messages: %w", err)
	}
	// process pending
	for _, res := range pending {
		for _, msg := range res.Messages {
			k1++
			rx.rdb.XAck(ctx, rx.StreamName, rx.GroupName, msg.ID)
		}
	}

	// read new (for block time)
	for {
		fresh, err := rx.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    rx.GroupName,
			Consumer: consumerName,
			Streams:  []string{rx.StreamName, ">"},
			Count:    1,
			Block:    blockTime,
			NoAck:    false,
		}).Result()
		if err != nil {
			if err == redis.Nil {
				break
			}
			return fmt.Errorf("error reading new messages: %w", err)
		}
		// process pending
		for _, res := range fresh {
			for _, msg := range res.Messages {
				k2++
				payload, err := NewTaskFromMessage[T](msg)
				if err != nil {
					return fmt.Errorf("error decoding message %s: %w", msg.ID, err)
				}
				if payload.RetryCount, payload.IdleTime, err = rx.getMsgState(ctx, msg.ID, rx.GroupName); err != nil {
					return err
				}
				if err := handlerFunc(payload); err != nil {
					fmt.Printf("error in %s: %v\n", msg.ID, err)
				} else {
					rx.rdb.XAck(ctx, rx.StreamName, rx.GroupName, msg.ID)
				}
			}
		}
	}
	return fmt.Errorf("timeout after %s, read %v pending, %v recent msgs", blockTime.String(), k1, k2)
}

// helper function to get retryCount and idleTime for given message
func (rx *RedisWorkerGroup[T]) getMsgState(ctx context.Context, msgId, groupName string) (int64, time.Duration, error) {

	pending, err := rx.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: rx.StreamName,
		Group:  groupName,
		Start:  msgId,
		End:    msgId,
		Count:  1,
	}).Result()
	if err != nil {
		return 0, 0, err
	}
	if len(pending) > 0 {
		return pending[0].RetryCount, pending[0].Idle, nil
	}
	return 0, 0, fmt.Errorf("error getting retry, idle for %s", pending[0].ID)

}
