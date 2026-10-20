package chatgpt

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform/secrets"
)

func TestAuthorizationValidatesIdentityAndRefreshesAfterRestart(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	var issuer, nonce, clientID string
	var mu sync.Mutex
	refreshes := 0
	jwt := func(subject string) string {
		h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"fixture"}`))
		p, _ := json.Marshal(map[string]any{"iss": issuer, "sub": subject, "aud": clientID, "exp": time.Now().Add(time.Hour).Unix(), "nonce": nonce, "email": "anonymous@example.invalid"})
		input := h + "." + base64.RawURLEncoding.EncodeToString(p)
		sum := sha256.Sum256([]byte(input))
		sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		return input + "." + base64.RawURLEncoding.EncodeToString(sig)
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]string{"issuer": issuer, "jwks_uri": issuer + "/jwks", "revocation_endpoint": issuer + "/revoke"})
		case "/jwks":
			json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{"kty": "RSA", "alg": "RS256", "kid": "fixture", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
		case "/api/accounts/oauth/token":
			_ = r.ParseForm()
			mu.Lock()
			defer mu.Unlock()
			if r.Form.Get("client_secret") != "" {
				t.Error("public client sent secret")
			}
			if r.Form.Get("grant_type") == "refresh_token" {
				refreshes++
				json.NewEncoder(w).Encode(map[string]any{"access_token": "renewed", "refresh_token": "replacement", "token_type": "Bearer", "expires_in": 3600})
				return
			}
			if r.Form.Get("code_verifier") == "" || r.Form.Get("client_id") != "oaiapp_fixture" {
				t.Error("missing PKCE or issued client")
			}
			json.NewEncoder(w).Encode(map[string]any{"access_token": "initial", "refresh_token": "refresh", "id_token": jwt("fixture-subject"), "token_type": "Bearer", "expires_in": 1, "scope": Scopes})
		case "/revoke":
			w.WriteHeader(200)
		default:
			http.NotFound(w, r)
		}
	}))
	defer s.Close()
	issuer = s.URL
	store := secrets.NewBundleStore(secrets.NewFake())
	m := New(store, secrets.IsNotFound)
	m.issuer = issuer
	m.http = s.Client()
	open := func(raw string) error {
		u, _ := url.Parse(raw)
		q := u.Query()
		nonce = q.Get("nonce")
		clientID = "oaiapp_fixture"
		if q.Get("client_id") != "dynamic_agent_client" || q.Get("agent_name_hint") != "Daygo" || q.Get("ext_agent_host_id") == "" || q.Get("code_challenge_method") != "S256" {
			t.Error("incorrect dynamic registration")
		}
		callback, _ := url.Parse(q.Get("redirect_uri"))
		v := url.Values{"state": {q.Get("state")}, "code": {"fixture-code"}, "client_id": {clientID}}
		callback.RawQuery = v.Encode()
		response, err := http.Get(callback.String())
		if err == nil {
			response.Body.Close()
		}
		return err
	}
	if err := m.Authorize(context.Background(), "fixture", open); err != nil {
		t.Fatal(err)
	}
	restarted := New(store, secrets.IsNotFound)
	restarted.issuer = issuer
	restarted.http = s.Client()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := restarted.Token(context.Background(), "fixture")
			if err != nil || token != "renewed" {
				t.Error("restart refresh failed", err)
			}
		}()
	}
	wg.Wait()
	if refreshes != 1 {
		t.Fatalf("refreshes=%d", refreshes)
	}
	revoked, err := restarted.Disconnect(context.Background(), "fixture")
	if err != nil || !revoked {
		t.Fatal("revocation failed", err)
	}
	if _, err := restarted.Token(context.Background(), "fixture"); err == nil {
		t.Fatal("signed-out credential remained usable")
	}
}

func TestAuthorizationRejectsWrongStateAndCancellation(t *testing.T) {
	m := New(secrets.NewBundleStore(secrets.NewFake()), secrets.IsNotFound)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := m.Authorize(ctx, "fixture", func(raw string) error {
		u, _ := url.Parse(raw)
		callback, _ := url.Parse(u.Query().Get("redirect_uri"))
		callback.RawQuery = url.Values{"state": {"wrong"}, "code": {"private-code"}}.Encode()
		res, e := http.Get(callback.String())
		if e != nil {
			return e
		}
		defer res.Body.Close()
		if res.StatusCode != 400 {
			t.Error("wrong state accepted")
		}
		cancel()
		return nil
	})
	if err == nil || strings.Contains(fmt.Sprint(err), "private-code") {
		t.Fatal("unsafe callback result")
	}
}

func TestRefreshFailurePreservesOrClearsCredentialsByErrorCode(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		connected  bool
	}{
		{"temporary", `{"error":"server_error","detail":"private"}`, 503, true},
		{"revoked", `{"error":"invalid_grant","detail":"private"}`, 400, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer s.Close()
			m := New(secrets.NewBundleStore(secrets.NewFake()), secrets.IsNotFound)
			m.http = s.Client()
			m.issuer = s.URL
			if err := m.save(context.Background(), "fixture", credential{ClientID: "oaiapp_fixture", Subject: "anonymous", AccessToken: "old", RefreshToken: "fixture-refresh", Scope: Scopes, ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
				t.Fatal(err)
			}
			_, err := m.Token(context.Background(), "fixture")
			if err == nil || strings.Contains(err.Error(), "private") {
				t.Fatal("unsafe refresh error")
			}
			_, connected, _ := m.Account(context.Background(), "fixture")
			if connected != tc.connected {
				t.Fatal("wrong credential retention")
			}
		})
	}
}

func TestSubscriptionPauseSurvivesRestartAndRequiresResume(t *testing.T) {
	store := secrets.NewBundleStore(secrets.NewFake())
	m := New(store, secrets.IsNotFound)
	c := credential{ClientID: "oaiapp_fixture", Subject: "anonymous", AccessToken: "fixture-access", Scope: Scopes, ExpiresAt: time.Now().Add(time.Hour)}
	if err := m.save(context.Background(), "fixture", c); err != nil {
		t.Fatal(err)
	}
	if err := m.Pause(context.Background(), "fixture"); err != nil {
		t.Fatal(err)
	}
	restarted := New(store, secrets.IsNotFound)
	if _, err := restarted.Token(context.Background(), "fixture"); err == nil {
		t.Fatal("restart bypassed pause")
	}
	if err := restarted.Resume(context.Background(), "fixture"); err != nil {
		t.Fatal(err)
	}
	if token, err := restarted.Token(context.Background(), "fixture"); err != nil || token != c.AccessToken {
		t.Fatal("resume failed")
	}
}

func TestIDTokenRejectsForgedSignatureAndWrongClaims(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	var issuer string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			json.NewEncoder(w).Encode(map[string]string{"issuer": issuer, "jwks_uri": issuer + "/jwks", "revocation_endpoint": issuer + "/revoke"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{"kty": "RSA", "kid": "fixture", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
	}))
	defer s.Close()
	issuer = s.URL
	m := New(secrets.NewBundleStore(secrets.NewFake()), secrets.IsNotFound)
	m.http = s.Client()
	m.issuer = issuer
	for _, tc := range []struct {
		name, issuer, audience, nonce string
		expired, forged               bool
	}{
		{name: "signature", issuer: issuer, audience: "client", nonce: "nonce", forged: true},
		{name: "issuer", issuer: "https://example.invalid", audience: "client", nonce: "nonce"},
		{name: "audience", issuer: issuer, audience: "wrong", nonce: "nonce"},
		{name: "nonce", issuer: issuer, audience: "client", nonce: "wrong"},
		{name: "expiry", issuer: issuer, audience: "client", nonce: "nonce", expired: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exp := time.Now().Add(time.Hour).Unix()
			if tc.expired {
				exp = time.Now().Add(-time.Hour).Unix()
			}
			raw, _ := json.Marshal(map[string]any{"iss": tc.issuer, "sub": "fixture", "aud": tc.audience, "nonce": tc.nonce, "exp": exp})
			input := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"fixture"}`)) + "." + base64.RawURLEncoding.EncodeToString(raw)
			sum := sha256.Sum256([]byte(input))
			sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
			if tc.forged {
				sig[0] ^= 1
			}
			if _, err := m.verify(context.Background(), input+"."+base64.RawURLEncoding.EncodeToString(sig), "client", "nonce"); err == nil {
				t.Fatal("untrusted identity accepted")
			}
		})
	}
}

