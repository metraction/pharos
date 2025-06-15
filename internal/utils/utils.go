package utils

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/joho/godotenv"
	"github.com/kos-v/dsnparser"
	"github.com/package-url/packageurl-go"
	"github.com/samber/lo"
)

// return left of digest, e.g. "sha256:f85340bf132ae1"
func ShortDigest(input string) string {
	return lo.Substring(input, 0, 19)
}

// return true if input string is one of 1, t, true, on, yes
func ToBool(input string) bool {
	input = strings.TrimSpace(input)
	input = strings.ToLower(input)

	if lo.Contains([]string{"1", "t", "true", "on", "yes"}, input) {
		return true
	}
	return false
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

// return function (closure) thats returns the <prefix>_<name> envvar if it exists, else the default value
func EnvOrDefaultFunc(prefix string) func(string, string) string {

	// load .env if it exists
	err := godotenv.Load()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Println("error", err)
			log.Fatal("Error loading .env file")
		}
	}
	return func(name, defval string) string {
		key := strings.ToUpper(prefix + "_" + name)
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defval
	}
}

// remove ansi color codes from string (from console output)
func NoColorCodes(input string) string {
	return stripansi.Strip(input)
}

// return true if given program is installed (found in $PATH)
func IsInstalled(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func OsWhich(cmd string) (string, error) {
	path, err := exec.LookPath(cmd)
	if err != nil {
		return "", fmt.Errorf("%s not found in PATH", cmd)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("error getting absolute path: %s", cmd)

	}
	return absPath, nil

}

func ElapsedFunc() func() time.Duration {
	startTime := time.Now()
	return func() time.Duration {
		return time.Since(startTime)
	}
}

// return humanized time delat rounded to minuts (not to have like 1h12m1.112521806s)
func HumanDeltaMin(delta time.Duration) string {
	return delta.Round(time.Minute).String()
}
func HumanDeltaSec(delta time.Duration) string {
	return delta.Round(time.Second).String()
}
func HumanDeltaMilisec(delta time.Duration) string {
	return delta.Round(10 * time.Millisecond).String()
}

// Number conversion

// parse string, return number or default
func UInt64Or(input string, defval uint64) uint64 {
	if u, err := strconv.ParseUint(input, 10, 64); err == nil {
		return u
	}
	return defval
}

// parse string, return time.Time or defauls
func DateStrOr(input string, defval time.Time) time.Time {
	if t, err := time.Parse("2006-01-02", input); err == nil {
		return t
	}
	return defval
}

// decode purl encoding,
// from pkg:deb/debian/adduser@3.134?arch=all\u0026distro=debian-12
//   to pkg:deb/debian/adduser@3.134?arch=all&distro=debian-12

func DecodePurl(input string) string {
	purl, err := packageurl.FromString(input)
	if err != nil {
		return input
	}
	return purl.ToString()
}
