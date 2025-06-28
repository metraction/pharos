package routing

import (
	"testing"

	"github.com/reugn/go-streams/extension"
)

func TestRedisQueueLimit(t *testing.T) {
	messageChan := make(chan any)
	go NewEosEnricher(extension.NewChanSource(messageChan))

}
