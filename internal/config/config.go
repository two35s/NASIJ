// Package config defines the NASIJ application configuration schema.
// All configuration is loaded through the Loader and decoded into Config.
package config

// Config is the top-level application configuration structure.
// Values are loaded from (in ascending priority order):
//  1. Built-in defaults
//  2. ~/.nasij/config.yaml
//  3. NASIJ_* environment variables
//  4. CLI flags (applied after loading)
type Config struct {
	// LogLevel controls the minimum log severity.
	// Valid values: debug, info, warn, error.
	LogLevel string `mapstructure:"log_level"`

	// LogFormat controls the log output format.
	// Valid values: pretty (human-readable), json (machine-readable).
	LogFormat string `mapstructure:"log_format"`

	// WorkspaceRoot is the directory where all workspace data is stored.
	// Supports ~ expansion.
	WorkspaceRoot string `mapstructure:"workspace_root"`

	// DB holds SQLite connection pool settings.
	DB DatabaseConfig `mapstructure:"db"`

	// Plugin holds plugin system settings.
	Plugin PluginConfig `mapstructure:"plugin"`
}

// DatabaseConfig holds per-workspace SQLite connection pool settings.
type DatabaseConfig struct {
	// MaxOpenConns is the maximum number of open DB connections.
	// SQLite is single-writer; keep this at 1.
	MaxOpenConns int `mapstructure:"max_open_conns"`

	// MaxIdleConns is the maximum number of idle DB connections kept open.
	MaxIdleConns int `mapstructure:"max_idle_conns"`
}

// PluginConfig holds settings for the plugin loader.
type PluginConfig struct {
	// Dir is the directory scanned for .so plugin files.
	Dir string `mapstructure:"dir"`

	// Enabled controls whether the plugin system is active.
	Enabled bool `mapstructure:"enabled"`
}

// Defaults returns a Config populated with safe, conservative defaults.
func Defaults() *Config {
	return &Config{
		LogLevel:      "info",
		LogFormat:     "pretty",
		WorkspaceRoot: "~/.nasij/workspaces",
		DB: DatabaseConfig{
			MaxOpenConns: 1,
			MaxIdleConns: 1,
		},
		Plugin: PluginConfig{
			Dir:     "plugins",
			Enabled: true,
		},
	}
}
