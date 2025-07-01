package model

import (
	"testing"
	"time"
)

func NewTestScanTask(t *testing.T, taskID, image string) PharosScanTask2 {
	t.Helper()
	task := PharosScanTask2{
		JobId:     taskID,
		ImageSpec: image,
		ScanTTL:   30 * time.Second,
		CacheTTL:  1 * time.Hour,
		Engine:    "test-engine",
		Timeout:   1 * time.Minute,
	}
	return task
}

// newTestScanResult is a test helper that creates a PharosScanResult for a given task and engine name.
func NewTestScanResult(task PharosScanTask2, engineName string) PharosScanResult {
	return PharosScanResult{
		Version:  "1.0",
		ScanTask: task,
		Image: PharosImageMeta{
			ImageSpec:      task.ImageSpec,
			ImageId:        "test-image-id",
			IndexDigest:    "sha256:test",
			ManifestDigest: "sha256:62b6b206d9514119fc410baf83ee96314e9f328790fdf934b24ffa88a240bbb3",
			ArchName:       "amd64",
			ArchOS:         "linux",
			DistroName:     "debian",
			DistroVersion:  "12",
		},
		Findings:        []PharosScanFinding{},
		Vulnerabilities: []PharosVulnerability{},
		Packages:        []PharosPackage{},
	}
}
