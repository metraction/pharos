// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	_ "github.com/dustinkirkland/golang-petname"
	"github.com/metraction/pharos/internal/integrations/redis"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
// 	n, err := lrw.ResponseWriter.Write(b)
// 	//lrw.size = int64(n)
// 	return n, err
// }

type ProbeResult struct {
	Body string `json:"body"`
}

type MetricsController struct {
	Path              string
	Api               *huma.API
	AsyncPublisher    *redis.RedisGtrsClient[model.PharosScanTask2, model.PharosScanResult]
	PriorityPublisher *redis.RedisGtrsClient[model.PharosScanTask2, model.PharosScanResult]
	Config            *model.Config
	Logger            *zerolog.Logger
	contextLabels     map[string]string      // Used to collect labels for contexts metric
	HttpRequests      *prometheus.CounterVec // Metric to track HTTP requests
	TaskChannel       chan any               // Channel for scan tasks
	Version           string
}

func NewMetricsController(api *huma.API, config *model.Config, taskChannel chan any) *MetricsController {
	logger := logging.NewLogger("info", "component", "MetricsController")
	httpRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pharos_http_request_count",
		Help: "Counter for HTTP requests to Pharos API",
	}, []string{"V1/operation.Path ", "operation_id", "method", "status_code"})
	err := prometheus.Register(httpRequests)
	if err != nil {
		logger.Warn().Msg("Failed to register pharos_scantask_status status metric duplicate registration?")
	}
	mc := &MetricsController{
		Path:         "/metrics",
		Api:          api,
		Config:       config,
		Logger:       logger,
		HttpRequests: httpRequests,
		TaskChannel:  taskChannel,
		Version:      "v1",
	}
	return mc
}

func (mc *MetricsController) V1AddRoutes() {
	{
		op, handler := mc.V1GetVulnerbilityMetrics()
		huma.Register(*mc.Api, op, handler)
	}
	{
		op, handler := mc.V1GetContextMetrics()
		huma.Register(*mc.Api, op, handler)
	}
	{
		op, handler := mc.V1GetDefaultMetrics()
		huma.Register(*mc.Api, op, handler)
	}
	{
		op, handler := mc.V1GetLiveness()
		huma.Register(*mc.Api, op, handler)
	}
	{
		op, handler := mc.V1GetReadiness()
		huma.Register(*mc.Api, op, handler)
	}
}

// returns a registry for serving context metrics, may return nil if no context labels are set yet.
func (mc *MetricsController) NewContextRegistry() (*prometheus.Registry, *prometheus.GaugeVec, error) {
	if mc.contextLabels == nil {
		return nil, nil, errors.New("No context labels set for metrics")
	}
	registry := prometheus.NewRegistry()
	desc := prometheus.GaugeOpts{
		Name: "pharos_contexts",
		Help: "Contexts for images",
	}
	keys := []string{}
	values := []string{}
	for i, v := range mc.contextLabels {
		keys = append(keys, i)
		values = append(values, v)
	}
	contexts := prometheus.NewGaugeVec(desc, keys)

	err := registry.Register(contexts)
	if err != nil {
		return nil, nil, err

	}
	return registry, contexts, nil
}

func (mc *MetricsController) NewVulnerabilityRegistry() (*prometheus.Registry, *prometheus.GaugeVec, error) {
	registry := prometheus.NewRegistry()
	desc := prometheus.GaugeOpts{
		Name: "pharos_vulnerabilities",
		Help: "Vulnerabilities for images",
	}
	vulnerabilities := prometheus.NewGaugeVec(desc, []string{"V1/image", "digest", "imageid", "platform", "severity"})
	err := registry.Register(vulnerabilities)
	if err != nil {
		return nil, nil, err

	}
	return registry, vulnerabilities, nil
}

// Gets Vulnerability metrics for pharos controller, vulnerabilities, and images. We are registering an operation with a handler, and use writer and request from a middleware.
// A bit of a hack, but now we have a common way to document things.

