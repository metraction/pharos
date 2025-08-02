package metriccollectors

import (
	"github.com/metraction/pharos/internal/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

var _ prometheus.Collector = (*ChannelCollector)(nil)

// Collects performance metrics for channels used in the application.

type ChannelCollector struct {
	Logger      *zerolog.Logger
	Channels    map[string]chan any // Map to hold channels for performance tracking
	QueueLength *prometheus.Desc    // Metric to track queue length
}

// NewChannelCollector creates a new ChannelCollector instance.
func NewChannelCollector() *ChannelCollector {
	return &ChannelCollector{
		Logger:   logging.NewLogger("info", "component", "ChannelCollector"),
		Channels: make(map[string]chan any),
		QueueLength: prometheus.NewDesc(
			"pharos_queue_length",
			"Size of queue for each channel",
			[]string{"queue_name"}, nil,
		)}
}

// Describe implements the prometheus.Collector interface.
func (cc *ChannelCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cc.QueueLength
}

// Collect implements the prometheus.Collector interface.
func (cc *ChannelCollector) Collect(ch chan<- prometheus.Metric) {
	cc.Logger.Info().Msg("Collect called")
	for name, channel := range cc.Channels {
		queueLength := float64(len(channel))
		ch <- prometheus.MustNewConstMetric(
			cc.QueueLength,
			prometheus.GaugeValue,
			queueLength,
			name,
		)
	}
}

// Registers a channel for performance tracking and returns the collector for chaining.
// Register a buffered channel to track its length.
func (cc *ChannelCollector) WithChannel(channelName string, channel chan any) *ChannelCollector {
	cc.Channels[channelName] = channel
	cc.Logger.Info().Str("channel_name", channelName).Msg("Channel registered for performance tracking")
	return cc
}
