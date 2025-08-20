package model

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnrichersFromFile(t *testing.T) {
	// Test with the existing enrichers.yaml file
	yamlPath := filepath.Join("..", "..", "testdata", "enrichers", "enrichers.yaml")
	enrichers, err := LoadEnrichersFromFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadEnrichersFromFile failed: %v", err)
	}
	if enrichers == nil {
		t.Fatalf("LoadEnrichersFromFile returned nil, expected valid result")
	}

	// Verify the content
	if len(enrichers.Order) != 2 {
		t.Errorf("Expected 2 items in Order, got %d", len(enrichers.Order))
	}
	if enrichers.Order[0] != "eos" || enrichers.Order[1] != "owner" {
		t.Errorf("Order items don't match expected values")
	}

	if len(enrichers.Sources) != 3 {
		t.Errorf("Expected 3 items in Sources, got %d", len(enrichers.Sources))
	}
	if enrichers.Sources[0].Name != "eos" || enrichers.Sources[1].Name != "owner" {
		t.Errorf("Source names don't match expected values")
	}

	// Test with a temporary file containing direct Enrichers structure
	tmpFile, err := os.CreateTemp("", "direct_enrichers_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	directContent := `order:
  - eos
  - owner
sources:
  - name: eos
  - name: owner
`
	if err := os.WriteFile(tmpFile.Name(), []byte(directContent), 0644); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	directEnrichers, err := LoadEnrichersFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadEnrichersFromFile failed with direct structure: %v", err)
	}
	if directEnrichers == nil {
		t.Fatalf("LoadEnrichersFromFile returned nil for direct structure")
	}

	// Verify the content of direct structure
	if len(directEnrichers.Order) != 2 {
		t.Errorf("Expected 2 items in Order for direct structure, got %d", len(directEnrichers.Order))
	}
	if len(directEnrichers.Sources) != 2 {
		t.Errorf("Expected 2 items in Sources for direct structure, got %d", len(directEnrichers.Sources))
	}
}
