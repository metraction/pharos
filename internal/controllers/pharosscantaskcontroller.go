// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/routing"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

type PharosScanTaskController struct {
	Path              string
	Api               *huma.API
	AsyncPublisher    *integrations.RedisGtrsClient[model.PharosScanTask2, model.PharosScanResult]
	PriorityPublisher *integrations.RedisGtrsClient[model.PharosScanTask2, model.PharosScanResult]
	Config            *model.Config
	Logger            *zerolog.Logger
	ResultChannel     chan any
}

// TODO: Rename as PharosScanTask2 is know from model, leads to confusion
type PharosScanTask2 struct {
	Body model.PharosScanTask2 `json:"body"`
}

func NewPharosScanTaskController(api *huma.API, config *model.Config) *PharosScanTaskController {
	pc := &PharosScanTaskController{
		Path:          "/pharosscantask",
		Api:           api,
		Config:        config,
		Logger:        logging.NewLogger("info", "component", "PharosScanTaskController"),
		ResultChannel: make(chan any), // Channel to handle scan
	}
	// Start the flow to handle scan results without scanner.
	go routing.NewScanResultsInternalFlow(model.NewDatabaseContext(&config.Database), pc.ResultChannel)
	return pc
}

type PharosScanResult struct {
	Body model.PharosScanResult `json:"body"`
}

func (pc *PharosScanTaskController) AddRoutes() {
	{
		op, handler := pc.SyncScan()
		huma.Register(*pc.Api, op, handler)
	}
	{
		op, handler := pc.AsyncScan()
		huma.Register(*pc.Api, op, handler)
	}
}

func (pc *PharosScanTaskController) WithPublisher(
	publisher *integrations.RedisGtrsClient[model.PharosScanTask2, model.PharosScanResult], priorityPublisher *integrations.RedisGtrsClient[model.PharosScanTask2, model.PharosScanResult]) *PharosScanTaskController {
	pc.AsyncPublisher = publisher
	pc.PriorityPublisher = priorityPublisher
	return pc
}

func (pc *PharosScanTaskController) sendScanRequest(ctx context.Context, publisher *integrations.RedisGtrsClient[model.PharosScanTask2, model.PharosScanResult], pharosScanTask *model.PharosScanTask2) (string, *model.PharosScanTask2, error) {
	// Set default values for ArchName and ArchOS if not provided
	//now := time.Now().UTC()
	if pharosScanTask.Platform == "" {
		pharosScanTask.Platform = "linux/amd64"
	}
	if pharosScanTask.JobId == "" {
		pharosScanTask.JobId = uuid.New().String() // Generate a new JobId if not provided
	}
	//pharosScanTask.Created = now
	//pharosScanTask.Updated = now
	timeout, err := time.ParseDuration(pc.Config.Publisher.Timeout)
	if err != nil {
		timeout = 30 * time.Second
	}
	pharosScanTask.ScanTTL = timeout
	if pharosScanTask.CacheTTL == 0 {
		pharosScanTask.CacheTTL = 24 * time.Hour // Default cache expiry
	}
	if pc.PriorityPublisher == nil {
		pc.Logger.Error().Msg("PriorityPublisher is not set, cannot send scan request")
		return "", nil, huma.Error500InternalServerError("PriorityPublisher is not set, cannot send scan request")
	}
	pc.Logger.Info().Str("image", pharosScanTask.ImageSpec).Msg("Sending image scan request")
	err, corrId := publisher.SendRequest(ctx, *pharosScanTask)
	pc.Logger.Info().Str("corrId", corrId).Msg("Sent scan task to scanner")
	if err != nil {
		pc.Logger.Error().Err(err).Msg("Failed to send request to scanner")
		return "", nil, huma.Error500InternalServerError("Failed to send request to scanner: " + err.Error())
	}
	return corrId, pharosScanTask, nil
}

// SyncScan handles the creation or update of a Docker image and initiates a scan.

func (pc *PharosScanTaskController) AsyncScan() (huma.Operation, func(ctx context.Context, input *PharosScanTask2) (*PharosScanTask2, error)) {
	return huma.Operation{
			OperationID: "AsyncScan",
			Method:      "POST",
			Path:        pc.Path + "/asyncscan",
			Summary:     "Do an async scan of an image",
			Description: `
				Submits an async scan of an image, and waits fo the scan to complete after returning the result.
				Example:
				  {
				    "imageSpec": {
				      "image": "redis:latest"
				    }	
				  }
				`,
			Tags: []string{"PharosScanTask"},
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
					pc.Logger.Info().Str("ImageId", value.ImageId).Msg("Image exists but is too old, re-scanning")
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
			_, pharosScanTask, err := pc.sendScanRequest(ctx, pc.AsyncPublisher, &input.Body)
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

func (pc *PharosScanTaskController) SyncScan() (huma.Operation, func(ctx context.Context, input *PharosScanTask2) (*PharosScanResult, error)) {
	return huma.Operation{
			OperationID: "SyncScan",
			Method:      "POST",
			Path:        pc.Path + "/syncscan",
			Summary:     "Do a sync scan of an image",
			Description: `
				Submits a sync scan of an image, adds the image to the database and returns the scan result.
				Example:
				  {
				    "imageSpec": {
				      "image": "redis:latest"
				    }	
				  }
				`,
			Tags: []string{"PharosScanTask"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A single image",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *PharosScanTask2) (*PharosScanResult, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			timeout, err := time.ParseDuration(pc.Config.Publisher.Timeout)
			if err != nil {
				timeout = 30 * time.Second
			}
			corrId, _, err := pc.sendScanRequest(ctx, pc.PriorityPublisher, &input.Body)
			if err != nil {
				pc.Logger.Error().Err(err).Msg("Failed to send scan request")
				return nil, err
			}
			pharosScanResult, err := pc.PriorityPublisher.ReceiveResponse(ctx, corrId, timeout)
			if pharosScanResult.ScanTask.Error != "" {
				pc.Logger.Warn().Str("corrId", corrId).Str("error", pharosScanResult.ScanTask.Error).Msg("Scan task failed")
				return nil, huma.Error500InternalServerError("Error during scan: " + pharosScanResult.ScanTask.Error)
			} else {
				if err := integrations.SaveScanResult(databaseContext, &pharosScanResult); err != nil {
					huma.Error500InternalServerError("Error saving result:", err)
				}
			}
			if err != nil {
				return nil, err
			}
			return &PharosScanResult{
				Body: pharosScanResult,
			}, nil
		}
}
