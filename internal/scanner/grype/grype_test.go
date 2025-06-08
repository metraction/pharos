package grype

import (
	"testing"

	"github.com/metraction/pharos/internal/logging"
	"github.com/stretchr/testify/assert"
)

var logger = logging.NewLogger("info")

func TestGrype(t *testing.T) {

	//_, err := NewGrypeScanner(30*time.Second, logger)

	version := `{"application": "grype","buildDate": "2025-05-20T22:12:30Z","compiler": "gc","gitCommit": "Homebrew","gitDescription": "[not provided]","goVersion": "go1.24.3","platform": "darwin/amd64","supportedDbSchema": 6,"syftVersion": "v1.26.0","version": "0.92.2"}`

	xver := GrypeVersion{}

	err := xver.FromBytes([]byte(version))

	assert.NoError(t, err)
}
