package alerting

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/metraction/pharos/pkg/model"
)

func GetPrometheusAlert(a *model.Alert) *model.PrometheusAlert {
	if a == nil {
		return nil
	}
	labels := make(map[string]string, len(a.Labels))
	for _, l := range a.Labels {
		labels[l.Name] = l.Value
	}
	annotations := make(map[string]string, len(a.Annotations))
	for _, a := range a.Annotations {
		annotations[a.Name] = a.Value
	}
	return &model.PrometheusAlert{
		Status:       a.Status,
		Labels:       labels,
		Annotations:  annotations,
		StartsAt:     a.StartsAt,
		EndsAt:       a.EndsAt,
		GeneratorURL: a.GeneratorURL,
		Fingerprint:  a.Fingerprint,
	}
}

func HandleAlerts(databaseContext *model.DatabaseContext) func(item model.PharosImageMeta) model.PharosImageMeta {
	return func(item model.PharosImageMeta) model.PharosImageMeta {
		for _, contextRoot := range item.ContextRoots {
			labels := []model.AlertLabel{}
			labels = append(labels, model.AlertLabel{
				Name:  "imagespec",
				Value: item.ImageSpec,
			})
			labels = append(labels, model.AlertLabel{
				Name:  "imageid",
				Value: item.ImageId,
			})
			labels = append(labels, model.AlertLabel{
				Name:  "digest",
				Value: item.ManifestDigest,
			})
			labels = append(labels, model.AlertLabel{
				Name:  "platform",
				Value: item.ArchOS + "/" + item.ArchName,
			})
			for _, context := range contextRoot.Contexts {
				for label, value := range context.Data {
					switch v := value.(type) {
					case string, int, int32, int64, float32, float64, bool, time.Time, time.Duration:
						labels = append(labels, model.AlertLabel{
							Name:  label,
							Value: fmt.Sprintf("%v", v),
						})
					default:
					}
				}
			}
			status := "firing"
			if contextRoot.IsExpired() {
				status = "resolved"
			}
			alert := model.Alert{
				Labels:      labels,
				Annotations: []model.AlertAnnotation{},
				Status:      status,
				StartsAt:    contextRoot.UpdatedAt,
				EndsAt:      contextRoot.UpdatedAt.Add(contextRoot.TTL),
			}
			hash := sha256.Sum256([]byte(contextRoot.ImageId + "/" + contextRoot.Key))
			alert.Fingerprint = "sha256:" + hex.EncodeToString(hash[:])
			var value model.Alert
			var query = model.Alert{
				Fingerprint: alert.Fingerprint,
			}
			if err := databaseContext.DB.Find(&value, &query).Error; err != nil {
				databaseContext.Logger.Error().Err(err).Msg("Failed to retrieve Alert")
				continue
			}
			if value.Fingerprint == "" {
				databaseContext.Logger.Debug().Str("fingerprint", alert.Fingerprint).Str("imageid", item.ImageId).Str("imagespec", item.ImageSpec).Str("status", alert.Status).Msg("Creating new alert")
				databaseContext.DB.Create(&alert)
			} else {
				databaseContext.Logger.Debug().Str("fingerprint", alert.Fingerprint).Str("imageid", item.ImageId).Str("imagespec", item.ImageSpec).Str("status", alert.Status).Msg("Updating existing alert")
				databaseContext.DB.Save(&alert)
			}
		}
		return item
	}
}
