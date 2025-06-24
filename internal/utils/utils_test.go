package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDsn1(t *testing.T) {
	//var scheme string
	//var user string
	//var password string
	var host string
	var err error

	_, _, _, host, err = ParseDsn("redis://usr:pwd@host/0")
	assert.NoError(t, err)
	assert.Equal(t, "host", host)

	_, _, _, host, err = ParseDsn("redis://usr:pwd@host:6379/0")
	assert.NoError(t, err)
	assert.Equal(t, "host:6379", host)

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

func TestParseDsn2(t *testing.T) {

	testsSuccess := []string{
		"redis://usr:pwd@host:6379/0?m=mimi",
		"registry://usr:pwd@docker.io:6379/?checktls=on&m=mimi",
	}

	dsn := DataSourceName{}

	for _, input := range testsSuccess {
		err := dsn.Parse(input)
		assert.NoError(t, err)
		assert.Equal(t, "mimi", dsn.ParaOr("m", "n/a"))
		assert.Equal(t, "n/a", dsn.ParaOr("xi", "n/a"))
		assert.True(t, strings.Contains(dsn.Masked("***"), "***"))
		//assert.Equal(t, "***", dsn.Masked("***"))

	}

}
