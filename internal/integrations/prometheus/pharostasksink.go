package prometheus

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams"
	"github.com/rs/zerolog"
)

type PharosTaskSink struct {
	log    *zerolog.Logger
	Config *model.Config // Assuming you
	in     chan any
	done   chan struct{}
}

func NewPharosTaskSink(config *model.Config) *PharosTaskSink {
	ps := &PharosTaskSink{
		log:    logging.NewLogger("info"),
		Config: config,
		in:     make(chan any),
		done:   make(chan struct{}),
	}
	go ps.process()
	return ps
}

var _ streams.Sink = (*PharosTaskSink)(nil)

func (ps *PharosTaskSink) process() {
	defer close(ps.done)
	for task := range ps.in {
		scanTask, _ := task.(model.PharosScanTask2)
		url := ps.Config.Prometheus.PharosURL + "/api/pharosscantask/asyncscan"

		client := &http.Client{}

		// Marshal the task to JSON
		jsonBody, err := json.Marshal(scanTask)
		if err != nil {
			ps.log.Error().Err(err).Msg("Failed to marshal task to JSON")
			continue
		}
		resp, err := client.Post(url, "application/json", bytes.NewReader(jsonBody))
		if err != nil {
			ps.log.Error().Err(err).Msg("Failed to POST to PharosScanTask endpoint")
			continue
		}
		pod := resp.Header.Get("Pharos-Pod-Name") // Read the pod name from the response header
		ps.log.Info().Str("image", scanTask.ImageSpec).Str("auth", utils.MaskDsn(scanTask.AuthDsn)).Str("Pod", pod).Msg("Sending task to PharosScanTask endpoint")
		resp.Body.Close()
	}
}

// In returns the input channel of the ImageSink connector.
func (is *PharosTaskSink) In() chan<- any {
	return is.in
}

// AwaitCompletion blocks until the ImageSink has processed all received data.
func (is *PharosTaskSink) AwaitCompletion() {
	<-is.done
}
