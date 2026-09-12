package app

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
	"time"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/factory"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
	"github.com/Jwz-git/Daygo/internal/settings"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// providersTimeout bounds one provider CRUD operation. The probe inside
// TestProvider keeps its own 30-second deadline.
const providersTimeout = 10 * time.Second

// providerStore returns the provider repository, or an error explaining why
// provider configuration is unavailable. Without a database there is nothing
// to configure; without a keychain there is no place for secrets.
func (b *Backend) providerStore() (*storage.ProviderRepo, error) {
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return nil, mapStorageError("open providers", err)
		}
		return nil, apperr.E(apperr.DatabaseError, "providers require a database", nil)
	}
	return store.Providers(), nil
}

// providerSecrets returns the keychain, or a native_unavailable error on
// platforms without one. Read-modify flows that tolerate a missing keychain
// (ListProviders) must not call this.
func (b *Backend) providerSecrets() error {
	if b.secrets == nil {
		return apperr.E(apperr.NativeUnavailable, "keychain is unavailable", nil)
	}
	return nil
}

// validateProviderInput checks the wire payload. The same rules apply to add
// and update so the two paths cannot drift apart.
func validateProviderInput(p ProviderInputDTO) (displayName, protocol, endpoint, model string, err error) {
	displayName = strings.TrimSpace(p.DisplayName)
	if displayName == "" {
		return "", "", "", "", apperr.E(apperr.InvalidArgument, "display name is required", nil)
	}
	proto := daygoai.Protocol(strings.TrimSpace(p.Protocol))
	if !proto.Valid() {
		return "", "", "", "", apperr.E(apperr.InvalidArgument, "unknown provider protocol", nil)
	}
	endpoint, err = normalizeTestEndpoint(p.Endpoint)
	if err != nil {
		return "", "", "", "", apperr.E(apperr.InvalidArgument, "endpoint must be a full http:// or https:// address", err)
	}
	model = strings.TrimSpace(p.Model)
	if model == "" {
		return "", "", "", "", apperr.E(apperr.InvalidArgument, "model is required", nil)
	}
	return displayName, string(proto), endpoint, model, nil
}

// newProviderID generates the opaque provider id. crypto/rand keeps it
// unguessable; the format is v4-shaped for debuggability only.
func newProviderID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", apperr.E(apperr.Internal, "generate provider id", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// ListProviders returns every configured provider with its secret presence.
func (b *Backend) ListProviders() ([]ProviderDTO, error) {
	repo, err := b.providerStore()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), providersTimeout)
	defer cancel()

	rows, err := repo.List(ctx)
	if err != nil {
		return nil, mapStorageError("list providers", err)
	}
	out := make([]ProviderDTO, 0, len(rows))
	for _, row := range rows {
		dto := ProviderDTO{
			ID:          row.ID,
			DisplayName: row.DisplayName,
			Protocol:    row.Protocol,
			Endpoint:    row.Endpoint,
			Model:       row.Model,
		}
		if b.secrets != nil {
			// Presence only: the value is fetched and discarded right here.
			getCtx, getCancel := context.WithTimeout(context.Background(), providersTimeout)
			_, getErr := b.secrets.Get(getCtx, row.ID)
			getCancel()
			dto.HasSecret = getErr == nil
		}
		out = append(out, dto)
	}
	return out, nil
}

// AddProvider creates a provider and, when a secret is supplied, stores it in
// the keychain. Both land or neither: a keychain failure rolls the row back by
// deleting it again, so the UI never sees a provider that claims a key it
// does not have.
func (b *Backend) AddProvider(p ProviderInputDTO) (string, error) {
	repo, err := b.providerStore()
	if err != nil {
		return "", err
	}
	displayName, protocol, endpoint, model, err := validateProviderInput(p)
	if err != nil {
		return "", err
	}
	id, err := newProviderID()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), providersTimeout)
	defer cancel()
	if err := repo.Add(ctx, storage.Provider{
		ID: id, DisplayName: displayName, Protocol: protocol, Endpoint: endpoint, Model: model,
	}); err != nil {
		return "", mapStorageError("add provider", err)
	}

	secret := strings.TrimSpace(p.Secret)
	if secret != "" {
		if err := b.providerSecrets(); err != nil {
			_ = repo.Delete(ctx, id)
			return "", err
		}
		if err := b.secrets.Set(ctx, id, secret); err != nil {
			// Roll the row back: a provider without its key would be a broken
			// state the UI cannot repair through AddProvider alone.
			_ = repo.Delete(ctx, id)
			return "", apperr.E(apperr.NativeUnavailable, "storing the api key failed", nil)
		}
	}

	b.emitSettingsChanged([]string{settings.KeyProvidersRouting})
	return id, nil
}

