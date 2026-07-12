// Package plugin defines the plugin interface and type system for NASIJ.
//
// All NASIJ plugins must implement the Plugin interface.
// Plugins are categorised by Kind (analyzer, reporter, exporter).
// The plugin system is intentionally minimal in Phase 0;
// capability-specific interfaces (Analyzer, Reporter, etc.) are added in later phases.
package plugin

import "context"

// Kind categorises a plugin by its primary function.
type Kind string

const (
	// KindAnalyzer plugins process collected data and emit findings.
	KindAnalyzer Kind = "analyzer"

	// KindReporter plugins format and output scan results.
	KindReporter Kind = "reporter"

	// KindExporter plugins convert results to third-party formats (Burp, Postman, etc.).
	KindExporter Kind = "exporter"
)

// Plugin is the minimal interface every NASIJ plugin must satisfy.
// Later phases will add capability-specific sub-interfaces that plugins
// can optionally implement (e.g. Analyzer, Reporter).
type Plugin interface {
	// Name returns the unique plugin identifier (e.g. "api-extractor").
	// Names must be stable — they are used as keys in the registry.
	Name() string

	// Version returns the plugin's semantic version string (e.g. "1.0.0").
	Version() string

	// Kind returns the plugin category.
	Kind() Kind

	// Init is called once immediately after the plugin is registered.
	// cfg is the plugin-specific configuration map (may be nil).
	// Plugins must be usable after Init returns without error.
	Init(ctx context.Context, cfg map[string]any) error
}
