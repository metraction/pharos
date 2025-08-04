package controllers

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/metraction/pharos/pkg/model"
	"gorm.io/gorm"
)

type Pagination struct {
	PageSize int `query:"page_size" default:"10" doc:"Maximum number of items"`
	Page     int `query:"page" default:"1" doc:"Page number"`
}

func Paginate(p *Pagination) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		switch {
		case p.PageSize > 100:
			p.PageSize = 100
		case p.PageSize <= 0:
			p.PageSize = 10
		}

		offset := (p.Page - 1) * p.PageSize
		return db.Offset(offset).Limit(p.PageSize)
	}
}

type CommonResponse struct {
	Body string `json:"body"`
}

func getDatabaseContext(ctx context.Context) (*model.DatabaseContext, error) {
	databaseContext, ok := ctx.Value("databaseContext").(*model.DatabaseContext)
	if !ok || databaseContext == nil {
		return nil, huma.Error500InternalServerError("Database context not found in request context")
	}
	// var values []model.DockerImage
	// if err := databaseContext.DB.Find(&values).Error; err != nil {
	// 	return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
	// }
	return databaseContext, nil
}
