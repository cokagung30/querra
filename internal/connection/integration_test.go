//go:build integration

package connection

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/cokagung30/querra/internal/apperr"
	"github.com/cokagung30/querra/internal/driver"
)

// Test Connection against the querra_test database from `wails3 task
// db:reset`. It only logs in.

func postgresSettings(t *testing.T) Settings {
	t.Helper()
	host := os.Getenv("QUERRA_DEV_POSTGRES_HOST")
	if host == "" {
		t.Skip("QUERRA_DEV_POSTGRES_HOST is not set (see dev/test.env.example)")
	}
	port, _ := strconv.Atoi(os.Getenv("QUERRA_DEV_POSTGRES_PORT"))
	return Settings{
		Name:     "querra_test",
		Engine:   driver.Postgres,
		Env:      EnvLocal,
		Host:     host,
		Port:     port,
		Username: "querra_test",
		Database: "querra_test",
	}
}

func TestTestPostgres(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	set := postgresSettings(t)

	res, err := svc.Test(ctx, "", set, setPw("querra_test"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.ServerVersion == "" || res.Error != nil {
		t.Fatalf("Test() = %+v, want OK with a server version", res)
	}
	wantTLS := "skipped"
	if res.TLS {
		wantTLS = "ok"
	}
	if got, want := stages(res), "tcp:ok tls:"+wantTLS+" auth:ok ping:ok"; got != want {
		t.Errorf("stages = %s, want %s", got, want)
	}

	// The saved password is used when the form keeps it.
	c, err := svc.Create(ctx, set, setPw("querra_test"))
	if err != nil {
		t.Fatal(err)
	}
	if res, err := svc.Test(ctx, c.ID, set, keep); err != nil || !res.OK {
		t.Fatalf("Test(saved password) = %+v, %v; want OK", res, err)
	}
}

func TestTestPostgresFailures(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	missing := postgresSettings(t)
	missing.Database = "querra_test_no_such_db"
	res, err := svc.Test(ctx, "", missing, setPw("querra_test"))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || res.Error.Code != apperr.DatabaseNotFound || res.Stages[2].Status != StageFailed {
		t.Errorf("Test(missing database) = %s, %+v; want the auth stage failing with database_not_found", stages(res), res.Error)
	}

	required := postgresSettings(t)
	required.SSL.Mode = driver.SSLRequire
	res, err = svc.Test(ctx, "", required, setPw("querra_test"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK && (res.Error.Code != apperr.TLSRequired || stages(res) != "tcp:ok tls:failed auth:not_run ping:not_run") {
		t.Errorf("Test(sslmode=require) = %s, %+v; want OK, or the TLS stage failing with tls_required", stages(res), res.Error)
	}

	res, err = svc.Test(ctx, "", postgresSettings(t), setPw("wrong-password-7f3a9c"))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Skip("the server trusts local connections, so any password works")
	}
	if res.Error.Code != apperr.AuthFailed || res.Stages[2].Status != StageFailed {
		t.Errorf("Test(wrong password) = %s, %+v; want the auth stage failing with auth_failed", stages(res), res.Error)
	}
}
