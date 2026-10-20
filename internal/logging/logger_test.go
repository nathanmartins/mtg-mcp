package logging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func TestInit(t *testing.T) {
	// Create a temporary directory for test log files
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		logFilePath string
		wantErr     bool
	}{
		{
			name:        "valid log file path",
			logFilePath: filepath.Join(tmpDir, "test.log"),
			wantErr:     false,
		},
		{
			name:        "log file in temp directory",
			logFilePath: filepath.Join(tmpDir, "test2.log"),
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Init(tt.logFilePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Check if log file was created
				if _, statErr := os.Stat(tt.logFilePath); os.IsNotExist(statErr) {
					t.Errorf("Init() log file not created at %s", tt.logFilePath)
				}

				// Test that logger can be used
				Get().Info().Msg("test log message")

				// Check that Get returns a non-nil logger
				if l := Get(); l == nil {
					t.Error("Get() returned nil")
				}
			}
		})
	}
}

func TestInit_InvalidPath(t *testing.T) {
	// A regular file where the log directory should be makes the path unusable on every OS.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	err := Init(filepath.Join(blocker, "test.log"))
	if err == nil {
		t.Fatal("Init() expected error with invalid path, got nil")
	}
	// The server must keep running on a stderr-only logger instead of exiting.
	Get().Info().Msg("still logging")
}

func TestInit_CreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "test.log")
	if err := Init(path); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	Get().Info().Msg("hello")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("log file not created: %v", err)
	}
}

func TestDefaultFilePath(t *testing.T) {
	path := DefaultFilePath()
	if filepath.Base(path) != FileName || filepath.Base(filepath.Dir(path)) != "mtg-mcp" {
		t.Errorf("DefaultFilePath() = %q, want .../mtg-mcp/%s", path, FileName)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("DefaultFilePath() = %q, want an absolute path independent of the working directory", path)
	}
}

func TestSetLevel(t *testing.T) {
	// Initialize logger first with a temp file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	if err := Init(logPath); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	tests := []struct {
		name      string
		level     string
		wantLevel zerolog.Level
	}{
		{
			name:      "debug level",
			level:     "debug",
			wantLevel: zerolog.DebugLevel,
		},
		{
			name:      "info level",
			level:     "info",
			wantLevel: zerolog.InfoLevel,
		},
		{
			name:      "warn level",
			level:     "warn",
			wantLevel: zerolog.WarnLevel,
		},
		{
			name:      "error level",
			level:     "error",
			wantLevel: zerolog.ErrorLevel,
		},
		{
			name:      "invalid level defaults to info",
			level:     "invalid",
			wantLevel: zerolog.InfoLevel,
		},
		{
			name:      "empty string defaults to info",
			level:     "",
			wantLevel: zerolog.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLevel(tt.level)
			currentLevel := zerolog.GlobalLevel()
			if currentLevel != tt.wantLevel {
				t.Errorf("SetLevel(%q) = %v, want %v", tt.level, currentLevel, tt.wantLevel)
			}
		})
	}
}

func TestGet(t *testing.T) {
	// Initialize logger first
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	if err := Init(logPath); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	l := Get()
	if l == nil {
		t.Error("Get() returned nil")
	}

	// Test that we can use the logger
	l.Info().Msg("test message from Get")
}

func TestLogger_MultipleWrites(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	if err := Init(logPath); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Write multiple log messages
	log := Get()
	log.Info().Msg("message 1")
	log.Debug().Msg("message 2")
	log.Warn().Msg("message 3")
	log.Error().Msg("message 4")

	// Check that file exists and has content
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Error("Log file is empty after writing messages")
	}
}

func TestLogger_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "concurrent.log")
	if err := Init(logPath); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Write concurrently from multiple goroutines
	done := make(chan bool)
	for i := range 10 {
		go func(id int) {
			Get().Info().Int("goroutine", id).Msg("concurrent write")
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for range 10 {
		<-done
	}

	// Check that file was written
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Error("Log file is empty after concurrent writes")
	}
}
