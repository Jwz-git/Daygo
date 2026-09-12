package storage

import (
	"context"
	"database/sql"
	"time"
)

// Provider is one row of the providers table. Storage stores what it is given;
// protocol membership and endpoint shape are validated by the caller layer
// (internal/settings owns normalization, docs/05 §5.6.3 rule 2 — the same
// split SettingsRepo uses).
type Provider struct {
	ID          string
	DisplayName string
	Protocol    string
	Endpoint    string
	Model       string
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
			`SELECT id, display_name, protocol, endpoint, model, created_at, updated_at
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
			`SELECT id, display_name, protocol, endpoint, model, created_at, updated_at
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
	return r.store.Write(ctx, "providers add", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO providers (id, display_name, protocol, endpoint, model, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			p.ID, p.DisplayName, p.Protocol, p.Endpoint, p.Model, at.Unix(), at.Unix())
		return err
	})
}

// Update replaces the mutable fields of one provider and bumps updated_at.
// CreatedAt from p is ignored: creation time never changes.
func (r *ProviderRepo) Update(ctx context.Context, id string, p Provider) error {
	at := r.store.now()
	return r.store.Write(ctx, "providers update", func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE providers
			 SET display_name = ?, protocol = ?, endpoint = ?, model = ?, updated_at = ?
			 WHERE id = ?`,
			p.DisplayName, p.Protocol, p.Endpoint, p.Model, at.Unix(), id)
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
	var createdAt, updatedAt int64
	if err := row.Scan(&p.ID, &p.DisplayName, &p.Protocol, &p.Endpoint, &p.Model, &createdAt, &updatedAt); err != nil {
		return Provider{}, err
	}
	p.CreatedAt = time.Unix(createdAt, 0)
	p.UpdatedAt = time.Unix(updatedAt, 0)
	return p, nil
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
