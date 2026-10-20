package main

import (
	"errors"
	"flag"
	"io"
	"testing"

	"github.com/nathanmartins/mtg-mcp/internal/logging"
)

func noEnv(string) string { return "" }

func TestParseOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		getenv  func(string) string
		want    options
		wantErr bool
	}{
		{name: "defaults", args: nil, getenv: noEnv, want: options{logFile: logging.DefaultFilePath()}},
		{
			name: "double-dash version", args: []string{"--version"}, getenv: noEnv,
			want: options{showVersion: true, logFile: logging.DefaultFilePath()},
		},
		{
			name: "single-dash version", args: []string{"-version"}, getenv: noEnv,
			want: options{showVersion: true, logFile: logging.DefaultFilePath()},
		},
		{
			name: "flags combine in any order", args: []string{"--debug", "--version"}, getenv: noEnv,
			want: options{showVersion: true, debug: true, logFile: logging.DefaultFilePath()},
		},
		{
			name: "env sets the log file",
			args: nil,
			getenv: func(key string) string {
				if key == logFileEnv {
					return testEnvLogFile
				}
				return ""
			},
			want: options{logFile: testEnvLogFile},
		},
		{
			name: "flag beats env",
			args: []string{"--log-file", "/tmp/flag.log"},
			getenv: func(string) string {
				return testEnvLogFile
			},
			want: options{logFile: "/tmp/flag.log"},
		},
		{name: "unknown flag", args: []string{"--bogus"}, getenv: noEnv, wantErr: true},
		{name: "stray argument", args: []string{"serve"}, getenv: noEnv, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseOptions(tt.args, tt.getenv, io.Discard)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseOptions() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseOptionsHelp(t *testing.T) {
	t.Parallel()
	if _, err := parseOptions([]string{"--help"}, noEnv, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("parseOptions(--help) error = %v, want flag.ErrHelp", err)
	}
}
