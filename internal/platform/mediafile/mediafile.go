// Package mediafile implements platform.Media for the file-backed frame
// store: every capture is a self-contained JPEG under the recordings root,
// so "decoding" a frame is a size-capped read of that file. Pixels stay
// behind the Media port (AGENTS.md: 像素读取必须经 platform.Media), paths
// from callers are treated as untrusted relative paths and resolved inside
// the root only. The port's EncodeVideo stays unimplemented — video encoding
// is the pending M2 codec decision (docs/09 §9.8), and nothing here presumes
// its outcome.
package mediafile

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // segment frames are JPEGs written by the capture adapter
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Jwz-git/Daygo/internal/platform"
)

// maxFrameBytes bounds one decoded frame. A screenshot at the tallest
// supported capture height encodes far below this; the cap turns a corrupted
// or planted file into an error instead of unbounded memory.
const maxFrameBytes = 64 << 20

// Media reads frames from JPEG segment files under a fixed recordings root.
type Media struct {
	root string
}

// New returns a Media rooted at dir. The root must be the recordings
// directory; every resolved path is required to stay inside it.
func New(dir string) Media {
	return Media{root: dir}
}

// resolve joins a slash-separated segment path onto the root and rejects any
// escape (absolute paths, "..", drive tricks) before touching the filesystem.
func (m Media) resolve(segmentPath string) (string, error) {
	if m.root == "" {
		return "", fmt.Errorf("mediafile: media root is not configured")
	}
	if segmentPath == "" || strings.ContainsRune(segmentPath, 0) {
		return "", fmt.Errorf("mediafile: invalid segment path")
	}
	clean := path.Clean("/" + segmentPath) // forces the path under the root
	rel := strings.TrimPrefix(clean, "/")
	abs := filepath.Join(m.root, filepath.FromSlash(rel))
	if abs == m.root || !strings.HasPrefix(abs, m.root+string(filepath.Separator)) {
		return "", fmt.Errorf("mediafile: segment path escapes recordings root")
	}
	return abs, nil
}

// DecodeFrame returns the raw encoded bytes of one frame. For JPEG segment
// files the frame index is always 0 (one file per frame, per the recorder).
func (m Media) DecodeFrame(ctx context.Context, req platform.DecodeRequest) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	abs, err := m.resolve(req.SegmentPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("mediafile: frame unavailable: %w", err)
	}
	if info.Size() > maxFrameBytes {
		return nil, fmt.Errorf("mediafile: frame %d bytes exceeds the %d byte cap", info.Size(), maxFrameBytes)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("mediafile: frame read failed: %w", err)
	}
	return data, nil
}

// DecodeFrames reads a batch of frames. A failed frame fails the batch with
// its index in the error; callers that tolerate gaps should call
// DecodeFrame per frame instead.
func (m Media) DecodeFrames(ctx context.Context, reqs []platform.DecodeRequest) ([][]byte, error) {
	frames := make([][]byte, 0, len(reqs))
	for _, req := range reqs {
		data, err := m.DecodeFrame(ctx, req)
		if err != nil {
			return nil, err
		}
		frames = append(frames, data)
	}
	return frames, nil
}

// ProbeSegment reports the geometry of a one-frame JPEG segment. Multi-frame
// segments belong to the pending codec decision and report as unreadable.
func (m Media) ProbeSegment(_ context.Context, segmentPath string) (platform.SegmentInfo, error) {
	abs, err := m.resolve(segmentPath)
	if err != nil {
		return platform.SegmentInfo{}, err
	}
	file, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return platform.SegmentInfo{Readable: false}, nil
		}
		return platform.SegmentInfo{}, fmt.Errorf("mediafile: probe failed: %w", err)
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return platform.SegmentInfo{Readable: false}, nil
	}
	return platform.SegmentInfo{FrameCount: 1, Width: config.Width, Height: config.Height, Readable: true}, nil
}

// EncodeVideo is the pending M2 codec decision; nothing may pick an encoding
// by silently implementing it here (docs/decisions + AGENTS.md 待定设计).
func (m Media) EncodeVideo(_ context.Context, _ platform.EncodeRequest) (platform.EncodeResult, error) {
	return platform.EncodeResult{}, fmt.Errorf("mediafile: video encoding awaits the codec decision")
}
