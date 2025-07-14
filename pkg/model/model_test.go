package model

import (
	"path/filepath"
	"testing"
)

// TestLoadResultFromFile verifies that LoadResultFromFile correctly loads a JSON encoded PharosScanResult from disk.
func TestLoadResultFromFile(t *testing.T) {
	// Load reference YAML file shipped with the repository.
	yamlPath := filepath.Join("..", "..", "kodata", "enrichers", "test-data.yaml")
	// Invoke the function under test.
	result, err := LoadResultFromFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadResultFromFile failed: %v", err)
	}
	if result == nil {
		t.Fatalf("LoadResultFromFile returned nil, expected valid result")
	}
	// Simple sanity checks – version and at least one finding.
	if result.Version == "" {
		t.Errorf("expected Version to be populated, got empty string")
	}
	if len(result.Findings) == 0 {
		t.Errorf("expected Findings to be populated from YAML, got 0 entries")
	}
}
