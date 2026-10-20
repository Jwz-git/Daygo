// Package chatgpt owns the official public-client OAuth lifecycle. It never
// exposes tokens through Wails, logs upstream bodies, or reads Codex credentials.
package chatgpt

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
)

const Scopes = "openid profile email offline_access resource.invoke chatgpt.tokens.use.direct"
const Resource = "https://api.openai.com/v1"
const planScope = "chatgpt.tokens.use.direct"

type Store interface {
	Get(context.Context, string) (string, error)
	Set(context.Context, string, string) error
	Delete(context.Context, string) error
}
type Error struct{ Code string }

func (e *Error) Error() string  { return "ChatGPT authorization: " + e.Code }
func failure(code string) error { return &Error{Code: code} }
func Code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "unavailable"
}

type credential struct {
	ClientID     string    `json:"clientId"`
	Subject      string    `json:"subject"`
	Email        string    `json:"email"`
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	IDToken      string    `json:"idToken"`
	Scope        string    `json:"scope"`
	ExpiresAt    time.Time `json:"expiresAt"`
	Confirmed    bool      `json:"confirmed"`
	Paused       bool      `json:"paused"`
}
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	Type         string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
}

// Manager serializes token reads/rotation and disconnects. There is no timer or
// detached refresh worker: each request renews as needed under its own context.
type Manager struct {
	paused  map[string]bool
	mu      sync.Mutex
	store   Store
	http    *http.Client
	issuer  string
	missing func(error) bool
}

