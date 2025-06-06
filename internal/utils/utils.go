package utils

import (
	"os"
	"strings"
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
