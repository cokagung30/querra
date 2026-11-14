package secrets

import (
	"errors"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// TestStores runs the same behaviour checks against every Store. The Keyring
// case uses go-keyring's in-memory mock provider, never the real keychain;
// keychain_test.go (build tag "keychain") runs them against the real one.
func TestStores(t *testing.T) {
	t.Run("memory", func(t *testing.T) {
		testStore(t, func() Store { return NewMemory() })
	})
	t.Run("keyring with mock provider", func(t *testing.T) {
		if realKeychain {
			t.Skip("the keychain build tag runs Keyring against the real keychain instead")
		}
		testStore(t, func() Store {
			keyring.MockInit()
			return NewKeyring()
		})
	})
}

// testStore checks the Store contract. Every connection id it uses is cleaned
// up afterwards, so it is safe to run against a real keychain.
func testStore(t *testing.T, newStore func() Store) {
	const secret = "s3cr3t-value-123"

	fresh := func(t *testing.T, ids ...string) Store {
		s := newStore()
		t.Cleanup(func() {
			for _, id := range ids {
				_ = s.DeleteAll(id)
			}
		})
		return s
	}

	t.Run("get of a missing secret", func(t *testing.T) {
		s := fresh(t, "conn-missing")
		_, err := s.Get("conn-missing", Password)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get() error = %v, want ErrNotFound", err)
		}
		if has, err := Has(s, "conn-missing", Password); err != nil || has {
			t.Fatalf("Has() = %v, %v; want false, nil", has, err)
		}
	})

	t.Run("set, get, and overwrite", func(t *testing.T) {
		s := fresh(t, "conn-a")
		if err := s.Set("conn-a", Password, secret); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if got, err := s.Get("conn-a", Password); err != nil || got != secret {
			t.Fatalf("Get() = %q, %v; want the stored value", got, err)
		}
		if err := s.Set("conn-a", Password, "changed"); err != nil {
			t.Fatalf("Set() overwrite error = %v", err)
		}
		if got, _ := s.Get("conn-a", Password); got != "changed" {
			t.Fatalf("Get() after overwrite = %q, want %q", got, "changed")
		}
		if has, err := Has(s, "conn-a", Password); err != nil || !has {
			t.Fatalf("Has() = %v, %v; want true, nil", has, err)
		}
	})

	t.Run("kinds are separate entries", func(t *testing.T) {
		s := fresh(t, "conn-b")
		if err := s.Set("conn-b", Password, "db"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Get("conn-b", SSHPassword); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get(SSHPassword) error = %v, want ErrNotFound", err)
		}
	})

	t.Run("delete, including a missing secret", func(t *testing.T) {
		s := fresh(t, "conn-c")
		if err := s.Set("conn-c", SSHPassphrase, secret); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete("conn-c", SSHPassphrase); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if _, err := s.Get("conn-c", SSHPassphrase); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get() after Delete error = %v, want ErrNotFound", err)
		}
		if err := s.Delete("conn-c", SSHPassphrase); err != nil {
			t.Fatalf("Delete() of a missing secret error = %v, want nil", err)
		}
	})

	t.Run("delete all keeps other connections", func(t *testing.T) {
		s := fresh(t, "conn-d", "conn-e")
		for _, kind := range Kinds() {
			if err := s.Set("conn-d", kind, secret); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.Set("conn-e", Password, "other"); err != nil {
			t.Fatal(err)
		}
		if err := s.DeleteAll("conn-d"); err != nil {
			t.Fatalf("DeleteAll() error = %v", err)
		}
		for _, kind := range Kinds() {
			if _, err := s.Get("conn-d", kind); !errors.Is(err, ErrNotFound) {
				t.Fatalf("Get(%s) after DeleteAll error = %v, want ErrNotFound", kind, err)
			}
		}
		if got, err := s.Get("conn-e", Password); err != nil || got != "other" {
			t.Fatalf("other connection's secret = %q, %v; want it untouched", got, err)
		}
	})

	t.Run("rejects invalid input without echoing the value", func(t *testing.T) {
		s := fresh(t, "conn-f")
		tests := []struct {
			name   string
			connID string
			kind   Kind
			value  string
		}{
			{"empty connection id", "", Password, secret},
			{"slash in connection id", "a/b", Password, secret},
			{"backslash in connection id", `a\b`, Password, secret},
			{"unknown kind", "conn-f", Kind("token"), secret},
			{"empty value", "conn-f", Password, ""},
			{"value over the limit", "conn-f", Password, secret + strings.Repeat("x", MaxValueBytes)},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := s.Set(tt.connID, tt.kind, tt.value)
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("Set() error = %v, want ErrInvalid", err)
				}
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error %q contains the secret value", err)
				}
			})
		}
	})
}

func TestAccount(t *testing.T) {
	got, err := account("6f1c", SSHPassphrase)
	if err != nil || got != "conn/6f1c/ssh-passphrase" {
		t.Fatalf("account() = %q, %v; want conn/6f1c/ssh-passphrase", got, err)
	}
}

func TestKeyringWrapsBackendErrorsWithoutTheValue(t *testing.T) {
	if realKeychain {
		t.Skip("needs the mock provider")
	}
	keyring.MockInitWithError(errors.New("backend unavailable"))
	t.Cleanup(keyring.MockInit)

	err := NewKeyring().Set("conn-g", Password, "s3cr3t-value-123")
	if err == nil || !strings.Contains(err.Error(), "backend unavailable") {
		t.Fatalf("Set() error = %v, want the backend error", err)
	}
	if strings.Contains(err.Error(), "s3cr3t") {
		t.Fatalf("error %q contains the secret value", err)
	}
	if _, err := NewKeyring().Get("conn-g", Password); errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v; a backend failure must not look like a missing secret", err)
	}
}
