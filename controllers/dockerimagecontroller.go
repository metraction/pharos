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
	controller := &DockerImageController{
		Path: "/dockerimage",
		Api:  group,
	}
	return controller
}

// func (c *DockerImageController) bindIdQuery(ctx *gin.Context) (interface{}, error) {
// 	var query model.DockerImage
// 	if _, ok := interface{}(query.SHA).(string); ok {
// 		var idQuery IdQuery
// 		err := ctx.ShouldBind(&idQuery)
// 		if err != nil {
// 			ctx.JSON(406, gin.H{"error": err.Error()})
// 			return nil, err
// 		}
// 		return &idQuery, nil
// 	} else {
// 		err := ctx.ShouldBind(&query)
// 		if err != nil {
// 			ctx.JSON(406, gin.H{"error": err.Error()})
// 			return nil, err
// 		}
// 		return &query, nil
// 	}
// }

func (c *DockerImageController) AddRoutes() {
	{
		op, handler := c.Get()
		huma.Register(*c.Api, op, handler)
	}
	{
		op, handler := c.GetAll()
		huma.Register(*c.Api, op, handler)
	}
}

func (c *DockerImageController) Get() (huma.Operation, func(ctx context.Context, input *DockerImageDigestInput) (*DockerImageSingleOutput, error)) {
	return huma.Operation{
			OperationID: "GetDockerImage",
			Method:      "GET",
			Path:        c.Path + "/{digest}",
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

func (c *DockerImageController) GetAll() (huma.Operation, func(ctx context.Context, input *struct{}) (*DockerImageMultipleOutput, error)) {
	return huma.Operation{
			OperationID: "GetAllDockerImages",
			Method:      "GET",
			Path:        c.Path,
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

func (c *DockerImageController) CreateOrUpdate() (huma.Operation, func(ctx context.Context, input *model.DockerImage) (*DockerImageSingleOutput, error)) {
	return huma.Operation{
			OperationID: "GetDockerImage",
			Method:      "POST",
			Path:        c.Path + "/{digest}",
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

// func (c *DockerImageController) GetAll() []model.DockerImage {
// 	handler := func(ctx *gin.Context) {
// 		var values []model.DockerImage
// 		databaseContext := ctx.MustGet("databaseContext").(*model.DatabaseContext)
// 		databaseContext.DB.Find(&values)
// 		ctx.JSON(200, values)
// 	}
// 	return handler
// }

// func (c *DockerImageController) GetById() gin.HandlerFunc {
// 	handler := func(ctx *gin.Context) {
// 		var values model.DockerImage
// 		databaseContext := ctx.MustGet("databaseContext").(*model.DatabaseContext)
// 		query, err := c.bindIdQuery(ctx)
// 		if err != nil {
// 			return
// 		}
// 		result := databaseContext.DB.First(&values, query)
// 		if result.Error != nil {
// 			ctx.JSON(404, gin.H{"error": result.Error.Error()})
// 			return
// 		}
// 		ctx.JSON(200, values)
// 	}
// 	return gin.HandlerFunc(handler)
// }

// func (c *DockerImageController) Create() gin.HandlerFunc {
// 	handler := func(ctx *gin.Context) {
// 		databaseContext := ctx.MustGet("databaseContext").(*model.DatabaseContext)
// 		var newEntry model.DockerImage
// 		err := ctx.ShouldBind(&newEntry)
// 		if err != nil {
// 			ctx.JSON(406, gin.H{"error": err.Error()})
// 			return
// 		}
// 		result := databaseContext.DB.Create(&newEntry)
// 		if result.Error != nil {
// 			ctx.JSON(404, gin.H{"error": result.Error.Error()})
// 			return
// 		}
// 		ctx.JSON(200, newEntry)
// 	}
// 	return gin.HandlerFunc(handler)
// }

// func (c *DockerImageController) Update() gin.HandlerFunc {
// 	handler := func(ctx *gin.Context) {
// 		var values model.DockerImage
// 		databaseContext := ctx.MustGet("databaseContext").(*model.DatabaseContext)
// 		var updatedEntry model.DockerImage
// 		err := ctx.ShouldBind(&updatedEntry)
// 		if err != nil {
// 			ctx.JSON(406, gin.H{"error": err.Error()})
// 			return
// 		}
// 		result := databaseContext.DB.First(&values, updatedEntry.SHA)
// 		if result.Error != nil {
// 			ctx.JSON(404, gin.H{"error": result.Error.Error()})
// 			return
// 		}
// 		result = databaseContext.DB.Save(&updatedEntry)
// 		if result.Error != nil {
// 			ctx.JSON(500, gin.H{"error": result.Error.Error()})
// 			return
// 		}
// 		ctx.JSON(200, updatedEntry)
// 	}
// 	return gin.HandlerFunc(handler)
// }

// func (c *DockerImageController) Delete() gin.HandlerFunc {
// 	handler := func(ctx *gin.Context) {
// 		var values model.DockerImage
// 		databaseContext := ctx.MustGet("databaseContext").(*model.DatabaseContext)
// 		query, err := c.bindIdQuery(ctx)
// 		if err != nil {
// 			return
// 		}
// 		result := databaseContext.DB.Delete(&values, query)
// 		if result.Error != nil {
// 			ctx.JSON(500, gin.H{"error": result.Error.Error()})
// 			return
// 		}
// 		ctx.JSON(200, gin.H{"success": "deleted"})
// 	}
// 	return gin.HandlerFunc(handler)
//}
