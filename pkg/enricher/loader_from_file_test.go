package enricher

import (
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

	if len(enrichers.Sources) != 4 {
		t.Errorf("Expected 4 items in Sources, got %d", len(enrichers.Sources))
	}
	if enrichers.Sources[0].Name != "eos" || enrichers.Sources[1].Name != "owner" {
		t.Errorf("Source names don't match expected values")
	}
}
