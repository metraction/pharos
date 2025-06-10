package model

import (
	"encoding/json"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/metraction/pharos/internal/scanner/grype"
	"github.com/metraction/pharos/internal/scanner/trivy"
	"github.com/metraction/pharos/internal/utils"
	"github.com/samber/lo"
)

// hold results if images scans returned from a variety of scanner engines
type PharosImageScanResult struct {
	Version    string           `json:"Version"`
	ScanEngine PharosScanEngine `json:"Scan"` // scanner & scan metadata
	// Add Scantask

	Image           PharosImageMeta       `json:"Image"`
	Findings        []PharosScanFinding   `json:"Findings"`        // instatiation of vulnerabilities in packages
	Vulnerabilities []PharosVulnerability `json:"Vulnerabilities"` // vulnerabilities found with vuln metadata (description, CVSS, ..)
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

// return model as []byte
func (rx *PharosImageScanResult) ToBytes() []byte {
	data, err := json.Marshal(rx)
	if err != nil {
		return []byte{}
	}
	return data
}

// populate model from grype scan
func (rx *PharosImageScanResult) LoadGrypeImageScan(scan *grype.GrypeScanType) error {

	// scan engine
	rx.Version = "1.0"
	rx.ScanEngine.Name = scan.Descriptor.Name
	rx.ScanEngine.Version = scan.Descriptor.Version
	rx.ScanEngine.ScanTime = scan.Descriptor.ScanTime

	// unique lists
	vulnsList := map[string]int{}

	target := scan.Source.Target

	// map image metadata
	rx.Image.ImageSpec = target.UserInput
	rx.Image.ImageId = target.ImageId
	rx.Image.ManigestDigest = target.ManifestDigest
	rx.Image.RepoDigests = lo.Map(target.RepoDigests, func(x string, k int) string { return parseDigest(x) })

	rx.Image.ArchName = target.Architecture
	rx.Image.ArchOS = target.OS
	rx.Image.DistroName = scan.Distro.Name
	rx.Image.DistroVersion = scan.Distro.Version
	rx.Image.Size = target.ImageSize

	rx.Image.Tags = target.Tags
	rx.Image.Layers = lo.Map(target.Layers, func(x grype.GrypeLayer, k int) string { return x.Digest })

	// map matches
	for _, match := range scan.Matches {

		vuln := match.Vulnerability
		artifact := match.Artifact

		finding := PharosScanFinding{
			AdvId:       vuln.Id,
			AdvSource:   vuln.Namespace,
			ScanDate:    time.Now().UTC(),
			UpdateDate:  time.Now().UTC(),
			DueDate:     time.Time{}, // nil populated later
			Severity:    vuln.Severity,
			FixState:    vuln.Fix.State,
			FixVersions: vuln.Fix.Versions,
			FoundIn:     lo.Map(artifact.Locations, func(x grype.GrypeLocation, k int) string { return x.Path }),
		}
		vulnerability := PharosVulnerability{
			AdvId:      vuln.Id,
			AdvSource:  vuln.Namespace,
			CreateDate: time.Now().UTC(),
			PubDate:    time.Time{},
			ModDate:    time.Time{},
			KevDate: utils.DateStrOr(
				lo.FirstOr(
					lo.Map(vuln.KnownExplpited, func(x grype.GrypeKnownExploited, k int) string { return x.DateAdded }),
					"",
				), time.Time{},
			),
			Severity:    vuln.Severity,
			CvssVectors: lo.Map(vuln.Cvss, func(x grype.GrypeCvss, k int) string { return x.Vector }),
			CvssBase:    lo.Max(lo.Map(vuln.Cvss, func(x grype.GrypeCvss, k int) float64 { return x.Metrics.BaseScore })),
			// cpes
			RansomwareUsed: lo.FirstOr(lo.Map(vuln.KnownExplpited, func(x grype.GrypeKnownExploited, k int) string { return x.RansomwareUsed }), ""),
			References:     lo.Map(vuln.Advisories, func(x grype.GrypeAdvisory, k int) string { return x.Link }),
			Description:    vuln.Description,
		}

		// append findings and vulns
		rx.Findings = append(rx.Findings, finding)
		if !lo.HasKey(vulnsList, vulnerability.AdvId) {
			vulnsList[vulnerability.AdvId] = 1
			rx.Vulnerabilities = append(rx.Vulnerabilities, vulnerability)
		}
	}

	return nil
}

// populate model from trivy scan
func (rx *PharosImageScanResult) LoadTrivyImageScan(sbom *cdx.BOM, scan *trivy.TrivyScanType) error {

	// unique
	vulnsList := map[string]int{}
	component := sbom.Metadata.Component
	properties := sbom.Metadata.Component.Properties

	// scan engine
	rx.Version = "1.0"
	rx.ScanEngine.Name = "trivy"
	rx.ScanEngine.Version = trivy.GetToolVersion(sbom)
	rx.ScanEngine.ScanTime = scan.CreatedAt

	// map image
	rx.Image.ImageSpec = sbom.Metadata.Component.Name
	rx.Image.ImageId = cdxFilterPropertyFirstOr("aquasecurity:trivy:ImageID", "", *properties)
	rx.Image.ManigestDigest = parseDigest(component.BOMRef)
	rx.Image.RepoDigests = scan.Metadata.RepoDigests

	rx.Image.ArchName = scan.Metadata.ImageConfig.Architecture
	rx.Image.ArchOS = scan.Metadata.ImageConfig.OS
	rx.Image.DistroName = scan.Metadata.OS.Famile
	rx.Image.DistroVersion = scan.Metadata.OS.Name
	rx.Image.Size = utils.UInt64Or(cdxFilterPropertyFirstOr("aquasecurity:trivy:Size", "", *properties), 0)

	rx.Image.Tags = scan.Metadata.RepoTags
	rx.Image.Layers = lo.Map(scan.Metadata.Layers, func(x trivy.TrivyLayer, k int) string { return x.DiffId })

	// map matches
	for _, match := range scan.Results {
		for _, vuln := range match.Vulnerabilities {
			finding := PharosScanFinding{
				AdvId:       vuln.VulnerabilityId,
				AdvSource:   vuln.SeveritySource,
				ScanDate:    time.Now().UTC(),
				UpdateDate:  time.Now().UTC(),
				DueDate:     time.Time{}, // nil populated later
				Severity:    vuln.Severity,
				FixState:    vuln.Status,
				FixVersions: []string{vuln.FixedVersion},
				FoundIn:     []string{},
			}

			vulnerability := PharosVulnerability{
				AdvId:      vuln.VulnerabilityId,
				AdvSource:  vuln.SeveritySource,
				CreateDate: time.Now().UTC(),
				PubDate:    vuln.PublishedDate,
				ModDate:    vuln.LastModifiedDate,
				//KevDate:
				Severity: vuln.Severity,
				//CvssVectors:
				//CvssBase:
				// cpes
				//RansomwareUsed:
				References:  vuln.References,
				Description: vuln.Description,
			}

			// append findings and vulns
			rx.Findings = append(rx.Findings, finding)
			if !lo.HasKey(vulnsList, vulnerability.AdvId) {
				vulnsList[vulnerability.AdvId] = 1
				rx.Vulnerabilities = append(rx.Vulnerabilities, vulnerability)
			}

		}
	}

	return nil
}
