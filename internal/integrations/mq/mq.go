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

// https://redis.io/docs/latest/commands/xinfo-groups/
type GroupStats struct {
	StreamName string
	StreamLen  int64 // stream current len
	StreamMax  int64 // stream maxlen
	Read       int64 // total messages read
	Pending    int64 // pending messages (processed at leased once, but not ACKed)
	Lag        int64 // messages never processed
	//BackPressure float64  // return indicator of unfinished work or default on 1) error or 2) unlimited stream length
	Groups []string // consumer group names
}

func (rx *GroupStats) BackPressureOr(defval float64) float64 {
	if rx.StreamMax == 0 {
		return defval
	}
	return float64(rx.Pending+rx.Lag) / float64(rx.StreamMax)
}

// main task queue object
type RedisWorkerGroup[T any] struct {
	Endpoint   string
	StreamName string
	GroupName  string
	Mode       string // "$" read from last processed message, "0" read from start of queue
	MaxLen     int64  // stream max length

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

	// mode: "$" read from last processed message
	// mode: "0" read from start of queue
	result := RedisWorkerGroup[T]{
		Endpoint:   redisEndpoint,
		StreamName: streamName,
		GroupName:  groupName,
		Mode:       mode,
		MaxLen:     maxStreamLen, // max number of messages in stream (Redis trims to this automatically)
		rdb:        rdb,
	}

	return &result, nil
}

// redis connect
func (rx *RedisWorkerGroup[T]) Connect(ctx context.Context) error {
	if err := rx.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis connect (ping): %v", err)
	}
	return nil
}

// return service name
func (rx *RedisWorkerGroup[T]) ServiceName() string {
	return "pharos-mq"
}

// create redis stream group
func (rx *RedisWorkerGroup[T]) CreateGroup(ctx context.Context) error {

	if err := rx.rdb.XGroupCreateMkStream(ctx, rx.StreamName, rx.GroupName, rx.Mode).Err(); err != nil {
		// don't thorow error if stream/group already exists
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			return err
		}
	}
	return nil
}

// safely close redis connection
func (rx *RedisWorkerGroup[T]) Close() {
	if rx.rdb != nil {
		rx.rdb.Close()
	}
}

// retrieve memory usage
func (rx *RedisWorkerGroup[T]) UsedMemory(ctx context.Context) (string, string, string) {

	memUsed := "N/A"
	memPeak := "N/A"
	memSystem := "N/A"

	if info, err := rx.rdb.Info(ctx, "memory").Result(); err == nil {
		for _, line := range strings.Split(info, "\n") {
			var parts []string
			line = strings.TrimSpace(line)
			if parts = strings.Split(line, "used_memory_human:"); len(parts) == 2 {
				memUsed = parts[1]
			}
			if parts = strings.Split(line, "used_memory_peak_human:"); len(parts) == 2 {
				memPeak = parts[1]
			}
			if parts = strings.Split(line, "total_system_memory_human:"); len(parts) == 2 {
				memSystem = parts[1]
			}
		}
	}
	return memUsed, memPeak, memSystem
}

// delete stream (usefull for to begin tests with defined state)
func (rx *RedisWorkerGroup[T]) Delete(ctx context.Context) error {
	if _, err := rx.rdb.Del(ctx, rx.StreamName).Result(); err != nil {
		return err
	}
	return nil
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

// get group stats, filter by groupName (or "*" for all groups in stream)
func (rx *RedisWorkerGroup[T]) GroupStats(ctx context.Context, groupName string) (GroupStats, error) {

	result := GroupStats{
		StreamName: rx.StreamName,
		StreamMax:  rx.MaxLen,
	}
	groups, err := rx.rdb.XInfoGroups(ctx, rx.StreamName).Result()
	if err != nil {
		return result, err
	}

	for _, group := range groups {
		if groupName == group.Name || groupName == "*" {
			result.Read += group.EntriesRead
			result.Pending += group.Pending
			result.Lag += group.Lag
			result.Groups = append(result.Groups, group.Name)

		}
		// fmt.Printf("Name: %s, Consumers: %d, Pending: %d, LastDeliveredID: %s, EntriesRead: %d, Lag: %d\n",
		// 	group.Name, group.Consumers, group.Pending, group.LastDeliveredID, group.EntriesRead, group.Lag)
	}

	if length, err := rx.rdb.XLen(ctx, rx.StreamName).Result(); err == nil {
		result.StreamLen = length
	}
	return result, nil
}

func (rx *RedisWorkerGroup[T]) PressureOr(ctx context.Context, defval float64) float64 {
	stats, err := rx.GroupStats(ctx, rx.GroupName)
	if err != nil {
		return defval
	}
	return stats.BackPressureOr(defval)
}

// subscribe worker to new and pending tasks
// 1) block/wait for new tasks. Unblock after blockTime or when new message arrives
// 2) process new message (ACK message if handlerFunc return err==nil)
// 3) get max claimBlock pending messages with >minIdle time
// repeat until runTimeout, then exit
func (rx *RedisWorkerGroup[T]) Subscribe(ctx context.Context, consumerName string, claimBlock int64, minIdle, blockTime, runTimeout time.Duration, handlerFunc WorkerFunc[T]) error {

	k0 := 0
	k1 := 0
	k2 := 0
	startTime := time.Now()

	// process message, provide retry & idle count for handler, ACK if err returned is  nil
	processMessage := func(msg redis.XMessage) error {
		payload, err := NewTaskFromMessage[T](rx.StreamName, rx.GroupName, msg)
		if err != nil {
			return fmt.Errorf("error decoding message %s: %w", msg.ID, err)
		}
		if payload.RetryCount, payload.IdleTime, err = rx.getMsgState(ctx, msg.ID, rx.GroupName); err != nil {
			return err
		}
		if err := handlerFunc(payload); err != nil {
			return err
		} else {
			rx.rdb.XAck(ctx, rx.StreamName, rx.GroupName, msg.ID)
		}
		return nil
	}

	nextId := "0-0"
	for {
		k0 += 1
		//fmt.Printf("loop [%v] reclaim %v for %v with nextid %v\n", k0, claimBlock, consumerName, nextId)
		// autoclaim
		msgs, _, err := rx.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   rx.StreamName,
			Group:    rx.GroupName,
			Consumer: consumerName,
			MinIdle:  minIdle,
			Start:    nextId,     // Start scanning from the beginning
			Count:    claimBlock, // Number of messages to claim per call
		}).Result()
		if err != nil {
			return fmt.Errorf("XAutoClaim error: %v", err)
		}

		// process autoclaimed messages
		for _, msg := range msgs {
			k1++
			processMessage(msg)
			nextId = msg.ID
		}

		// subscribe to new: wait and fire on new, terminate after blockTime
		fresh, err := rx.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Streams:  []string{rx.StreamName, ">"},
			Group:    rx.GroupName,
			Consumer: consumerName,
			Count:    1,
			Block:    blockTime,
			NoAck:    false,
		}).Result()
		if err != nil {
			if err != redis.Nil {
				return fmt.Errorf("error reading new messages: %w", err)
			}
		}

		// process new messages
		for _, res := range fresh {
			for _, msg := range res.Messages {
				k2++
				processMessage(msg)
			}
		}

		// exit main loop
		if time.Since(startTime) > runTimeout {
			break
		}
	}

	// elapsed := time.Since(startTime)
	// fmt.Printf("done after %v iterations in (%vs / %vs): claimed:%v, read:%v\n", k0, elapsed.Seconds(), runTimeout.Seconds(), k1, k2)
	return nil
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
