// Package secrets keeps connection secrets (database passwords, SSH passwords,
// and SSH key passphrases) in the OS keychain: the Keychain on macOS and the
// Credential Manager on Windows (PLAN §3.6). Nothing else in Querra stores
// them: not the local database, logs, exports, or error messages.
//
// Each secret is a keychain entry with service ServiceName and account
// conn/<connection id>/<kind>.
package secrets

import (
	"errors"
	"fmt"
	"strings"
)

// ServiceName is the keychain service every Querra secret is stored under.
const ServiceName = "com.balikaryadigital.querra"

// Kind names a secret of a connection.
type Kind string

const (
	Password      Kind = "password"       // database password
	SSHPassword   Kind = "ssh-password"   // SSH password authentication
	SSHPassphrase Kind = "ssh-passphrase" // passphrase of the SSH private key
)

// Kinds returns every kind of secret a connection can have.
func Kinds() []Kind {
	return []Kind{Password, SSHPassword, SSHPassphrase}
}

// MaxValueBytes bounds a secret. Windows Credential Manager refuses values
// above 2,560 bytes, and passwords never come close; the lower limit gives
// both platforms (and the fake) the same behaviour.
const MaxValueBytes = 1024

var (
	// ErrNotFound means the connection has no secret of that kind.
	ErrNotFound = errors.New("secret not found")
	// ErrInvalid means a connection id, kind, or value cannot be stored.
	ErrInvalid = errors.New("invalid secret")
)

// Store reads and writes connection secrets. Implementations must never log
// or wrap secret values into errors.
type Store interface {
	// Get returns the secret, or ErrNotFound.
	Get(connID string, kind Kind) (string, error)
	// Set stores or replaces the secret.
	Set(connID string, kind Kind, value string) error
	// Delete removes the secret; removing a missing secret is not an error.
	Delete(connID string, kind Kind) error
	// DeleteAll removes every secret of the connection.
	DeleteAll(connID string) error
}

// Has reports whether the connection has a secret of the given kind. The UI
// gets only this flag, never the value (CLAUDE.md, "Secrets and privacy").
func Has(s Store, connID string, kind Kind) (bool, error) {
	_, err := s.Get(connID, kind)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, ErrNotFound):
		return false, nil
	default:
		return false, err
	}
}

// account returns the keychain account for a secret after validating its
// parts.
func account(connID string, kind Kind) (string, error) {
	if connID == "" || strings.ContainsAny(connID, "/\\") {
		return "", fmt.Errorf("%w: connection id %q", ErrInvalid, connID)
	}
	if !validKind(kind) {
		return "", fmt.Errorf("%w: kind %q", ErrInvalid, kind)
	}
	return "conn/" + connID + "/" + string(kind), nil
}

func validKind(kind Kind) bool {
	switch kind {
	case Password, SSHPassword, SSHPassphrase:
		return true
	}
	return false
}

// checkValue rejects values the keychain cannot hold. The error says why, but
// never includes the value.
func checkValue(value string) error {
	if value == "" {
		return fmt.Errorf("%w: empty value (use Delete to remove a secret)", ErrInvalid)
	}
	if len(value) > MaxValueBytes {
		return fmt.Errorf("%w: value is %d bytes, the limit is %d", ErrInvalid, len(value), MaxValueBytes)
	}
	return nil
}
