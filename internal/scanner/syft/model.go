package syft

import (
	"encoding/json"

	"github.com/metraction/pharos/internal/scanner/grype"
)

// output of grype -o json
type SyftSbomType struct {
	Artifacts  []SyftArtifact `json:"artifacts"`
	Descriptor SyftDescriptor `json:"descriptor"`
}

// return model as []byte
func (rx *SyftSbomType) ToBytes() []byte {
	data, err := json.Marshal(rx)
	if err != nil {
		return []byte{}
	}
	return data
}

// `json:""`
type SyftArtifact struct {
	Id        string                `json:"id"`
	Name      string                `json:"name"`
	Version   string                `json:"version"`
	Type      string                `json:"type"`
	FoundBy   string                `json:"foundBy"`
	Language  string                `json:"language"`
	Purl      string                `json:"purl"`
	Cpes      []SyftCpe             `json:"cpes"`
	Locations []grype.GrypeLocation `json:"location"`
}

type SyftCpe struct {
	Cpe    string `json:"cpe"`
	Source string `json:"source"`
}

type SyftDescriptor struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type SyftFile struct {
}
