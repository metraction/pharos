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
	Path      string
	Api       *huma.API
	Publisher *integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult]
	Config    *model.Config
	Logger    *zerolog.Logger
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
}

func (pc *PharosScanTaskController) WithPublisher(publisher *integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult]) *PharosScanTaskController {
	pc.Publisher = publisher
	return pc
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
			now := time.Now().UTC()
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			// Set default values for ArchName and ArchOS if not provided
			if input.Body.ImageSpec.Platform == "" {
				input.Body.ImageSpec.Platform = "linux/amd64"
			}
			if input.Body.JobId == "" {
				input.Body.JobId = uuid.New().String() // Generate a new JobId if not provided
			}
			input.Body.Created = now
			input.Body.Updated = now
			timeout, err := time.ParseDuration(pc.Config.Publisher.Timeout)
			if err != nil {
				timeout = 30 * time.Second
			}
			input.Body.Timeout = timeout
			if input.Body.ImageSpec.CacheExpiry == 0 {
				input.Body.ImageSpec.CacheExpiry = 24 * time.Hour // Default cache expiry
			}
			pc.Logger.Info().Str("image", input.Body.ImageSpec.Image).Str("requestqueue", pc.Config.Scanner.RequestQueue).Msg("Sending image scan request")
			err, corrId := pc.Publisher.SendRequest(ctx, input.Body)
			pc.Logger.Info().Str("corrId", corrId).Msg("Sent scan task to scanner")
			if err != nil {
				pc.Logger.Error().Err(err).Msg("Failed to send request to scanner")
				return nil, huma.Error500InternalServerError("Failed to send request to scanner: " + err.Error())
			}

			pharosScanResult, err := pc.Publisher.ReceiveResponse(ctx, corrId, timeout)
			pc.Logger.Info().Str("corrId", corrId).Msg("Received scan result from scanner")
			if err != nil {
				pc.Logger.Error().Err(err).Str("coorId", corrId).Msg("Failed to receive scan result ")
				return nil, huma.Error500InternalServerError("Failed to receive scan result: " + err.Error())
			}
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
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
			}
			if value.ImageId == "" {
				pc.Logger.Info().Str("imageId", pharosScanResult.Image.ImageId).Msg("Image ID does not exist, creating new image metadata")
				tx := databaseContext.DB.Create(pharosScanResult.Image) // Try to Create the updated image metadata
				if tx.Error != nil {
					pc.Logger.Error().Err(tx.Error).Msg("Failed to save image metadata in database")
					return nil, huma.Error500InternalServerError("Failed to save image metadata in database: " + tx.Error.Error())
				}
				pc.Logger.Info().Str("imageId", pharosScanResult.Image.ImageId).Msg("Created image metadata in database")
			} else {
				pc.Logger.Info().Str("imageId", pharosScanResult.Image.ImageId).Msg("Updating existing image metadata")
				tx := databaseContext.DB.Save(pharosScanResult.Image) // Try to Save the updated image metadata
				if tx.Error != nil {
					pc.Logger.Error().Err(tx.Error).Msg("Failed to save image metadata in database")
					return nil, huma.Error500InternalServerError("Failed to save image metadata in database: " + tx.Error.Error())
				}
				pc.Logger.Info().Str("imageId", pharosScanResult.Image.ImageId).Msg("Updated image metadata in database")
			}
			return &PharosScanResult{
				Body: pharosScanResult,
			}, nil
		}
}
