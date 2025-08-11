package db

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/metraction/pharos/pkg/model"
)

type ImageDbHandler struct {
	DatabaseContext *model.DatabaseContext
}

func NewImageDbHandler(databaseContext *model.DatabaseContext) *ImageDbHandler {
	return &ImageDbHandler{
		DatabaseContext: databaseContext,
	}
}

func (ih *ImageDbHandler) RemoveExpiredContexts(item model.PharosImageMeta) model.PharosImageMeta {
	for _, contextRoot := range item.ContextRoots {
		if contextRoot.IsExpired() {
			ih.DatabaseContext.Logger.Info().Str("ImageId", item.ImageId).Str("ContextRootKey", contextRoot.Key).Msg("Removing expired context root")
			tx := ih.DatabaseContext.DB.Model(&model.ContextRoot{}).Delete(&contextRoot)
			if tx.Error != nil {
				ih.DatabaseContext.Logger.Error().Err(tx.Error).Str("ImageId", item.ImageId).Str("ContextRootKey", contextRoot.Key).Msg("Failed to remove expired context root")
			}

		}
	}
	return item
}

func (ih *ImageDbHandler) HandleAlerts(item model.PharosImageMeta) model.PharosImageMeta {

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
		severities := item.GetSummary().Severities
		for k, v := range severities {
			labels = append(labels, model.AlertLabel{
				Name:  k,
				Value: fmt.Sprintf("%v", v),
			})
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
		if err := ih.DatabaseContext.DB.Find(&value, &query).Error; err != nil {
			ih.DatabaseContext.Logger.Error().Err(err).Msg("Failed to retrieve Alert")
			continue
		}
		if value.Fingerprint == "" {
			ih.DatabaseContext.DB.Create(&alert)
		} else {
			ih.DatabaseContext.DB.Save(&alert)
		}
	}
	return item
}

func (ih *ImageDbHandler) RemoveImagesWithoutContext(item model.PharosImageMeta) model.PharosImageMeta {
	if len(item.ContextRoots) == 0 {
		ih.DatabaseContext.Logger.Info().Str("ImageId", item.ImageId).Msg("Image has no context roots, removing image")
		tx := ih.DatabaseContext.DB.Model(&model.PharosImageMeta{}).Delete(&item)
		if tx.Error != nil {
			ih.DatabaseContext.Logger.Error().Err(tx.Error).Str("ImageId", item.ImageId).Msg("Failed to remove image")

		}
	}
	// TODO: Remove image with ImageId = "" - we have to find out where it comes from.
	ih.DatabaseContext.DB.Model(&model.PharosImageMeta{}).Where("image_id = ?", "").Delete(&model.PharosImageMeta{})
	return item
}
