package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Connection is a saved connection as stored. It never contains secrets:
// passwords and passphrases live in the OS keychain, and an SSH private key is
// referenced by its file path. Values are validated by the connection
// package (M1.3) before they get here.
type Connection struct {
	ID        string
	GroupID   string // "" means no group
	Name      string
	Engine    string // e.g. "postgres", "mysql"
	Env       string // e.g. "local", "staging", "production"
	Color     string // "" means the environment's default color
	Host      string
	Port      int
	Username  string
	Database  string
	SSL       json.RawMessage // TLS settings; empty means {}
	SSH       json.RawMessage // SSH tunnel settings; empty means {}
	Options   json.RawMessage // engine-specific options; empty means {}
	ReadOnly  bool
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Connections stores saved connections.
type Connections struct{ s *Store }

const connectionColumns = `id, group_id, name, engine, env, color, host, port, username,
	database_name, ssl_json, ssh_json, options_json, read_only, sort_order, created_at, updated_at`

// List returns all connections by sort order, then name.
func (r *Connections) List(ctx context.Context) ([]Connection, error) {
	rows, err := r.s.db.QueryContext(ctx,
		`SELECT `+connectionColumns+` FROM connections ORDER BY sort_order, name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer func() { _ = rows.Close() }()

	conns := []Connection{}
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, fmt.Errorf("list connections: %w", err)
		}
		conns = append(conns, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	return conns, nil
}

// Get returns the connection with the given id, or ErrNotFound.
func (r *Connections) Get(ctx context.Context, id string) (Connection, error) {
	row := r.s.db.QueryRowContext(ctx, `SELECT `+connectionColumns+` FROM connections WHERE id = ?`, id)
	c, err := scanConnection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Connection{}, fmt.Errorf("get connection %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return Connection{}, fmt.Errorf("get connection %s: %w", id, err)
	}
	return c, nil
}

// Create inserts c, assigning an ID when it has none and setting both
// timestamps, and returns the stored connection.
func (r *Connections) Create(ctx context.Context, c Connection) (Connection, error) {
	if c.ID == "" {
		c.ID = newID()
	}
	now := r.s.now().UTC().Truncate(time.Millisecond)
	c.CreatedAt, c.UpdatedAt = now, now

	ssl, ssh, opts, err := connectionJSON(c)
	if err != nil {
		return Connection{}, fmt.Errorf("create connection: %w", err)
	}
	_, err = r.s.db.ExecContext(ctx,
		`INSERT INTO connections (`+connectionColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, nullIfEmpty(c.GroupID), c.Name, c.Engine, c.Env, c.Color, c.Host, c.Port, c.Username,
		c.Database, ssl, ssh, opts, c.ReadOnly, c.SortOrder, formatTime(c.CreatedAt), formatTime(c.UpdatedAt))
	if err != nil {
		return Connection{}, fmt.Errorf("create connection: %w", err)
	}
	c.SSL, c.SSH, c.Options = json.RawMessage(ssl), json.RawMessage(ssh), json.RawMessage(opts)
	return c, nil
}

// Update saves every field of c except CreatedAt, sets UpdatedAt, and returns
// the stored connection.
func (r *Connections) Update(ctx context.Context, c Connection) (Connection, error) {
	ssl, ssh, opts, err := connectionJSON(c)
	if err != nil {
		return Connection{}, fmt.Errorf("update connection %s: %w", c.ID, err)
	}
	c.UpdatedAt = r.s.now().UTC().Truncate(time.Millisecond)

	res, err := r.s.db.ExecContext(ctx,
		`UPDATE connections SET group_id = ?, name = ?, engine = ?, env = ?, color = ?, host = ?,
			port = ?, username = ?, database_name = ?, ssl_json = ?, ssh_json = ?, options_json = ?,
			read_only = ?, sort_order = ?, updated_at = ?
		WHERE id = ?`,
		nullIfEmpty(c.GroupID), c.Name, c.Engine, c.Env, c.Color, c.Host, c.Port, c.Username,
		c.Database, ssl, ssh, opts, c.ReadOnly, c.SortOrder, formatTime(c.UpdatedAt), c.ID)
	if err != nil {
		return Connection{}, fmt.Errorf("update connection %s: %w", c.ID, err)
	}
	if err := expectOneRow(res, "update connection "+c.ID); err != nil {
		return Connection{}, err
	}
	return r.Get(ctx, c.ID)
}

// Delete removes a connection. Its keychain entries are the caller's to
// remove (internal/secrets).
func (r *Connections) Delete(ctx context.Context, id string) error {
	res, err := r.s.db.ExecContext(ctx, `DELETE FROM connections WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete connection %s: %w", id, err)
	}
	return expectOneRow(res, "delete connection "+id)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanConnection(row scanner) (Connection, error) {
	var (
		c                Connection
		groupID          sql.NullString
		ssl, ssh, opts   string
		created, updated string
	)
	err := row.Scan(&c.ID, &groupID, &c.Name, &c.Engine, &c.Env, &c.Color, &c.Host, &c.Port,
		&c.Username, &c.Database, &ssl, &ssh, &opts, &c.ReadOnly, &c.SortOrder, &created, &updated)
	if err != nil {
		return Connection{}, err
	}
	c.GroupID = groupID.String
	c.SSL, c.SSH, c.Options = json.RawMessage(ssl), json.RawMessage(ssh), json.RawMessage(opts)
	if c.CreatedAt, err = parseTime(created); err != nil {
		return Connection{}, err
	}
	if c.UpdatedAt, err = parseTime(updated); err != nil {
		return Connection{}, err
	}
	return c, nil
}

// connectionJSON returns c's JSON columns, using {} for empty ones and
// rejecting invalid JSON.
func connectionJSON(c Connection) (ssl, ssh, opts string, err error) {
	fields := []struct {
		name string
		raw  json.RawMessage
		out  *string
	}{{"ssl", c.SSL, &ssl}, {"ssh", c.SSH, &ssh}, {"options", c.Options, &opts}}
	for _, f := range fields {
		if len(f.raw) == 0 {
			*f.out = "{}"
			continue
		}
		if !json.Valid(f.raw) {
			return "", "", "", fmt.Errorf("%s settings are not valid JSON", f.name)
		}
		*f.out = string(f.raw)
	}
	return ssl, ssh, opts, nil
}

func nullIfEmpty(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func expectOneRow(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if n == 0 {
		return fmt.Errorf("%s: %w", op, ErrNotFound)
	}
	return nil
}
