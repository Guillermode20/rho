// Package providers implements LLM provider integrations for rho.
package providers

import (
	"fmt"
	"sync"

	"github.com/earendil-works/rho/pkg/ai"
)

// StreamProvider wraps a provider's stream functions.
type StreamProvider struct {
	API          ai.API
	Stream       func(model ai.Model, context ai.Context, options *ai.StreamOptions, callback ai.StreamEventCallback) error
	StreamSimple func(model ai.Model, context ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error
}

var (
	registry   = make(map[ai.API]*StreamProvider)
	registryMu sync.RWMutex
)

// Register registers a provider for a given API type.
func Register(p *StreamProvider) {
	registryMu.Lock()
	registry[p.API] = p
	registryMu.Unlock()
}

// Get returns the provider for a given API, or nil.
func Get(api ai.API) *StreamProvider {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[api]
}

// Stream calls the appropriate provider's stream function.
func Stream(model ai.Model, context ai.Context, options *ai.StreamOptions, callback ai.StreamEventCallback) error {
	p := Get(model.API)
	if p == nil {
		return fmt.Errorf("no provider registered for api %q", model.API)
	}
	if p.Stream == nil {
		return fmt.Errorf("provider %q does not support stream", model.API)
	}
	return p.Stream(model, context, options, callback)
}

// StreamSimple calls the appropriate provider's streamSimple function.
func StreamSimple(model ai.Model, context ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
	p := Get(model.API)
	if p == nil {
		return fmt.Errorf("no provider registered for api %q", model.API)
	}
	if p.StreamSimple == nil {
		return fmt.Errorf("provider %q does not support streamSimple", model.API)
	}
	return p.StreamSimple(model, context, options, callback)
}
