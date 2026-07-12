package workspace

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var ErrNotFound = errors.New("workspace not found")

const metaFilename = "meta.json"

type Manager struct {
	root string
	log  *zap.Logger
}

func NewManager(root string, log *zap.Logger) (*Manager, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("workspace manager: create root %q: %w", root, err)
	}
	return &Manager{root: root, log: log}, nil
}

// createDirs creates all subdirectories for a workspace root.
func createDirs(root string) error {
	dirs := []string{
		root,
		filepath.Join(root, "assets"),
		filepath.Join(root, "reports"),
		filepath.Join(root, "logs"),
		filepath.Join(root, "cache"),
		filepath.Join(root, "screenshots"),
		filepath.Join(root, "snapshots"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o750); err != nil {
			return fmt.Errorf("mkdir %q: %w", d, err)
		}
	}
	return nil
}

func (m *Manager) Create(name, target string) (*Workspace, error) {
	if name == "" {
		return nil, fmt.Errorf("workspace create: name must not be empty")
	}
	if target == "" {
		return nil, fmt.Errorf("workspace create: target must not be empty")
	}

	id := uuid.New().String()
	root := filepath.Join(m.root, id)

	if err := createDirs(root); err != nil {
		_ = os.RemoveAll(root)
		return nil, fmt.Errorf("workspace create: %w", err)
	}

	now := time.Now().UTC()
	ws := &Workspace{
		ID:         id,
		Name:       name,
		Target:     target,
		Root:       root,
		Version:    SchemaVersion,
		Status:     StatusIdle,
		CreatedAt:  now,
		ModifiedAt: now,
		ScanCount:  0,
	}
	ws.Hash = ws.ContentHash()

	if err := m.writeMeta(ws); err != nil {
		_ = os.RemoveAll(root)
		return nil, err
	}

	m.log.Info("workspace created",
		zap.String("id", id),
		zap.String("name", name),
		zap.String("target", target),
		zap.String("root", root),
	)

	return ws, nil
}

// Open loads a workspace by ID. Returns ErrNotFound if it does not exist.
func (m *Manager) Open(id string) (*Workspace, error) {
	return m.readMeta(id)
}

// Resume opens a workspace and verifies its directory structure is intact.
// Returns the workspace and any integrity warnings.
func (m *Manager) Resume(id string) (*Workspace, error) {
	ws, err := m.readMeta(id)
	if err != nil {
		return nil, err
	}

	var warnings []string

	for _, dir := range []string{
		ws.Root,
		ws.AssetsPath(),
		ws.ReportsPath(),
		ws.LogsPath(),
		ws.CachePath(),
		ws.ScreenshotsPath(),
		ws.SnapshotsPath(),
	} {
		info, err := os.Stat(dir)
		if os.IsNotExist(err) {
			warnings = append(warnings, "missing directory: "+dir)
			continue
		}
		if err != nil {
			warnings = append(warnings, "stat error: "+dir+": "+err.Error())
			continue
		}
		if !info.IsDir() {
			warnings = append(warnings, "not a directory: "+dir)
		}
	}

	// Verify content hash
	if ws.Hash != "" && ws.ContentHash() != ws.Hash {
		warnings = append(warnings, "content hash mismatch — metadata may have been modified externally")
	}

	if len(warnings) > 0 {
		m.log.Warn("workspace resume: integrity warnings",
			zap.String("id", id),
			zap.Strings("warnings", warnings),
		)
		ws.Status = StatusError
		_ = m.writeMeta(ws)
		return ws, fmt.Errorf("workspace resume %q: integrity check failed: %v", id, warnings)
	}

	m.log.Info("workspace resumed",
		zap.String("id", id),
		zap.String("name", ws.Name),
	)

	ws.Status = StatusIdle
	ws.ModifiedAt = time.Now().UTC()
	_ = m.writeMeta(ws)

	return ws, nil
}

// UpdateMeta persists the current workspace metadata to disk.
func (m *Manager) UpdateMeta(ws *Workspace) error {
	ws.ModifiedAt = time.Now().UTC()
	ws.Hash = ws.ContentHash()
	return m.writeMeta(ws)
}

// SetStatus updates the workspace status and persists metadata.
func (m *Manager) SetStatus(id, status string) error {
	ws, err := m.readMeta(id)
	if err != nil {
		return err
	}
	ws.Status = status
	ws.ModifiedAt = time.Now().UTC()
	ws.Hash = ws.ContentHash()
	return m.writeMeta(ws)
}

// IncrementScanCount increments the scan counter and sets last scan timestamp.
func (m *Manager) IncrementScanCount(id string) error {
	ws, err := m.readMeta(id)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	ws.LastScanAt = &now
	ws.ScanCount++
	return m.UpdateMeta(ws)
}

func (m *Manager) List() ([]*Workspace, error) {
	entries, err := os.ReadDir(m.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("workspace list: read dir %q: %w", m.root, err)
	}

	var workspaces []*Workspace
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ws, err := m.readMeta(e.Name())
		if err != nil {
			m.log.Warn("skipping workspace with unreadable metadata",
				zap.String("id", e.Name()),
				zap.Error(err),
			)
			continue
		}
		workspaces = append(workspaces, ws)
	}

	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].CreatedAt.After(workspaces[j].CreatedAt)
	})

	return workspaces, nil
}

func (m *Manager) Delete(id string) error {
	root := filepath.Join(m.root, id)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("workspace delete %q: %w", id, err)
	}
	m.log.Info("workspace deleted", zap.String("id", id))
	return nil
}

func (m *Manager) Root() string { return m.root }

// --- private helpers ---

func (m *Manager) writeMeta(ws *Workspace) error {
	if ws.Hash == "" {
		ws.Hash = ws.ContentHash()
	}
	path := filepath.Join(ws.Root, metaFilename)
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return fmt.Errorf("workspace meta marshal: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o640); err != nil {
		return fmt.Errorf("workspace meta write %q: %w", path, err)
	}
	return nil
}

func (m *Manager) readMeta(id string) (*Workspace, error) {
	path := filepath.Join(m.root, id, metaFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return nil, fmt.Errorf("workspace meta read %q: %w", path, err)
	}

	var ws Workspace
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("workspace meta decode %q: %w", path, err)
	}
	return &ws, nil
}

// NewID generates a new workspace ID (UUID v4).
func NewID() string {
	return uuid.New().String()
}

// ComputeHash returns a SHA-256 hash of the raw data.
func ComputeHash(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}
