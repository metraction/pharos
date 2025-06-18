package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDSN(t *testing.T) {

	var err error
	auth := PharosRepoAuth{}

	// user+pwd
	err = auth.FromDsn("registry://user:pwd@docker.io:123")
	assert.NoError(t, err)

	assert.Equal(t, "registry://user:***@docker.io:123", auth.ToMaskedDsn("***"))
	assert.Equal(t, "docker.io:123", auth.Authority)
	assert.Equal(t, "user", auth.Username)
	assert.Equal(t, "pwd", auth.Password)
	assert.Equal(t, "", auth.Token)

}
