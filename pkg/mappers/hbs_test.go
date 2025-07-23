package mappers

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
	"gopkg.in/yaml.v3"
)

// Vulnerability represents a single vulnerability with a Risk value.
type Vulnerability struct {
	L1 int
	L2 int
}

// Image represents an image with vulnerabilities.
type Image struct {
	Numbers         []int
	Vulnerabilities []Vulnerability
	Namespace       string
	Distro          string
	Version         string
}

type RiskV1 struct {
	Version string `yaml:"version"`
	Kind    string `yaml:"kind"`
	Spec    struct {
		Risk int `yaml:"risk"`
	} `yaml:"spec"`
}

type RiskV2 struct {
	Version string `yaml:"version"`
	Kind    string `yaml:"kind"`
	Spec    struct {
		Risk   int    `yaml:"risk"`
		Reason string `yaml:"reason"`
	} `yaml:"spec"`
}

func ToRisk(buf *bytes.Buffer) (interface{}, error) {
	// Unmarshal only the version field first
	var versionProbe struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(buf.Bytes(), &versionProbe); err != nil {
		log.Printf("Error unmarshalling:\n%s", buf.Bytes())
		return nil, err
	}

	switch versionProbe.Version {
	case "v1":
		var riskV1 RiskV1
		if err := yaml.Unmarshal(buf.Bytes(), &riskV1); err != nil {
			return nil, err
		}
		return riskV1, nil
	case "v2":
		var riskV2 RiskV2
		if err := yaml.Unmarshal(buf.Bytes(), &riskV2); err != nil {
			return nil, err
		}
		return riskV2, nil
	default:
		return nil, fmt.Errorf("unsupported risk version: %s", versionProbe.Version)
	}

}

func TestApplyRiskV1Template(t *testing.T) {

	// Prepare test data
	img := Image{
		Numbers: []int{1, 2, 3},
		Vulnerabilities: []Vulnerability{
			{L1: 5},
			{L1: 7},
		},
		Distro: "alpine",
	}

	// Ensure the template exists for the test

	templatePath := filepath.Join("..", "..", "testdata", "enrichers", "risk_v1.hbs")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Fatalf("Template file %s does not exist", templatePath)
	}

	rp, err := NewPolicy[RiskV1](templatePath)
	if err != nil {
		t.Fatalf("NewRiskPolicy returned error: %v", err)
	}

	buf, err := rp.Evaluate(img)
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	risk, err := ToRisk(buf)
	if err != nil {
		t.Fatalf("ToRisk returned error: %v", err)
	}

	riskV1, ok := risk.(RiskV1)
	if !ok {
		t.Fatalf("Expected RiskV1, got %T", risk)
	}

	if riskV1.Version == "" || riskV1.Kind == "" {
		t.Errorf("RiskV1 struct not properly filled: %+v", riskV1)
	}

	if riskV1.Kind != "RiskPolicy" {
		t.Errorf("Expected Kind to be 'RiskPolicy', got '%s'", riskV1.Kind)
	}

	if riskV1.Spec.Risk != 12 {
		t.Errorf("Expected Risk to be 14, got %d", riskV1.Spec.Risk)
	}
}

func TestApplyRiskV2Template(t *testing.T) {
	tests := []struct {
		name           string
		img            Image
		expectedRiskV2 RiskV2
	}{
		{
			name: "Frontend Namespace",
			img: Image{
				Vulnerabilities: []Vulnerability{
					{L1: 2, L2: 3},
					{L1: 4, L2: 5},
				},
				Namespace: "frontend",
			},
			expectedRiskV2: RiskV2{
				Version: "v2",
				Kind:    "RiskPolicy",
				Spec: struct {
					Risk   int    `yaml:"risk"`
					Reason string `yaml:"reason"`
				}{
					Risk:   14,
					Reason: "Interet facing image",
				},
			},
		},
		// Add more test cases here as needed.  For example, you might add a test
		// case for a "dmz" namespace, or for different combinations of vulnerabilities.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// applyRiskV2TemplateAndVerify(t, tt.img, tt.expectedRiskV2)
			templatePath := filepath.Join("..", "..", "testdata", "enrichers", "risk_v2.hbs")
			if _, err := os.Stat(templatePath); os.IsNotExist(err) {
				t.Fatalf("Template file %s does not exist", templatePath)
				t.Fatalf("ToRisk returned error: %v", err)
			}
			rp, err := NewPolicy[RiskV2](templatePath)
			if err != nil {
				t.Fatalf(": %v", err)
			}

			buf, err := rp.Evaluate(tt.img)
			if err != nil {
				t.Fatalf("Evaluate returned error: %v", err)
			}
			risk, err := ToRisk(buf)
			if err != nil {
				t.Fatalf("ToRisk returned error: %v", err)
			}

			actualRiskV2, ok := risk.(RiskV2)
			if !ok {
				t.Fatalf("Expected RiskV2, got %T", risk)
			}

			if !ok {
				t.Fatalf("Expected RiskV2, got %T", tt.expectedRiskV2)
			}

			if !reflect.DeepEqual(actualRiskV2, tt.expectedRiskV2) {
				t.Errorf("applyRiskV2Template(%v) = %v, want %v", tt.img, actualRiskV2, tt.expectedRiskV2)
			}
		})
	}
}

