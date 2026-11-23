package ai

import (
	"encoding/base64"
	"io"
)

// Base64Len returns the wire size of the immutable media payload.
func (p Part) Base64Len() int { return base64.StdEncoding.EncodedLen(len(p.data)) }

// OpenBase64 opens an independent upload stream without cloning the raw media.
func (p Part) OpenBase64() io.Reader { return &base64Reader{remaining: p.data} }

type base64Reader struct {
	remaining []byte
	buffer    [4096]byte
	pending   []byte
}

func (r *base64Reader) Read(dst []byte) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}
	if len(r.pending) == 0 {
		if len(r.remaining) == 0 {
			return 0, io.EOF
		}
		// Only the final block can require base64 padding.
		n := min(len(r.remaining), len(r.buffer)/4*3)
		r.pending = r.buffer[:base64.StdEncoding.EncodedLen(n)]
		base64.StdEncoding.Encode(r.pending, r.remaining[:n])
		r.remaining = r.remaining[n:]
	}
	n := copy(dst, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}
