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

var ErrTaskqueueTimeout = errors.New("taskqueue timeout")

// convert msg values map back to structure
func valuesToStruct[T any](values map[string]interface{}) (T, error) {
	var result T
	if err := gtrsconvert.MapToStruct(&result, values); err != nil {
		return result, err
	}
	return result, nil
}

type TaskMessage[T any] struct {
	RetryCount int64         // redis msg retry count
	IdleTime   time.Duration // redis msg idle time
	Id         string        // redis msg id
	Data       T
}

// return task message given redis message
func NewTaskMessage[T any](retryCount int64, idleTime time.Duration, msg redis.XMessage) (TaskMessage[T], error) {

	payload, err := valuesToStruct[T](msg.Values)
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

// main task queue object
type RedisTaskQueue[T any] struct {
	StreamName string
	MaxRetry   int64         // delete message with retryCount > maxRetry
	MaxLen     int64         // stream max length
	MaxTTL     time.Duration // delete messages with idleTime > maxTTL

	rdb *redis.Client
}

// return new task queue object with arguments set
func NewRedisGtrsQueue[T any](ctx context.Context, redisEndpoint, streamName string, maxStreamLen, maxRetry int64, maxTTL time.Duration) (*RedisTaskQueue[T], error) {

	options, err := redis.ParseURL(redisEndpoint)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(options)

	// NOTE: The stream is created with either automatically with XAdd or with the ConsumerGroup using rx.CreateGroup()

	result := RedisTaskQueue[T]{
		StreamName: streamName,
		MaxLen:     maxStreamLen, // max number of messages in stream (REdis trims to this automatically)
		MaxRetry:   maxRetry,     // delete messageas after maxRetry
		MaxTTL:     maxTTL,       // delete unacknowledged messages after maxTTL seconds
		rdb:        rdb,
	}

	return &result, nil
}

// delete stream
func (rx *RedisTaskQueue[T]) DeleteStream(ctx context.Context, steamName string) error {
	if _, err := rx.rdb.Del(ctx, steamName).Result(); err != nil {
		return err
	}
	return nil
}

// connect
func (rx *RedisTaskQueue[T]) Connect(ctx context.Context) error {
	if err := rx.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis connect (ping): %v", err)
	}
	return nil
}

// safely close connection
func (rx *RedisTaskQueue[T]) Close() {
	if rx.rdb != nil {
		rx.rdb.Close()
	}
}

// create stream and consumer group
func (rx *RedisTaskQueue[T]) CreateGroup(ctx context.Context, groupName, mode string) error {

	// mode: "$" read from last processed message
	// mode: "0" read from start of queue

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

func (rx *RedisTaskQueue[T]) RemoveStale(ctx context.Context, groupName string, batch int64, idleTime time.Duration) (int64, error) {

	var err error
	var staleIds = []string{}

	fmt.Println("- exec:RemoveStale() ...")

	// return list of pending tasks (pending: message not acknowledged  or  msg.idle > idleTime)
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
	fmt.Printf("- exec:RemoveStale() %v messages\n", len(pending))
	for _, msg := range pending {
		// delete if retry or TTL exceeded
		if msg.RetryCount > rx.MaxRetry || msg.Idle > rx.MaxTTL {
			staleIds = append(staleIds, msg.ID)
			fmt.Printf("remove.stale[%s] retry:%v, idle:%v\n", msg.ID, msg.RetryCount, msg.Idle.Seconds())

			// ATTN: ACK needed to remove stale
			rx.rdb.XAck(ctx, rx.StreamName, groupName, msg.ID)
			continue
		}
	}

	// now delete collected messages in one go
	if len(staleIds) > 0 {
		num, err := rx.rdb.XDel(ctx, rx.StreamName, staleIds...).Result()
		if err != nil {
			return 0, err
		}
		fmt.Printf("- deleted %v of %v messages\n", num, len(staleIds))
	}

	return int64(len(staleIds)), nil
}

// helper function to get retryCount and idleTime for given message
func (rx *RedisTaskQueue[T]) getMessageState(ctx context.Context, msgId, groupName string) (int64, time.Duration, bool, error) {

	pending, err := rx.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: rx.StreamName,
		Group:  groupName,
		Start:  msgId,
		End:    msgId,
		Count:  1,
	}).Result()

	if err != nil {
		return 0, 0, false, err
	}
	var retryCount int64
	var idleTime time.Duration

	if len(pending) > 0 {
		idleTime = pending[0].Idle
		retryCount = pending[0].RetryCount
	}

	// return true if message is expired, only do this check if either MaxRetry or MaxTTL is set
	if rx.MaxRetry > 0 && rx.MaxTTL > 0 {
		if retryCount > rx.MaxRetry || idleTime > rx.MaxTTL {
			return retryCount, idleTime, true, nil
		}
	}
	return retryCount, idleTime, false, nil
}

