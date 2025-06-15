package scanning

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelper(t *testing.T) {

	var os, arch, variant string

	os, arch, variant = SplitPlatformStr("")
	assert.Equal(t, "", os)
	assert.Equal(t, "", arch)
	assert.Equal(t, "", variant)

	os, arch, variant = SplitPlatformStr("linux")
	assert.Equal(t, "", os)
	assert.Equal(t, "", arch)
	assert.Equal(t, "", variant)

	os, arch, variant = SplitPlatformStr("linux/")
	assert.Equal(t, "linux", os)
	assert.Equal(t, "", arch)
	assert.Equal(t, "", variant)

	os, arch, variant = SplitPlatformStr("linux/arm")
	assert.Equal(t, "linux", os)
	assert.Equal(t, "arm", arch)
	assert.Equal(t, "", variant)

	os, arch, variant = SplitPlatformStr("linux/arm/v7")
	assert.Equal(t, "linux", os)
	assert.Equal(t, "arm", arch)
	assert.Equal(t, "v7", variant)
}
