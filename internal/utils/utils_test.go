package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDsn(t *testing.T) {

	testsSuccess := []string{
		"redis://usr:pwd@host:6379/0",
	}

	for _, x := range testsSuccess {
		scheme, user, password, host, err := ParseDsn(x)
		assert.NoError(t, err)
		assert.Equal(t, "redis", scheme)
		assert.GreaterOrEqual(t, user, "u")
		assert.GreaterOrEqual(t, password, "p")
		assert.GreaterOrEqual(t, host, "h")

	}
}