// UpdateProvider replaces the mutable fields. Secret "" keeps the stored key;
// a non-empty secret replaces it.
func (b *Backend) UpdateProvider(id string, p ProviderInputDTO) error {
	repo, err := b.providerStore()
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return apperr.E(apperr.InvalidArgument, "provider id is required", nil)
	}
	displayName, protocol, endpoint, model, err := validateProviderInput(p)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), providersTimeout)
	defer cancel()
	if err := repo.Update(ctx, id, storage.Provider{
		DisplayName: displayName, Protocol: protocol, Endpoint: endpoint, Model: model,
	}); err != nil {
		return mapStorageError("update provider", err)
	}

	if secret := strings.TrimSpace(p.Secret); secret != "" {
		if err := b.providerSecrets(); err != nil {
			return err
		}
		if err := b.secrets.Set(ctx, id, secret); err != nil {
			return apperr.E(apperr.NativeUnavailable, "storing the api key failed", nil)
		}
	}

	b.emitSettingsChanged([]string{settings.KeyProvidersRouting})
	return nil
}

// DeleteProvider removes the provider, its keychain entry, and its routing
// slots. Conversations pinned to it fall back to the routing chain; their
// provider_id is nulled in the same operation.
func (b *Backend) DeleteProvider(id string) error {
	repo, err := b.providerStore()
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return apperr.E(apperr.InvalidArgument, "provider id is required", nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), providersTimeout)
	defer cancel()

	// Prune routing before deleting the row: SetProviderRouting validates ids
	// against the table, so the order matters.
	routing, err := b.loadRouting(ctx, repo)
	if err != nil {
		return err
	}
	pruned := make([]string, 0, len(routing.Chain))
	for _, entry := range routing.Chain {
		if entry != id {
			pruned = append(pruned, entry)
		}
	}
	if len(pruned) != len(routing.Chain) {
		if err := b.saveRouting(ctx, pruned); err != nil {
			return err
		}
	}

	// Unpin conversations that referenced this provider.
	if chat := b.store().Chat(); chat != nil {
		conversations, err := chat.ListConversations(ctx)
		if err != nil {
			return mapStorageError("list conversations", err)
		}
		for _, c := range conversations {
			if c.ProviderID != nil && *c.ProviderID == id {
				if err := chat.UpdateConversation(ctx, c.ID, c.Title, nil); err != nil {
					return mapStorageError("unpin conversation", err)
				}
			}
		}
	}

	if err := repo.Delete(ctx, id); err != nil {
		return mapStorageError("delete provider", err)
	}

	// The keychain entry is best-effort: a provider whose key is already gone
	// must still be deletable.
	if b.secrets != nil {
		if err := b.secrets.Delete(ctx, id); err != nil && !secrets.IsNotFound(err) {
			return apperr.E(apperr.NativeUnavailable, "deleting the stored api key failed", nil)
		}
	}

	b.emitSettingsChanged([]string{settings.KeyProvidersRouting})
	return nil
}

// GetProviderRouting returns the ordered routing chain.
func (b *Backend) GetProviderRouting() (ProviderRoutingDTO, error) {
	repo, err := b.providerStore()
	if err != nil {
		return ProviderRoutingDTO{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), providersTimeout)
	defer cancel()

	routing, err := b.loadRouting(ctx, repo)
	if err != nil {
		return ProviderRoutingDTO{}, err
	}
	chain := routing.Chain
	if chain == nil {
		chain = []string{}
	}
	return ProviderRoutingDTO{Chain: chain}, nil
}

