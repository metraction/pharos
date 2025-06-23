package grype

import (
	"testing"

	"github.com/metraction/pharos/internal/logging"
	"github.com/stretchr/testify/assert"
)

var logger = logging.NewLogger("info")

func TestGrype(t *testing.T) {

	var err error

	// output of grype version -o json
	version := `{"application": "grype","buildDate": "2025-05-20T22:12:30Z","compiler": "gc","gitCommit": "Homebrew","gitDescription": "[not provided]","goVersion": "go1.24.3","platform": "darwin/amd64","supportedDbSchema": 6,"syftVersion": "v1.26.0","version": "0.92.2"}`

	// output of grype db status -o json
	dbstate := `{"schemaVersion": "v6.0.2", "from": "https://grype.anchore.io/databases/v6/vulnerability-db_v6.0.2_2025-06-08T01:32:20Z_1749356097.tar.zst?checksum=sha256%3A", "built": "2025-06-08T04:14:57Z", "path": "/Users/sam/Library/Caches/grype/db/6/vulnerability.db", "valid": true}`

	// grype version
	xver := GrypeVersion{}
	err = xver.FromBytes([]byte(version))

	assert.NoError(t, err)
	assert.Equal(t, "0.92.2", xver.GrypeVersion)

	// grype db state
	xstate := GrypeDbStatus{}
	err = xstate.FromBytes([]byte(dbstate))

	assert.NoError(t, err)
	assert.Equal(t, "2025-06-08 04:14:57 +0000 UTC", xstate.Built.String())
	assert.Equal(t, "v6.0.2", xstate.SchemaVersion)

}
