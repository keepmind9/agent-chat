package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_Success(t *testing.T) {
	content := `
port: "9090"
db: /tmp/test.db
api_key: my-secret-key
retention: 15
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, "9090", cfg.Port)
	require.Equal(t, "/tmp/test.db", cfg.DB)
	require.Equal(t, "my-secret-key", cfg.APIKey)
	require.Equal(t, 15, cfg.Retention)
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "read config")
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("port: ["), 0644))

	_, err := Load(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse config")
}

func TestLoad_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(""), 0644))

	cfg, err := Load(path)
	require.NoError(t, err)
	require.Empty(t, cfg.Port)
	require.Empty(t, cfg.DB)
}

func TestExists(t *testing.T) {
	require.False(t, Exists("/nonexistent/config.yaml"))

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("port: 8080"), 0644))
	require.True(t, Exists(path))
}

func TestDataDir(t *testing.T) {
	dir := DataDir()
	require.NotEmpty(t, dir)
	require.Contains(t, dir, ".agent-chat")
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	require.Contains(t, path, ".agent-chat")
	require.Contains(t, path, "config.yaml")
}
