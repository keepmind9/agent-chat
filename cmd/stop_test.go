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