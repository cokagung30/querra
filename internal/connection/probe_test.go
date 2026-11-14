package connection

import (
	"context"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"strconv"
	"sync"
	"testing"

	_ "modernc.org/sqlite" // a real *sql.DB for the fake connection's pool

	"github.com/cokagung30/querra/internal/apperr"
	"github.com/cokagung30/querra/internal/driver"
)

// fakeDriver opens connections the way a test tells it to.
type fakeDriver struct {
	driver.Driver // methods the tests never call
	engine        driver.Engine
	port          int
	open          func(ctx context.Context, cfg driver.Config, dial driver.DialFunc) (driver.Conn, error)
}

func (d fakeDriver) Engine() driver.Engine { return d.engine }

func (d fakeDriver) Capabilities() driver.Capabilities { return driver.Capabilities{} }

func (d fakeDriver) DefaultPort() int { return d.port }

func (d fakeDriver) Open(ctx context.Context, cfg driver.Config, dial driver.DialFunc) (driver.Conn, error) {
	if d.open == nil {
		return nil, errors.New("fakeDriver: no open func")
	}
	return d.open(ctx, cfg, dial)
}

type fakeConn struct {
	driver.Conn // methods the tests never call
	db          *sql.DB
	info        driver.ServerInfo
	infoErr     error
}

func (c *fakeConn) Pool() *sql.DB { return c.db }

func (c *fakeConn) ServerInfo(context.Context) (driver.ServerInfo, error) { return c.info, c.infoErr }

func (c *fakeConn) Close() error { return c.db.Close() }

func newFakeConn(t *testing.T, info driver.ServerInfo, infoErr error) *fakeConn {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return &fakeConn{db: db, info: info, infoErr: infoErr}
}

// server accepts TCP connections and closes them, enough for a dial to
// succeed.
func server(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	return ln.Addr().String()
}

// closedPort returns an address nothing listens on.
func closedPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// opens returns an open func that dials addr (when addr is not "") and then
// returns conn or err.
func opens(addr string, conn driver.Conn, err error) func(context.Context, driver.Config, driver.DialFunc) (driver.Conn, error) {
	return func(ctx context.Context, _ driver.Config, dial driver.DialFunc) (driver.Conn, error) {
		if addr != "" {
			c, dialErr := dial(ctx, "tcp", addr)
			if dialErr != nil {
				return nil, fmt.Errorf("connect: %w", dialErr)
			}
			_ = c.Close()
		}
		return conn, err
	}
}

func stages(r TestResult) string {
	out := ""
	for _, s := range r.Stages {
		out += string(s.Stage) + ":" + string(s.Status) + " "
	}
	return out[:len(out)-1]
}

func TestProbe(t *testing.T) {
	up := server(t)
	login := errors.New("server error")
	withTLS := &driver.ServerInfo{Version: "17.2", TLS: true}
	plain := &driver.ServerInfo{Version: "17.2"}

	tests := []struct {
		name string
		port int // 0: an engine without a network server
		mode driver.SSLMode
		dial string // "up", "closed", or "" to fail before dialing
		// Open fails with err, or succeeds with a connection whose
		// ServerInfo returns info and infoErr.
		err     error
		info    *driver.ServerInfo
		infoErr error

		stages string
		code   apperr.Code // "" when OK
	}{
		{
			name: "refused", port: 5432, mode: driver.SSLPrefer, dial: "closed",
			stages: "tcp:failed tls:not_run auth:not_run ping:not_run", code: apperr.ConnectionRefused,
		},
		{
			name: "wrong password over TLS", port: 5432, mode: driver.SSLRequire, dial: "up",
			err:    driver.Mark(login, driver.ErrAuthFailed),
			stages: "tcp:ok tls:ok auth:failed ping:not_run", code: apperr.AuthFailed,
		},
		{
			name: "wrong password after the server declined TLS", port: 5432, mode: driver.SSLPrefer, dial: "up",
			err:    driver.Mark(login, driver.ErrAuthFailed, driver.ErrTLSRefused),
			stages: "tcp:ok tls:skipped auth:failed ping:not_run", code: apperr.AuthFailed,
		},
		{
			name: "missing database without TLS", port: 5432, mode: driver.SSLDisable, dial: "up",
			err:    driver.Mark(login, driver.ErrDatabaseNotFound),
			stages: "tcp:ok tls:skipped auth:failed ping:not_run", code: apperr.DatabaseNotFound,
		},
		{
			name: "TLS required but refused", port: 5432, mode: driver.SSLRequire, dial: "up",
			err:    driver.Mark(login, driver.ErrTLSRefused),
			stages: "tcp:ok tls:failed auth:not_run ping:not_run", code: apperr.TLSRequired,
		},
		{
			name: "untrusted certificate", port: 5432, mode: driver.SSLVerifyFull, dial: "up",
			err:    fmt.Errorf("tls: %w", x509.UnknownAuthorityError{}),
			stages: "tcp:ok tls:failed auth:not_run ping:not_run", code: apperr.TLSVerifyFailed,
		},
		{
			name: "CA file missing", port: 5432, mode: driver.SSLVerifyCA,
			err:    &fs.PathError{Op: "open", Path: "ca.pem", Err: fs.ErrNotExist},
			stages: "tcp:not_run tls:failed auth:not_run ping:not_run", code: apperr.TLSFailed,
		},
		{
			name: "rejected before dialing", port: 5432, mode: driver.SSLPrefer,
			err:    errors.New("bad settings"),
			stages: "tcp:failed tls:not_run auth:not_run ping:not_run", code: apperr.Internal,
		},
		{
			name: "connected with TLS", port: 5432, mode: driver.SSLPrefer, dial: "up", info: withTLS,
			stages: "tcp:ok tls:ok auth:ok ping:ok",
		},
		{
			name: "connected after the server declined TLS", port: 5432, mode: driver.SSLPrefer, dial: "up", info: plain,
			stages: "tcp:ok tls:skipped auth:ok ping:ok",
		},
		{
			name: "connected but the server stopped answering", port: 5432, mode: driver.SSLDisable, dial: "up",
			info: plain, infoErr: &net.OpError{Op: "read", Err: errors.New("connection reset")},
			stages: "tcp:ok tls:skipped auth:ok ping:failed", code: apperr.NetworkError,
		},
		{
			name: "file engine opened", mode: driver.SSLPrefer, info: plain,
			stages: "tcp:skipped tls:skipped auth:ok ping:ok",
		},
		{
			name: "file engine failed", mode: driver.SSLPrefer,
			err:    driver.Mark(login, driver.ErrDatabaseNotFound),
			stages: "tcp:skipped tls:skipped auth:failed ping:not_run", code: apperr.DatabaseNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := map[string]string{"up": up, "closed": closedPort(t)}[tt.dial]
			var conn driver.Conn
			if tt.info != nil {
				conn = newFakeConn(t, *tt.info, tt.infoErr)
			}
			d := fakeDriver{engine: "test-probe", port: tt.port, open: opens(addr, conn, tt.err)}

			got := probe(context.Background(), d, driver.Config{Host: "db", SSL: driver.SSL{Mode: tt.mode}})
			if s := stages(got); s != tt.stages {
				t.Errorf("stages = %s, want %s", s, tt.stages)
			}
			if tt.code != "" {
				if got.OK || got.Error == nil || got.Error.Code != tt.code {
					t.Errorf("probe() = %+v, want a failure with code %q", got, tt.code)
				}
				return
			}
			if !got.OK || got.Error != nil || got.TLS != tt.info.TLS || got.ServerVersion != tt.info.Version {
				t.Errorf("probe() = %+v, want OK with %+v", got, *tt.info)
			}
		})
	}
}

