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
			obs, err := s.transcribeGroupWithVideo(context.Background(), chain, []storage.AnalysisFrame{frames[0], frames[2]}, frames)
			wantEnd := frames[1].CapturedAt
			wantCalls := 1
			if mode != "video" {
				wantEnd = frames[2].CapturedAt
			}
			if mode == "fallback" {
				wantCalls = 2
			}
			if err != nil || len(obs) != 1 || obs[0].End != wantEnd || calls != wantCalls {
				t.Fatalf("unexpected observations or routing: %+v, %v, calls=%d", obs, err, calls)
			}
		})
	}
}
