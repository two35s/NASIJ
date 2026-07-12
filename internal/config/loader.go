package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Load initialises configuration from (in priority order):
//  1. Built-in defaults
//  2. Config file at cfgPath (if non-empty) or ~/.nasij/config.yaml
//  3. Environment variables prefixed with NASIJ_ (dots become underscores)
//
// If the config file is not found, defaults + environment variables are used
// (this is not an error — it allows first-run usage without any config file).
func Load(cfgPath string) (*Config, error) {
	v := viper.New()

	cfg := Defaults()

	// Register all defaults so viper knows about every key.
	v.SetDefault("log_level", cfg.LogLevel)
	v.SetDefault("log_format", cfg.LogFormat)
	v.SetDefault("workspace_root", cfg.WorkspaceRoot)
	v.SetDefault("db.max_open_conns", cfg.DB.MaxOpenConns)
	v.SetDefault("db.max_idle_conns", cfg.DB.MaxIdleConns)
	v.SetDefault("plugin.dir", cfg.Plugin.Dir)
	v.SetDefault("plugin.enabled", cfg.Plugin.Enabled)

	// Environment variable support (NASIJ_LOG_LEVEL, NASIJ_DB_MAX_OPEN_CONNS, …)
	v.SetEnvPrefix("NASIJ")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Config file discovery
	if cfgPath != "" {
		v.SetConfigFile(cfgPath)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("config: determine home dir: %w", err)
		}
		v.AddConfigPath(filepath.Join(home, ".nasij"))
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) || os.IsNotExist(err) {
			// No config file — use defaults + env. This is expected on first run.
		} else {
			// A config file was found but could not be parsed.
			return nil, fmt.Errorf("config: read %q: %w", v.ConfigFileUsed(), err)
		}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	// Expand ~ so downstream code receives an absolute path.
	cfg.WorkspaceRoot = ExpandHome(cfg.WorkspaceRoot)

	return cfg, nil
}

// DefaultConfigPath returns the canonical config file path for the current user.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: home dir: %w", err)
	}
	return filepath.Join(home, ".nasij", "config.yaml"), nil
}

// validate enforces allowed values for enumerated fields.
func validate(cfg *Config) error {
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[cfg.LogLevel] {
		return fmt.Errorf("config: invalid log_level %q (must be one of: debug, info, warn, error)", cfg.LogLevel)
	}

	validFormats := map[string]bool{"json": true, "pretty": true}
	if !validFormats[cfg.LogFormat] {
		return fmt.Errorf("config: invalid log_format %q (must be one of: json, pretty)", cfg.LogFormat)
	}

	return nil
}

// ExpandHome replaces a leading ~/ with the current user's home directory.
func ExpandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}
