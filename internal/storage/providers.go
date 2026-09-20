package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// Provider is one row of the providers table. Storage stores what it is given;
// protocol membership, endpoint shape and the image-cap range are validated by
// the caller layer (internal/settings owns normalization, docs/05 §5.6.3
// rule 2 — the same split SettingsRepo uses). MaxImages caps the image parts
// one request may carry; 0 means the ai.MaxImages default.
//
// Models is the ordered list of models configured under this provider's single
// endpoint and key (decisions/providers-multi-model). It is stored as a JSON
// array in the `models` column; the routing chain references a (provider, model)
// pair, so a provider with several models contributes several chain entries.
type Provider struct {
	ID          string
	DisplayName string
	Protocol    string
	Endpoint    string
	Models      []string
	MaxImages   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ProviderRepo is the typed access layer over providers (docs/03 §3.3.5).
// Routing is not a column here: it lives in app_settings under
// providers.routing, so provider rows carry no ordering of their own.
type ProviderRepo struct {
	store *Store
}

// Providers returns the repository bound to this store.
func (s *Store) Providers() *ProviderRepo {
	if s == nil {
		return nil
	}
	return &ProviderRepo{store: s}
}

// List returns every provider ordered by display name then id, so the caller
// gets a stable order without imposing a user-visible sort column.
func (r *ProviderRepo) List(ctx context.Context) ([]Provider, error) {
	var out []Provider
	err := r.store.Read(ctx, "providers list", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT id, display_name, protocol, endpoint, models, max_images, created_at, updated_at
			 FROM providers ORDER BY display_name, id`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			p, err := scanProvider(rows)
			if err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns one provider by id.
func (r *ProviderRepo) Get(ctx context.Context, id string) (Provider, error) {
	var p Provider
	err := r.store.Read(ctx, "providers get", func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			`SELECT id, display_name, protocol, endpoint, models, max_images, created_at, updated_at
			 FROM providers WHERE id = ?`, id)
		var err error
		p, err = scanProvider(row)
		return err
	})
	if err != nil {
		return Provider{}, err
	}
	return p, nil
}

// Add inserts a provider. CreatedAt and UpdatedAt are both set to now; the
// caller does not supply timestamps.
func (r *ProviderRepo) Add(ctx context.Context, p Provider) error {
	at := r.store.now()
	models, err := encodeModels(p.Models)
	if err != nil {
		return err
	}
	return r.store.Write(ctx, "providers add", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO providers (id, display_name, protocol, endpoint, models, max_images, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			p.ID, p.DisplayName, p.Protocol, p.Endpoint, models, p.MaxImages, at.Unix(), at.Unix())
		return err
	})
}

// Update replaces the mutable fields of one provider and bumps updated_at.
// CreatedAt from p is ignored: creation time never changes.
func (r *ProviderRepo) Update(ctx context.Context, id string, p Provider) error {
	at := r.store.now()
	models, err := encodeModels(p.Models)
	if err != nil {
		return err
	}
	return r.store.Write(ctx, "providers update", func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE providers
			 SET display_name = ?, protocol = ?, endpoint = ?, models = ?, max_images = ?, updated_at = ?
			 WHERE id = ?`,
			p.DisplayName, p.Protocol, p.Endpoint, models, p.MaxImages, at.Unix(), id)
		if err != nil {
			return err
		}
		return requireUpdated(res, "providers update")
	})
}

// Delete removes one provider. Deleting an absent id is a not-found error, not
// a silent success: callers prune the routing chain and secrets alongside this
// call, and a silent no-op would let them skip that pruning for a typo'd id.
func (r *ProviderRepo) Delete(ctx context.Context, id string) error {
	return r.store.Write(ctx, "providers delete", func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM providers WHERE id = ?`, id)
		if err != nil {
			return err
		}
		return requireUpdated(res, "providers delete")
	})
}

func scanProvider(row scanner) (Provider, error) {
	var p Provider
	var models string
	var createdAt, updatedAt int64
	if err := row.Scan(&p.ID, &p.DisplayName, &p.Protocol, &p.Endpoint, &models, &p.MaxImages, &createdAt, &updatedAt); err != nil {
		return Provider{}, err
	}
	decoded, err := decodeModels(models)
	if err != nil {
		return Provider{}, err
	}
	p.Models = decoded
	p.CreatedAt = time.Unix(createdAt, 0)
	p.UpdatedAt = time.Unix(updatedAt, 0)
	return p, nil
}

// encodeModels renders the models list for the JSON column. A nil slice encodes
// as "[]" rather than "null", so the stored form is always a JSON array.
func encodeModels(models []string) (string, error) {
	if models == nil {
		models = []string{}
	}
	encoded, err := json.Marshal(models)
	if err != nil {
		return "", wrap("encode provider models", err)
	}
	return string(encoded), nil
}

// decodeModels parses the JSON column. An empty column (a provider written
// before the models column existed cannot reach here, but a defensive empty
// string still decodes) yields an empty slice.
func decodeModels(raw string) ([]string, error) {
	if raw == "" {
		return []string{}, nil
	}
	var models []string
	if err := json.Unmarshal([]byte(raw), &models); err != nil {
		return nil, wrap("decode provider models", err)
	}
	if models == nil {
		models = []string{}
	}
	return models, nil
}

// requireUpdated turns an UPDATE/DELETE that matched zero rows into a
// not-found error.
func requireUpdated(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return newError(KindNotFound, op)
	}
	return nil
}
