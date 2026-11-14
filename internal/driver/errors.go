package driver

import "errors"

// Connection failures an adapter recognizes in its engine's errors. Open
// marks its error with every one that applies (see Mark), and apperr maps
// them to stable codes (PLAN §3.9). Network, DNS, timeout and certificate
// errors need no mark: they are the same standard Go errors for every
// engine.
var (
	// ErrAuthFailed means the server rejected the user name or password.
	ErrAuthFailed = errors.New("authentication failed")
	// ErrDatabaseNotFound means the server has no database by that name.
	ErrDatabaseNotFound = errors.New("database does not exist")
	// ErrPermissionDenied means the user may not connect to the database.
	ErrPermissionDenied = errors.New("permission denied")
	// ErrTLSRefused means the server does not accept TLS connections.
	ErrTLSRefused = errors.New("server does not support TLS")
)

// Mark returns err marked with the given errors, so errors.Is finds each of
// them while the message stays err's own. It returns err unchanged when
// there is nothing to add.
func Mark(err error, marks ...error) error {
	if err == nil || len(marks) == 0 {
		return err
	}
	return &markedError{err: err, marks: marks}
}

type markedError struct {
	err   error
	marks []error
}

func (e *markedError) Error() string { return e.err.Error() }

func (e *markedError) Unwrap() []error {
	return append([]error{e.err}, e.marks...)
}
