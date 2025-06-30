package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDsnParsing(t *testing.T) {

	// "docker://user:pwd@pharos.alfa.lan:123/?mi=off"

	// GetHostPortOr
	defval := "n/a"
	assert.Equal(t, "", DsnHostPortOr("", defval))
	assert.Equal(t, "host", DsnHostPortOr("docker://host", defval))
	assert.Equal(t, "host:123", DsnHostPortOr("docker://host:123", defval))
	assert.Equal(t, "host:123", DsnHostPortOr("docker://user:name@host:123", defval))
	assert.Equal(t, "host:123", DsnHostPortOr("docker://user:name@host:123?no=yes", defval))
	assert.Equal(t, "host:123", DsnHostPortOr("docker://user:name@host:123/?no=yes", defval))
	assert.Equal(t, "127.0.0.1:123", DsnHostPortOr("docker://127.0.0.1:123/?no=yes", defval))

	//
	assert.Equal(t, "", DsnHostPortOr("mimi.com/alfa", defval))

	// MaskDsn
	assert.Equal(t, "mimi", MaskDsn("mimi"))
	assert.Equal(t, "docker://host:123/tls=on", MaskDsn("docker://host:123/tls=on"))
	assert.Equal(t, "docker://:***@host:123/tls=on", MaskDsn("docker://:pwd@host:123/tls=on"))
	assert.Equal(t, "docker://usr:***@host:123/tls=on", MaskDsn("docker://usr:pwd@host:123/tls=on"))

	// User & password
	input := "docker://user:pwd@host:123/?tlscheck=on&star=1337&security=off"
	assert.Equal(t, "user", DsnUserOr(input, ""))
	assert.Equal(t, "pwd", DsnPasswordOr(input, ""))

	// Parameter
	assert.Equal(t, "on", DsnParaOr(input, "tlscheck", ""))
	assert.Equal(t, "1337", DsnParaOr(input, "star", ""))
	assert.Equal(t, "no", DsnParaOr(input, "mimi", "no"))

	// Parameter bool
	assert.True(t, DsnParaBoolOr(input, "not-set", true))
	assert.True(t, DsnParaBoolOr(input, "tlscheck", true))
	assert.False(t, DsnParaBoolOr(input, "security", true)) // default true, set false if given

	//assert.True(t, DsnParaBoolOr(input, "not-set", true))

}
