package grype

import (
	"encoding/json"
	"fmt"
	"time"
)

var ErrorGrypeNoLocalDatabase = fmt.Errorf("no local grype database found")

// local db state from: grype db check -o json
// hint: remote db state: https://grype.anchore.io/databases/v6/latest.json
type GrypeLocalDbState struct {
	SchemaVersion string    `json:"schemaVersion"`
	From          string    `json:"from"`
	Built         time.Time `json:"built"`
	Path          string    `json:"path"`
	Valid         bool      `json:"valid"`
	Error         string    `json:"error"`
}

// parse from stdout bytes
func (rx *GrypeLocalDbState) FromBytes(input []byte) error {
	err := json.Unmarshal(input, &rx)
	if err != nil {
		return err
	}
	return nil
}

// grype version
type GrypeVersion struct {
	Application       string    `json:"application"`
	BuildDate         time.Time `json:"buildDate"`
	Platform          string    `json:"platform"`
	SupportedDbSchema string    `json:"supportedDbSchema"`
	GrypeVersion      string    `json:"version"`
	SyftVersion       string    `json:"syftVersion"`
}

func (rx *GrypeVersion) FromBytes(input []byte) error {
	err := json.Unmarshal(input, &rx)
	if err != nil {
		return err
	}
	return nil
}
