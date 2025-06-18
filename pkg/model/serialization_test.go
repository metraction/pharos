package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPharosScanResultSerialization(t *testing.T) {
	// Create a test scan task
	taskID := "test-job-123"
	imageRef := "alpine:latest"
	task, err := NewPharosScanTask(
		taskID,
		imageRef,
		"linux/amd64",
		PharosRepoAuth{},
		1*time.Hour,
		30*time.Second,
	)
	require.NoError(t, err)

	// Create a test scan result with all nested types
	original := PharosScanResult{
		Version:  "1.0",
		ScanTask: task,
		ScanEngine: PharosScanEngine{
			Name:    "test-engine",
			Version: "1.0.0",
		},
		Image: PharosImageMeta{
			ImageId:   "sha256:abc123",
			Digest:    "sha256:def456",
			ImageSpec: imageRef,
			Size:      10240,
		},
		Findings: []PharosScanFinding{
			{
				AdvId:       "CVE-2023-1234",
				AdvSource:   "NVD",
				ScanDate:    time.Now(),
				UpdateDate:  time.Now(),
				Severity:    "HIGH",
				FixState:    "UNFIXED",
				FixVersions: []string{"1.1.1h"},
				FoundIn:     []string{"/usr/lib/openssl"},
			},
			{
				AdvId:       "CVE-2023-5678",
				AdvSource:   "NVD",
				ScanDate:    time.Now(),
				UpdateDate:  time.Now(),
				Severity:    "MEDIUM",
				FixState:    "UNFIXED",
				FixVersions: []string{"5.0.18"},
				FoundIn:     []string{"/bin/bash"},
			},
		},
		Vulnerabilities: []PharosVulnerability{
			{
				AdvId:       "CVE-2023-1234",
				AdvSource:   "NVD",
				CreateDate:  time.Now(),
				PubDate:     time.Now(),
				ModDate:     time.Now(),
				Description: "A critical vulnerability in OpenSSL",
				Severity:    "HIGH",
				CvssBase:    9.8,
			},
			{
				AdvId:       "CVE-2023-5678",
				AdvSource:   "NVD",
				CreateDate:  time.Now(),
				PubDate:     time.Now(),
				ModDate:     time.Now(),
				Description: "A medium severity vulnerability in Bash",
				Severity:    "MEDIUM",
				CvssBase:    5.5,
			},
		},
		Packages: []PharosPackage{
			{
				Key:     "openssl@1.1.1g",
				Name:    "openssl",
				Version: "1.1.1g",
				Type:    "binary",
				Purl:    "pkg:deb/debian/openssl@1.1.1g",
				Cpes:    []string{"cpe:/a:openssl:openssl:1.1.1g"},
			},
			{
				Key:     "bash@5.0.17",
				Name:    "bash",
				Version: "5.0.17",
				Type:    "binary",
				Purl:    "pkg:deb/debian/bash@5.0.17",
				Cpes:    []string{"cpe:/a:gnu:bash:5.0.17"},
			},
		},
	}

	// Test MarshalBinary
	t.Run("MarshalBinary", func(t *testing.T) {
		data, err := original.MarshalBinary()
		require.NoError(t, err)
		require.NotEmpty(t, data)

		// Verify the data is valid JSON (since we're using JSON under the hood)
		var jsonMap map[string]interface{}
		err = json.Unmarshal(data, &jsonMap)
		require.NoError(t, err)

		// Verify the serializable structure has the expected fields
		assert.Contains(t, jsonMap, "Version")
		assert.Contains(t, jsonMap, "ScanTask")
		assert.Contains(t, jsonMap, "ScanEngine")
		assert.Contains(t, jsonMap, "Image")
		assert.Contains(t, jsonMap, "Findings")
		assert.Contains(t, jsonMap, "Vulnerabilities")
		assert.Contains(t, jsonMap, "Packages")
	})

	// Test UnmarshalBinary
	t.Run("UnmarshalBinary", func(t *testing.T) {
		// First marshal the original
		data, err := original.MarshalBinary()
		require.NoError(t, err)

		// Then unmarshal into a new instance
		var deserialized PharosScanResult
		err = deserialized.UnmarshalBinary(data)
		require.NoError(t, err)

		// Verify all fields were correctly deserialized
		assert.Equal(t, original.Version, deserialized.Version)
		assert.Equal(t, original.ScanTask.JobId, deserialized.ScanTask.JobId)
		assert.Equal(t, original.ScanEngine.Name, deserialized.ScanEngine.Name)
		assert.Equal(t, original.ScanEngine.Version, deserialized.ScanEngine.Version)

		// Verify Image metadata
		assert.Equal(t, original.Image.ImageId, deserialized.Image.ImageId)
		assert.Equal(t, original.Image.Digest, deserialized.Image.Digest)
		assert.Equal(t, original.Image.ImageSpec, deserialized.Image.ImageSpec)
		assert.Equal(t, original.Image.Size, deserialized.Image.Size)

		// Verify slices have the correct length
		assert.Equal(t, len(original.Findings), len(deserialized.Findings))
		assert.Equal(t, len(original.Vulnerabilities), len(deserialized.Vulnerabilities))
		assert.Equal(t, len(original.Packages), len(deserialized.Packages))

		// Verify contents of Findings slice
		if len(original.Findings) > 0 {
			assert.Equal(t, original.Findings[0].AdvId, deserialized.Findings[0].AdvId)
			assert.Equal(t, original.Findings[0].AdvSource, deserialized.Findings[0].AdvSource)
			assert.Equal(t, original.Findings[0].Severity, deserialized.Findings[0].Severity)
			assert.Equal(t, original.Findings[0].FixState, deserialized.Findings[0].FixState)
			assert.Equal(t, len(original.Findings[0].FixVersions), len(deserialized.Findings[0].FixVersions))
			assert.Equal(t, len(original.Findings[0].FoundIn), len(deserialized.Findings[0].FoundIn))
		}

		// Verify contents of Vulnerabilities slice
		if len(original.Vulnerabilities) > 0 {
			assert.Equal(t, original.Vulnerabilities[0].AdvId, deserialized.Vulnerabilities[0].AdvId)
			assert.Equal(t, original.Vulnerabilities[0].AdvSource, deserialized.Vulnerabilities[0].AdvSource)
			assert.Equal(t, original.Vulnerabilities[0].Description, deserialized.Vulnerabilities[0].Description)
			assert.Equal(t, original.Vulnerabilities[0].Severity, deserialized.Vulnerabilities[0].Severity)
			assert.Equal(t, original.Vulnerabilities[0].CvssBase, deserialized.Vulnerabilities[0].CvssBase)
		}

		// Verify contents of Packages slice
		if len(original.Packages) > 0 {
			assert.Equal(t, original.Packages[0].Key, deserialized.Packages[0].Key)
			assert.Equal(t, original.Packages[0].Name, deserialized.Packages[0].Name)
			assert.Equal(t, original.Packages[0].Version, deserialized.Packages[0].Version)
			assert.Equal(t, original.Packages[0].Type, deserialized.Packages[0].Type)
			assert.Equal(t, original.Packages[0].Purl, deserialized.Packages[0].Purl)
			assert.Equal(t, len(original.Packages[0].Cpes), len(deserialized.Packages[0].Cpes))
		}
	})

	// Test round-trip with Redis binary marshaling
	t.Run("RoundTrip", func(t *testing.T) {
		// Marshal to binary
		data, err := original.MarshalBinary()
		require.NoError(t, err)

		// Unmarshal from binary
		var deserialized PharosScanResult
		err = deserialized.UnmarshalBinary(data)
		require.NoError(t, err)

		// Marshal the deserialized object back to binary
		data2, err := deserialized.MarshalBinary()
		require.NoError(t, err)

		// Unmarshal the second binary into a third object
		var final PharosScanResult
		err = final.UnmarshalBinary(data2)
		require.NoError(t, err)

		// Verify the final object matches the original
		assert.Equal(t, original.Version, final.Version)
		assert.Equal(t, original.ScanTask.JobId, final.ScanTask.JobId)
		assert.Equal(t, len(original.Findings), len(final.Findings))
		assert.Equal(t, len(original.Vulnerabilities), len(final.Vulnerabilities))
		assert.Equal(t, len(original.Packages), len(final.Packages))
	})
}
