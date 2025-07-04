package model

import (
	"regexp"
	"time"

	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/grypetype"
	"github.com/metraction/pharos/pkg/syfttype"

	"github.com/samber/lo"
)

// populate model from grype scan
// digest is the manifest digest used as uniq ID for image
func (rx *PharosScanResult) LoadGrypeImageScan(digest string, task PharosScanTask2, sbom syfttype.SyftSbomType, scan grypetype.GrypeScanType) error {

	// result version
	// 1.1 added ScanMeta{}
	rx.Version = "1.1"

	rx.ScanMeta.Engine = scan.Descriptor.Name
	rx.ScanMeta.EngineVersion = scan.Descriptor.Version
	rx.ScanMeta.ScanDate = scan.Descriptor.ScanTime
	rx.ScanMeta.DbBuiltDate = scan.Descriptor.Db.Status.Built

	// unique lists
	vulnsList := map[string]int{}
	packagesList := map[string]int{}

	target := scan.Source.Target

	// (1) load image metadata
	rx.Image.ImageSpec = utils.MaskDsn(rx.ScanTask.ImageSpec)
	rx.Image.ImageId = utils.ShortDigest(digest) + "-grype" // was target.ImageId
	rx.Image.ManifestDigest = digest                        // was target.ManifestDigest)

	if len(target.RepoDigests) > 0 {
		re := regexp.MustCompile(`@(.+)$`)
		matches := re.FindStringSubmatch(target.RepoDigests[0])
		if len(matches) == 2 {
			rx.Image.IndexDigest = matches[1]
		} else {
			rx.Image.IndexDigest = ""
		}
	} // TODO: check if this is correct, but we use this for now.
	rx.Image.RepoDigests = lo.Map(target.RepoDigests, func(x string, k int) string { return ParseDigest(x) })

	rx.Image.ArchName = target.Architecture
	rx.Image.ArchOS = target.OS
	rx.Image.DistroName = scan.Distro.Name
	rx.Image.DistroVersion = scan.Distro.Version
	rx.Image.Size = target.ImageSize

	rx.Image.Tags = target.Tags
	rx.Image.Layers = lo.Map(target.Layers, func(x grypetype.GrypeLayer, k int) string { return x.Digest })

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
			FoundIn:     lo.Map(artifact.Locations, func(x grypetype.GrypeLocation, k int) string { return x.Path }),
		}
		vulnerability := PharosVulnerability{
			AdvId:      vuln.Id,
			AdvSource:  vuln.Namespace,
			CreateDate: time.Now().UTC(),
			PubDate:    time.Time{},
			ModDate:    time.Time{},
			KevDate: utils.DateStrOr(
				lo.FirstOr(
					lo.Map(vuln.KnownExplpited, func(x grypetype.GrypeKnownExploited, k int) string { return x.DateAdded }),
					"",
				), time.Time{},
			),
			Severity:    vuln.Severity,
			CvssVectors: lo.Map(vuln.Cvss, func(x grypetype.GrypeCvss, k int) string { return x.Vector }),
			CvssBase:    lo.Max(lo.Map(vuln.Cvss, func(x grypetype.GrypeCvss, k int) float64 { return x.Metrics.BaseScore })),
			// cpes
			RansomwareUsed: lo.FirstOr(lo.Map(vuln.KnownExplpited, func(x grypetype.GrypeKnownExploited, k int) string { return x.RansomwareUsed }), ""),
			References:     lo.Map(vuln.Advisories, func(x grypetype.GrypeAdvisory, k int) string { return x.Link }),
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
