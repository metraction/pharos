package trivy

import (
	"encoding/json"
	"time"
)

// trivy version
type TrivyVersion struct {
	Version         string `json:"version"`
	VulnerabilityDb struct {
		Version      int       `json:"version"`
		NextUpdate   time.Time `json:"NextUpdate"`
		UpdatedAt    time.Time `json:"UpdatedAt"`
		DownloadedAt time.Time `json:"DownloadedAt"`
	} `json:"VulnerabilityDB"`
}

func (rx *TrivyVersion) FromBytes(input []byte) error {
	err := json.Unmarshal(input, &rx)
	if err != nil {
		return err
	}
	return nil
}