// helper function to delete message when it exceeds maxRetry or maxIdel time
func (rx *RedisTaskQueue[T]) DeleteMessage(ctx context.Context, groupName string, msgId string) error {

	rx.rdb.XAck(ctx, rx.StreamName, groupName, msgId)
	if _, err := rx.rdb.XDel(ctx, rx.StreamName, msgId).Result(); err != nil {
		return err
	}
	return nil
}

// reclaim messages left in PEL for longer than timeout
func (rx *RedisTaskQueue[T]) ReclaimStale(ctx context.Context, groupName, consumerName string, batch int64, minIdle time.Duration, handlerFunc func(TaskMessage[T]) error) (int64, error) {

	var err error
	var expired bool
	var retryCount int64
	var idleTime time.Duration
	var task TaskMessage[T]

	fmt.Printf("- exec:ReclaimStale() for %s...\n", consumerName)
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
		fmt.Printf("- reclaim %v\n", id)

		// check and enforce maxRetry and maxTTL exceeded
		if retryCount, idleTime, expired, err = rx.getMessageState(ctx, msg.ID, groupName); err != nil {
			return 0, err
		}
		if expired {
			if err = rx.DeleteMessage(ctx, groupName, msg.ID); err != nil {
				return 0, err
			}
			continue
		}

		// parse and process message
		if task, err = NewTaskMessage[T](retryCount, idleTime, msg); err != nil {
			return 0, err
		}
		err = handlerFunc(task)
		if err == nil {
			rx.rdb.XAck(ctx, rx.StreamName, groupName, msg.ID) // Acknowledge the message if no error returned
		}

	}
	return int64(len(x)), nil
}

// add message to stream
// TODO: implement priority
func (rx *RedisTaskQueue[T]) AddMessage(ctx context.Context, priority int, payload T) (string, error) {

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

// subscribe to messages from consumer group, execute handler for each message, timeout after blockTime
func (rx *RedisTaskQueue[T]) GroupSubscribe(ctx context.Context, mode, groupName, consumerName string, blockTime time.Duration, handlerFunc func(x TaskMessage[T]) error) error {

	var expired bool
	var retryCount int64
	var idleTime time.Duration
	var task TaskMessage[T]

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
			fmt.Printf("loop:GroupSubscribe %v msgs\n", len(stream.Messages))
			for _, msg := range stream.Messages {

				// check and enforce maxRetry and maxTTL exceeded
				if retryCount, idleTime, expired, err = rx.getMessageState(ctx, msg.ID, groupName); err != nil {
					return err
				}
				if expired {
					if err = rx.DeleteMessage(ctx, groupName, msg.ID); err != nil {
						return err
					}
					continue
				}
				// parse and process message
				if task, err = NewTaskMessage[T](retryCount, idleTime, msg); err != nil {
					return err
				}
				err = handlerFunc(task)
				if err == nil {
					rx.rdb.XAck(ctx, rx.StreamName, groupName, msg.ID) // Acknowledge the message if no error returned
				}
			}
		}
	}
}

// return stream length, queued, unconfirmed messages
func (rx *RedisTaskQueue[T]) GetState(ctx context.Context) (int64, int64, int64, error) {
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

func (rx *RedisTaskQueue[T]) XLen(ctx context.Context, streamName string) (int64, error) {
	length, err := rx.rdb.XLen(ctx, streamName).Result()
	if err != nil {
		return 0, err
	}

	return length, nil
}

func (rx *RedisTaskQueue[T]) XPending(ctx context.Context, streamName, groupName string) (*redis.XPending, error) {
	// Get number of pending entries for the group
	pending, err := rx.rdb.XPending(ctx, streamName, groupName).Result()
	if err != nil {
		return nil, err
	}
	return pending, nil
}
