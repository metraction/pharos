package scanner

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type RepoAuthType struct {
	Authority string `json:"Authority"`
	Username  string `json:"Username"`
	Password  string `json:"Password"`
	Token     string `json:"Token"`
}

// split "linux/amd64" or "linux/arm/v6" to OS, architecture, variant
func SplitPlatformStr(input string) (string, string, string) {

	parts := strings.Split(strings.TrimSpace(input)+"/", "/")
	if len(parts) > 2 {
		return parts[0], parts[1], parts[2]
	}
	return "", "", ""
}

// return platform specific digest for image given imageRef (docker.io/redis:latest) and platform ("linux/amd64")
func GetImageDigests(imageRef, platform string, auth RepoAuthType) (string, string, error) {

	var options []remote.Option

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
	options = append(options, remote.WithAuth(&authn.Basic{
		Username: auth.Username,
		Password: auth.Password,
	}))

	// get description, image, index
	desc, err := remote.Get(ref, options...)
	if err != nil {
		return "", "", err
	}
	img, _ := desc.Image()
	idx, _ := desc.ImageIndex()

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

	// digest1 := desc.Digest	// == index digest
	digest1, err := img.Digest()
	if err != nil {
		return "", "", err
	}

	// index digest
	digest2, err := idx.Digest()
	if err != nil {
		return "", "", err
	}
	return digest1.String(), digest2.String(), nil

}
