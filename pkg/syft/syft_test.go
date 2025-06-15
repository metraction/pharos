package syft

import (
	"testing"
	"time"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/stretchr/testify/assert"
)

var logger = logging.NewLogger("info")

func TestCreateSbom(t *testing.T) {

	goodImage := "docker.io/busybox:1.37.0"
	goodPlatform := "linux/amd64"

	badImage := "docker.io/noimage:1.0"
	badImageHost := "nohost.lan/noimage:notag"
	badPlatform := "darth/vader64"

	auth := model.RepoAuth{}
	xbom, err := NewSyftSbomCreator(30*time.Second, logger)

	assert.NoError(t, err)

	errorTests := [][3]string{
		{"", "", "no image provided"},
		{badImageHost, "", "no platform provided"},
		{badImageHost, badPlatform, "ERROR invalid platform:"},
		{badImageHost, goodPlatform, "ERROR could not determine source:"},
		{badImage, goodPlatform, "ERROR could not determine source:"}, // takes time
	}
	for _, x := range errorTests {
		_, _, err = xbom.CreateSbom(x[0], x[1], "syft-json", auth)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), x[2])
	}

	// check success
	// Check OK

	xbom, _ = NewSyftSbomCreator(100*time.Millisecond, logger)
	_, _, err = xbom.CreateSbom(goodImage, goodPlatform, "syft-json", auth)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create sbom: timeout after")

	xbom, _ = NewSyftSbomCreator(100*time.Second, logger)
	_, _, err = xbom.CreateSbom(goodImage, goodPlatform, "syft-json", auth) // takes time
	assert.NoError(t, err)

}
