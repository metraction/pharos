// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type PharosScanTaskController struct {
	Path              string
	Api               *huma.API
	AsyncPublisher    *integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult]
	PriorityPublisher *integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult]
	Config            *model.Config
	Logger            *zerolog.Logger
}

type PharosScanTask struct {
	Body model.PharosScanTask `json:"body"`
}

func NewPharosScanTaskController(api *huma.API, config *model.Config) *PharosScanTaskController {
	pc := &PharosScanTaskController{
		Path:   "/pharosscantask",
		Api:    api,
		Config: config,
		Logger: logging.NewLogger("info"),
	}
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
	publisher *integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult], priorityPublisher *integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult]) *PharosScanTaskController {
	pc.AsyncPublisher = publisher
	pc.PriorityPublisher = priorityPublisher
	return pc
}

func (pc *PharosScanTaskController) sendScanRequest(ctx context.Context, publisher *integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult], pharosScanTask *model.PharosScanTask) (string, *model.PharosScanTask, error) {
	// Set default values for ArchName and ArchOS if not provided
	now := time.Now().UTC()
	if pharosScanTask.ImageSpec.Platform == "" {
		pharosScanTask.ImageSpec.Platform = "linux/amd64"
	}
	if pharosScanTask.JobId == "" {
		pharosScanTask.JobId = uuid.New().String() // Generate a new JobId if not provided
	}
	pharosScanTask.Created = now
	pharosScanTask.Updated = now
	timeout, err := time.ParseDuration(pc.Config.Publisher.Timeout)
	if err != nil {
		timeout = 30 * time.Second
	}
	pharosScanTask.Timeout = timeout
	if pharosScanTask.ImageSpec.CacheExpiry == 0 {
		pharosScanTask.ImageSpec.CacheExpiry = 24 * time.Hour // Default cache expiry
	}
	if pc.PriorityPublisher == nil {
		log.Error().Msg("PriorityPublisher is not set, cannot send scan request")
		return "", nil, huma.Error500InternalServerError("PriorityPublisher is not set, cannot send scan request")
	}
	log.Info().Str("image", pharosScanTask.ImageSpec.Image).Msg("Sending image scan request")
	err, corrId := publisher.SendRequest(ctx, *pharosScanTask)
	log.Info().Str("corrId", corrId).Msg("Sent scan task to scanner")
	if err != nil {
		log.Error().Err(err).Msg("Failed to send request to scanner")
		return "", nil, huma.Error500InternalServerError("Failed to send request to scanner: " + err.Error())
	}
	return corrId, pharosScanTask, nil
}

// SyncScan handles the creation or update of a Docker image and initiates a scan.

func (pc *PharosScanTaskController) AsyncScan() (huma.Operation, func(ctx context.Context, input *PharosScanTask) (*PharosScanTask, error)) {
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
		}, func(ctx context.Context, input *PharosScanTask) (*PharosScanTask, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var value model.PharosImageMeta
			// Split the platform string into OS and Arch
			platform := input.Body.ImageSpec.Platform
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
				ImageSpec: input.Body.ImageSpec.Image,
				ArchOS:    archOS,
				ArchName:  archName,
			}
			if err := databaseContext.DB.Find(&value, &query).Error; err != nil {
				log.Error().Err(err).Msg("Failed to retrieve Docker images")
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
			}
			if value.ImageId != "" {
				log.Info().Str("imageId", value.ImageId).Msg("Image already exists in database, using existing image metadata")

				return nil, huma.Error409Conflict("Image with ImageSpec " + input.Body.ImageSpec.Image + " already exists in database")
			}
			_, pharosScanTask, err := pc.sendScanRequest(ctx, pc.AsyncPublisher, &input.Body)
			if err != nil {
				log.Error().Err(err).Msg("Failed to send scan request")
				return nil, err
			}
			// TODO: This must go into some sort of queue or async processing
			// This is where we receiver the scan result.
			/*
				go func(databaseContext *model.DatabaseContext, corrId string) {
					ctx := context.Background()
					log.Info().Str("corrId", corrId).Msg("Starting async scan for image")
					log.Info().Str("image", pharosScanTask.ImageSpec.Image).Msg("Waiting for async scan to complete")
					timeout := time.Duration(3600 * time.Second) // Default timeout for receiving response
					pharosScanResult, err := pc.AsyncPublisher.ReceiveResponse(ctx, corrId, timeout)
					if err != nil {
						log.Error().Err(err).Str("corrId", corrId).Msg("Failed to receive scan result for async scan")
						return
					}
					if pharosScanResult.ScanTask.Error != "" {
						log.Warn().Str("corrId", corrId).Str("error", pharosScanResult.ScanTask.Error).Msg("Scan task failed during async scan")
					} else {
						pc.saveScanResult(databaseContext, &pharosScanResult)
					}

					//time.Sleep(10 * time.Second) // Simulate waiting for the scan to complete
					log.Info().Str("image", pharosScanTask.ImageSpec.Image).Msg("Async scan completed")
				}(databaseContext, corrId)
			*/
			return &PharosScanTask{
				Body: *pharosScanTask,
			}, nil
		}
}

// SyncScan handles the creation or update of a Docker image and initiates a scan.

func (pc *PharosScanTaskController) SyncScan() (huma.Operation, func(ctx context.Context, input *PharosScanTask) (*PharosScanResult, error)) {
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
		}, func(ctx context.Context, input *PharosScanTask) (*PharosScanResult, error) {
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
				log.Error().Err(err).Msg("Failed to send scan request")
				return nil, err
			}
			pharosScanResult, err := pc.PriorityPublisher.ReceiveResponse(ctx, corrId, timeout)
			if pharosScanResult.ScanTask.Error != "" {
				log.Warn().Str("corrId", corrId).Str("error", pharosScanResult.ScanTask.Error).Msg("Scan task failed")
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
