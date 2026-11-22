package app

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/cokagung30/querra/internal/apperr"
	"github.com/cokagung30/querra/internal/connection"
	"github.com/cokagung30/querra/internal/driver"
	_ "github.com/cokagung30/querra/internal/driver/postgres" // registered in main.go too
	"github.com/cokagung30/querra/internal/secrets"
	"github.com/cokagung30/querra/internal/store"
)

func newTestConnectionService(t *testing.T) (*ConnectionService, *secrets.Memory) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "querra.db"))
	if err != nil {
		t.Fatal(err)
	}
	mem := secrets.NewMemory()
	s := NewConnectionService(st, mem)
	t.Cleanup(func() { _ = s.ServiceShutdown() })
	return s, mem
}

func testSettings() connection.Settings {
	return connection.Settings{
		Name:     "Shop",
		Engine:   driver.Postgres,
		Env:      connection.EnvLocal,
		Host:     "127.0.0.1",
		Port:     1, // nothing listens there
		Username: "app",
		Database: "shop",
	}
}

var keepPassword = connection.PasswordChange{Action: connection.PasswordKeep}

// wantAppErr checks that err is an *apperr.Error with the given code and,
// when detail is not "", that detail.
func wantAppErr(t *testing.T, err error, code apperr.Code, detail string) {
	t.Helper()
	e, ok := errors.AsType[*apperr.Error](err)
	if !ok {
		t.Fatalf("error = %#v (%v), want an *apperr.Error", err, err)
	}
	if e.Code != code || e.Message == "" || (detail != "" && e.Detail != detail) {
		t.Fatalf("error = %#v, want code %q, a message, and detail %q", e, code, detail)
	}
}

func TestConnectionServiceErrors(t *testing.T) {
	s, _ := newTestConnectionService(t)
	ctx := context.Background()

	noHost := testSettings()
	noHost.Host = ""
	_, err := s.Create(ctx, noHost, keepPassword)
	wantAppErr(t, err, apperr.InvalidInput, "host")

	_, err = s.Create(ctx, testSettings(), connection.PasswordChange{Action: connection.PasswordSet})
	wantAppErr(t, err, apperr.InvalidInput, "password")

	_, err = s.Get(ctx, "missing")
	wantAppErr(t, err, apperr.NotFound, "")
	wantAppErr(t, s.Delete(ctx, "missing"), apperr.NotFound, "")
	_, err = s.RenameGroup(ctx, "missing", "Work")
	wantAppErr(t, err, apperr.NotFound, "")

	_, err = s.Import(ctx, []byte(`{"format": "something else"}`))
	wantAppErr(t, err, apperr.InvalidFile, "")

	// A file with an invalid connection wraps both ErrInvalidFile and
	// ErrInvalid; it is the file that is wrong, not the form.
	bad := `{"format": "querra-connections", "version": 1, "groups": [],
		"connections": [{"name": "x", "engine": "postgres", "env": "local", "ssl": {}, "readOnly": false, "passwordRequired": false}]}`
	_, err = s.Import(ctx, []byte(bad))
	wantAppErr(t, err, apperr.InvalidFile, "")

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = s.Test(cancelled, "", testSettings(), keepPassword)
	wantAppErr(t, err, apperr.Cancelled, "")
}

func TestConnectionServiceErrorReachesTheFrontend(t *testing.T) {
	s, _ := newTestConnectionService(t)
	noHost := testSettings()
	noHost.Host = ""
	_, err := s.Create(context.Background(), noHost, keepPassword)

	// What MarshalError sends as error.cause.
	var cause map[string]any
	if err := json.Unmarshal(apperr.Marshal(err), &cause); err != nil {
		t.Fatal(err)
	}
	if cause["code"] != "invalid_input" || cause["detail"] != "host" || cause["message"] == "" {
		t.Errorf("error.cause = %v, want invalid_input for host with a message", cause)
	}
}

func TestConnectionServiceRoundTrip(t *testing.T) {
	s, mem := newTestConnectionService(t)
	ctx := context.Background()

	// Success must be a nil error interface, not a nil *apperr.Error, or
	// Wails would reject the call.
	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if b, _ := json.Marshal(list); string(b) != "[]" {
		t.Errorf("List on an empty store = %s, want []", b)
	}

	c, err := s.Create(ctx, testSettings(), connection.PasswordChange{Action: connection.PasswordSet, Value: "pw"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.PasswordState != connection.PasswordSaved {
		t.Errorf("PasswordState = %q, want %q", c.PasswordState, connection.PasswordSaved)
	}
	if v, err := mem.Get(c.ID, secrets.Password); err != nil || v != "pw" {
		t.Errorf("keychain password = %q, %v; want pw", v, err)
	}

	data, err := s.Export(ctx, nil)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	res, err := s.Import(ctx, data)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Connections != 1 {
		t.Errorf("Import added %d connections, want 1", res.Connections)
	}

	if err := s.Delete(ctx, c.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := mem.Get(c.ID, secrets.Password); !errors.Is(err, secrets.ErrNotFound) {
		t.Errorf("password after Delete: %v, want ErrNotFound", err)
	}
}

func TestConnectionServiceEngines(t *testing.T) {
	s, _ := newTestConnectionService(t)
	for _, e := range s.Engines() {
		if e.Engine == driver.Postgres {
			if e.DefaultPort != 5432 {
				t.Errorf("postgres DefaultPort = %d, want 5432", e.DefaultPort)
			}
			return
		}
	}
	t.Errorf("Engines() = %+v, want postgres", s.Engines())
}

func TestConnectionServiceShutdownClosesStore(t *testing.T) {
	s, _ := newTestConnectionService(t)
	if err := s.ServiceShutdown(); err != nil {
		t.Fatalf("ServiceShutdown: %v", err)
	}
	if _, err := s.List(context.Background()); err == nil {
		t.Error("List after ServiceShutdown succeeded, want an error from the closed store")
	}
}
