package mq

import (
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

func PayloadEncode(payload any) ([]byte, error) {
	values, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return values, nil
}
func PayloadDecode[T any](msg redis.XMessage) (T, error) {
	var data T
	if err := json.Unmarshal([]byte(msg.Values["data"].(string)), &data); err != nil {
		return data, err
	}
	return data, nil
}

type WorkerFunc[T any] func(x TaskMessage[T]) error

// task message for handler function
type TaskMessage[T any] struct {
	RetryCount int64         // redis msg retry count
	IdleTime   time.Duration // redis msg idle time
	StreamName string
	GroupName  string
	Id         string // redis msg id
	Data       T
}

// return task message given Redis message
func NewTaskFromMessage[T any](streamName, groupName string, msg redis.XMessage) (TaskMessage[T], error) {

	// payload, err := ValuesToStruct[T](msg.Values)
	payload, err := PayloadDecode[T](msg)
	if err != nil {
		return TaskMessage[T]{}, err
	}
	return TaskMessage[T]{
		Id:         msg.ID,
		StreamName: streamName,
		GroupName:  groupName,
		Data:       payload,
	}, nil
}
