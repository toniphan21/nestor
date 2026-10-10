package tui

import (
	"sync"

	"nhatp.com/go/nestor"
)

// APIProvider lazily builds a nestor.API via maker and caches it until
// Reset is called, so a page can force the next Get to rebuild from disk.
type APIProvider struct {
	mu    sync.Mutex
	maker func() (nestor.API, error)
	api   nestor.API
}

func NewAPIProvider(maker func() (nestor.API, error)) *APIProvider {
	return &APIProvider{maker: maker}
}

func (p *APIProvider) Get() (nestor.API, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.api != nil {
		return p.api, nil
	}

	api, err := p.maker()
	if err != nil {
		return nil, err
	}
	p.api = api
	return p.api, nil
}

// Verify tries building a fresh API without caching it, so callers can
// check the next instance is sound before committing to Reset.
func (p *APIProvider) Verify() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, err := p.maker()
	return err
}

func (p *APIProvider) Set(api nestor.API) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.api = api
}

func (p *APIProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.api = nil
}