// SetProviderRouting replaces the routing chain. Every id must exist and
// appear at most once.
func (b *Backend) SetProviderRouting(r ProviderRoutingDTO) error {
	repo, err := b.providerStore()
	if err != nil {
		return err
	}
	if r.Chain == nil {
		return apperr.E(apperr.InvalidArgument, "chain is required", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), providersTimeout)
	defer cancel()

	known := make(map[string]struct{})
	rows, err := repo.List(ctx)
	if err != nil {
		return mapStorageError("list providers", err)
	}
	for _, row := range rows {
		known[row.ID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(r.Chain))
	for _, id := range r.Chain {
		if _, ok := known[id]; !ok {
			return apperr.E(apperr.InvalidArgument, "routing references an unknown provider", nil)
		}
		if _, dup := seen[id]; dup {
			return apperr.E(apperr.InvalidArgument, "routing contains a provider twice", nil)
		}
		seen[id] = struct{}{}
	}

	if err := b.saveRouting(ctx, r.Chain); err != nil {
		return err
	}
	b.emitSettingsChanged([]string{settings.KeyProvidersRouting})
	return nil
}

// SetProviderSecret stores or replaces one provider's key.
func (b *Backend) SetProviderSecret(id string, secret string) error {
	repo, err := b.providerStore()
	if err != nil {
		return err
	}
	if err := b.providerSecrets(); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return apperr.E(apperr.InvalidArgument, "provider id is required", nil)
	}
	if strings.TrimSpace(secret) == "" {
		return apperr.E(apperr.InvalidArgument, "secret must not be empty; use DeleteProviderSecret to clear", nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), providersTimeout)
	defer cancel()
	if _, err := repo.Get(ctx, id); err != nil {
		return mapStorageError("get provider", err)
	}
	if err := b.secrets.Set(ctx, id, secret); err != nil {
		return apperr.E(apperr.NativeUnavailable, "storing the api key failed", nil)
	}
	return nil
}

// DeleteProviderSecret removes one provider's key. Removing an absent key is
// not an error: the caller's desired end state is "no key stored".
func (b *Backend) DeleteProviderSecret(id string) error {
	if err := b.providerSecrets(); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return apperr.E(apperr.InvalidArgument, "provider id is required", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), providersTimeout)
	defer cancel()
	if err := b.secrets.Delete(ctx, id); err != nil && !secrets.IsNotFound(err) {
		return apperr.E(apperr.NativeUnavailable, "deleting the stored api key failed", nil)
	}
	return nil
}

// TestProvider runs the connection probe against a saved provider: the secret
// comes from the keychain by provider id, never from the call.
func (b *Backend) TestProvider(id string) (ProviderTestResultDTO, error) {
	repo, err := b.providerStore()
	if err != nil {
		return ProviderTestResultDTO{}, err
	}
	if err := b.providerSecrets(); err != nil {
		return ProviderTestResultDTO{}, err
	}
	if strings.TrimSpace(id) == "" {
		return ProviderTestResultDTO{}, apperr.E(apperr.InvalidArgument, "provider id is required", nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), providersTimeout)
	defer cancel()
	row, err := repo.Get(ctx, id)
	if err != nil {
		return ProviderTestResultDTO{}, mapStorageError("get provider", err)
	}
	secret, err := b.secrets.Get(ctx, id)
	if err != nil {
		if secrets.IsNotFound(err) {
			return ProviderTestResultDTO{}, apperr.E(apperr.InvalidArgument, "no api key stored for this provider", nil)
		}
		return ProviderTestResultDTO{}, apperr.E(apperr.NativeUnavailable, "reading the stored api key failed", nil)
	}

	// No client timeout: the 30-second probe deadline in ai.TestConnection
	// governs the whole exchange.
	provider, err := factory.NewClient(&http.Client{}, factory.Config{
		Protocol: daygoai.Protocol(row.Protocol),
		Endpoint: row.Endpoint,
		Model:    row.Model,
		Secret:   secret,
	})
	if err != nil {
		return testFailure(err), nil
	}
	result, err := daygoai.TestConnection(context.Background(), provider)
	if err != nil {
		return testFailure(err), nil
	}

	capabilities := make([]string, len(result.Capabilities))
	for index, capability := range result.Capabilities {
		capabilities[index] = string(capability)
	}
	return ProviderTestResultDTO{
		OK:           true,
		Model:        result.Model,
		LatencyMs:    result.Latency.Milliseconds(),
		Capabilities: capabilities,
	}, nil
}

// loadRouting reads the current chain through the typed settings layer.
func (b *Backend) loadRouting(ctx context.Context, repo *storage.ProviderRepo) (settings.Routing, error) {
	access := settings.New(b.store().Settings())
	routing, err := access.Routing(ctx)
	if err != nil {
		return settings.Routing{}, mapStorageError("read provider routing", err)
	}
	return routing, nil
}

// saveRouting writes the chain through the typed settings layer.
func (b *Backend) saveRouting(ctx context.Context, chain []string) error {
	access := settings.New(b.store().Settings())
	if err := access.SetRouting(ctx, settings.Routing{Chain: chain}); err != nil {
		return mapStorageError("write provider routing", err)
	}
	return nil
}
