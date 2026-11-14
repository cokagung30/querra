package secrets

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// Keyring stores secrets in the OS keychain through go-keyring. On macOS
// go-keyring runs /usr/bin/security instead of the native Keychain API, so
// updates of the ad-hoc signed app cause no keychain prompts, at the cost of
// other processes of the same user being able to read the entries
// (docs/spikes/fase-0.md §4.5 and §7).
type Keyring struct {
	service string
}

// NewKeyring returns a Store backed by the OS keychain.
func NewKeyring() *Keyring {
	return &Keyring{service: ServiceName}
}

// Get implements Store.
func (k *Keyring) Get(connID string, kind Kind) (string, error) {
	acct, err := account(connID, kind)
	if err != nil {
		return "", err
	}
	value, err := keyring.Get(k.service, acct)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", fmt.Errorf("%s: %w", acct, ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("read keychain entry %s: %w", acct, err)
	}
	return value, nil
}

// Set implements Store.
func (k *Keyring) Set(connID string, kind Kind, value string) error {
	acct, err := account(connID, kind)
	if err != nil {
		return err
	}
	if err := checkValue(value); err != nil {
		return err
	}
	if err := keyring.Set(k.service, acct, value); err != nil {
		return fmt.Errorf("write keychain entry %s: %w", acct, err)
	}
	return nil
}

// Delete implements Store.
func (k *Keyring) Delete(connID string, kind Kind) error {
	acct, err := account(connID, kind)
	if err != nil {
		return err
	}
	if err := keyring.Delete(k.service, acct); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("delete keychain entry %s: %w", acct, err)
	}
	return nil
}

// DeleteAll implements Store.
func (k *Keyring) DeleteAll(connID string) error {
	var errs []error
	for _, kind := range Kinds() {
		if err := k.Delete(connID, kind); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
