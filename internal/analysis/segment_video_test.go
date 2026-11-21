package analysis

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/storage"
)

type segmentReaderFixture struct {
	read func(string, int) ([]byte, error)
}

func (f segmentReaderFixture) ReadAnalysisSegment(_ context.Context, path string, count, _ int) ([]byte, error) {
	return f.read(path, count)
}

func (f segmentReaderFixture) FrameBytes(context.Context, string, int) ([]byte, error) {
	return []byte("fixture-jpeg"), nil
}

type segmentProviderFixture func(ai.Request) (ai.Result, error)

func (f segmentProviderFixture) Generate(_ context.Context, req ai.Request) (ai.Result, error) {
	return f(req)
}

func TestTranscribeOriginalVideoAndImageFallback(t *testing.T) {
	first := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	frames := []storage.AnalysisFrame{
		{SegmentPath: "segments/fixture.mp4", FrameIndex: 0, CapturedAt: first},
		{SegmentPath: "segments/fixture.mp4", FrameIndex: 1, CapturedAt: first.Add(time.Minute)},
		{SegmentPath: "segments/fixture.mp4", FrameIndex: 2, CapturedAt: first.Add(3 * time.Minute)},
	}
	for _, mode := range []string{"video", "fallback", "partial"} {
		t.Run(mode, func(t *testing.T) {
			source := segmentReaderFixture{read: func(path string, count int) ([]byte, error) {
				if path != frames[0].SegmentPath || count != len(frames) {
					t.Fatalf("unexpected segment request %s, %d", path, count)
				}
				if mode == "partial" {
					return nil, fmt.Errorf("fixture has extra samples")
				}
				return []byte("original-fixture-mp4"), nil
			}}
			s := &Service{cfg: Config{Media: source, SegmentVideo: source, Language: func(context.Context) string { return "en" }}}
			response := ai.Result{Text: `{"observations":[{"from_frame":0,"to_frame":1,"observation":"fixture","apps":[]}]}`}
			calls := 0
			gemini := segmentProviderFixture(func(req ai.Request) (ai.Result, error) {
				calls++
				if mode == "partial" {
					if req.Parts[1].Kind() != ai.PartImage || !strings.Contains(req.Parts[0].Text(), "independent screenshots") {
						t.Fatal("partial segment must use screenshots")
					}
					return response, nil
				}
				if len(req.Parts) != 2 || req.Parts[1].Kind() != ai.PartVideo || string(req.Parts[1].Bytes()) != "original-fixture-mp4" {
					t.Fatal("Gemini must receive the original video bytes")
				}
				if !strings.Contains(req.Parts[0].Text(), "playback 1s: frame 1, captured at 10:01 AM") {
					t.Fatal("video request is missing the playback-to-capture mapping")
				}
				if mode == "fallback" {
					return ai.Result{}, ai.NewError(ai.ErrorInvalidRequest, "fixture rejection", 400, nil)
				}
				return response, nil
			})
			images := segmentProviderFixture(func(req ai.Request) (ai.Result, error) {
				calls++
				if len(req.Parts) != 3 || req.Parts[1].Kind() != ai.PartImage || req.Parts[2].Kind() != ai.PartImage ||
					!strings.Contains(req.Parts[0].Text(), "frame 1: captured at 10:03 AM") {
					t.Fatal("fallback must receive screenshots only")
				}
				return response, nil
			})
			chain := ai.NewChain([]ai.ChainEntry{
				{ID: "gemini", Provider: gemini, PrefersVideo: true}, {ID: "images", Provider: images},
			}, 0)
			obs, usedVideo, err := s.transcribeGroupWithVideo(context.Background(), chain, []storage.AnalysisFrame{frames[0], frames[2]}, frames)
			wantEnd := frames[1].CapturedAt
			wantCalls := 1
			if mode != "video" {
				wantEnd = frames[2].CapturedAt
			}
			if mode == "fallback" {
				wantCalls = 2
			}
			if err != nil || len(obs) != 1 || obs[0].End != wantEnd || calls != wantCalls || usedVideo != (mode == "video") {
				t.Fatalf("unexpected observations or routing: %+v, %v, calls=%d", obs, err, calls)
			}
		})
	}
}

func TestTranscribeCompleteSegmentWithDefaultImageCap(t *testing.T) {
	frames := make([]storage.AnalysisFrame, 60)
	for i := range frames {
		frames[i] = storage.AnalysisFrame{SegmentPath: "segments/fixture.mp4", FrameIndex: i,
			CapturedAt: time.Unix(int64(i*10), 0).UTC(), FileSize: 1024}
	}
	const response = `{"observations":[{"from_frame":0,"to_frame":59,"observation":"fixture","apps":[]}]}`
	for _, mode := range []string{"video", "unreadable", "rejected"} {
		t.Run(mode, func(t *testing.T) {
			var reads, videoCalls, imageCalls, imageCount int
			source := segmentReaderFixture{read: func(path string, count int) ([]byte, error) {
				reads++
				if path != frames[0].SegmentPath || count != len(frames) {
					t.Errorf("unexpected segment request %q, %d", path, count)
				}
				if mode == "unreadable" {
					return nil, fmt.Errorf("fixture has extra samples")
				}
				return []byte("original-fixture-mp4"), nil
			}}
			service := &Service{cfg: Config{
				Media: source, SegmentVideo: source, Workers: 1, Providers: fakeChainSource{},
				Language: func(context.Context) string { return "en" },
			}}
			images := segmentProviderFixture(func(req ai.Request) (ai.Result, error) {
				imageCalls++
				if len(req.Parts) != ai.MaxImages+1 {
					return ai.Result{}, fmt.Errorf("expected 5 images, got %d", len(req.Parts)-1)
				}
				for _, part := range req.Parts[1:] {
					if part.Kind() != ai.PartImage {
						return ai.Result{}, fmt.Errorf("fallback received non-image media")
					}
					imageCount++
				}
				return ai.Result{Text: response}, nil
			})
			gemini := segmentProviderFixture(func(req ai.Request) (ai.Result, error) {
				if req.Parts[1].Kind() != ai.PartVideo {
					return images.Generate(context.Background(), req)
				}
				videoCalls++
				if len(req.Parts) != 2 || string(req.Parts[1].Bytes()) != "original-fixture-mp4" {
					return ai.Result{}, fmt.Errorf("expected one original video")
				}
				if mode == "rejected" {
					return ai.Result{}, ai.NewError(ai.ErrorInvalidRequest, "fixture rejection", 400, nil)
				}
				return ai.Result{Text: response}, nil
			})
			chain := ai.NewChain([]ai.ChainEntry{
				{ID: "gemini", Provider: gemini, PrefersVideo: true}, {ID: "images", Provider: images},
			}, 0)
			observations, err := service.transcribe(context.Background(), chain, frames)
			wantVideoCalls, wantImageCalls := 1, 0
			if mode != "video" {
				wantImageCalls = 3
			}
			if mode == "unreadable" {
				wantVideoCalls = 0
			}
			if err != nil || reads != 1 || videoCalls != wantVideoCalls || imageCalls != wantImageCalls || imageCount != wantImageCalls*5 {
				t.Fatalf("reads=%d video=%d image requests=%d images=%d err=%v", reads, videoCalls, imageCalls, imageCount, err)
			}
			if len(observations) != max(wantImageCalls, 1) || !observations[0].Start.Equal(frames[0].CapturedAt) ||
				!observations[len(observations)-1].End.Equal(frames[len(frames)-1].CapturedAt) {
				t.Fatalf("observations must cover first through last frame: %+v", observations)
			}
		})
	}
}
