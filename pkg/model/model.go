package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// hold results if images scans returned from a variety of scanner engines
type PharosScanResult struct {
	Version    string           `json:"Version"`
	ScanTask   PharosScanTask   `json:"ScanTask"`
	ScanEngine PharosScanEngine `json:"ScanEngine"` // scanner & scan metadata
	// Add Scantask

	Image           PharosImageMeta       `json:"Image"`
	Findings        []PharosScanFinding   `json:"Findings"`        // instatiation of vulnerabilities in packages
	Vulnerabilities []PharosVulnerability `json:"Vulnerabilities"` // vulnerabilities found with vuln metadata (description, CVSS, ..)
	Packages        []PharosPackage       `json:"Packages"`
	// Context
}

func (rx *PharosScanResult) SetStatus(status string) PharosScanResult {
	rx.ScanTask.SetStatus(status)
	return *rx
}
func (rx *PharosScanResult) SetError(err error) PharosScanResult {
	rx.ScanTask.Error = err.Error()
	return *rx
}

type StringSlice []string

func (ss *StringSlice) Scan(src any) error {
	str, ok := src.(string)
	if !ok {
		return errors.New("src value cannot cast to string")
	}
	*ss = strings.Split(str, ",")
	return nil
}
func (ss StringSlice) Value() (driver.Value, error) {
	if len(ss) == 0 {
		return nil, nil
	}
	return strings.Join(ss, ","), nil
}

// scan metadata to identify scanner tool and versions
// (this is importan once we have a variety of scanners)
type PharosScanEngine struct {
	Name     string    `json:"Name"`
	Version  string    `json:"Version"`
	ScanTime time.Time `json:"ScanTime"`
}

// metadata about the asset (image, code, vm, ..)
type PharosImageMeta struct {
	ImageSpec       string                `json:"ImageSpec" required:"true" doc:"image url, e.g. docker.io/nginx:latest"` // scan input / image uri
	ImageId         string                `json:"ImageId" gorm:"primaryKey" hidden:"true" doc:"internal image ID, e.g. sha256:1234.."`
	IndexDigest     string                `json:"IndexDigest" required:"true"` // internal ID for cache
	ManifestDigest  string                `json:"ManifestDigest" required:"false"`
	RepoDigests     StringSlice           `json:"RepoDigests" required:"false" gorm:"type:VARCHAR"`
	ArchName        string                `json:"ArchName" required:"false" doc:"image platform architecture default: amd64"` // image platform architecture amd64/..
	ArchOS          string                `json:"ArchOS" required:"false" doc:"image platform OS default: linux"`             // image platform OS
	DistroName      string                `json:"DistroName" required:"false"`
	DistroVersion   string                `json:"DistroVersion" required:"false"`
	Size            uint64                `json:"Size" required:"false"`
	Tags            StringSlice           `json:"Tags" gorm:"type:VARCHAR" required:"false"`
	Layers          StringSlice           `json:"Layers" gorm:"type:VARCHAR" required:"false"`
	Vulnerabilities []PharosVulnerability `json:"Vulnerabilities" required:"false" gorm:"many2many:join_pharos_vulnerability_with_pharos_image_meta;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Findings        []PharosScanFinding   `json:"Findings" required:"false" gorm:"many2many:join_pharos_scan_finding_with_pharos_image_meta;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Packages        []PharosPackage       `json:"Packages" required:"false" gorm:"many2many:join_pharos_package_with_pharos_image_meta;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

type PharosFindingSummary struct {
	Severities map[string]int `json:"Severities"` // map of severity to count
}

func (pm *PharosImageMeta) GetSummary() PharosFindingSummary {
	severities := make(map[string]int)
	for _, finding := range pm.Findings {
		severities[finding.Severity]++
	}
	return PharosFindingSummary{
		Severities: severities,
	}
}

// a finding is an instantiation of a vulnerability in an asset/package (scan result)
type PharosScanFinding struct {
	AdvId       string      `json:"AdvId" gorm:"primaryKey"`     // finding CVE, GHSA, ..
	AdvSource   string      `json:"AdvSource" gorm:"primaryKey"` // advisory source, like NVD, GItHub, Uuntu
	ScanDate    time.Time   `json:"ScanDate"`                    // finding first found
	UpdateDate  time.Time   `json:"UpdateDate"`                  // finding updated/last scan
	Severity    string      `json:"Severity"`
	DueDate     time.Time   `json:"DueDate"` // needs to be fixed by
	FixState    string      `json:"FixState"`
	FixVersions StringSlice `json:"FixVersions" gorm:"type:VARCHAR"`
	FoundIn     StringSlice `json:"FoundIn" gorm:"type:VARCHAR"` // Paths of vulnerable artifacts
}

// a vulnerability is generic description of a weakness, a scan finds vulns in packages
type PharosVulnerability struct {
	AdvId          string      `json:"AdvId" gorm:"primaryKey"`     // finding CVE, GHSA, ..
	AdvSource      string      `json:"AdvSource" gorm:"primaryKey"` // advisory source, like NVD, GItHub, Ubuntu
	AdvAliases     string      `json:"Aliases"`
	CreateDate     time.Time   `json:"CreateDate"` // finding first found
	PubDate        time.Time   `json:"PubDate"`    // vuln publication
	ModDate        time.Time   `json:"ModDate"`    // last modified
	KevDate        time.Time   `json:"KevDate"`    // known exploited in wild pubdate)
	Severity       string      `json:"Severity"`
	CvssVectors    StringSlice `json:"CvssVectors" gorm:"type:VARCHAR"`
	CvssBase       float64     `json:"CvssBase"`                       // max cvss score
	RiskScoce      float64     `json:"RiskScore"`                      // from grype
	Cpes           StringSlice `json:"Cpes" gorm:"type:VARCHAR"`       // Mitre CPEs
	Cwes           StringSlice `json:"Cwes" gorm:"type:VARCHAR"`       // Mitre CWEs
	References     StringSlice `json:"References" gorm:"type:VARCHAR"` // external references
	RansomwareUsed string      `json:"RansomwareUsed"`                 // Exploit used in ransomware
	Description    string      `json:"Description"`
}

// sbom packages
type PharosPackage struct {
	Key     string      `json:"Key" gorm:"primaryKey"` // unique key to deduplicate packages
	Name    string      `json:"Name"`
	Version string      `json:"Version"`
	Type    string      `json:"Type"`
	Purl    string      `json:"Purl"`
	Cpes    StringSlice `json:"Cpes" gorm:"type:VARCHAR"`
}

// return model as []byte
func (rx *PharosScanResult) ToBytes() []byte {
	data, err := json.Marshal(rx)
	if err != nil {
		return []byte{}
	}
	return data
}
