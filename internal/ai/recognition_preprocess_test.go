package ai

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"
)

type recordingProvider struct {
	calls   int
	inspect func(Request)
	err     error
}

func (p *recordingProvider) Generate(_ context.Context, request Request) (Result, error) {
	p.calls++
	if p.inspect != nil {
		p.inspect(request)
	}
	return Result{Text: "summary"}, p.err
}

func TestGenerateRecognitionLeavesDisabledRequestUnchanged(t *testing.T) {
	imagePart, err := ImagePart(MediaPNG, encodeTestPNG(t, 12, 10))
	if err != nil {
		t.Fatal(err)
	}
	request := Request{
		Purpose: PurposeTranscribe,
		Parts:   []Part{TextPart("unchanged prompt"), imagePart},
	}
	provider := &recordingProvider{inspect: func(got Request) {
		if len(got.Parts) != len(request.Parts) {
			t.Fatalf("parts = %d, want %d", len(got.Parts), len(request.Parts))
		}
		if got.Parts[0].Text() != request.Parts[0].Text() {
			t.Fatal("text changed while enhancement was disabled")
		}
		if !bytes.Equal(got.Parts[1].Bytes(), request.Parts[1].Bytes()) {
			t.Fatal("image changed while enhancement was disabled")
		}
	}}

	if _, err := GenerateRecognition(context.Background(), provider, request, false); err != nil {
		t.Fatalf("GenerateRecognition: %v", err)
	}
	if !bytes.Equal(request.Parts[1].Bytes(), imagePart.Bytes()) {
		t.Fatal("disabled path cleared the caller's original image")
	}
}

func TestGenerateRecognitionCreatesFourOverlappingTiles(t *testing.T) {
	imagePart, err := ImagePart(MediaPNG, encodeCoordinatePNG(t, 100, 80))
	if err != nil {
		t.Fatal(err)
	}
	var retained []Part
	provider := &recordingProvider{inspect: func(got Request) {
		if len(got.Parts) != 5 {
			t.Fatalf("parts = %d, want text plus four images", len(got.Parts))
		}
		if got.Parts[0].Kind() != PartText || got.Parts[0].Text() != "same text" {
			t.Fatalf("text part changed: %#v", got.Parts[0])
		}
		wantOrigins := [][2]int{{0, 0}, {30, 0}, {0, 20}, {30, 20}}
		for i, want := range wantOrigins {
			part := got.Parts[i+1]
			if part.Kind() != PartImage || part.MediaType() != MediaPNG {
				t.Fatalf("part %d = %#v, want PNG image", i+1, part)
			}
			decoded, _, err := image.Decode(bytes.NewReader(part.Bytes()))
			if err != nil {
				t.Fatalf("decode tile %d: %v", i, err)
			}
			if got := decoded.Bounds().Size(); got.X != 70 || got.Y != 60 {
				t.Fatalf("tile %d size = %v, want 70x60", i, got)
			}
			pixel := color.NRGBAModel.Convert(decoded.At(0, 0)).(color.NRGBA)
			if int(pixel.R) != want[0] || int(pixel.G) != want[1] {
				t.Fatalf("tile %d origin = (%d,%d), want %v", i, pixel.R, pixel.G, want)
			}
		}
		retained = append([]Part(nil), got.Parts[1:]...)
	}}

	request := Request{Purpose: PurposeTranscribe, Parts: []Part{TextPart("same text"), imagePart}}
	if _, err := GenerateRecognition(context.Background(), provider, request, true); err != nil {
		t.Fatalf("GenerateRecognition: %v", err)
	}
	for i, part := range retained {
		if !allZero(part.Bytes()) {
			t.Fatalf("temporary tile %d was not cleared after provider returned", i)
		}
	}
}

func TestGenerateRecognitionRejectsInvalidRequestBeforeCallingProvider(t *testing.T) {
	provider := &recordingProvider{}
	_, err := GenerateRecognition(context.Background(), provider, Request{}, false)
	if ErrorKindOf(err) != ErrorInvalidRequest {
		t.Fatalf("error = %v, want invalid request", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
}

func TestRecognitionTileSizeMatchesReference(t *testing.T) {
	part, err := ImagePart(MediaPNG, encodeTestPNG(t, 1280, 960))
	if err != nil {
		t.Fatal(err)
	}
	provider := &recordingProvider{inspect: func(got Request) {
		for i, tile := range got.Parts {
			decoded, _, err := image.Decode(bytes.NewReader(tile.Bytes()))
			if err != nil {
				t.Fatalf("decode tile %d: %v", i, err)
			}
			if size := decoded.Bounds().Size(); size.X != 660 || size.Y != 500 {
				t.Fatalf("tile %d size = %v, want 660x500", i, size)
			}
		}
	}}
	if _, err := GenerateRecognition(context.Background(), provider, Request{Parts: []Part{part}}, true); err != nil {
		t.Fatal(err)
	}
}

func encodeTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func encodeCoordinatePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 1, A: 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func allZero(data []byte) bool {
	for _, value := range data {
		if value != 0 {
			return false
		}
	}
	return true
}
