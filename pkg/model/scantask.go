package model

import "time"

type PharosScanTask struct {
	JobId      string          `json:"jobId" required:"false"` // jobid for batch jobs tracking
	Auth       PharosRepoAuth  `json:"auth" required:"false"`
	ImageSpec  PharosImageSpec `json:"imageSpec" required:"true"`
	Timeout    time.Duration   `json:"timeout" required:"false"` // scan timeout in sec
	Created    time.Time       `json:"created" required:"false"`
	Updated    time.Time       `json:"updated" required:"false"`
	SbomEngine string          `json:"sbomEngine" required:"false"` // SBOM generator tool
	ScanEngine string          `json:"scanEngine" required:"false"` // Scan generator tool
	Status     string          `json:"status" required:"false"`
	Error      string          `json:"error" required:"false"`
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
	Image       string         `json:"image" required:"true"`
	Platform    string         `json:"platform" required:"false"`
	CacheExpiry time.Duration  `json:"cacheExpiry" required:"false"` // cache expiry in sec
	Context     map[string]any `json:"context" required:"false"`
}
