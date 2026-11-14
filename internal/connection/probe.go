package connection

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cokagung30/querra/internal/apperr"
	"github.com/cokagung30/querra/internal/driver"
	"github.com/cokagung30/querra/internal/secrets"
)

// Stage is one step of Test Connection, in the order they run (PLAN §3.9).
// The SSH stage joins with SSH tunnels in M2.7.
type Stage string

const (
	StageTCP  Stage = "tcp"
	StageTLS  Stage = "tls"
	StageAuth Stage = "auth"
	StagePing Stage = "ping"
)

// StageStatus is how far a stage got.
type StageStatus string

const (
	StageOK     StageStatus = "ok"
	StageFailed StageStatus = "failed"
	// StageSkipped means the stage does not apply, e.g. TLS with SSL mode
	// disable, or a server that declined TLS under prefer.
	StageSkipped StageStatus = "skipped"
	// StageNotRun means an earlier stage failed.
	StageNotRun StageStatus = "not_run"
)

// StageResult is one stage's outcome.
type StageResult struct {
	Stage  Stage       `json:"stage"`
	Status StageStatus `json:"status"`
}

// TestResult is what Test Connection reports (FR-04).
type TestResult struct {
	OK     bool          `json:"ok"`
	Stages []StageResult `json:"stages"`
	// Error is why the failed stage failed; nil when OK.
	Error *apperr.Error `json:"error,omitempty"`
	// ServerVersion is the server's version string when OK.
	ServerVersion string `json:"serverVersion,omitempty"`
	// TLS means the tested connection was encrypted.
	TLS        bool  `json:"tls"`
	DurationMs int64 `json:"durationMs"`
}

// Test tries the connection set describes, stage by stage, the way opening it
// would. id names the saved connection whose password a PasswordKeep uses
// ("" for a connection not saved yet).
//
// A connection that fails is a TestResult with OK false. Test returns an
// error only for settings that cannot be tested, a keychain failure, or a
// cancelled ctx.
func (s *Service) Test(ctx context.Context, id string, set Settings, pw PasswordChange) (TestResult, error) {
	set = normalize(set)
	if err := validate(set); err != nil {
		return TestResult{}, err
	}
	if err := checkPassword(pw); err != nil {
		return TestResult{}, err
	}
	password, err := s.testPassword(id, pw)
	if err != nil {
		return TestResult{}, err
	}
	d, err := driver.Lookup(set.Engine)
	if err != nil {
		return TestResult{}, err
	}

	start := time.Now()
	res := probe(ctx, d, set.driverConfig(password))
	if err := ctx.Err(); err != nil {
		return TestResult{}, err
	}
	res.DurationMs = time.Since(start).Milliseconds()
	return res, nil
}

func (s *Service) testPassword(id string, pw PasswordChange) (string, error) {
	switch {
	case pw.Action == PasswordSet:
		return pw.Value, nil
	case pw.Action == PasswordClear || id == "":
		return "", nil
	}
	v, err := s.secrets.Get(id, secrets.Password)
	if errors.Is(err, secrets.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read saved password: %w", err)
	}
	return v, nil
}

// probe opens cfg and reports how far it got.
func probe(ctx context.Context, d driver.Driver, cfg driver.Config) TestResult {
	network := d.DefaultPort() != 0
	tlsOn := network && cfg.SSL.Mode != driver.SSLDisable
	res := newResult(network)

	dialer := &probeDialer{}
	conn, err := d.Open(ctx, cfg, dialer.dial)
	if err != nil {
		appErr := apperr.FromConnect(err)
		attempted, connected := dialer.state()
		stage := failedStage(appErr.Code, network, attempted, connected)
		res.fail(stage, appErr)
		switch {
		case stage == StageTLS && !attempted:
			// The certificate files are read before connecting.
			res.set(StageTCP, StageNotRun)
		case stage == StageAuth && network && (!tlsOn || errors.Is(err, driver.ErrTLSRefused)):
			// The login was tried without TLS.
			res.set(StageTLS, StageSkipped)
		}
		return res
	}
	defer func() { _ = conn.Close() }()

	if network {
		res.set(StageTCP, StageOK)
		res.set(StageTLS, StageSkipped)
		if tlsOn {
			res.set(StageTLS, StageOK)
		}
	}
	res.set(StageAuth, StageOK)

	info, err := conn.ServerInfo(ctx)
	if err == nil {
		if !info.TLS && res.status(StageTLS) == StageOK {
			res.set(StageTLS, StageSkipped) // prefer, and the server declined TLS
		}
		err = conn.Pool().PingContext(ctx)
	}
	if err != nil {
		res.fail(StagePing, apperr.FromConnect(err))
		return res
	}
	res.set(StagePing, StageOK)
	res.OK, res.ServerVersion, res.TLS = true, info.Version, info.TLS
	return res
}

// failedStage says where a failed Open stopped. An engine without a network
// server has only the login stage.
func failedStage(code apperr.Code, network, attempted, connected bool) Stage {
	tlsCode := code == apperr.TLSRequired || code == apperr.TLSVerifyFailed || code == apperr.TLSFailed
	switch {
	case !network:
		return StageAuth
	case !attempted:
		// It stopped before dialing: unreadable certificate files, or
		// settings the driver rejected.
		if tlsCode {
			return StageTLS
		}
		return StageTCP
	case !connected:
		return StageTCP
	case tlsCode:
		return StageTLS
	}
	// The server answered: a refused login, or anything after it.
	return StageAuth
}

func newResult(network bool) TestResult {
	res := TestResult{Stages: []StageResult{
		{StageTCP, StageNotRun}, {StageTLS, StageNotRun}, {StageAuth, StageNotRun}, {StagePing, StageNotRun},
	}}
	if !network {
		res.set(StageTCP, StageSkipped)
		res.set(StageTLS, StageSkipped)
	}
	return res
}

func (r *TestResult) set(stage Stage, status StageStatus) {
	for i := range r.Stages {
		if r.Stages[i].Stage == stage {
			r.Stages[i].Status = status
		}
	}
}

func (r *TestResult) status(stage Stage) StageStatus {
	for _, s := range r.Stages {
		if s.Stage == stage {
			return s.Status
		}
	}
	return StageNotRun
}

// fail marks stage failed and every earlier stage that ran as passed; later
// stages stay not run.
func (r *TestResult) fail(stage Stage, err *apperr.Error) {
	for i := range r.Stages {
		if r.Stages[i].Stage == stage {
			r.Stages[i].Status = StageFailed
			break
		}
		if r.Stages[i].Status == StageNotRun {
			r.Stages[i].Status = StageOK
		}
	}
	r.Error = err
}

// probeDialer dials like the driver would, and remembers whether the server
// was reached, to tell a network failure from a refused login.
type probeDialer struct {
	mu                   sync.Mutex
	attempted, connected bool
}

func (p *probeDialer) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, network, addr)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attempted = true
	p.connected = p.connected || err == nil
	return c, err
}

func (p *probeDialer) state() (attempted, connected bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.attempted, p.connected
}
