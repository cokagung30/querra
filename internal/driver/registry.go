package driver

import (
	"errors"
	"fmt"
	"slices"
	"sync"
)

// ErrUnknownEngine is returned by Lookup for an engine with no driver.
var ErrUnknownEngine = errors.New("unknown database engine")

// The registry is the one piece of package-level state CLAUDE.md allows:
// engine packages fill it from init.
var (
	registryMu sync.RWMutex
	registry   = map[Engine]Driver{}
)

// Register makes d available through Lookup. Engine packages call it from
// init. It panics if the engine already has a driver.
func Register(d Driver) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[d.Engine()]; dup {
		panic(fmt.Sprintf("driver: engine %q registered twice", d.Engine()))
	}
	registry[d.Engine()] = d
}

// Lookup returns the driver for an engine.
func Lookup(e Engine) (Driver, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	d, ok := registry[e]
	if !ok {
		return nil, fmt.Errorf("%w %q", ErrUnknownEngine, e)
	}
	return d, nil
}

// Engines lists the engines that have a driver, sorted.
func Engines() []Engine {
	registryMu.RLock()
	defer registryMu.RUnlock()
	engines := make([]Engine, 0, len(registry))
	for e := range registry {
		engines = append(engines, e)
	}
	slices.Sort(engines)
	return engines
}