func New(store Store, missing func(error) bool) *Manager {
	return &Manager{paused: make(map[string]bool), store: store, missing: missing, http: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, issuer: "https://auth.openai.com"}
}
func recordKey(id string) string { return "chatgpt." + id }
func (m *Manager) load(ctx context.Context, id string) (credential, error) {
	raw, err := m.store.Get(ctx, recordKey(id))
	if err != nil {
		return credential{}, err
	}
	var c credential
	if json.Unmarshal([]byte(raw), &c) != nil {
		return c, failure("credentials_unavailable")
	}
	return c, nil
}
func (m *Manager) save(ctx context.Context, id string, c credential) error {
	raw, _ := json.Marshal(c)
	if err := m.store.Set(ctx, recordKey(id), string(raw)); err != nil {
		return failure("credentials_unavailable")
	}
	return nil
}
func hasScope(scope, want string) bool {
	for _, s := range strings.Fields(scope) {
		if s == want {
			return true
		}
	}
	return false
}
func randomText() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", failure("unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
func (m *Manager) host(ctx context.Context) (string, error) {
	const key = "chatgpt.host"
	host, err := m.store.Get(ctx, key)
	if err == nil {
		if !strings.HasPrefix(host, "urn:uuid:") {
			return "", failure("credentials_unavailable")
		}
		return host, nil
	}
	if m.missing == nil || !m.missing(err) {
		return "", failure("credentials_unavailable")
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", failure("unavailable")
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	host = fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
	if err := m.store.Set(ctx, key, host); err != nil {
		return "", failure("credentials_unavailable")
	}
	return host, nil
}

// Authorize owns a loopback server for at most five minutes. Cancellation joins
// the server goroutine before returning, including failed browser launch.
func (m *Manager) Authorize(parent context.Context, id string, open func(string) error) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	m.mu.Lock()
	previous, loadErr := m.load(ctx, id)
	if loadErr != nil && (m.missing == nil || !m.missing(loadErr)) {
		m.mu.Unlock()
		return failure("credentials_unavailable")
	}
	host, err := m.host(ctx)
	m.mu.Unlock()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return failure("unavailable")
	}
	state, err := randomText()
	if err != nil {
		listener.Close()
		return err
	}
	nonce, err := randomText()
	if err != nil {
		listener.Close()
		return err
	}
	verifier, err := randomText()
	if err != nil {
		listener.Close()
		return err
	}
	redirect := "http://" + listener.Addr().String() + "/auth/callback"
	type callback struct {
		code, clientID string
		err            error
	}
	callbacks := make(chan callback, 1)
	var once sync.Once
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, MaxHeaderBytes: 16 << 10}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		if r.Method != http.MethodGet || r.URL.Path != "/auth/callback" || r.Host != listener.Addr().String() || len(r.URL.RawQuery) > 16384 {
			w.WriteHeader(400)
			return
		}
		q, parseErr := url.ParseQuery(r.URL.RawQuery)
		if parseErr != nil || len(q["state"]) != 1 || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			w.WriteHeader(400)
			return
		}
		result := callback{code: q.Get("code"), clientID: q.Get("client_id")}
		if q.Get("error") != "" {
			result.err = failure("access_denied")
		}
		if result.err == nil && (len(q["code"]) != 1 || result.code == "" || len(q["client_id"]) > 1) {
			result.err = failure("invalid_callback")
		}
		if previous.ClientID != "" {
			if result.clientID != "" && result.clientID != previous.ClientID {
				result.err = failure("identity_mismatch")
			}
			result.clientID = previous.ClientID
		}
		if result.err == nil && (!strings.HasPrefix(result.clientID, "oaiapp_") || len(result.clientID) > 256) {
			result.err = failure("invalid_callback")
		}
		once.Do(func() { callbacks <- result })
		w.WriteHeader(204)
	})
	done := make(chan struct{})
	go func() { defer close(done); _ = server.Serve(listener) }()
	defer func() { _ = server.Close(); <-done }()
	challenge := sha256.Sum256([]byte(verifier))
	clientID := previous.ClientID
	if clientID == "" {
		clientID = "dynamic_agent_client"
	}
	query := url.Values{"client_id": {clientID}, "ext_agent_host_id": {host}, "response_type": {"code"}, "redirect_uri": {redirect}, "scope": {Scopes}, "resource": {Resource}, "state": {state}, "nonce": {nonce}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}}
	if previous.ClientID == "" {
		query.Set("agent_name_hint", "Daygo")
	}
	if err := open(m.issuer + "/api/accounts/authorize?" + query.Encode()); err != nil {
		return failure("browser_unavailable")
	}
	var reply callback
	select {
	case <-ctx.Done():
		return ctx.Err()
	case reply = <-callbacks:
	}
	if reply.err != nil {
		return reply.err
	}
	token, err := m.exchange(ctx, url.Values{"grant_type": {"authorization_code"}, "client_id": {reply.clientID}, "code": {reply.code}, "code_verifier": {verifier}, "redirect_uri": {redirect}, "resource": {Resource}})
	if err != nil {
		return err
	}
	claims, err := m.verify(ctx, token.IDToken, reply.clientID, nonce)
	if err != nil {
		return err
	}
	if previous.Subject != "" && previous.Subject != claims.Subject {
		return failure("identity_mismatch")
	}
	c := credential{Confirmed: previous.Confirmed, ClientID: reply.clientID, Subject: claims.Subject, Email: claims.Email, Scope: token.Scope}
	enabled := hasScope(token.Scope, planScope) && hasScope(token.Scope, "resource.invoke")
	if enabled && token.RefreshToken == "" {
		return failure("invalid_token")
	}
	if enabled {
		c.AccessToken = token.AccessToken
		c.RefreshToken = token.RefreshToken
		c.IDToken = token.IDToken
		c.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.save(ctx, id, c); err != nil {
		return err
	}
	delete(m.paused, id)
	if !enabled {
		return failure("plan_permission_required")
	}
	return nil
}
func (m *Manager) exchange(ctx context.Context, form url.Values) (tokenResponse, error) {
	raw, status, err := m.request(ctx, http.MethodPost, m.issuer+"/api/accounts/oauth/token", form)
	if err != nil {
		return tokenResponse{}, err
	}
	if status != 200 {
		var p struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &p)
		switch p.Error {
		case "invalid_client":
			return tokenResponse{}, failure("invalid_client")
		case "invalid_grant", "invalid_refresh_token", "token_expired", "refresh_token_expired", "refresh_token_invalidated", "refresh_token_reused":
			return tokenResponse{}, failure("reauthorize")
		}
		return tokenResponse{}, failure("unavailable")
	}
	var t tokenResponse
	if json.Unmarshal(raw, &t) != nil || t.Type != "Bearer" || t.AccessToken == "" || t.ExpiresIn < 1 || t.ExpiresIn > 86400 || strings.ContainsAny(t.AccessToken, "\r\n") {
		return t, failure("invalid_token")
	}
	return t, nil
}
func (m *Manager) Token(ctx context.Context, id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.paused[id] {
		return "", ai.NewError(ai.ErrorRateLimited, "ChatGPT subscription usage limit reached", 0, ai.ErrSubscriptionLimit)
	}
	c, err := m.load(ctx, id)
	if err != nil || c.AccessToken == "" || !hasScope(c.Scope, planScope) {
		return "", ai.NewError(ai.ErrorAuthentication, "ChatGPT authorization is required", 0, nil)
	}
	if c.Paused {
		return "", ai.NewError(ai.ErrorRateLimited, "ChatGPT subscription usage limit reached", 0, ai.ErrSubscriptionLimit)
	}
	if time.Now().Before(c.ExpiresAt.Add(-time.Minute)) {
		return c.AccessToken, nil
	}
	if c.RefreshToken == "" {
		return "", ai.NewError(ai.ErrorAuthentication, "ChatGPT authorization must be renewed", 0, nil)
	}
	t, err := m.exchange(ctx, url.Values{"grant_type": {"refresh_token"}, "client_id": {c.ClientID}, "refresh_token": {c.RefreshToken}, "resource": {Resource}})
	if err != nil {
		if Code(err) == "reauthorize" {
			c.AccessToken = ""
			c.RefreshToken = ""
			c.IDToken = ""
			_ = m.save(ctx, id, c)
			return "", ai.NewError(ai.ErrorAuthentication, "ChatGPT authorization must be renewed", 0, nil)
		}
		return "", ai.NewError(ai.ErrorUnavailable, "ChatGPT authorization refresh is unavailable", 0, err)
	}
	if t.RefreshToken == "" {
		return "", ai.NewError(ai.ErrorAuthentication, "ChatGPT renewal did not return a replacement credential", 0, nil)
	}
	if t.Scope != "" {
		c.Scope = t.Scope
	}
	if !hasScope(c.Scope, planScope) || !hasScope(c.Scope, "resource.invoke") {
		return "", ai.NewError(ai.ErrorAuthentication, "ChatGPT plan permission is required", 0, nil)
	}
	c.AccessToken = t.AccessToken
	c.RefreshToken = t.RefreshToken
	c.ExpiresAt = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	if t.IDToken != "" {
		claims, err := m.verify(ctx, t.IDToken, c.ClientID, "")
		if err != nil || claims.Subject != c.Subject {
			return "", ai.NewError(ai.ErrorAuthentication, "ChatGPT renewal identity was rejected", 0, nil)
		}
		c.IDToken = t.IDToken
	}
	if err := m.save(ctx, id, c); err != nil {
		return "", ai.NewError(ai.ErrorUnavailable, "ChatGPT renewed credentials could not be saved", 0, nil)
	}
	return c.AccessToken, nil
}

