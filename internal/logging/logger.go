// Package logging configures the process-wide zerolog logger. Output goes to stderr and a log
// file, never stdout, which the MCP stdio transport owns.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

// Log level names accepted by SetLevel.
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

// testLogsEnv, when set, makes the default logger write to stderr; see newLoggerInstance.
const testLogsEnv = "MTG_MCP_TEST_LOGS"

// newLoggerInstance returns the logger used until Init runs. The binary always calls Init
// first, so in practice this is the logger tests see, and it discards everything to keep test
// output readable. Set MTG_MCP_TEST_LOGS=1 to send it to stderr while debugging a test.
func newLoggerInstance() *loggerInstance {
	if os.Getenv(testLogsEnv) == "" {
		return &loggerInstance{logger: zerolog.Nop()}
	}
	return &loggerInstance{
		logger: zerolog.New(os.Stderr).With().Timestamp().Logger(),
	}
}

var loggerHolder = newLoggerInstance() //nolint:gochecknoglobals // logger needs to be accessible throughout the application

// FileName is the log file's name inside the default log directory.
const FileName = "mtg-commander-server.log"

// DefaultFilePath returns where the log file goes when none is configured: an mtg-mcp folder
// in the user's cache directory. MCP clients may launch the server from any working directory,
// including read-only ones such as "/", so the working directory is never used. It falls back
// to the temp directory when the platform reports no cache directory.
func DefaultFilePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "mtg-mcp", FileName)
}

// Init installs the global logger, writing to stderr and to logFilePath, creating the file's
// directory if needed. The log file is a convenience, not a requirement: when it cannot be
// opened, Init still installs a stderr-only logger and returns the error, so the caller can
// warn and keep serving.
func Init(logFilePath string) error {
	// Console writer with colors. MUST be stderr: the MCP stdio transport owns
	// stdout, so any diagnostics on stdout corrupt the JSON-RPC stream.
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
		NoColor:    false,
	}
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	logFile, err := openLogFile(logFilePath)
	if err != nil {
		loggerHolder.logger = zerolog.New(consoleWriter).With().Timestamp().Caller().Logger()
		return err
	}

	loggerHolder.logger = zerolog.New(zerolog.MultiLevelWriter(consoleWriter, logFile)).
		With().
		Timestamp().
		Caller().
		Logger()
	return nil
}

// openLogFile opens path for appending, creating it and its directory as needed.
func openLogFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	logFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return logFile, nil
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
