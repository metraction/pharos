package grype

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/metraction/pharos/internal/utils"
	cpy "github.com/otiai10/copy"
)

// helper
func TranslateMessage(msg string) string {
	// translate as original messages that are missleading is missleading
	msg = strings.Replace(msg, "No vulnerability database update available", "OK, no update required", 1)
	msg = strings.TrimSpace(msg)
	return msg
}

// grype update check (from: grype db check -o json)
type GrypeDbCheck struct {
	UpdateAvailable bool `json:"updateAvailable"`
}

// grype version check (from: grype version -o json)
type GrypeVersion struct {
	Application  string    `json:"application"`
	BuildDate    time.Time `json:"buildDate"`
	Platform     string    `json:"platform"`
	GrypeVersion string    `json:"version"`
	SyftVersion  string    `json:"syftVersion"`
	//SupportedDbSchema int       `json:"supportedDbSchema"`
}

// grype local database status (from: grype db status -o json)
type GrypeDbStatus struct {
	SchemaVersion string    `json:"schemaVersion"`
	From          string    `json:"from"`
	Built         time.Time `json:"built"`
	Path          string    `json:"path"`
	Valid         bool      `json:"valid"`
	Error         string    `json:"error"`
}

// run grype executable, parse result from json into T
func GrypeExeOutput[T any](cmd *exec.Cmd) (T, error) {

	var result T
	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// execute command
	err1 := cmd.Run()
	err2 := json.Unmarshal(stdout.Bytes(), &result)

	if err1 != nil || err2 != nil {
		msg := TranslateMessage(stderr.String())
		return result, fmt.Errorf("%s", utils.NoColorCodes(msg))
	}
	return result, nil
}

// check grype binary version
func GetScannerVersion(scannerBin string) (string, error) {

	cmd := exec.Command(scannerBin, "version", "-o", "json")

	result, err := GrypeExeOutput[GrypeVersion](cmd)
	if err != nil {
		return "", err
	}
	return result.GrypeVersion, nil
}

// check grype local database status, update DbState
func GetDatabaseStatus(scannerBin string) (string, time.Time, error) {

	cmd := exec.Command(scannerBin, "db", "status", "-o", "json")

	result, err := GrypeExeOutput[GrypeDbStatus](cmd)
	if err != nil {
		return "", time.Time{}, err
	}
	return result.SchemaVersion, result.Built, nil
}

// check if local db in targetDir requires an update
func GrypeUpdateRequired(scannerBin, targetDir string) bool {

	cmd := exec.Command(scannerBin, "db", "check", "-o", "json")
	cmd.Env = append(cmd.Env, "GRYPE_DB_CACHE_DIR="+targetDir)

	grypeCheck, err := GrypeExeOutput[GrypeDbCheck](cmd)
	if err != nil {
		return true
	}
	return grypeCheck.UpdateAvailable
}

// check if update is required, if so download to targetDir
func GetGrypeUpdate(scannerBin, targetDir string) error {

	// if !GrypeUpdateRequired(scannerBin, targetDir) {
	// 	return nil
	// }
	// do update
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(scannerBin, "db", "update")
	cmd.Env = append(cmd.Env, "GRYPE_DB_CACHE_DIR="+targetDir)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("grype update %s [%s]", utils.NoColorCodes(stderr.String()), targetDir)
	}
	return nil
}

// deploy staged update from sourceDir into targetDir
func DeployStagedUpdate(sourceDir, targetDir string) error {
	// validate
	if !utils.DirExists(sourceDir) {
		return fmt.Errorf("deploy staged update: source not found: %v", sourceDir)
	}
	// delete target, then copy source to target
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("deploy staged update: source not found: %v", sourceDir)
	}
	// copy
	if err := cpy.Copy(sourceDir, targetDir); err != nil {
		return fmt.Errorf("deploy staged update: cannot copy to %v", targetDir)
	}
	return nil
}
