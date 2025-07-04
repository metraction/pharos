package model

import (
	"time"

	"github.com/metraction/pharos/internal/utils"

	"github.com/metraction/pharos/pkg/trivytype"
	"github.com/samber/lo"
)

// populate model from trivy scan
func (rx *PharosScanResult) LoadTrivyImageScan(digest string, task PharosScanTask2, sbom trivytype.TrivySbomType, scan trivytype.TrivyScanType) error {

	// unique
	vulnsList := map[string]int{}
	packagesList := map[string]int{}

	properties := sbom.Metadata.Component.Properties

	// scan engine
	rx.Version = "1.1"

	rx.ScanMeta.Engine, rx.ScanMeta.EngineVersion = trivytype.GetAppVersion(*sbom.Metadata.Tools.Components)
	rx.ScanMeta.ScanDate = scan.CreatedAt
	rx.ScanMeta.DbBuiltDate = scan.CreatedAt // no better data available

	// (1) load image metadata
	rx.Image.ImageSpec = sbom.Metadata.Component.Name
	rx.Image.ImageId = utils.ShortDigest(digest) + "-trivy" // was target.ImageId
	//rx.Image.ImageId = digest        // was cdxFilterPropertyFirstOr("aquasecurity:trivy:ImageID", "", *properties)
	rx.Image.ManifestDigest = digest // was target.ManifestDigest)
	rx.Image.RepoDigests = scan.Metadata.RepoDigests

	// ATTN: architecture non populated in scan, take it from BOMRef
	os, arch, _ := utils.SplitPlatformStr(task.Platform)
	rx.Image.ArchOS = lo.CoalesceOrEmpty(scan.Metadata.ImageConfig.OS, os)
	rx.Image.ArchName = lo.CoalesceOrEmpty(scan.Metadata.ImageConfig.Architecture, arch)

	rx.Image.DistroName = scan.Metadata.OS.Famile
	rx.Image.DistroVersion = scan.Metadata.OS.Name
	rx.Image.Size = utils.ToNumOr[uint64](cdxFilterPropertyFirstOr("aquasecurity:trivy:Size", "", *properties), 0)

	rx.Image.Tags = scan.Metadata.RepoTags
	rx.Image.Layers = lo.Map(scan.Metadata.Layers, func(x trivytype.TrivyLayer, k int) string { return x.DiffId })

	// (2) load findings and vulnerabilities
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

	// (3) load packages from sbom
	for _, artifact := range *sbom.Components {
		pack := PharosPackage{
			Type:    string(artifact.Type),
			Name:    artifact.Name,
			Version: artifact.Version,
			Purl:    utils.DecodePurl(artifact.PackageURL),
		}

		if !lo.HasKey(packagesList, pack.Purl) {
			packagesList[pack.Purl] += 1
			rx.Packages = append(rx.Packages, pack)
		}
	}
	return nil
}
