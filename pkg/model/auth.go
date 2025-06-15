package model

import (
	"fmt"

	"github.com/kos-v/dsnparser"
)

// authentication for image repos
type PharosRepoAuth struct {
	Authority string `json:"authority"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Token     string `json:"token"`
}

// return true if auth is not empty
func (rx PharosRepoAuth) HasAuth() bool {
	return rx.Authority != "" && (rx.Username != "" || rx.Token != "")
}

// return DSN without password
func (rx PharosRepoAuth) ToMaskedDsn(mask string) string {
	if rx.Password != "" {
		return fmt.Sprintf("registry://%s:%s@%s/?type=password", rx.Username, mask, rx.Authority)
	}
	if rx.Token != "" {
		return fmt.Sprintf("registry://%s:%s@%s/?type=token", rx.Username, mask, rx.Authority)
	}
	return ""
}

// parse DSN
// registry://user:password@docker.io/type=password
// registry://user:token@docker.io/type=token
func (rx *PharosRepoAuth) FromDsn(input string) error {

	// reset
	rx.Authority = ""
	rx.Username = ""
	rx.Password = ""
	rx.Token = ""

	if input == "" {
		return nil
	}
	dsn := dsnparser.Parse(input)
	if dsn == nil {
		return fmt.Errorf("invalid registry %v", input)
	}
	// build authority
	rx.Authority = dsn.GetHost()
	if dsn.GetPort() != "" {
		rx.Authority = dsn.GetHost() + ":" + dsn.GetPort()
	}
	what := dsn.GetParam("type")
	if what == "password" {
		rx.Username = dsn.GetUser()
		rx.Password = dsn.GetPassword()
		return nil
	} else if what == "token" {
		rx.Token = dsn.GetPassword()
		return nil
	} else {
		return fmt.Errorf("invalid DSN type '%s' (e.g. 'registry://usr:pwd@docker.io/?type=password')", what)
	}

}