// captureEngine records what Test hands to the driver.
const captureEngine driver.Engine = "test-capture"

var captured struct {
	sync.Mutex
	cfg driver.Config
}

func init() {
	driver.Register(fakeDriver{engine: captureEngine, port: 1, open: func(ctx context.Context, cfg driver.Config, _ driver.DialFunc) (driver.Conn, error) {
		captured.Lock()
		captured.cfg = cfg
		captured.Unlock()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("stop here")
	}})
}

func TestTestPassword(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	set := validSettings()
	set.Engine = captureEngine
	saved, err := svc.Create(ctx, set, setPw("saved-pw"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		id   string
		pw   PasswordChange
		want string
	}{
		{"saved password", saved.ID, keep, "saved-pw"},
		{"typed password", saved.ID, setPw("typed-pw"), "typed-pw"},
		{"cleared", saved.ID, clearPw, ""},
		{"new connection", "", keep, ""},
		{"nothing saved", "unsaved-id", keep, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := svc.Test(ctx, tt.id, set, tt.pw)
			if err != nil {
				t.Fatal(err)
			}
			if res.OK || res.DurationMs < 0 {
				t.Fatalf("Test() = %+v, want the failure the fake driver returns", res)
			}
			captured.Lock()
			defer captured.Unlock()
			if captured.cfg.Password != tt.want {
				t.Errorf("password given to the driver = %q, want %q", captured.cfg.Password, tt.want)
			}
			if captured.cfg.Host != "db.example.com" || captured.cfg.SSL.Mode != driver.SSLPrefer {
				t.Errorf("config = %+v, want the normalized settings", captured.cfg)
			}
		})
	}
}

func TestTestRejectsAndCancels(t *testing.T) {
	svc, _ := newTestService(t)
	set := validSettings()
	set.Engine = captureEngine

	bad := set
	bad.Host = ""
	_, err := svc.Test(context.Background(), "", bad, keep)
	wantField(t, err, "host")
	_, err = svc.Test(context.Background(), "", set, setPw(""))
	wantField(t, err, "password")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Test(ctx, "", set, keep); !errors.Is(err, context.Canceled) {
		t.Fatalf("Test() with a cancelled ctx = %v, want context.Canceled", err)
	}
}

// The real PostgreSQL adapter must hand its dialing to Test's dialer.
func TestTestPostgresReportsTheNetworkStage(t *testing.T) {
	svc, _ := newTestService(t)
	host, port, err := net.SplitHostPort(closedPort(t))
	if err != nil {
		t.Fatal(err)
	}
	set := validSettings()
	set.Host, set.SSL.Mode = host, driver.SSLDisable
	set.Port, _ = strconv.Atoi(port)

	res, err := svc.Test(context.Background(), "", set, keep)
	if err != nil {
		t.Fatal(err)
	}
	if s := stages(res); s != "tcp:failed tls:not_run auth:not_run ping:not_run" || res.Error.Code != apperr.ConnectionRefused {
		t.Fatalf("Test(closed port) = %s, %+v; want the TCP stage refused", s, res.Error)
	}

	set.Host = "querra-test.invalid" // .invalid never resolves (RFC 2606)
	res, err = svc.Test(context.Background(), "", set, keep)
	if err != nil {
		t.Fatal(err)
	}
	if s := stages(res); s != "tcp:failed tls:not_run auth:not_run ping:not_run" || res.Error.Code != apperr.DNSFailed {
		t.Fatalf("Test(unknown host) = %s, %+v; want the TCP stage failing DNS", s, res.Error)
	}
}
