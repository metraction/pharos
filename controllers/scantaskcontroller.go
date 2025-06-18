package controllers

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/metraction/pharos/pkg/model"
)

type ScanTaskController struct {
	Path string
	Api  *huma.API
}

type ScanTaskSubmitOutput struct {
	Body ScanTaskSubmitOutputBody `json:"body"`
}

type ScanTaskSubmitOutputBody struct {
	ScanTask model.PharosScanTask `json:"scantask"`
	Message  string               `json:"message,omitempty" example:"Scan task submitted successfully"`
}

func NewScanTaskController(group *huma.API) *ScanTaskController {
	sc := &ScanTaskController{
		Path: "/scantask",
		Api:  group,
	}
	return sc
}

func (sc *ScanTaskController) AddRoutes() {
	// {
	// 	op, handler := sc.Get()
	// 	huma.Register(*sc.Api, op, handler)
	// }
	// {
	// 	op, handler := sc.GetAll()
	// 	huma.Register(*sc.Api, op, handler)
	// }
}

func (sc *ScanTaskController) Submit() (huma.Operation, func(ctx context.Context, scantask *model.PharosScanTask) (*ScanTaskSubmitOutput, error)) {
	return huma.Operation{
			OperationID: "SubmitScanTask",
			Method:      "POST",
			Path:        sc.Path + "/submit",
			Summary:     "Submit a new scan task",
			Description: "Submits a new scan task for processing. The task includes details about the Docker image to be scanned.",
			Tags:        []string{"DockerImage"},

			Responses: map[string]*huma.Response{
				"200": {
					Description: "The submitted scantask",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, scantask *model.PharosScanTask) (*ScanTaskSubmitOutput, error) {
			now := time.Now().UTC()
			scantask.Created = now
			scantask.Status = "new"
			return &ScanTaskSubmitOutput{
				Body: ScanTaskSubmitOutputBody{
					ScanTask: *scantask,
					Message:  "Scan task submitted successfully",
				},
			}, nil
			// task := PharosScanTask{
			// 	JobId: jobId,
			// 	Auth:  auth,
			// 	ImageSpec: PharosImageSpec{
			// 		Image:       imageRef,
			// 		Platform:    platform,
			// 		CacheExpiry: cacheExpiry,
			// 	},
			// 	Timeout: scanTimeout,
			// 	Created: now,
			// 	Updated: now,
			// 	Status:  "new",
			// }
		}
}
