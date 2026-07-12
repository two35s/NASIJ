package workspace_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nasij/nasij/internal/logger"
	"github.com/nasij/nasij/internal/workspace"
)

func newTestManager(t *testing.T) *workspace.Manager {
	t.Helper()
	root := t.TempDir()
	log := logger.Nop()
	m, err := workspace.NewManager(root, log)
	require.NoError(t, err)
	return m
}

func TestManager_Create_Basic(t *testing.T) {
	m := newTestManager(t)

	ws, err := m.Create("test-workspace", "https://example.com")
	require.NoError(t, err)

	assert.NotEmpty(t, ws.ID)
	assert.Equal(t, "test-workspace", ws.Name)
	assert.Equal(t, "https://example.com", ws.Target)
	assert.NotEmpty(t, ws.Root)
	assert.False(t, ws.CreatedAt.IsZero())
	assert.Equal(t, 0, ws.ScanCount)
	assert.Nil(t, ws.LastScanAt)
}

func TestManager_Create_DirectoryStructure(t *testing.T) {
	m := newTestManager(t)

	ws, err := m.Create("dirs-test", "https://example.com")
	require.NoError(t, err)

	// All expected subdirectories must exist
	require.DirExists(t, ws.Root)
	require.DirExists(t, ws.AssetsPath())
	require.DirExists(t, ws.ReportsPath())
	require.DirExists(t, ws.LogsPath())
	require.DirExists(t, ws.CachePath())
	require.DirExists(t, ws.ScreenshotsPath())
	require.DirExists(t, ws.SnapshotsPath())

	// meta.json must be written
	require.FileExists(t, ws.Root+"/meta.json")
}

