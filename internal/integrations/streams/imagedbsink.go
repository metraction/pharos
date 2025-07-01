package streams

import (
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams"
	"github.com/rs/zerolog"
)

var _ streams.Sink = (*ImageDbSink)(nil)

type ImageDbSink struct {
	Logger          *zerolog.Logger
	DatabaseContext *model.DatabaseContext
	in              chan any
	done            chan struct{}
}

func NewImageDbSink(databaseContext *model.DatabaseContext) *ImageDbSink {
	is := &ImageDbSink{
		Logger:          logging.NewLogger("info", "component", "ImageSink"),
		DatabaseContext: databaseContext,
		in:              make(chan any),
		done:            make(chan struct{}),
	}
	go is.process()
	return is
}

func (is *ImageDbSink) process() {

	defer close(is.done)
	for elem := range is.in {
		pharosScanResult, ok := elem.(model.PharosScanResult)
		if !ok {
			is.Logger.Error().Msg("Received non-PharosScanResult item")
			continue
		}
		logger := is.Logger.With().Str("ImageId", pharosScanResult.Image.ImageId).Logger()
		pharosScanResult.Image.Vulnerabilities = pharosScanResult.Vulnerabilities // Ensure vulnerabilities are set
		pharosScanResult.Image.Findings = pharosScanResult.Findings               // Ensure findings are set
		pharosScanResult.Image.Packages = pharosScanResult.Packages               // Ensure packages are set
		// Does the image already exist in the database?
		var value model.PharosImageMeta
		var query = model.PharosImageMeta{
			ImageId: pharosScanResult.Image.ImageId,
		}
		if err := is.DatabaseContext.DB.Find(&value, &query).Error; err != nil {
			logger.Error().Err(err).Msg("Failed to retrieve Docker images")
			continue
		}
		if value.ImageId == "" {
			logger.Info().Msg("Image ID does not exist, creating new image metadata")
			tx := is.DatabaseContext.DB.Create(pharosScanResult.Image) // Try to Create the updated image metadata
			if tx.Error != nil {
				logger.Error().Err(tx.Error).Msg("Failed to save image metadata in database")
				continue
			}
			logger.Info().Msg("Created image metadata in database")
		} else {
			logger.Info().Msg("Updating existing image metadata")
			var query = model.ContextRoot{
				ImageId: pharosScanResult.Image.ImageId,
				Key:     pharosScanResult.ScanTask.ContextRootKey,
			}
			tx := is.DatabaseContext.DB.Delete(&query)
			if tx.Error != nil {
				logger.Error().Err(tx.Error).Msg("Failed to delete associations")
				continue
			}
			tx = is.DatabaseContext.DB.Save(pharosScanResult.Image) // Try to Save the updated image metadata
			if tx.Error != nil {
				logger.Error().Err(tx.Error).Msg("Failed to save image metadata in database")
				continue
			}
			logger.Info().Msg("Updated image metadata in database")
		}
		logger.Info().Msg("Image saved successfully")
	}
}

// In returns the input channel of the ImageSink connector.
func (is *ImageDbSink) In() chan<- any {
	return is.in
}

// AwaitCompletion blocks until the ImageSink has processed all received data.
func (is *ImageDbSink) AwaitCompletion() {
	<-is.done
}
