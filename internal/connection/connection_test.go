package connection

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cokagung30/querra/internal/driver"
	_ "github.com/cokagung30/querra/internal/driver/postgres" // a real network engine
	"github.com/cokagung30/querra/internal/secrets"
	"github.com/cokagung30/querra/internal/store"
)

// fileEngine is a registered engine without a network server, like SQLite.
const fileEngine driver.Engine = "test-file"

func init() {
	driver.Register(fakeDriver{engine: fileEngine})
}

func newTestService(t *testing.T) (*Service, *secrets.Memory) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "querra.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	mem := secrets.NewMemory()
	return NewService(st, mem), mem
}

func validSettings() Settings {
	return Settings{
		Name:     "Shop",
		Engine:   driver.Postgres,
		Env:      EnvLocal,
		Host:     "db.example.com",
		Port:     5432,
		Username: "app",
		Database: "shop",
	}
}

var (
	keep     = PasswordChange{Action: PasswordKeep}
	clearPw  = PasswordChange{Action: PasswordClear}
	setPw    = func(v string) PasswordChange { return PasswordChange{Action: PasswordSet, Value: v} }
	keychain = errors.New("keychain unavailable")
)

// wantField checks that err is an ErrInvalid for the given field.
func wantField(t *testing.T, err error, field string) {
	t.Helper()
	fe, ok := errors.AsType[*FieldError](err)
	if !ok || !errors.Is(err, ErrInvalid) || fe.Field != field {
		t.Fatalf("error = %v, want an invalid %s", err, field)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*Settings)
		field string // "" means valid
	}{
		{"valid", func(*Settings) {}, ""},
		{"default port", func(s *Settings) { s.Port = 0 }, ""},
		{"all colors", func(s *Settings) { s.Color = ColorPurple }, ""},
		{"client certificate", func(s *Settings) { s.SSL.CertFile, s.SSL.KeyFile = "c.pem", "k.pem" }, ""},
		{"no name", func(s *Settings) { s.Name = "  " }, "name"},
		{"long name", func(s *Settings) { s.Name = strings.Repeat("é", maxNameRunes+1) }, "name"},
		{"unknown engine", func(s *Settings) { s.Engine = "oracle" }, "engine"},
		{"unknown env", func(s *Settings) { s.Env = "prod" }, "env"},
		{"no env", func(s *Settings) { s.Env = "" }, "env"},
		{"unknown color", func(s *Settings) { s.Color = "#ff0000" }, "color"},
		{"no host", func(s *Settings) { s.Host = " " }, "host"},
		{"negative port", func(s *Settings) { s.Port = -1 }, "port"},
		{"port too high", func(s *Settings) { s.Port = 65536 }, "port"},
		{"unknown SSL mode", func(s *Settings) { s.SSL.Mode = "allow" }, "ssl.mode"},
		{"certificate without key", func(s *Settings) { s.SSL.CertFile = "c.pem" }, "ssl.keyFile"},
		{"key without certificate", func(s *Settings) { s.SSL.KeyFile = "k.pem" }, "ssl.certFile"},
		{"file engine needs no host", func(s *Settings) { s.Engine, s.Host, s.Port, s.Database = fileEngine, "", 0, "/data/app.db" }, ""},
		{"file engine needs a file", func(s *Settings) { s.Engine, s.Host, s.Database = fileEngine, "", "" }, "database"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSettings()
			tt.edit(&s)
			err := validate(normalize(s))
			if tt.field == "" {
				if err != nil {
					t.Fatalf("validate() = %v, want nil", err)
				}
				return
			}
			wantField(t, err, tt.field)
		})
	}
}

func TestNormalize(t *testing.T) {
	s := validSettings()
	s.Name, s.Host, s.Username = "  Shop ", " db.example.com\t", " app "
	got := normalize(s)
	if got.Name != "Shop" || got.Host != "db.example.com" || got.SSL.Mode != driver.SSLPrefer {
		t.Errorf("normalize() = %+v, want trimmed name and host and SSL mode prefer", got)
	}
	if got.Username != " app " {
		t.Errorf("Username = %q, want it as typed", got.Username)
	}
}

