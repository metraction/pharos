package routing

import (
	"fmt"
	"testing"

	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/policy-engine/pkg/enricher"
	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
)

func TestEosEnricher(t *testing.T) {
	messageChan := make(chan any, 1)
	messageChan <- model.NewTestScanResult(model.NewTestScanTask(t, "test-1", "test-image-1"), "test-engine-1")
	source := extension.NewChanSource(messageChan).
		Via(flow.NewMap(enricher.NewMapOfMaps(), 1))

	stream := NewEosEnricher(source, "../../testdata/enrichers").
		Via(flow.NewMap(enricher.NewDebug(), 1))
	result := (<-stream.Out()).(map[string]interface{})

	fmt.Println("Result:", result)
}
