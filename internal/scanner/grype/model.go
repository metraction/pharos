package grype

import (
	"encoding/json"
	"os"
	"time"
)

// output of grype -o json
type GrypeScanType struct {
	Type       string           `json:"type"`
	Matches    []GrypeMatch     `json:"matches"`
	Distro     GrypeDistro      `json:"distro"`
	Descriptor GrypeDescriptor  `json:"descriptor"`
	Source     GrypeSourceImage `json:"source"` // code/image scan fill different fields
}

// // used to unmarshal image scans
type GrypeImageType struct {
	Matches    []GrypeMatch     `json:"matches"`
	Distro     GrypeDistro      `json:"distro"`
	Descriptor GrypeDescriptor  `json:"descriptor"`
	Source     GrypeSourceImage `json:"source"`
}

// used to unmarshal code scans
type GrypeCodeType struct {
	Matches    []GrypeMatch    `json:"matches"`
	Distro     GrypeDistro     `json:"distro"`
	Descriptor GrypeDescriptor `json:"descriptor"`
	Source     GrypeSourceCode `json:"source"`
}

// image target type
type GrypeSourceImage struct {
	Type       string      `json:"type"`
	TargetPath string      `json:"path"`
	Target     GrypeTarget `json:"target"`
}

// code target type
type GrypeSourceCode struct {
	Type   string `json:"type"`
	Target string `json:"target"`
}

func (rx *GrypeScanType) ReadJson(infile string) error {
	data, err := os.ReadFile(infile)
	if err != nil {
		return err
	}
	return rx.ReadBytes(data)
}

// parse grype scan input, handle both code and image scan outputs
func (rx *GrypeScanType) ReadBytes(data []byte) error {

	var err error

	// attempt to parse image scan
	var imageScan *GrypeImageType
	if err = json.Unmarshal(data, &imageScan); err == nil {
		rx.Type = imageScan.Source.Type
		rx.Matches = imageScan.Matches
		rx.Distro = imageScan.Distro
		rx.Descriptor = imageScan.Descriptor
		rx.Source = imageScan.Source
		return nil
	}
	// attempt to parse code scan
	var codeScan *GrypeCodeType
	if err = json.Unmarshal(data, &codeScan); err == nil {
		rx.Type = codeScan.Source.Type
		rx.Matches = codeScan.Matches
		rx.Distro = codeScan.Distro
		rx.Descriptor = codeScan.Descriptor
		rx.Source.Type = codeScan.Source.Type
		rx.Source.TargetPath = codeScan.Source.Target
		return nil
	}
	return err
}

type GrypeMatch struct {
	Vulnerability GrypeVulnerability `json:"vulnerability"`
	Artifact      GrypeArtifact      `json:"artifact"`
}
type GrypeVulnerability struct {
	Id             string                `json:"id"`
	Severity       string                `json:"severity"`
	Description    string                `json:"description"`
	Namespace      string                `json:"namespace"`
	Cvss           []GrypeCvss           `json:"cvss"`
	Epss           []GrypeEpss           `json:"epss"`
	KnownExplpited []GrypeKnownExploited `json:"knownExploited"`

	Fix        GrypeFix        `json:"fix"`
	Advisories []GrypeAdvisory `json:"advisories"`
	Risk       float64         `json:"risk"`
}
type GrypeAdvisory struct {
	Id   string `json:"id"`
	Link string `json:"link"`
}
type GrypeArtifact struct {
	Id        string          `json:"id"`
	Name      string          `json:"name"`
	Version   string          `json:"version"`
	Type      string          `json:"type"`
	Locations []GrypeLocation `json:"locations"`
	Purl      string          `json:"purl"`
}
type GrypeLocation struct {
	Path       string `json:"path"`
	LayerId    string `json:"layerID"`
	AccessPath string `json:"accesPath"`
}
type GrypeFix struct {
	Versions []string `json:"versions"`
	State    string   `json:"state"`
}

type GrypeCvss struct {
	Source  string `json:"source"`
	Type    string `json:"type"`
	Version string `json:"version"`
	Vector  string `json:"vector"`
	Metrics struct {
		BaseScore   float64 `json:"baseScore"`
		ExpoitScore float64 `json:"exploitabilityScore"`
		ImpactScore float64 `json:"impactScore"`
	} `json:"metrics"`
}

type GrypeEpss struct {
	Cve        string  `json:"cve"`
	Epss       float64 `json:"epss"`
	Percentile float64 `json:"percentile"`
	Date       string  `json:"date"`
}

type GrypeKnownExploited struct {
	Cve            string   `json:"cve"`
	Vendor         string   `json:"vendor"`
	Product        string   `json:"product"`
	DateAdded      string   `json:"dateAdded"`
	Action         string   `json:"requiredAction"`
	RansomwareUsed string   `json:"knownRansomwareCampaignUse"`
	Urls           []string `json:"urls"`
}
type GrypeTarget struct {
	UserInput      string       `json:"userInput"`
	ImageId        string       `json:"imageId"`
	ManifestDigest string       `json:"manifestDigest"`
	MediaType      string       `json:"mediaType"`
	Tags           []string     `json:"tags"`
	ImageSize      uint64       `json:"imageSize"`
	RepoDigests    []string     `json:"repoDigests"`
	Architecture   string       `json:"architecture"`
	OS             string       `json:"os"`
	Layers         []GrypeLayer `json:"layers"`
}
type GrypeLayer struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}
type GrypeDistro struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type GrypeDescriptor struct {
	Name     string    `json:"name"`
	Version  string    `json:"version"`
	ScanTime time.Time `json:"timestamp"`
}
