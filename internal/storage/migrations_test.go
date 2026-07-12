package storage_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nasij/nasij/internal/storage"
)

func TestOpen_CreatesDatabase(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := storage.Open(ctx, dbPath)
	require.NoError(t, err)
	defer db.Close()

	require.FileExists(t, dbPath)
}

func TestOpen_RunsMigrations(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := storage.Open(ctx, dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Verify expected tables exist
	tables := []string{
		"schema_version", "workspaces", "audit_log",
		"js_assets", "api_records", "findings",
		"snapshots", "asset_blobs",
		"scope_entries", "scope_config",
	}
	for _, table := range tables {
		var name string
		err := db.DB().QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		require.NoError(t, err, "table %q should exist", table)
		assert.Equal(t, table, name)
	}
}

func TestOpen_MigrationsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Open twice — second open should not fail or duplicate data
	db1, err := storage.Open(ctx, dbPath)
	require.NoError(t, err)
	require.NoError(t, db1.Close())

	db2, err := storage.Open(ctx, dbPath)
	require.NoError(t, err)
	defer db2.Close()

	// schema_version should have exactly 2 rows (two migrations applied)
	var count int
	err = db2.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_version`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestDB_Ping(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(ctx, filepath.Join(dir, "ping.db"))
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, db.Ping(ctx))
}

func TestDB_SQLiteVersion(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(ctx, filepath.Join(dir, "version.db"))
	require.NoError(t, err)
	defer db.Close()

	v, err := db.SQLiteVersion(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, v)
	t.Logf("SQLite version: %s", v)
}

func TestDB_WithTx_Commit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(ctx, filepath.Join(dir, "tx.db"))
	require.NoError(t, err)
	defer db.Close()

	err = db.WithTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO workspaces (id, name, target, created_at) VALUES (?, ?, ?, datetime('now'))`,
			"tx-id", "tx-test", "https://example.com",
		)
		return err
	})
	require.NoError(t, err)

	var count int
	err = db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM workspaces`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestDB_WithTx_Rollback(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(ctx, filepath.Join(dir, "tx_rollback.db"))
	require.NoError(t, err)
	defer db.Close()

	err = db.WithTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO workspaces (id, name, target, created_at) VALUES (?, ?, ?, datetime('now'))`,
			"rollback-id", "rollback-test", "https://example.com",
		)
		require.NoError(t, err)
		return fmt.Errorf("force rollback")
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "force rollback")

	// Verify the insert was rolled back
	var count int
	err = db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM workspaces`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
