package analysis

import (
	"context"
	"strings"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/storage"
)

type transcriptionGroup struct {
	imageGroups [][]storage.AnalysisFrame
	video       []storage.AnalysisFrame
}

// A video candidate must contain the original indices 0..n-1 in order.
// Reading the sealed MP4 also verifies n against its exact sample count.
func planTranscriptionGroups(frames []storage.AnalysisFrame, imageCap int, useVideo bool) []transcriptionGroup {
	var plans []transcriptionGroup
	addImages := func(group []storage.AnalysisFrame) {
		for _, images := range groupFrames(sampleFrames(group, imageCap), imageCap) {
			plans = append(plans, transcriptionGroup{imageGroups: [][]storage.AnalysisFrame{images}})
		}
	}
	if !useVideo {
		addImages(frames)
		return plans
	}
	nextImages := 0
	for start := 0; start < len(frames); {
		end := start + 1
		segment := frames[start].SegmentPath
		for end < len(frames) && frames[end].SegmentPath == segment {
			end++
		}
		if strings.HasSuffix(strings.ToLower(segment), ".mp4") && contiguousSegmentFrames(frames[start:end]) {
			group := frames[start:end]
			addImages(frames[nextImages:start])
			plans = append(plans, transcriptionGroup{
				imageGroups: groupFrames(sampleFrames(group, imageCap), imageCap),
				video:       group,
			})
			nextImages = end
		}
		start = end
	}
	addImages(frames[nextImages:])
	return plans
}

func (s *Service) transcribePlannedGroup(ctx context.Context, chain *ai.Chain, group transcriptionGroup) ([]storage.Observation, error) {
	video := group.video
	var observations []storage.Observation
	for _, images := range group.imageGroups {
		next, usedVideo, err := s.transcribeGroupWithVideo(ctx, chain, images, video)
		if err != nil {
			return nil, err
		}
		if usedVideo {
			// The first request covered the whole segment. The remaining
			// image groups are only needed on the screenshot path.
			return next, nil
		}
		observations = append(observations, next...)
		video = nil
	}
	return observations, nil
}

func contiguousSegmentFrames(group []storage.AnalysisFrame) bool {
	for index, frame := range group {
		if frame.FrameIndex != index {
			return false
		}
	}
	return true
}
