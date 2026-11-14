// Package postgres is Querra's PostgreSQL adapter: pgx v5 through
// database/sql. It registers itself with the driver registry; import it for
// that side effect. The catalog queries need PostgreSQL 11 or newer.
package postgres

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgconn/ctxwatch"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/cokagung30/querra/internal/driver"
)

func init() {
	driver.Register(Driver{})
}

const (
	defaultPort     = 5432
	applicationName = "Querra"
	connectTimeout  = 10 * time.Second
	// A cancelled query gets this long to stop after the cancel request
	// before its connection is closed instead (PLAN §3.8).
	cancelDeadlineDelay = 5 * time.Second

	// The shared pool serves the grid, explorer and export of one window;
	// editor tabs have their own sessions outside it.
	maxOpenConns    = 8
	maxIdleConns    = 2
	connMaxIdleTime = 5 * time.Minute
)

// Driver is the PostgreSQL driver.
type Driver struct{}

func (Driver) Engine() driver.Engine { return driver.Postgres }

func (Driver) Capabilities() driver.Capabilities {
	return driver.Capabilities{Schemas: true, Databases: true, MaterializedViews: true, Returning: true}
}

func (Driver) Dialect() driver.Dialect { return dialect{} }

func (Driver) Codec() driver.Codec { return codec{} }

func (Driver) DefaultPort() int { return defaultPort }

// Open connects to cfg.Database. PostgreSQL connections are bound to one
// database, so browsing another one means opening another Conn.
func (Driver) Open(ctx context.Context, cfg driver.Config, dial driver.DialFunc) (driver.Conn, error) {
	connCfg, err := connConfig(cfg, dial)
	if err != nil {
		return nil, err
	}
	db := openPool(connCfg)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to postgres: %w", markErr(err))
	}
	return &conn{db: db, cfg: connCfg}, nil
}

// connConfig turns cfg into a pgx config. Settings go through pgx's libpq
// parser, so sslmode, CA, client certificate and fallback behave as in psql.
func connConfig(cfg driver.Config, dial driver.DialFunc) (*pgx.ConnConfig, error) {
	mode := cfg.SSL.Mode
	if mode == "" {
		mode = driver.SSLPrefer
	}
	switch mode {
	case driver.SSLDisable, driver.SSLPrefer, driver.SSLRequire, driver.SSLVerifyCA, driver.SSLVerifyFull:
	default:
		return nil, fmt.Errorf("unknown SSL mode %q", mode)
	}
	port := cfg.Port
	if port == 0 {
		port = defaultPort
	}

	settings := [][2]string{
		{"host", cfg.Host},
		{"port", strconv.Itoa(port)},
		{"user", cfg.User},
		{"dbname", cfg.Database},
		{"sslmode", string(mode)},
		{"application_name", applicationName},
	}
	if mode != driver.SSLDisable {
		settings = append(settings,
			[2]string{"sslrootcert", cfg.SSL.CAFile},
			[2]string{"sslcert", cfg.SSL.CertFile},
			[2]string{"sslkey", cfg.SSL.KeyFile})
	}
	connCfg, err := pgx.ParseConfig(keywordValue(settings))
	if err != nil {
		return nil, fmt.Errorf("postgres settings: %w", err)
	}

	// The password goes into the config directly, never into a connection
	// string. Setting it also overrides PGPASSWORD and ~/.pgpass, so only
	// what the user saved is used.
	connCfg.Password = cfg.Password
	connCfg.ConnectTimeout = connectTimeout
	// No statement cache: user SQL rarely repeats, cached plans break after
	// DDL, and named prepared statements fail behind PgBouncer. String
	// arguments are sent as text and typed by the server (PLAN §3.5).
	connCfg.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec
	// Cancel with a protocol cancel request, which keeps the session; pgx's
	// default closes the connection (PLAN §3.8).
	connCfg.BuildContextWatcherHandler = func(pc *pgconn.PgConn) ctxwatch.Handler {
		return &pgconn.CancelRequestContextWatcherHandler{Conn: pc, DeadlineDelay: cancelDeadlineDelay}
	}
	if dial != nil {
		connCfg.DialFunc = pgconn.DialFunc(dial)
		// A tunnel resolves the host name at its far end.
		connCfg.LookupFunc = func(_ context.Context, host string) ([]string, error) {
			return []string{host}, nil
		}
	}
	return connCfg, nil
}

