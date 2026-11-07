package mediafile

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jwz-git/Daygo/internal/platform"
)

var errIneligibleVideo = errors.New("media: original segment is unavailable for analysis")

// ReadAnalysisSegment returns the original bytes only when the finalized
// single-track MP4 contains exactly the expected number of video samples.
// Validate the same bytes we return, so a moved/replaced file cannot race the
// coverage check. The caller verifies contiguous batch indices starting at 0.
func ReadAnalysisSegment(ctx context.Context, dir, segment string, expectedFrames, maxBytes int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if dir == "" || !platform.ValidSegmentPath(segment) || !strings.EqualFold(filepath.Ext(segment), ".mp4") ||
		expectedFrames <= 0 || maxBytes <= 0 {
		return nil, errIneligibleVideo
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, errIneligibleVideo
	}
	defer root.Close()
	file, err := root.Open(filepath.FromSlash(segment))
	if err != nil {
		return nil, errIneligibleVideo
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > int64(maxBytes) {
		return nil, errIneligibleVideo
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(maxBytes)+1))
	if err != nil || len(data) > maxBytes || !completeVideo(data, expectedFrames) {
		return nil, errIneligibleVideo
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

// completeVideo recognizes the non-fragmented, video-only MP4 layout written
// by the recorder. Duration is deliberately not used as a sample count.
func completeVideo(data []byte, expectedFrames int) bool {
	top := mp4Boxes(data)
	moov := uniqueBox(top, "moov")
	movie := mp4Boxes(moov)
	media := mp4Boxes(uniqueBox(mp4Boxes(uniqueBox(movie, "trak")), "mdia"))
	handler := uniqueBox(media, "hdlr")
	sampleTable := mp4Boxes(uniqueBox(mp4Boxes(uniqueBox(media, "minf")), "stbl"))
	sizes := uniqueBox(sampleTable, "stsz")
	if len(uniqueBox(top, "ftyp")) < 8 || len(uniqueBox(top, "mdat")) == 0 ||
		len(top["moof"]) != 0 || len(movie["mvex"]) != 0 || len(handler) < 12 ||
		string(handler[8:12]) != "vide" || len(sizes) < 12 || binary.BigEndian.Uint32(sizes) != 0 ||
		len(sampleTable["stz2"]) != 0 || uint64(binary.BigEndian.Uint32(sizes[8:])) != uint64(expectedFrames) {
		return false
	}
	if binary.BigEndian.Uint32(sizes[4:]) != 0 {
		return len(sizes) == 12
	}
	return uint64(len(sizes)-12) == uint64(expectedFrames)*4
}

func uniqueBox(boxes map[string][][]byte, name string) []byte {
	if matches := boxes[name]; len(matches) == 1 {
		return matches[0]
	}
	return nil
}

func mp4Boxes(data []byte) map[string][][]byte {
	boxes := make(map[string][][]byte)
	for len(data) != 0 {
		if len(data) < 8 || len(boxes) >= 64 {
			return nil
		}
		header := uint64(8)
		size := uint64(binary.BigEndian.Uint32(data))
		switch size {
		case 0:
			size = uint64(len(data))
		case 1:
			if len(data) < 16 {
				return nil
			}
			header = 16
			size = binary.BigEndian.Uint64(data[8:])
		}
		if size < header || size > uint64(len(data)) {
			return nil
		}
		name := string(data[4:8])
		if len(boxes[name]) >= 2 {
			return nil
		}
		boxes[name] = append(boxes[name], data[header:size])
		data = data[size:]
	}
	return boxes
}
