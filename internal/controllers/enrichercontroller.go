package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EnricherController struct {
	Path    string
	Api     *huma.API
	Config  *model.Config
	Logger  *zerolog.Logger
	Version string
}

type Enrichers struct {
	Body []model.Enricher `json:"body"`
}

type Enricher struct {
	Body model.Enricher `json:"body"`
}

type EnricherIdInput struct {
	EnricherId uint `path:"enricherid" doc:"ID of the Enricher to retrieve"`
}

type UpsertEnricherInput struct {
	Body model.Enricher `json:"body"`
}

type EnricherSearchInput struct {
	Pagination
	Id     string             `query:"id" doc:"ID of the Enricher to retrieve, can be a glob pattern, exclusive with search"`
	Type   model.EnricherType `query:"type" doc:"Type of the Enricher to retrieve"`
	Name   string             `query:"name" doc:"Name of the Enricher to retrieve, can be a glob pattern, exclusive with search"`
	Search string             `query:"search" doc:"Any field of the Enricher to retrieve, can be a glob pattern, exclusive with id and name"`
	Detail bool               `query:"detail" default:"false" doc:"If true, returns detailed information about the enricher"`
}

type EnricherDeleteResponse struct {
	Message string `json:"message"`
}

func NewEnricherController(api *huma.API, config *model.Config) *EnricherController {
	ec := &EnricherController{
		Path:    "/enricher",
		Api:     api,
		Config:  config,
		Logger:  logging.NewLogger("info", "component", "EnricherController"),
		Version: "v1",
	}
	return ec
}

func (ec *EnricherController) V1AddRoutes() {
	{
		op, handler := ec.V1Get()
		huma.Register(*ec.Api, op, handler)
	}
	{
		op, handler := ec.V1GetBySearch()
		huma.Register(*ec.Api, op, handler)
	}
	{
		op, handler := ec.V1UpsertEnricher()
		huma.Register(*ec.Api, op, handler)
	}
	{
		op, handler := ec.V1DeleteEnricher()
		huma.Register(*ec.Api, op, handler)
	}
}

func (ec *EnricherController) V1Get() (huma.Operation, func(ctx context.Context, input *EnricherIdInput) (*Enricher, error)) {
	return huma.Operation{
			OperationID: "V1GetEnricher",
			Method:      "GET",
			Path:        ec.Path + "/{enricherid}",
			Summary:     "Get one enricher by ID",
			Description: "Retrieves an Enricher by its ID.",
			Tags:        []string{"V1/Enricher"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A single enricher",
				},
				"500": {
					Description: "Internal server error",
				},
				"404": {
					Description: "Enricher not found",
				},
			},
		}, func(ctx context.Context, input *EnricherIdInput) (*Enricher, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var value model.Enricher
			var query = model.Enricher{
				ID: input.EnricherId,
			}
			if err := databaseContext.DB.First(&value, &query).Error; err != nil {
				ec.Logger.Error().Err(err).Uint("enricherId", input.EnricherId).Msg("Failed to retrieve enricher")
				if err == gorm.ErrRecordNotFound {
					return nil, huma.Error404NotFound("Enricher with ID " + fmt.Sprint(input.EnricherId) + " not found")
				} else {
					return nil, huma.Error500InternalServerError("Failed to retrieve enricher: " + err.Error())
				}
			}
			return &Enricher{
				Body: value,
			}, nil
		}
}

