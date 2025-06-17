package scanning

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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
//
//	indexDigest (general)
//	manifestDigest (platform specific)
//
// func GetImageDigests(imageRef, platform string, auth model.PharosRepoAuth, tlsCheck bool) (string, string, error) {
func GetImageDigests(task model.PharosScanTask) (string, string, error) {

	var options []remote.Option

	auth := task.Auth
	imageRef := task.ImageSpec.Image
	platform := task.ImageSpec.Platform

	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return "", "", err
	}

	// prepare platform option
	if platform != "" {
		os, arch, variant := SplitPlatformStr(platform)
		if os == "" || arch == "" {
			return "", "", fmt.Errorf("invalid platform '%s'", platform)
		}
		options = append(options, remote.WithPlatform(v1.Platform{
			OS:           os,
			Architecture: arch,
			Variant:      variant,
		}))
	}

	// prepare auth option
	if auth.HasAuth(imageRef) {
		options = append(options, remote.WithAuth(&authn.Basic{
			Username: auth.Username,
			Password: auth.Password,
		}))
		// TODO Token Auth
	}

	// slip tls certificate verify
	if !auth.TlsCheck {
		options = append(options, remote.WithTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}))
	}

	// get image description
	desc, err := remote.Get(ref, options...)
	if err != nil {
		return "", "", err
	}
	indexDigest := desc.Digest.String() // same accross platforms
	manifestDigest := "N/A"             // depends on platform (set default here for  single-image manifest)

	img, err := desc.Image() //desc.ImageIndex()
	if err == nil {
		digest, err := img.Digest()
		if err != nil {
			return "", "", err
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

	return indexDigest, manifestDigest, nil

}
