package gemini

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
	"testing/iotest"

	"github.com/Jwz-git/Daygo/internal/ai"
)

func TestUploadReplayAndContentLength(t *testing.T) {
	// Cross both the encoding block boundary and its final base64 padding.
	for _, size := range []int{1, 2, 3, 3071, 3072, 3073, 12349} {
		data := bytes.Repeat([]byte{0xfb}, size)
		part, err := ai.ImagePart(ai.MediaPNG, data)
		if err != nil {
			t.Fatal(err)
		}
		text := "quote \" and newline\n <>&"
		request := ai.Request{Parts: []ai.Part{part, ai.TextPart(text), part}, MaxOutputTokens: 37,
			Output: &ai.OutputSchema{Name: "fixture", Schema: json.RawMessage(`{"type":"object"}`)}}
		body, err := newUpload(request)
		if err != nil {
			t.Fatal(err)
		}
		want, err := json.Marshal(struct {
			Contents []content        `json:"contents"`
			Config   generationConfig `json:"generationConfig"`
		}{[]content{{Parts: []contentPart{
			{InlineData: &mediaBlob{ai.MediaPNG, data}}, {Text: text}, {InlineData: &mediaBlob{ai.MediaPNG, data}},
		}}}, generationConfig{37, "application/json", request.Output.Schema}})
		if err != nil {
			t.Fatal(err)
		}
		first := body.open()
		if _, err := io.ReadFull(first, make([]byte, 70)); err != nil {
			t.Fatal(err)
		}
		second := body.open()
		got, err := io.ReadAll(iotest.OneByteReader(second))
		if err != nil || !bytes.Equal(got, want) || int64(len(want)) != body.length {
			t.Fatalf("replayed media of %d bytes has wrong body or length: %v", size, err)
		}
		rest, err := io.ReadAll(first)
		if err != nil || !bytes.Equal(rest, want[70:]) {
			t.Fatalf("independent replay: %v", err)
		}
	}
}
