// Package workspace defines the Workspace domain entity.
//
// A Workspace is an isolated unit of recon state for a single target.
// Each workspace owns its own SQLite database, downloaded assets,
// generated reports, and audit logs.
package workspace

import "time"

// Workspace represents a NASIJ reconnaissance workspace.
type Workspace struct {
	// ID is the unique workspace identifier (UUID v4).
	ID string `json:"id"`

	// Name is the human-readable label given by the operator.
	Name string `json:"name"`

	// Target is the primary URL being assessed (e.g. "https://example.com").
	Target string `json:"target"`

	// Root is the absolute filesystem path to the workspace directory.
	Root string `json:"root"`

	// CreatedAt is the UTC timestamp when this workspace was created.
	CreatedAt time.Time `json:"created_at"`

	// LastScanAt is the UTC timestamp of the most recent scan, or nil if never scanned.
	LastScanAt *time.Time `json:"last_scan_at,omitempty"`

	// ScanCount is the total number of scans run against this workspace.
	ScanCount int `json:"scan_count"`
}

// DBPath returns the absolute path to the workspace's SQLite database file.
func (w *Workspace) DBPath() string {
	return w.Root + "/workspace.db"
}

// AssetsPath returns the absolute path to the workspace's downloaded-assets directory.
func (w *Workspace) AssetsPath() string {
	return w.Root + "/assets"
}

// ReportsPath returns the absolute path to the workspace's reports directory.
func (w *Workspace) ReportsPath() string {
	return w.Root + "/reports"
}

// LogsPath returns the absolute path to the workspace's audit-log directory.
func (w *Workspace) LogsPath() string {
	return w.Root + "/logs"
}
