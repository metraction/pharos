package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/metraction/pharos/internal/utils"
	"gopkg.in/yaml.v3"
)

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

// hold results if images scans returned from a variety of scanner engines
// Update: Stefan 2025-06-29
// Context and scanner info is in ScanTask
type PharosScanResult struct {
	Version  string          `json:"Version" yaml:"Version"`
	ScanTask PharosScanTask2 `json:"ScanTask" yaml:"ScanTask"`

	//ScanEngine PharosScanEngine `json:"ScanEngine"` // scanner info in scan task

	Image           PharosImageMeta       `json:"Image" yaml:"Image"`
	Findings        []PharosScanFinding   `json:"Findings" yaml:"Findings"`               // instatiation of vulnerabilities in packages
	Vulnerabilities []PharosVulnerability `json:"Vulnerabilities" yaml:"Vulnerabilities"` // vulnerabilities found with vuln metadata (description, CVSS, ..)
	Packages        []PharosPackage       `json:"Packages" yaml:"Packages"`
}

// ContextRootKey string // Composite Foreign Key to the ContextRoot Table
// ImageId        string // Composite Foreign Key to the ContextRoot Table
// Owner          string // The owner of the Context, this is the plugin that has created / changed it. Will be a Foreign Key to the Plugins Table
// UpdatedAt      time.Time
// Data           map[string]any `gorm:"serializer:json"` // Context data

func (rx *PharosScanResult) GetContextRoot(owner string, ttl time.Duration) ContextRoot {
	return ContextRoot{
		Key:       rx.ScanTask.ContextRootKey,
		ImageId:   rx.Image.ImageId,
		UpdatedAt: time.Now(),
		TTL:       ttl, // 25 minutes
		Contexts: []Context{
			{
				ContextRootKey: rx.ScanTask.ContextRootKey,
				ImageId:        rx.Image.ImageId,
				Owner:          owner,
				UpdatedAt:      time.Now(),
				Data:           rx.ScanTask.Context,
			},
		},
	}
}

// mask auth info in scantask (e.g. before submitting results)
func (rx *PharosScanResult) MaskAuth() PharosScanResult {
	rx.ScanTask.AuthDsn = utils.MaskDsn(rx.ScanTask.AuthDsn)
	return *rx
}

// scan metadata to identify scanner tool and versions
// (this is importan once we have a variety of scanners)
type Delete_PharosScanEngine struct {
	Name     string    `json:"Name" yaml:"Name"`
	Version  string    `json:"Version" yaml:"Version"`
	ScanTime time.Time `json:"ScanTime" yaml:"ScanTime"`
}

