package utils

import (
	"os"
)

// Return true if path exists and is a direcroty
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && info.IsDir()
}
