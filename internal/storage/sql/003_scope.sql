-- Migration 003: Scope management
-- Tables for workspace scope definitions: allowed domains, subdomains,
-- CIDR ranges, exclude rules, regex patterns, rate limiting, and auth.

-- Scope entries: individual include/exclude rules for a workspace.
CREATE TABLE IF NOT EXISTS scope_entries (
    id           TEXT     PRIMARY KEY,
    workspace_id TEXT     NOT NULL,
    entry_type   TEXT     NOT NULL CHECK (entry_type IN ('domain','subdomain','cidr','exclude','regex')),
    pattern      TEXT     NOT NULL,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_scope_entries_workspace ON scope_entries(workspace_id);
CREATE INDEX IF NOT EXISTS idx_scope_entries_type ON scope_entries(entry_type);

-- Scope configuration: singleton config row per workspace for rate limits and auth.
CREATE TABLE IF NOT EXISTS scope_config (
    workspace_id     TEXT     PRIMARY KEY,
    rate_limit_rps   REAL     NOT NULL DEFAULT 10.0,
    rate_limit_burst INTEGER  NOT NULL DEFAULT 5,
    auth_type        TEXT     CHECK (auth_type IN ('','header','cookie','basic','bearer')),
    auth_value       TEXT     NOT NULL DEFAULT '',
    custom_headers   TEXT,               -- JSON object of extra headers
    respect_robots   INTEGER  NOT NULL DEFAULT 1,
    robots_body      TEXT,               -- cached robots.txt content
    created_at       DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at       DATETIME NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);
