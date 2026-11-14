package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

var fixedNow = time.Date(2026, 11, 8, 9, 30, 15, 123_000_000, time.UTC)

// openTest opens a fresh database in a temporary directory whose path contains
// a space, as many Windows user folders do, with a fixed clock.
func openTest(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "with space", "querra.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	s.now = func() time.Time { return fixedNow }
	return s
}

func userVersion(t *testing.T, s *Store) int {
	t.Helper()
	var v int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestOpenMigratesAndReopens(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "dir", "querra.db")

	s, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	migs, err := loadMigrations(migrationFiles())
	if err != nil {
		t.Fatal(err)
	}
	if got := userVersion(t, s); got != len(migs) {
		t.Fatalf("user_version = %d, want %d", got, len(migs))
	}
	if _, err := s.Groups.Create(ctx, Group{Name: "Work"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopening an up-to-date database applies nothing and keeps the data.
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	defer func() { _ = s.Close() }()
	groups, err := s.Groups.List(ctx)
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups after reopen = %v, %v; want the one created before", groups, err)
	}
}

// Two Querra windows started at once on first run both migrate the same new
// database; separate *sql.DB handles stand in for the two processes.
func TestConcurrentOpensOfANewDatabase(t *testing.T) {
	const opens = 4
	for round := range 5 {
		path := filepath.Join(t.TempDir(), "querra.db")
		start := make(chan struct{})
		errs := make(chan error, opens)
		var wg sync.WaitGroup
		for range opens {
			wg.Go(func() {
				<-start
				s, err := Open(context.Background(), path)
				if err == nil {
					err = s.Close()
				}
				errs <- err
			})
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("round %d: concurrent Open() error = %v", round, err)
			}
		}
	}
}

func TestOpenSetsPragmas(t *testing.T) {
	s := openTest(t)
	var fk int
	var mode string
	if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys = %d, %v; want 1", fk, err)
	}
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q, %v; want wal", mode, err)
	}
}

func TestOpenRejectsNewerSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "querra.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	if _, err := Open(ctx, path); !errors.Is(err, ErrNewerSchema) {
		t.Fatalf("Open() error = %v, want ErrNewerSchema", err)
	}
}

func TestFailedMigrationRollsBack(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "querra.db")
	files := fstest.MapFS{
		"0001_ok.sql":     {Data: []byte("CREATE TABLE a (x INTEGER);")},
		"0002_broken.sql": {Data: []byte("CREATE TABLE b (y INTEGER); THIS IS NOT SQL;")},
	}

	if _, err := open(ctx, path, files); err == nil || !strings.Contains(err.Error(), "0002_broken") {
		t.Fatalf("open() error = %v, want a 0002_broken failure", err)
	}

	// Migration 1 stays applied; migration 2 left nothing behind.
	s, err := open(ctx, path, fstest.MapFS{"0001_ok.sql": files["0001_ok.sql"]})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if got := userVersion(t, s); got != 1 {
		t.Fatalf("user_version = %d, want 1", got)
	}
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name = 'b'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("table b exists (%d, %v); the failed migration was not rolled back", n, err)
	}
}

