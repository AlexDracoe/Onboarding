// Package store keeps the command log in SQLite. Same schema as the Python service, so an
// existing drone.db works with either backend.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // registers the "sqlite" driver (pure Go, works with CGO_ENABLED=0)
)

const schema = `
CREATE TABLE IF NOT EXISTS commands (
    id         TEXT PRIMARY KEY,      -- uuid4 string
    created_at REAL NOT NULL,         -- unix seconds (wall clock)
    updated_at REAL NOT NULL,         -- unix seconds, last status change
    system_id  INTEGER NOT NULL,
    cmd        TEXT NOT NULL,         -- JSON object: {"name": ..., "params": {...}}
    status     TEXT NOT NULL,
    error      TEXT                   -- NULL unless the command failed
);
CREATE INDEX IF NOT EXISTS idx_commands_system_created
    ON commands (system_id, created_at DESC);
`

// ErrUnknownID means no row has that command id.
var ErrUnknownID = errors.New("unknown command id")

// Record is one row of the command log, shaped like the HTTP contract.
type Record struct {
	ID        string          `json:"id"`
	SystemID  uint8           `json:"system_id"`
	Cmd       json.RawMessage `json:"cmd"`
	Status    string          `json:"status"`
	Error     *string         `json:"error"`
	CreatedAt float64         `json:"created_at"`
	UpdatedAt float64         `json:"updated_at"`
}

// SQLite is the command log. It implements app.CommandLog. Safe for concurrent use.
type SQLite struct {
	db *sql.DB
}

// Open opens (or creates) the database at path and makes sure the table exists.
func Open(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	// One connection, like the Python service's single connection + lock: SQLite allows one
	// writer anyway, and ":memory:" databases are per connection.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema in %s: %w", path, err)
	}
	return &SQLite{db: db}, nil
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

// storedCmd is the cmd column: {"name": ..., "params": {...}} in that key order.
type storedCmd struct {
	Name   string `json:"name"`
	Params any    `json:"params"`
}

// Create inserts a PENDING row and returns its new id. params is the decoded params struct; nil stores {}.
func (s *SQLite) Create(systemID uint8, name string, params any) (string, error) {
	if params == nil {
		params = struct{}{}
	}
	cmd, err := json.Marshal(storedCmd{Name: name, Params: params})
	if err != nil {
		return "", fmt.Errorf("encode params for %s: %w", name, err)
	}
	id := uuid.NewString()
	now := unixSeconds(time.Now())
	_, err = s.db.Exec(
		"INSERT INTO commands (id, created_at, updated_at, system_id, cmd, status) VALUES (?, ?, ?, ?, ?, ?)",
		id, now, now, systemID, string(cmd), "PENDING",
	)
	if err != nil {
		return "", fmt.Errorf("insert command: %w", err)
	}
	return id, nil
}

// UpdateStatus sets the final status. A nil cmdErr stores NULL in the error column.
func (s *SQLite) UpdateStatus(id, status string, cmdErr error) error {
	var errText *string
	if cmdErr != nil {
		errText = new(cmdErr.Error())
	}
	res, err := s.db.Exec(
		"UPDATE commands SET status = ?, error = ?, updated_at = ? WHERE id = ?",
		status, errText, unixSeconds(time.Now()), id,
	)
	if err != nil {
		return fmt.Errorf("update command %s: %w", id, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("update command %s: %w", id, ErrUnknownID)
	}
	return nil
}

const selectColumns = "SELECT id, system_id, cmd, status, error, created_at, updated_at FROM commands"

// Get returns one record; ok is false if the id is unknown.
func (s *SQLite) Get(id string) (rec Record, ok bool, err error) {
	rec, err = scanRecord(s.db.QueryRow(selectColumns+" WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("get command %s: %w", id, err)
	}
	return rec, true, nil
}

// List returns up to limit records, newest first. A nil systemID means all vehicles.
func (s *SQLite) List(systemID *uint8, limit int) ([]Record, error) {
	query, args := selectColumns, []any{}
	if systemID != nil {
		query += " WHERE system_id = ?"
		args = append(args, *systemID)
	}
	query += " ORDER BY created_at DESC, rowid DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list commands: %w", err)
	}
	defer rows.Close()

	out := []Record{} // JSON [] rather than null when empty
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("list commands: %w", err)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// scanner is what *sql.Row and *sql.Rows have in common.
type scanner interface {
	Scan(dest ...any) error
}

func scanRecord(row scanner) (Record, error) {
	var rec Record
	var cmd string
	var errText sql.NullString
	if err := row.Scan(&rec.ID, &rec.SystemID, &cmd, &rec.Status, &errText, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return Record{}, err
	}
	rec.Cmd = json.RawMessage(cmd)
	if errText.Valid {
		rec.Error = &errText.String
	}
	return rec, nil
}

func unixSeconds(t time.Time) float64 {
	return float64(t.UnixMicro()) / 1e6
}
