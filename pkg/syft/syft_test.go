package syft

import (
	"testing"

	"github.com/metraction/pharos/internal/logging"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

var logger = logging.NewLogger("info")

func TestSyft(t *testing.T) {

	assert.Equal(t, "AA", lo.CoalesceOrEmpty("AA", "BB"))
	assert.Equal(t, "BB", lo.CoalesceOrEmpty("", "BB"))
}