type contextualStore struct{ Store }

func (s contextualStore) Set(ctx context.Context, key, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.Store.Set(ctx, key, value)
}

func TestDisconnectClearsLocalTokensAfterRemoteTimeout(t *testing.T) {
	var issuer string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			json.NewEncoder(w).Encode(map[string]string{"issuer": issuer, "jwks_uri": issuer + "/jwks", "revocation_endpoint": issuer + "/revoke"})
			return
		}
		select {
		case <-r.Context().Done():
		case <-time.After(200 * time.Millisecond):
		}
	}))
	defer s.Close()
	issuer = s.URL
	m := New(contextualStore{secrets.NewBundleStore(secrets.NewFake())}, secrets.IsNotFound)
	m.issuer = issuer
	m.http = s.Client()
	if err := m.save(context.Background(), "fixture", credential{ClientID: "oaiapp_fixture", AccessToken: "fixture-access", RefreshToken: "fixture-refresh", Scope: Scopes}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	revoked, err := m.Disconnect(ctx, "fixture")
	if err != nil || revoked {
		t.Fatal("incorrect remote revocation result", err)
	}
	c, err := m.load(context.Background(), "fixture")
	if err != nil || c.AccessToken != "" || c.RefreshToken != "" || c.ClientID != "oaiapp_fixture" {
		t.Fatal("local signout did not preserve registration and clear tokens")
	}
}
