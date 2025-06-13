package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestZStd(t *testing.T) {

	testsData := []string{
		//"",
		//"\\0",
		"Hello World",
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.",
	}
	packer, err := NewZStd()
	assert.NoError(t, err)
	for _, x := range testsData {
		data := []byte(x)
		packed := packer.Compress(data)
		result, err := packer.Decompress(packed)
		assert.NoError(t, err)
		assert.Equal(t, data, result)

	}

}
