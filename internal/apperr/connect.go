package apperr

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/fs"
	"net"
	"os"
	"syscall"

	"github.com/cokagung30/querra/internal/driver"
)

// FromConnect classifies an error from reaching or logging in to a database
// server: DNS, network and TLS failures are standard Go errors, and adapters
// mark what the server refused (driver.ErrAuthFailed and the others). Anything
// else is Internal. The detail is err's text, which adapters keep free of
// passwords.
func FromConnect(err error) *Error {
	code := connectCode(err)
	return &Error{Code: code, Message: messages[code], Detail: err.Error()}
}

func connectCode(err error) Code {
	switch {
	// A refused login means the server was reached, even if an earlier
	// attempt failed (sslmode=prefer tries TLS first), so these come first.
	case errors.Is(err, driver.ErrAuthFailed):
		return AuthFailed
	case errors.Is(err, driver.ErrDatabaseNotFound):
		return DatabaseNotFound
	case errors.Is(err, driver.ErrPermissionDenied):
		return PermissionDenied
	case errors.Is(err, driver.ErrTLSRefused):
		return TLSRequired
	case isCertificateError(err):
		return TLSVerifyFailed
	case isTLSError(err):
		return TLSFailed
	case isType[*net.DNSError](err):
		return DNSFailed
	case isRefused(err):
		return ConnectionRefused
	case isTimeout(err):
		return Timeout
	case isType[*net.OpError](err):
		return NetworkError
	}
	return Internal
}

func isType[E error](err error) bool {
	_, ok := errors.AsType[E](err)
	return ok
}

func isCertificateError(err error) bool {
	return isType[*tls.CertificateVerificationError](err) ||
		isType[x509.UnknownAuthorityError](err) ||
		isType[x509.HostnameError](err) ||
		isType[x509.CertificateInvalidError](err)
}

// isTLSError also covers files that cannot be read: the only files a database
// connection reads are its TLS certificates and keys. (SSH keys get codes of
// their own in M2.7.)
func isTLSError(err error) bool {
	return isType[tls.RecordHeaderError](err) ||
		isType[tls.AlertError](err) ||
		isType[*fs.PathError](err)
}

// wsaECONNREFUSED is what Windows reports for a refused connection; there,
// syscall.ECONNREFUSED is a made-up value no network call returns.
const wsaECONNREFUSED = syscall.Errno(10061)

func isRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, wsaECONNREFUSED)
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	netErr, ok := errors.AsType[net.Error](err)
	return ok && netErr.Timeout()
}
