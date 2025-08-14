// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

type PharosScanTaskController struct {
	Path          string
	Api           *huma.API
	Config        *model.Config
	Logger        *zerolog.Logger
	TaskChannel   chan any
	ResultChannel chan any
	Version       string
}

// TODO: Rename as PharosScanTask2 is know from model, leads to confusion
type PharosScanTask2 struct {
	Body model.PharosScanTask2 `json:"body"`
}

func NewPharosScanTaskController(api *huma.API, config *model.Config, sourceChannel chan any, resultChannel chan any) *PharosScanTaskController {
	pc := &PharosScanTaskController{
		Path:          "/pharosscantask",
		Api:           api,
		Config:        config,
		Logger:        logging.NewLogger("info", "component", "PharosScanTaskController"),
		TaskChannel:   sourceChannel,
		ResultChannel: resultChannel,
		Version:       "v1",
	}

	return pc
}

type PharosScanResult struct {
	Body model.PharosScanResult `json:"body"`
}

func (pc *PharosScanTaskController) V1AddRoutes() {
	{
		op, handler := pc.V1PostSyncScan()
		huma.Register(*pc.Api, op, handler)
	}
	{
		op, handler := pc.V1PostAsyncScan()
		huma.Register(*pc.Api, op, handler)
	}
}

func (pc *PharosScanTaskController) sendScanRequest(ctx context.Context, pharosScanTask *model.PharosScanTask2) (*model.PharosScanTask2, error) {
	// Set default values for ArchName and ArchOS if not provided
	//now := time.Now().UTC()
	if pharosScanTask.Platform == "" {
		pharosScanTask.Platform = "linux/amd64"
	}
	if pharosScanTask.JobId == "" {
		pharosScanTask.JobId = uuid.New().String() // Generate a new JobId if not provided
	}
	if pharosScanTask.ScanTTL == 0 {
		pharosScanTask.ScanTTL = 12 * time.Hour
	}
	if pharosScanTask.CacheTTL == 0 {
		pharosScanTask.CacheTTL = 24 * time.Hour // Default cache expiry
	}

	//pc.Logger.Info().Str("image", pharosScanTask.ImageSpec).Msg("Sending image scan request")
	pc.Logger.Info().Str("image", pharosScanTask.ImageSpec).Int("queue size", len(pc.TaskChannel)).Msg("Sending image scan request to scanner queue")
	pc.TaskChannel <- *pharosScanTask

	pc.Logger.Info().Msg("Sent scan task to scanner")
	return pharosScanTask, nil
}

// SyncScan handles the creation or update of a Docker image and initiates a scan.

func (pc *PharosScanTaskController) V1PostAsyncScan() (huma.Operation, func(ctx context.Context, input *PharosScanTask2) (*PharosScanTask2, error)) {
	return huma.Operation{
			OperationID: "V1PostAyncScan",
			Method:      "POST",
			Path:        pc.Path + "/asyncscan",
			Summary:     "Do an async scan of an image",
			Description: `
				Submits an async scan of an image, and puts the scan task in the queue scanner queue if there is not existing result, otherwise it will update the context in the database.
				Example:
					{
					"imageSpec": "redis:latest"
					}
				`,
			Tags: []string{"V1/PharosScanTask"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A single image",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *PharosScanTask2) (*PharosScanTask2, error) {
			// Check if the image already exists in the database - Shortcut not to send the scan request if it already exists
			// For now we don't use the shortcut.
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var value model.PharosImageMeta
			// Split the platform string into OS and Arch
			platform := input.Body.Platform
			archOS := ""
			archName := ""
			if platform != "" {
				parts := strings.Split(platform, "/")
				if len(parts) == 2 {
					archOS = parts[0]
					archName = parts[1]
				}
			}
			var query = model.PharosImageMeta{
				ImageSpec: input.Body.ImageSpec,
				ArchOS:    archOS,
				ArchName:  archName,
			}
			if err := databaseContext.DB.
				Preload("Vulnerabilities").
				Preload("Findings").
				Preload("Packages").Find(&value, &query).Error; err != nil {
				pc.Logger.Error().Err(err).Msg("Failed to retrieve Docker images")
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
			}
			if value.ImageId != "" {
				age := time.Since(value.LastSuccessfulScan)
				randomValue := 0
				if value.TTL.Seconds() != 0 {
					randomValue = rand.Intn(int(value.TTL.Seconds())) - int(value.TTL.Seconds()/2)
				}
				if age > value.TTL+time.Duration(randomValue)*time.Second {
					pc.Logger.Info().Str("ImageId", value.ImageId).Str("ImageSpec", value.ImageSpec).Msg("Image exists but is too old, re-scanning")
				} else {
					// If the image is not too old, we can return the existing image metadata
					pc.Logger.Info().Str("ImageId", value.ImageId).Msg("Image already exists in database, using existing image metadata")
					// TODO: We must create a scanresult and send that to the results stream here.
					PharosScanResult := model.PharosScanResult{
						ScanTask: input.Body,
						Image:    value,
					}
					// Send the scan result to the results channel
					pc.ResultChannel <- PharosScanResult
					return nil, huma.Error409Conflict("Image with ImageSpec " + input.Body.ImageSpec + " already exists in database")
				}
			}
			pharosScanTask, err := pc.sendScanRequest(ctx, &input.Body)
			if err != nil {
				pc.Logger.Error().Err(err).Msg("Failed to send scan request")
				return nil, err
			}
			return &PharosScanTask2{
				Body: *pharosScanTask,
			}, nil
		}
}

// SyncScan handles the creation or update of a Docker image and initiates a scan.

func (pc *PharosScanTaskController) V1PostSyncScan() (huma.Operation, func(ctx context.Context, input *PharosScanTask2) (*PharosScanResult, error)) {
	return huma.Operation{
			OperationID: "V1PostSyncScan",
			Method:      "POST",
			Path:        pc.Path + "/syncscan",
			Summary:     "Do a sync scan of an image",
			Description: `
				Submits a sync scan of an image, adds the image to the database and returns the scan result.
				Example:
					{
					"imageSpec": "redis:latest"
					}
				`,
			Tags: []string{"V1/PharosScanTask"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A single image",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *PharosScanTask2) (*PharosScanResult, error) {
			// We create one responseChannel per request, and store it in the Scantask.
			receiver := make(chan model.PharosScanResult, 1)
			input.Body.SetReceiver(&receiver)
			_, err := pc.sendScanRequest(ctx, &input.Body)
			if err != nil {
				pc.Logger.Error().Err(err).Msg("Failed to send scan request")
				return nil, err
			}
			// Wait for submitted scan task to be processed
			pharosScanResult := <-receiver

			if pharosScanResult.ScanTask.Error != "" {
				pc.Logger.Warn().Str("taskId", pharosScanResult.ScanTask.JobId).Str("error", pharosScanResult.ScanTask.Error).Msg("Scan task failed")
				return nil, huma.Error500InternalServerError("Error during scan: " + pharosScanResult.ScanTask.Error)
			}
			pc.Logger.Info().Str("taskId", pharosScanResult.ScanTask.JobId).Str("ImageId", pharosScanResult.Image.ImageId).Msg("Received sync scan result for image")
			return &PharosScanResult{
				Body: pharosScanResult,
			}, nil
		}
}
