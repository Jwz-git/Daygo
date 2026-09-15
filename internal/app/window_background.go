package app

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

/*
 * The native window backdrop shows through wherever the webview has not
 * painted yet — most visibly while the user drags a window edge and
 * rasterization lags one frame behind. The launch-time option in app.go is a
 * fixed light value, so in a dark theme the drag region flashed light until
 * the webview caught up. The frontend calls SetWindowBackground whenever the
 * resolved appearance changes, carrying the page's own --dg-window-bg so the
 * backdrop always matches what the page paints.
 */
func (b *Backend) setWindowContext(ctx context.Context) {
	b.windowCtxMu.Lock()
	defer b.windowCtxMu.Unlock()
	b.windowCtx = ctx
}

// SetWindowBackground accepts a "#rrggbb" colour and repaints the native
// window backdrop with it. A nil window context (headless construction in
// tests) has nothing to paint, so the call is a no-op rather than an error.
func (b *Backend) SetWindowBackground(colour string) error {
	red, green, blue, err := parseHexColour(colour)
	if err != nil {
		return apperr.E(apperr.InvalidArgument, "invalid window background colour", err)
	}
	b.windowCtxMu.Lock()
	ctx := b.windowCtx
	b.windowCtxMu.Unlock()
	if ctx == nil {
		return nil
	}
	runtime.WindowSetBackgroundColour(ctx, red, green, blue, 255)
	return nil
}

// parseHexColour accepts only the six-digit lowercase-or-uppercase form the
// frontend sends; the CSS variable it carries is defined as exactly that.
func parseHexColour(colour string) (uint8, uint8, uint8, error) {
	if len(colour) != 7 || colour[0] != '#' {
		return 0, 0, 0, fmt.Errorf("colour %q is not #rrggbb", colour)
	}
	channel := func(part string) (uint8, error) {
		value, err := strconv.ParseUint(part, 16, 8)
		if err != nil {
			return 0, err
		}
		return uint8(value), nil
	}
	red, err := channel(colour[1:3])
	if err != nil {
		return 0, 0, 0, err
	}
	green, err := channel(colour[3:5])
	if err != nil {
		return 0, 0, 0, err
	}
	blue, err := channel(colour[5:7])
	if err != nil {
		return 0, 0, 0, err
	}
	return red, green, blue, nil
}
