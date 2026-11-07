package mediafile

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func videoBox(name string, payload ...[]byte) []byte {
	body := bytes.Join(payload, nil)
	box := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(box, uint32(8+len(body)))
	copy(box[4:], name)
	return append(box, body...)
}

func TestReadAnalysisSegmentRequiresExactVideoCoverage(t *testing.T) {
	sizes := make([]byte, 20)
	binary.BigEndian.PutUint32(sizes[8:], 2)
	binary.BigEndian.PutUint32(sizes[12:], 1)
	binary.BigEndian.PutUint32(sizes[16:], 1)
	handler := videoBox("hdlr", []byte("\x00\x00\x00\x00\x00\x00\x00\x00vide"))
	track := videoBox("trak", videoBox("mdia", handler, videoBox("minf", videoBox("stbl", videoBox("stsz", sizes)))))
	video := bytes.Join([][]byte{videoBox("ftyp", []byte("isom\x00\x00\x00\x00")), videoBox("mdat", []byte{1, 2}), videoBox("moov", track)}, nil)
	root := t.TempDir()
	path := filepath.Join(root, "segment.mp4")
	if err := os.WriteFile(path, video, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadAnalysisSegment(context.Background(), root, "segment.mp4", 2, len(video))
	if err != nil || !bytes.Equal(got, video) {
		t.Fatalf("whole segment: %v", err)
	}
	for _, count := range []int{1, 3} {
		if _, err := ReadAnalysisSegment(context.Background(), root, "segment.mp4", count, len(video)); err == nil {
			t.Fatalf("accepted mismatched sample count %d", count)
		}
	}
	if _, err := ReadAnalysisSegment(context.Background(), root, "segment.mp4", 2, len(video)-1); err == nil {
		t.Fatal("accepted oversize segment")
	}
	if err := os.WriteFile(path, append(video, videoBox("moof")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAnalysisSegment(context.Background(), root, "segment.mp4", 2, len(video)+8); err == nil {
		t.Fatal("accepted fragmented video")
	}
	outside := filepath.Join(t.TempDir(), "outside.mp4")
	if err := os.WriteFile(outside, video, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.mp4")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := ReadAnalysisSegment(context.Background(), root, "escape.mp4", 2, len(video)); err == nil {
		t.Fatal("accepted a path outside the recordings root")
	}
}
