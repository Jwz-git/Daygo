package storage

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	FrameIndex    int
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

func (r *CaptureRepo) Begin(ctx context.Context, relativePath string, frameIndex int, capturedAt time.Time, idle *int, width, height int, redacted bool) (int64, error) {
	if r == nil || r.store == nil {
		return 0, fmt.Errorf("captures: store unavailable")
	}
	if !platform.ValidSegmentPath(relativePath) || frameIndex < 0 || width < 1 || height < 1 {
		return 0, fmt.Errorf("captures: invalid pending capture")
	}
	var id int64
	err := r.store.Write(ctx, "capture begin", func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO pending_captures(relative_path,frame_index,captured_at,idle_seconds,width,height,redacted,state,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, relativePath, frameIndex, capturedAt.Unix(), idle, width, height, boolInt(redacted), PendingCaptureState, time.Now().Unix())
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
		if _, err := tx.ExecContext(ctx, `INSERT INTO screenshots(segment_path,frame_index,captured_at,idle_seconds_at_capture,width,height,redacted,file_size) SELECT relative_path,frame_index,captured_at,idle_seconds,width,height,redacted,? FROM pending_captures WHERE id=? ON CONFLICT(segment_path,frame_index) DO NOTHING`, fileSize, id); err != nil {
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
		rows, err := tx.QueryContext(ctx, `SELECT id,relative_path,frame_index,captured_at,idle_seconds,width,height,redacted,file_size,state FROM pending_captures WHERE state=? ORDER BY id`, PendingCaptureState)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p PendingCapture
			var ts int64
			var idle sql.NullInt64
			var red int
			if err := rows.Scan(&p.ID, &p.RelativePath, &p.FrameIndex, &ts, &idle, &p.Width, &p.Height, &red, &p.FileSize, &p.State); err != nil {
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

// Abandon drops a pending intent whose frame will never be written — the
// recorder deleted the file mid-capture because recording was paused before
// the image hit disk. Without it the pending_captures row would outlive its
// file forever (Reconcile only reconciles files that exist or stat cleanly).
func (r *CaptureRepo) Abandon(ctx context.Context, id int64) error {
	if r == nil || r.store == nil {
		return fmt.Errorf("captures: store unavailable")
	}
	return r.store.Write(ctx, "capture abandon", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM pending_captures WHERE id=? AND state=?`, id, PendingCaptureState)
		return err
	})
}

// Reconcile walks pending intents at startup and settles them against the
// filesystem: a file that exists and is non-empty is committed into
// screenshots (the crash-recovery path — the pixels made it to disk but the
// process died before Commit), and a file that is gone is dropped. root is
// the recordings directory the recorder writes into.
func (r *CaptureRepo) Reconcile(ctx context.Context, root string) error {
	pending, err := r.Pending(ctx)
	if err != nil {
		return err
	}
	for _, p := range pending {
		filePath := filepath.Join(root, filepath.FromSlash(p.RelativePath))
		info, e := os.Stat(filePath)
		if os.IsNotExist(e) {
			if e = r.Abandon(ctx, p.ID); e != nil {
				return e
			}
			continue
		} else if e != nil {
			return e
		}

		// For MP4 segments, verify the segment has been finalized (has moov atom).
		// An unfinalized MP4 lacks moov and cannot be read; per decision doc §4,
		// unfinalized frames are dropped and pending intent abandoned.
		ext := strings.ToLower(filepath.Ext(p.RelativePath))
		if ext == ".mp4" {
			if !hasMoovAtom(filePath) {
				if e = r.Abandon(ctx, p.ID); e != nil {
					return e
				}
				continue
			}
		}

		if info.Size() > 0 {
			if e = r.Commit(ctx, p.ID, info.Size()); e != nil {
				return e
			}
			if ext == ".mp4" {
				_ = r.AmortizeSegment(ctx, p.RelativePath, info.Size())
			}
		} else {
			if e = r.Abandon(ctx, p.ID); e != nil {
				return e
			}
		}
	}

	// Also reconcile committed segments: if an MP4 segment on disk lacks a moov
	// atom, it was left unfinalized by an interrupted process and can never be
	// decoded. Mark all its frames as is_deleted = 1 so analysis batches do not
	// repeatedly fail trying to decode unfinalizable files.
	var distinctSegments []string
	if err := r.store.Read(ctx, "capture reconcile distinct segments", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT DISTINCT segment_path FROM screenshots WHERE is_deleted = 0`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var seg string
			if err := rows.Scan(&seg); err != nil {
				return err
			}
			distinctSegments = append(distinctSegments, seg)
		}
		return rows.Err()
	}); err != nil {
		return err
	}

	for _, seg := range distinctSegments {
		if strings.ToLower(filepath.Ext(seg)) != ".mp4" {
			continue
		}
		filePath := filepath.Join(root, filepath.FromSlash(seg))
		if !hasMoovAtom(filePath) {
			_ = r.store.Write(ctx, "capture reconcile mark corrupt segment deleted", func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `UPDATE screenshots SET is_deleted = 1 WHERE segment_path = ?`, seg)
				return err
			})
		}
	}

	return nil
}

