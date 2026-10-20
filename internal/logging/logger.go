// Package logging configures the process-wide zerolog logger. Output goes to stderr and a log
// file, never stdout, which the MCP stdio transport owns.
package logging

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
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

// Init initializes the global logger with both console and file output.
func Init(logFilePath string) error {
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

// SetLevel sets the global log level.
func SetLevel(level string) {
	switch level {
	case LevelDebug:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case LevelInfo:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case LevelWarn:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case LevelError:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

// Get returns the global logger.
func Get() *zerolog.Logger {
	return loggerHolder.get()
}
