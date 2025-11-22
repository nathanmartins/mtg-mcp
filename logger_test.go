package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func TestInitLogger(t *testing.T) {
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
			err := InitLogger(tt.logFilePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitLogger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Check if log file was created
				if _, statErr := os.Stat(tt.logFilePath); os.IsNotExist(statErr) {
					t.Errorf("InitLogger() log file not created at %s", tt.logFilePath)
				}

				// Test that logger can be used
				GetLogger().Info().Msg("test log message")

				// Check that GetLogger returns a non-nil logger
				if l := GetLogger(); l == nil {
					t.Error("GetLogger() returned nil")
				}
			}
		})
	}
}

func TestInitLogger_InvalidPath(t *testing.T) {
	// Try to create a log file in a non-existent directory without creating it
	invalidPath := "/nonexistent/directory/that/does/not/exist/test.log"
	err := InitLogger(invalidPath)
	if err == nil {
		t.Error("InitLogger() expected error with invalid path, got nil")
	}
}

func TestSetLogLevel(t *testing.T) {
	// Initialize logger first with a temp file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	if err := InitLogger(logPath); err != nil {
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
			SetLogLevel(tt.level)
			currentLevel := zerolog.GlobalLevel()
			if currentLevel != tt.wantLevel {
				t.Errorf("SetLogLevel(%q) = %v, want %v", tt.level, currentLevel, tt.wantLevel)
			}
		})
	}
}

func TestGetLogger(t *testing.T) {
	// Initialize logger first
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	if err := InitLogger(logPath); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	l := GetLogger()
	if l == nil {
		t.Error("GetLogger() returned nil")
	}

	// Test that we can use the logger
	l.Info().Msg("test message from GetLogger")
}

func TestLogger_MultipleWrites(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	if err := InitLogger(logPath); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Write multiple log messages
	log := GetLogger()
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
	if err := InitLogger(logPath); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Write concurrently from multiple goroutines
	done := make(chan bool)
	for i := range 10 {
		go func(id int) {
			GetLogger().Info().Int("goroutine", id).Msg("concurrent write")
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