func (mc *MetricsController) V1GetVulnerbilityMetrics() (huma.Operation, func(ctx context.Context, input *struct{}) (*struct{ Body string }, error)) {
	return huma.Operation{
			OperationID: "V1GetVulnerbilityMetrics",
			Method:      "GET",
			Path:        mc.Path + "/vulnerabilities",
			Summary:     "Gets Vulnerbility metrics",
			Description: "Gets Vulnerbility metrics",
			Tags:        []string{"V1/Metrics"},
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
				mc.Logger.Info().Str("Handler", "VulnerbilityMetrics").Float64("duration_seconds", v).Msg("Metrics handler duration")
			}))
			defer timer.ObserveDuration()
			registry, _, err := mc.NewVulnerabilityRegistry()
			if err != nil {
				return nil, huma.Error500InternalServerError("Failed to create vulnerbility registry: " + err.Error())
			}
			h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			mc.Logger.Info().Msg("Serving metrics")
			writer := ctx.Value("writer").(http.ResponseWriter)
			request := ctx.Value("request").(*http.Request)
			var images []model.PharosImageMeta
			if err := databaseContext.DB.Find(&images).Error; err != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
			}
			mc.contextLabels = map[string]string{
				"image":    "",
				"digest":   "",
				"imageid":  "",
				"platform": "",
			}
			for _, image := range images {
				var fullImage model.PharosImageMeta
				if err := databaseContext.DB.Preload("Findings").Preload("ContextRoots.Contexts").First(&fullImage, "image_id = ?", image.ImageId).Error; err != nil {
					mc.Logger.Warn().Err(err).Str("imageId", image.ImageId).Msg("Failed to retrieve Docker image")
				} else {
					for _, contextRoot := range fullImage.ContextRoots {
						for _, context := range contextRoot.Contexts {
							for label := range context.Data {
								mc.contextLabels[label] = ""
							}
						}
					}

				}
			}
			h.ServeHTTP(writer, request)
			return nil, nil
		}
}

func (mc *MetricsController) V1GetContextMetrics() (huma.Operation, func(ctx context.Context, input *struct{}) (*struct{ Body string }, error)) {

	return huma.Operation{
			OperationID: "V1GetContextMetrics",
			Method:      "GET",
			Path:        mc.Path + "/contexts",
			Summary:     "Gets Context metrics",
			Description: "Gets Context metrics",
			Tags:        []string{"V1/Metrics"},
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
				mc.Logger.Info().Str("Handler", "ContextMetrics").Float64("duration_seconds", v).Msg("Metrics handler duration")
			}))
			defer timer.ObserveDuration()
			registry, contexts, err := mc.NewContextRegistry()
			if err != nil {
				return nil, huma.Error500InternalServerError("Failed to create context registry: " + err.Error())
			}
			h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
			writer := ctx.Value("writer").(http.ResponseWriter)
			request := ctx.Value("request").(*http.Request)
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var images []model.PharosImageMeta
			if err := databaseContext.DB.Find(&images).Error; err != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve Docker images: " + err.Error())
			}
			for _, image := range images {
				var fullImage model.PharosImageMeta
				if err := databaseContext.DB.Preload("Findings").Preload("ContextRoots.Contexts").First(&fullImage, "image_id = ?", image.ImageId).Error; err != nil {
					mc.Logger.Warn().Err(err).Str("imageId", image.ImageId).Msg("Failed to retrieve Docker image")
				} else {
					for _, contextRoot := range fullImage.ContextRoots {
						// we must reset the labels first
						// Empty values will be ignored by Prometheus, so we can just set them to empty strings.
						for label := range mc.contextLabels {
							mc.contextLabels[label] = ""
						}
						mc.contextLabels["image"] = fullImage.ImageSpec
						mc.contextLabels["digest"] = fullImage.IndexDigest
						mc.contextLabels["imageid"] = fullImage.ImageId
						mc.contextLabels["platform"] = fullImage.ArchOS + "/" + fullImage.ArchName
						for _, context := range contextRoot.Contexts {
							for label, value := range context.Data {
								switch v := value.(type) {
								case string, int, int32, int64, float32, float64, bool, time.Time, time.Duration:
									mc.contextLabels[label] = fmt.Sprintf("%v", v)
								default:
									mc.contextLabels[label] = "UNSUPPORTED_TYPE"
								}
							}
						}
						contexts.With(mc.contextLabels).Set(1)
					}
				}
			}

			h.ServeHTTP(writer, request)
			return nil, nil
		}
}

