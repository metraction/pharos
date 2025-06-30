package utils

import (
	"fmt"
	"net/url"
	"strings"
)

// return host:port from DSN like "docker://user:pwd@pharos.alfa.lan:123/?mi=off"
func DsnHostPortOr(input, defval string) string {
	dsn, err := url.Parse(input)
	if err != nil {
		return defval
	}
	if dsn.Port() != "" {
		return dsn.Hostname() + ":" + dsn.Port()
	}
	return dsn.Hostname()
}

func DsnUserOr(input, defval string) string {
	dsn, err := url.Parse(input)
	if err != nil {
		return defval
	}
	return dsn.User.Username()
}

func DsnParaOr(input, key, defval string) string {
	dsn, err := url.Parse(input)
	if err != nil {
		return defval
	}
	params := dsn.Query()
	value := params.Get(key)
	if value == "" {
		return defval
	}
	return value
}

func DsnParaBoolOr(input, key string, defval bool) bool {
	value := DsnParaOr(input, key, fmt.Sprintf("%t", defval))
	return ToBool(value)
}

func DsnPasswordOr(input, defval string) string {
	dsn, err := url.Parse(input)
	if err != nil {
		return defval
	}
	if password, ok := dsn.User.Password(); ok {
		return password
	}
	return defval
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
