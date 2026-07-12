-- Migration 001: Initial schema
-- Applied by the migration runner on first open of a workspace database.
-- The schema_version table itself is created by the runner before this file runs.

-- Workspace metadata mirror (allows SQL queries over workspace list)
CREATE TABLE IF NOT EXISTS workspaces (
    id           TEXT     PRIMARY KEY,
    name         TEXT     NOT NULL,
    target       TEXT     NOT NULL,
    created_at   DATETIME NOT NULL,
    last_scan_at DATETIME,
    scan_count   INTEGER  NOT NULL DEFAULT 0
);

-- Audit log: every outbound request made by NASIJ within this workspace.
-- Populated by the HTTP client in Phase 2.
CREATE TABLE IF NOT EXISTS audit_log (
    id           INTEGER  PRIMARY KEY AUTOINCREMENT,
    ts           DATETIME NOT NULL DEFAULT (datetime('now')),
    workspace_id TEXT     NOT NULL,
    event        TEXT     NOT NULL,
    detail       TEXT,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

-- JS assets inventory (populated in Phase 2)
CREATE TABLE IF NOT EXISTS js_assets (
    id             TEXT     PRIMARY KEY,
    workspace_id   TEXT     NOT NULL,
    url            TEXT     NOT NULL,
    hash           TEXT     NOT NULL,
    asset_type     TEXT     NOT NULL,  -- inline | external | sourcemap
    source_map_url TEXT,
    first_seen     DATETIME NOT NULL,
    last_seen      DATETIME NOT NULL,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

-- API records discovered through static analysis or runtime observation (Phase 2+)
CREATE TABLE IF NOT EXISTS api_records (
    id              TEXT     PRIMARY KEY,
    workspace_id    TEXT     NOT NULL,
    method          TEXT     NOT NULL,
    route           TEXT     NOT NULL,
    auth_mechanism  TEXT,
    source_file     TEXT,
    source_function TEXT,
    discovered_via  TEXT     NOT NULL,  -- static | runtime | both
    confidence      REAL     NOT NULL DEFAULT 0.5,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

-- Findings (populated by analyzer plugins in Phase 2+)
CREATE TABLE IF NOT EXISTS findings (
    id             TEXT     PRIMARY KEY,
    workspace_id   TEXT     NOT NULL,
    category       TEXT     NOT NULL,
    severity_score REAL     NOT NULL DEFAULT 0.0,
    evidence       TEXT,               -- JSON array
    related_ids    TEXT,               -- JSON array
    created_at     DATETIME NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_audit_log_workspace ON audit_log(workspace_id);
CREATE INDEX IF NOT EXISTS idx_js_assets_workspace ON js_assets(workspace_id);
CREATE INDEX IF NOT EXISTS idx_api_records_workspace ON api_records(workspace_id);
CREATE INDEX IF NOT EXISTS idx_findings_workspace ON findings(workspace_id);
CREATE INDEX IF NOT EXISTS idx_findings_severity ON findings(severity_score DESC);
