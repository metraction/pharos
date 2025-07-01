package routing

import (
	"fmt"
	"testing"
	"time"

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

	contextRoot := model.ContextRoot{
		Key:       "test-1",
		ImageId:   "test-image-1",
		UpdatedAt: time.Now(),
		TTL:       1 * time.Hour,
		Contexts: []model.Context{
			{
				Owner: "eos",
				Data:  (<-stream.Out()).(map[string]interface{}),
			},
		},
	}
	if contextRoot.Contexts[0].Data == nil {
		t.Error("Context is nil")
		return
	}

	fmt.Printf("Context: %v\n", contextRoot.Contexts[0].Data)
	if contextRoot.Contexts[0].Data["spec"].(map[string]interface{})["eos"] == nil {
		t.Error("Context has no eos")
		return
	}
}
