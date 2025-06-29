package utils

import "strings"

// return value at key or default
func PropOr[T any](xc map[string]any, key string, defval T) T {
	value, ok := xc[key].(T)
	if !ok {
		return defval
	}
	return value
}

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
