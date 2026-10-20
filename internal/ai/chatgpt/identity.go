package chatgpt

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/url"
	"strings"
	"time"
)

type discovery struct {
	Issuer     string `json:"issuer"`
	JWKS       string `json:"jwks_uri"`
	Revocation string `json:"revocation_endpoint"`
}

func (m *Manager) discovery(ctx context.Context, out *discovery) error {
	raw, status, err := m.request(ctx, "GET", m.issuer+"/.well-known/openid-configuration", nil)
	if err != nil || status != 200 || json.Unmarshal(raw, out) != nil || out.Issuer != m.issuer || !m.trustedEndpoint(out.JWKS) || !m.trustedEndpoint(out.Revocation) {
		return failure("identity_unavailable")
	}
	return nil
}
func (m *Manager) trustedEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	root, _ := url.Parse(m.issuer)
	return err == nil && u.Scheme == root.Scheme && u.Host == root.Host && u.User == nil && u.RawQuery == "" && u.Fragment == ""
}

type identity struct {
	AuthorizedParty string          `json:"azp"`
	Subject         string          `json:"sub"`
	Issuer          string          `json:"iss"`
	Audience        json.RawMessage `json:"aud"`
	Expires         int64           `json:"exp"`
	NotBefore       int64           `json:"nbf"`
	Nonce           string          `json:"nonce"`
	Email           string          `json:"email"`
}

func (m *Manager) verify(ctx context.Context, token, clientID, nonce string) (identity, error) {
	invalid := func() (identity, error) { return identity{}, failure("identity_rejected") }
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return invalid()
	}
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return invalid()
	}
	var h struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if json.Unmarshal(header, &h) != nil || h.Alg != "RS256" || h.Kid == "" {
		return invalid()
	}
	var d discovery
	if err := m.discovery(ctx, &d); err != nil {
		return identity{}, err
	}
	raw, status, err := m.request(ctx, "GET", d.JWKS, nil)
	if err != nil || status != 200 {
		return identity{}, failure("identity_unavailable")
	}
	var jwks struct {
		Keys []struct {
			Type string `json:"kty"`
			ID   string `json:"kid"`
			Alg  string `json:"alg"`
			Use  string `json:"use"`
			N    string `json:"n"`
			E    string `json:"e"`
		} `json:"keys"`
	}
	if json.Unmarshal(raw, &jwks) != nil {
		return invalid()
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return invalid()
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	valid := false
	for _, k := range jwks.Keys {
		if k.ID != h.Kid || k.Type != "RSA" || (k.Alg != "" && k.Alg != "RS256") || (k.Use != "" && k.Use != "sig") {
			continue
		}
		n, ne := base64.RawURLEncoding.DecodeString(k.N)
		e, ee := base64.RawURLEncoding.DecodeString(k.E)
		if ne != nil || ee != nil || len(n) < 256 || len(n) > 1024 || len(e) > 4 {
			continue
		}
		exponent := new(big.Int).SetBytes(e).Int64()
		if exponent < 3 || exponent > 1<<31-1 {
			continue
		}
		valid = rsa.VerifyPKCS1v15(&rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(exponent)}, crypto.SHA256, digest[:], sig) == nil
		if valid {
			break
		}
	}
	if !valid {
		return invalid()
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return invalid()
	}
	var c identity
	if json.Unmarshal(payload, &c) != nil || c.Issuer != m.issuer || c.Subject == "" || c.Expires <= time.Now().Unix() || c.NotBefore > time.Now().Unix()+30 || (nonce != "" && c.Nonce != nonce) {
		return invalid()
	}
	var audience string
	var audiences []string
	if json.Unmarshal(c.Audience, &audience) == nil {
		audiences = []string{audience}
	} else if json.Unmarshal(c.Audience, &audiences) != nil {
		return invalid()
	}
	matched := false
	for _, a := range audiences {
		if a == clientID {
			matched = true
		}
	}
	if len(audiences) > 1 && c.AuthorizedParty != clientID {
		return invalid()
	}
	if c.AuthorizedParty != "" && c.AuthorizedParty != clientID {
		return invalid()
	}
	if !matched {
		return invalid()
	}
	return c, nil
}
