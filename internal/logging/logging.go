package logging

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// return new logger with <logoutput> json,console and <loglevel> debug,info,warn

func NewLogger(logLevel string, extra ...string) *zerolog.Logger {

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
	if (len(extra) % 2) != 0 {
		panic("extra strings must be in key=value format (even number of arguments)")
	}
	for i := 0; i < len(extra)/2; i += 2 {
		logger = logger.With().Str(extra[i], extra[i+1]).Logger()
	}

	return &logger
}