// metadata about the asset (image, code, vm, ..)
type PharosImageMeta struct {
	ImageSpec          string                `json:"ImageSpec" yaml:"ImageSpec" required:"true" doc:"image url, e.g. docker.io/nginx:latest"` // scan input / image uri
	ImageId            string                `json:"ImageId" yaml:"ImageId" gorm:"primaryKey" hidden:"true" doc:"internal image ID, e.g. sha256:1234.."`
	IndexDigest        string                `json:"IndexDigest" yaml:"IndexDigest" required:"true" gorm:"index"` // internal ID for cache
	ManifestDigest     string                `json:"ManifestDigest" yaml:"ManifestDigest" required:"false" gorm:"index"`
	RepoDigests        StringSlice           `json:"RepoDigests" yaml:"RepoDigests" required:"false" gorm:"type:VARCHAR"`
	ArchName           string                `json:"ArchName" yaml:"ArchName" required:"false" doc:"image platform architecture default: amd64" gorm:"index"` // image platform architecture amd64/..
	ArchOS             string                `json:"ArchOS" yaml:"ArchOS" required:"false" doc:"image platform OS default: linux" gorm:"index"`               // image platform OS
	DistroName         string                `json:"DistroName" yaml:"DistroName" required:"false"`
	DistroVersion      string                `json:"DistroVersion" yaml:"DistroVersion" required:"false"`
	Size               uint64                `json:"Size" yaml:"Size" required:"false"`
	Tags               StringSlice           `json:"Tags" yaml:"Tags" gorm:"type:VARCHAR" required:"false"`
	Layers             StringSlice           `json:"Layers" yaml:"Layers" gorm:"type:VARCHAR" required:"false"`
	Vulnerabilities    []PharosVulnerability `json:"Vulnerabilities" yaml:"Vulnerabilities" required:"false" gorm:"many2many:join_pharos_vulnerability_with_pharos_image_meta;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Findings           []PharosScanFinding   `json:"Findings" yaml:"Findings" required:"false" gorm:"many2many:join_pharos_scan_finding_with_pharos_image_meta;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Packages           []PharosPackage       `json:"Packages" yaml:"Packages" required:"false" gorm:"many2many:join_pharos_package_with_pharos_image_meta;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	ContextRoots       []ContextRoot         `json:"ContextRoots" yaml:"ContextRoots" required:"false" gorm:"foreignKey:ImageId;references:ImageId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	TTL                time.Duration         `json:"TTL" yaml:"TTL" required:"false" gorm:"default:43200000000"`
	LastSuccessfulScan time.Time             `json:"LastSuccessfulScan" yaml:"LastSuccessfulScan"` // last update time
}

type PharosFindingSummary struct {
	Severities map[string]int `json:"Severities" yaml:"Severities"` // map of severity to count
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
	AdvId       string      `json:"AdvId" yaml:"AdvId" gorm:"primaryKey"`         // finding CVE, GHSA, ..
	AdvSource   string      `json:"AdvSource" yaml:"AdvSource" gorm:"primaryKey"` // advisory source, like NVD, GItHub, Uuntu
	ScanDate    time.Time   `json:"ScanDate" yaml:"ScanDate"`                     // finding first found
	UpdateDate  time.Time   `json:"UpdateDate" yaml:"UpdateDate"`                 // finding updated/last scan
	Severity    string      `json:"Severity" yaml:"Severity"`
	DueDate     time.Time   `json:"DueDate" yaml:"DueDate"` // needs to be fixed by
	FixState    string      `json:"FixState" yaml:"FixState"`
	FixVersions StringSlice `json:"FixVersions" yaml:"FixVersions" gorm:"type:VARCHAR"`
	FoundIn     StringSlice `json:"FoundIn" yaml:"FoundIn" gorm:"type:VARCHAR"` // Paths of vulnerable artifacts
}

// a vulnerability is generic description of a weakness, a scan finds vulns in packages
type PharosVulnerability struct {
	AdvId          string      `json:"AdvId" yaml:"AdvId" gorm:"primaryKey"`         // finding CVE, GHSA, ..
	AdvSource      string      `json:"AdvSource" yaml:"AdvSource" gorm:"primaryKey"` // advisory source, like NVD, GItHub, Ubuntu
	AdvAliases     string      `json:"Aliases" yaml:"Aliases"`
	CreateDate     time.Time   `json:"CreateDate" yaml:"CreateDate"` // finding first found
	PubDate        time.Time   `json:"PubDate" yaml:"PubDate"`       // vuln publication
	ModDate        time.Time   `json:"ModDate" yaml:"ModDate"`       // last modified
	KevDate        time.Time   `json:"KevDate" yaml:"KevDate"`       // known exploited in wild pubdate)
	Severity       string      `json:"Severity" yaml:"Severity"`
	CvssVectors    StringSlice `json:"CvssVectors" yaml:"CvssVectors" gorm:"type:VARCHAR"`
	CvssBase       float64     `json:"CvssBase" yaml:"CvssBase"`                         // max cvss score
	RiskScoce      float64     `json:"RiskScore" yaml:"RiskScore"`                       // from grype
	Cpes           StringSlice `json:"Cpes" yaml:"Cpes" gorm:"type:VARCHAR"`             // Mitre CPEs
	Cwes           StringSlice `json:"Cwes" yaml:"Cwes" gorm:"type:VARCHAR"`             // Mitre CWEs
	References     StringSlice `json:"References" yaml:"References" gorm:"type:VARCHAR"` // external references
	RansomwareUsed string      `json:"RansomwareUsed" yaml:"RansomwareUsed"`             // Exploit used in ransomware
	Description    string      `json:"Description" yaml:"Description"`
}

// sbom packages
type PharosPackage struct {
	Key     string      `json:"Key" yaml:"Key" gorm:"primaryKey"` // unique key to deduplicate packages
	Name    string      `json:"Name" yaml:"Name"`
	Version string      `json:"Version" yaml:"Version"`
	Type    string      `json:"Type" yaml:"Type"`
	Purl    string      `json:"Purl" yaml:"Purl"`
	Cpes    StringSlice `json:"Cpes" yaml:"Cpes" gorm:"type:VARCHAR"`
}

// return model as []byte
func (rx *PharosScanResult) ToBytes() []byte {
	data, err := json.Marshal(rx)
	if err != nil {
		return []byte{}
	}
	return data
}

func LoadResultFromFile(path string) (*PharosScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return loadResultFromBytes(data)
}

func loadResultFromBytes(data []byte) (*PharosScanResult, error) {
	var rx PharosScanResult
	err := yaml.Unmarshal(data, &rx)
	if err != nil {
		return nil, err
	}
	return &rx, nil
}
