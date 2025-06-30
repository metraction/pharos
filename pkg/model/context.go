package model

import (
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"
)

type ContextRoot struct {
	Key       string `gorm:"primaryKey"`
	ImageId   string `gorm:"primaryKey"`
	UpdatedAt time.Time
	TTL       time.Duration
	Contexts  []Context `gorm:"foreignKey:ContextRootKey,ImageId;references:Key,ImageId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// TODO: Should return pure error, not huma.Error
func (cr *ContextRoot) Save(databaseContext *DatabaseContext, log *zerolog.Logger) error {
	var query = ContextRoot{
		Key: cr.Key,
	}
	var value ContextRoot
	if err := databaseContext.DB.Find(&value, &query).Error; err != nil {
		log.Error().Err(err).Msg("Failed to retrieve Docker images")
		return fmt.Errorf("Failed to retrieve Docker images: %w", err)
	}
	if value.Key == "" {
		log.Info().Str("key", cr.Key).Msg("Image ID does not exist, creating new image ContextRoot")
		tx := databaseContext.DB.Create(cr) // Try to Create the updated image metadata
		if tx.Error != nil {
			log.Error().Err(tx.Error).Msg("Failed to save ContextRoot in database")
			return huma.Error500InternalServerError("Failed to save ContextRoot: " + tx.Error.Error())
		}
		log.Info().Str("key", cr.Key).Msg("Created image metadata in database")
	} else {
		log.Info().Str("key", cr.Key).Msg("Updating existing ContextRoot")
		tx := databaseContext.DB.Save(cr) // Try to Save the updated image metadata
		if tx.Error != nil {
			log.Error().Err(tx.Error).Msg("Failed to save image metadata in database")
			return huma.Error500InternalServerError("Failed to save image metadata in database: " + tx.Error.Error())
		}
		log.Info().Str("key", cr.Key).Msg("Updated image ContextRoot in database")
	}
	return nil
}

type Context struct {
	ID             uint   `gorm:"primaryKey"` // Auto-incrementing primary key
	ContextRootKey string // Composite Foreign Key to the ContextRoot Table
	ImageId        string // Composite Foreign Key to the ContextRoot Table
	Owner          string // The owner of the Context, this is the plugin that has created / changed it. Will be a Foreign Key to the Plugins Table
	UpdatedAt      time.Time
	Data           map[string]any `gorm:"serializer:json"` // Context data
}
