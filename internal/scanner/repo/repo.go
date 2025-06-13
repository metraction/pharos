package repo

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kos-v/dsnparser"
)

// authentication for image repos
type RepoAuth struct {
	Authority string `json:"Authority"`
	Username  string `json:"Username"`
	Password  string `json:"Password"`
	Token     string `json:"Token"`
}

// return true if auth is not empty
func (rx RepoAuth) HasAuth() bool {
	return rx.Authority != "" && (rx.Username != "" || rx.Token != "")
}

// return DSN without password
func (rx RepoAuth) ToMaskedDsn(mask string) string {
	if rx.Password != "" {
		return fmt.Sprintf("registry://%s:%s@%s/?type=password", rx.Username, mask, rx.Authority)
	}
	if rx.Token != "" {
		return fmt.Sprintf("registry://%s:%s@%s/?type=token", rx.Username, mask, rx.Authority)
	}
	return ""
}

// parse DSN
// registry://user:password@docker.io/type=password
// registry://user:token@docker.io/type=token
func (rx *RepoAuth) FromDsn(input string) error {

	// reset
	rx.Authority = ""
	rx.Username = ""
	rx.Password = ""
	rx.Token = ""

	if input == "" {
		return nil
	}
	dsn := dsnparser.Parse(input)
	if dsn == nil {
		return fmt.Errorf("invalid DSN")
	}
	what := dsn.GetParam("type")
	rx.Authority = dsn.GetHost()
	if what == "password" {
		rx.Username = dsn.GetUser()
		rx.Password = dsn.GetPassword()
		return nil
	}
	return fmt.Errorf("invalid DSN type '%s' (e.g. 'registry://usr:pwd@docker.io/?type=password')", what)
}

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
func GetImageDigests(imageRef, platform string, auth RepoAuth) (string, string, error) {

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
	if auth.HasAuth() {
		options = append(options, remote.WithAuth(&authn.Basic{
			Username: auth.Username,
			Password: auth.Password,
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

	//fmt.Println("image", imageRef)
	//fmt.Println("- digest.idx", indexDigest)
	//fmt.Println("- digest.man", manifestDigest, platform)

	return indexDigest, manifestDigest, nil

}
