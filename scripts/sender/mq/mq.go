package mq

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dranikpg/gtrs"
	"github.com/metraction/pharos/internal/gtrsconvert"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
)

var ErrTaskqueueTimeout = errors.New("taskqueue timeout")

func ValuesToStruct[T any](values map[string]interface{}) (T, error) {
	var result T
	if err := gtrsconvert.MapToStruct(&result, values); err != nil {
		return result, err
	}
	return result, nil
}

type TaskMessage[T any] struct {
	RetryCount int64         // Redis msg retry count
	IdleTime   time.Duration // Redis msg idle time
	Id         string        // Redis msg id
	Data       T
}

// return task message given redis message
func NewTaskMessage[T any](retryCount int64, idleTime time.Duration, msg redis.XMessage) (TaskMessage[T], error) {

	payload, err := ValuesToStruct[T](msg.Values)
	if err != nil {
		return TaskMessage[T]{}, err
	}
	return TaskMessage[T]{
		Id:         msg.ID,
		RetryCount: retryCount,
		IdleTime:   idleTime,
		Data:       payload,
	}, nil
}

type RedisGtrsQueue[T any] struct {
	StreamName string
	MaxRetry   int64         // delete message with retryCount > maxRetry
	MaxLen     int64         // stream max length
	MaxTTL     time.Duration // delete messages with idleTime > maxTTL

	stream1 *gtrs.Stream[T]

	rdb *redis.Client
}

func NewRedisGtrsQueue[T any](ctx context.Context, redisEndpoint, streamName string, maxStreamLen, maxRetry int64, maxTTL time.Duration) (*RedisGtrsQueue[T], error) {

	options, err := redis.ParseURL(redisEndpoint)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(options)

	// NOTE: The stream is created with eiter automatically with XAdd or with the ConsumerGroup using rx.CreateGroup()

	result := RedisGtrsQueue[T]{
		StreamName: streamName,
		MaxLen:     maxStreamLen, // max number of messages in stream (REdis trims to this automatically)
		MaxRetry:   maxRetry,     // delete messageas after maxRetry
		MaxTTL:     maxTTL,       // delete unacknowledged messages after maxTTL seconds
		rdb:        rdb,
	}

	return &result, nil
}

// create stream and consumer group
func (rx *RedisGtrsQueue[T]) CreateGroup(ctx context.Context, groupName, mode string) error {
	// mode: "$" read from last, "0" read from start
	if err := rx.rdb.XGroupCreateMkStream(ctx, rx.StreamName, groupName, mode).Err(); err != nil {
		// don't thorow error if stream/group already exists
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			return err
		}
	}
	return nil
}

// delete messages left unacknowleged or failed for longer than timeout
// call this method on scheduled intervals to cleanup the taskqueue
// https://redis.io/docs/latest/commands/xpending/
func (rx *RedisGtrsQueue[T]) RemoveStale(ctx context.Context, groupName string, batch int64, idleTime time.Duration) (int64, error) {

	var err error
	var staleIds = []string{}

	fmt.Println("- exec:RemoveStale() ...")
	// return list of pending tasks (not ACKED, msg.idle > idleTime)
	pending, err := rx.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: rx.StreamName,
		Group:  groupName,
		Start:  "-",
		End:    "+",
		Idle:   idleTime, // how far to look back
		Count:  batch,    // batch size
	}).Result()
	if err != nil {
		return 0, err
	}

	// collect messages to delete
	for _, msg := range pending {
		// delete if retry or TTL exceeded
		if msg.RetryCount > rx.MaxRetry || msg.Idle > rx.MaxTTL {
			staleIds = append(staleIds, msg.ID)
			fmt.Printf("remove.stale[%s] retry:%v, idle:%v\n", msg.ID, msg.RetryCount, msg.Idle.Seconds())
			// ACK needed to remove stale
			rx.rdb.XAck(ctx, rx.StreamName, groupName, msg.ID)
			continue
		}
	}

	// delete in one go
	if len(staleIds) > 0 {
		num, err := rx.rdb.XDel(ctx, rx.StreamName, staleIds...).Result()
		if err != nil {
			return 0, err
		}
		fmt.Printf("- deleted %v of %v messages\n", num, len(staleIds))
	}

	return int64(len(staleIds)), nil
}

