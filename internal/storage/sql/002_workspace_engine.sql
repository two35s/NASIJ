-- Migration 002: Workspace engine enhancements
-- Adds content hashing, status tracking, scan snapshots, and cache/screenshots support.

ALTER TABLE workspaces ADD COLUMN version     INTEGER NOT NULL DEFAULT 1;
ALTER TABLE workspaces ADD COLUMN hash        TEXT;
ALTER TABLE workspaces ADD COLUMN status      TEXT    NOT NULL DEFAULT 'idle';
ALTER TABLE workspaces ADD COLUMN modified_at DATETIME;

-- Scan snapshots: point-in-time records of workspace state.
CREATE TABLE IF NOT EXISTS snapshots (
    id             TEXT     PRIMARY KEY,
    workspace_id   TEXT     NOT NULL,
    created_at     DATETIME NOT NULL DEFAULT (datetime('now')),
    label          TEXT,
    asset_count    INTEGER  NOT NULL DEFAULT 0,
    api_count      INTEGER  NOT NULL DEFAULT 0,
    finding_count  INTEGER  NOT NULL DEFAULT 0,
    size_bytes     INTEGER  NOT NULL DEFAULT 0,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_snapshots_workspace ON snapshots(workspace_id);

-- Asset content tracking: content-addressed storage for deduplication.
CREATE TABLE IF NOT EXISTS asset_blobs (
    hash       TEXT     PRIMARY KEY,
    size       INTEGER NOT NULL,
    mime_type  TEXT,
    first_seen DATETIME NOT NULL DEFAULT (datetime('now')),
    ref_count  INTEGER NOT NULL DEFAULT 1
);
