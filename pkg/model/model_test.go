package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDSN(t *testing.T) {

	testsSucceed := []string{
		"",
		"registry://user:pwd@docker.io/?type=password",
	}
	testsFail := []string{
		"registry://user:pwd@docker.io?type=password",
		"registry://user:pwd@docker.io",
	}

	auth := PharosRepoAuth{}

	sample := "registry://user:pwd@docker.io/?type=password"
	err := auth.FromDsn(sample)
	assert.NoError(t, err)
	assert.Equal(t, "registry://user:***@docker.io/?type=password", auth.ToMaskedDsn("***"))

	// test success
	for _, dsn := range testsSucceed {
		err := auth.FromDsn(dsn)

		assert.NoError(t, err)
		assert.Equal(t, "docker.io", auth.Authority)
		assert.Equal(t, "user", auth.Username)
		assert.Equal(t, "pwd", auth.Password)
		assert.Equal(t, "", auth.Token)
	}
	// test fail
	for _, dsn := range testsFail {
		err := auth.FromDsn(dsn)

		assert.Error(t, err)
	}

}
