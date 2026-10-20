package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/chatgpt"
	"github.com/Jwz-git/Daygo/internal/ai/factory"
	"github.com/Jwz-git/Daygo/internal/ai/openai"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
	"github.com/Jwz-git/Daygo/internal/settings"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type chatgptRuntime struct {
	manager *chatgpt.Manager
	attempt *chatgptAttempt
}
type chatgptAttempt struct {
	id       string
	existing bool
	cancel   context.CancelFunc
	done     chan struct{}
	status   ChatGPTConnectionDTO
}

// ChatGPTConnectionDTO carries only safe status, never OAuth values or URLs.
type ChatGPTConnectionDTO struct {
	NeedsConfirmation bool     `json:"needsConfirmation"`
	ProviderID        string   `json:"providerId"`
	State             string   `json:"state"`
	ErrorCode         string   `json:"errorCode"`
	Email             string   `json:"email"`
	Models            []string `json:"models"`
	Paused            bool     `json:"paused"`
}

func (b *Backend) chatgptManager() (*chatgpt.Manager, error) {
	if err := b.providerSecrets(); err != nil {
		return nil, err
	}
	b.chatgptMu.Lock()
	defer b.chatgptMu.Unlock()
	if b.chatgptState.manager == nil {
		b.chatgptState.manager = chatgpt.New(secrets.NewBundleStore(b.secrets), secrets.IsNotFound)
	}
	return b.chatgptState.manager, nil
}
func (b *Backend) chatgptOwner() error {
	if writable, owner := b.instanceOwnership(); !writable || !owner {
		return apperr.E(apperr.NotCaptureOwner, "ChatGPT authorization requires the writable owner", nil)
	}
	return nil
}
func (b *Backend) chatgptParent() context.Context {
	b.windowCtxMu.Lock()
	defer b.windowCtxMu.Unlock()
	if b.windowCtx != nil {
		return b.windowCtx
	}
	return context.Background()
}

// BeginChatGPTLogin opens the system browser. The returned id is opaque, never
// an authorization code. A new provider is not created until model selection.
func (b *Backend) BeginChatGPTLogin(providerID string) (string, error) {
	if err := b.chatgptOwner(); err != nil {
		return "", err
	}
	repo, err := b.providerStore()
	if err != nil {
		return "", err
	}
	manager, err := b.chatgptManager()
	if err != nil {
		return "", err
	}
	existing := providerID != ""
	if existing {
		row, err := repo.Get(b.chatgptParent(), providerID)
		if err != nil {
			return "", mapStorageError("get ChatGPT provider", err)
		}
		if row.Protocol != "chatgpt" {
			return "", apperr.E(apperr.InvalidArgument, "provider is not a ChatGPT subscription", nil)
		}
	} else {
		providerID, err = newProviderID()
		if err != nil {
			return "", err
		}
	}
	b.chatgptMu.Lock()
	if b.chatgptState.attempt != nil {
		b.chatgptMu.Unlock()
		return "", apperr.E(apperr.Conflict, "another ChatGPT authorization is active", nil)
	}
	ctx, cancel := context.WithTimeout(b.chatgptParent(), 5*time.Minute)
	a := &chatgptAttempt{id: providerID, existing: existing, cancel: cancel, done: make(chan struct{}), status: ChatGPTConnectionDTO{ProviderID: providerID, State: "waiting", Models: []string{}}}
	b.chatgptState.attempt = a
	b.chatgptMu.Unlock()
	go func() {
		defer close(a.done)
		defer cancel()
		open := b.chatgptOpenURL
		if open == nil {
			open = func(raw string) error {
				b.windowCtxMu.Lock()
				ctx := b.windowCtx
				b.windowCtxMu.Unlock()
				if ctx == nil {
					return errors.New("desktop runtime unavailable")
				}
				runtime.BrowserOpenURL(ctx, raw)
				return nil
			}
		}
		err := manager.Authorize(ctx, providerID, open)
		status := ChatGPTConnectionDTO{ProviderID: providerID, State: "error", Models: []string{}}
		if err == nil {
			token, tokenErr := manager.Token(ctx, providerID)
			if tokenErr != nil {
				err = tokenErr
			} else {
				status.Models, err = ai.ListModels(ctx, ai.ProtocolChatGPT, openai.SubscriptionEndpoint, token, "")
			}
		}
		if err == nil {
			status.State = "ready"
			status.Email, _, status.Paused = manager.Account(ctx, providerID)
		} else {
			status.ErrorCode = chatgpt.Code(err)
		}
		b.chatgptMu.Lock()
		if b.chatgptState.attempt == a {
			a.status = status
		}
		b.chatgptMu.Unlock()
	}()
	return providerID, nil
}
func (b *Backend) GetChatGPTConnection(id string) (ChatGPTConnectionDTO, error) {
	b.chatgptMu.Lock()
	a := b.chatgptState.attempt
	if a != nil && a.id == id {
		status := a.status
		manager := b.chatgptState.manager
		status.Models = append([]string{}, status.Models...)
		b.chatgptMu.Unlock()
		if status.State == "ready" {
			ctx, cancel := context.WithTimeout(b.chatgptParent(), providersTimeout)
			defer cancel()
			status.NeedsConfirmation = manager.NeedsConfirmation(ctx, id)
		}
		return status, nil
	}
	b.chatgptMu.Unlock()
	repo, err := b.providerStore()
	if err != nil {
		return ChatGPTConnectionDTO{}, err
	}
	ctx, cancel := context.WithTimeout(b.chatgptParent(), providersTimeout)
	defer cancel()
	row, err := repo.Get(ctx, id)
	if err != nil {
		return ChatGPTConnectionDTO{}, mapStorageError("get ChatGPT provider", err)
	}
	if row.Protocol != "chatgpt" {
		return ChatGPTConnectionDTO{}, apperr.E(apperr.InvalidArgument, "provider is not a ChatGPT subscription", nil)
	}
	manager, err := b.chatgptManager()
	if err != nil {
		return ChatGPTConnectionDTO{}, err
	}
	email, connected, paused := manager.Account(ctx, id)
	state := "disconnected"
	if connected {
		state = "connected"
	}
	return ChatGPTConnectionDTO{NeedsConfirmation: manager.NeedsConfirmation(ctx, id), ProviderID: id, State: state, Email: email, Models: row.Models, Paused: paused}, nil
}
func (b *Backend) cancelChatGPTAttempt(id string) {
	b.chatgptMu.Lock()
	a := b.chatgptState.attempt
	if a == nil || (id != "" && id != a.id) {
		b.chatgptMu.Unlock()
		return
	}
	a.cancel()
	b.chatgptMu.Unlock()
	<-a.done
}
func (b *Backend) CancelChatGPTLogin(id string) error {
	if err := b.chatgptOwner(); err != nil {
		return err
	}
	b.cancelChatGPTAttempt(id)
	b.chatgptMu.Lock()
	a := b.chatgptState.attempt
	if a == nil || a.id != id {
		b.chatgptMu.Unlock()
		return nil
	}
	b.chatgptState.attempt = nil
	manager := b.chatgptState.manager
	b.chatgptMu.Unlock()
	if !a.existing && manager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), providersTimeout)
		defer cancel()
		// Best-effort remote cleanup. No provider row or usable local token remains.
		_, _ = manager.Disconnect(ctx, id)
		_ = manager.Forget(ctx, id)
	}
	return nil
}

