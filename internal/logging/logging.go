package logging

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// return new logger with <logoutput> json,console and <loglevel> debug,info,warn

func NewLogger(logLevel string) *zerolog.Logger {

	// Set log output format
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}).With().Timestamp().Logger()
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().UTC()
	}
	// Set log level, default info
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	switch strings.ToLower(logLevel) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	}
	return &logger
}
