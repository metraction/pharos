package utils

import (
	"fmt"
	"strings"

	"github.com/kos-v/dsnparser"
	"github.com/samber/lo"
)

type DataSourceName struct {
	Endpoint string
	dsn      *dsnparser.DSN
}

func (rx *DataSourceName) Parse(input string) error {

	rx.dsn = dsnparser.Parse(input)
	rx.Endpoint = ""
	if rx.dsn == nil {
		return fmt.Errorf("invalid DSN '%s'", input)
	}
	rx.Endpoint = input
	return nil
}
func (rx *DataSourceName) ParaOr(key, defval string) string {
	if rx.dsn == nil {
		return defval
	}
	val := rx.dsn.GetParam(key)
	return lo.Ternary(val != "", val, defval)
}

// return DSN with password masked as ***
func (rx *DataSourceName) Masked(mask string) string {
	if rx.dsn == nil {
		return ""
	}
	return strings.Replace(rx.Endpoint, ":"+rx.dsn.GetPassword()+"@", ":"+mask+"@", 1)
}

// return service, user, password, host from
//
//		redis://pwd@localhost:6379/0
//	 registry://usr:pwd@docker.io/?type=password
func ParseDsn(input string) (string, string, string, string, error) {

	dsn := dsnparser.Parse(input)
	if dsn == nil {
		return "", "", "", "", fmt.Errorf("invalid DSN '%s'", input)
	}
	hostPort := dsn.GetHost()
	if dsn.GetPort() != "" {
		hostPort = dsn.GetHost() + ":" + dsn.GetPort()
	}
	return dsn.GetScheme(), dsn.GetUser(), dsn.GetPassword(), hostPort, nil
}

// return DSN with password masked as ***
func MaskDsn(input string) string {
	_, _, password, _, _ := ParseDsn(input)
	if password == "" {
		return input
	}
	return strings.Replace(input, ":"+password+"@", ":***@", 1)
}
