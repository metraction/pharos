package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDsnParsing(t *testing.T) {

	// "docker://user:pwd@pharos.alfa.lan:123/?mi=off"

	// GetHostPortOr
	defval := "n/a"
	assert.Equal(t, "", GetHostPortOr("", defval))
	assert.Equal(t, "host", GetHostPortOr("docker://host", defval))
	assert.Equal(t, "host:123", GetHostPortOr("docker://host:123", defval))
	assert.Equal(t, "host:123", GetHostPortOr("docker://user:name@host:123", defval))
	assert.Equal(t, "host:123", GetHostPortOr("docker://user:name@host:123?no=yes", defval))
	assert.Equal(t, "host:123", GetHostPortOr("docker://user:name@host:123/?no=yes", defval))
	assert.Equal(t, "127.0.0.1:123", GetHostPortOr("docker://127.0.0.1:123/?no=yes", defval))

	//
	assert.Equal(t, "", GetHostPortOr("mimi.com/alfa", defval))

	// MaskDsn
	assert.Equal(t, "mimi", MaskDsn("mimi"))
	assert.Equal(t, "docker://host:123/tls=on", MaskDsn("docker://host:123/tls=on"))
	assert.Equal(t, "docker://:***@host:123/tls=on", MaskDsn("docker://:pwd@host:123/tls=on"))
	assert.Equal(t, "docker://usr:***@host:123/tls=on", MaskDsn("docker://usr:pwd@host:123/tls=on"))
}
