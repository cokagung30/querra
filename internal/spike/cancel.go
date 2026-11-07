package spike

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql" // registers the "mysql" driver
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgconn/ctxwatch"
	"github.com/jackc/pgx/v5/stdlib"
)

// CancelResult reports how a long server-side sleep behaved when the caller's
// context was cancelled.
type CancelResult struct {
	Engine    string `json:"engine"`
	ElapsedMs int64  `json:"elapsedMs"`
	Err       string `json:"err,omitempty"`
	// StillRunning is whether the server still showed the sleep as running
	// half a second after the call returned.
	StillRunning bool `json:"stillRunning"`
	// SessionUsable is whether the same pinned connection could run another
	// query afterwards (it matters for editor tabs, PLAN §3.8).
	SessionUsable bool   `json:"sessionUsable"`
	CheckErr      string `json:"checkErr,omitempty"`
}

type sleepProbe struct {
	open       func(dsn string) (*sql.DB, error)
	dsnEnv     string
	defaultDSN string
	sleep      string
	// sessionID returns the server-side id of the pinned connection; running
	// counts that session's sleeps. Checking one session keeps a sleep left
	// over from an earlier probe from skewing the result.
	sessionID string
	running   string
	// killQuery cancels with KILL QUERY from another connection instead of
	// handing the caller's context to the driver (the MySQL plan, PLAN §3.8).
	killQuery bool
}

const (
	postgresDSN = "postgres://querra_test:querra_test@127.0.0.1:5432/querra_test?sslmode=disable"
	mysqlDSN    = "querra_test:querra_test@tcp(127.0.0.1:3306)/querra_test"

	postgresSleep     = "SELECT pg_sleep(30)"
	postgresSessionID = "SELECT pg_backend_pid()"
	postgresRunning   = `SELECT count(*) FROM pg_stat_activity
		WHERE pid = $1 AND state = 'active' AND query LIKE 'SELECT pg_sleep%'`
	mysqlSleep     = "SELECT SLEEP(30)"
	mysqlSessionID = "SELECT CONNECTION_ID()"
	mysqlRunning   = "SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE ID = ? AND INFO LIKE 'SELECT SLEEP%'"
)

// Probe names are the engine plus the cancellation strategy under test.
// Defaults match the querra_test databases created by dev/testdb.sh.
var sleepProbes = map[string]sleepProbe{
	// pgx's default handler: on cancel it sets a deadline on the socket.
	"postgres": {
		open:       func(dsn string) (*sql.DB, error) { return sql.Open("pgx", dsn) },
		dsnEnv:     "QUERRA_SPIKE_POSTGRES_DSN",
		defaultDSN: postgresDSN,
		sleep:      postgresSleep,
		sessionID:  postgresSessionID,
		running:    postgresRunning,
	},
	// pgx with a protocol cancel request, which should keep the session.
	"postgres-cancel-request": {
		open:       openPostgresWithCancelRequest,
		dsnEnv:     "QUERRA_SPIKE_POSTGRES_DSN",
		defaultDSN: postgresDSN,
		sleep:      postgresSleep,
		sessionID:  postgresSessionID,
		running:    postgresRunning,
	},
	// go-sql-driver/mysql's default: on cancel it closes the connection.
	"mysql": {
		open:       func(dsn string) (*sql.DB, error) { return sql.Open("mysql", dsn) },
		dsnEnv:     "QUERRA_SPIKE_MYSQL_DSN",
		defaultDSN: mysqlDSN,
		sleep:      mysqlSleep,
		sessionID:  mysqlSessionID,
		running:    mysqlRunning,
	},
	"mysql-kill-query": {
		open:       func(dsn string) (*sql.DB, error) { return sql.Open("mysql", dsn) },
		dsnEnv:     "QUERRA_SPIKE_MYSQL_DSN",
		defaultDSN: mysqlDSN,
		sleep:      mysqlSleep,
		sessionID:  mysqlSessionID,
		running:    mysqlRunning,
		killQuery:  true,
	},
}

func openPostgresWithCancelRequest(dsn string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.BuildContextWatcherHandler = func(conn *pgconn.PgConn) ctxwatch.Handler {
		// The deadline is only a fallback for a server that ignores the cancel.
		return &pgconn.CancelRequestContextWatcherHandler{Conn: conn, DeadlineDelay: 5 * time.Second}
	}
	return stdlib.OpenDB(*cfg), nil
}

// SleepAndObserve runs a 30-second server-side sleep on a pinned connection
// until ctx is cancelled, then checks whether the server stopped the query and
// whether the connection is still usable.
func SleepAndObserve(ctx context.Context, engine string) CancelResult {
	res := CancelResult{Engine: engine}
	probe, ok := sleepProbes[engine]
	if !ok {
		res.Err = "unknown engine: " + engine
		return res
	}
	dsn := os.Getenv(probe.dsnEnv)
	if dsn == "" {
		dsn = probe.defaultDSN
	}

	db, err := probe.open(dsn)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	defer func() { _ = db.Close() }()

	checkCtx, cancelCheck := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancelCheck()
	conn, err := db.Conn(checkCtx)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	defer func() { _ = conn.Close() }()

	var sessionID int64
	if err := conn.QueryRowContext(checkCtx, probe.sessionID).Scan(&sessionID); err != nil {
		res.Err = fmt.Sprintf("session id: %v", err)
		return res
	}

	start := time.Now()
	if probe.killQuery {
		err = sleepWithKillQuery(ctx, checkCtx, db, conn, sessionID, probe.sleep)
	} else {
		var ignored any
		err = conn.QueryRowContext(ctx, probe.sleep).Scan(&ignored)
	}
	res.ElapsedMs = time.Since(start).Milliseconds()
	if err != nil {
		res.Err = err.Error()
	}

	// Give the server a moment to act on the cancellation before looking.
	time.Sleep(500 * time.Millisecond)
	var running int
	if err := db.QueryRowContext(checkCtx, probe.running, sessionID).Scan(&running); err != nil {
		res.CheckErr = err.Error()
	}
	res.StillRunning = running > 0

	var one int
	res.SessionUsable = conn.QueryRowContext(checkCtx, "SELECT 1").Scan(&one) == nil
	return res
}

// sleepWithKillQuery runs query on conn (server thread threadID) without
// giving the driver ctx. When ctx is cancelled it sends KILL QUERY from another
// pool connection, so the query stops on the server and conn stays open.
func sleepWithKillQuery(ctx, background context.Context, db *sql.DB, conn *sql.Conn, threadID int64, query string) error {
	done := make(chan error, 1)
	go func() {
		var ignored any
		done <- conn.QueryRowContext(background, query).Scan(&ignored)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_, killErr := db.ExecContext(background, fmt.Sprintf("KILL QUERY %d", threadID))
		queryErr := <-done
		// An interrupted SLEEP() returns 1 instead of failing; other queries
		// fail with error 1317. Either way the caller sees the cancellation.
		return errors.Join(ctx.Err(), killErr, queryErr)
	}
}
