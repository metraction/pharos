package trivy

import (
	"testing"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/trivytype"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

var logger = logging.NewLogger("info")

func TestTrivy(t *testing.T) {

	version := `{"Version":"0.63.0","VulnerabilityDB":{"Version":2,"NextUpdate":"2025-06-09T06:16:50.711955298Z","UpdatedAt":"2025-06-08T06:16:50.711955458Z","DownloadedAt":"2025-06-08T12:26:59Z"}}`

	// trivy version and db state
	xver := TrivyVersion{}
	err := xver.FromBytes([]byte(version))

	assert.NoError(t, err)
	assert.Equal(t, "0.63.0", xver.Version)
	assert.Equal(t, "2025-06-08 12:26:59 +0000 UTC", xver.VulnerabilityDb.DownloadedAt.String())
	assert.Equal(t, 2, xver.VulnerabilityDb.Version)

}

func TestTrivyType(t *testing.T) {

	bomRefAmd := "pkg:oci/alpine@sha256%3Af85873713?arch=amd64&repository_url=pharos.lan%2Fdocker.io%2Falpine"
	bomRefArm := "pkg:oci/alpine@sha256%3Af85873713?arch=arm64&repository_url=pharos.lan%2Fdocker.io%2Falpine"
	bomRefNon := "pkg:oci/alpine@sha256%3Af85873713?xrch=arm64&repository_url=pharos.lan%2Fdocker.io%2Falpine"

	assert.Equal(t, "amd64", trivytype.GetArchOr(bomRefAmd, "n/a"))
	assert.Equal(t, "arm64", trivytype.GetArchOr(bomRefArm, "n/a"))
	assert.Equal(t, "n/a", trivytype.GetArchOr(bomRefNon, "n/a"))

	assert.Equal(t, "due", lo.CoalesceOrEmpty("", "due"))

}
