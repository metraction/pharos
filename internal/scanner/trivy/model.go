package trivy

import (
	"encoding/json"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/samber/lo"
)

// SBOM accessors for nexted properties
func GetToolVersion(sbom *cdx.BOM) string {
	values := lo.Map(
		*sbom.Metadata.Tools.Components,
		func(x cdx.Component, k int) string { return x.Version },
	)

	return lo.FirstOr(values, "")
}

//

type TrivyScanType struct {
	SchemaVersion int       `json:"SchemaVersion"`
	CreatedAt     time.Time `json:"CreatedAt"`

	ArtifactName string `json:"ArtifactName"`
	ArtifactType string `json:"ArtifactType"`

	Metadata TrivyMetadata `json:"Metadata"`
	Results  []TrivyResult `json:"Results"`
}

// parse trivy scan
func (rx *TrivyScanType) ReadBytes(data []byte) error {
	if err := json.Unmarshal(data, &rx); err != nil {
		return err
	}
	return nil
}

// return complete list of vulnerabilities
func (rx *TrivyScanType) ListVulnerabilities() []TrivyVulnerability {
	vulns := []TrivyVulnerability{}

	for _, result := range rx.Results {
		vulns = append(vulns, result.Vulnerabilities...)
	}
	return vulns
}

// `json:""`
type TrivyMetadata struct {
	ImageId string `json:"ImageID"`
	//Size    uint64 `json:"Size"`
	OS struct {
		Famile string `json:"Family"`
		Name   string `json:"Name"`
		EOSL   bool   `json:"EOSL"`
	} `json:"OS"`
	RepoTags    []string         `json:"RepoTags"`
	RepoDigests []string         `json:"RepoDigests"`
	ImageConfig TrivyImageConfig `json:"ImageConfig"`
	Layers      []TrivyLayer     `json:"Layers"`
}

type TrivyLayer struct {
	Size   uint64 `json:"Size"`
	DiffId string `json:"DiffID"`
}
type TrivyImageConfig struct {
	Architecture string    `json:"architecture"`
	Created      time.Time `json:"created"`
	OS           string    `json:"os"`
	Rootfs       struct {
		Type string `json:"type"`
	} `json:"rootfs"`
}

type TrivyResult struct {
	Target          string               `json:"Target"`
	Class           string               `json:"Class"`
	Type            string               `json:"Type"`
	Vulnerabilities []TrivyVulnerability `json:"Vulnerabilities"`
}

type TrivyVulnerability struct {
	VulnerabilityId  string              `json:"VulnerabilityID"`
	PkgId            string              `json:"PkgID"`
	PkgName          string              `json:"PkgName"`
	PkgIdentifier    TrivyPkgIdentifier  `json:"PkgIdentifier"`
	InstalledVersion string              `json:"InstalledVersion"`
	FixedVersion     string              `json:"FixedVersion"`
	Status           string              `json:"Status"`
	SeveritySource   string              `json:"SeveritySource"`
	PrimaryURL       string              `json:"PrimaryURL"`
	DataSource       TrivyDataSource     `json:"DataSource"`
	Title            string              `json:"Title"`
	Description      string              `json:"Description"`
	Severity         string              `json:"Severity"`
	CweIds           []string            `json:"CweIDs"`
	VendorSecurity   TrivyVendorSevurity `json:"VendorSeverity"`
	Cvss             TrivyCvss           `json:"CVSS"`
	References       []string            `json:"References"`
	PublishedDate    time.Time           `json:"PublishedDate"`
	LastModifiedDate time.Time           `json:"LastModifiedDate"`
}

type TrivyPkgIdentifier struct {
	Purl   string `json:"PURL"`
	Uid    string `json:"UID"`
	BomRef string `json:"BOMRef"`
}

type TrivyDataSource struct {
	Id   string `json:"ID"`
	Name string `json:"Name"`
	Url  string `json:"URL"`
}

type TrivyVendorSevurity struct {
	Azure      int `json:"azure"`
	CblMariner int `json:"cbl-mariner"`
	NVD        int `json:"nvd"`
	RedHat     int `json:"redhat"`
	Ubuntu     int `json:"ubuntu"`
}

// `json:""`
type TrivyCvss struct {
	Azure      TrivyCvssEntry `json:"azure"`
	CblMariner TrivyCvssEntry `json:"cbl-mariner"`
	Nvd        TrivyCvssEntry `json:"nvd"`
	RedHat     TrivyCvssEntry `json:"redhat"`
	Ubuntu     TrivyCvssEntry `json:"ubuntu"`
}
type TrivyCvssEntry struct {
	V2Vector string  `json:"V2Vector"`
	V2Score  float64 `json:"V2Score"`

	V3Vector string  `json:"V3Vector"`
	V3Score  float64 `json:"V3Score"`

	V4Vector string  `json:"V4Vector"`
	V4Score  float64 `json:"V4Score"`
}
