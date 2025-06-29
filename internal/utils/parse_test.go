package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {

	// RightOfPrefixOr
	prefix := "# platform:"
	defval := "nothing"

	// Fail
	assert.Equal(t, defval, RightOfPrefixOr("", prefix, defval))
	assert.Equal(t, defval, RightOfPrefixOr("platform:", prefix, defval))
	assert.Equal(t, defval, RightOfPrefixOr("hello world", prefix, defval))
	assert.Equal(t, defval, RightOfPrefixOr("platform:", prefix, defval))
	assert.Equal(t, defval, RightOfPrefixOr("hello # platform: mimi", prefix, defval))

	// OK
	assert.Equal(t, "alfa/1.20", RightOfPrefixOr("# platform:alfa/1.20", prefix, defval))
	assert.Equal(t, "alfa/1.20", RightOfPrefixOr("# platform: alfa/1.20", prefix, defval))
	assert.Equal(t, "alfa/1.20", RightOfPrefixOr("# platform: alfa/1.20 ", prefix, defval))

	// RightOfFirstOr

	assert.Equal(t, "", RightOfFirstOr("", "", defval))
	assert.Equal(t, "mixi", RightOfFirstOr("mixi", "", defval))
	assert.Equal(t, "mixi", RightOfFirstOr("<s>mixi", "<s>", defval))
	assert.Equal(t, "mixi<s>maxi", RightOfFirstOr("kiri<s>mixi<s>maxi", "<s>", defval))

	// LeftOfFirstOr
	assert.Equal(t, "", LeftOfFirstOr("", "", defval))
	assert.Equal(t, "", LeftOfFirstOr("mixi", "", defval))
	assert.Equal(t, defval, LeftOfFirstOr("mixi", "<s>", defval))
	assert.Equal(t, "mixi", LeftOfFirstOr("mixi<s>", "<s>", defval))
	assert.Equal(t, "mixi", LeftOfFirstOr("mixi<s>maxi<s>", "<s>", defval))
	assert.Equal(t, defval, LeftOfFirstOr("mixi<x>maxi", "<s>", defval))

}