func TestManager_Create_EmptyName(t *testing.T) {
	m := newTestManager(t)
	_, err := m.Create("", "https://example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestManager_Create_EmptyTarget(t *testing.T) {
	m := newTestManager(t)
	_, err := m.Create("test", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target")
}

func TestManager_Open(t *testing.T) {
	m := newTestManager(t)
	ws, err := m.Create("open-test", "https://example.com")
	require.NoError(t, err)

	opened, err := m.Open(ws.ID)
	require.NoError(t, err)

	assert.Equal(t, ws.ID, opened.ID)
	assert.Equal(t, ws.Name, opened.Name)
	assert.Equal(t, ws.Target, opened.Target)
	assert.WithinDuration(t, ws.CreatedAt, opened.CreatedAt, time.Second)
}

func TestManager_Open_NotFound(t *testing.T) {
	m := newTestManager(t)
	_, err := m.Open("00000000-0000-0000-0000-000000000000")
	require.Error(t, err)
	assert.True(t, errors.Is(err, workspace.ErrNotFound))
}

func TestManager_List_Empty(t *testing.T) {
	m := newTestManager(t)
	list, err := m.List()
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestManager_List_SortedNewestFirst(t *testing.T) {
	m := newTestManager(t)

	ws1, err := m.Create("first", "https://first.example.com")
	require.NoError(t, err)
	ws2, err := m.Create("second", "https://second.example.com")
	require.NoError(t, err)

	list, err := m.List()
	require.NoError(t, err)
	require.Len(t, list, 2)

	// Newest first (ws2 was created after ws1)
	assert.True(t, list[0].CreatedAt.After(list[1].CreatedAt) ||
		list[0].CreatedAt.Equal(list[1].CreatedAt))
	_ = ws1
	_ = ws2
}

func TestManager_Delete(t *testing.T) {
	m := newTestManager(t)

	ws, err := m.Create("to-delete", "https://example.com")
	require.NoError(t, err)

	require.NoError(t, m.Delete(ws.ID))

	list, err := m.List()
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestManager_Delete_NotFound(t *testing.T) {
	m := newTestManager(t)
	err := m.Delete("00000000-0000-0000-0000-000000000000")
	require.Error(t, err)
	assert.True(t, errors.Is(err, workspace.ErrNotFound))
}

func TestWorkspace_Paths(t *testing.T) {
	m := newTestManager(t)
	ws, err := m.Create("path-test", "https://example.com")
	require.NoError(t, err)

	assert.Equal(t, ws.Root+"/workspace.db", ws.DBPath())
	assert.Equal(t, ws.Root+"/assets", ws.AssetsPath())
	assert.Equal(t, ws.Root+"/reports", ws.ReportsPath())
	assert.Equal(t, ws.Root+"/logs", ws.LogsPath())
	assert.Equal(t, ws.Root+"/cache", ws.CachePath())
	assert.Equal(t, ws.Root+"/screenshots", ws.ScreenshotsPath())
	assert.Equal(t, ws.Root+"/snapshots", ws.SnapshotsPath())
}

func TestManager_Create_HasMetadata(t *testing.T) {
	m := newTestManager(t)
	ws, err := m.Create("meta-test", "https://example.com")
	require.NoError(t, err)

	assert.Equal(t, 2, ws.Version, "schema version should be 2")
	assert.NotEmpty(t, ws.Hash, "content hash should be set")
	assert.Equal(t, "idle", ws.Status)
	assert.False(t, ws.ModifiedAt.IsZero())
	assert.NotEmpty(t, ws.ContentHash())
}

func TestManager_Resume_Healthy(t *testing.T) {
	m := newTestManager(t)
	ws, err := m.Create("resume-test", "https://example.com")
	require.NoError(t, err)

	rws, err := m.Resume(ws.ID)
	require.NoError(t, err)
	assert.Equal(t, "idle", rws.Status)
	assert.Equal(t, ws.Name, rws.Name)
}

func TestManager_Resume_MissingDir(t *testing.T) {
	m := newTestManager(t)
	ws, err := m.Create("resume-missing", "https://example.com")
	require.NoError(t, err)

	require.NoError(t, os.RemoveAll(ws.AssetsPath()))

	_, err = m.Resume(ws.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "integrity check failed")
}

func TestManager_SetStatus(t *testing.T) {
	m := newTestManager(t)
	ws, err := m.Create("status-test", "https://example.com")
	require.NoError(t, err)

	require.NoError(t, m.SetStatus(ws.ID, "scanning"))

	updated, err := m.Open(ws.ID)
	require.NoError(t, err)
	assert.Equal(t, "scanning", updated.Status)
}

func TestManager_UpdateMeta(t *testing.T) {
	m := newTestManager(t)
	ws, err := m.Create("update-test", "https://example.com")
	require.NoError(t, err)

	oldHash := ws.Hash
	ws.Name = "renamed"
	require.NoError(t, m.UpdateMeta(ws))

	updated, err := m.Open(ws.ID)
	require.NoError(t, err)
	assert.Equal(t, "renamed", updated.Name)
	assert.NotEqual(t, oldHash, updated.Hash, "hash should change after rename")
}

func TestManager_IncrementScanCount(t *testing.T) {
	m := newTestManager(t)
	ws, err := m.Create("scancount-test", "https://example.com")
	require.NoError(t, err)
	assert.Equal(t, 0, ws.ScanCount)

	require.NoError(t, m.IncrementScanCount(ws.ID))
	require.NoError(t, m.IncrementScanCount(ws.ID))

	updated, err := m.Open(ws.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, updated.ScanCount)
	assert.NotNil(t, updated.LastScanAt)
}

func TestWorkspace_ContentHash_ChangesWithFields(t *testing.T) {
	ws1 := &workspace.Workspace{
		ID: "a", Name: "test", Target: "https://example.com", Root: "/tmp/ws",
		Version: 2,
	}
	ws2 := &workspace.Workspace{
		ID: "b", Name: "test", Target: "https://example.com", Root: "/tmp/ws",
		Version: 2,
	}

	assert.NotEqual(t, ws1.ContentHash(), ws2.ContentHash(), "different IDs should produce different hashes")
}
