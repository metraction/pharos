package mappers

import (
	"fmt"
	"testing"

	"github.com/metraction/pharos/pkg/model"
)

func TestWrap(t *testing.T) {
	r := model.NewTestScanResult(model.NewTestScanTask(t, "test-1", "test-image-1"), "test-engine-1")

	fn := Wrap(NewPureHbs[map[string]interface{}, map[string]interface{}]("../../testdata/enrichers/pass_through.hbs"))

	result := fn(WrappedResult{Result: r})

	fmt.Println(result)
}
