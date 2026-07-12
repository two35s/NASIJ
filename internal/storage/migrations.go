package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed sql/*.sql
var sqlFS embed.FS

// runMigrations applies all unapplied SQL migration files in version order.
//
// Migration files must live in the embedded sql/ directory and follow the
// naming convention: NNN_description.sql (e.g. 001_init.sql, 002_add_index.sql).
// The numeric prefix determines application order.
//
// This function is idempotent: already-applied migrations are detected via
// the schema_version table and skipped.
func runMigrations(ctx context.Context, db *sql.DB) error {
	// Bootstrap: create schema_version before reading it.
	// This statement is safe to re-run (IF NOT EXISTS).
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_version (
			version    INTEGER  PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`); err != nil {
		return fmt.Errorf("migrations: bootstrap schema_version: %w", err)
	}

	// Read the highest applied version.
	var current int
	row := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`)
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("migrations: read current version: %w", err)
	}

	// Collect and sort migration files.
	migrations, err := collectMigrations()
	if err != nil {
		return err
	}

	// Apply unapplied migrations in order.
	for _, m := range migrations {
		if m.version <= current {
			continue
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("migrations: begin tx for v%d: %w", m.version, err)
		}

		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrations: apply %q: %w", m.name, err)
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_version (version) VALUES (?)`, m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrations: record v%d: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrations: commit v%d: %w", m.version, err)
		}
	}

	return nil
}

type migration struct {
	version int
	name    string
	sql     string
}

func collectMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(sqlFS, "sql")
	if err != nil {
		return nil, fmt.Errorf("migrations: read embedded sql dir: %w", err)
	}

	var migrations []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}

		// Parse version number from filename prefix (e.g. "001" from "001_init.sql")
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) < 1 {
			continue
		}
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			continue // skip non-numeric prefixes silently
		}

		content, err := sqlFS.ReadFile("sql/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("migrations: read %q: %w", e.Name(), err)
		}

		migrations = append(migrations, migration{
			version: v,
			name:    e.Name(),
			sql:     string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	return migrations, nil
}
