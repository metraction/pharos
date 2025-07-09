package mappers

import (
	"fmt"
	"testing"
)

func TestStarlark(t *testing.T) {

	//r := model.NewTestScanResult(model.NewTestScanTask(t, "test-1", "test-image-1"), "test-engine-1")

	fn := NewStarlark("../../testdata/enrichers/fibonachi.star")

	result := fn(map[string]interface{}{
		"n": 10,
	})

	fmt.Println(result)
}
