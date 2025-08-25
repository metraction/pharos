package mappers

import (
	"testing"

	"github.com/metraction/pharos/pkg/model"
)

func TestYaegi(t *testing.T) {
	type args struct {
		scriptPath string
		inputItem  model.PharosScanResult
	}
	type want struct {
		hasUpdate bool
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Non-Velero image",
			args: args{
				scriptPath: "../../testdata/enrichers/yaegi/has-update.yaegi",
				inputItem:  model.NewTestScanResult(model.NewTestScanTask(t, "test-1", "test-image-1"), "test-engine-1"),
			},
			want: want{
				hasUpdate: false,
			},
		},
		{
			name: "Velero image",
			args: args{
				scriptPath: "../../testdata/enrichers/yaegi/has-update.yaegi",
				inputItem:  model.NewTestScanResult(model.NewTestScanTask(t, "test-2", "velero/velero:v1.10.0"), "test-engine-1"),
			},
			want: want{
				hasUpdate: true, // Assuming there's a newer version available
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert input to map and run the mapper
			inputMap := map[string]interface{}{
				"payload": ToMap(tt.args.inputItem),
			}
			result := NewYaegi(tt.args.scriptPath)(inputMap)

			if result["hasUpdate"] != tt.want.hasUpdate {
				t.Errorf("Expected result[hasUpdate] to be %v, got %v", tt.want.hasUpdate, result["hasUpdate"])
			}
		})
	}
}
