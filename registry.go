package entree

import (
	"fmt"
	"sync"
)

// ProviderFactory creates a Provider from credentials.
type ProviderFactory func(creds Credentials) (Provider, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]ProviderFactory{}
)

// RegisterProvider registers a provider factory under the given slug.
// Called from provider init() functions.
func RegisterProvider(slug string, factory ProviderFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[slug] = factory
}

// NewProvider creates a provider by slug using registered factories.
// Returns error if slug is unknown or factory fails.
func NewProvider(slug string, creds Credentials) (Provider, error) {
	registryMu.RLock()
	factory, ok := registry[slug]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", slug)
	}
	return factory(creds)
}

// RegisteredProviders returns the slugs of all registered providers.
func RegisteredProviders() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	slugs := make([]string, 0, len(registry))
	for slug := range registry {
		slugs = append(slugs, slug)
	}
	return slugs
}
