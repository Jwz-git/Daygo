package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	MaxImages     = 20
	MaxImageBytes = 5 << 20
	MaxTotalBytes = 20 << 20
)

type Purpose string

const (
	PurposeTest       Purpose = "test"
	PurposeTranscribe Purpose = "transcribe"
	PurposeCards      Purpose = "cards"
	PurposeChat       Purpose = "chat"
)

type PartKind string

const (
	PartText  PartKind = "text"
	PartImage PartKind = "image"
)

type MediaType string

const (
	MediaJPEG MediaType = "image/jpeg"
	MediaPNG  MediaType = "image/png"
	MediaWebP MediaType = "image/webp"
)

type Part struct {
	kind      PartKind
	text      string
	mediaType MediaType
	data      []byte
}

func TextPart(text string) Part {
	return Part{kind: PartText, text: text}
}

func ImagePart(mediaType MediaType, data []byte) (Part, error) {
	if !mediaType.valid() {
		return Part{}, NewError(ErrorInvalidRequest, "unsupported image type", 0, nil)
	}
	if len(data) == 0 || len(data) > MaxImageBytes {
		return Part{}, NewError(ErrorInvalidRequest, "image size is outside the allowed range", 0, nil)
	}
	return Part{kind: PartImage, mediaType: mediaType, data: append([]byte(nil), data...)}, nil
}

func (p Part) Kind() PartKind       { return p.kind }
func (p Part) Text() string         { return p.text }
func (p Part) MediaType() MediaType { return p.mediaType }
func (p Part) Bytes() []byte        { return append([]byte(nil), p.data...) }

func (m MediaType) valid() bool {
	return m == MediaJPEG || m == MediaPNG || m == MediaWebP
}

type OutputSchema struct {
	Name   string
	Schema json.RawMessage
	Strict bool
}

type Request struct {
	Purpose         Purpose
	Parts           []Part
	Output          *OutputSchema
	MaxOutputTokens int
}

func (r Request) Validate() error {
	if len(r.Parts) == 0 {
		return NewError(ErrorInvalidRequest, "request must contain at least one part", 0, nil)
	}
	images := 0
	totalBytes := 0
	for _, part := range r.Parts {
		switch part.kind {
		case PartText:
			if part.text == "" {
				return NewError(ErrorInvalidRequest, "text part must not be empty", 0, nil)
			}
		case PartImage:
			if !part.mediaType.valid() || len(part.data) == 0 || len(part.data) > MaxImageBytes {
				return NewError(ErrorInvalidRequest, "invalid image part", 0, nil)
			}
			images++
			totalBytes += len(part.data)
		default:
			return NewError(ErrorInvalidRequest, "unknown part kind", 0, nil)
		}
	}
	if images > MaxImages || totalBytes > MaxTotalBytes {
		return NewError(ErrorInvalidRequest, "image limits exceeded", 0, nil)
	}
	if r.Output != nil {
		if r.Output.Name == "" || len(r.Output.Schema) == 0 || !json.Valid(r.Output.Schema) {
			return NewError(ErrorInvalidRequest, "invalid output schema", 0, nil)
		}
	}
	if r.MaxOutputTokens < 0 {
		return NewError(ErrorInvalidRequest, "max output tokens must not be negative", 0, nil)
	}
	return nil
}

type Usage struct {
	InputTokens      *int64
	OutputTokens     *int64
	CacheReadTokens  *int64
	CacheWriteTokens *int64
}

type Result struct {
	Text  string
	JSON  json.RawMessage
	Model string
	Usage Usage
}

type Provider interface {
	Generate(context.Context, Request) (Result, error)
}

type ErrorKind string

const (
	ErrorAuthentication     ErrorKind = "authentication"
	ErrorRateLimited        ErrorKind = "rate_limited"
	ErrorTimeout            ErrorKind = "timeout"
	ErrorUnavailable        ErrorKind = "unavailable"
	ErrorInvalidRequest     ErrorKind = "invalid_request"
	ErrorUnsupportedFeature ErrorKind = "unsupported_feature"
	ErrorInvalidOutput      ErrorKind = "invalid_output"
	ErrorCanceled           ErrorKind = "canceled"
)

type Error struct {
	Kind       ErrorKind
	HTTPStatus int
	Message    string
	RetryAfter time.Duration
	err        error
}

func NewError(kind ErrorKind, message string, status int, err error) *Error {
	return &Error{Kind: kind, HTTPStatus: status, Message: message, err: err}
}

func (e *Error) Error() string {
	return fmt.Sprintf("ai: %s: %s", e.Kind, e.Message)
}

func (e *Error) Unwrap() error { return e.err }

func ErrorKindOf(err error) ErrorKind {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return ErrorCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorTimeout
	}
	var aiErr *Error
	if errors.As(err, &aiErr) {
		return aiErr.Kind
	}
	return ErrorUnavailable
}

func HTTPStatusOf(err error) int {
	var aiErr *Error
	if errors.As(err, &aiErr) {
		return aiErr.HTTPStatus
	}
	return 0
}

func RetryAfterOf(err error) time.Duration {
	var aiErr *Error
	if errors.As(err, &aiErr) && aiErr.RetryAfter > 0 {
		return aiErr.RetryAfter
	}
	return 0
}

func Retryable(err error) bool {
	switch ErrorKindOf(err) {
	case ErrorRateLimited, ErrorTimeout, ErrorUnavailable, ErrorInvalidOutput:
		return true
	default:
		return false
	}
}
