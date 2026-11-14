// Package store is Querra's local database: connection groups, saved
// connections, and settings (PLAN §3.6). It is a SQLite file in the user's
// config directory, opened with the pure-Go modernc.org/sqlite driver and
// migrated on open.
//
// Secrets never go here. Passwords and passphrases live in the OS keychain
// (internal/secrets); this database only holds what is safe to read in plain
// text.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// ErrNotFound is returned when the requested record does not exist.
var ErrNotFound = errors.New("not found")

// Store is an open local database. Its repositories share one connection.
type Store struct {
	db  *sql.DB
	now func() time.Time

	Groups      *Groups
	Connections *Connections
	Settings    *Settings
}

// DefaultPath returns where the app keeps its database: <config dir>/Querra/
// querra.db, i.e. ~/Library/Application Support/Querra on macOS and
// %AppData%\Querra on Windows. (WebView2 keeps its own data in
// %AppData%\querra.exe, so the two never mix.)
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate config dir: %w", err)
	}
	return filepath.Join(dir, "Querra", "querra.db"), nil
}

// Open opens the database at path, creating it and its directory if needed,
// and migrates it to the current schema.
func Open(ctx context.Context, path string) (*Store, error) {
	return open(ctx, path, migrationFiles())
}

func open(ctx context.Context, path string, files fs.FS) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// The app is the database's only user and SQLite serializes writes anyway;
	// one connection avoids SQLITE_BUSY between pooled connections.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	if err := migrate(ctx, db, files); err != nil {
		_ = db.Close()
		return nil, err
	}
	return newStore(db, time.Now), nil
}

func newStore(db *sql.DB, now func() time.Time) *Store {
	s := &Store{db: db, now: now}
	s.Groups = &Groups{s: s}
	s.Connections = &Connections{s: s}
	s.Settings = &Settings{s: s}
	return s
}

// dsn builds the modernc.org/sqlite data source name. The path stays a plain
// file name (not a file: URI), so spaces in it, as in many Windows user
// folders, need no escaping. _pragma parameters run on every new connection.
func dsn(path string) string {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	return path + "?" + q.Encode()
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Times are stored as fixed-width UTC text so they sort correctly as strings.
const timeLayout = "2006-01-02T15:04:05.000Z"

func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored time %q: %w", s, err)
	}
	return t, nil
}
