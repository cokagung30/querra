//go:build keychain

package secrets

import (
	"testing"
	"time"
)

// realKeychain is true with the keychain build tag: the mock-provider tests
// are skipped, because go-keyring's mock cannot be switched off again once
// installed.
const realKeychain = true

// TestKeychainSmoke runs the Store contract against the real OS keychain, the
// one thing normal tests never touch. It writes dummy values under its own
// service name and deletes every entry afterwards, so it never sees real
// Querra secrets. Run it by hand on each platform (docs/test/M1.1.md):
//
//	go test -tags keychain ./internal/secrets -run TestKeychainSmoke -v
func TestKeychainSmoke(t *testing.T) {
	service := ServiceName + ".smoke-test"
	start := time.Now()
	testStore(t, func() Store { return &Keyring{service: service} })
	t.Logf("keychain service %s: %s total", service, time.Since(start).Round(time.Millisecond))
}
