//go:build daygo_desktop

package app

import (
	"context"
	"log"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Window creation, foregrounding and release are serialized on the app thread.
// Bindings can concurrently take a reference to the currently usable window.
type desktopWindows struct {
	host       *application.App
	backend    *Backend
	ctx        context.Context
	current    atomic.Pointer[application.WebviewWindow]
	quit       *atomic.Bool
	release    windowRelease
	allowClose *atomic.Bool
	timer      *time.Timer
}

func (d *desktopWindows) create() *application.WebviewWindow {
	opts := desktopWindowOptions(runtime.GOOS)
	if d.backend.windowResume.Load() != nil {
		opts.URL = "/?resume=1"
	}
	w := d.host.Window.NewWithOptions(opts)
	allowClose := new(atomic.Bool)
	d.allowClose = allowClose
	d.current.Store(w)
	if native, ok := d.backend.system.(platform.MainWindowAttachment); ok {
		native.AttachMainWindow(w.NativeWindow())
	}
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if d.quit.Load() || allowClose.Load() {
			return
		}
		e.Cancel()
		d.backend.setWindowHidden(true)
		if runtime.GOOS == "darwin" {
			application.InvokeSync(d.host.Hide)
		} else {
			w.Hide()
		}
		go func() {
			if err := d.backend.enterBackground(d.ctx); err != nil {
				log.Printf("drop dock icon on window close unavailable: %v", err)
			}
		}()
	})
	return w
}

func (d *desktopWindows) cancelRelease() {
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	d.release.cancel()
}

func (d *desktopWindows) show() {
	application.InvokeSync(func() {
		d.cancelRelease()
		w := d.current.Load()
		if w == nil {
			w = d.create()
		}
		if runtime.GOOS == "darwin" {
			d.host.Show()
		}
		w.Show()
		if runtime.GOOS == "darwin" {
			w.Focus()
		}
	})
	d.backend.setWindowHidden(false)
}

func (d *desktopWindows) scheduleRelease() {
	application.InvokeAsync(func() {
		d.cancelRelease()
		if d.ctx.Err() != nil || d.current.Load() == nil {
			return
		}
		token := d.release.begin()
		d.timer = time.AfterFunc(2*time.Minute, func() {
			if d.ctx.Err() != nil {
				return
			}
			application.InvokeAsync(func() {
				if d.ctx.Err() == nil && d.release.request(token) {
					d.host.Event.Emit("ui:prepare-window-release", token)
				}
			})
		})
	})
}

func (d *desktopWindows) confirm(token int, snapshot WindowResumeDTO) {
	application.InvokeAsync(func() {
		w := d.current.Load()
		if d.ctx.Err() != nil || w == nil || d.backend.GetUIVisibility().Visible || w.IsVisible() || !d.release.accept(token, snapshot) {
			return
		}
		d.backend.windowResume.Store(&snapshot)
		d.allowClose.Store(true)
		d.current.Store(nil)
		w.Close()
	})
}