func TestCheckPassword(t *testing.T) {
	for _, ok := range []PasswordChange{keep, clearPw, setPw("s3cret")} {
		if err := checkPassword(ok); err != nil {
			t.Errorf("checkPassword(%s) = %v", ok.Action, err)
		}
	}
	tooLong := strings.Repeat("x", secrets.MaxValueBytes+1)
	for _, bad := range []PasswordChange{setPw(""), setPw(tooLong), {Action: ""}, {Action: "save"}} {
		err := checkPassword(bad)
		wantField(t, err, "password")
		if bad.Value != "" && strings.Contains(err.Error(), bad.Value) {
			t.Error("the error contains the password")
		}
	}
}

func TestCreateAndList(t *testing.T) {
	ctx := context.Background()
	svc, mem := newTestService(t)
	g, err := svc.CreateGroup(ctx, "Clients")
	if err != nil {
		t.Fatal(err)
	}

	s := validSettings()
	s.GroupID, s.Name = g.ID, "  Shop "
	shop, err := svc.Create(ctx, s, setPw("s3cret"))
	if err != nil {
		t.Fatal(err)
	}
	if shop.Settings.Name != "Shop" || shop.Settings.SSL.Mode != driver.SSLPrefer || shop.PasswordState != PasswordSaved {
		t.Fatalf("Create() = %+v, want a trimmed name, SSL mode prefer and a saved password", shop)
	}
	if v, err := mem.Get(shop.ID, secrets.Password); err != nil || v != "s3cret" {
		t.Fatalf("keychain password = %q, %v; want the one given", v, err)
	}

	s.Name = "Blog"
	blog, err := svc.Create(ctx, s, keep)
	if err != nil {
		t.Fatal(err)
	}
	s.GroupID, s.Name = "", "Local"
	local, err := svc.Create(ctx, s, clearPw)
	if err != nil {
		t.Fatal(err)
	}
	if blog.PasswordState != PasswordNone || mem.Len() != 1 {
		t.Fatalf("Create() with keep: state %q, %d secrets; want none and only Shop's", blog.PasswordState, mem.Len())
	}
	if shop.SortOrder != 0 || blog.SortOrder != 1 || local.SortOrder != 0 {
		t.Fatalf("sort orders = %d, %d, %d; want 0, 1 in the group and 0 outside it", shop.SortOrder, blog.SortOrder, local.SortOrder)
	}

	got, err := svc.Get(ctx, shop.ID)
	if err != nil || got.Settings != shop.Settings || !got.CreatedAt.Equal(shop.CreatedAt) {
		t.Fatalf("Get() = %+v, %v; want %+v", got, err, shop)
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 3 {
		t.Fatalf("List() = %d connections, %v; want 3", len(list), err)
	}
	if _, err := svc.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestCreateRejectsAndStoresNothing(t *testing.T) {
	ctx := context.Background()
	svc, mem := newTestService(t)

	s := validSettings()
	s.Host = ""
	_, err := svc.Create(ctx, s, setPw("s3cret"))
	wantField(t, err, "host")

	s = validSettings()
	s.GroupID = "no-such-group"
	_, err = svc.Create(ctx, s, keep)
	wantField(t, err, "groupId")

	_, err = svc.Create(ctx, validSettings(), setPw(""))
	wantField(t, err, "password")

	if list, _ := svc.List(ctx); len(list) != 0 || mem.Len() != 0 {
		t.Fatalf("after rejected creates: %d connections, %d secrets; want none", len(list), mem.Len())
	}
}

// failingSecrets fails the operations it is told to, like a locked keychain.
type failingSecrets struct {
	secrets.Store
	set, del bool
}

func (f *failingSecrets) Set(id string, kind secrets.Kind, v string) error {
	if f.set {
		return keychain
	}
	return f.Store.Set(id, kind, v)
}

func (f *failingSecrets) Delete(id string, kind secrets.Kind) error {
	if f.del {
		return keychain
	}
	return f.Store.Delete(id, kind)
}

func (f *failingSecrets) DeleteAll(id string) error {
	if f.del {
		return keychain
	}
	return f.Store.DeleteAll(id)
}

func TestCreateUndoesTheConnectionWhenTheKeychainFails(t *testing.T) {
	ctx := context.Background()
	svc, mem := newTestService(t)
	svc.secrets = &failingSecrets{Store: mem, set: true}

	if _, err := svc.Create(ctx, validSettings(), setPw("s3cret")); !errors.Is(err, keychain) {
		t.Fatalf("Create() error = %v, want the keychain error", err)
	}
	if list, _ := svc.List(ctx); len(list) != 0 {
		t.Fatalf("List() = %v, want the connection removed again", list)
	}
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	svc, mem := newTestService(t)
	g, _ := svc.CreateGroup(ctx, "Clients")
	other, _ := svc.Create(ctx, validSettings(), keep)
	c, err := svc.Create(ctx, validSettings(), setPw("old"))
	if err != nil {
		t.Fatal(err)
	}
	if c.SortOrder != 1 {
		t.Fatalf("SortOrder = %d, want 1", c.SortOrder)
	}

	s := c.Settings
	s.Name, s.Env, s.ReadOnly = "Shop (prod)", EnvProduction, true
	got, err := svc.Update(ctx, c.ID, s, keep)
	if err != nil {
		t.Fatal(err)
	}
	if got.Settings.Name != "Shop (prod)" || !got.Settings.ReadOnly || got.SortOrder != 1 || got.PasswordState != PasswordSaved {
		t.Fatalf("Update(keep) = %+v, want new settings, same place, password still saved", got)
	}
	if !got.CreatedAt.Equal(c.CreatedAt) {
		t.Fatalf("CreatedAt changed from %v to %v", c.CreatedAt, got.CreatedAt)
	}
	if v, _ := mem.Get(c.ID, secrets.Password); v != "old" {
		t.Fatalf("password after keep = %q, want old", v)
	}

	if got, err = svc.Update(ctx, c.ID, s, setPw("new")); err != nil || got.PasswordState != PasswordSaved {
		t.Fatalf("Update(set) = %+v, %v", got, err)
	}
	if v, _ := mem.Get(c.ID, secrets.Password); v != "new" {
		t.Fatalf("password after set = %q, want new", v)
	}

	// Moving to a group puts it last there.
	s.GroupID = g.ID
	if got, err = svc.Update(ctx, c.ID, s, clearPw); err != nil {
		t.Fatal(err)
	}
	if got.Settings.GroupID != g.ID || got.SortOrder != 0 || got.PasswordState != PasswordNone {
		t.Fatalf("Update(move, clear) = %+v, want group %s, order 0, no password", got, g.ID)
	}
	if ok, _ := secrets.Has(mem, c.ID, secrets.Password); ok {
		t.Fatal("password still in the keychain after clear")
	}
	if o, _ := svc.Get(ctx, other.ID); o.Settings != other.Settings {
		t.Fatal("Update changed another connection")
	}

	if _, err := svc.Update(ctx, "missing", s, keep); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update(missing) error = %v, want ErrNotFound", err)
	}
	s.Port = 70000
	_, err = svc.Update(ctx, c.ID, s, keep)
	wantField(t, err, "port")
}

func TestUpdateChangesNothingWhenTheKeychainFails(t *testing.T) {
	ctx := context.Background()
	svc, mem := newTestService(t)
	c, err := svc.Create(ctx, validSettings(), setPw("old"))
	if err != nil {
		t.Fatal(err)
	}
	svc.secrets = &failingSecrets{Store: mem, set: true, del: true}

	s := c.Settings
	s.Name = "Renamed"
	for _, pw := range []PasswordChange{setPw("new"), clearPw} {
		if _, err := svc.Update(ctx, c.ID, s, pw); !errors.Is(err, keychain) {
			t.Fatalf("Update(%s) error = %v, want the keychain error", pw.Action, err)
		}
	}
	got, _ := svc.Get(ctx, c.ID)
	if got.Settings.Name != "Shop" || got.PasswordState != PasswordSaved {
		t.Fatalf("after failed updates: %+v, want it unchanged", got)
	}
	if v, _ := mem.Get(c.ID, secrets.Password); v != "old" {
		t.Fatalf("password = %q, want old", v)
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	svc, mem := newTestService(t)
	c, _ := svc.Create(ctx, validSettings(), setPw("s3cret"))
	_ = mem.Set(c.ID, secrets.SSHPassphrase, "ssh-s3cret") // as an SSH tunnel will (M2.7)
	keepMe, _ := svc.Create(ctx, validSettings(), setPw("other"))

	failing := &failingSecrets{Store: mem, del: true}
	svc.secrets = failing
	if err := svc.Delete(ctx, c.ID); !errors.Is(err, keychain) {
		t.Fatalf("Delete() with a failing keychain = %v, want the keychain error", err)
	}
	if _, err := svc.Get(ctx, c.ID); err != nil {
		t.Fatalf("connection gone although its secrets could not be removed: %v", err)
	}

	failing.del = false
	if err := svc.Delete(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after Delete = %v, want ErrNotFound", err)
	}
	if mem.Len() != 1 {
		t.Fatalf("%d secrets left, want only the other connection's", mem.Len())
	}
	if v, _ := mem.Get(keepMe.ID, secrets.Password); v != "other" {
		t.Fatal("Delete removed another connection's password")
	}
	if err := svc.Delete(ctx, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Delete() = %v, want ErrNotFound", err)
	}
}

func TestDuplicate(t *testing.T) {
	ctx := context.Background()
	svc, mem := newTestService(t)
	g, _ := svc.CreateGroup(ctx, "Clients")
	s := validSettings()
	s.GroupID, s.Env, s.Color, s.ReadOnly = g.ID, EnvProduction, ColorRed, true
	s.SSL = SSL{Mode: driver.SSLVerifyFull, CAFile: "/certs/ca.pem"}

	a, _ := svc.Create(ctx, s, setPw("s3cret"))
	_ = mem.Set(a.ID, secrets.SSHPassphrase, "ssh-s3cret")
	s.Name = "Blog"
	b, _ := svc.Create(ctx, s, keep)

	cp, err := svc.Duplicate(ctx, a.ID, " Shop copy ")
	if err != nil {
		t.Fatal(err)
	}
	want := a.Settings
	want.Name = "Shop copy"
	if cp.ID == a.ID || cp.Settings != want || cp.PasswordState != PasswordSaved {
		t.Fatalf("Duplicate() = %+v, want a new connection with %+v and a saved password", cp, want)
	}
	for kind, v := range map[secrets.Kind]string{secrets.Password: "s3cret", secrets.SSHPassphrase: "ssh-s3cret"} {
		if got, err := mem.Get(cp.ID, kind); err != nil || got != v {
			t.Errorf("copied %s = %q, %v; want %q", kind, got, err, v)
		}
	}

	list, _ := svc.List(ctx)
	var order []string
	for _, c := range list {
		order = append(order, c.ID)
	}
	if want := []string{a.ID, cp.ID, b.ID}; !slices.Equal(order, want) {
		t.Fatalf("order = %v, want the copy right after the original %v", order, want)
	}

	_, err = svc.Duplicate(ctx, a.ID, "")
	wantField(t, err, "name")
	if _, err := svc.Duplicate(ctx, "missing", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Duplicate(missing) = %v, want ErrNotFound", err)
	}
}

func TestDuplicateUndoesTheCopyWhenTheKeychainFails(t *testing.T) {
	ctx := context.Background()
	svc, mem := newTestService(t)
	a, _ := svc.Create(ctx, validSettings(), setPw("s3cret"))
	svc.secrets = &failingSecrets{Store: mem, set: true}

	if _, err := svc.Duplicate(ctx, a.ID, "Copy"); !errors.Is(err, keychain) {
		t.Fatalf("Duplicate() error = %v, want the keychain error", err)
	}
	if list, _ := svc.List(ctx); len(list) != 1 {
		t.Fatalf("%d connections after a failed Duplicate, want 1", len(list))
	}
}

func TestGroups(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	a, err := svc.CreateGroup(ctx, " Clients ")
	if err != nil || a.Name != "Clients" || a.SortOrder != 0 {
		t.Fatalf("CreateGroup() = %+v, %v", a, err)
	}
	b, err := svc.CreateGroup(ctx, "Internal")
	if err != nil || b.SortOrder != 1 {
		t.Fatalf("CreateGroup() = %+v, %v; want sort order 1", b, err)
	}
	_, err = svc.CreateGroup(ctx, "clients")
	wantField(t, err, "name")
	_, err = svc.CreateGroup(ctx, "")
	wantField(t, err, "name")

	if g, err := svc.RenameGroup(ctx, a.ID, "CLIENTS"); err != nil || g.Name != "CLIENTS" || g.SortOrder != 0 {
		t.Fatalf("RenameGroup(own name, other case) = %+v, %v", g, err)
	}
	_, err = svc.RenameGroup(ctx, b.ID, "clients")
	wantField(t, err, "name")
	if _, err := svc.RenameGroup(ctx, "missing", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RenameGroup(missing) = %v, want ErrNotFound", err)
	}

	if err := svc.ArrangeGroups(ctx, []string{b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	groups, _ := svc.Groups(ctx)
	if len(groups) != 2 || groups[0].ID != b.ID || groups[1].Name != "CLIENTS" {
		t.Fatalf("Groups() = %+v, want Internal then CLIENTS", groups)
	}

	s := validSettings()
	s.GroupID = a.ID
	c, _ := svc.Create(ctx, s, keep)
	if err := svc.DeleteGroup(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := svc.Get(ctx, c.ID); err != nil || got.Settings.GroupID != "" {
		t.Fatalf("connection after its group was deleted = %+v, %v; want it kept without a group", got, err)
	}
	if err := svc.DeleteGroup(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteGroup(missing) = %v, want ErrNotFound", err)
	}
}

func TestArrange(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	g, _ := svc.CreateGroup(ctx, "Clients")
	a, _ := svc.Create(ctx, validSettings(), keep)
	b, _ := svc.Create(ctx, validSettings(), keep)

	if err := svc.Arrange(ctx, g.ID, []string{b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	list, _ := svc.List(ctx)
	if list[0].ID != b.ID || list[0].Settings.GroupID != g.ID || list[1].ID != a.ID {
		t.Fatalf("List() after Arrange = %+v, want b then a, both in the group", list)
	}
	err := svc.Arrange(ctx, "no-such-group", []string{a.ID})
	wantField(t, err, "groupId")
	if err := svc.Arrange(ctx, "", []string{"missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Arrange(missing) = %v, want ErrNotFound", err)
	}
}

func TestEngines(t *testing.T) {
	engines := Engines()
	i := slices.IndexFunc(engines, func(e EngineInfo) bool { return e.Engine == driver.Postgres })
	if i < 0 {
		t.Fatalf("Engines() = %+v, want postgres", engines)
	}
	if pg := engines[i]; pg.DefaultPort != 5432 || !pg.Capabilities.Schemas {
		t.Errorf("postgres = %+v, want default port 5432 and schemas", pg)
	}
}
