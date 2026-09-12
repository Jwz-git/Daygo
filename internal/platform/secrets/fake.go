package secrets

import (
	"context"
	"sync"

	"github.com/Jwz-git/Daygo/internal/platform"
)

// Fake is an in-memory platform.Secrets for tests and the Linux CI, where no
// keychain exists. It is exported only for construction via NewFake; nothing
// in a production binary calls it.
type Fake struct {
	mu     sync.Mutex
	values map[string]string
}

var _ platform.Secrets = (*Fake)(nil)

// NewFake builds an empty fake.
func NewFake() *Fake {
	return &Fake{values: make(map[string]string)}
}

func (f *Fake) Get(_ context.Context, providerID string) (string, error) {
	if err := validateProvider(providerID); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.values[providerID]
	if !ok {
		return "", errNotFound(providerID)
	}
	return value, nil
}

func (f *Fake) Set(_ context.Context, providerID, secret string) error {
	if err := validateProvider(providerID); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.values[providerID] = secret
	return nil
}

func (f *Fake) Delete(_ context.Context, providerID string) error {
	if err := validateProvider(providerID); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.values[providerID]; !ok {
		return errNotFound(providerID)
	}
	delete(f.values, providerID)
	return nil
}
