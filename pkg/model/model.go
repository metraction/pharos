package model

import (
	"encoding/json"
	"time"

	"github.com/metraction/pharos/internal/utils"
)

// hold results if images scans returned from a variety of scanner engines
// Update: Stefan 2025-06-29
// Context and scanner info is in ScanTask
type PharosScanResult struct {
	Version         string                `json:"Version"`
	ScanTask        PharosScanTask2       `json:"ScanTask"`
	ScanMeta        PharosScanMeta        `json:"ScanMeta"` // scanner info in scan task
	Image           PharosImageMeta       `json:"Image"`
	Findings        []PharosScanFinding   `json:"Findings"`        // instatiation of vulnerabilities in packages
	Vulnerabilities []PharosVulnerability `json:"Vulnerabilities"` // vulnerabilities found with vuln metadata (description, CVSS, ..)
	Packages        []PharosPackage       `json:"Packages"`
}

// scanner engine and scan execution metadata
type PharosScanMeta struct {
	Engine        string        `json:"Engine"`
	EngineVersion string        `json:"EngineVersion"`
	ScanDate      time.Time     `json:"ScanDate"`
	DbBuiltDate   time.Time     `json:"DbBuiltDate"`
	ScanElapsed   time.Duration `json:"ScanElapsed"`
}

func (rx *PharosScanResult) SetTaskStatus(status string) *PharosScanResult {
	rx.ScanTask.Status = status
	return rx
}

func (rx *PharosScanResult) SetTaskError(err error) *PharosScanResult {
	rx.ScanTask.SetError(err)
	return rx
}
func (rx *PharosScanResult) SetScanElapsed(elapsed time.Duration) *PharosScanResult {
	rx.ScanMeta.ScanElapsed = elapsed
	return rx
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
	Name     string    `json:"Name"`
	Version  string    `json:"Version"`
	ScanTime time.Time `json:"ScanTime"`
}

// metadata about the asset (image, code, vm, ..)
// digest naming is confusing between products.
// ImageId:        uique Id per image assigned by scanner (this digest is not found with scopeo inspect, unknown how it is created)
// ManifestDigest: unique digest per image and platform (we should use the manifestDigest as cache key and to correlte in db)
// IndexDigest:    digest per image, stays the same accross platforms
// RepoDigests:	   aribtrarily ordered list of digests reported by scanner

type PharosImageMeta struct {
	ImageSpec      string      `json:"ImageSpec" required:"true" doc:"image url, e.g. docker.io/nginx:latest"` // scan input / image uri
	ImageId        string      `json:"ImageId" gorm:"primaryKey" hidden:"true" doc:"internal image ID, e.g. sha256:1234.."`
	IndexDigest    string      `json:"IndexDigest" required:"true"` // internal ID for cache
	ManifestDigest string      `json:"ManifestDigest" required:"false"`
	RepoDigests    StringSlice `json:"RepoDigests" required:"false" gorm:"type:VARCHAR"`
	ArchName       string      `json:"ArchName" required:"false" doc:"image platform architecture default: amd64"` // image platform architecture amd64/..
	ArchOS         string      `json:"ArchOS" required:"false" doc:"image platform OS default: linux"`             // image platform OS
	DistroName     string      `json:"DistroName" required:"false"`
	DistroVersion  string      `json:"DistroVersion" required:"false"`
	Size           uint64      `json:"Size" required:"false"`
	Tags           StringSlice `json:"Tags" gorm:"type:VARCHAR" required:"false"`
	Layers         StringSlice `json:"Layers" gorm:"type:VARCHAR" required:"false"`

	// TODO: The below are ORL artifacts and must be removed from this struct
	// -> Context is in PharosScanTask
	// -> TTLs are in PharosScanTask
	// -> Vulns/Packages/Findings are in PharosResult
	Vulnerabilities    []PharosVulnerability `json:"Vulnerabilities" required:"false" gorm:"many2many:join_pharos_vulnerability_with_pharos_image_meta;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Findings           []PharosScanFinding   `json:"Findings" required:"false" gorm:"many2many:join_pharos_scan_finding_with_pharos_image_meta;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Packages           []PharosPackage       `json:"Packages" required:"false" gorm:"many2many:join_pharos_package_with_pharos_image_meta;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	ContextRoots       []ContextRoot         `json:"ContextRoots" required:"false" gorm:"foreignKey:ImageId;references:ImageId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	TTL                time.Duration         `json:"TTL" required:"false" gorm:"default:43200000000"` // context root TTL, default 12 hours (43200 seconds)
	LastSuccessfulScan time.Time             `json:"LastSuccessfulScan"`                              // last update time
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
