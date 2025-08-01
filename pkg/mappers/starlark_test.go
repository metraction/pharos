package mappers

import (
	"testing"

	"github.com/metraction/pharos/pkg/model"
)

func TestStarlark(t *testing.T) {
	type args struct {
		scriptPath string
		inputItem  model.PharosScanResult
	}
	type want struct {
		exempted bool
		reason   string
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Some image",
			args: args{
				scriptPath: "../../testdata/enrichers/starlark/exemption.star",
				inputItem:  model.NewTestScanResult(model.NewTestScanTask(t, "test-1", "test-image-1"), "test-engine-1"),
			},
			want: want{
				exempted: false,
				reason:   "No exemption found",
			},
		},
		{
			name: "Corda image",
			args: args{
				scriptPath: "../../testdata/enrichers/starlark/exemption.star",
				inputItem:  model.NewTestScanResult(model.NewTestScanTask(t, "test-2", "corda-1"), "test-engine-1"),
			},
			want: want{
				exempted: true,
				reason:   "Corda image has jar files that are not used, but can't be removed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert input to map and run the mapper
			inputMap := map[string]interface{}{
				"payload": ToMap(tt.args.inputItem),
			}
			result := NewStarlark(tt.args.scriptPath)(inputMap)

			if result["exempted"] != tt.want.exempted {
				t.Errorf("Expected result[exempted] to be %v, got %v", tt.want.exempted, result["exempted"])
			}
			if result["reason"] != tt.want.reason {
				t.Errorf("Expected result[reason] to be %v, got %v", tt.want.reason, result["reason"])
			}
		})
	}
}
