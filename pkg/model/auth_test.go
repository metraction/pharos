package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMatchingAuth(t *testing.T) {

	repos := []string{
		"registry://me:secret@docker.io",
		"registry://me:secret@artifactory.internal",
		"registry://me:secret@exploit.nsa",
	}

	auths := []PharosRepoAuth{}

	for _, dsn := range repos {
		auth, err := NewPharosRepoAuth(dsn)
		assert.NoError(t, err)
		auths = append(auths, auth)
	}
	assert.Equal(t, len(repos), len(auths))

	for k, host := range []string{"exploit.nsa", "artifactory.internal", "docker.io"} {
		repo := fmt.Sprintf("%s/internal/nginx:1.%v", host, k)
		auth := GetMatchingAuth(repo, auths)
		assert.Equal(t, host, auth.Authority)
	}

}

func TestDSN(t *testing.T) {

	var err error
	auth := PharosRepoAuth{}

	// user+pwd
	err = auth.FromDsn("registry://user:pwd@docker.io:123")
	assert.NoError(t, err)
	assert.Equal(t, "registry://user:***@docker.io:123/?tlscheck=true", auth.ToMaskedDsn("***"))
	assert.Equal(t, "docker.io:123", auth.Authority)
	assert.Equal(t, "user", auth.Username)
	assert.Equal(t, "pwd", auth.Password)
	assert.Equal(t, "", auth.Token)
	assert.Equal(t, true, auth.TlsCheck)

	// user+pwd
	err = auth.FromDsn("registry://user:pwd@docker.io/?tlscheck=off")
	assert.NoError(t, err)
	assert.Equal(t, "registry://user:***@docker.io/?tlscheck=false", auth.ToMaskedDsn("***"))
	assert.Equal(t, "docker.io", auth.Authority)
	assert.Equal(t, "user", auth.Username)
	assert.Equal(t, "pwd", auth.Password)
	assert.Equal(t, "", auth.Token)
	assert.Equal(t, false, auth.TlsCheck)

	err = auth.FromDsn("registry://docker.io/?tlscheck=off")
	assert.NoError(t, err)
	assert.Equal(t, "docker.io", auth.Authority)
	assert.Equal(t, "", auth.Username)
	assert.Equal(t, "", auth.Password)
	assert.Equal(t, "", auth.Token)
	assert.Equal(t, false, auth.TlsCheck)
	assert.Equal(t, "registry://docker.io/?tlscheck=false", auth.ToMaskedDsn("***"))
}
