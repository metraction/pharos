package model

import (
	"encoding/json"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/metraction/pharos/internal/scanner/grype"
	"github.com/metraction/pharos/internal/scanner/trivy"
	"github.com/metraction/pharos/internal/utils"
	"github.com/samber/lo"
)

// hold results from divers scanners
type ScanResultModel struct {
	Asset AssetMetaType `json:"assetmeta"`
}

// hold metadata for different asset types [image,code,vm,..]
type AssetMetaType struct {
	Image ImageMetaType `json:"image"`
}

// metadata about the asset (image, code, vm, ..)
type ImageMetaType struct {
	Input          string   `json:"Input"`
	ImageId        string   `json:"ImageId"`
	ManigestDigest string   `json:"ManifestDigest"`
	RepoDigests    []string `json:"RepoDigests"`
	Architecture   string   `json:"Architecture"`
	OS             string   `json:"OS"`
	DistroName     string   `json:"DistroName"`
	DistroVersion  string   `json:"DistroVersion"`
	Size           uint64   `json:"Size"`

	Tags   []string `json:"tags"`
	Layers []string `json:"layers"`
}

// return model as []byte
func (rx *ScanResultModel) ToBytes() []byte {
	data, err := json.Marshal(rx)
	if err != nil {
		return []byte{}
	}
	return data
}

// populate model from grype scan
func (rx *ScanResultModel) LoadGrypeScan(scan *grype.GrypeScanType) error {

	target := scan.Source.Target

	// map image
	rx.Asset.Image.Input = target.UserInput
	rx.Asset.Image.ImageId = target.ImageId
	rx.Asset.Image.ManigestDigest = target.ManifestDigest
	rx.Asset.Image.RepoDigests = lo.Map(target.RepoDigests,
		func(x string, k int) string {
			return parseDigest(x)
		})

	rx.Asset.Image.Architecture = target.Architecture
	rx.Asset.Image.OS = target.OS
	rx.Asset.Image.Size = target.ImageSize

	rx.Asset.Image.DistroName = scan.Distro.Name
	rx.Asset.Image.DistroVersion = scan.Distro.Version

	rx.Asset.Image.Tags = target.Tags
	rx.Asset.Image.Layers = lo.Map(target.Layers, func(x grype.GrypeLayer, k int) string { return x.Digest })

	// map matches
	return nil
}

// populate model from trivy scan
func (rx *ScanResultModel) LoadTrivyScan(sbom *cdx.BOM, scan *trivy.TrivyScanType) error {

	// map image
	component := sbom.Metadata.Component
	properties := sbom.Metadata.Component.Properties

	rx.Asset.Image.Input = component.Name
	rx.Asset.Image.Name = ""
	rx.Asset.Image.Tag = ""
	rx.Asset.Image.ImageId = cdxFilterPropertyFirstOr("aquasecurity:trivy:ImageID", "", *properties)
	rx.Asset.Image.ManigestDigest = parseDigest(component.BOMRef)
	rx.Asset.Image.RepoDigests = lo.Map(cdxFilterProperty("aquasecurity:trivy:RepoDigest", *properties),
		func(x string, k int) string {
			return parseDigest(x)
		})

	// rx.Asset.Image.Architecture = target.Architecture
	// rx.Asset.Image.OS = target.OS
	rx.Asset.Image.Size = utils.UInt64Or(cdxFilterPropertyFirstOr("aquasecurity:trivy:Size", "", *properties), 0)

	// rx.Asset.Image.Tags = target.Tags
	rx.Asset.Image.Layers = cdxFilterProperty("aquasecurity:trivy:DiffID", *properties)

	// map matches
	return nil
}
