package model

import (
	"encoding/json"
	"fmt"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/metraction/pharos/internal/scanner/grype"
	"github.com/metraction/pharos/internal/scanner/trivy"
	"github.com/samber/lo"
)

// hold results from divers scanners
type PharosImageScanResult struct {
	Image PharosImageMeta `json:"image"`
}

// metadata about the asset (image, code, vm, ..)
type PharosImageMeta struct {
	ImageSpec      string   `json:"ImageSpec"` // scan input / image uri
	ImageId        string   `json:"ImageId"`
	Digest         string   `json:"digest"` // internal ID for cache
	ManigestDigest string   `json:"ManifestDigest"`
	RepoDigests    []string `json:"RepoDigests"`
	ArchName       string   `json:"ArchName"` // image platform architecture amd64/..
	ArchOS         string   `json:"ArchOS"`   // image platform OS
	DistroName     string   `json:"DistroName"`
	DistroVersion  string   `json:"DistroVersion"`
	Size           uint64   `json:"Size"`
	Tags           []string `json:"tags"`
	Layers         []string `json:"layers"`
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

	return nil
}

// populate model from trivy scan
func (rx *PharosImageScanResult) LoadTrivyImageScan(sbom *cdx.BOM, scan *trivy.TrivyScanType) error {

	// component := sbom.Metadata.Component
	// cdxFilterPropertyFirstOr("aquasecurity:trivy:ImageID", "", *properties)
	properties := sbom.Metadata.Component.Properties

	fmt.Println()

	// map image
	rx.Image.ImageSpec = sbom.Metadata.Component.Name
	rx.Image.ImageId = cdxFilterPropertyFirstOr("aquasecurity:trivy:ImageID", "", *properties)
	rx.Image.ManigestDigest = "" // parseDigest(component.BOMRef)
	rx.Image.RepoDigests = scan.Metadata.RepoDigests

	rx.Image.ArchName = scan.Metadata.ImageConfig.Architecture
	rx.Image.ArchOS = scan.Metadata.ImageConfig.OS
	rx.Image.DistroName = scan.Metadata.OS.Famile
	rx.Image.DistroVersion = scan.Metadata.OS.Name
	rx.Image.Size = scan.Metadata.Size

	rx.Image.Tags = scan.Metadata.RepoTags
	rx.Image.Layers = lo.Map(scan.Metadata.Layers, func(x trivy.TrivyLayer, k int) string { return x.DiffId })

	// map matches

	return nil
}
