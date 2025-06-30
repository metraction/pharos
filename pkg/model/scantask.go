package model

import "time"

// Stefan 2025-06-29
// new flat model to reduce complexity, facilitate tesintg, and add context
// this object is used mainly to submit tasks to scanner and report status back
// JobId: correlation ID for tracking state of tasks
// AuthDsn: registry://user:pwd@repo.host.net/?tlscheck=off

type PharosScanTask2 struct {
	// task status
	JobId  string `json:"jobId" required:"false"` // jobid for batch jobs tracking
	Status string `json:"status" required:"false"`
	Engine string `json:"engine" required:"false"`
	Error  string `json:"error" required:"false"`
	// image
	AuthDsn    string         `json:"authdsn"`
	ImageSpec  string         `json:"imagespec" required:"true"`
	Platform   string         `json:"platform" required:"false"`
	Context    map[string]any `json:"context" required:"false"`
	RxDigest   string         `json:"rxdigest" required:"false"`   // manifest digest retrieved from repo
	RxPlatform string         `json:"rxplatform" required:"false"` // platform retrieved from repo

	// scanner
	CacheTTL time.Duration `json:"cachettl" required:"false"` // cache expiry in sec
	ScanTTL  time.Duration `json:"scanttl" required:"false"`  // cache expiry in sec

	// TODO - this is ORM fields, can we remove this?
	Timeout time.Duration `json:"timeout" required:"false"` // scan timeout in sec
	Created time.Time     `json:"created" required:"false"`
	Updated time.Time     `json:"updated" required:"false"`
}

// set error and status
func (rx *PharosScanTask2) SetError(err error) *PharosScanTask2 {
	rx.Status = "error"
	rx.Error = err.Error()
	return rx
}

// legacy
type XXPharosScanTask struct {
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

// func DeleteNewPharosScanTask(jobId, imageRef, platform string, auth PharosRepoAuth, cacheExpiry, scanTimeout time.Duration) (PharosScanTask, error) {
// 	now := time.Now().UTC()

// 	task := PharosScanTask{
// 		JobId: jobId,
// 		Auth:  auth,
// 		ImageSpec: PharosImageSpec{
// 			Image:       imageRef,
// 			Platform:    platform,
// 			CacheExpiry: cacheExpiry,
// 		},
// 		Timeout: scanTimeout,
// 		Created: now,
// 		Updated: now,
// 		Status:  "new",
// 	}
// 	return task, nil
// }

// func (rx *PharosScanTask) SetStatus(status string) {
// 	rx.Status = status
// 	rx.Updated = time.Now().UTC()
// }

type PharosImageSpec struct {
	Image       string         `json:"image" required:"true"`
	Platform    string         `json:"platform" required:"false"`
	CacheExpiry time.Duration  `json:"cacheExpiry" required:"false"` // cache expiry in sec
	Context     map[string]any `json:"context" required:"false"`
}
