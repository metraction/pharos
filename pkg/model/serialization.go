package model

import (
	"encoding/json"
)

// MarshalBinary implements encoding.BinaryMarshaler for PharosImageSpec
func (p PharosImageSpec) MarshalBinary() ([]byte, error) {
	return json.Marshal(p)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler for PharosImageSpec
func (p *PharosImageSpec) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, p)
}

// MarshalBinary implements encoding.BinaryMarshaler for PharosRepoAuth
func (p PharosRepoAuth) MarshalBinary() ([]byte, error) {
	return json.Marshal(p)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler for PharosRepoAuth
func (p *PharosRepoAuth) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, p)
}

// MarshalBinary implements encoding.BinaryMarshaler for PharosScanTask
func (p PharosScanTask) MarshalBinary() ([]byte, error) {
	return json.Marshal(p)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler for PharosScanTask
func (p *PharosScanTask) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, p)
}

// MarshalBinary implements encoding.BinaryMarshaler for PharosScanResult
func (p PharosScanResult) MarshalBinary() ([]byte, error) {
	return json.Marshal(p)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler for PharosScanResult
func (p *PharosScanResult) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, p)
}
