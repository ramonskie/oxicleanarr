package utils

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// InitLogger initializes the global logger with the specified configuration
func InitLogger(level, format string) {
	// Set log level
	logLevel := zerolog.InfoLevel
	switch level {
	case "debug":
		logLevel = zerolog.DebugLevel
	case "info":
		logLevel = zerolog.InfoLevel
	case "warn":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	// Set log format
	if format == "text" {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		})
	} else {
		// JSON format (default)
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	log.Info().
		Str("level", logLevel.String()).
		Str("format", format).
		Msg("Logger initialized")
}
