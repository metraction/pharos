// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
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
		pc.Logger.Error().Msg("PriorityPublisher is not set, cannot send scan request")
		return "", nil, huma.Error500InternalServerError("PriorityPublisher is not set, cannot send scan request")
	}
	pc.Logger.Info().Str("image", pharosScanTask.ImageSpec.Image).Str("requestqueue", pc.Config.Publisher.PriorityRequestQueue).Msg("Sending image scan request")
	err, corrId := publisher.SendRequest(ctx, *pharosScanTask)
	pc.Logger.Info().Str("corrId", corrId).Msg("Sent scan task to scanner")
	if err != nil {
		pc.Logger.Error().Err(err).Msg("Failed to send request to scanner")
		return "", nil, huma.Error500InternalServerError("Failed to send request to scanner: " + err.Error())
	}
	go func() {
		pc.Logger.Info().Str("corrId", corrId).Msg("Waiting for result from asyns scan")
		time.Sleep(10 * time.Second)
		pc.Logger.Info().Str("corrId", corrId).Msg("Received result from asyns scan")
	}()
	return corrId, pharosScanTask, nil
}

func (pc *PharosScanTaskController) saveScanResult(databaseContext *model.DatabaseContext, pharosScanResult *model.PharosScanResult) error {
	pharosScanResult.Image.Vulnerabilities = pharosScanResult.Vulnerabilities // Ensure vulnerabilities are set
	pharosScanResult.Image.Findings = pharosScanResult.Findings               // Ensure findings are set
	pharosScanResult.Image.Packages = pharosScanResult.Packages               // Ensure packages are set
	// Does the image already exist in the database?
	var value model.PharosImageMeta
	var query = model.PharosImageMeta{
		ImageId: pharosScanResult.Image.ImageId,
	}
	if err := databaseContext.DB.Find(&value, &query).Error; err != nil {
		pc.Logger.Error().Err(err).Msg("Failed to retrieve Docker images")
		return huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
	}
	if value.ImageId == "" {
		pc.Logger.Info().Str("imageId", pharosScanResult.Image.ImageId).Msg("Image ID does not exist, creating new image metadata")
		tx := databaseContext.DB.Create(pharosScanResult.Image) // Try to Create the updated image metadata
		if tx.Error != nil {
			pc.Logger.Error().Err(tx.Error).Msg("Failed to save image metadata in database")
			return huma.Error500InternalServerError("Failed to save image metadata in database: " + tx.Error.Error())
		}
		pc.Logger.Info().Str("imageId", pharosScanResult.Image.ImageId).Msg("Created image metadata in database")
	} else {
		pc.Logger.Info().Str("imageId", pharosScanResult.Image.ImageId).Msg("Updating existing image metadata")
		tx := databaseContext.DB.Save(pharosScanResult.Image) // Try to Save the updated image metadata
		if tx.Error != nil {
			pc.Logger.Error().Err(tx.Error).Msg("Failed to save image metadata in database")
			return huma.Error500InternalServerError("Failed to save image metadata in database: " + tx.Error.Error())
		}
		pc.Logger.Info().Str("imageId", pharosScanResult.Image.ImageId).Msg("Updated image metadata in database")
	}
	return nil
}

// SyncScan handles the creation or update of a Docker image and initiates a scan.

func (pc *PharosScanTaskController) AsyncScan() (huma.Operation, func(ctx context.Context, input *PharosScanTask) (*PharosScanTask, error)) {
	return huma.Operation{
			OperationID: "AsyncScan",
			Method:      "POST",
			Path:        pc.Path + "/asyncsyncscan",
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
			corrId, pharosScanTask, err := pc.sendScanRequest(ctx, pc.AsyncPublisher, &input.Body)
			if err != nil {
				pc.Logger.Error().Err(err).Msg("Failed to send scan request")
				return nil, err
			}
			// TODO: This must go into some sort of queue or async processing
			go func(databaseContext *model.DatabaseContext, corrId string) {
				ctx := context.Background()
				pc.Logger.Info().Str("corrId", corrId).Msg("Starting async scan for image")
				pc.Logger.Info().Str("image", pharosScanTask.ImageSpec.Image).Msg("Waiting for async scan to complete")
				timeout := time.Duration(3600 * time.Second) // Default timeout for receiving response
				pharosScanResult, err := pc.AsyncPublisher.ReceiveResponse(ctx, corrId, timeout)
				if err != nil {
					pc.Logger.Error().Err(err).Str("corrId", corrId).Msg("Failed to receive scan result")
					return
				}
				pc.saveScanResult(databaseContext, &pharosScanResult)
				//time.Sleep(10 * time.Second) // Simulate waiting for the scan to complete
				pc.Logger.Info().Str("image", pharosScanTask.ImageSpec.Image).Msg("Async scan completed")
			}(databaseContext, corrId)
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
				pc.Logger.Error().Err(err).Msg("Failed to send scan request")
				return nil, err
			}
			pharosScanResult, err := pc.PriorityPublisher.ReceiveResponse(ctx, corrId, timeout)
			err = pc.saveScanResult(databaseContext, &pharosScanResult)
			if err != nil {
				return nil, err
			}
			return &PharosScanResult{
				Body: pharosScanResult,
			}, nil
		}
}
