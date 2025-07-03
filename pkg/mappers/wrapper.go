package mappers

import (
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/flow"
)

/*
For mappers simplicity origin input might be lost.
Wrapper preservers origin scan result and modifies only Context
This allows to store only last context in result
*/
type WrappedResult struct {
	Context map[string]interface{}
	Result  model.PharosScanResult
}

func Wrap(fn flow.MapFunction[map[string]interface{}, map[string]interface{}]) flow.MapFunction[WrappedResult, WrappedResult] {
	return func(input WrappedResult) WrappedResult {
		return WrappedResult{
			Context: fn(input.Context),
			Result:  input.Result,
		}
	}
}
