package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
)

const (
	PendingCaptureState   = "pending"
	CommittedCaptureState = "committed"
	BlockedCaptureState   = "blocked"
)

type PendingCapture struct {
	ID            int64
	RelativePath  string
	CapturedAt    time.Time
	IdleSeconds   *int
	Width, Height int
	Redacted      bool
	FileSize      int64
	State         string
}
type CaptureRepo struct{ store *Store }

func (s *Store) Captures() *CaptureRepo {
	if s == nil {
		return nil
	}
	return &CaptureRepo{store: s}
}

func (r *CaptureRepo) Begin(ctx context.Context, relativePath string, capturedAt time.Time, idle *int, width, height int, redacted bool) (int64, error) {
	if r == nil || r.store == nil {
		return 0, fmt.Errorf("captures: store unavailable")
	}
	if !platform.ValidSegmentPath(relativePath) || width < 1 || height < 1 {
		return 0, fmt.Errorf("captures: invalid pending capture")
	}
	var id int64
	err := r.store.Write(ctx, "capture begin", func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO pending_captures(relative_path,captured_at,idle_seconds,width,height,redacted,state,created_at) VALUES(?,?,?,?,?,?,?,?)`, relativePath, capturedAt.Unix(), idle, width, height, boolInt(redacted), PendingCaptureState, time.Now().Unix())
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	return id, err
}

func (r *CaptureRepo) Commit(ctx context.Context, id int64, fileSize int64) error {
	if r == nil || r.store == nil {
		return fmt.Errorf("captures: store unavailable")
	}
	if id < 1 || fileSize < 1 {
		return fmt.Errorf("captures: invalid commit")
	}
	return r.store.Write(ctx, "capture commit", func(ctx context.Context, tx *sql.Tx) error {
		var state string
		if err := tx.QueryRowContext(ctx, `SELECT state FROM pending_captures WHERE id=?`, id).Scan(&state); err != nil {
			return err
		}
		if state == CommittedCaptureState {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO screenshots(segment_path,frame_index,captured_at,idle_seconds_at_capture,width,height,redacted,file_size) SELECT relative_path,0,captured_at,idle_seconds,width,height,redacted,? FROM pending_captures WHERE id=? ON CONFLICT(segment_path,frame_index) DO NOTHING`, fileSize, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE pending_captures SET state=?,file_size=? WHERE id=?`, CommittedCaptureState, fileSize, id)
		return err
	})
}

func (r *CaptureRepo) MarkBlocked(ctx context.Context, id int64) error {
	if r == nil || r.store == nil {
		return fmt.Errorf("captures: store unavailable")
	}
	return r.store.Write(ctx, "capture blocked", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE pending_captures SET state=? WHERE id=? AND state=?`, BlockedCaptureState, id, PendingCaptureState)
		return err
	})
}

func (r *CaptureRepo) Pending(ctx context.Context) ([]PendingCapture, error) {
	if r == nil || r.store == nil {
		return nil, fmt.Errorf("captures: store unavailable")
	}
	var out []PendingCapture
	err := r.store.Read(ctx, "capture pending", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT id,relative_path,captured_at,idle_seconds,width,height,redacted,file_size,state FROM pending_captures WHERE state=? ORDER BY id`, PendingCaptureState)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p PendingCapture
			var ts int64
			var idle sql.NullInt64
			var red int
			if err := rows.Scan(&p.ID, &p.RelativePath, &ts, &idle, &p.Width, &p.Height, &red, &p.FileSize, &p.State); err != nil {
				return err
			}
			p.CapturedAt = time.Unix(ts, 0)
			if idle.Valid {
				x := int(idle.Int64)
				p.IdleSeconds = &x
			}
			p.Redacted = red != 0
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, err
}

func (r *CaptureRepo) Reconcile(ctx context.Context) error {
	pending, err := r.Pending(ctx)
	if err != nil {
		return err
	}
	root := filepath.Join(filepath.Dir(r.store.Path()), "recordings")
	for _, p := range pending {
		info, e := os.Stat(filepath.Join(root, p.RelativePath))
		if os.IsNotExist(e) {
			e = r.store.Write(ctx, "capture reconcile missing", func(ctx context.Context, tx *sql.Tx) error {
				_, x := tx.ExecContext(ctx, `DELETE FROM pending_captures WHERE id=? AND state=?`, p.ID, PendingCaptureState)
				return x
			})
			if e != nil {
				return e
			}
		} else if e != nil {
			return e
		} else if info.Size() > 0 {
			if e = r.Commit(ctx, p.ID, info.Size()); e != nil {
				return e
			}
		}
	}
	return nil
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
