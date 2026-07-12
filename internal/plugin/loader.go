package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// Loader scans a directory for plugin binaries and loads them into a Registry.
//
// Phase 0: discovers .so files and reports counts; actual dlopen loading
// is deferred to Phase 3 when the plugin SDK is defined.
//
// Phase 3+: will use the Go plugin package to open .so files and look up
// the exported "NASIJPlugin" symbol (which must implement Plugin).
type Loader struct {
	dir      string
	registry *Registry
	log      *zap.Logger
}

// NewLoader creates a Loader that will scan dir and register found plugins
// into registry.
func NewLoader(dir string, registry *Registry, log *zap.Logger) *Loader {
	return &Loader{
		dir:      dir,
		registry: registry,
		log:      log,
	}
}

// Load scans the plugin directory and loads available plugins.
// Returns the number of plugin files found (Phase 0: found, not loaded).
// A missing directory is not an error.
func (l *Loader) Load(ctx context.Context) (int, error) {
	info, err := os.Stat(l.dir)
	if os.IsNotExist(err) {
		l.log.Debug("plugin directory not found; skipping scan",
			zap.String("dir", l.dir),
		)
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("plugin loader: stat %q: %w", l.dir, err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("plugin loader: %q is not a directory", l.dir)
	}

	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return 0, fmt.Errorf("plugin loader: read dir %q: %w", l.dir, err)
	}

	found := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".so") {
			continue
		}
		path := filepath.Join(l.dir, entry.Name())
		l.log.Info("discovered plugin binary (Phase 3 loading pending)",
			zap.String("path", path),
			zap.String("name", entry.Name()),
		)
		found++
	}

	l.log.Info("plugin scan complete",
		zap.String("dir", l.dir),
		zap.Int("found", found),
		zap.Int("registered", l.registry.Count()),
	)

	return found, nil
}

// Dir returns the plugin directory this Loader scans.
func (l *Loader) Dir() string { return l.dir }
