package mappers

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/flow"
)

func TestStarlet(t *testing.T) {

	item := model.NewTestScanResult(model.NewTestScanTask(t, "test-1", "test-image-1"), "test-engine-1")
	mapper, err := NewStarlet("../../testdata/enrichers/starlet/risk.star")
	if err != nil {
		t.Fatal(err)
	}
	input := ToMap(item)
	result := mapper(input)

	fmt.Println("Result:", result)
	if len(result) == 0 {
		t.Fatal("Result is empty")
	}
}

func TestNewStarlet(t *testing.T) {
	type args struct {
		rule string
	}
	tests := []struct {
		name string
		args args
		want flow.MapFunction[map[string]interface{}, map[string]interface{}]
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := NewStarlet(tt.args.rule); err != nil {
				t.Fatal(err)
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewStarlet() = %v, want %v", got, tt.want)
			}
		})
	}
}
