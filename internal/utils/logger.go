package utils

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	backendWriter io.Writer
	webWriter     io.Writer
	logDir        string
)

// InitLogger initializes the global logger with sane defaults
// Logs to both stdout and rotating log files (separate files for backend and web)
// Environment variables can override:
//   - LOG_LEVEL: debug, info, warn, error (default: info)
//   - LOG_FORMAT: json, text (default: json)
//   - LOG_DIR: directory for log files (default: /app/logs in Docker, ./logs locally)
func InitLogger(level, format, component string) {
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

	// Determine log directory (allow override via env var)
	logDir = os.Getenv("LOG_DIR")
	if logDir == "" {
		// Default to /app/logs in Docker, ./logs for local development
		// Check if /app exists (Docker environment indicator)
		if _, err := os.Stat("/app"); err == nil {
			logDir = "/app/logs"
		} else {
			logDir = "./logs"
		}
	}

	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Error().Err(err).Str("dir", logDir).Msg("Failed to create log directory, using stdout only")
		// Fallback to stdout only
		if format == "text" {
			log.Logger = log.Output(zerolog.ConsoleWriter{
				Out:        os.Stdout,
				TimeFormat: time.RFC3339,
			})
		} else {
			log.Logger = zerolog.New(os.Stdout).With().Timestamp().Str("component", component).Logger()
		}
		return
	}

	// Create rotating file writers for backend and web logs
	backendWriter = createLogWriter(filepath.Join(logDir, "backend.log"))
	webWriter = createLogWriter(filepath.Join(logDir, "web.log"))

	// Default component name if not provided
	if component == "" {
		component = "backend"
	}

	// Create multi-writer: stdout + backend file (backend logs go to backend.log)
	// Web logs will be handled separately by the middleware
	var writers []io.Writer
	writers = append(writers, os.Stdout)
	writers = append(writers, backendWriter)

	multiWriter := io.MultiWriter(writers...)

	// Set log format
	if format == "text" {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        multiWriter,
			TimeFormat: time.RFC3339,
		})
	} else {
		// JSON format (default)
		log.Logger = zerolog.New(multiWriter).With().Timestamp().Str("component", component).Logger()
	}

	log.Info().
		Str("level", logLevel.String()).
		Str("format", format).
		Str("log_dir", logDir).
		Msg("Logger initialized")
}

// createLogWriter creates a rotating log file writer
func createLogWriter(filename string) io.Writer {
	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    100,  // 100 MB per file
		MaxBackups: 3,    // Keep 3 old log files
		MaxAge:     28,   // Keep logs for 28 days
		Compress:   true, // Compress rotated logs with gzip
	}
}

// GetWebLogger returns a logger for web/API requests that writes to web.log
func GetWebLogger() zerolog.Logger {
	if webWriter == nil {
		// Fallback to default logger if not initialized
		return log.Logger
	}

	// Create multi-writer: stdout + web file
	multiWriter := io.MultiWriter(os.Stdout, webWriter)
	return zerolog.New(multiWriter).With().Timestamp().Str("component", "web").Logger()
}
