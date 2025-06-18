// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/metraction/pharos/pkg/model"
)

type DockerImageController struct {
	Path string
	Api  *huma.API
}

type DockerImageMultipleOutput struct {
	Body struct {
		DockerImages []model.DockerImage `json:"dockerimages"`
	} `json:"body"`
}

type DockerImageSingleOutput struct {
	Body struct {
		DockerImage model.DockerImage `json:"dockerimage"`
	} `json:"body"`
}

type DockerImageDigestInput struct {
	Digest string `path:"digest" maxLength:"72" example:"sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03" doc:"Digest of the Docker image to retrieve"`
}

func NewDockerImageController(group *huma.API) *DockerImageController {
	dc := &DockerImageController{
		Path: "/dockerimage",
		Api:  group,
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
}

func (dc *DockerImageController) Get() (huma.Operation, func(ctx context.Context, input *DockerImageDigestInput) (*DockerImageSingleOutput, error)) {
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
		}, func(ctx context.Context, input *DockerImageDigestInput) (*DockerImageSingleOutput, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var value model.DockerImage
			var query = model.DockerImage{
				Digest: &input.Digest,
			}
			if err := databaseContext.DB.Find(&value, &query).Error; err != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker image: " + err.Error())
			}
			if value.Digest == nil {
				return nil, huma.Error404NotFound("Docker image with digest " + input.Digest + " not found")
			}
			return &DockerImageSingleOutput{
				Body: struct {
					DockerImage model.DockerImage `json:"dockerimage"`
				}{
					value,
				},
			}, nil
		}
}

func (dc *DockerImageController) GetAll() (huma.Operation, func(ctx context.Context, input *struct{}) (*DockerImageMultipleOutput, error)) {
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
		}, func(ctx context.Context, input *struct{}) (*DockerImageMultipleOutput, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var values []model.DockerImage
			if err := databaseContext.DB.Find(&values).Error; err != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
			}
			return &DockerImageMultipleOutput{
				Body: struct {
					DockerImages []model.DockerImage `json:"dockerimages"`
				}{
					values,
				},
			}, nil
		}
}

func (dc *DockerImageController) CreateOrUpdate() (huma.Operation, func(ctx context.Context, input *model.DockerImage) (*DockerImageSingleOutput, error)) {
	return huma.Operation{
			OperationID: "GetDockerImage",
			Method:      "POST",
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
		}, func(ctx context.Context, input *model.DockerImage) (*DockerImageSingleOutput, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var value model.DockerImage
			var query = model.DockerImage{
				Digest: input.Digest,
			}
			if err := databaseContext.DB.Find(&value, &query).Error; err != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
			}
			if value.Digest == nil {
				// create new entry
			} else {
				// existing entry
			}

			return &DockerImageSingleOutput{
				Body: struct {
					DockerImage model.DockerImage `json:"dockerimage"`
				}{
					value,
				},
			}, nil
		}
}