// SaveChatGPTProvider consumes a completed OAuth attempt and model selection.
// Existing subscriptions can also be renamed/configured without reauthorization.
func (b *Backend) SaveChatGPTProvider(id, name string, models []string) error {
	if err := b.chatgptOwner(); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	normalized, err := normalizeModels(models)
	if err != nil {
		return err
	}
	if name == "" {
		return apperr.E(apperr.InvalidArgument, "display name is required", nil)
	}
	repo, err := b.providerStore()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(b.chatgptParent(), providersTimeout)
	defer cancel()
	b.chatgptMu.Lock()
	defer b.chatgptMu.Unlock()
	a := b.chatgptState.attempt
	row, getErr := repo.Get(ctx, id)
	if getErr == nil && row.Protocol != "chatgpt" {
		return apperr.E(apperr.InvalidArgument, "provider is not a ChatGPT subscription", nil)
	}
	if a != nil && a.id == id {
		if a.status.State != "ready" {
			return apperr.E(apperr.Conflict, "ChatGPT authorization is not ready", nil)
		}
		for _, model := range normalized {
			found := false
			for _, available := range a.status.Models {
				if model == available {
					found = true
				}
			}
			if !found {
				return apperr.E(apperr.InvalidArgument, "model is not in the ChatGPT catalog", nil)
			}
		}
	} else if getErr != nil {
		return mapStorageError("get ChatGPT provider", getErr)
	}
	if b.chatgptState.manager == nil || b.chatgptState.manager.NeedsConfirmation(ctx, id) {
		return apperr.E(apperr.Conflict, "confirm ChatGPT plan usage before saving", nil)
	}
	row.ID = id
	row.DisplayName = name
	row.Protocol = "chatgpt"
	row.Endpoint = openai.SubscriptionEndpoint
	row.Models = normalized
	row.UserAgent = ""
	if getErr == nil {
		err = repo.Update(ctx, id, row)
	} else if storage.IsKind(getErr, storage.KindNotFound) {
		err = repo.Add(ctx, row)
	} else {
		return mapStorageError("get ChatGPT provider", getErr)
	}
	if err != nil {
		return mapStorageError("save ChatGPT provider", err)
	}
	if a != nil && a.id == id {
		a.existing = true
	}
	routing, err := b.loadRouting(ctx)
	if err != nil {
		return err
	}
	if len(routing.Chain) == 0 {
		if err := b.saveRouting(ctx, []settings.RoutingEntry{{ProviderID: id}}); err != nil {
			return err
		}
	}
	if a != nil && a.id == id {
		a.existing = true
		b.chatgptState.attempt = nil
	}
	b.emitSettingsChanged([]string{settings.KeyProvidersRouting})
	return nil
}

