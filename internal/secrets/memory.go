package secrets

import (
	"fmt"
	"sync"
)

// Memory is an in-memory Store for tests. It validates exactly like Keyring,
// so code tested against it behaves the same with the real keychain. Tests
// must use it instead of the OS keychain.
type Memory struct {
	mu      sync.Mutex
	entries map[string]string
}

// NewMemory returns an empty in-memory Store.
func NewMemory() *Memory {
	return &Memory{entries: map[string]string{}}
}

// Get implements Store.
func (m *Memory) Get(connID string, kind Kind) (string, error) {
	acct, err := account(connID, kind)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.entries[acct]
	if !ok {
		return "", fmt.Errorf("%s: %w", acct, ErrNotFound)
	}
	return value, nil
}

// Set implements Store.
func (m *Memory) Set(connID string, kind Kind, value string) error {
	acct, err := account(connID, kind)
	if err != nil {
		return err
	}
	if err := checkValue(value); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[acct] = value
	return nil
}

// Delete implements Store.
func (m *Memory) Delete(connID string, kind Kind) error {
	acct, err := account(connID, kind)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, acct)
	return nil
}

// DeleteAll implements Store.
func (m *Memory) DeleteAll(connID string) error {
	for _, kind := range Kinds() {
		if err := m.Delete(connID, kind); err != nil {
			return err
		}
	}
	return nil
}

// Len returns the number of stored secrets (for tests).
func (m *Memory) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}
