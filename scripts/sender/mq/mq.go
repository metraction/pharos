package mq

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dranikpg/gtrs"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
)

type TaskMessage[T any] struct {
	IdleTime   time.Duration
	RetryCount int64
	MsgId      string
	Data       T
}

func NewTaskMessage[T any](idleTime time.Duration, retryCount int64, msg gtrs.Message[T]) TaskMessage[T] {
	return TaskMessage[T]{
		IdleTime:   idleTime,
		RetryCount: retryCount,
		MsgId:      msg.ID,
		Data:       msg.Data,
	}
}

type RedisGtrsQueue[T any] struct {
	StreamName string
	MaxLen     int64
	MaxTTL     time.Duration // delete messages with idleTime > MAxTTL
	MaxRetry   int64         // delete message with retryCount > MaxRetry
	stream     *gtrs.Stream[T]
	rdb        *redis.Client
}

func NewRedisGtrsQueue[T any](ctx context.Context, redisEndpoint, streamName string, maxStreamLen, maxRetry int64, maxTTL time.Duration) (*RedisGtrsQueue[T], error) {

	options, err := redis.ParseURL(redisEndpoint)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(options)

	stream := gtrs.NewStream[T](rdb, streamName, &gtrs.Options{
		TTL:    30 * time.Second,
		MaxLen: maxStreamLen,
		Approx: true,
	})

	result := RedisGtrsQueue[T]{
		StreamName: streamName,
		MaxLen:     maxStreamLen,
		MaxRetry:   maxRetry,
		MaxTTL:     maxTTL,

		rdb:    rdb,
		stream: &stream,
	}

	return &result, nil
}

// delete and recreate stream
func (rx *RedisGtrsQueue[T]) DeleteStream(ctx context.Context, steamName string) error {
	_, err := rx.rdb.Del(ctx, steamName).Result()
	if err != nil {
		return err
	}
	return nil
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

func (rx *RedisGtrsQueue[T]) CreateGroup(ctx context.Context, groupName, mode string) error {
	if err := rx.rdb.XGroupCreateMkStream(ctx, rx.StreamName, groupName, mode).Err(); err != nil {
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			return err
		}
	}
	return nil
}

// delete messages left unacknowleged or failed for longer than timeout
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
		}
	}
	// delete in one go
	if len(staleIds) > 0 {
		_, err := rx.rdb.XDel(ctx, rx.StreamName, staleIds...).Result()
		if err != nil {
			return 0, err
		}
	}
	return int64(len(staleIds)), nil
}

// return stream length, queued, unconfirmed messages
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

func (rx *RedisGtrsQueue[T]) Subscribe(ctx context.Context, groupName, consumerName, mode string, blockTime time.Duration, handlerFunc func(TaskMessage[T]) error) error {
	// source: https://github.com/dranikpg/gtrs
	// mode "0" all history, ">" new entries
	groupConfig := gtrs.GroupConsumerConfig{
		StreamConsumerConfig: gtrs.StreamConsumerConfig{
			Block:      blockTime, // 0 means infinite
			Count:      1,         // maximum number of entries per request
			BufferSize: 1,         // how many entries to prefetch at most
		},
		AckBufferSize: 1, // size of the acknowledgement buffer
	}
	group := gtrs.NewGroupConsumer[T](ctx, rx.rdb, groupName, consumerName, rx.StreamName, mode, groupConfig)
	defer group.Close()

	fmt.Printf("Waiting for stream:%s group:%s consumer:%s\n", rx.StreamName, groupName, consumerName)
	for msg := range group.Chan() {
		if msg.Err != nil {
			fmt.Println("Sub [channel:err] ", msg.Err)
			break
		}
		// get the message idleTime and retryCount
		pending, err := rx.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
			Stream: rx.StreamName,
			Group:  groupName,
			Start:  msg.ID,
			End:    msg.ID,
			Count:  1,
		}).Result()
		if err != nil {
			fmt.Println("Sub [pending:err] ", err)

		}
		idleTime := lo.Ternary(len(pending) > 0, pending[0].Idle, 0)
		retryCount := lo.Ternary(len(pending) > 0, pending[0].RetryCount, 0)

		// delete when maxRetry or maxTTL is exceeded
		if retryCount > rx.MaxRetry || idleTime > rx.MaxTTL {
			_, err := rx.rdb.XDel(ctx, rx.StreamName, msg.ID).Result()
			fmt.Printf("Sub [delete] %s: retry=%v, idle=%v sec, err: %v\n", msg.ID, retryCount, idleTime.Seconds(), err)
			continue
		}

		// call handler, acknowledge when no error is thrown
		message := NewTaskMessage(idleTime, retryCount, msg)
		err = handlerFunc(message)
		if err == nil {
			group.Ack(msg)
		} else {
			fmt.Println("Sub [handler:err] ", err)
		}
	}
	fmt.Println("Sub Done")
	return fmt.Errorf("timeout waiting for reply")
}

// reclaim messages left in PEL for longer than timeout
func (rx *RedisGtrsQueue[T]) Reclaim(ctx context.Context, groupName, consumerName string, batch int64, minIdle time.Duration, handlerFunc func(TaskMessage[T]) error) (int64, error) {

	fmt.Printf("- reclaim %v tasks for %v\n", batch, consumerName)
	x, msgs, err := rx.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   rx.StreamName,
		Group:    groupName,
		Consumer: consumerName,
		MinIdle:  minIdle,
		Start:    "0-0",
		Count:    batch, // batch size
	}).Result()
	if err != nil {
		return 0, err
	}
	fmt.Printf("- reclaim m: %v\n", msgs)
	for id, msg := range x {
		fmt.Printf("- reclaim x: %v: %v\n", id, msg)
		// call handler, acknowledge when no error is thrown
		message := NewTaskMessage(0, 0, msg)
		err = handlerFunc(message)
		if err == nil {
			if err := rx.rdb.XAck(ctx, rx.StreamName, groupName, msg.ID).Err(); err != nil {
				return 0, err
			}
		} else {
			fmt.Println("Sub [handler:err] ", err)
		}

	}

	return int64(len(msgs)), nil
}
