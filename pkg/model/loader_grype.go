package model

import (
	"time"

	"github.com/metraction/pharos/internal/scanner/grype"
	"github.com/metraction/pharos/internal/scanner/syft"
	"github.com/metraction/pharos/internal/utils"
	"github.com/samber/lo"
)

// populate model from grype scan
func (rx *PharosImageScanResult) LoadGrypeImageScan(sbom syft.SyftSbomType, scan grype.GrypeScanType) error {

	// scan engine
	rx.Version = "1.0"
	rx.ScanEngine.Name = scan.Descriptor.Name
	rx.ScanEngine.Version = scan.Descriptor.Version
	rx.ScanEngine.ScanTime = scan.Descriptor.ScanTime

	// unique lists
	vulnsList := map[string]int{}
	packagesList := map[string]int{}

	target := scan.Source.Target

	// (1) load image metadata
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

	// (2) load findings and vulnerabilities
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
			vulnsList[vulnerability.AdvId] += 1
			rx.Vulnerabilities = append(rx.Vulnerabilities, vulnerability)
		}
	}

	// (3) load packages from sbom
	for _, artifact := range sbom.Artifacts {
		pack := PharosPackage{
			Type:    artifact.Type,
			Name:    artifact.Name,
			Version: artifact.Version,
			Purl:    utils.DecodePurl(artifact.Purl),
		}

		if !lo.HasKey(packagesList, pack.Purl) {
			packagesList[pack.Purl] += 1
			rx.Packages = append(rx.Packages, pack)
		}
	}

	return nil
}
