package utils

import (
	"net/url"
	"strings"
)

// return host:port from DSN like "docker://user:pwd@pharos.alfa.lan:123/?mi=off"
func GetHostPortOr(input, defval string) string {
	dsn, err := url.Parse(input)
	if err != nil {
		return defval
	}
	if dsn.Port() != "" {
		return dsn.Hostname() + ":" + dsn.Port()
	}
	return dsn.Hostname()
}

// return DSN with password masked as ***
func MaskDsn(input string) string {
	dsn, err := url.Parse(input)
	if err != nil {
		return input
	}
	if password, ok := dsn.User.Password(); ok {
		return strings.Replace(input, ":"+password+"@", ":***@", 1)
	}
	return input
}
