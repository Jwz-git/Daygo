package app

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/factory"
	"github.com/Jwz-git/Daygo/internal/recordinglocation"
)

func (b *Backend) recordingRoot(ctx context.Context) (string, error) {
	if b.storage == nil {
		return "", fmt.Errorf("recording directory unavailable")
	}
	defaultRoot := filepath.Join(filepath.Dir(b.storage.Path()), "recordings")
	if runtime.GOOS != "windows" {
		return defaultRoot, nil
	}
	return recordinglocation.Active(ctx, b.storage.Settings(), defaultRoot)
}

// locationMedia holds the read lock for the whole decode, so a migration cannot
// switch or remove files while an analysis or playback request uses them.
type locationMedia struct {
	backend  *Backend
	fallback string
}

func (m locationMedia) current(ctx context.Context) (platform.Media, func(), error) {
	b := m.backend
	b.moveMu.RLock()
	root := m.fallback
	if b.storage != nil {
		var err error
		root, err = b.recordingRoot(ctx)
		if err != nil {
			b.moveMu.RUnlock()
			return nil, nil, err
		}
	}
	return factory.NewMedia(root), b.moveMu.RUnlock, nil
}
func (m locationMedia) DecodeFrame(ctx context.Context, req platform.DecodeRequest) ([]byte, error) {
	media, unlock, err := m.current(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return media.DecodeFrame(ctx, req)
}
func (m locationMedia) DecodeFrames(ctx context.Context, reqs []platform.DecodeRequest) ([][]byte, error) {
	media, unlock, err := m.current(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return media.DecodeFrames(ctx, reqs)
}
func (m locationMedia) EncodeVideo(ctx context.Context, req platform.EncodeRequest) (platform.EncodeResult, error) {
	media, unlock, err := m.current(ctx)
	if err != nil {
		return platform.EncodeResult{}, err
	}
	defer unlock()
	return media.EncodeVideo(ctx, req)
}
func (m locationMedia) ProbeSegment(ctx context.Context, path string) (platform.SegmentInfo, error) {
	media, unlock, err := m.current(ctx)
	if err != nil {
		return platform.SegmentInfo{}, err
	}
	defer unlock()
	return media.ProbeSegment(ctx, path)
}
