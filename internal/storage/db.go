// Package storage provides SQLite-backed persistence for NASIJ workspaces.
//
// Each workspace has its own SQLite database file (workspace.db).
// Connections use WAL journal mode for concurrent read performance,
// with a single writer enforced at the Go level.
//
// The modernc.org/sqlite driver is used — it is pure Go with no CGO
// dependency, making cross-compilation and static binaries straightforward.
package storage

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // register "sqlite" driver with database/sql
)

// DB wraps a *sql.DB configured for a NASIJ workspace SQLite file.
type DB struct {
	db   *sql.DB
	path string
}

// Open opens (or creates) the SQLite database at path, applies all pending
// migrations, and returns a ready-to-use DB.
//
// The database is opened with WAL journal mode, foreign keys enabled,
// and a 5-second busy timeout to handle brief write contention.
func Open(ctx context.Context, path string) (*DB, error) {
	dsn := path + "?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("storage open %q: %w", path, err)
	}

	// Single writer: SQLite does not support concurrent writes.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("storage ping %q: %w", path, err)
	}

	store := &DB{db: db, path: path}

	if err := runMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("storage migrations %q: %w", path, err)
	}

	return store, nil
}

// Close releases the database connection pool.
func (d *DB) Close() error {
	return d.db.Close()
}

// Ping verifies the database connection is alive.
func (d *DB) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// WithTx executes fn inside a serialisable transaction.
// The transaction is automatically rolled back if fn returns a non-nil error,
// and committed otherwise.
func (d *DB) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// SQLiteVersion returns the SQLite library version string.
func (d *DB) SQLiteVersion(ctx context.Context) (string, error) {
	var v string
	if err := d.db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&v); err != nil {
		return "", fmt.Errorf("storage sqlite_version: %w", err)
	}
	return v, nil
}

// DB returns the underlying *sql.DB for advanced usage (e.g. custom queries).
func (d *DB) DB() *sql.DB { return d.db }

// Path returns the file path this DB was opened from.
func (d *DB) Path() string { return d.path }
