//go:build windows

package windows

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/Jwz-git/Daygo/internal/platform"
)

type Media struct{ root string }

func NewMedia(root string) platform.Media { return &Media{root: root} }

func (m *Media) resolve(segmentPath string) (string, error) {
	if m.root == "" || segmentPath == "" || strings.ContainsRune(segmentPath, 0) {
		return "", fmt.Errorf("windows media: invalid segment path")
	}
	clean := path.Clean("/" + segmentPath)
	rel := strings.TrimPrefix(clean, "/")
	abs := filepath.Join(m.root, filepath.FromSlash(rel))
	if abs == m.root || !strings.HasPrefix(abs, m.root+string(filepath.Separator)) {
		return "", fmt.Errorf("windows media: segment path escapes recordings root")
	}
	return rel, nil
}

func (m *Media) DecodeFrame(ctx context.Context, req platform.DecodeRequest) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rel, err := m.resolve(req.SegmentPath)
	if err != nil {
		return nil, err
	}
	return frameDecode(ctx, m.root, rel, req.FrameIndex, req.MaxPixelSize)
}

func (m *Media) DecodeFrames(ctx context.Context, reqs []platform.DecodeRequest) ([][]byte, error) {
	frames := make([][]byte, 0, len(reqs))
	for _, req := range reqs {
		frame, err := m.DecodeFrame(ctx, req)
		if err != nil {
			return nil, err
		}
		frames = append(frames, frame)
	}
	return frames, nil
}

func (m *Media) ProbeSegment(ctx context.Context, segmentPath string) (platform.SegmentInfo, error) {
	if err := ctx.Err(); err != nil {
		return platform.SegmentInfo{}, err
	}
	rel, err := m.resolve(segmentPath)
	if err != nil {
		return platform.SegmentInfo{}, err
	}
	return segmentProbe(ctx, m.root, rel)
}

func (*Media) EncodeVideo(context.Context, platform.EncodeRequest) (platform.EncodeResult, error) {
	return platform.EncodeResult{}, fmt.Errorf("windows media: video encoding awaits the codec decision")
}
