package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// Settings stores app settings as JSON values under string keys, e.g. the
// keybinding overrides (M4.5) or the default row limit.
type Settings struct{ s *Store }

// Get decodes the value stored under key into dst. It returns ErrNotFound
// when the key has never been set.
func (r *Settings) Get(ctx context.Context, key string, dst any) error {
	var raw string
	err := r.s.db.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key = ?`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("get setting %q: %w", key, ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("get setting %q: %w", key, err)
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		return fmt.Errorf("decode setting %q: %w", key, err)
	}
	return nil
}

// Set stores value, encoded as JSON, under key.
func (r *Settings) Set(ctx context.Context, key string, value any) error {
	if key == "" {
		return errors.New("set setting: empty key")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode setting %q: %w", key, err)
	}
	_, err = r.s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value_json) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET value_json = excluded.value_json`,
		key, string(raw))
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// Delete removes key, so Get falls back to ErrNotFound (the default). Deleting
// a key that is not set is not an error.
func (r *Settings) Delete(ctx context.Context, key string) error {
	if _, err := r.s.db.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key); err != nil {
		return fmt.Errorf("delete setting %q: %w", key, err)
	}
	return nil
}
