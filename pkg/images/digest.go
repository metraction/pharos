package images

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
)

// split "linux/amd64" or "linux/arm/v6" to OS, architecture, variant
func SplitPlatformStr(input string) (string, string, string) {
	parts := strings.Split(strings.TrimSpace(input)+"/", "/")
	if len(parts) > 2 {
		return parts[0], parts[1], parts[2]
	}
	return "", "", ""
}

// return platform specific digests for image given imageRef (docker.io/redis:latest) and platform ("linux/amd64")
// return
//   - indexDigest (general)
//   - manifestDigest (platform specific)
//
// func GetImageDigests(imageRef, platform string, auth model.PharosRepoAuth, tlsCheck bool) (string, string, error) {
func GetImageDigests(task model.PharosScanTask2) (string, string, string, error) {

	var options []remote.Option
	platform := task.Platform // TODO: Empty platform -> default? What if @sha:.. is given, so no platform needed

	ref, err := name.ParseReference(task.ImageSpec)
	if err != nil {
		return "", "", "", err
	}

	// add platform option if given
	if platform != "" {
		os, arch, variant := SplitPlatformStr(platform)
		if os == "" || arch == "" {
			return "", "", "", fmt.Errorf("invalid platform '%s'", platform)
		}
		options = append(options, remote.WithPlatform(v1.Platform{
			OS:           os,
			Architecture: arch,
			Variant:      variant,
		}))
	}

	// add auth option if given
	if utils.DsnUserOr(task.AuthDsn, "") != "" {
		options = append(options, remote.WithAuth(&authn.Basic{
			Username: utils.DsnUserOr(task.AuthDsn, ""),
			Password: utils.DsnPasswordOr(task.AuthDsn, ""),
		}))
	}

	// slip tls certificate verify if set to off (default is on)
	if !utils.DsnParaBoolOr(task.AuthDsn, "tlscheck", true) {
		options = append(options, remote.WithTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}))
	}

	// get image description
	desc, err := remote.Get(ref, options...)
	if err != nil {
		return "", "", "", err
	}

	rxPlatform := ""
	if desc != nil && desc.Platform != nil {

		fmt.Println("remote.Get Arch:   ", desc.Platform.Architecture)
		fmt.Println("remote.Get OS:     ", desc.Platform.OS)
		fmt.Println("remote.Get Variant: ", desc.Platform.Variant)
		rxPlatform = fmt.Sprintf("%s/%s/%s", desc.Platform.OS, desc.Platform.Architecture, desc.Platform.Variant)
	}

	indexDigest := desc.Digest.String() // same accross platforms
	manifestDigest := "N/A"             // depends on platform (set default here for  single-image manifest)

	img, err := desc.Image() //desc.ImageIndex()
	if err == nil {
		digest, err := img.Digest()
		if err != nil {
			return "", "", "", err
		}
		manifestDigest = digest.String()
	}

	// ref.Name()			index.docker.io/library/busybox:1.37.0
	// ref.Identifier()		1.37.0
	// layers, err := img.Layers()
	// if err != nil {
	// 	return "", "", "", err
	// }
	// for k, layer := range layers {
	// 	digest, _ := layer.Digest()
	// 	fmt.Printf("L%d:\t%s\n", k, digest.String())
	// }

	return indexDigest, manifestDigest, rxPlatform, nil

}
