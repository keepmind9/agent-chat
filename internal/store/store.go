// Package store provides a SQLite-backed persistence layer for agent-chat.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

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
		registered_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	CREATE TABLE IF NOT EXISTS messages (
		id          TEXT PRIMARY KEY,
		from_agent  TEXT NOT NULL DEFAULT '',
		to_agent    TEXT NOT NULL DEFAULT '',
		grp         TEXT NOT NULL DEFAULT '',
		content     TEXT NOT NULL DEFAULT '',
		created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	CREATE TABLE IF NOT EXISTS message_reads (
		message_id  TEXT NOT NULL,
		agent_name  TEXT NOT NULL,
		read_at     DATETIME NOT NULL DEFAULT (datetime('now')),
		PRIMARY KEY (message_id, agent_name)
	);
	`
	_, err := db.Exec(schema)
	return err
}

// RegisterAgent inserts a new agent with its groups encoded as JSON.
func (s *Store) RegisterAgent(name string, groups []string) error {
	groupsJSON, err := json.Marshal(groups)
	if err != nil {
		return fmt.Errorf("marshal groups: %w", err)
	}
	_, err = s.db.Exec(
		"INSERT INTO agents (name, groups, status, registered_at) VALUES (?, ?, 'online', ?)",
		name, string(groupsJSON), time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert agent: %w", err)
	}
	return nil
}

// GetAgent retrieves an agent by name.
func (s *Store) GetAgent(name string) (*protocol.Agent, error) {
	row := s.db.QueryRow(
		"SELECT name, groups, status, registered_at FROM agents WHERE name = ?",
		name,
	)
	return scanAgent(row)
}

func scanAgent(row *sql.Row) (*protocol.Agent, error) {
	var a protocol.Agent
	var groupsStr string
	err := row.Scan(&a.Name, &groupsStr, &a.Status, &a.RegisteredAt)
	if err != nil {
		return nil, fmt.Errorf("scan agent: %w", err)
	}
	if err := json.Unmarshal([]byte(groupsStr), &a.Groups); err != nil {
		return nil, fmt.Errorf("unmarshal groups: %w", err)
	}
	return &a, nil
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
	rows, err := s.db.Query("SELECT name, groups, status, registered_at FROM agents")
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var agents []*protocol.Agent
	for rows.Next() {
		var a protocol.Agent
		var groupsStr string
		if err := rows.Scan(&a.Name, &groupsStr, &a.Status, &a.RegisteredAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		if err := json.Unmarshal([]byte(groupsStr), &a.Groups); err != nil {
			return nil, fmt.Errorf("unmarshal groups: %w", err)
		}
		agents = append(agents, &a)
	}
	return agents, rows.Err()
}

// SaveMessage persists a new message and returns its generated ID.
func (s *Store) SaveMessage(from, to, group, content string) (string, error) {
	id := fmt.Sprintf("msg-%d", time.Now().UnixNano())
	_, err := s.db.Exec(
		"INSERT INTO messages (id, from_agent, to_agent, grp, content, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, from, to, group, content, time.Now().UTC(),
	)
	if err != nil {
		return "", fmt.Errorf("insert message: %w", err)
	}
	return id, nil
}

// GetMessage retrieves a single message by ID.
func (s *Store) GetMessage(id string) (*protocol.Message, error) {
	row := s.db.QueryRow(
		"SELECT id, from_agent, to_agent, grp, content, created_at FROM messages WHERE id = ?",
		id,
	)
	var m protocol.Message
	err := row.Scan(&m.ID, &m.FromAgent, &m.ToAgent, &m.Group, &m.Content, &m.CreatedAt)
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
		SELECT m.id, m.from_agent, m.to_agent, m.grp, m.content, m.created_at
		FROM messages m
		WHERE m.from_agent != ?
		  AND (
		    m.to_agent = ?
		    OR EXISTS (
		      SELECT 1 FROM agents a
		      WHERE a.name = ?
		        AND grp != ''
		        AND EXISTS (
		          SELECT 1 FROM json_each(a.groups) je WHERE je.value = m.grp
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
func (s *Store) MarkRead(agent string, messageIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	now := time.Now().UTC()
	for _, id := range messageIDs {
		_, err := tx.Exec(
			"INSERT OR IGNORE INTO message_reads (message_id, agent_name, read_at) VALUES (?, ?, ?)",
			id, agent, now,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("mark read: %w", err)
		}
	}
	return tx.Commit()
}

// GetGroupMembers returns the names of all agents belonging to a group.
func (s *Store) GetGroupMembers(group string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT a.name FROM agents a, json_each(a.groups) je
		WHERE je.value = ?
	`, group)
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
	rows, err := s.db.Query("SELECT DISTINCT je.value FROM agents a, json_each(a.groups) je")
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
		"SELECT id, from_agent, to_agent, grp, content, created_at FROM messages ORDER BY created_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query recent: %w", err)
	}
	return scanMessages(rows)
}

// scanMessages scans a set of rows into protocol.Message slices.
func scanMessages(rows *sql.Rows) ([]*protocol.Message, error) {
	defer rows.Close()

	var msgs []*protocol.Message
	for rows.Next() {
		var m protocol.Message
		if err := rows.Scan(&m.ID, &m.FromAgent, &m.ToAgent, &m.Group, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, &m)
	}
	return msgs, rows.Err()
}