func (mc *MetricsController) V1GetDefaultMetrics() (huma.Operation, func(ctx context.Context, input *struct{}) (*struct{ Body string }, error)) {

	return huma.Operation{
			OperationID: "V1GetMetrics",
			Method:      "GET",
			Path:        mc.Path,
			Summary:     "Gets Default metrics",
			Description: "Gets Default metrics",
			Tags:        []string{"V1/Metrics"},
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
				mc.Logger.Info().Str("Handler", "DefaultMetrics").Float64("duration_seconds", v).Msg("Metrics handler duration")
			}))
			defer timer.ObserveDuration()
			writer := ctx.Value("writer").(http.ResponseWriter)
			request := ctx.Value("request").(*http.Request)
			h := promhttp.Handler() // default handler for Prometheus metrics

			h.ServeHTTP(writer, request)
			return nil, nil
		}
}

func (mc *MetricsController) V1GetLiveness() (huma.Operation, func(ctx context.Context, input *struct{}) (*ProbeResult, error)) {

	return huma.Operation{
			OperationID: "V1LivenessProbe",
			Method:      "GET",
			Path:        mc.Path + "/liveness",
			Summary:     "Liveness probe",
			Description: "Used for liveness probe",
			Tags:        []string{"V1/Probes"},
			Responses: map[string]*huma.Response{
				"200": {
					Content: map[string]*huma.MediaType{
						"text/plain": {},
					},
					Description: "Check if the service is alive",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *struct{}) (*ProbeResult, error) {
			return &ProbeResult{Body: "OK"}, nil
		}
}

func (mc *MetricsController) V1GetReadiness() (huma.Operation, func(ctx context.Context, input *struct{}) (*ProbeResult, error)) {

	return huma.Operation{
			OperationID: "V1ReadinessProbe",
			Method:      "GET",
			Path:        mc.Path + "/readiness",
			Summary:     "Readiness probe",
			Description: "Returns error if queue is full, otherwise returns OK",
			Tags:        []string{"V1/Probes"},
			Responses: map[string]*huma.Response{
				"200": {
					Content: map[string]*huma.MediaType{
						"text/plain": {},
					},
					Description: "Returns error if queue is full, otherwise returns OK",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *struct{}) (*ProbeResult, error) {
			if len(mc.TaskChannel) == mc.Config.Publisher.QueueSize {
				return nil, huma.Error500InternalServerError("Queue is full, cannot process requests at the moment") // Return 500 if queue is full
			}
			return &ProbeResult{Body: "Ready to serve requests"}, nil
		}
}

// We have to inject the request and writer into the context, so we can use them in the Metrics handler.

func (mc *MetricsController) MetricsMiddleware() func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humachi.Unwrap(ctx)
		ctx = huma.WithValue(ctx, "request", r)
		ctx = huma.WithValue(ctx, "writer", w)
		ctx.AppendHeader("Pharos-Pod-Name", os.Getenv("HOSTNAME")) // Add pod name
		next(ctx)
		mc.HttpRequests.WithLabelValues(
			ctx.Operation().Path,
			ctx.Operation().OperationID,
			ctx.Method(),
			fmt.Sprintf("%d", ctx.Status()),
		).Inc()
		//mc.Logger.Info().Str("Ul.Version +"/" +  Ul.Path ", r.URL.Version +"/" +  URL.Path ).Int("code", ctx.Status()).Str("method", ctx.Method()).Str("OperationId", ctx.Operation().OperationID).Msg("Metrics middleware called")
	}
}
