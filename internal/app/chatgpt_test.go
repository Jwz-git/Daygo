package app

import (
	"context"
	"encoding/json"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/openai"
	"github.com/Jwz-git/Daygo/internal/storage"
)

func TestChatGPTCannotBeCreatedOrRetargetedThroughAPIKeyBindings(t *testing.T) {
	b, _, s := backendWithStoreAndSecrets(t)
	p := validProviderInput()
	p.Protocol = "chatgpt"
	p.Endpoint = openai.SubscriptionEndpoint
	p.Secret = "fixture-token"
	if _, err := b.AddProvider(p); err == nil {
		t.Fatal("API key binding bypassed OAuth")
	}
	id, _ := newProviderID()
	if err := b.store().Providers().Add(context.Background(), storage.Provider{ID: id, Protocol: "chatgpt", Endpoint: openai.SubscriptionEndpoint, Models: []string{"fixture"}, DisplayName: "fixture"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(context.Background(), id, "private-credential"); err != nil {
		t.Fatal(err)
	}
	p = validProviderInput()
	p.Endpoint = "https://example.invalid"
	if err := b.UpdateProvider(id, p); err == nil {
		t.Fatal("subscription retargeted through key form")
	}
	if err := b.DeleteProviderSecret(id); err == nil {
		t.Fatal("subscription bypassed revoke")
	}
	rows, err := b.ListProviders()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(rows)
	if strings.Contains(string(raw), "private-credential") {
		t.Fatal("credential crossed DTO")
	}
}
func TestChatGPTOwnerGateAndLoginCancellation(t *testing.T) {
	b := newBackend(nil, nil, nil, false, false)
	if _, err := b.BeginChatGPTLogin(""); err == nil {
		t.Fatal("read-only login started")
	}
	b = playgroundBackend(t)
	b.chatgptOpenURL = func(string) error { return nil }
	id, err := b.BeginChatGPTLogin("")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.CancelChatGPTLogin(id); err != nil {
		t.Fatal(err)
	}
	status, err := b.GetChatGPTConnection(id)
	if err == nil && status.State == "waiting" {
		t.Fatal("canceled callback still running")
	}
}
func TestChatGPTUsageLimitPausesWithoutAutomaticReset(t *testing.T) {
	if ai.Retryable(ai.NewError(ai.ErrorRateLimited, "limit", 429, ai.ErrSubscriptionLimit)) {
		t.Fatal("subscription limit retried")
	}
}

func TestChatGPTSaveRequiresConfirmationAndRoutesFirstSubscription(t *testing.T) {
	b := playgroundBackend(t)
	id, _ := newProviderID()
	ctx := context.Background()
	raw, _ := json.Marshal(map[string]any{"clientId": "oaiapp_fixture", "subject": "fixture-subject", "email": "anonymous@example.invalid", "accessToken": "anonymous-access", "scope": "chatgpt.tokens.use.direct resource.invoke", "expiresAt": time.Now().Add(time.Hour)})
	if err := secrets.NewBundleStore(b.secrets).Set(ctx, "chatgpt."+id, string(raw)); err != nil {
		t.Fatal(err)
	}
	manager, err := b.chatgptManager()
	if err != nil {
		t.Fatal(err)
	}
	_, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	close(done)
	b.chatgptState.attempt = &chatgptAttempt{id: id, cancel: cancel, done: done, status: ChatGPTConnectionDTO{ProviderID: id, State: "ready", Models: []string{"fixture-model"}}}
	if err := b.SaveChatGPTProvider(id, "fixture", []string{"fixture-model"}); err == nil {
		t.Fatal("unconfirmed plan usage saved")
	}
	if err := b.ConfirmChatGPTPlan(id); err != nil {
		t.Fatal(err)
	}
	if err := b.SaveChatGPTProvider(id, "fixture", []string{"not-in-catalog"}); err == nil {
		t.Fatal("out-of-catalog model saved")
	}
	if err := b.SaveChatGPTProvider(id, "fixture", []string{"fixture-model"}); err != nil {
		t.Fatal(err)
	}
	rows, err := b.ListProviders()
	if err != nil || len(rows) != 1 || rows[0].Protocol != "chatgpt" || !rows[0].HasSecret {
		t.Fatal("saved subscription not listed", err)
	}
	serialized, _ := json.Marshal(rows)
	if strings.Contains(string(serialized), "anonymous-access") {
		t.Fatal("OAuth credential crossed binding")
	}
	routing, err := b.GetProviderRouting()
	if err != nil || len(routing.Chain) != 1 || routing.Chain[0].ProviderID != id {
		t.Fatal("first subscription not routed")
	}
	config, err := b.providerClientConfig(ctx, storage.Provider{ID: id, Protocol: "chatgpt", Endpoint: openai.SubscriptionEndpoint}, "fixture-model")
	if err != nil || config.Secret != "" || config.TokenSource == nil {
		t.Fatal("subscription used API key path")
	}
	if token, err := config.TokenSource(ctx); err != nil || token != "anonymous-access" {
		t.Fatal("scoped token source failed")
	}
	if err := b.DeleteProvider(id); err == nil {
		t.Fatal("connected subscription deleted without disconnect")
	}
	revoked, err := b.DisconnectChatGPT(id)
	if err != nil || revoked {
		t.Fatal("access token without refresh cannot prove remote revocation", err)
	}
	_, connected, _ := manager.Account(ctx, id)
	if connected {
		t.Fatal("disconnect did not clear local authorization")
	}
	if err := b.DeleteProvider(id); err != nil {
		t.Fatal(err)
	}
}
