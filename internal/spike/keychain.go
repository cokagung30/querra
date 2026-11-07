package spike

import (
	"time"

	"github.com/zalando/go-keyring"
)

// The spike uses its own keychain entry with a dummy value, never real secrets.
const (
	keychainService = "com.balikaryadigital.querra.spike"
	keychainAccount = "probe"
)

// KeychainStep is one keychain operation and how long it took.
type KeychainStep struct {
	Op  string `json:"op"`
	Ms  int64  `json:"ms"`
	Err string `json:"err,omitempty"`
	// Value is set by "read"; it is the spike's dummy value.
	Value string `json:"value,omitempty"`
}

// KeychainWrite stores value in the spike's keychain entry.
func KeychainWrite(value string) KeychainStep {
	return timed("write", func() (string, error) {
		return "", keyring.Set(keychainService, keychainAccount, value)
	})
}

// KeychainRead reads the spike's keychain entry.
func KeychainRead() KeychainStep {
	return timed("read", func() (string, error) {
		return keyring.Get(keychainService, keychainAccount)
	})
}

// KeychainDelete removes the spike's keychain entry.
func KeychainDelete() KeychainStep {
	return timed("delete", func() (string, error) {
		return "", keyring.Delete(keychainService, keychainAccount)
	})
}

func timed(op string, fn func() (string, error)) KeychainStep {
	start := time.Now()
	value, err := fn()
	step := KeychainStep{Op: op, Ms: time.Since(start).Milliseconds(), Value: value}
	if err != nil {
		step.Err = err.Error()
	}
	return step
}
