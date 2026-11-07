package analysis

import (
	"strings"

	"github.com/Jwz-git/Daygo/internal/storage"
)

type transcriptionGroup struct {
	images []storage.AnalysisFrame
	video  []storage.AnalysisFrame
}

// A video candidate must contain the original indices 0..n-1 in order.
// Reading the sealed MP4 also verifies n against its exact sample count.
func planTranscriptionGroups(frames []storage.AnalysisFrame, imageCap int, useVideo bool) []transcriptionGroup {
	var plans []transcriptionGroup
	addImages := func(group []storage.AnalysisFrame) {
		for _, images := range groupFrames(sampleFrames(group, imageCap), imageCap) {
			plans = append(plans, transcriptionGroup{images: images})
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
			sampled := groupFrames(sampleFrames(group, imageCap), imageCap)
			if len(sampled) == 1 {
				addImages(frames[nextImages:start])
				plans = append(plans, transcriptionGroup{images: sampled[0], video: group})
				nextImages = end
			}
		}
		start = end
	}
	addImages(frames[nextImages:])
	return plans
}

func contiguousSegmentFrames(group []storage.AnalysisFrame) bool {
	for index, frame := range group {
		if frame.FrameIndex != index {
			return false
		}
	}
	return true
}
