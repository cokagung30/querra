package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
)

// ErrNewerSchema means the database was migrated by a newer Querra than this
// one. Opening it anyway could corrupt data the newer schema relies on.
var ErrNewerSchema = errors.New("database was created by a newer version of Querra")

// Migrations are numbered SQL files, applied in order and recorded in
// PRAGMA user_version. Never edit a migration that has shipped; add a new one.
//
//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// migrationFiles returns the embedded migrations directory.
func migrationFiles() fs.FS {
	sub, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		panic(err) // the embed pattern guarantees the directory exists
	}
	return sub
}

type migration struct {
	version int
	name    string
	sql     string
}

var migrationName = regexp.MustCompile(`^(\d{4})_([a-z0-9_]+)\.sql$`)

// loadMigrations reads NNNN_name.sql files and checks that their versions
// run 1, 2, 3, … without gaps or duplicates.
func loadMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var migs []migration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := migrationName.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migration %q: name must look like 0001_description.sql", e.Name())
		}
		version, _ := strconv.Atoi(m[1])
		body, err := fs.ReadFile(fsys, path.Clean(e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", e.Name(), err)
		}
		migs = append(migs, migration{version: version, name: m[2], sql: string(body)})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })
	for i, m := range migs {
		if m.version != i+1 {
			return nil, fmt.Errorf("migration versions must run 1, 2, 3, …: found %04d at position %d", m.version, i+1)
		}
	}
	return migs, nil
}

// migrate applies the migrations newer than the database's user_version, each
// in its own transaction together with the version bump.
func migrate(ctx context.Context, db *sql.DB, files fs.FS) error {
	migs, err := loadMigrations(files)
	if err != nil {
		return err
	}
	var current int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > len(migs) {
		return fmt.Errorf("%w (schema version %d, this build knows %d)", ErrNewerSchema, current, len(migs))
	}
	for _, m := range migs[current:] {
		if err := applyMigration(ctx, db, m); err != nil {
			return fmt.Errorf("migration %04d_%s: %w", m.version, m.name, err)
		}
	}
	return nil
}

// applyMigration applies m unless another Querra process did so first: the
// transaction holds the write lock (_txlock=immediate in dsn), and the version
// is checked again under it.
func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // no-op after Commit

	var current int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return err
	}
	if current >= m.version {
		return nil
	}
	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return err
	}
	// PRAGMA takes no bind parameters; the version is our own integer, never
	// user input.
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(m.version)); err != nil {
		return err
	}
	return tx.Commit()
}
