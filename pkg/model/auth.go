package model

import (
	"fmt"
	"strings"

	"github.com/kos-v/dsnparser"
	"github.com/metraction/pharos/internal/utils"
)

// from list of AuthDSNs return the first that matches the image host
// imageSpec docker.io/nginx:1.20
// auths []"registry://user:pwd@pharos.secimo.net
func GetMatchingAuthDsn(imageSpec string, auths []string) string {

	imageHost := utils.LeftOfFirstOr(imageSpec, "/", "")
	if imageHost == "" {
		return ""
	}
	for _, auth := range auths {
		authHost := utils.DsnHostPortOr(auth, "")
		if authHost == "" {
			continue
		}
		if imageHost == authHost {
			return auth
		}
	}
	return ""
}

func GetMatchingAuth(imageSpec string, auths []PharosRepoAuth) PharosRepoAuth {

	parts := strings.Split(imageSpec, "/")
	if len(parts) == 0 {
		return PharosRepoAuth{}
	}
	repoHost := parts[0]
	for _, auth := range auths {

		if strings.HasPrefix(auth.Authority+"/", repoHost+"/") {
			return auth
		}
	}
	return PharosRepoAuth{}
}

// authentication for image repos
// TODO: here, json tags have lowercase names, but other models use PascalCase names.
type PharosRepoAuth struct {
	Authority string `json:"authority" required:"false"`
	Username  string `json:"username" required:"false"`
	Password  string `json:"password" required:"false"`
	Token     string `json:"token" required:"false"`
	//
	TlsCheck bool `json:"tlscheck" required:"false"` // disable TLS cert check for authority
}

func NewPharosRepoAuth(authDsn string) (PharosRepoAuth, error) {
	auth := PharosRepoAuth{
		TlsCheck: true,
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

	if rx.Token != "" {
		return fmt.Sprintf("registry://%s:%s@%s/?tlscheck=%v", rx.Token, "", rx.Authority, rx.TlsCheck)
	}

	if rx.Password != "" {
		return fmt.Sprintf("registry://%s:%s@%s/?tlscheck=%v", rx.Username, mask, rx.Authority, rx.TlsCheck)
	}
	if rx.Authority != "" {
		return fmt.Sprintf("registry://%s/?tlscheck=%v", rx.Authority, rx.TlsCheck)
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
	rx.TlsCheck = true

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
	// only set tlscheck if given:
	if dsn.GetParam("tlscheck") != "" {
		rx.TlsCheck = utils.ToBool(dsn.GetParam("tlscheck"))
	}

	if rx.Authority != "" {
		if dsn.GetUser() != "" && dsn.GetPassword() != "" {
			rx.Username = dsn.GetUser()
			rx.Password = dsn.GetPassword()
			return nil
		}
		if dsn.GetUser() != "" {
			rx.Token = dsn.GetUser()
			return nil
		}
		return nil
	}
	return fmt.Errorf("invalid auth dsn: %s", input)
}
