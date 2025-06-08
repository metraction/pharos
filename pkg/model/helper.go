package model

import (
	"net/url"
	"regexp"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/samber/lo"
)

// extract repot digest from uri
// index.docker.io/library/nginx@sha256:38f8c1d9613f3f42e7969c3b1dd5c3277e635d4576713e6453c6193e66270a6d
func getRepoDigest(input string) string {
	parts := strings.Split(input, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return input
}

// return list of property.value for properties with given name
func cdxFilterProperty(name string, data []cdx.Property) []string {
	filtered := lo.Filter(data, func(x cdx.Property, k int) bool {
		return x.Name == name
	})
	return lo.Map(filtered, func(x cdx.Property, k int) string {
		return x.Value
	})
}

func cdxFilterPropertyFirstOr(name, defval string, data []cdx.Property) string {
	filtered := cdxFilterProperty(name, data)
	if len(filtered) > 0 {
		return filtered[0]
	}
	return defval
}

// return digest from "bom-ref": "pkg:oci/alpine@sha256%3A0db9d004361b106932f8c7632ae54d56e92c18281e2dd203127d77405020abf6?arch=amd64&repository_url=index.docker.io%2Flibrary%2Falpine",
func parseDigest(input string) string {

	decoded, _ := url.QueryUnescape(input)

	re := regexp.MustCompile(`@([0-9a-z]+:[0-9a-z]+)`) //`@sha256:([a-fA-F0-9]{64})`
	match := re.FindStringSubmatch(decoded)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}
