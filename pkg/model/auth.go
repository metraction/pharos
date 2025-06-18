package model

import (
	"fmt"
	"strings"

	"github.com/kos-v/dsnparser"
)

// authentication for image repos
type PharosRepoAuth struct {
	Authority string `json:"authority"`
	TlsCheck  bool   `json:"tlscheck"` // disable TLS cert check for authority
	Username  string `json:"username"`
	Password  string `json:"password"`
	Token     string `json:"token"`
}

func NewPharosRepoAuth(authDsn string, tlsCheck bool) (PharosRepoAuth, error) {
	auth := PharosRepoAuth{
		TlsCheck: tlsCheck,
	}
	if err := auth.FromDsn(authDsn); err != nil {
		return PharosRepoAuth{}, err
	}
	return auth, nil
}

// return true if auth is not empty and matchies imageRef repo
func (rx PharosRepoAuth) HasAuth(imageRef string) bool {

	if rx.Authority != "" {
		if rx.Username != "" || rx.Token != "" {
			if strings.HasPrefix(imageRef, rx.Authority) {
				return true
			}
		}
	}

	return false
}

// return DSN without password
func (rx PharosRepoAuth) ToMaskedDsn(mask string) string {
	if rx.Password != "" {
		return fmt.Sprintf("registry://%s:%s@%s", rx.Username, mask, rx.Authority)
	}
	if rx.Token != "" {
		return fmt.Sprintf("registry://%s:%s@%s", rx.Token, "", rx.Authority)
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

	// no input is avlid to streamline code flow in calls
	if input == "" {
		return nil
	}
	dsn := dsnparser.Parse(input)
	if dsn == nil {
		return fmt.Errorf("invalid registry dsn: %s", input)
	}
	// build authority
	rx.Authority = dsn.GetHost()
	if dsn.GetPort() != "" {
		rx.Authority = dsn.GetHost() + ":" + dsn.GetPort()
	}
	fmt.Printf("auth u=%s, p=%s, h=%s, p=%s\n", dsn.GetUser(), dsn.GetPassword(), dsn.GetHost(), dsn.GetPort())

	if dsn.GetUser() != "" && dsn.GetPassword() != "" {
		rx.Username = dsn.GetUser()
		rx.Password = dsn.GetPassword()
		return nil
	}
	if dsn.GetUser() != "" {
		rx.Token = dsn.GetUser()
		return nil
	}
	return fmt.Errorf("invalid auth dsn: %s", input)
}
