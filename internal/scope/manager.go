package scope

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Manager handles scope CRUD operations against a workspace SQLite database.
type Manager struct {
	db *sql.DB
}

// NewManager creates a scope manager backed by the given DB connection.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// Load retrieves the full scope definition for a workspace from the database.
func (m *Manager) Load(ctx context.Context, workspaceID, targetHost string) (*Scope, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, workspace_id, entry_type, pattern, created_at
		 FROM scope_entries WHERE workspace_id = ?
		 ORDER BY entry_type, created_at`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("scope load entries: %w", err)
	}
	defer rows.Close()

	s := &Scope{
		WorkspaceID:   workspaceID,
		TargetHost:    targetHost,
		RespectRobots: true,
	}

	for rows.Next() {
		var e ScopeEntry
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.EntryType, &e.Pattern, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scope load scan: %w", err)
		}
		switch e.EntryType {
		case TypeDomain:
			s.Domains = append(s.Domains, e.Pattern)
		case TypeSubdomain:
			s.Subdomains = append(s.Subdomains, e.Pattern)
		case TypeCIDR:
			s.RawCIDRs = append(s.RawCIDRs, e.Pattern)
		case TypeExclude:
			s.Excludes = append(s.Excludes, e.Pattern)
		case TypeRegex:
			s.RawRegex = append(s.RawRegex, e.Pattern)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scope load rows: %w", err)
	}

	// Parse CIDRs and compile regexes
	s.CIDRs, _ = ParseCIDRs(s.RawCIDRs)
	s.Regexes = CompileRegex(s.RawRegex)

	// Load config (rate limits, auth, robots)
	var (
		authType      sql.NullString
		authValue     sql.NullString
		customHdrs    sql.NullString
		respectRbt    sql.NullInt64
		robotsBody    sql.NullString
		rateLimitRps  sql.NullFloat64
		rateLimitBrst sql.NullInt64
	)
	err = m.db.QueryRowContext(ctx,
		`SELECT rate_limit_rps, rate_limit_burst, auth_type, auth_value,
		        custom_headers, respect_robots, robots_body
		 FROM scope_config WHERE workspace_id = ?`, workspaceID).Scan(
		&rateLimitRps, &rateLimitBrst, &authType, &authValue,
		&customHdrs, &respectRbt, &robotsBody)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("scope load config: %w", err)
	}

	if rateLimitRps.Valid {
		s.RateLimit.RequestsPerSec = rateLimitRps.Float64
	} else {
		s.RateLimit.RequestsPerSec = 10.0
	}
	if rateLimitBrst.Valid {
		s.RateLimit.Burst = int(rateLimitBrst.Int64)
	} else {
		s.RateLimit.Burst = 5
	}
	if authType.Valid && authType.String != "" {
		s.Auth.Type = authType.String
		s.Auth.Value = authValue.String
	}
	if customHdrs.Valid && customHdrs.String != "" {
		_ = json.Unmarshal([]byte(customHdrs.String), &s.Auth.CustomHeaders)
	}
	if respectRbt.Valid {
		s.RespectRobots = respectRbt.Int64 == 1
	}

	return s, nil
}

// AddRule inserts a new scope rule.
func (m *Manager) AddRule(ctx context.Context, workspaceID, entryType, pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Errorf("scope add rule: pattern must not be empty")
	}

	switch entryType {
	case TypeDomain:
		if err := ValidateDomain(pattern); err != nil {
			return fmt.Errorf("scope add domain: %w", err)
		}
	case TypeCIDR:
		if err := ValidateCIDR(pattern); err != nil {
			return fmt.Errorf("scope add cidr: %w", err)
		}
	case TypeRegex:
		if err := ValidateRegex(pattern); err != nil {
			return fmt.Errorf("scope add regex: %w", err)
		}
	case TypeSubdomain:
		if IsWildcardDomain(pattern) {
			pattern = DomainFromWildcard(pattern)
			entryType = TypeDomain
		}
	case TypeExclude:
		// any string is valid
	default:
		return fmt.Errorf("scope add rule: unknown entry type %q", entryType)
	}

	id := uuid.New().String()
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO scope_entries (id, workspace_id, entry_type, pattern, created_at)
		 VALUES (?, ?, ?, ?, datetime('now'))`,
		id, workspaceID, entryType, pattern)
	if err != nil {
		return fmt.Errorf("scope add rule: %w", err)
	}
	return nil
}

