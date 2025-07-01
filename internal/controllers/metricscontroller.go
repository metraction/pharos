// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"gorm.io/gorm/clause"
)

type MetricsController struct {
	Path              string
	Vulnerabilities   prometheus.GaugeVec
	Api               *huma.API
	AsyncPublisher    *integrations.RedisGtrsClient[model.PharosScanTask2, model.PharosScanResult]
	PriorityPublisher *integrations.RedisGtrsClient[model.PharosScanTask2, model.PharosScanResult]
	Config            *model.Config
	Logger            *zerolog.Logger
}

func NewMetricsController(api *huma.API, config *model.Config) *MetricsController {
	var vulnerabilities = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pharos_vulnerabilities",
		Help: "Vulnerabilites for images",
	}, []string{"image", "digest", "imageid", "platform", "severity"})

	prometheus.MustRegister(vulnerabilities)
	mc := &MetricsController{
		Path:            "/metrics",
		Api:             api,
		Config:          config,
		Logger:          logging.NewLogger("info", "component", "MetricsController"),
		Vulnerabilities: *vulnerabilities,
	}
	return mc
}

func (mc *MetricsController) AddRoutes() {
	{
		op, handler := mc.Metrics()
		huma.Register(*mc.Api, op, handler)
	}
}

// Gets metrics for pharos controller, vulnerabilities, and images. We are registering an operation with a handler, and use writer and request from a middleware.
// A bit of a hack, but now we have a common way to document things.

func (mc *MetricsController) Metrics() (huma.Operation, func(ctx context.Context, input *struct{}) (*struct{ Body string }, error)) {
	h := promhttp.Handler()
	return huma.Operation{
			OperationID: "GetMetrics",
			Method:      "GET",
			Path:        mc.Path,
			Summary:     "Gets metrics for pharos controller, vulnerabilities, and images",
			Description: "Gets metrics for pharos controller, vulnerabilities, and images",
			Tags:        []string{"Metrics"},
			Responses: map[string]*huma.Response{
				"200": {
					Content: map[string]*huma.MediaType{
						"text/plain": {},
					},
					Description: "Metrics",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *struct{}) (*struct{ Body string }, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			mc.Vulnerabilities.Reset()
			mc.Logger.Info().Msg("Serving metrics")
			writer := ctx.Value("writer").(http.ResponseWriter)
			request := ctx.Value("request").(*http.Request)
			var images []model.PharosImageMeta
			if err := databaseContext.DB.Find(&images).Error; err != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
			}
			for _, image := range images {
				var fullImage model.PharosImageMeta
				if err := databaseContext.DB.Preload(clause.Associations).First(&fullImage, &image).Error; err != nil {
					mc.Logger.Warn().Err(err).Str("imageId", image.ImageId).Msg("Failed to retrieve Docker image")
				} else {
					summary := fullImage.GetSummary()
					mc.Logger.Info().Str("imageId", image.ImageId).Any("summary", summary).Msg("Found image in database")
					for level, count := range summary.Severities {
						mc.Vulnerabilities.WithLabelValues(fullImage.ImageSpec, fullImage.IndexDigest, fullImage.ImageId, fullImage.ArchOS+"/"+fullImage.ArchName, level).Set(float64(count))
					}
				}
			}
			h.ServeHTTP(writer, request)
			return nil, nil
		}
}

// We have to ingject the request and writer into the context, so we can use them in the Metrics handler.

func (mc *MetricsController) MetricsMiddleware() func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humago.Unwrap(ctx)
		ctx = huma.WithValue(ctx, "request", r)
		ctx = huma.WithValue(ctx, "writer", w)
		next(ctx)
	}
}
