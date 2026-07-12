package workspace

import (
	"crypto/sha256"
	"fmt"
	"time"
)

const (
	// SchemaVersion is the current workspace metadata schema version.
	SchemaVersion = 2

	// StatusIdle indicates the workspace is ready but not actively scanning.
	StatusIdle = "idle"

	// StatusScanning indicates an active scan is in progress.
	StatusScanning = "scanning"

	// StatusPaused indicates a scan was paused and can be resumed.
	StatusPaused = "paused"

	// StatusError indicates the workspace is in an error state.
	StatusError = "error"

	// DBName is the SQLite database filename within the workspace root.
	DBName = "workspace.db"
)

type Workspace struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Target     string     `json:"target"`
	Root       string     `json:"root"`
	Version    int        `json:"version"`
	Hash       string     `json:"hash"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	ModifiedAt time.Time  `json:"modified_at"`
	LastScanAt *time.Time `json:"last_scan_at,omitempty"`
	ScanCount  int        `json:"scan_count"`
}

// ContentHash computes a SHA-256 hash of the workspace's identity fields.
// Used to detect tampering or accidental changes to workspace metadata.
func (w *Workspace) ContentHash() string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%d", w.ID, w.Name, w.Target, w.Root, w.Version)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (w *Workspace) DBPath() string {
	return w.Root + "/" + DBName
}

func (w *Workspace) AssetsPath() string {
	return w.Root + "/assets"
}

func (w *Workspace) ReportsPath() string {
	return w.Root + "/reports"
}

func (w *Workspace) LogsPath() string {
	return w.Root + "/logs"
}

func (w *Workspace) CachePath() string {
	return w.Root + "/cache"
}

func (w *Workspace) ScreenshotsPath() string {
	return w.Root + "/screenshots"
}

func (w *Workspace) SnapshotsPath() string {
	return w.Root + "/snapshots"
}