// RemoveRule deletes a scope rule by its ID.
func (m *Manager) RemoveRule(ctx context.Context, ruleID string) error {
	res, err := m.db.ExecContext(ctx, `DELETE FROM scope_entries WHERE id = ?`, ruleID)
	if err != nil {
		return fmt.Errorf("scope remove rule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("scope remove rule: rule %q not found", ruleID)
	}
	return nil
}

// ClearRules removes all scope rules of the given type for a workspace.
// If entryType is empty, all rules are removed.
func (m *Manager) ClearRules(ctx context.Context, workspaceID, entryType string) error {
	if entryType == "" {
		_, err := m.db.ExecContext(ctx, `DELETE FROM scope_entries WHERE workspace_id = ?`, workspaceID)
		return err
	}
	_, err := m.db.ExecContext(ctx,
		`DELETE FROM scope_entries WHERE workspace_id = ? AND entry_type = ?`,
		workspaceID, entryType)
	return err
}

// SetRateLimit updates the rate limit config for a workspace.
func (m *Manager) SetRateLimit(ctx context.Context, workspaceID string, rps float64, burst int) error {
	return m.upsertConfig(ctx, workspaceID, map[string]interface{}{
		"rate_limit_rps":   rps,
		"rate_limit_burst": burst,
	})
}

// SetAuth updates the auth config for a workspace.
func (m *Manager) SetAuth(ctx context.Context, workspaceID, authType, authValue string) error {
	return m.upsertConfig(ctx, workspaceID, map[string]interface{}{
		"auth_type":  authType,
		"auth_value": authValue,
	})
}

// SetCustomHeaders updates the custom headers for a workspace.
func (m *Manager) SetCustomHeaders(ctx context.Context, workspaceID string, headers map[string]string) error {
	data, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("scope set headers: %w", err)
	}
	return m.upsertConfig(ctx, workspaceID, map[string]interface{}{
		"custom_headers": string(data),
	})
}

// SetRespectRobots sets whether the scanner should respect robots.txt.
func (m *Manager) SetRespectRobots(ctx context.Context, workspaceID string, respect bool) error {
	v := 0
	if respect {
		v = 1
	}
	return m.upsertConfig(ctx, workspaceID, map[string]interface{}{
		"respect_robots": v,
	})
}

// ListEntries returns all scope entries for a workspace.
func (m *Manager) ListEntries(ctx context.Context, workspaceID string) ([]ScopeEntry, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, workspace_id, entry_type, pattern, created_at
		 FROM scope_entries WHERE workspace_id = ?
		 ORDER BY entry_type, created_at`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("scope list: %w", err)
	}
	defer rows.Close()

	var entries []ScopeEntry
	for rows.Next() {
		var e ScopeEntry
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.EntryType, &e.Pattern, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scope list scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// --- private helpers ---

func (m *Manager) upsertConfig(ctx context.Context, workspaceID string, cols map[string]interface{}) error {
	var keys []string
	var args []interface{}
	args = append(args, workspaceID)

	for k, v := range cols {
		keys = append(keys, k)
		args = append(args, v)
	}

	// Build SET clauses for ON CONFLICT using excluded.* to reference INSERT values
	var setClauses []string
	for _, k := range keys {
		setClauses = append(setClauses, fmt.Sprintf("%s = excluded.%s", k, k))
	}
	setClauses = append(setClauses, "updated_at = datetime('now')")

	keysStr := strings.Join(keys, ", ")
	placeholders := strings.Repeat(", ?", len(keys))

	query := fmt.Sprintf(
		`INSERT INTO scope_config (workspace_id, %[1]s, created_at, updated_at)
		 VALUES (?%[2]s, datetime('now'), datetime('now'))
		 ON CONFLICT(workspace_id) DO UPDATE SET %[3]s`,
		keysStr, placeholders, strings.Join(setClauses, ", "),
	)

	_, err := m.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("scope upsert config: %w", err)
	}
	return nil
}