// AmortizeSegment redistributes the segment's total file_size evenly across
// all committed, non-deleted frames in that segment (docs/03 §3.4 and AGENTS.md:
// screenshots.file_size is the amortized per-frame share).
func (r *CaptureRepo) AmortizeSegment(ctx context.Context, segmentPath string, totalSize int64) error {
	if r == nil || r.store == nil || segmentPath == "" {
		return nil
	}
	return r.store.Write(ctx, "capture amortize segment", func(ctx context.Context, tx *sql.Tx) error {
		var count int64
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM screenshots
			WHERE segment_path = ? AND is_deleted = 0`, segmentPath).Scan(&count); err != nil {
			return err
		}
		if count <= 0 {
			return nil
		}
		if totalSize <= 0 {
			if err := tx.QueryRowContext(ctx, `
				SELECT COALESCE(MAX(file_size), 0)
				FROM screenshots
				WHERE segment_path = ? AND is_deleted = 0`, segmentPath).Scan(&totalSize); err != nil {
				return err
			}
		}
		if totalSize <= 0 {
			return nil
		}
		perFrame := totalSize / count
		if perFrame <= 0 {
			perFrame = 1
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE screenshots
			SET file_size = ?
			WHERE segment_path = ? AND is_deleted = 0`, perFrame, segmentPath)
		return err
	})
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// FrameRef identifies one committed frame for media playback. IDs are the
// only handle that ever reaches the UI: the resource handler re-resolves the
// path server-side (AGENTS.md: 资源处理器只接受数字 ID).
type FrameRef struct {
	ID         int64
	CapturedAt int64 // unix seconds
}

// maxMediaFrames caps one card's frame listing. A card spans minutes, so the
// cap is far above any real batch window; the stride keeps a pathological
// range from dragging thousands of rows into a binding response.
const maxMediaFrames = 600

// FramesInRange returns committed, non-deleted frames captured in
// [start, end] (unix seconds), evenly sampled down to at most limit entries
// and ordered oldest first.
func (r *CaptureRepo) FramesInRange(ctx context.Context, start, end int64, limit int) ([]FrameRef, error) {
	if r == nil || r.store == nil {
		return nil, fmt.Errorf("captures: store unavailable")
	}
	if limit <= 0 || limit > maxMediaFrames {
		limit = maxMediaFrames
	}
	var out []FrameRef
	err := r.store.Read(ctx, "capture frames in range", func(ctx context.Context, tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM screenshots WHERE captured_at >= ? AND captured_at <= ? AND is_deleted = 0`,
			start, end).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return nil
		}
		stride := (count + limit - 1) / limit
		rows, err := tx.QueryContext(ctx,
			`SELECT id, captured_at FROM (
				 SELECT id, captured_at, ROW_NUMBER() OVER (ORDER BY captured_at, id) AS rn
				 FROM screenshots
				 WHERE captured_at >= ? AND captured_at <= ? AND is_deleted = 0
			 ) WHERE (rn - 1) % ? = 0 ORDER BY captured_at, id`,
			start, end, stride)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ref FrameRef
			if err := rows.Scan(&ref.ID, &ref.CapturedAt); err != nil {
				return err
			}
			out = append(out, ref)
		}
		return rows.Err()
	})
	return out, err
}

// FramePath resolves one frame's relative segment path. Deleted frames
// resolve as not found so an ID that outlived a cleanup cannot serve pixels.
func (r *CaptureRepo) FramePath(ctx context.Context, id int64) (string, error) {
	path, _, err := r.FrameLocation(ctx, id)
	return path, err
}

// FrameLocation resolves one frame's relative segment path and frame index.
func (r *CaptureRepo) FrameLocation(ctx context.Context, id int64) (string, int, error) {
	if r == nil || r.store == nil {
		return "", 0, fmt.Errorf("captures: store unavailable")
	}
	var segmentPath string
	var frameIndex int
	err := r.store.Read(ctx, "capture frame location", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT segment_path, frame_index FROM screenshots WHERE id = ? AND is_deleted = 0`, id,
		).Scan(&segmentPath, &frameIndex)
	})
	if err != nil {
		return "", 0, err
	}
	return segmentPath, frameIndex, nil
}

func hasMoovAtom(filePath string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil || stat.Size() < 8 {
		return false
	}
	fileSize := stat.Size()
	var offset int64

	header := make([]byte, 8)
	for offset+8 <= fileSize {
		if _, err := f.ReadAt(header, offset); err != nil {
			break
		}
		boxSize := int64(binary.BigEndian.Uint32(header[0:4]))
		boxType := string(header[4:8])
		if boxType == "moov" {
			return true
		}
		if boxSize == 1 {
			ext := make([]byte, 8)
			if _, err := f.ReadAt(ext, offset+8); err != nil {
				break
			}
			boxSize = int64(binary.BigEndian.Uint64(ext))
			if boxSize < 16 {
				break
			}
		} else if boxSize == 0 {
			break
		} else if boxSize < 8 {
			break
		}
		offset += boxSize
	}
	return false
}
