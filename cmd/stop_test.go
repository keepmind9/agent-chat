package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunStop_NotRunning(t *testing.T) {
	dir := t.TempDir()
	orig := getDataDir
	getDataDir = func() string { return dir }
	defer func() { getDataDir = orig }()

	err := runStop(nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not running")
}

func TestRunStop_InvalidPID(t *testing.T) {
	dir := t.TempDir()
	orig := getDataDir
	getDataDir = func() string { return dir }
	defer func() { getDataDir = orig }()

	pidPath := filepath.Join(dir, pidFileName)
	require.NoError(t, os.WriteFile(pidPath, []byte("not-a-number"), 0644))

	err := runStop(nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid PID")
}

func TestRunStop_ProcessNotFound(t *testing.T) {
	dir := t.TempDir()
	orig := getDataDir
	getDataDir = func() string { return dir }
	defer func() { getDataDir = orig }()

	pidPath := filepath.Join(dir, pidFileName)
	require.NoError(t, os.WriteFile(pidPath, []byte("999999"), 0644))

	err := runStop(nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")

	_, err = os.Stat(pidPath)
	require.True(t, os.IsNotExist(err))
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected string // slog.Level string representation
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"warning", "WARN"},
		{"error", "ERROR"},
		{"", "INFO"},
		{"unknown", "INFO"},
		{"DEBUG", "DEBUG"},
	}
	for _, tc := range tests {
		level := parseLogLevel(tc.input)
		if level.String() != tc.expected {
			t.Errorf("parseLogLevel(%q) = %s, want %s", tc.input, level.String(), tc.expected)
		}
	}
}
