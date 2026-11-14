package connection

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cokagung30/querra/internal/driver"
	"github.com/cokagung30/querra/internal/secrets"
	"github.com/cokagung30/querra/internal/store"
)

var (
	// ErrInvalid marks settings or a request that cannot be used; the error
	// is a *FieldError that says which field and why.
	ErrInvalid = errors.New("invalid connection settings")
	// ErrNotFound means the connection or group does not exist.
	ErrNotFound = store.ErrNotFound
)

// FieldError is a value the form must fix. Field is the JSON name of the
// setting, e.g. "host" or "ssl.keyFile".
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

func (e *FieldError) Unwrap() error { return ErrInvalid }

func invalid(field, format string, args ...any) error {
	return &FieldError{Field: field, Message: fmt.Sprintf(format, args...)}
}

const maxNameRunes = 200

// validate checks normalized settings.
func validate(s Settings) error {
	if err := checkName("name", s.Name); err != nil {
		return err
	}
	d, err := driver.Lookup(s.Engine)
	if err != nil {
		return invalid("engine", "unknown database engine %q", s.Engine)
	}
	switch s.Env {
	case EnvLocal, EnvStaging, EnvProduction:
	default:
		return invalid("env", "unknown environment %q", s.Env)
	}
	switch s.Color {
	case ColorDefault, ColorRed, ColorOrange, ColorYellow, ColorGreen, ColorTeal, ColorBlue, ColorPurple, ColorGray:
	default:
		return invalid("color", "unknown color %q", s.Color)
	}

	if d.DefaultPort() == 0 {
		// An engine without a server opens a file.
		if s.Database == "" {
			return invalid("database", "the database file is required")
		}
	} else if s.Host == "" {
		return invalid("host", "the host is required")
	}
	if s.Port < 0 || s.Port > 65535 {
		return invalid("port", "the port must be between 1 and 65535")
	}

	switch s.SSL.Mode {
	case driver.SSLDisable, driver.SSLPrefer, driver.SSLRequire, driver.SSLVerifyCA, driver.SSLVerifyFull:
	default:
		return invalid("ssl.mode", "unknown SSL mode %q", s.SSL.Mode)
	}
	if (s.SSL.CertFile == "") != (s.SSL.KeyFile == "") {
		field := "ssl.keyFile"
		if s.SSL.CertFile == "" {
			field = "ssl.certFile"
		}
		return invalid(field, "a client certificate needs both its certificate and its key file")
	}
	return nil
}

func checkName(field, name string) error {
	if name == "" {
		return invalid(field, "the name is required")
	}
	if utf8.RuneCountInString(name) > maxNameRunes {
		return invalid(field, "the name is longer than %d characters", maxNameRunes)
	}
	return nil
}

// checkPassword validates a change before anything is saved. The value is
// never part of the error.
func checkPassword(pw PasswordChange) error {
	switch pw.Action {
	case PasswordKeep, PasswordClear:
		return nil
	case PasswordSet:
		if pw.Value == "" {
			return invalid("password", "enter a password, or save none")
		}
		if len(pw.Value) > secrets.MaxValueBytes {
			return invalid("password", "the password is longer than %d bytes", secrets.MaxValueBytes)
		}
		return nil
	}
	return invalid("password", "unknown password action %q", pw.Action)
}

// sameName compares group names the way people read them.
func sameName(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