func TestStream(t *testing.T) {
	templatePath := filepath.Join("..", "..", "testdata", "enrichers", "risk_v2.hbs")

	img := Image{
		Vulnerabilities: []Vulnerability{
			{L1: 5},
			{L1: 7},
		},
		Namespace: "portal",
		Distro:    "alpine",
		Version:   "3.16",
	}

	outChan := make(chan any)
	go func() {
		outChan <- img
		close(outChan)
	}()

	mapper := extension.NewChanSource(outChan).
		Via(flow.NewMap(NewPureHbs[Image, RiskV2](templatePath), 1))
	result := <-mapper.Out()
	risk := result.(RiskV2)

	t.Logf("Risk: %d, Reason: %s", risk.Spec.Risk, risk.Spec.Reason)
}

func TestSingleHbs(t *testing.T) {
	type args struct {
		scriptPath string
		inputItem  model.PharosScanResult
	}

	tests := []struct {
		name string
		args args
		want map[string]interface{}
	}{
		{
			name: "Evaluate risk",
			args: args{
				scriptPath: "../../testdata/enrichers-flat/risk_v1.hbs",
				inputItem:  model.NewTestScanResult(model.NewTestScanTask(t, "test-1", "test-image-1"), "test-engine-1"),
			},
			want: map[string]interface{}{
				"vulnerabilities": []interface{}{"CVE-2023-44487"},
				"risk":            1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanResult := model.NewTestScanResult(model.NewTestScanTask(t, "test-1", "test-image-1"), "test-engine-1")
			outChan := make(chan any, 1)
			outChan <- map[string]interface{}{
				"payload": ToMap(scanResult),
			}
			close(outChan)

			mapper := extension.NewChanSource(outChan).
				Via(flow.NewMap(NewPureHbs[map[string]interface{}, map[string]interface{}](tt.args.scriptPath), 1))
			result := <-mapper.Out()
			got := result.(map[string]interface{})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Expected result to be %v, got %v", tt.want, got)
			}
		})
	}

}

func TestAppendFile(t *testing.T) {
	img := Image{
		Vulnerabilities: []Vulnerability{
			{L1: 5},
			{L1: 7},
		},
		Namespace: "portal",
		Distro:    "alpine",
		Version:   "3.16",
	}

	outChan := make(chan any)
	go func() {
		outChan <- map[string]interface{}{
			"payload": ToMap(img),
		}
		close(outChan)
	}()

	// Use filepath.Join to create a platform-independent path to the test data file
	eosYamlPath := filepath.Join("..", "..", "testdata", "enrichers", "risk", "eos.yaml")
	templatePath := filepath.Join("..", "..", "testdata", "enrichers", "risk", "eos_v1.hbs")

	mapper := extension.NewChanSource(outChan).
		Via(flow.NewMap(NewAppendFile(eosYamlPath), 1)).
		//Via(flow.NewMap(NewDebug("eos"), 1)).
		Via(flow.NewMap(NewPureHbs[map[string]interface{}, map[string]interface{}](templatePath), 1))

	result := (<-mapper.Out()).(map[string]interface{})

	t.Logf("Result keys: %v", getMapKeys(result))

	// Assert that the result contains the expected structure
	spec, ok := result["spec"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to contain 'spec' as map[string]interface{}, got %T", result["spec"])
	}

	// Assert that spec contains the eos field
	_, ok = spec["eos"]
	if !ok {
		t.Fatalf("Expected spec to contain 'eos' field, got keys: %v", getMapKeys(spec))
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

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		criteria string
		expected bool
	}{
		{"Exact match", "3.16.0", "3.16.0", true},
		{"No match", "3.16.0", "3.16.1", false},
		{"Wildcard at end", "3.16.%", "3.16.2", true},
		{"Wildcard at end - no match", "3.16.%", "3.17.0", false},
		{"Wildcard in middle", "3.%.0", "3.16.0", true},
		{"Wildcard in middle - no match", "3.%.0", "3.16.1", false},
		{"Multiple wildcards", "3.%.%", "3.16.2", true},
		{"Multiple wildcards - no match", "3.%.%", "4.16.2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a map with the pattern
			item := map[string]interface{}{
				"version": tt.pattern,
			}

			// Call the matchWildcard function
			result := filterOperators["matchWildcard"](item, "version", tt.criteria)

			// Check the result
			if result != tt.expected {
				t.Errorf("matchWildcard(%q, %q) = %v, want %v",
					tt.pattern, tt.criteria, result, tt.expected)
			}
		})
	}

	// Test with Alpine version patterns from eos.yaml
	t.Run("Alpine version patterns", func(t *testing.T) {
		alpinePatterns := []struct {
			pattern string
			version string
			matches bool
		}{
			{"3.16.%", "3.16.0", true},
			{"3.16.%", "3.16.1", true},
			{"3.16.%", "3.16.10", true},
			{"3.16.%", "3.17.0", false},
			{"3.20.%", "3.20.3", true},
			{"3.20.%", "3.20.0", true},
			{"3.20.%", "3.20.10", true},
			{"3.20.%", "3.21.0", false},
		}

		for _, tc := range alpinePatterns {
			item := map[string]interface{}{
				"version": tc.pattern,
			}

			result := filterOperators["matchWildcard"](item, "version", tc.version)

			if result != tc.matches {
				t.Errorf("matchWildcard with pattern %q and version %q = %v, want %v",
					tc.pattern, tc.version, result, tc.matches)
			}
		}
	})
}
