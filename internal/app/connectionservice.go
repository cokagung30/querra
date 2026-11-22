package app

import (
	"context"
	"errors"

	"github.com/cokagung30/querra/internal/apperr"
	"github.com/cokagung30/querra/internal/connection"
	"github.com/cokagung30/querra/internal/secrets"
	"github.com/cokagung30/querra/internal/store"
)

// ConnectionService exposes saved connections, their groups, Test Connection,
// and export and import (PLAN M1.3). It forwards to connection.Service and
// turns its errors into *apperr.Error. Passwords go in through
// connection.PasswordChange and never come back out; the frontend only sees
// connection.PasswordState.
type ConnectionService struct {
	store *store.Store
	conns *connection.Service
}

// NewConnectionService returns a ConnectionService over an open store, which
// it closes on shutdown, and a secrets store (the OS keychain in the app).
func NewConnectionService(st *store.Store, sec secrets.Store) *ConnectionService {
	return &ConnectionService{store: st, conns: connection.NewService(st, sec)}
}

// ServiceShutdown closes the local store when the app quits.
func (s *ConnectionService) ServiceShutdown() error {
	return s.store.Close()
}

// List returns every connection, sorted by sort order, then name.
func (s *ConnectionService) List(ctx context.Context) ([]connection.Connection, error) {
	return result(s.conns.List(ctx))
}

// Get returns one connection.
func (s *ConnectionService) Get(ctx context.Context, id string) (connection.Connection, error) {
	return result(s.conns.Get(ctx, id))
}

// Create saves a new connection at the end of its group, and its password
// when password sets one.
func (s *ConnectionService) Create(ctx context.Context, settings connection.Settings, password connection.PasswordChange) (connection.Connection, error) {
	return result(s.conns.Create(ctx, settings, password))
}

// Update saves new settings for a connection and applies the password change.
func (s *ConnectionService) Update(ctx context.Context, id string, settings connection.Settings, password connection.PasswordChange) (connection.Connection, error) {
	return result(s.conns.Update(ctx, id, settings, password))
}

// Delete removes a connection and every secret it has.
func (s *ConnectionService) Delete(ctx context.Context, id string) error {
	return toAppErr(s.conns.Delete(ctx, id))
}

// Duplicate copies a connection, its secrets included, under the given name,
// right after the original.
func (s *ConnectionService) Duplicate(ctx context.Context, id, name string) (connection.Connection, error) {
	return result(s.conns.Duplicate(ctx, id, name))
}

// Arrange puts the connections ids, in that order, into group groupID (""
// for no group). Connections not listed keep their place.
func (s *ConnectionService) Arrange(ctx context.Context, groupID string, ids []string) error {
	return toAppErr(s.conns.Arrange(ctx, groupID, ids))
}

// Groups returns every group in order.
func (s *ConnectionService) Groups(ctx context.Context) ([]connection.Group, error) {
	return result(s.conns.Groups(ctx))
}

// CreateGroup adds a group at the end. Names are unique, ignoring case.
func (s *ConnectionService) CreateGroup(ctx context.Context, name string) (connection.Group, error) {
	return result(s.conns.CreateGroup(ctx, name))
}

// RenameGroup renames a group.
func (s *ConnectionService) RenameGroup(ctx context.Context, id, name string) (connection.Group, error) {
	return result(s.conns.RenameGroup(ctx, id, name))
}

// DeleteGroup removes a group. Its connections stay, without a group.
func (s *ConnectionService) DeleteGroup(ctx context.Context, id string) error {
	return toAppErr(s.conns.DeleteGroup(ctx, id))
}

// ArrangeGroups sets the order of the groups.
func (s *ConnectionService) ArrangeGroups(ctx context.Context, ids []string) error {
	return toAppErr(s.conns.ArrangeGroups(ctx, ids))
}

// Test tries a connection stage by stage. id is the saved connection whose
// password a "keep" uses ("" for one not saved yet). A connection that fails
// resolves with ok false; the call itself fails only for settings that
// cannot be tested, a keychain failure, or a cancelled call.
func (s *ConnectionService) Test(ctx context.Context, id string, settings connection.Settings, password connection.PasswordChange) (connection.TestResult, error) {
	return result(s.conns.Test(ctx, id, settings, password))
}

// Export returns the connections ids (all of them when ids is empty) as a
// JSON file without secrets. In TypeScript the bytes arrive base64-encoded.
func (s *ConnectionService) Export(ctx context.Context, ids []string) ([]byte, error) {
	return result(s.conns.Export(ctx, ids))
}

// Import adds every connection in an export file as a new connection. In
// TypeScript, data is the file's bytes base64-encoded.
func (s *ConnectionService) Import(ctx context.Context, data []byte) (connection.ImportResult, error) {
	return result(s.conns.Import(ctx, data))
}

// Engines lists the engines Querra can connect to, for the connection form.
func (s *ConnectionService) Engines() []connection.EngineInfo {
	return connection.Engines()
}

func result[T any](v T, err error) (T, error) {
	if err != nil {
		var zero T
		return zero, toAppErr(err)
	}
	return v, nil
}

// toAppErr turns an error from connection.Service into the *apperr.Error the
// frontend receives as error.cause. It returns a nil error for nil, never a
// nil *apperr.Error.
func toAppErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, connection.ErrInvalidFile):
		// Checked before ErrInvalid, which a bad connection in the file
		// also wraps. The text says what is wrong and holds no secrets:
		// export files have none.
		return apperr.WithDetail(apperr.InvalidFile, err.Error())
	case errors.Is(err, connection.ErrInvalid):
		e := apperr.WithDetail(apperr.InvalidInput, "")
		if fe, ok := errors.AsType[*connection.FieldError](err); ok {
			// Detail names the field for the form; Message says why.
			e.Message, e.Detail = fe.Message, fe.Field
		}
		return e
	case errors.Is(err, connection.ErrNotFound):
		return apperr.WithDetail(apperr.NotFound, err.Error())
	}
	return apperr.From(err)
}
