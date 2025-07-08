package routing

import (
	"fmt"

	"github.com/reugn/go-streams/flow"
)

func NewMono(element any) chan any {
	outChan := make(chan any, 1)
	go func() {
		outChan <- element
		close(outChan)
	}()
	return outChan
}

func NewDebug(label string) flow.MapFunction[any, any] {
	return func(data any) any {
		fmt.Println(label, data)
		return data
	}
}

func NewNotifier(channel chan any) flow.MapFunction[any, any] {
	return func(data any) any {
		channel <- data
		return data
	}
}