// Account is status only and never refreshes credentials (safe for read-only UI).
func (m *Manager) Account(ctx context.Context, id string) (email string, connected, paused bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.load(ctx, id)
	return c.Email, err == nil && c.AccessToken != "" && hasScope(c.Scope, planScope), c.Paused || m.paused[id]
}
func (m *Manager) Pause(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.paused[id] = true
	c, err := m.load(ctx, id)
	if err != nil {
		return failure("credentials_unavailable")
	}
	c.Paused = true
	return m.save(ctx, id, c)
}
func (m *Manager) Resume(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.load(ctx, id)
	if err != nil {
		return failure("credentials_unavailable")
	}
	c.Paused = false
	if err := m.save(ctx, id, c); err != nil {
		return err
	}
	delete(m.paused, id)
	return nil
}
func (m *Manager) Disconnect(ctx context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.load(ctx, id)
	if err != nil {
		return false, failure("credentials_unavailable")
	}
	revoked := c.RefreshToken == "" && c.AccessToken == ""
	if c.RefreshToken != "" {
		var config discovery
		if m.discovery(ctx, &config) == nil {
			_, status, err := m.request(ctx, http.MethodPost, config.Revocation, url.Values{"token": {c.RefreshToken}, "token_type_hint": {"refresh_token"}, "client_id": {c.ClientID}})
			revoked = err == nil && status == 200
		}
	}
	c.AccessToken = ""
	c.RefreshToken = ""
	c.IDToken = ""
	c.Paused = false
	// Remote revocation can consume the caller's deadline. Local sign-out must
	// still get its own bounded opportunity to clear the active token record.
	localCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := m.save(localCtx, id, c); err != nil {
		return revoked, err
	}
	delete(m.paused, id)
	return revoked, nil
}
func (m *Manager) Forget(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store.Delete(ctx, recordKey(id))
}
func (m *Manager) request(ctx context.Context, method, endpoint string, form url.Values) ([]byte, int, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	r, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, 0, failure("unavailable")
	}
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response, err := m.http.Do(r)
	if err != nil {
		if ctx.Err() != nil {
			return nil, 0, ctx.Err()
		}
		return nil, 0, failure("unavailable")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return nil, response.StatusCode, failure("invalid_response")
	}
	return raw, response.StatusCode, nil
}

func (m *Manager) NeedsConfirmation(ctx context.Context, id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.load(ctx, id)
	return err == nil && c.AccessToken != "" && !c.Confirmed
}
func (m *Manager) Confirm(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.load(ctx, id)
	if err != nil {
		return failure("credentials_unavailable")
	}
	c.Confirmed = true
	return m.save(ctx, id, c)
}
