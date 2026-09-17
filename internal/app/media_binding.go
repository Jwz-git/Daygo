package app

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/factory"
)

/*
 * Card media: playback of a card's timespan as the discrete screenshots the
 * recorder actually stored. Frames are served by numeric ID only — the HTTP
 * handler re-resolves the segment path server-side and pins it inside the
 * recordings root (AGENTS.md 安全: 资源处理器只接受数字 ID，解析后的路径必须
 * 落在录制目录内，否则 403).
 */

// mediaFrameLimit matches storage's cap; the two must stay in agreement so
// the binding response is never truncated differently per layer.
const mediaFrameLimit = 600

// CardMediaFrameDTO is one playable frame: its resource ID and capture time.
type CardMediaFrameDTO struct {
	ID         int64 `json:"id"`
	CapturedAt int64 `json:"capturedAt"`
}

// CardMediaDTO lists the frames covering a card's timespan, oldest first.
type CardMediaDTO struct {
	CardID int64               `json:"cardId"`
	Frames []CardMediaFrameDTO `json:"frames"`
}

// attachMedia wires the file-backed Media implementation once the recording
// directory is known. It runs during startup before any request can arrive,
// so no lock guards the fields.
func (b *Backend) attachMedia(root string) {
	b.media = factory.NewMedia(root)
	b.mediaRoot = root
}

// GetCardMedia returns the frame listing for one card. A card without frames
// (analysis ran before recording, or the frames were cleaned up) returns an
// empty list rather than an error — the player renders its placeholder.
func (b *Backend) GetCardMedia(cardID int64) (CardMediaDTO, error) {
	if cardID <= 0 {
		return CardMediaDTO{}, apperr.E(apperr.InvalidArgument, "invalid card id", nil)
	}
	store := b.store()
	if store == nil {
		return CardMediaDTO{}, apperr.E(apperr.DatabaseError, "media requires a database", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	card, err := store.Cards().CardByID(ctx, cardID)
	if err != nil {
		return CardMediaDTO{}, apperr.E(apperr.NotFound, "card not found", err)
	}
	refs, err := store.Captures().FramesInRange(ctx, card.StartTs, card.EndTs, mediaFrameLimit)
	if err != nil {
		return CardMediaDTO{}, apperr.E(apperr.DatabaseError, "frame listing failed", err)
	}
	frames := make([]CardMediaFrameDTO, 0, len(refs))
	for _, ref := range refs {
		frames = append(frames, CardMediaFrameDTO{ID: ref.ID, CapturedAt: ref.CapturedAt})
	}
	return CardMediaDTO{CardID: cardID, Frames: frames}, nil
}

// serveFrame is the asset-server fallback handler: GET /media/frame?id=NNN
// streams one screenshot. Everything else is 404. IDs are digits only, the
// resolved path is pinned inside the recordings root by the Media adapter,
// and frames are immutable once written, so responses may be cached.
func (b *Backend) serveFrame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/media/frame" {
		http.NotFound(w, r)
		return
	}
	media, _ := b.mediaSnapshot()
	if media == nil {
		http.NotFound(w, r)
		return
	}
	rawID := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 || len(rawID) > 18 {
		http.Error(w, "invalid frame id", http.StatusBadRequest)
		return
	}

	store := b.store()
	if store == nil {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	segmentPath, frameIndex, err := store.Captures().FrameLocation(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data, err := media.DecodeFrame(ctx, platform.DecodeRequest{SegmentPath: segmentPath, FrameIndex: frameIndex})
	if err != nil {
		// The row can outlive its file (cleanup removes whole segments
		// asynchronously); that is a missing resource, not a server error.
		log.Printf("media frame %d unavailable: %v", id, err)
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

// mediaSnapshot returns the wired Media implementation and root, or nil when
// media is unavailable (headless construction, no storage).
func (b *Backend) mediaSnapshot() (platform.Media, string) {
	b.mediaMu.RLock()
	defer b.mediaMu.RUnlock()
	return b.media, b.mediaRoot
}