func TestLoadMigrationsValidatesNames(t *testing.T) {
	sql := &fstest.MapFile{Data: []byte("SELECT 1;")}
	tests := []struct {
		name  string
		files fstest.MapFS
		ok    bool
	}{
		{"contiguous", fstest.MapFS{"0001_a.sql": sql, "0002_b.sql": sql}, true},
		{"gap", fstest.MapFS{"0001_a.sql": sql, "0003_c.sql": sql}, false},
		{"duplicate version", fstest.MapFS{"0001_a.sql": sql, "0001_b.sql": sql}, false},
		{"not starting at 1", fstest.MapFS{"0002_b.sql": sql}, false},
		{"bad name", fstest.MapFS{"0001-a.sql": sql}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadMigrations(tt.files)
			if (err == nil) != tt.ok {
				t.Fatalf("loadMigrations() error = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestGroups(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)

	b, err := s.Groups.Create(ctx, Group{Name: "beta", SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.Groups.Create(ctx, Group{Name: "Alpha", SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Groups.Create(ctx, Group{Name: "zeta", SortOrder: 0})
	if err != nil {
		t.Fatal(err)
	}
	if b.ID == "" || b.ID == a.ID {
		t.Fatalf("Create() ids = %q, %q; want distinct generated ids", b.ID, a.ID)
	}

	groups, err := s.Groups.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(groups); got != "zeta,Alpha,beta" {
		t.Fatalf("List() order = %s, want zeta,Alpha,beta (sort order, then name ignoring case)", got)
	}

	first.Name = "Renamed"
	if err := s.Groups.Update(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := s.Groups.Update(ctx, Group{ID: "missing", Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update(missing) error = %v, want ErrNotFound", err)
	}
	if err := s.Groups.Delete(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(missing) error = %v, want ErrNotFound", err)
	}
}

func TestDeletingAGroupKeepsItsConnections(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	g, err := s.Groups.Create(ctx, Group{Name: "Clients"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Connections.Create(ctx, Connection{GroupID: g.ID, Name: "db", Engine: "postgres", Env: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Groups.Delete(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.Connections.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("connection gone after deleting its group: %v", err)
	}
	if got.GroupID != "" {
		t.Fatalf("GroupID = %q, want empty", got.GroupID)
	}
}

func TestConnectionRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	g, err := s.Groups.Create(ctx, Group{Name: "Clients"})
	if err != nil {
		t.Fatal(err)
	}

	want := Connection{
		GroupID:   g.ID,
		Name:      "Shop production",
		Engine:    "mysql",
		Env:       "production",
		Color:     "#d33",
		Host:      "db.example.com",
		Port:      3306,
		Username:  "app",
		Database:  "shop",
		SSL:       json.RawMessage(`{"mode":"verify-full"}`),
		SSH:       json.RawMessage(`{"host":"bastion","keyPath":"~/.ssh/id_ed25519"}`),
		Options:   json.RawMessage(`{"timezone":"+08:00"}`),
		ReadOnly:  true,
		SortOrder: 3,
	}
	created, err := s.Connections.Create(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(created.ID) {
		t.Fatalf("ID = %q, want a v4 UUID", created.ID)
	}
	if !created.CreatedAt.Equal(fixedNow) || !created.UpdatedAt.Equal(fixedNow) {
		t.Fatalf("timestamps = %v / %v, want %v", created.CreatedAt, created.UpdatedAt, fixedNow)
	}

	got, err := s.Connections.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	want.ID, want.CreatedAt, want.UpdatedAt = created.ID, fixedNow, fixedNow
	assertConnection(t, got, want)
}

func TestConnectionDefaultsAndValidation(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)

	c, err := s.Connections.Create(ctx, Connection{Name: "local", Engine: "postgres", Env: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if string(c.SSL) != "{}" || string(c.SSH) != "{}" || string(c.Options) != "{}" || c.GroupID != "" {
		t.Fatalf("defaults = ssl %s, ssh %s, options %s, group %q; want {} and no group", c.SSL, c.SSH, c.Options, c.GroupID)
	}

	c, err = s.Connections.Create(ctx, Connection{Name: "null", Engine: "postgres", Env: "local", SSH: json.RawMessage(` null `)})
	if err != nil || string(c.SSH) != "{}" {
		t.Fatalf("Create() with null SSH settings = %s, %v; want {}", c.SSH, err)
	}
	for _, bad := range []string{`{`, `[]`, `1`, `"x"`, `{} {}`} {
		_, err = s.Connections.Create(ctx, Connection{Name: "bad", Engine: "postgres", Env: "local", SSH: json.RawMessage(bad)})
		if err == nil || !strings.Contains(err.Error(), "ssh settings must be a JSON object") {
			t.Errorf("Create() with SSH settings %s: error = %v; want a JSON object error", bad, err)
		}
	}
	_, err = s.Connections.Create(ctx, Connection{GroupID: "no-such-group", Name: "x", Engine: "postgres", Env: "local"})
	if err == nil {
		t.Fatal("Create() with an unknown group succeeded; want a foreign key error")
	}
}

func TestConnectionUpdateListDelete(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)

	b, err := s.Connections.Create(ctx, Connection{Name: "beta", Engine: "postgres", Env: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Connections.Create(ctx, Connection{Name: "Alpha", Engine: "mysql", Env: "staging"}); err != nil {
		t.Fatal(err)
	}

	later := fixedNow.Add(time.Hour)
	s.now = func() time.Time { return later }
	b.Name, b.Port, b.ReadOnly = "Beta renamed", 5433, true
	updated, err := s.Connections.Update(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Beta renamed" || updated.Port != 5433 || !updated.ReadOnly {
		t.Fatalf("Update() = %+v, want the new values", updated)
	}
	if !updated.CreatedAt.Equal(fixedNow) || !updated.UpdatedAt.Equal(later) {
		t.Fatalf("timestamps = %v / %v, want created kept and updated set", updated.CreatedAt, updated.UpdatedAt)
	}

	list, err := s.Connections.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "Alpha" || list[1].Name != "Beta renamed" {
		t.Fatalf("List() = %v, want Alpha then Beta renamed", list)
	}

	if _, err := s.Connections.Update(ctx, Connection{ID: "missing", Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update(missing) error = %v, want ErrNotFound", err)
	}
	if err := s.Connections.Delete(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Connections.Get(ctx, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after Delete error = %v, want ErrNotFound", err)
	}
	if err := s.Connections.Delete(ctx, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Delete() error = %v, want ErrNotFound", err)
	}
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)

	var limit int
	if err := s.Settings.Get(ctx, "query.maxRows", &limit); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() of an unset key error = %v, want ErrNotFound", err)
	}

	type keybindings map[string]*string
	ctrlK := "Mod+K"
	if err := s.Settings.Set(ctx, "keybindings", keybindings{"tab.new": &ctrlK, "tab.close": nil}); err != nil {
		t.Fatal(err)
	}
	if err := s.Settings.Set(ctx, "query.maxRows", 10000); err != nil {
		t.Fatal(err)
	}
	if err := s.Settings.Set(ctx, "query.maxRows", 5000); err != nil {
		t.Fatal(err)
	}

	var gotBindings keybindings
	if err := s.Settings.Get(ctx, "keybindings", &gotBindings); err != nil {
		t.Fatal(err)
	}
	if v := gotBindings["tab.new"]; v == nil || *v != "Mod+K" {
		t.Fatalf("keybindings[tab.new] = %v, want Mod+K", v)
	}
	if v, ok := gotBindings["tab.close"]; !ok || v != nil {
		t.Fatalf("keybindings[tab.close] = %v (present %v), want an explicit null", v, ok)
	}
	if err := s.Settings.Get(ctx, "query.maxRows", &limit); err != nil || limit != 5000 {
		t.Fatalf("query.maxRows = %d, %v; want the overwritten 5000", limit, err)
	}

	if err := s.Settings.Delete(ctx, "query.maxRows"); err != nil {
		t.Fatal(err)
	}
	if err := s.Settings.Delete(ctx, "query.maxRows"); err != nil {
		t.Fatalf("Delete() of a missing key error = %v, want nil", err)
	}
	if err := s.Settings.Get(ctx, "query.maxRows", &limit); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after Delete error = %v, want ErrNotFound", err)
	}
	if err := s.Settings.Set(ctx, "", 1); err == nil {
		t.Fatal("Set() with an empty key succeeded")
	}
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Skipf("no config dir in this environment: %v", err)
	}
	if filepath.Base(path) != "querra.db" || filepath.Base(filepath.Dir(path)) != "Querra" {
		t.Fatalf("DefaultPath() = %q, want …/Querra/querra.db", path)
	}
}

func names(groups []Group) string {
	out := make([]string, len(groups))
	for i, g := range groups {
		out[i] = g.Name
	}
	return strings.Join(out, ",")
}

func assertConnection(t *testing.T, got, want Connection) {
	t.Helper()
	gotJSON, _ := json.Marshal(normalized(got))
	wantJSON, _ := json.Marshal(normalized(want))
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("connection mismatch\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

// normalized makes RawMessage fields comparable regardless of whitespace.
func normalized(c Connection) Connection {
	for _, raw := range []*json.RawMessage{&c.SSL, &c.SSH, &c.Options} {
		var v any
		if json.Unmarshal(*raw, &v) == nil {
			*raw, _ = json.Marshal(v)
		}
	}
	return c
}
