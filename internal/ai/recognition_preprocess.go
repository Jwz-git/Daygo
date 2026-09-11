package ai

import (
	"bytes"
	"context"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
)

const recognitionOverlapPixels = 40

// GenerateRecognition applies recognition-only image preprocessing for one
// provider call. Temporary tiles are kept in memory and cleared when the call
// returns; they are never written to storage by this package.
func GenerateRecognition(ctx context.Context, provider Provider, request Request, enhancementEnabled bool) (Result, error) {
	if provider == nil {
		return Result{}, NewError(ErrorInvalidRequest, "provider is required", 0, nil)
	}
	if err := request.Validate(); err != nil {
		return Result{}, err
	}
	if !enhancementEnabled {
		return provider.Generate(ctx, request)
	}

	prepared, temporary, err := prepareRecognitionRequest(request)
	if err != nil {
		return Result{}, err
	}
	defer clearTemporaryParts(temporary)
	return provider.Generate(ctx, prepared)
}

func prepareRecognitionRequest(request Request) (Request, []Part, error) {
	parts := make([]Part, 0, len(request.Parts)+3)
	temporary := make([]Part, 0, 4)
	for _, part := range request.Parts {
		if part.Kind() != PartImage {
			parts = append(parts, part)
			continue
		}
		tiles, err := splitRecognitionImage(part)
		if err != nil {
			clearTemporaryParts(temporary)
			return Request{}, nil, err
		}
		parts = append(parts, tiles...)
		temporary = append(temporary, tiles...)
	}
	prepared := request
	prepared.Parts = parts
	if err := prepared.Validate(); err != nil {
		clearTemporaryParts(temporary)
		return Request{}, nil, err
	}
	return prepared, temporary, nil
}

func splitRecognitionImage(part Part) ([]Part, error) {
	if part.Kind() != PartImage {
		return nil, NewError(ErrorInvalidRequest, "recognition input must be an image", 0, nil)
	}
	decoded, format, err := image.Decode(bytes.NewReader(part.data))
	if err != nil {
		return nil, NewError(ErrorInvalidRequest, "recognition image could not be decoded", 0, err)
	}
	if decoded.Bounds().Dx() < 2 || decoded.Bounds().Dy() < 2 {
		return nil, NewError(ErrorInvalidRequest, "recognition image is too small to split", 0, nil)
	}
	if !formatMatchesMediaType(format, part.MediaType()) {
		return nil, NewError(ErrorInvalidRequest, "recognition image type does not match its contents", 0, nil)
	}

	rects := recognitionTileRects(decoded.Bounds())
	tiles := make([]Part, 0, len(rects))
	for _, rect := range rects {
		encoded, err := encodeRecognitionTile(decoded, rect, part.MediaType())
		if err != nil {
			clearTemporaryParts(tiles)
			return nil, err
		}
		tile, err := ImagePart(part.MediaType(), encoded)
		clearBytes(encoded)
		if err != nil {
			clearTemporaryParts(tiles)
			return nil, err
		}
		tiles = append(tiles, tile)
	}
	return tiles, nil
}

func recognitionTileRects(bounds image.Rectangle) [4]image.Rectangle {
	width, height := bounds.Dx(), bounds.Dy()
	overlapX := min(recognitionOverlapPixels, width-1)
	overlapY := min(recognitionOverlapPixels, height-1)
	tileWidth := (width + overlapX + 1) / 2
	tileHeight := (height + overlapY + 1) / 2
	rightX := bounds.Max.X - tileWidth
	bottomY := bounds.Max.Y - tileHeight

	return [4]image.Rectangle{
		image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+tileWidth, bounds.Min.Y+tileHeight),
		image.Rect(rightX, bounds.Min.Y, bounds.Max.X, bounds.Min.Y+tileHeight),
		image.Rect(bounds.Min.X, bottomY, bounds.Min.X+tileWidth, bounds.Max.Y),
		image.Rect(rightX, bottomY, bounds.Max.X, bounds.Max.Y),
	}
}

func encodeRecognitionTile(source image.Image, rect image.Rectangle, mediaType MediaType) ([]byte, error) {
	tile := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(tile, tile.Bounds(), source, rect.Min, draw.Src)
	var out bytes.Buffer
	var err error
	switch mediaType {
	case MediaPNG:
		err = png.Encode(&out, tile)
	case MediaJPEG:
		err = jpeg.Encode(&out, tile, &jpeg.Options{Quality: 95})
	default:
		return nil, NewError(ErrorUnsupportedFeature, "recognition enhancement cannot encode this image type", 0, nil)
	}
	if err != nil {
		return nil, NewError(ErrorInvalidRequest, "recognition tile could not be encoded", 0, err)
	}
	return out.Bytes(), nil
}

func formatMatchesMediaType(format string, mediaType MediaType) bool {
	return format == "png" && mediaType == MediaPNG || format == "jpeg" && mediaType == MediaJPEG
}

func clearTemporaryParts(parts []Part) {
	for i := range parts {
		clearBytes(parts[i].data)
	}
}

func clearBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}
