// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/metraction/pharos/internal/integrations/redis"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

type MetricsController struct {
	Path              string
	Vulnerabilities   prometheus.GaugeVec
	Contexts          prometheus.GaugeVec
	Api               *huma.API
	AsyncPublisher    *redis.RedisGtrsClient[model.PharosScanTask2, model.PharosScanResult]
	PriorityPublisher *redis.RedisGtrsClient[model.PharosScanTask2, model.PharosScanResult]
	Config            *model.Config
	Logger            *zerolog.Logger
}

func NewMetricsController(api *huma.API, config *model.Config) *MetricsController {
	var vulnerabilities = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pharos_vulnerabilities",
		Help: "Vulnerabilites for images",
	}, []string{"image", "digest", "imageid", "platform", "severity"})

	var contexts = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pharos_test",
		Help: "Vulnerabilites for images",
	}, []string{})

	prometheus.MustRegister(vulnerabilities)
	mc := &MetricsController{
		Path:            "/metrics",
		Api:             api,
		Config:          config,
		Logger:          logging.NewLogger("info", "component", "MetricsController"),
		Vulnerabilities: *vulnerabilities,
		Contexts:        *contexts,
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
			timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
				mc.Logger.Info().Float64("duration_seconds", v).Msg("Metrics handler duration")
			}))
			defer timer.ObserveDuration()
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
			// This is a two step process:
			// 1. We first iterate over all images and collect the labels we need for and set the vulnerabilities metric.
			// 2. We then create a GaugeVec with the labels we collected and set the values for the contexts metric.
			var labels = map[string]string{
				"image":    "",
				"digest":   "",
				"imageid":  "",
				"platform": "",
			}
			// step 1: Collect labels and set vulnerabilities metric
			for _, image := range images {
				var fullImage model.PharosImageMeta
				if err := databaseContext.DB.Preload("Findings").Preload("ContextRoots.Contexts").First(&fullImage, "image_id = ?", image.ImageId).Error; err != nil {
					mc.Logger.Warn().Err(err).Str("imageId", image.ImageId).Msg("Failed to retrieve Docker image")
				} else {
					summary := fullImage.GetSummary()
					mc.Logger.Debug().Str("imageId", image.ImageId).Any("summary", summary).Msg("Found image in database")

					for level, count := range summary.Severities {
						mc.Vulnerabilities.WithLabelValues(fullImage.ImageSpec, fullImage.IndexDigest, fullImage.ImageId, fullImage.ArchOS+"/"+fullImage.ArchName, level).Set(float64(count))
					}
					for _, contextRoot := range fullImage.ContextRoots {
						for _, context := range contextRoot.Contexts {
							for label := range context.Data {
								labels[label] = ""
							}
						}
					}

				}
			}
			// Step 2: Create GaugeVec for contexts metric and fill it with data
			// First we have to know what labels we have to expect, so we can create the GaugeVec with the correct labels.
			// Now that we have all the labels, we can create the contexts metric
			desc := prometheus.GaugeOpts{
				Name: "pharos_contexts",
				Help: "Contexts for images",
			}
			keys := []string{}
			values := []string{}
			for i, v := range labels {
				keys = append(keys, i)
				values = append(values, v)
			}
			contexts := prometheus.NewGaugeVec(desc, keys)
			prometheus.Unregister(contexts) // Unregister if it already exists to avoid duplicate registration
			prometheus.MustRegister(contexts)
			// now we have to iterate again, and write the contexts
			for _, image := range images {
				var fullImage model.PharosImageMeta
				if err := databaseContext.DB.Preload("Findings").Preload("ContextRoots.Contexts").First(&fullImage, "image_id = ?", image.ImageId).Error; err != nil {
					mc.Logger.Warn().Err(err).Str("imageId", image.ImageId).Msg("Failed to retrieve Docker image")
				} else {
					for _, contextRoot := range fullImage.ContextRoots {
						// we must reset the labels first
						// Empty values will be ignored by Prometheus, so we can just set them to empty strings.
						for label := range labels {
							labels[label] = ""
						}
						labels["image"] = fullImage.ImageSpec
						labels["digest"] = fullImage.IndexDigest
						labels["imageid"] = fullImage.ImageId
						labels["platform"] = fullImage.ArchOS + "/" + fullImage.ArchName
						for _, context := range contextRoot.Contexts {
							for label, value := range context.Data {
								labels[label] = fmt.Sprintf("%v", value)
							}
						}
						contexts.With(labels).Set(1)
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
