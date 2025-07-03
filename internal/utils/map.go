package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// resolve {{.alfa.bravo}} with data context
func ResolveMap(input string, data map[string]any) string {

	re := regexp.MustCompile(`{{\.(\w+(?:\.\w+)*)}}`)
	result := input

	for _, match := range re.FindAllStringSubmatch(input, -1) {
		// resolve key/path
		key := strings.Replace(match[1], ".", "/", -1)
		val := PathStrOr[any](key, "", data)
		result = strings.Replace(result, match[0], val, -1)
	}
	return result
}

// return element at path "alfa/bravo" or default
func PathStrOr[T any](path, defval string, data map[string]any) string {
	keys := strings.Split(path, "/")
	var current any = data
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return defval
		}
		current, ok = m[key]
		if !ok {
			return defval
		}
	}
	return fmt.Sprintf("%v", current)
}

// return value at key or default
func PropOr[T any](xc map[string]any, key string, defval T) T {
	value, ok := xc[key].(T)
	if !ok {
		return defval
	}
	return value
}

// set element at path "alfa/bravo"
func SetPath(m map[string]any, path string, value any) {
	keys := strings.Split(path, "/")
	last := len(keys) - 1
	curr := m

	for i, k := range keys {
		if i == last {
			curr[k] = value
			return
		}
		// If the key doesn't exist or isn't a map, create a new map
		if next, ok := curr[k]; ok {
			if nextMap, ok := next.(map[string]any); ok {
				curr = nextMap
			} else {
				// Overwrite non-map value with a new map
				newMap := make(map[string]any)
				curr[k] = newMap
				curr = newMap
			}
		} else {
			newMap := make(map[string]any)
			curr[k] = newMap
			curr = newMap
		}
	}
}
