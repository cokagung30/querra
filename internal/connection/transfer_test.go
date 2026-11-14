package connection

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cokagung30/querra/internal/driver"
)

// The export format is a promise to other machines and later versions;
// changing it means a new version number.
func TestExportFormat(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	g, _ := svc.CreateGroup(ctx, "Clients")
	s := validSettings()
	s.GroupID, s.Env, s.Color, s.ReadOnly = g.ID, EnvProduction, ColorRed, true
	s.SSL = SSL{Mode: driver.SSLVerifyFull, CAFile: "/certs/ca.pem"}
	c, err := svc.Create(ctx, s, setPw("s3cret"))
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.Export(ctx, []string{c.ID})
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "format": "querra-connections",
  "version": 1,
  "groups": [
    {
      "name": "Clients"
    }
  ],
  "connections": [
    {
      "group": "Clients",
      "name": "Shop",
      "engine": "postgres",
      "env": "production",
      "color": "red",
      "host": "db.example.com",
      "port": 5432,
      "username": "app",
      "database": "shop",
      "ssl": {
        "mode": "verify-full",
        "caFile": "/certs/ca.pem",
        "certFile": "",
        "keyFile": ""
      },
      "readOnly": true,
      "passwordRequired": true
    }
  ]
}
`
	if string(got) != want {
		t.Errorf("Export() =\n%s\nwant\n%s", got, want)
	}
	if bytes.Contains(got, []byte("s3cret")) {
		t.Fatal("the export contains the password")
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	ctx := context.Background()
	src, _ := newTestService(t)
	clients, _ := src.CreateGroup(ctx, "Clients")
	_, _ = src.CreateGroup(ctx, "Empty")

	s := validSettings()
	s.GroupID, s.Env = clients.ID, EnvStaging
	shop, _ := src.Create(ctx, s, setPw("s3cret"))
	s.GroupID, s.Name, s.Port = "", "Local", 0
	local, _ := src.Create(ctx, s, keep)

	data, err := src.Export(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	dst, mem := newTestService(t)
	res, err := dst.Import(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	if res.Connections != 2 || res.Groups != 2 {
		t.Fatalf("Import() = %+v, want 2 connections and 2 groups (the empty one too)", res)
	}
	if mem.Len() != 0 {
		t.Fatal("Import() wrote to the keychain")
	}

	groups, _ := dst.Groups(ctx)
	if len(groups) != 2 || groups[0].Name != "Clients" || groups[1].Name != "Empty" {
		t.Fatalf("groups after import = %+v, want Clients, Empty", groups)
	}
	list, _ := dst.List(ctx)
	if len(list) != 2 {
		t.Fatalf("%d connections after import, want 2", len(list))
	}
	byName := map[string]Connection{}
	for _, c := range list {
		byName[c.Settings.Name] = c
	}

	gotShop, gotLocal := byName["Shop"], byName["Local"]
	wantShop := shop.Settings
	wantShop.GroupID = groups[0].ID
	if gotShop.Settings != wantShop || gotShop.PasswordState != PasswordRequired || gotShop.ID == shop.ID {
		t.Errorf("imported Shop = %+v, want %+v with a password required and a new id", gotShop, wantShop)
	}
	if gotLocal.Settings != local.Settings || gotLocal.PasswordState != PasswordNone {
		t.Errorf("imported Local = %+v, want %+v with no password", gotLocal, local.Settings)
	}
}

func TestImportMatchesGroupsAndOnlyAdds(t *testing.T) {
	ctx := context.Background()
	src, _ := newTestService(t)
	g, _ := src.CreateGroup(ctx, "Clients")
	s := validSettings()
	s.GroupID = g.ID
	_, _ = src.Create(ctx, s, keep)
	data, _ := src.Export(ctx, nil)

	dst, _ := newTestService(t)
	existing, _ := dst.CreateGroup(ctx, "clients")
	mine, _ := dst.Create(ctx, validSettings(), keep)

	for range 2 {
		if _, err := dst.Import(ctx, data); err != nil {
			t.Fatal(err)
		}
	}
	groups, _ := dst.Groups(ctx)
	if len(groups) != 1 || groups[0].ID != existing.ID {
		t.Fatalf("groups = %+v, want only the existing one, matched ignoring case", groups)
	}
	list, _ := dst.List(ctx)
	if len(list) != 3 {
		t.Fatalf("%d connections, want 3: the existing one and two imports", len(list))
	}
	var inGroup []int
	for _, c := range list {
		if c.ID == mine.ID && c.Settings != mine.Settings {
			t.Error("Import changed an existing connection")
		}
		if c.Settings.GroupID == existing.ID {
			inGroup = append(inGroup, c.SortOrder)
		}
	}
	if len(inGroup) != 2 || inGroup[0] != 0 || inGroup[1] != 1 {
		t.Errorf("imported sort orders = %v, want 0 and 1", inGroup)
	}
}

func TestExportSelected(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.CreateGroup(ctx, "A")
	_, _ = svc.CreateGroup(ctx, "B")
	s := validSettings()
	s.GroupID = a.ID
	c, _ := svc.Create(ctx, s, keep)
	_, _ = svc.Create(ctx, validSettings(), keep)

	data, err := svc.Export(ctx, []string{c.ID})
	if err != nil {
		t.Fatal(err)
	}
	file, err := parseExport(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Groups) != 1 || file.Groups[0].Name != "A" || len(file.Connections) != 1 {
		t.Fatalf("Export(one) = %+v, want only that connection and its group", file)
	}
	if _, err := svc.Export(ctx, []string{"missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Export(missing) = %v, want ErrNotFound", err)
	}
}

func TestImportRejects(t *testing.T) {
	tests := []struct {
		name, data, want string
	}{
		{"not JSON", `hello`, "not a Querra connections file"},
		{"other JSON", `{"connections": []}`, "not a Querra connections file"},
		{"newer", `{"format": "querra-connections", "version": 2}`, "newer version of Querra"},
		{"no version", `{"format": "querra-connections"}`, "unknown format version"},
		{"unknown field", `{"format": "querra-connections", "version": 1, "secrets": {}}`, `unknown field "secrets"`},
		{
			"invalid connection",
			`{"format": "querra-connections", "version": 1, "groups": [{"name": "G"}],
			  "connections": [{"name": "ok", "engine": "postgres", "env": "local", "host": "h"},
			                  {"name": "no host", "engine": "postgres", "env": "local"}]}`,
			`connection 2 ("no host"): host`,
		},
		{
			"group without a name",
			`{"format": "querra-connections", "version": 1, "groups": [{"name": " "}], "connections": []}`,
			`group " ": name: the name is required`,
		},
		{"too big", `{"format": "querra-connections", "version": 1, "pad": "` + strings.Repeat("x", maxImportBytes) + `"}`, "larger than"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			svc, _ := newTestService(t)
			_, err := svc.Import(ctx, []byte(tt.data))
			if !errors.Is(err, ErrInvalidFile) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Import() error = %v, want ErrInvalidFile mentioning %q", err, tt.want)
			}
			list, _ := svc.List(ctx)
			groups, _ := svc.Groups(ctx)
			if len(list) != 0 || len(groups) != 0 {
				t.Fatalf("a rejected file added %d connections and %d groups", len(list), len(groups))
			}
		})
	}
}
