// Package apperr is the structured error the frontend receives (PLAN §3.9).
// Services return *Error with a stable Code; the frontend shows the
// translation for that code (src/i18n/en.json, key errors.<code>) and falls
// back to Message. Messages and details never contain secrets.
package apperr

import (
	"context"
	"encoding/json"
	"errors"
)

// Code identifies a kind of error. Codes are stable: the frontend translates
// and branches on them. Add a code here and its errors.<code> key in
// frontend/src/i18n/en.json in the same change (TestEveryCodeIsTranslated).
type Code string

const (
	// Internal is a bug or an error nobody anticipated.
	Internal Code = "internal"
	// InvalidInput means a request carried values that cannot be used.
	InvalidInput Code = "invalid_input"
	// InvalidFile means an imported file is unreadable or from a newer Querra.
	InvalidFile Code = "invalid_file"
	// NotFound means the record the request names does not exist.
	NotFound Code = "not_found"
	// Cancelled means the user cancelled the call.
	Cancelled Code = "cancelled"

	// Reaching the server.
	DNSFailed         Code = "dns_failed"
	ConnectionRefused Code = "connection_refused"
	NetworkError      Code = "network_error"
	Timeout           Code = "timeout"

	// TLS.
	TLSRequired     Code = "tls_required"
	TLSVerifyFailed Code = "tls_verify_failed"
	TLSFailed       Code = "tls_failed"

	// Logging in.
	AuthFailed       Code = "auth_failed"
	DatabaseNotFound Code = "database_not_found"
	PermissionDenied Code = "permission_denied"
)

// Codes returns every code.
func Codes() []Code {
	return []Code{
		Internal, InvalidInput, InvalidFile, NotFound, Cancelled,
		DNSFailed, ConnectionRefused, NetworkError, Timeout,
		TLSRequired, TLSVerifyFailed, TLSFailed,
		AuthFailed, DatabaseNotFound, PermissionDenied,
	}
}

// Error is the error a bound method returns. Wails serializes its exported
// fields, and the frontend receives them as error.cause.
type Error struct {
	Code Code `json:"code"`
	// Message is a short English sentence, shown when the frontend has no
	// translation for Code.
	Message string `json:"message"`
	// Detail is the underlying error text, e.g. the server's own message.
	Detail string `json:"detail,omitempty"`
	// Hint is the server's suggestion, when it gave one.
	Hint string `json:"hint,omitempty"`
	// Position is the 1-based character in the SQL where the error is, or 0.
	Position int `json:"position,omitempty"`
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return e.Message
	}
	return e.Message + ": " + e.Detail
}

// New returns an error with the given code and message.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// From turns any error into an *Error: an *Error inside err is returned as is,
// a cancelled context becomes Cancelled, errors from reaching or logging in to
// a database get their connection code (FromConnect), and anything else is
// Internal with err's text as the detail.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := errors.AsType[*Error](err); ok {
		return e
	}
	if errors.Is(err, context.Canceled) {
		return &Error{Code: Cancelled, Message: messages[Cancelled]}
	}
	return FromConnect(err)
}

// Marshal is the MarshalError hook for Wails services: it sends every error
// to the frontend as an *Error.
func Marshal(err error) []byte {
	b, jsonErr := json.Marshal(From(err))
	if jsonErr != nil {
		return nil // Wails falls back to its default
	}
	return b
}

// messages are the fallback messages; the frontend's translations win.
var messages = map[Code]string{
	Internal:          "Something went wrong",
	InvalidInput:      "Some values are not valid",
	InvalidFile:       "The file cannot be imported",
	NotFound:          "It no longer exists",
	Cancelled:         "Cancelled",
	DNSFailed:         "The host name could not be found",
	ConnectionRefused: "The server refused the connection",
	NetworkError:      "The server could not be reached",
	Timeout:           "The server did not answer in time",
	TLSRequired:       "The server does not support TLS",
	TLSVerifyFailed:   "The server's certificate could not be verified",
	TLSFailed:         "The TLS connection failed",
	AuthFailed:        "The user name or password was not accepted",
	DatabaseNotFound:  "The database does not exist",
	PermissionDenied:  "Permission denied",
}
