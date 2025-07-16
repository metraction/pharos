package model

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadEnrichersFromFile loads an Enrichers configuration from a YAML file.
// The YAML file can either contain a direct Enrichers struct or have it nested under an "enrichers" key.
func LoadEnrichersFromFile(path string) (*Enrichers, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read enrichers file: %w", err)
	}
	dir := filepath.Dir(path)
	enrichers, err := loadEnrichersFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize enrichers: %w", err)
	}
	for i := range enrichers.Sources {
		if enrichers.Sources[i].Path == "" {
			enrichers.Sources[i].Path = enrichers.Sources[i].Name
		}
		if !filepath.IsAbs(enrichers.Sources[i].Path) {
			enrichers.Sources[i].Path = filepath.Join(dir, enrichers.Sources[i].Path)
		}
	}
	return enrichers, nil
}

// loadEnrichersFromBytes unmarshals the given bytes into an Enrichers struct.
// It handles both direct Enrichers struct and one nested under an "enrichers" key.
func loadEnrichersFromBytes(data []byte) (*Enrichers, error) {
	// First try to unmarshal directly into Enrichers struct
	var enrichers Enrichers
	err := yaml.Unmarshal(data, &enrichers)

	// If we have valid data (at least one of Order or Sources is non-empty), return it
	if err == nil && (len(enrichers.Order) > 0 || len(enrichers.Sources) > 0) {
		return &enrichers, nil
	}

	// If direct unmarshaling fails or results in empty struct, try with wrapper
	var wrapper struct {
		Enrichers Enrichers `yaml:"enrichers"`
	}

	err = yaml.Unmarshal(data, &wrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to parse enrichers file: %w", err)
	}

	// Check if we got any data
	if len(wrapper.Enrichers.Order) == 0 && len(wrapper.Enrichers.Sources) == 0 {
		return nil, fmt.Errorf("no valid enrichers configuration found in file")
	}

	return &wrapper.Enrichers, nil
}
