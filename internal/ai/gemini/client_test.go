package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Jwz-git/Daygo/internal/ai"
)

func TestGenerateVideo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/fixture-model:generateContent" || r.Header.Get("x-goog-api-key") != "fixture-key" || r.URL.RawQuery != "" {
			t.Error("incorrect Gemini route or credential transport")
		}
		var body struct {
			Contents []struct {
				Parts []struct {
					InlineData struct {
						MIMEType string
						Data     []byte
					}
				}
			}
			GenerationConfig struct {
				ResponseMIMEType   string
				ResponseJSONSchema json.RawMessage
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		} else if len(body.Contents) != 1 || len(body.Contents[0].Parts) != 2 ||
			body.Contents[0].Parts[1].InlineData.MIMEType != "video/mp4" ||
			string(body.Contents[0].Parts[1].InlineData.Data) != "video-fixture" ||
			body.GenerationConfig.ResponseMIMEType != "application/json" ||
			len(body.GenerationConfig.ResponseJSONSchema) == 0 {
			t.Error("video or structured-output config missing")
		}
		_, _ = w.Write([]byte(`{"modelVersion":"fixture-model","candidates":[{"finishReason":"STOP","content":{"parts":[{"thought":true,"text":"omit"},{"text":"{}"}]}}],"usageMetadata":{"promptTokenCount":4}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.Client(), server.URL+"/v1beta", "models/fixture-model", "fixture-key")
	if err != nil {
		t.Fatal(err)
	}
	video, err := ai.VideoPart([]byte("video-fixture"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Generate(context.Background(), ai.Request{Parts: []ai.Part{ai.TextPart("fixture"), video},
		Output: &ai.OutputSchema{Name: "fixture", Schema: json.RawMessage(`{"type":"object"}`), Strict: true}})
	if err != nil || result.Text != "{}" || result.Model != "fixture-model" || result.Usage.InputTokens == nil || *result.Usage.InputTokens != 4 {
		t.Fatalf("unexpected generation result: %+v, %v", result, err)
	}
}
