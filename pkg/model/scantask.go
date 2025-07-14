package model

import "time"

// Stefan 2025-06-29
// new flat model to reduce complexity, facilitate tesintg, and add context
// this object is used mainly to submit tasks to scanner and report status back
// JobId: correlation ID for tracking state of tasks
// AuthDsn: registry://user:pwd@repo.host.net/?tlscheck=off

type PharosScanTask2 struct {
	// task status
	JobId  string `json:"jobId" required:"false" default:"" doc:"you can give a job id here to track the job."` // jobid for batch jobs tracking
	Status string `json:"status" required:"false" readOnly:"true"`
	Engine string `json:"engine" required:"false" default:"grype" doc:"scanner engine used for the scan, e.g. trivy, grype, .."` // scanner engine used for the scan
	Error  string `json:"error" required:"false" readOnly:"true"`
	// image
	AuthDsn        string         `json:"authdsn" required:"false" default:"registry:///?tlscheck=false"` // TODO: has to be documented.
	ImageSpec      string         `json:"imagespec" required:"true"`
	Platform       string         `json:"platform" required:"false" default:"linux/amd64"`
	Context        map[string]any `json:"context" required:"false" doc:"context data for the scan, e.g. namespace, labels, .." default:"{}"`
	ContextRootKey string         `json:"contextRootKey" required:"false" default:""`  // key to the context root, if any
	RxDigest       string         `json:"rxdigest" required:"false" readOnly:"true"`   // manifest digest retrieved from repo
	RxPlatform     string         `json:"rxplatform" required:"false" readOnly:"true"` // platform retrieved from repo

	// scanner
	CacheTTL time.Duration `json:"cachettl" yaml:"cachettl" required:"false" default:"86400000000000" doc:"how long to cache sbom in scanner (nanoseconds)"` // cache expiry in sec
	ScanTTL  time.Duration `json:"scanttl" yaml:"scanttl" required:"false" default:"300000000000" doc:"how long to scan result in scanner (nanoseconds)"`  // cache expiry in sec

	Created  time.Time              `json:"created" yaml:"created" required:"false"`
	Updated  time.Time              `json:"updated" yaml:"updated" required:"false"`
	receiver *chan PharosScanResult // receiver channel to send results to when doing async scan
}

func (pt *PharosScanTask2) SetReceiver(ch *chan PharosScanResult) {
	pt.receiver = ch
}

func (pt *PharosScanTask2) GetReceiver() *chan PharosScanResult {
	return pt.receiver
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

type PharosImageSpec struct {
	Image       string         `json:"image" required:"true"`
	Platform    string         `json:"platform" required:"false"`
	CacheExpiry time.Duration  `json:"cacheExpiry" required:"false"` // cache expiry in sec
	Context     map[string]any `json:"context" required:"false"`
}