// reclaim messages left in PEL for longer than timeout
func (rx *RedisGtrsQueue[T]) ReclaimStale(ctx context.Context, groupName, consumerName string, batch int64, minIdle time.Duration, handlerFunc func(TaskMessage[T]) error) (int64, error) {

	var task TaskMessage[T]
	var retryCount int64
	var idleTime time.Duration

	fmt.Println("- exec:ReclaimStale() ...")
	x, _, err := rx.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
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

	fmt.Printf("- reclaim m: %v\n", len(x))
	for id, msg := range x {
		fmt.Printf("- reclaim %v: %v\n", id, msg)

		retryCount, idleTime, err = rx.GetMessageMeta(ctx, msg.ID, groupName)
		if err != nil {
			return 0, err
		}

		// call handler, acknowledge when no error is thrown
		task, err = NewTaskMessage[T](retryCount, idleTime, msg)
		if err != nil {
			return 0, err
		}
		err = handlerFunc(task)
		if err == nil {
			if err := rx.rdb.XAck(ctx, rx.StreamName, groupName, msg.ID).Err(); err != nil {
				return 0, err
			}
		} else {
			fmt.Println("Sub [handler:err] ", err)
		}
	}

	return int64(len(x)), nil
}

// add message to stream
// TODO: implement priority
func (rx *RedisGtrsQueue[T]) AddMessage(ctx context.Context, priority int, payload T) (string, error) {

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

// return retryCount, idleTime for given message
func (rx *RedisGtrsQueue[T]) GetMessageMeta(ctx context.Context, msgId, groupName string) (int64, time.Duration, error) {

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
	idleTime := lo.Ternary(len(pending) > 0, pending[0].Idle, 0)
	retryCount := lo.Ternary(len(pending) > 0, pending[0].RetryCount, 0)
	return retryCount, idleTime, nil
}

// subscribe to messages from consumer group, execute handler for each message
func (rx *RedisGtrsQueue[T]) GroupSubscribe(ctx context.Context, mode, groupName, consumerName string, blockTime time.Duration, handlerFunc func(x TaskMessage[T]) error) error {

	var task TaskMessage[T]
	var retryCount int64
	var idleTime time.Duration

	for {
		// wait for messages
		res, err := rx.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    groupName,
			Consumer: consumerName,
			Streams:  []string{rx.StreamName, mode}, // ">" means new messages only
			Block:    blockTime,                     // 0=inifinte, else unblock after N seconds
			Count:    1,                             // Max messages to return at once
		}).Result()
		if err != nil {
			if err == redis.Nil {
				return ErrTaskqueueTimeout // No new messages, unblocked after timeout
			}
			return err // Error reading from group
		}

		// process received messages
		for _, stream := range res {
			for _, msg := range stream.Messages {

				// if payload, err = ValuesToStruct[T](msg.Values); err != nil {
				// 	return err
				// }

				// check and enforce maxRetry and maxTTL
				retryCount, idleTime, err = rx.GetMessageMeta(ctx, msg.ID, groupName)
				if err != nil {
					return err
				}
				if rx.MaxRetry > 0 && rx.MaxTTL > 0 {
					if retryCount > rx.MaxRetry || idleTime > rx.MaxTTL {
						fmt.Printf("del[%s] retry:%v, idle:%v\n", msg.ID, retryCount, idleTime.Seconds())
						if _, err := rx.rdb.XDel(ctx, rx.StreamName, msg.ID).Result(); err != nil {
							return err
						}
						continue
					}
				}
				// parse message
				task, err = NewTaskMessage[T](retryCount, idleTime, msg)
				if err != nil {
					return err
				}
				err = handlerFunc(task)
				if err == nil {
					// Acknowledge the message
					rx.rdb.XAck(ctx, rx.StreamName, groupName, msg.ID)
				}
			}
		}
	}

}

func (rx *RedisGtrsQueue[T]) Publish(ctx context.Context, payload T) (string, error) {
	id, err := rx.stream1.Add(ctx, payload)
	if err != nil {
		return "", err
	}
	return id, nil
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
