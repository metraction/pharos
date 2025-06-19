package controllers

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/metraction/pharos/pkg/model"
)

type CommonResponse struct {
	Body string `json:"body"`
}

func getDatabaseContext(ctx context.Context) (*model.DatabaseContext, error) {
	databaseContext, ok := ctx.Value("databaseContext").(*model.DatabaseContext)
	if !ok || databaseContext == nil {
		return nil, huma.Error500InternalServerError("Database context not found in request context")
	}
	var values []model.DockerImage
	if err := databaseContext.DB.Find(&values).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
	}
	return databaseContext, nil
}
