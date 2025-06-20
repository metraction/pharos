// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/danielgtaylor/huma/v2"
	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/pkg/model"
)

type ImageController struct {
	Path      string
	Api       *huma.API
	Publisher *integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult]
	Config    *model.Config
}

type Images struct {
	Body []model.PharosImageMeta `json:"body"`
}

type Image struct {
	Body model.PharosImageMeta `json:"body"`
}

type PharosScanResult struct {
	Body model.PharosScanResult `json:"body"`
}

type ImageDigestInput struct {
	ImageId string `path:"imageid" doc:"Imageid of the Docker image to retrieve"`
}

func NewimageController(api *huma.API, config *model.Config) *ImageController {
	ic := &ImageController{
		Path:   "/image",
		Api:    api,
		Config: config,
	}
	return ic
}

func (ic *ImageController) AddRoutes() {
	{
		op, handler := ic.Get()
		huma.Register(*ic.Api, op, handler)
	}
	{
		op, handler := ic.GetAll()
		huma.Register(*ic.Api, op, handler)
	}
	{
		op, handler := ic.SyncScan()
		huma.Register(*ic.Api, op, handler)
	}
}

func (ic *ImageController) WithPublisher(publisher *integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult]) *ImageController {
	ic.Publisher = publisher
	return ic
}

func (ic *ImageController) Get() (huma.Operation, func(ctx context.Context, input *ImageDigestInput) (*Image, error)) {
	return huma.Operation{
			OperationID: "Getimage",
			Method:      "GET",
			Path:        ic.Path + "/{digest}",
			Summary:     "Get one Docker image by digest",
			Description: "Retrieves a Docker image by its digest (SHA).",
			Tags:        []string{"image"},

			Responses: map[string]*huma.Response{
				"200": {
					Description: "A single Docker image",
				},
				"500": {
					Description: "Internal server error",
				},
				"404": {
					Description: "Docker image not found",
				},
			},
		}, func(ctx context.Context, input *ImageDigestInput) (*Image, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var value model.PharosImageMeta
			var query = model.PharosImageMeta{
				ImageId: input.ImageId,
			}
			if err := databaseContext.DB.Find(&value, &query).Error; err != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker image: " + err.Error())
			}
			if value.IndexDigest == "" {
				return nil, huma.Error404NotFound("Docker image with digest " + input.ImageId + " not found")
			}
			return &Image{
				Body: value,
			}, nil
		}
}

func (ic *ImageController) GetAll() (huma.Operation, func(ctx context.Context, input *struct{}) (*Images, error)) {
	return huma.Operation{
			OperationID: "GetAllImages",
			Method:      "GET",
			Path:        ic.Path,
			Summary:     "Get all Docker images",
			Description: "Retrieves all Docker images stored in the database.",
			Tags:        []string{"image"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A list of Docker images",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *struct{}) (*Images, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var values []model.PharosImageMeta
			if err := databaseContext.DB.Find(&values).Error; err != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
			}
			return &Images{
				Body: values,
			}, nil
		}
}

// SyncScan handles the creation or update of a Docker image and initiates a scan.

func (ic *ImageController) SyncScan() (huma.Operation, func(ctx context.Context, input *Image) (*PharosScanResult, error)) {
	return huma.Operation{
			OperationID: "SyncScan",
			Method:      "POST",
			Path:        ic.Path + "/syncscan",
			Summary:     "Update / Create Image and Scan",
			Description: `
				Creates a new Docker image or updates an existing one based on the provided digest. 
				If the image already exists, it will be updated; otherwise, a new image will be created.
				A scan task is created and sent to the scanner for processing.`,
			Tags: []string{"image"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A single Docker image",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *Image) (*PharosScanResult, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			// Set default values for ArchName and ArchOS if not provided
			if input.Body.ArchName == "" {
				input.Body.ArchName = "amd64"
			}
			if input.Body.ArchOS == "" {
				input.Body.ArchOS = "linux"
			}
			if input.Body.ImageId == "" {
				input.Body.ImageId = uuid.New().String() // Temporary ImageId until we check a scan result
			}
			var value model.PharosImageMeta
			var query = model.PharosImageMeta{
				IndexDigest: input.Body.IndexDigest,
				ArchName:    input.Body.ArchName,
				ArchOS:      input.Body.ArchOS,
			}
			if err := databaseContext.DB.Find(&value, &query).Error; err != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
			}
			if value.IndexDigest == "" {
				log.Printf("Creating new Docker image with digest %s", input.Body.IndexDigest)
				databaseContext.DB.Create(&input.Body)
			} else {
				log.Printf("Updating existing Docker image with digest %s", input.Body.IndexDigest)
				input.Body.ImageId = value.ImageId // Use existing ImageId
				databaseContext.DB.Save(&input.Body)
			}
			timeout, err := time.ParseDuration(ic.Config.Publisher.Timeout)
			if err != nil {
				timeout = 30 * time.Second
			}

			// Create a full scan task from the Docker image information
			pharosScanTask, err := model.NewPharosScanTask(
				uuid.New().String(),  // jobId
				input.Body.ImageSpec, // imageRef
				fmt.Sprintf("%s/%s", input.Body.ArchOS, input.Body.ArchName), // platform
				model.PharosRepoAuth{}, // auth
				24*time.Hour,           // cacheExpiry
				timeout,                // scanTimeout
			)

			if err != nil {
				log.Printf("Error creating scan task: %v\n", err)
				return nil, huma.Error500InternalServerError("Error creating scan task: " + err.Error())
			}
			fmt.Println("Sending image scan request:", pharosScanTask, " to ", ic.Config.Publisher.RequestQueue)
			err, corrId := ic.Publisher.SendRequest(ctx, pharosScanTask)
			log.Printf("Sent scan task %v\n", corrId)
			if err != nil {
				log.Printf("Failed to get result for %s: %v\n", pharosScanTask.ImageSpec.Image, err)
				return nil, huma.Error500InternalServerError("Failed to get result: " + err.Error())
			}

			pharosScanResult, err := ic.Publisher.ReceiveResponse(ctx, corrId, timeout)
			if err != nil {
				log.Printf("Failed to receive scan result for %s: %v\n", corrId, err)
				return nil, huma.Error500InternalServerError("Failed to receive scan result: " + err.Error())
			}
			// Now we can update the ImageId with the scan result
			tx := databaseContext.DB.Model(&model.PharosImageMeta{}).Where("image_id = ?", input.Body.ImageId).Update("image_id", pharosScanResult.Image.ImageId)
			if tx.Error != nil {
				log.Printf("Failed to update image ID in database: %v\n", tx.Error)
				return nil, huma.Error500InternalServerError("Failed to update image ID in database" + tx.Error.Error())
			}
			databaseContext.DB.Save(pharosScanResult.Image) // Save the updated image metadata

			return &PharosScanResult{
				Body: pharosScanResult,
			}, nil
		}
}
