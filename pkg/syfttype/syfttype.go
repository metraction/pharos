package syfttype

import (
	"encoding/json"

	"github.com/metraction/pharos/pkg/grypetype"
)

// output of grype -o json
type SyftSbomType struct {
	Source     SyftSource     `json:"source"`
	Distro     SyftDistro     `json:"distro"`
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

// parse []bytes to model
func (rx *SyftSbomType) FromBytes(input []byte) error {
	err := json.Unmarshal(input, &rx)
	if err != nil {
		return err
	}
	return nil
}

type SyftSource struct {
	Name     string       `json:"name"`
	Version  string       `json:"version"`
	Metadata SyftMetadata `json:"metadata"`
}

type SyftMetadata struct {
	UserInput string `json:"userInput"`
	ImageId   string `json:"imageId"`
	ImageSize uint64 `json:"imageSize"`
}

type SyftDistro struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// `json:""`
type SyftArtifact struct {
	Id        string                    `json:"id"`
	Name      string                    `json:"name"`
	Version   string                    `json:"version"`
	Type      string                    `json:"type"`
	FoundBy   string                    `json:"foundBy"`
	Language  string                    `json:"language"`
	Purl      string                    `json:"purl"`
	Cpes      []SyftCpe                 `json:"cpes"`
	Locations []grypetype.GrypeLocation `json:"location"`
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
