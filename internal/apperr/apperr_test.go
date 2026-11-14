package apperr

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/cokagung30/querra/internal/driver"
)

func TestFrom(t *testing.T) {
	if From(nil) != nil {
		t.Error("From(nil) is not nil")
	}

	app := New(NotFound, "connection not found")
	if got := From(fmt.Errorf("get: %w", app)); got != app {
		t.Errorf("From(wrapped *Error) = %#v, want the *Error itself", got)
	}

	if got := From(fmt.Errorf("test: %w", context.Canceled)); got.Code != Cancelled {
		t.Errorf("From(context.Canceled).Code = %q, want %q", got.Code, Cancelled)
	}

	got := From(errors.New("disk full"))
	if got.Code != Internal || got.Detail != "disk full" || got.Message == "" {
		t.Errorf("From(other) = %#v, want Internal with the text as detail", got)
	}
}

func TestErrorString(t *testing.T) {
	if got := New(NotFound, "gone").Error(); got != "gone" {
		t.Errorf("Error() = %q, want %q", got, "gone")
	}
	e := &Error{Code: Internal, Message: "Something went wrong", Detail: "disk full"}
	if got := e.Error(); got != "Something went wrong: disk full" {
		t.Errorf("Error() = %q", got)
	}
}

func TestFromConnect(t *testing.T) {
	login := errors.New("failed to connect: server error")
	opErr := func(err error) error { return &net.OpError{Op: "dial", Net: "tcp", Err: err} }

	tests := []struct {
		name string
		err  error
		want Code
	}{
		{"wrong password", driver.Mark(login, driver.ErrAuthFailed), AuthFailed},
		{"no database", driver.Mark(login, driver.ErrDatabaseNotFound), DatabaseNotFound},
		{"no privilege", driver.Mark(login, driver.ErrPermissionDenied), PermissionDenied},
		{"TLS refused", driver.Mark(login, driver.ErrTLSRefused), TLSRequired},
		{"prefer fallback, then wrong password", driver.Mark(login, driver.ErrAuthFailed, driver.ErrTLSRefused), AuthFailed},
		{"unknown CA", x509.UnknownAuthorityError{}, TLSVerifyFailed},
		{"wrong host name", x509.HostnameError{Certificate: &x509.Certificate{}, Host: "db"}, TLSVerifyFailed},
		{"expired", x509.CertificateInvalidError{Reason: x509.Expired}, TLSVerifyFailed},
		{"verification", &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}, TLSVerifyFailed},
		{"not TLS", tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"}, TLSFailed},
		{"TLS alert", fmt.Errorf("handshake: %w", tls.AlertError(40)), TLSFailed},
		{"CA file missing", fmt.Errorf("read CA: %w", &fs.PathError{Op: "open", Path: "ca.pem", Err: fs.ErrNotExist}), TLSFailed},
		{"DNS", opErr(&net.DNSError{Err: "no such host", Name: "db.invalid", IsNotFound: true}), DNSFailed},
		{"refused", opErr(os.NewSyscallError("connect", syscall.ECONNREFUSED)), ConnectionRefused},
		{"refused on Windows", opErr(os.NewSyscallError("connectex", wsaECONNREFUSED)), ConnectionRefused},
		{"dial timeout", opErr(os.ErrDeadlineExceeded), Timeout},
		{"connect timeout", fmt.Errorf("connect: %w", context.DeadlineExceeded), Timeout},
		{"unreachable", opErr(errors.New("network is unreachable")), NetworkError},
		{"other", errors.New("unexpected message"), Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromConnect(tt.err)
			if got.Code != tt.want {
				t.Errorf("FromConnect().Code = %q, want %q", got.Code, tt.want)
			}
			if got.Message != messages[tt.want] || got.Detail != tt.err.Error() {
				t.Errorf("FromConnect() = %#v, want the code's message and the error text as detail", got)
			}
		})
	}
}

// A real refused connection, because each OS reports it differently.
func TestFromConnectRealRefusal(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	c, err := net.Dial("tcp", addr)
	if err == nil {
		_ = c.Close()
		t.Skip("something else is listening on the freed port")
	}
	if got := FromConnect(err); got.Code != ConnectionRefused {
		t.Errorf("FromConnect(%v).Code = %q, want %q", err, got.Code, ConnectionRefused)
	}
}

func TestMarshal(t *testing.T) {
	var got map[string]any
	if err := json.Unmarshal(Marshal(errors.New("disk full")), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != string(Internal) || got["detail"] != "disk full" || got["message"] == "" {
		t.Errorf("Marshal(other) = %v, want an internal error with the text as detail", got)
	}

	got = nil
	if err := json.Unmarshal(Marshal(New(NotFound, "gone")), &got); err != nil {
		t.Fatal(err)
	}
	if want := map[string]any{"code": "not_found", "message": "gone"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Marshal(*Error) = %v, want %v (empty fields left out)", got, want)
	}
}

// Every code needs a fallback message here and a translation in the frontend
// (CLAUDE.md, "Errors").
func TestEveryCodeIsTranslated(t *testing.T) {
	raw, err := os.ReadFile("../../frontend/src/i18n/en.json")
	if err != nil {
		t.Fatal(err)
	}
	var en struct {
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(raw, &en); err != nil {
		t.Fatal(err)
	}
	for _, code := range Codes() {
		if messages[code] == "" {
			t.Errorf("no fallback message for %q", code)
		}
		if en.Errors[string(code)] == "" {
			t.Errorf("frontend/src/i18n/en.json has no errors.%s", code)
		}
	}
	if len(messages) != len(Codes()) {
		t.Errorf("messages has %d codes, Codes() %d", len(messages), len(Codes()))
	}
}
