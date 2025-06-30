package model

import (
	"encoding/json"
)

// ToMap implements ConvertibleTo interface for PharosScanResult
func (p PharosScanResult) ToMap() (map[string]any, error) {
	// Convert the entire struct to JSON first
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	// Return a map with a single field containing the JSON data
	return map[string]any{
		"data": string(data),
	}, nil
}

// FromMap implements ConvertibleFrom interface for PharosScanResult
func (p *PharosScanResult) FromMap(values map[string]any) error {
	// Extract the JSON data from the map
	dataField, ok := values["data"]
	if !ok {
		return nil // No data to convert
	}

	// Convert to string and unmarshal
	dataStr, ok := dataField.(string)
	if !ok {
		return nil // Invalid data format
	}

	// Unmarshal the JSON data
	return json.Unmarshal([]byte(dataStr), p)
}

// ToMap implements ConvertibleTo interface for PharosScanResult
func (p PharosScanTask2) ToMap() (map[string]any, error) {
	// Convert the entire struct to JSON first
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	// Return a map with a single field containing the JSON data
	return map[string]any{
		"data": string(data),
	}, nil
}

// FromMap implements ConvertibleFrom interface for PharosScanResult
func (p *PharosScanTask2) FromMap(values map[string]any) error {
	// Extract the JSON data from the map
	dataField, ok := values["data"]
	if !ok {
		return nil // No data to convert
	}

	// Convert to string and unmarshal
	dataStr, ok := dataField.(string)
	if !ok {
		return nil // Invalid data format
	}

	// Unmarshal the JSON data
	return json.Unmarshal([]byte(dataStr), p)
}
