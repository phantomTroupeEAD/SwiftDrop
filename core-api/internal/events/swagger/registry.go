package swagger

import (
	"errors"
	"fmt"
	"sync"
)

var (
	mu    sync.RWMutex
	specs = make(map[string][]byte)
)

// Register adds a named OpenAPI spec to the shared index.
// Fails if name is empty or already registered.
func Register(name string, spec []byte) error {
	mu.Lock()
	defer mu.Unlock()

	if name == "" {
		return errors.New("swagger: spec name cannot be empty")
	}
	if _, exists := specs[name]; exists {
		return fmt.Errorf("swagger: spec %q is already registered", name)
	}

	specs[name] = spec
	return nil
}

// ListSpecs returns a copy of all registered specifications.
func ListSpecs() map[string][]byte {
	mu.RLock()
	defer mu.RUnlock()

	res := make(map[string][]byte, len(specs))
	for k, v := range specs {
		res[k] = v
	}
	return res
}

// GetSpec returns a registered spec by name.
func GetSpec(name string) ([]byte, bool) {
	mu.RLock()
	defer mu.RUnlock()

	spec, ok := specs[name]
	return spec, ok
}

// ResetForTesting clears the registry (used only in unit tests).
func ResetForTesting() {
	mu.Lock()
	defer mu.Unlock()
	specs = make(map[string][]byte)
}
