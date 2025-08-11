package controllers

import (
	"context"
	"net/http"
	"regexp"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
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

type CommonController struct {
	Logger *zerolog.Logger
}

func NewCommonController() *CommonController {

	return &CommonController{
		Logger: logging.NewLogger("info", "component", "CommonController"),
	}
}

func (cc *CommonController) RedirectToV1(next http.Handler) http.Handler {

	fn := func(w http.ResponseWriter, r *http.Request) {
		var path string
		rctx := chi.RouteContext(r.Context())
		if rctx != nil && rctx.RoutePath != "" {
			path = rctx.RoutePath
		} else {
			path = r.URL.Path
		}
		if path == "/" || path == "" {
			http.Redirect(w, r, "/api/docs", 301)
			return
		}
		if !(regexp.MustCompile(`^/api/v[0-9]+`).MatchString(path)) {
			newPath := regexp.MustCompile(`^/api`).ReplaceAllString(path, "/api/v1")
			if newPath != path {
				cc.Logger.Info().Str("path", path).Str("new_path", newPath).Msg("Redirecting to v1")
				http.Redirect(w, r, newPath, http.StatusMovedPermanently)
				return
			}
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
