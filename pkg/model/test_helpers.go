package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newTestScanTask is a test helper that creates a PharosScanTask with standard defaults.
func NewTestScanTask(t *testing.T, taskID, image string) PharosScanTask {
	t.Helper()
	task, err := NewPharosScanTask(
		taskID,
		image,
		"",               // platform
		PharosRepoAuth{}, // auth
		1*time.Hour,      // cache expiry
		30*time.Second,   // scan timeout
	)
	require.NoError(t, err)
	return task
}

// newTestScanResult is a test helper that creates a PharosScanResult for a given task and engine name.
func NewTestScanResult(task PharosScanTask, engineName string) PharosScanResult {
	return PharosScanResult{
		Version:  "1.0",
		ScanTask: task,
		ScanEngine: PharosScanEngine{
			Name:    engineName,
			Version: "1.0",
		},
		Image: PharosImageMeta{
			ImageSpec:   task.ImageSpec.Image,
			ImageId:     "test-image-id",
			IndexDigest: "sha256:test",
		},
		Findings:        []PharosScanFinding{},
		Vulnerabilities: []PharosVulnerability{},
		Packages:        []PharosPackage{},
	}
}
