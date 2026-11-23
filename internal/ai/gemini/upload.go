package gemini

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/Jwz-git/Daygo/internal/ai"
)

// Each fragment contains JSON metadata followed by an optional base64 stream.
// Fragment boundaries are inside JSON string values; the standard base64
// alphabet contains no characters that need JSON escaping.
type uploadFragment struct {
	json  []byte
	media ai.Part
}

type upload struct {
	fragments []uploadFragment
	length    int64
}

func newUpload(request ai.Request) (*upload, error) {
	config := generationConfig{MaxOutputTokens: request.MaxOutputTokens}
	if request.Output != nil {
		config.ResponseMIMEType = "application/json"
		config.ResponseJSONSchema = request.Output.Schema
	}
	encodedConfig, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var body upload
	meta := []byte(`{"contents":[{"parts":[`)
	for i, part := range request.Parts {
		if i > 0 {
			meta = append(meta, ',')
		}
		if part.Kind() == ai.PartText {
			encoded, err := json.Marshal(contentPart{Text: part.Text()})
			if err != nil {
				return nil, err
			}
			meta = append(meta, encoded...)
			continue
		}
		// Validate has restricted MediaType to the protocol's fixed values.
		meta = append(meta, `{"inlineData":{"mimeType":"`...)
		meta = append(meta, part.MediaType()...)
		meta = append(meta, `","data":"`...)
		body.fragments = append(body.fragments, uploadFragment{meta, part})
		body.length += int64(len(meta)) + int64(part.Base64Len())
		meta = []byte(`"}}`)
	}
	meta = append(meta, `]}],"generationConfig":`...)
	meta = append(meta, encodedConfig...)
	meta = append(meta, '}')
	body.fragments = append(body.fragments, uploadFragment{json: meta})
	body.length += int64(len(meta))
	return &body, nil
}

func (u *upload) open() io.ReadCloser {
	readers := make([]io.Reader, 0, len(u.fragments)*2)
	for _, fragment := range u.fragments {
		readers = append(readers, bytes.NewReader(fragment.json))
		if fragment.media.Kind() != "" {
			readers = append(readers, fragment.media.OpenBase64())
		}
	}
	return io.NopCloser(io.MultiReader(readers...))
}
