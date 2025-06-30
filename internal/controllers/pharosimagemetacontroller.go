// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type PharosImageMetaController struct {
	Path   string
	Api    *huma.API
	Config *model.Config
	Logger *zerolog.Logger
}

type PharosImageMetas struct {
	Body []model.PharosImageMeta `json:"body"`
}

type PharosImageMeta struct {
	Body model.PharosImageMeta `json:"body"`
}

type ImageDigestInput struct {
	ImageId string `path:"imageid" doc:"Imageid of the Docker image to retrieve"`
}

func NewimageController(api *huma.API, config *model.Config) *PharosImageMetaController {
	pc := &PharosImageMetaController{
		Path:   "/pharosimagemeta",
		Api:    api,
		Config: config,
		Logger: logging.NewLogger("info", "component", "PharosImageMetaController"),
	}
	return pc
}

func (pc *PharosImageMetaController) AddRoutes() {
	{
		op, handler := pc.Get()
		huma.Register(*pc.Api, op, handler)
	}
	{
		op, handler := pc.GetAll()
		huma.Register(*pc.Api, op, handler)
	}
	// {
	// 	op, handler := pc.Find()
	// 	huma.Register(*pc.Api, op, handler)
	// }
}

// For later use, if we want to implement a search functionality.
// func (pc *PharosImageMetaController) Find() (huma.Operation, func(ctx context.Context, input *PharosImageMeta) (*PharosImageMetas, error)) {
// 	return huma.Operation{
// 			OperationID: "FindImage",
// 			Method:      "POST",
// 			Path:        pc.Path + "/find",
// 			Summary:     "Finds images based on search criteria",
// 			Description: "Retrieves a list of Docker images based on the provided search criteria.",
// 			Tags:        []string{"PharosImageMeta"},

// 			Responses: map[string]*huma.Response{
// 				"200": {
// 					Description: "A list of images matching the search criteria",
// 				},
// 				"500": {
// 					Description: "Internal server error",
// 				},
// 				"404": {
// 					Description: "Nothing found matching the search criteria",
// 				},
// 			},
// 		}, func(ctx context.Context, input *PharosImageMeta) (*PharosImageMetas, error) {
// 			databaseContext, err := getDatabaseContext(ctx)
// 			if err != nil {
// 				return nil, huma.Error500InternalServerError("Database context not found in request context")
// 			}
// 			var values []model.PharosImageMeta
// 			if err := databaseContext.DB.
// 				Preload("ContextRoots.Contexts").
// 				Find(&values, &input.Body).Error; err != nil {
// 				pc.Logger.Error().Err(err).Msg("Failed to retrieve Docker images")
// 			}
// 			PharosImageMetas := &PharosImageMetas{
// 				Body: values,
// 			}
// 			return PharosImageMetas, nil
// 		}
// }

func (pc *PharosImageMetaController) Get() (huma.Operation, func(ctx context.Context, input *ImageDigestInput) (*PharosImageMeta, error)) {
	return huma.Operation{
			OperationID: "Getimage",
			Method:      "GET",
			Path:        pc.Path + "/{imageid}",
			Summary:     "Get one image by ImageId",
			Description: "Retrieves a Docker image by its ImageId. Returns related objects such as vulnerabilities, packages and findings.",
			Tags:        []string{"PharosImageMeta"},

			Responses: map[string]*huma.Response{
				"200": {
					Description: "A single image",
				},
				"500": {
					Description: "Internal server error",
				},
				"404": {
					Description: "image not found",
				},
			},
		}, func(ctx context.Context, input *ImageDigestInput) (*PharosImageMeta, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var value model.PharosImageMeta
			var query = model.PharosImageMeta{
				ImageId: input.ImageId,
			}
			if err := databaseContext.DB.
				Preload("ContextRoots.Contexts").
				Preload("Vulnerabilities").
				Preload("Findings").
				Preload("Packages").
				First(&value, &query).Error; err != nil {
				pc.Logger.Error().Err(err).Str("imageId", input.ImageId).Msg("Failed to retrieve Docker image")
				if err == gorm.ErrRecordNotFound {
					return nil, huma.Error404NotFound("Image with ImageId " + input.ImageId + " not found")
				} else {
					return nil, huma.Error500InternalServerError("Failed to retrieve Docker image: " + err.Error())
				}
			}
			if value.IndexDigest == "" {
				return nil, huma.Error404NotFound("Image with ImageId " + input.ImageId + " not found")
			}
			return &PharosImageMeta{
				Body: value,
			}, nil
		}
}

func (pc *PharosImageMetaController) GetAll() (huma.Operation, func(ctx context.Context, input *struct{}) (*PharosImageMetas, error)) {
	return huma.Operation{
			OperationID: "GetAllImages",
			Method:      "GET",
			Path:        pc.Path,
			Summary:     "Get all images",
			Description: "Retrieves all  images stored in the database.",
			Tags:        []string{"PharosImageMeta"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A list of images",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *struct{}) (*PharosImageMetas, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var values []model.PharosImageMeta
			if err := databaseContext.DB.Find(&values).Error; err != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
			}
			return &PharosImageMetas{
				Body: values,
			}, nil
		}
}
