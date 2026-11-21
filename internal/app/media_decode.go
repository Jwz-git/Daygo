package app

import (
	"container/list"
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
	"golang.org/x/sync/singleflight"
)

const (
	frameDecodeTimeout = 5 * time.Second
	frameDecodeSlots   = 4
	thumbnailEntries   = 128
	thumbnailBytes     = 8 << 20
)

type frameDecoder struct {
	slots   chan struct{}
	pending singleflight.Group
	mu      sync.Mutex
	cache   map[string]*list.Element
	order   list.List
	bytes   int
}

type cachedThumbnail struct {
	key  string
	data []byte
}

func newFrameDecoder() *frameDecoder {
	return &frameDecoder{
		slots: make(chan struct{}, frameDecodeSlots),
		cache: make(map[string]*list.Element),
	}
}

func (d *frameDecoder) decode(ctx, host context.Context, id int64, media platform.Media, req platform.DecodeRequest) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := strconv.FormatInt(id, 10) + ":" + strconv.Itoa(req.MaxPixelSize)
	if data := d.thumbnail(key); data != nil {
		return data, nil
	}
	flight := d.pending.DoChan(key, func() (any, error) {
		if data := d.thumbnail(key); data != nil {
			return data, nil
		}
		// Duplicate readers have independent HTTP lifetimes. The shared job
		// is bounded by the host and the decode deadline, not the first reader.
		decodeCtx, cancel := context.WithTimeout(host, frameDecodeTimeout)
		defer cancel()
		select {
		case d.slots <- struct{}{}:
			defer func() { <-d.slots }()
		case <-decodeCtx.Done():
			return nil, decodeCtx.Err()
		}
		if err := decodeCtx.Err(); err != nil {
			return nil, err
		}
		data, err := media.DecodeFrame(decodeCtx, req)
		if err == nil && req.MaxPixelSize > 0 {
			d.rememberThumbnail(key, data)
		}
		return data, err
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-flight:
		if result.Err != nil {
			return nil, result.Err
		}
		return result.Val.([]byte), nil
	}
}

func (d *frameDecoder) thumbnail(key string) []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	entry := d.cache[key]
	if entry == nil {
		return nil
	}
	d.order.MoveToFront(entry)
	return entry.Value.(cachedThumbnail).data
}

func (d *frameDecoder) rememberThumbnail(key string, data []byte) {
	if len(data) == 0 || len(data) > thumbnailBytes {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache[key] = d.order.PushFront(cachedThumbnail{key: key, data: data})
	d.bytes += len(data)
	for len(d.cache) > thumbnailEntries || d.bytes > thumbnailBytes {
		oldest := d.order.Remove(d.order.Back()).(cachedThumbnail)
		delete(d.cache, oldest.key)
		d.bytes -= len(oldest.data)
	}
}
