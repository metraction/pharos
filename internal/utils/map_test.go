package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveMap(t *testing.T) {

	data := map[string]any{
		"hello":  "Darth_Vader",
		"number": 1337,
		"mope":   "nothing",
		"world": map[string]any{
			"wide": "web",
		},
	}

	assert.Equal(t, "a:Darth_Vader, b:web, c:1337", ResolveMap("a:{{.hello}}, b:{{.world.wide}}, c:{{.number}}", data))

	assert.True(t, true)

}