// DisconnectChatGPT returns whether remote revocation was confirmed. Local
// credentials are cleared even when the remote service cannot be reached.
func (b *Backend) DisconnectChatGPT(id string) (bool, error) {
	if err := b.chatgptOwner(); err != nil {
		return false, err
	}
	repo, err := b.providerStore()
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(b.chatgptParent(), providersTimeout)
	defer cancel()
	row, err := repo.Get(ctx, id)
	if err != nil {
		return false, mapStorageError("get ChatGPT provider", err)
	}
	if row.Protocol != "chatgpt" {
		return false, apperr.E(apperr.InvalidArgument, "provider is not a ChatGPT subscription", nil)
	}
	b.cancelChatGPTAttempt(id)
	b.chatgptMu.Lock()
	if a := b.chatgptState.attempt; a != nil && a.id == id {
		b.chatgptState.attempt = nil
	}
	b.chatgptMu.Unlock()
	manager, err := b.chatgptManager()
	if err != nil {
		return false, err
	}
	revoked, err := manager.Disconnect(ctx, id)
	if err != nil {
		return revoked, apperr.E(apperr.NativeUnavailable, "clearing ChatGPT credentials failed", nil)
	}
	b.emitSettingsChanged([]string{settings.KeyProvidersRouting})
	return revoked, nil
}
func (b *Backend) ResumeChatGPTSubscription(id string) error {
	if err := b.chatgptOwner(); err != nil {
		return err
	}
	repo, err := b.providerStore()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(b.chatgptParent(), providersTimeout)
	defer cancel()
	row, err := repo.Get(ctx, id)
	if err != nil {
		return mapStorageError("get ChatGPT provider", err)
	}
	if row.Protocol != "chatgpt" {
		return apperr.E(apperr.InvalidArgument, "provider is not a ChatGPT subscription", nil)
	}
	manager, err := b.chatgptManager()
	if err != nil {
		return err
	}
	if err := manager.Resume(ctx, id); err != nil {
		return apperr.E(apperr.NativeUnavailable, "resuming ChatGPT subscription failed", nil)
	}
	b.emitSettingsChanged([]string{settings.KeyProvidersRouting})
	return nil
}

func (b *Backend) providerClientConfig(ctx context.Context, row storage.Provider, model string) (factory.Config, error) {
	cfg := factory.Config{Protocol: ai.Protocol(row.Protocol), Endpoint: row.Endpoint, Model: model, UserAgent: row.UserAgent}
	if cfg.Protocol == ai.ProtocolChatGPT {
		manager, err := b.chatgptManager()
		if err != nil {
			return cfg, err
		}
		cfg.TokenSource = func(ctx context.Context) (string, error) {
			if err := b.chatgptOwner(); err != nil {
				return "", ai.NewError(ai.ErrorAuthentication, "ChatGPT requires the writable owner", 0, nil)
			}
			return manager.Token(ctx, row.ID)
		}
		cfg.OnError = func(ctx context.Context, err error) {
			if errors.Is(err, ai.ErrSubscriptionLimit) {
				pauseCtx, cancel := context.WithTimeout(context.Background(), providersTimeout)
				defer cancel()
				_ = manager.Pause(pauseCtx, row.ID)
				b.emitSettingsChanged([]string{settings.KeyProvidersRouting})
			}
		}
		return cfg, nil
	}
	if b.secrets != nil {
		secret, err := b.secrets.Get(ctx, row.ID)
		if err != nil && !secrets.IsNotFound(err) {
			return cfg, err
		}
		cfg.Secret = secret
	}
	return cfg, nil
}

func (b *Backend) ConfirmChatGPTPlan(id string) error {
	if err := b.chatgptOwner(); err != nil {
		return err
	}
	if _, err := b.GetChatGPTConnection(id); err != nil {
		return err
	}
	manager, err := b.chatgptManager()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(b.chatgptParent(), providersTimeout)
	defer cancel()
	if err := manager.Confirm(ctx, id); err != nil {
		return apperr.E(apperr.NativeUnavailable, "saving ChatGPT plan confirmation failed", nil)
	}
	return nil
}
