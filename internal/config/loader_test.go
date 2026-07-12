package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nasij/nasij/internal/config"
)

func TestDefaults(t *testing.T) {
	cfg := config.Defaults()
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "pretty", cfg.LogFormat)
	assert.Contains(t, cfg.WorkspaceRoot, ".nasij/workspaces")
	assert.Equal(t, 1, cfg.DB.MaxOpenConns)
	assert.Equal(t, 1, cfg.DB.MaxIdleConns)
	assert.Equal(t, "plugins", cfg.Plugin.Dir)
	assert.True(t, cfg.Plugin.Enabled)
}

func TestLoad_NilConfigFile_ReturnsDefaults(t *testing.T) {
	// Unset any NASIJ env vars that could interfere
	t.Setenv("NASIJ_LOG_LEVEL", "")
	t.Setenv("NASIJ_LOG_FORMAT", "")

	cfg, err := config.Load("/nonexistent/path/that/does/not/exist.yaml")
	require.NoError(t, err, "missing config file should not be an error")
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "pretty", cfg.LogFormat)
}

func TestLoad_ValidConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
log_level: debug
log_format: json
workspace_root: /tmp/test-workspaces
db:
  max_open_conns: 2
  max_idle_conns: 2
plugin:
  dir: /tmp/plugins
  enabled: false
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(content), 0o644))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "/tmp/test-workspaces", cfg.WorkspaceRoot)
	assert.Equal(t, 2, cfg.DB.MaxOpenConns)
	assert.False(t, cfg.Plugin.Enabled)
}

func TestLoad_EnvVarOverride(t *testing.T) {
	t.Setenv("NASIJ_LOG_LEVEL", "warn")
	t.Setenv("NASIJ_LOG_FORMAT", "json")

	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Equal(t, "warn", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("log_level: turbo\n"), 0o644))

	_, err := config.Load(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "log_level")
}

func TestLoad_InvalidLogFormat(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("log_format: xml\n"), 0o644))

	_, err := config.Load(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "log_format")
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	result := config.ExpandHome("~/.nasij")
	assert.Equal(t, filepath.Join(home, ".nasij"), result)

	result = config.ExpandHome("/absolute/path")
	assert.Equal(t, "/absolute/path", result)

	result = config.ExpandHome("relative/path")
	assert.Equal(t, "relative/path", result)
}
