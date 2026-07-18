package main

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

const (
	logLevelDebug = "debug"
	logLevelInfo  = "info"
	logLevelWarn  = "warn"
	logLevelError = "error"
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

	// Console writer with colors. MUST be stderr: the MCP stdio transport owns
	// stdout, so any diagnostics on stdout corrupt the JSON-RPC stream.
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stderr,
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
	case logLevelDebug:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case logLevelInfo:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case logLevelWarn:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case logLevelError:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

// GetLogger returns the global logger.
func GetLogger() *zerolog.Logger {
	return loggerHolder.get()
}
