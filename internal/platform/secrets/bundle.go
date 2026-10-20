package secrets

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Jwz-git/Daygo/internal/platform"
)

// BundleStore commits a bounded credential bundle through a small manifest.
// Immutable chunks fit Windows Credential Manager's 2560-byte blob limit.
// All values remain in the system credential store. Callers serialize access.
type BundleStore struct{ store platform.Secrets }
type bundleManifest struct {
	Generation string `json:"generation"`
	Count      int    `json:"count"`
}

func NewBundleStore(store platform.Secrets) *BundleStore { return &BundleStore{store: store} }
func (b *BundleStore) manifest(ctx context.Context, key string) (bundleManifest, error) {
	raw, err := b.store.Get(ctx, key)
	if err != nil {
		return bundleManifest{}, err
	}
	var m bundleManifest
	if json.Unmarshal([]byte(raw), &m) != nil || len(m.Generation) != 32 || m.Count < 1 || m.Count > 128 {
		return m, &SecretsError{Code: SecretNative, Provider: key}
	}
	if _, err := hex.DecodeString(m.Generation); err != nil {
		return m, &SecretsError{Code: SecretNative, Provider: key}
	}
	return m, nil
}
func chunkKey(key string, m bundleManifest, i int) string {
	return fmt.Sprintf("%s.chunk.%s.%d", key, m.Generation, i)
}
func (b *BundleStore) Get(ctx context.Context, key string) (string, error) {
	m, err := b.manifest(ctx, key)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for i := 0; i < m.Count; i++ {
		part, err := b.store.Get(ctx, chunkKey(key, m, i))
		if err != nil {
			if IsNotFound(err) {
				return "", &SecretsError{Code: SecretNative, Provider: key}
			}
			return "", err
		}
		if len(part) > 2000 {
			return "", &SecretsError{Code: SecretNative, Provider: key}
		}
		out.WriteString(part)
	}
	decoded, err := base64.RawStdEncoding.DecodeString(out.String())
	if err != nil {
		return "", &SecretsError{Code: SecretNative, Provider: key}
	}
	return string(decoded), nil
}
func (b *BundleStore) Set(ctx context.Context, key, value string) error {
	if len(value) == 0 || len(value) > 180000 {
		return &SecretsError{Code: SecretInvalidArgument, Provider: key}
	}
	value = base64.RawStdEncoding.EncodeToString([]byte(value))
	old, err := b.manifest(ctx, key)
	if err != nil && !IsNotFound(err) {
		return err
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return &SecretsError{Code: SecretNative, Provider: key}
	}
	m := bundleManifest{Generation: hex.EncodeToString(id[:]), Count: (len(value) + 1999) / 2000}
	written := 0
	for i := 0; i < m.Count; i++ {
		end := min((i+1)*2000, len(value))
		if err := b.store.Set(ctx, chunkKey(key, m, i), value[i*2000:end]); err != nil {
			b.cleanup(ctx, key, m, written)
			return err
		}
		written++
	}
	raw, _ := json.Marshal(m)
	if err := b.store.Set(ctx, key, string(raw)); err != nil {
		b.cleanup(ctx, key, m, written)
		return err
	}
	b.cleanup(ctx, key, old, old.Count)
	return nil
}
func (b *BundleStore) Delete(ctx context.Context, key string) error {
	m, err := b.manifest(ctx, key)
	if err != nil {
		return err
	}
	// Removing the manifest first immediately makes the whole bundle unusable.
	if err := b.store.Delete(ctx, key); err != nil {
		return err
	}
	b.cleanup(ctx, key, m, m.Count)
	return nil
}
func (b *BundleStore) cleanup(ctx context.Context, key string, m bundleManifest, count int) {
	for i := 0; i < count; i++ {
		_ = b.store.Delete(ctx, chunkKey(key, m, i))
	}
}
