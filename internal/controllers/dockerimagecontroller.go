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

type DockerImageController struct {
	Path      string
	Api       *huma.API
	Publisher *integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult]
	Config    *model.Config
}

type DockerImages struct {
	Body []model.DockerImage `json:"body"`
}

type DockerImage struct {
	Body model.DockerImage `json:"body"`
}

type DockerImageDigestInput struct {
	Digest string `path:"digest" maxLength:"72" example:"sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03" doc:"Digest of the Docker image to retrieve"`
}

func NewDockerImageController(api *huma.API, config *model.Config) *DockerImageController {
	dc := &DockerImageController{
		Path:   "/dockerimage",
		Api:    api,
		Config: config,
	}
	return dc
}

func (dc *DockerImageController) AddRoutes() {
	{
		op, handler := dc.Get()
		huma.Register(*dc.Api, op, handler)
	}
	{
		op, handler := dc.GetAll()
		huma.Register(*dc.Api, op, handler)
	}
	{
		op, handler := dc.CreateOrUpdate()
		huma.Register(*dc.Api, op, handler)
	}
}

func (dc *DockerImageController) WithPublisher(publisher *integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult]) *DockerImageController {
	dc.Publisher = publisher
	return dc
}

func (dc *DockerImageController) Get() (huma.Operation, func(ctx context.Context, input *DockerImageDigestInput) (*DockerImage, error)) {
	return huma.Operation{
			OperationID: "GetDockerImage",
			Method:      "GET",
			Path:        dc.Path + "/{digest}",
			Summary:     "Get one Docker image by digest",
			Description: "Retrieves a Docker image by its digest (SHA).",
			Tags:        []string{"DockerImage"},

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
		}, func(ctx context.Context, input *DockerImageDigestInput) (*DockerImage, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var value model.DockerImage
			var query = model.DockerImage{
				Digest: input.Digest,
			}
			if err := databaseContext.DB.Find(&value, &query).Error; err != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker image: " + err.Error())
			}
			if value.Digest == "" {
				return nil, huma.Error404NotFound("Docker image with digest " + input.Digest + " not found")
			}
			return &DockerImage{
				Body: value,
			}, nil
		}
}

func (dc *DockerImageController) GetAll() (huma.Operation, func(ctx context.Context, input *struct{}) (*DockerImages, error)) {
	return huma.Operation{
			OperationID: "GetAllDockerImages",
			Method:      "GET",
			Path:        dc.Path,
			Summary:     "Get all Docker images",
			Description: "Retrieves all Docker images stored in the database.",
			Tags:        []string{"DockerImage"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A list of Docker images",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *struct{}) (*DockerImages, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var values []model.DockerImage
			if err := databaseContext.DB.Find(&values).Error; err != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
			}
			return &DockerImages{
				Body: values,
			}, nil
		}
}

func (dc *DockerImageController) CreateOrUpdate() (huma.Operation, func(ctx context.Context, input *DockerImage) (*CommonResponse, error)) {
	return huma.Operation{
			OperationID: "CreateOrUpdateDockerImage",
			Method:      "POST",
			Path:        dc.Path,
			Summary:     "Create a new docker image or update an existing one",
			Description: "Creates a new Docker image or updates an existing one based on the provided digest. If the image already exists, it will be updated; otherwise, a new image will be created.",
			Tags:        []string{"DockerImage"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A single Docker image",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *DockerImage) (*CommonResponse, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var value model.DockerImage
			var query = model.DockerImage{
				Digest: input.Body.Digest,
			}
			if err := databaseContext.DB.Find(&value, &query).Error; err != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
			}
			action := "Did nothing"
			if value.Digest == "" {
				log.Printf("Creating new Docker image with digest %s", input.Body.Digest)
				databaseContext.DB.Create(&input.Body)
				action = "Created new Docker image"
			} else {
				log.Printf("Updating existing Docker image with digest %s", input.Body.Digest)
				databaseContext.DB.Save(&input.Body)
				action = "Updated existing Docker image"
			}
			timeout, err := time.ParseDuration(dc.Config.Publisher.Timeout)
			if err != nil {
				timeout = 30 * time.Second
			}

			// Create a full scan task from the Docker image information
			pharosScanTask, err := model.NewPharosScanTask(
				uuid.New().String(),    // jobId
				input.Body.Name,        // imageRef
				"linux/amd64",          // platform
				model.PharosRepoAuth{}, // auth
				24*time.Hour,           // cacheExpiry
				timeout,                // scanTimeout
			)
			pharosScanTask.Updated = time.Now()
			if err != nil {
				log.Printf("Error creating scan task: %v\n", err)
				return nil, huma.Error500InternalServerError("Error creating scan task: " + err.Error())
			}
			fmt.Println("Sending image scan request:", pharosScanTask, " to ", dc.Config.Publisher.RequestQueue)
			err, corrId := dc.Publisher.SendRequest(ctx, pharosScanTask)
			log.Printf("Sent scan task %v\n", corrId)
			if err != nil {
				log.Printf("Failed to get result for %s: %v\n", pharosScanTask.ImageSpec.Image, err)
				return nil, huma.Error500InternalServerError("Failed to get result: " + err.Error())
			}
			pharosScanResult, err := dc.Publisher.ReceiveResponse(ctx, corrId, timeout)
			if err != nil {
				log.Printf("Failed to receive scan result for %s: %v\n", corrId, err)
				return nil, huma.Error500InternalServerError("Failed to receive scan result: " + err.Error())
			}
			return &CommonResponse{
				Body: fmt.Sprintf("Docker image with digest %s %s - Scanresult: %v", input.Body.Digest, action, pharosScanResult),
			}, nil
		}
}
