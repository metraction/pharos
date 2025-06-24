package utils

import (
	"fmt"
	"os"
)

func Hostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		panic(fmt.Sprintf("Error: Hostname(): %v", err))
	}
	return hostname
}

// Return true if path exists and is a direcroty
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && info.IsDir()
}
