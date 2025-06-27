package integrations

import (
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog/log"
)

/*
TODO Should return pure errors instead puma
*/
func SaveScanResult(databaseContext *model.DatabaseContext, pharosScanResult *model.PharosScanResult) error {
	pharosScanResult.Image.Vulnerabilities = pharosScanResult.Vulnerabilities // Ensure vulnerabilities are set
	pharosScanResult.Image.Findings = pharosScanResult.Findings               // Ensure findings are set
	pharosScanResult.Image.Packages = pharosScanResult.Packages               // Ensure packages are set
	// Does the image already exist in the database?
	var value model.PharosImageMeta
	var query = model.PharosImageMeta{
		ImageId: pharosScanResult.Image.ImageId,
	}
	if err := databaseContext.DB.Find(&value, &query).Error; err != nil {
		log.Error().Err(err).Msg("Failed to retrieve Docker images")
		return fmt.Errorf("Failed to retrieve Docker images.", err)
	}
	if value.ImageId == "" {
		log.Info().Str("imageId", pharosScanResult.Image.ImageId).Msg("Image ID does not exist, creating new image metadata")
		tx := databaseContext.DB.Create(pharosScanResult.Image) // Try to Create the updated image metadata
		if tx.Error != nil {
			log.Error().Err(tx.Error).Msg("Failed to save image metadata in database")
			return huma.Error500InternalServerError("Failed to save image metadata in database: " + tx.Error.Error())
		}
		log.Info().Str("imageId", pharosScanResult.Image.ImageId).Msg("Created image metadata in database")
	} else {
		log.Info().Str("imageId", pharosScanResult.Image.ImageId).Msg("Updating existing image metadata")
		tx := databaseContext.DB.Save(pharosScanResult.Image) // Try to Save the updated image metadata
		if tx.Error != nil {
			log.Error().Err(tx.Error).Msg("Failed to save image metadata in database")
			return huma.Error500InternalServerError("Failed to save image metadata in database: " + tx.Error.Error())
		}
		log.Info().Str("imageId", pharosScanResult.Image.ImageId).Msg("Updated image metadata in database")
	}
	return nil
}
