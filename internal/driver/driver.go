// Package driver is the contract between Querra and a database engine
// (PLAN §3.3). Each engine lives in its own subpackage and registers a Driver;
// everything else reaches databases only through these interfaces, and the UI
// branches on Capabilities, never on the engine name.
package driver

import (
	"context"
	"database/sql"
	"errors"
	"net"
)

// Engine names a database engine. It is stored with saved connections.
type Engine string

const (
	Postgres Engine = "postgres"
	MySQL    Engine = "mysql" // also MariaDB
	SQLite   Engine = "sqlite"
)

// ErrNotFound is returned when a database object does not exist.
var ErrNotFound = errors.New("not found")

// Driver is one engine's entry point.
type Driver interface {
	Engine() Engine
	Capabilities() Capabilities
	Dialect() Dialect
	Codec() Codec
	// DefaultPort is the port Open uses when Config.Port is 0, or 0 for an
	// engine without a network server.
	DefaultPort() int
	// Open connects to cfg's server and database and checks that the
	// connection works. A non-nil dial replaces the network dialer, e.g. with
	// an SSH tunnel (M2.7); the host name then reaches it unresolved. Errors
	// are marked with ErrAuthFailed and the like where they apply (Mark).
	Open(ctx context.Context, cfg Config, dial DialFunc) (Conn, error)
}

// Capabilities tells the UI what an engine supports.
type Capabilities struct {
	// Schemas means objects are grouped in schemas inside a database.
	Schemas bool `json:"schemas"`
	// Databases means one server holds several databases worth listing.
	Databases         bool `json:"databases"`
	MaterializedViews bool `json:"materializedViews"`
	// Returning means INSERT, UPDATE and DELETE support RETURNING.
	Returning bool `json:"returning"`
}

// Config is what Open needs to reach one database. Password comes from the
// keychain; adapters keep it out of connection strings, errors and logs.
type Config struct {
	Host     string
	Port     int // 0 means the engine's default port
	User     string
	Password string
	Database string
	SSL      SSL
}

// SSLMode follows libpq's sslmode values.
type SSLMode string

const (
	SSLDisable SSLMode = "disable"
	// SSLPrefer uses TLS when the server offers it, without verifying the
	// certificate, and falls back to a plain connection.
	SSLPrefer SSLMode = "prefer"
	// SSLRequire insists on TLS but, like libpq, verifies the certificate
	// only when a CA file is set (then it acts as SSLVerifyCA).
	SSLRequire SSLMode = "require"
	// SSLVerifyCA checks that a trusted CA signed the server certificate.
	SSLVerifyCA SSLMode = "verify-ca"
	// SSLVerifyFull also checks that the certificate matches the host name.
	SSLVerifyFull SSLMode = "verify-full"
)

// SSL configures TLS. The files are paths on disk; their contents are never
// stored.
type SSL struct {
	Mode     SSLMode // "" means SSLPrefer
	CAFile   string
	CertFile string
	KeyFile  string
}

// DialFunc opens the network connection to a database server.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// Conn is one open target: a server and one of its databases.
type Conn interface {
	// Pool serves the grid, the explorer and export.
	Pool() *sql.DB
	// NewSession opens a server connection of its own for one editor tab.
	NewSession(ctx context.Context) (Session, error)
	Introspector() Introspector
	// ServerInfo describes the server and how the pool reaches it.
	ServerInfo(ctx context.Context) (ServerInfo, error)
	Close() error
}

// ServerInfo is what Test Connection reports about a server.
type ServerInfo struct {
	// Version is the server's own version string, e.g. "17.2".
	Version string
	// TLS means the connection is encrypted.
	TLS bool
}

// Session is one editor tab's own server connection, so transactions, SET and
// temporary tables carry over between statements. A cancelled ctx stops the
// running statement and leaves the session usable (PLAN §3.8). Close ends the
// server session; its connection never serves anyone else.
type Session interface {
	Query(ctx context.Context, stmt string) (*sql.Rows, error)
	Exec(ctx context.Context, stmt string) (sql.Result, error)
	Close() error
}

// Introspector reads the structure of the open database.
type Introspector interface {
	// Databases lists the databases on the server that accept connections.
	Databases(ctx context.Context) ([]Database, error)
	Schemas(ctx context.Context) ([]Schema, error)
	// Objects lists a schema's tables, views, materialized views and
	// routines, by name.
	Objects(ctx context.Context, schema string) ([]Object, error)
	// Table describes a table or view, or returns ErrNotFound.
	Table(ctx context.Context, ref TableRef) (*TableInfo, error)
	// EstimateRows returns the planner's row estimate, -1 when the engine
	// has no statistics for the table yet, or ErrNotFound.
	EstimateRows(ctx context.Context, ref TableRef) (int64, error)
}

// Dialect writes the SQL Querra generates for an engine. BuildSelect (M1.7)
// and BuildChange (M3.2) join it with the milestones that need them.
type Dialect interface {
	QuoteIdent(name string) string
	// Placeholder returns the bind parameter for the n-th argument, from 1.
	Placeholder(n int) string
	// ReadOnlySessionSQL makes the session it runs in read-only (PLAN §3.7).
	ReadOnlySessionSQL() string
}