func (ec *EnricherController) V1GetBySearch() (huma.Operation, func(ctx context.Context, input *EnricherSearchInput) (*Enrichers, error)) {
	return huma.Operation{
			OperationID: "V1SearchEnrichers",
			Method:      "GET",
			Path:        ec.Path,
			Summary:     "Search for enrichers",
			Description: "Retrieves all enrichers stored in the database.",
			Tags:        []string{"V1/Enricher"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A list of enrichers",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *EnricherSearchInput) (*Enrichers, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var db *gorm.DB
			db = databaseContext.DB
			if input.Detail {
				db = databaseContext.DB
			} else {
				db = databaseContext.DB.Omit("Code")
			}
			specificSearch := false
			if input.Id != "" {
				db = db.Where("id LIKE ?", strings.ReplaceAll(input.Id, "*", "%"))
				specificSearch = true
			}
			if input.Name != "" {
				db = db.Where("name LIKE ?", strings.ReplaceAll(input.Name, "*", "%"))
				specificSearch = true
			}
			if input.Type != "" {
				db = db.Where("type = ?", input.Type)
				specificSearch = true
			}
			if input.Search != "" {
				if specificSearch {
					return nil, huma.Error400BadRequest("Cannot use search with id or name")
				}
				search := strings.ReplaceAll(input.Search, "*", "%")
				db = db.Where("id LIKE ? OR name LIKE ?", search, search)
			}
			db = db.Order("name ASC")

			var values []model.Enricher
			result := db.Scopes(Paginate(&input.Pagination)).Find(&values)
			if result.Error != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve enrichers: " + result.Error.Error())
			}

			return &Enrichers{
				Body: values,
			}, nil
		}
}

func (ec *EnricherController) V1UpsertEnricher() (huma.Operation, func(ctx context.Context, input *UpsertEnricherInput) (*Enricher, error)) {
	op := huma.Operation{
		OperationID: "V1UpsertEnricher",
		Method:      "POST",
		Path:        ec.Path,
		Summary:     "Create or update an enricher",
		Description: "Creates a new enricher or updates an existing one by ID.",
		Tags:        []string{"V1/Enricher"},
		Responses: map[string]*huma.Response{
			"200": {
				Description: "Enricher created or updated",
			},
			"500": {
				Description: "Internal server error",
			},
		},
	}

	handler := func(ctx context.Context, input *UpsertEnricherInput) (*Enricher, error) {
		databaseContext, err := getDatabaseContext(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("Database context not found in request context")
		}
		var enricher = input.Body
		if enricher.ID == 0 {
			// Create new enricher
			if err := databaseContext.DB.Create(&enricher).Error; err != nil {
				ec.Logger.Error().Err(err).Msg("Failed to create enricher")
				return nil, huma.Error500InternalServerError("Failed to create enricher: " + err.Error())
			}
		} else {
			// Update existing enricher
			if err := databaseContext.DB.Save(&enricher).Error; err != nil {
				ec.Logger.Error().Err(err).Uint("enricherId", enricher.ID).Msg("Failed to update enricher")
				return nil, huma.Error500InternalServerError("Failed to update enricher: " + err.Error())
			}
		}
		return &Enricher{Body: enricher}, nil
	}

	return op, handler
}

func (ec *EnricherController) V1DeleteEnricher() (huma.Operation, func(ctx context.Context, input *EnricherIdInput) (*EnricherDeleteResponse, error)) {
	op := huma.Operation{
		OperationID: "V1DeleteEnricher",
		Method:      "DELETE",
		Path:        ec.Path + "/{enricherid}",
		Summary:     "Delete an enricher by ID",
		Description: "Deletes an enricher from the database by its ID.",
		Tags:        []string{"V1/Enricher"},
		Responses: map[string]*huma.Response{
			"204": {
				Description: "Enricher deleted successfully",
			},
			"404": {
				Description: "Enricher not found",
			},
			"500": {
				Description: "Internal server error",
			},
		},
	}

	handler := func(ctx context.Context, input *EnricherIdInput) (*EnricherDeleteResponse, error) {
		databaseContext, err := getDatabaseContext(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("Database context not found in request context")
		}
		result := databaseContext.DB.Delete(&model.Enricher{}, input.EnricherId)
		if result.Error != nil {
			ec.Logger.Error().Err(result.Error).Uint("enricherId", input.EnricherId).Msg("Failed to delete enricher")
			return nil, huma.Error500InternalServerError("Failed to delete enricher: " + result.Error.Error())
		}
		if result.RowsAffected == 0 {
			return nil, huma.Error404NotFound(fmt.Sprintf("Enricher with ID %d not found", input.EnricherId))
		}
		return &EnricherDeleteResponse{Message: fmt.Sprintf("Enricher with ID %d deleted", input.EnricherId)}, nil
	}

	return op, handler
}
