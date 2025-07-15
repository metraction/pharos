package mappers

import (
	"fmt"
	"testing"

	"github.com/metraction/pharos/pkg/model"
)

func TestStarlark(t *testing.T) {

	item := model.NewTestScanResult(model.NewTestScanTask(t, "test-1", "test-image-1"), "test-engine-1")
	result := NewStarlark("../../testdata/enrichers/fibonachi.star")(ToMap(item))

	fmt.Println(result)
}
