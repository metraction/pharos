package mq

import (
	"fmt"
	"regexp"
	"time"

	"github.com/metraction/pharos/internal/utils"
)

// return parts from task queue DSN "queue://stream:group/?maxlen=1000&maxretry=2&maxttl=1h"
func ParseTaskQueueDsn(input string) (string, string, int64, int64, time.Duration, error) {

	var streamName string
	var groupName string
	var maxLen int64
	var maxRetry int64
	var maxTTL time.Duration

	rex1 := regexp.MustCompile(`queue://([^:]+):([^:/]+).*`)
	if match := rex1.FindStringSubmatch(input); len(match) > 1 {
		streamName = match[1]
		groupName = match[2]
	} else {
		return "", "", 0, 0, maxTTL, fmt.Errorf("invalid DSN (1) %v", input)
	}

	rex2 := regexp.MustCompile(`([?&])([^=&]+)=([^&]+)`)
	for _, match := range rex2.FindAllStringSubmatch(input, -1) {
		// match[2] is the key, match[3] is the value
		key := match[2]
		val := match[3]
		if key == "maxlen" {
			maxLen = utils.ToNumOr[int64](val, 0)
		}
		if key == "maxretry" {
			maxRetry = utils.ToNumOr[int64](val, 0)
		}
		if key == "maxttl" {
			maxTTL, _ = time.ParseDuration(val)
		}

	}
	return streamName, groupName, maxLen, maxRetry, maxTTL, nil
}
