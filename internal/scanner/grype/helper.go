package grype

import (
	"encoding/json"
	"fmt"
	"time"
)

var ErrorGrypeNoLocalDatabase = fmt.Errorf("no local grype database found")

// update url: https://grype.anchore.io/databases/v6/latest.json
// from: grype db check -o json
type GrypeDatabaseState struct {
	Current         CurrentDbState   `json:"currentDB"`
	Candidate       CandidateDbState `json:"candidateDB"`
	UpdateAvailable bool             `json:"updateAvailable"`
}
type CurrentDbState struct {
	SchemaVersion string    `json:"schemaVersion"`
	Built         time.Time `json:"built"`
}
type CandidateDbState struct {
	SchemaVersion string    `json:"schemaVersion"`
	Built         time.Time `json:"built"`
}

// parse text
func (rx *GrypeDatabaseState) FromBytes(input []byte) error {
	err := json.Unmarshal(input, &rx)
	if err != nil {
		return err
	}
	return nil
}