// keywordValue writes libpq keyword/value settings. Every value is quoted, so
// spaces, quotes and backslashes in names and file paths survive. Empty values
// are left out and take libpq's defaults.
func keywordValue(settings [][2]string) string {
	escape := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	var b strings.Builder
	for _, s := range settings {
		if s[1] == "" {
			continue
		}
		fmt.Fprintf(&b, "%s='%s' ", s[0], escape.Replace(s[1]))
	}
	return b.String()
}

func openPool(cfg *pgx.ConnConfig) *sql.DB {
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxIdleTime(connMaxIdleTime)
	return db
}

type conn struct {
	db  *sql.DB
	cfg *pgx.ConnConfig
}

func (c *conn) Pool() *sql.DB { return c.db }

func (c *conn) Introspector() driver.Introspector { return introspector{db: c.db} }

func (c *conn) Close() error { return c.db.Close() }

// ServerInfo reads the version the server reported at startup and whether a
// pooled connection uses TLS; with sslmode=prefer it may not.
func (c *conn) ServerInfo(ctx context.Context) (driver.ServerInfo, error) {
	sc, err := c.db.Conn(ctx)
	if err != nil {
		return driver.ServerInfo{}, fmt.Errorf("postgres server info: %w", markErr(err))
	}
	defer func() { _ = sc.Close() }()

	var info driver.ServerInfo
	err = sc.Raw(func(dc any) error {
		pc, ok := dc.(*stdlib.Conn)
		if !ok {
			return fmt.Errorf("unexpected driver connection %T", dc)
		}
		pg := pc.Conn().PgConn()
		info.Version = pg.ParameterStatus("server_version")
		_, info.TLS = pg.Conn().(*tls.Conn)
		return nil
	})
	if err != nil {
		return driver.ServerInfo{}, fmt.Errorf("postgres server info: %w", err)
	}
	return info, nil
}

// SQLSTATEs a failed connection attempt can end with.
const (
	codeInvalidPassword       = "28P01"
	codeInvalidAuthorization  = "28000" // unknown role, or no pg_hba.conf entry
	codeInvalidCatalogName    = "3D000" // no such database
	codeInsufficientPrivilege = "42501" // no CONNECT privilege
)

// markErr marks a connection error with the driver errors it shows (PLAN
// §3.9). With sslmode=prefer, pgx joins the error of the TLS attempt with that
// of the plain one, so one error can carry both a refused TLS and a failed
// login.
func markErr(err error) error {
	var marks []error
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch pgErr.Code {
		case codeInvalidPassword, codeInvalidAuthorization:
			marks = append(marks, driver.ErrAuthFailed)
		case codeInvalidCatalogName:
			marks = append(marks, driver.ErrDatabaseNotFound)
		case codeInsufficientPrivilege:
			marks = append(marks, driver.ErrPermissionDenied)
		}
	}
	// pgconn reports this with errors.New, so its text is all there is.
	if strings.Contains(err.Error(), "server refused TLS connection") {
		marks = append(marks, driver.ErrTLSRefused)
	}
	return driver.Mark(err, marks...)
}

// NewSession gives the session a one-connection pool of its own, so its
// server state is never shared and its connection never returns to the
// shared pool.
func (c *conn) NewSession(ctx context.Context) (driver.Session, error) {
	db := stdlib.OpenDB(*c.cfg)
	db.SetMaxOpenConns(1)
	sc, err := db.Conn(ctx)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open postgres session: %w", markErr(err))
	}
	return &session{db: db, conn: sc}, nil
}

type session struct {
	db   *sql.DB
	conn *sql.Conn
}

func (s *session) Query(ctx context.Context, stmt string) (*sql.Rows, error) {
	return s.conn.QueryContext(ctx, stmt)
}

func (s *session) Exec(ctx context.Context, stmt string) (sql.Result, error) {
	return s.conn.ExecContext(ctx, stmt)
}

func (s *session) Close() error {
	return errors.Join(s.conn.Close(), s.db.Close())
}

type dialect struct{}

func (dialect) QuoteIdent(name string) string {
	// PostgreSQL identifiers cannot hold NUL; drop it as pgx.Identifier does.
	name = strings.ReplaceAll(name, "\x00", "")
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func (dialect) Placeholder(n int) string { return "$" + strconv.Itoa(n) }

func (dialect) ReadOnlySessionSQL() string {
	return "SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY"
}
