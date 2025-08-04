// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PharosImageMetaController struct {
	Path   string
	Api    *huma.API
	Config *model.Config
	Logger *zerolog.Logger
}

type ContextEntries struct {
	Body []model.ContextEntry `json:"body"`
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
		op, handler := pc.GetBySearch()
		huma.Register(*pc.Api, op, handler)
	}
	{
		op, handler := pc.GetContexts()
		huma.Register(*pc.Api, op, handler)
	}
}

func (pc *PharosImageMetaController) Get() (huma.Operation, func(ctx context.Context, input *ImageDigestInput) (*PharosImageMeta, error)) {
	return huma.Operation{
			OperationID: "GetImage",
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

func (pc *PharosImageMetaController) GetContexts() (huma.Operation, func(ctx context.Context, input *ImageDigestInput) (*ContextEntries, error)) {
	return huma.Operation{
			OperationID: "GetContexts",
			Method:      "GET",
			Path:        pc.Path + "/contexts/{imageid}",
			Summary:     "Get Contexts for Image",
			Description: "Returns a flattened list of contexts for the image, to be used by Grafana.",
			Tags:        []string{"PharosImageMeta"},

			Responses: map[string]*huma.Response{
				"200": {
					Description: "A list of contexts for the image",
				},
				"500": {
					Description: "Internal server error",
				},
				"404": {
					Description: "image not found",
				},
			},
		}, func(ctx context.Context, input *ImageDigestInput) (*ContextEntries, error) {
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
			var contextEntries = []model.ContextEntry{}
			for _, contextRoot := range value.ContextRoots {
				for _, context := range contextRoot.Contexts {
					for key, value := range context.Data {
						contextEntries = append(contextEntries, model.ContextEntry{
							ContextRootKey: context.ContextRootKey,
							Key:            key,
							Value:          value,
							Owner:          context.Owner,
							UpdatedAt:      context.UpdatedAt,
						})
					}
				}
			}
			return &ContextEntries{
				Body: contextEntries,
			}, nil
		}
}

type PharosImageMetaSearchInput struct {
	ImageId        string `query:"image_id" doc:"ImageId of the Docker image to retrieve, can be a glob pattern, exclusive with any_digest"`
	IndexDigest    string `query:"index_digest" doc:"Index digest of the Docker image to retrieve, can be a glob pattern, exclusive with any_digest"`
	ManifestDigest string `query:"manifest_digest" doc:"Manifest digest of the Docker image to retrieve, can be a glob pattern, exclusive with any_digest"`
	ImageSpec      string `query:"image_spec" doc:"ImageSpec of the Docker image to retrieve, can be a glob pattern"`
	Digest         string `query:"digest" doc:"Any digest or image_id of the Docker image to retrieve, can be a glob pattern, exclusive with ImageId, IndexDigest and ManifestDigest"`
	Detail         bool   `query:"detail" default:"false" doc:"If true, returns detailed information about the image, including vulnerabilities, packages and findings"`
}

func (pc *PharosImageMetaController) GetBySearch() (huma.Operation, func(ctx context.Context, input *PharosImageMetaSearchInput) (*PharosImageMetas, error)) {
	return huma.Operation{
			OperationID: "SearchImages",
			Method:      "GET",
			Path:        pc.Path,
			Summary:     "Search for images",
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
		}, func(ctx context.Context, input *PharosImageMetaSearchInput) (*PharosImageMetas, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var db *gorm.DB
			if input.Detail {
				db = databaseContext.DB.
					Preload("ContextRoots.Contexts").
					Preload("Vulnerabilities").
					Preload("Findings").
					Preload("Packages")
			} else {
				db = databaseContext.DB.Omit(clause.Associations)
			}
			specificDigest := false
			if input.ImageId != "" {
				db = db.Where("image_id LIKE ?", strings.ReplaceAll(input.ImageId, "*", "%"))
				specificDigest = true
			}
			if input.IndexDigest != "" {
				db = db.Where("index_digest LIKE ?", strings.ReplaceAll(input.IndexDigest, "*", "%"))
				specificDigest = true
			}
			if input.ManifestDigest != "" {
				db = db.Where("manifest_digest LIKE ?", strings.ReplaceAll(input.ManifestDigest, "*", "%"))
				specificDigest = true
			}
			if input.Digest != "" {
				if specificDigest {
					return nil, huma.Error400BadRequest("Cannot use digest with image_id, index_digest, or manifest_digest")
				}
				digest := strings.ReplaceAll(input.Digest, "*", "%")
				db = db.Where("image_id LIKE ? OR index_digest LIKE ? OR manifest_digest LIKE ?", digest, digest, digest)
			}
			if input.ImageSpec != "" {
				db = db.Where("image_spec LIKE ?", strings.Replace(input.ImageSpec, "*", "%", -1))
			}
			var values []model.PharosImageMeta
			result := db.Find(&values)
			if result.Error != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + result.Error.Error())
			}

			return &PharosImageMetas{
				Body: values,
			}, nil
		}
}
