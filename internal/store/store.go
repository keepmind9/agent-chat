// Package store provides a SQLite-backed persistence layer for agent-chat.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/keepmind9/agent-chat/pkg/protocol"
)

// Store wraps a SQLite database with CRUD operations for agents and messages.
type Store struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at path and runs schema migrations.
func Open(path string) (*Store, error) {
	dsn := path
	if !strings.Contains(dsn, "?") {
		dsn += "?"
	} else {
		dsn += "&"
	}
	dsn += "_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// SQLite single-writer constraint: serialize all access through one connection.
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

// Close releases the database connection.
func (s *Store) Close() {
	s.db.Close()
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS agents (
		name         TEXT PRIMARY KEY,
		groups       TEXT NOT NULL DEFAULT '[]',
		status       TEXT NOT NULL DEFAULT 'online',
		registered_at DATETIME NOT NULL DEFAULT (datetime('now')),
		last_seen_at  DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	CREATE TABLE IF NOT EXISTS messages (
		id          TEXT PRIMARY KEY,
		from_agent  TEXT NOT NULL DEFAULT '',
		to_agent    TEXT NOT NULL DEFAULT '',
		grp         TEXT NOT NULL DEFAULT '',
		content     TEXT NOT NULL DEFAULT '',
		in_reply_to TEXT NOT NULL DEFAULT '',
		created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	CREATE TABLE IF NOT EXISTS message_reads (
		message_id  TEXT NOT NULL,
		agent_name  TEXT NOT NULL,
		read_at     DATETIME NOT NULL DEFAULT (datetime('now')),
		PRIMARY KEY (message_id, agent_name)
	);
	CREATE TABLE IF NOT EXISTS agent_groups (
		agent_name TEXT NOT NULL,
		group_name TEXT NOT NULL,
		PRIMARY KEY (agent_name, group_name)
	);
	CREATE INDEX IF NOT EXISTS idx_agent_groups_group ON agent_groups(group_name);
	`
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	// One-time migration: add last_seen_at column if missing (upgrade from older schema).
	var colCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('agents') WHERE name='last_seen_at'").Scan(&colCount); err != nil {
		return err
	}
	if colCount == 0 {
		// SQLite does not allow non-constant defaults in ALTER TABLE, so add without default
		// then backfill from registered_at.
		if _, err := db.Exec("ALTER TABLE agents ADD COLUMN last_seen_at DATETIME"); err != nil {
			return err
		}
		if _, err := db.Exec("UPDATE agents SET last_seen_at = registered_at WHERE last_seen_at IS NULL"); err != nil {
			return err
		}
	}

	// One-time migration: copy group data from legacy JSON column to agent_groups.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM agent_groups").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		_, _ = db.Exec(`
			INSERT OR IGNORE INTO agent_groups (agent_name, group_name)
			SELECT a.name, je.value FROM agents a, json_each(a.groups) je
			WHERE a.groups != '[]'
		`)
	}

	return nil
}

// RegisterAgent creates or updates an agent and its group memberships.
// Re-registering an existing agent updates its groups and refreshes registered_at.
func (s *Store) RegisterAgent(name string, groups []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	now := time.Now().UTC()
	groupsJSON, _ := json.Marshal(groups)
	_, err = tx.Exec(
		"INSERT OR REPLACE INTO agents (name, groups, status, registered_at, last_seen_at) VALUES (?, ?, 'idle', ?, ?)",
		name, string(groupsJSON), now, now,
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("insert agent: %w", err)
	}

	if err := syncAgentGroups(tx, name, groups); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// syncAgentGroups replaces the group memberships for an agent within a transaction.
func syncAgentGroups(tx *sql.Tx, agentName string, groups []string) error {
	_, err := tx.Exec("DELETE FROM agent_groups WHERE agent_name = ?", agentName)
	if err != nil {
		return fmt.Errorf("delete old groups: %w", err)
	}
	for _, g := range groups {
		_, err := tx.Exec(
			"INSERT INTO agent_groups (agent_name, group_name) VALUES (?, ?)",
			agentName, g,
		)
		if err != nil {
			return fmt.Errorf("insert group: %w", err)
		}
	}
	return nil
}

// GetAgent retrieves an agent by name.
func (s *Store) GetAgent(name string) (*protocol.Agent, error) {
	row := s.db.QueryRow(`
		SELECT a.name, a.status, a.registered_at, GROUP_CONCAT(ag.group_name), a.last_seen_at
		FROM agents a
		LEFT JOIN agent_groups ag ON a.name = ag.agent_name
		WHERE a.name = ?
		GROUP BY a.name
	`, name)
	return scanAgentWithGroups(row)
}

// scanAgentWithGroups scans a row that includes a GROUP_CONCAT groups column.
func scanAgentWithGroups(row *sql.Row) (*protocol.Agent, error) {
	var a protocol.Agent
	var groupsStr sql.NullString
	err := row.Scan(&a.Name, &a.Status, &a.RegisteredAt, &groupsStr, &a.LastSeenAt)
	if err != nil {
		return nil, fmt.Errorf("scan agent: %w", err)
	}
	a.Groups = splitGroups(groupsStr)
	return &a, nil
}

// splitGroups splits a GROUP_CONCAT result into a slice.
// Returns nil (not []) for empty/null input.
func splitGroups(groupsStr sql.NullString) []string {
	if !groupsStr.Valid || groupsStr.String == "" {
		return nil
	}
	return strings.Split(groupsStr.String, ",")
}

// SetAgentStatus updates the status field of an agent.
func (s *Store) SetAgentStatus(name, status string) error {
	res, err := s.db.Exec("UPDATE agents SET status = ? WHERE name = ?", status, name)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("agent %q not found", name)
	}
	return nil
}

// ListAgents returns all registered agents.
func (s *Store) ListAgents() ([]*protocol.Agent, error) {
	rows, err := s.db.Query(`
		SELECT a.name, a.status, a.registered_at, GROUP_CONCAT(ag.group_name), a.last_seen_at
		FROM agents a
		LEFT JOIN agent_groups ag ON a.name = ag.agent_name
		GROUP BY a.name
	`)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var agents []*protocol.Agent
	for rows.Next() {
		var a protocol.Agent
		var groupsStr sql.NullString
		if err := rows.Scan(&a.Name, &a.Status, &a.RegisteredAt, &groupsStr, &a.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		a.Groups = splitGroups(groupsStr)
		agents = append(agents, &a)
	}
	return agents, rows.Err()
}

// randomHex generates n random bytes and returns them as a hex string.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// SaveMessage persists a new message and returns its generated ID.
func (s *Store) SaveMessage(from, to, group, content, inReplyTo string) (string, error) {
	id := fmt.Sprintf("msg-%d-%s", time.Now().UnixMilli(), randomHex(3))
	_, err := s.db.Exec(
		"INSERT INTO messages (id, from_agent, to_agent, grp, content, in_reply_to, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, from, to, group, content, inReplyTo, time.Now().UTC(),
	)
	if err != nil {
		return "", fmt.Errorf("insert message: %w", err)
	}
	return id, nil
}

// GetMessage retrieves a single message by ID.
func (s *Store) GetMessage(id string) (*protocol.Message, error) {
	row := s.db.QueryRow(
		"SELECT id, from_agent, to_agent, grp, content, in_reply_to, created_at FROM messages WHERE id = ?",
		id,
	)
	var m protocol.Message
	err := row.Scan(&m.ID, &m.FromAgent, &m.ToAgent, &m.Group, &m.Content, &m.InReplyTo, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	return &m, nil
}

// GetUnreadMessages returns unread messages for an agent:
//   - direct messages where to_agent matches
//   - group messages where the agent belongs to the group
//
// Excludes the agent's own messages and messages already marked as read.
func (s *Store) GetUnreadMessages(agent string, limit int) ([]*protocol.Message, error) {
	rows, err := s.db.Query(`
		SELECT m.id, m.from_agent, m.to_agent, m.grp, m.content, m.in_reply_to, m.created_at
		FROM messages m
		WHERE m.from_agent != ?
		  AND (
		    m.to_agent = ?
		    OR (
		      m.grp != ''
		      AND EXISTS (
		        SELECT 1 FROM agent_groups ag
		        WHERE ag.agent_name = ? AND ag.group_name = m.grp
		      )
		    )
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM message_reads mr
		    WHERE mr.message_id = m.id AND mr.agent_name = ?
		  )
		ORDER BY m.created_at DESC
		LIMIT ?
	`, agent, agent, agent, agent, limit)
	if err != nil {
		return nil, fmt.Errorf("query unread: %w", err)
	}
	return scanMessages(rows)
}

// MarkRead marks the given message IDs as read by the specified agent.
// It returns the subset of message IDs that were newly marked (not previously read).
func (s *Store) MarkRead(agent string, messageIDs []string) ([]string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	now := time.Now().UTC()
	var newlyRead []string
	for _, id := range messageIDs {
		// Check if already read.
		var count int
		if err := tx.QueryRow(
			"SELECT COUNT(*) FROM message_reads WHERE message_id = ? AND agent_name = ?",
			id, agent,
		).Scan(&count); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("check read status: %w", err)
		}
		if count > 0 {
			continue
		}
		_, err := tx.Exec(
			"INSERT INTO message_reads (message_id, agent_name, read_at) VALUES (?, ?, ?)",
			id, agent, now,
		)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("mark read: %w", err)
		}
		newlyRead = append(newlyRead, id)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return newlyRead, nil
}

// GetGroupMembers returns the names of all agents belonging to a group.
func (s *Store) GetGroupMembers(group string) ([]string, error) {
	rows, err := s.db.Query(
		"SELECT agent_name FROM agent_groups WHERE group_name = ?",
		group,
	)
	if err != nil {
		return nil, fmt.Errorf("query group members: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// ListGroups returns all distinct group names across all agents.
func (s *Store) ListGroups() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT group_name FROM agent_groups")
	if err != nil {
		return nil, fmt.Errorf("query groups: %w", err)
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// GetRecentMessages returns the most recent messages ordered by created_at DESC.
func (s *Store) GetRecentMessages(limit int) ([]*protocol.Message, error) {
	rows, err := s.db.Query(
		"SELECT id, from_agent, to_agent, grp, content, in_reply_to, created_at FROM messages ORDER BY created_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query recent: %w", err)
	}
	return scanMessages(rows)
}

// TouchAgent updates last_seen_at for an agent to the current time.
func (s *Store) TouchAgent(name string) error {
	_, err := s.db.Exec(
		"UPDATE agents SET last_seen_at = ? WHERE name = ?",
		time.Now().UTC(), name,
	)
	return err
}

// ExpireStaleAgents sets agents to "offline" if their last_seen_at is older than ttl.
// Returns the names of agents that were expired.
func (s *Store) ExpireStaleAgents(ttl time.Duration) ([]string, error) {
	cutoff := time.Now().UTC().Add(-ttl)
	rows, err := s.db.Query(
		"SELECT name FROM agents WHERE status != 'offline' AND last_seen_at < ?",
		cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("query stale agents: %w", err)
	}
	defer rows.Close()

	var expired []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan stale agent: %w", err)
		}
		expired = append(expired, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, name := range expired {
		if err := s.SetAgentStatus(name, "offline"); err != nil {
			// Agent may have been deleted between query and update.
			continue
		}
	}
	return expired, nil
}

// DeregisterAgent removes an agent and its group memberships.
func (s *Store) DeregisterAgent(name string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	_, err = tx.Exec("DELETE FROM agent_groups WHERE agent_name = ?", name)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("delete agent groups: %w", err)
	}
	res, err := tx.Exec("DELETE FROM agents WHERE name = ?", name)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("delete agent: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		tx.Rollback()
		return fmt.Errorf("agent %q not found", name)
	}
	return tx.Commit()
}

// DeleteOldMessages removes messages older than retentionDays and their
// associated read records. Returns the number of messages deleted.
func (s *Store) DeleteOldMessages(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)

	_, err := s.db.ExecContext(ctx,
		"DELETE FROM message_reads WHERE message_id IN (SELECT id FROM messages WHERE created_at < ?)",
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("delete old message reads: %w", err)
	}

	res, err := s.db.ExecContext(ctx,
		"DELETE FROM messages WHERE created_at < ?",
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("delete old messages: %w", err)
	}

	n, _ := res.RowsAffected()
	return n, nil
}

// scanMessages scans a set of rows into protocol.Message slices.
func scanMessages(rows *sql.Rows) ([]*protocol.Message, error) {
	defer rows.Close()

	var msgs []*protocol.Message
	for rows.Next() {
		var m protocol.Message
		if err := rows.Scan(&m.ID, &m.FromAgent, &m.ToAgent, &m.Group, &m.Content, &m.InReplyTo, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, &m)
	}
	return msgs, rows.Err()
}

// MessageQuery holds parameters for filtering message history.
type MessageQuery struct {
	Agent string // required: match from_agent, to_agent, or group membership
	With  string // optional: filter to direct messages between agent and this peer
	Group string // optional: filter to a specific group
	Since string // optional RFC3339: only messages after this time
	Until string // optional RFC3339: only messages before this time
	Limit int    // max results (default 50)
}

// sqliteDatetimeFormat matches the datetime format used by SQLite's datetime() function.
const sqliteDatetimeFormat = "2006-01-02 15:04:05"

// DefaultMessageQueryLimit is the default number of results returned by QueryMessages.
const DefaultMessageQueryLimit = 50

// QueryMessages returns messages matching the given filter criteria.
func (s *Store) QueryMessages(q MessageQuery) ([]*protocol.Message, error) {
	if q.Limit <= 0 {
		q.Limit = DefaultMessageQueryLimit
	}

	// Build WHERE clauses dynamically.
	where := []string{}
	args := []interface{}{}

	// Agent filter: messages where the agent is sender, recipient, or in the group.
	agentClause := `(m.from_agent = ? OR m.to_agent = ? OR (m.grp != '' AND EXISTS (SELECT 1 FROM agent_groups ag WHERE ag.agent_name = ? AND ag.group_name = m.grp)))`
	where = append(where, agentClause)
	args = append(args, q.Agent, q.Agent, q.Agent)

	// With filter: direct messages between agent and a specific peer.
	if q.With != "" {
		where = append(where, `((m.from_agent = ? AND m.to_agent = ?) OR (m.from_agent = ? AND m.to_agent = ?))`)
		args = append(args, q.Agent, q.With, q.With, q.Agent)
	}

	// Group filter: messages in a specific group.
	if q.Group != "" {
		where = append(where, `m.grp = ?`)
		args = append(args, q.Group)
	}

	// Since filter: messages after this time.
	if q.Since != "" {
		t, err := time.Parse(time.RFC3339, q.Since)
		if err == nil {
			where = append(where, `m.created_at > ?`)
			args = append(args, t.UTC().Format(sqliteDatetimeFormat))
		}
	}

	// Until filter: messages before this time.
	if q.Until != "" {
		t, err := time.Parse(time.RFC3339, q.Until)
		if err == nil {
			where = append(where, `m.created_at < ?`)
			args = append(args, t.UTC().Format(sqliteDatetimeFormat))
		}
	}

	query := `SELECT m.id, m.from_agent, m.to_agent, m.grp, m.content, m.in_reply_to, m.created_at FROM messages m`
	if len(where) > 0 {
		query += ` WHERE ` + joinStrings(where, " AND ")
	}
	query += ` ORDER BY m.created_at DESC LIMIT ?`
	args = append(args, q.Limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	return scanMessages(rows)
}

// joinStrings joins a slice of strings with a separator.
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}
