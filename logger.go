package main

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// loggerInstance wraps the zerolog logger for thread-safe access.
type loggerInstance struct {
	logger zerolog.Logger
}

func (l *loggerInstance) get() *zerolog.Logger {
	return &l.logger
}

// newLoggerInstance creates a new logger instance with default configuration.
func newLoggerInstance() *loggerInstance {
	return &loggerInstance{
		logger: zerolog.New(os.Stderr).With().Timestamp().Logger(),
	}
}

var loggerHolder = newLoggerInstance() //nolint:gochecknoglobals // logger needs to be accessible throughout the application

// InitLogger initializes the global logger with both console and file output.
func InitLogger(logFilePath string) error {
	// Create log file
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}

	// Console writer with colors
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    false,
	}

	// Multi-writer: write to both console and file
	multi := zerolog.MultiLevelWriter(consoleWriter, logFile)

	// Create logger
	loggerHolder.logger = zerolog.New(multi).
		With().
		Timestamp().
		Caller().
		Logger()

	// Set global log level
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	return nil
}

// SetLogLevel sets the global log level.
func SetLogLevel(level string) {
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

// GetLogger returns the global logger.
func GetLogger() *zerolog.Logger {
	return loggerHolder.get()
}
