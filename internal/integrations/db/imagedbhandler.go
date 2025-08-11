package db

import (
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
