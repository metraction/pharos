package utils

import (
	"strings"
)

// return right of key of input or default
func RightOfFirstOr(input, key, defval string) string {
	if key == "" {
		return input
	}

	k := strings.Index(input, key)
	if k < 0 {
		return defval
	}
	return input[k+len(key):]
}

// return left of key of input or default
func LeftOfFirstOr(input, key, defval string) string {
	if key == "" {
		return ""
	}
	k := strings.Index(input, key)
	if k < 0 {
		return defval
	}
	return input[:k]
}

// return right of <prefix> of string or default
func RightOfPrefixOr(input, prefix, defval string) string {
	if !strings.HasPrefix(input, prefix) {
		return defval
	}
	return strings.TrimSpace(input[len(prefix):])
}
