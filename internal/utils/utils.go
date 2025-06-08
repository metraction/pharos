package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/acarl005/stripansi"
)

// return function (closure) thats returns the <prefix>_<name> envvar if it exists, else the default value
func EnvOrDefaultFunc(prefix string) func(string, string) string {
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
