package model

import "time"

type PharosScanTask struct {
	JobId      string          `json:"jobId"` // jobid for batch jobs tracking
	Auth       PharosRepoAuth  `json:"auth"`
	ImageSpec  PharosImageSpec `json:"imageSpec"`
	Timeout    time.Duration   `json:"timeout"` // scan timeout in sec
	Created    time.Time       `json:"created"`
	Updated    time.Time       `json:"updated"`
	SbomEngine string          `json:"sbomEngine"` // SBOM generator tool
	ScanEngine string          `json:"scanEngine"` // Scan generator tool
	Status     string          `json:"status"`
	Error      string          `json:"error"`
}

func NewPharosScanTask(jobId, imageRef, platform string, auth PharosRepoAuth, cacheExpiry, scanTimeout time.Duration) (PharosScanTask, error) {
	now := time.Now().UTC()

	task := PharosScanTask{
		JobId: jobId,
		Auth:  auth,
		ImageSpec: PharosImageSpec{
			Image:       imageRef,
			Platform:    platform,
			CacheExpiry: cacheExpiry,
		},
		Timeout: scanTimeout,
		Created: now,
		Updated: now,
		Status:  "new",
	}
	return task, nil
}

func (rx *PharosScanTask) SetStatus(status string) {
	rx.Status = status
	rx.Updated = time.Now().UTC()
}

type PharosImageSpec struct {
	Image       string         `json:"image"`
	Platform    string         `json:"platform"`
	CacheExpiry time.Duration  `json:"cacheExpiry"` // cache expiry in sec
	Context     map[string]any `json:"context"`
}
