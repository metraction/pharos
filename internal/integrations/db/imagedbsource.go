package db

import (
	"time"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
	"github.com/rs/zerolog"
)

type ImageDbSource struct {
	Config          *model.Config
	in              chan any
	DatabaseContext *model.DatabaseContext
	Logger          *zerolog.Logger
	ScannerFlow     bool
}

var _ streams.Source = (*ImageDbSource)(nil)

func NewImageDbSource(databaseContext *model.DatabaseContext, config *model.Config) *ImageDbSource {
	is := &ImageDbSource{
		DatabaseContext: databaseContext,
		in:              make(chan any),
		Logger:          logging.NewLogger("info", "component", "ImageDbSource"),
		Config:          config,
	}
	is.Start() // Start the periodic run
	return is
}

// Out returns the output channel of the ChanSource connector.
func (is *ImageDbSource) Out() <-chan any {
	return is.in
}

// Via asynchronously streams data to the given Flow and returns it.
func (is *ImageDbSource) Via(operator streams.Flow) streams.Flow {
	flow.DoStream(is, operator)
	return operator
}

func (is *ImageDbSource) Start() {
	ticker := time.NewTicker(60 * time.Second)
	go is.RunAsync() // Initial run to populate the channel
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				is.Logger.Info().Msg("ImageDbSource wakes up")
				is.RunAsync()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func (sr *ImageDbSource) CheckSplit(element any) bool {
	_, ret := element.(string)
	if ret {
		return false
	}
	// If the element is a string, it indicates a maintenance task
	return true
}

func (is *ImageDbSource) RunAsync() {
	is.Logger.Info().Msg("Running ImageDbSource asynchronously")
	var images []model.PharosImageMeta

	is.Logger.Info().Msg("Fetchng images from database")
	is.DatabaseContext.DB.Preload("ContextRoots").Find(&images)
	for _, image := range images {
		is.in <- image
	}

}
