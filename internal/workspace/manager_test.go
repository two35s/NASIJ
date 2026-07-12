package workspace_test

import (
	"errors"
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
}
