package cmd

import (
	"fmt"
	"sort"
	"testing"

	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/extension"
	"gopkg.in/yaml.v3"
)

func TestStarlarkPlugin(t *testing.T) {
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
			name: "Starlark",
			args: args{
				scriptPath: "../testdata/enrichers/starlark",
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
			enrichers := &model.EnrichersConfig{
				Order: []string{"results"},
				Sources: []model.EnricherSource{
					{
						Name: "results",
						Path: tt.args.scriptPath,
					},
				},
			}

			inputChannel := make(chan any, 1)
			inputChannel <- tt.args.inputItem
			close(inputChannel)

			plugin := CreateEnrichersFlow(extension.NewChanSource(inputChannel), enrichers, nil, nil)
			result := (<-plugin.Out()).(model.PharosScanResult)

			// Find the results context
			var starlarkContext *model.Context
			for i := range result.Image.ContextRoots[0].Contexts {
				if result.Image.ContextRoots[0].Contexts[i].Owner == "results" {
					starlarkContext = &result.Image.ContextRoots[0].Contexts[i]
					break
				}
			}

			out, err := yaml.Marshal(result.Image.ContextRoots)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			fmt.Println(string(out))

			if starlarkContext == nil {
				t.Fatalf("Results context not found in result")
			}

			fmt.Printf("Number of contexts: %d\n", len(result.Image.ContextRoots[0].Contexts))
			for i, ctx := range result.Image.ContextRoots[0].Contexts {
				fmt.Printf("Context %d - Owner: %s, Data keys: %v\n", i, ctx.Owner, getMapKeys(ctx.Data))
			}

			if starlarkContext.Data["exempted"] != tt.want.exempted {
				t.Errorf("Expected result[exempted] to be %v, got %v", tt.want.exempted, starlarkContext.Data["exempted"])
			}
			if starlarkContext.Data["reason"] != tt.want.reason {
				t.Errorf("Expected result[reason] to be %v, got %v", tt.want.reason, starlarkContext.Data["reason"])
			}
		})
	}
}

func TestCreateConfigMapConventions_HelmMode(t *testing.T) {
	enrichers := &model.EnrichersConfig{
		Order: []string{"eos", "owner", "findings-summary"},
		Sources: []model.EnricherSource{
			{Name: "eos"},
			{Name: "owner"},
			{Name: "findings-summary"},
		},
	}
	configMap, err := createConfigMap(enrichers, "custom-name", false)
	if err != nil {
		t.Fatalf("createConfigMap error: %v", err)
	}

	data, ok := configMap["data"].(map[string]*yaml.Node)
	if !ok {
		t.Fatalf("data type = %T, want map[string]*yaml.Node", configMap["data"])
	}

	if data["enrichers.yaml"] == nil {
		t.Fatalf("enrichers type = %T, want map[string]*yaml.Node", configMap["data"])
	}
}

// getMapKeys returns a sorted slice of keys from a map[string]interface{}.
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
