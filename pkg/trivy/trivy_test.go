package trivy

import (
	"testing"

	"github.com/metraction/pharos/internal/logging"
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
