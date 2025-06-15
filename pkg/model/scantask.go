package model

import "time"

type PharosImageScanTask struct {
	JobId     string          `json:"jobId"` // jobid for batch jobs tracking
	Auth      PharosRepoAuth  `json:"auth"`
	ImageSpec PharosImageSpec `json:"imageSpec"`
	Timeout   time.Duration   `json:"timeout"` // scan timeout in sec
}

type PharosImageSpec struct {
	Image       string         `json:"image"`
	Platform    string         `json:"platform"`
	CacheExpiry time.Duration  `json:"cacheExpiry"` // cache expiry in sec
	Context     map[string]any `json:"context"`
}
