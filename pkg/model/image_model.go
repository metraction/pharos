package model

import (
	"encoding/json"
	"time"
)

type ImageScanTask struct {
}

// hold results if images scans returned from a variety of scanner engines
type PharosImageScanResult struct {
	Version    string           `json:"Version"`
	ScanEngine PharosScanEngine `json:"Scan"` // scanner & scan metadata
	// Add Scantask

	Image           PharosImageMeta       `json:"Image"`
	Findings        []PharosScanFinding   `json:"Findings"`        // instatiation of vulnerabilities in packages
	Vulnerabilities []PharosVulnerability `json:"Vulnerabilities"` // vulnerabilities found with vuln metadata (description, CVSS, ..)
	Packages        []PharosPackage       `json:"Packages"`
	// Context
}

// scan metadata to identify scanner tool and versions
// (this is importan once we have a variety of scanners)
type PharosScanEngine struct {
	Name     string    `json:"Name"`
	Version  string    `json:"Version"`
	ScanTime time.Time `json:"ScanTime"`
	Status   string    `json:"Status"`
}

// metadata about the asset (image, code, vm, ..)
type PharosImageMeta struct {
	ImageSpec      string   `json:"ImageSpec"` // scan input / image uri
	ImageId        string   `json:"ImageId"`
	Digest         string   `json:"Digest"` // internal ID for cache
	ManigestDigest string   `json:"ManifestDigest"`
	RepoDigests    []string `json:"RepoDigests"`
	ArchName       string   `json:"ArchName"` // image platform architecture amd64/..
	ArchOS         string   `json:"ArchOS"`   // image platform OS
	DistroName     string   `json:"DistroName"`
	DistroVersion  string   `json:"DistroVersion"`
	Size           uint64   `json:"Size"`
	Tags           []string `json:"Tags"`
	Layers         []string `json:"Layers"`
}

// a finding is an instantiation of a vulnerability in an asset/package (scan result)
type PharosScanFinding struct {
	AdvId       string    `json:"AdvId"`      // finding CVE, GHSA, ..
	AdvSource   string    `json:"AdvSource"`  // advisory source, like NVD, GItHub, Uuntu
	ScanDate    time.Time `json:"ScanDate"`   // finding first found
	UpdateDate  time.Time `json:"UpdateDate"` // finding updated/last scan
	Severity    string    `json:"Severity"`
	DueDate     time.Time `json:"DueDate"` // needs to be fixed by
	FixState    string    `json:"FixState"`
	FixVersions []string  `json:"FixVersions"`
	FoundIn     []string  `json:"FoundIn"` // Paths of vulnerable artifacts
}

// a vulnerability is generic description of a weakness, a scan finds vulns in packages
type PharosVulnerability struct {
	AdvId          string    `json:"AdvId"`     // finding CVE, GHSA, ..
	AdvSource      string    `json:"AdvSource"` // advisory source, like NVD, GItHub, Ubuntu
	AdvAliases     string    `json:"Aliases"`
	CreateDate     time.Time `json:"CreateDate"` // finding first found
	PubDate        time.Time `json:"PubDate"`    // vuln publication
	ModDate        time.Time `json:"ModDate"`    // last modified
	KevDate        time.Time `json:"KevDate"`    // known exploited in wild pubdate)
	Severity       string    `json:"Severity"`
	CvssVectors    []string  `json:"CvssVectors"`
	CvssBase       float64   `json:"CvssBase"`       // max cvss score
	RiskScoce      float64   `json:"RiskScore"`      // from grype
	Cpes           []string  `json:"Cpes"`           // Mitre CPEs
	Cwes           []string  `json:"Cwes"`           // Mitre CWEs
	References     []string  `json:"References"`     // external references
	RansomwareUsed string    `json:"RansomwareUsed"` // Exploit used in ransomware
	Description    string    `json:"Description"`
}

// sbom packages
type PharosPackage struct {
	Key     string   `json:"Key"` // unique key to deduplicate packages
	Name    string   `json:"Name"`
	Version string   `json:"Version"`
	Type    string   `json:"Type"`
	Purl    string   `json:"Purl"`
	Cpes    []string `json:"Cpes"`
}

// return model as []byte
func (rx *PharosImageScanResult) ToBytes() []byte {
	data, err := json.Marshal(rx)
	if err != nil {
		return []byte{}
	}
	return data
}
