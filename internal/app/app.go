package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Jwz-git/Daygo/frontend"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

// ApplicationSupportDirName is the directory under the user's Application
// Support folder that holds every Daygo file. The name is part of the
// published identity and cannot change after the first public release
// (AGENTS.md, "身份标识").
const ApplicationSupportDirName = "Daygo"

// supportDir returns the application support directory. os.UserConfigDir maps
// to ~/Library/Application Support on macOS, which is the path docs/03 §3.1
// specifies.
func supportDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate application support directory: %w", err)
	}
	return filepath.Join(base, ApplicationSupportDirName), nil
}

// Run starts the Daygo desktop shell.
//
// The database is opened before the window, so the ownership the UI reports is
// the ownership actually held. Failing to open it is not fatal: a second
// instance is expected to run without the write lock, and a damaged database
// should still let the user see the app rather than a process that exits
// silently. Storage problems surface through GetDiagnostics instead.
func Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend := NewBackend(nil, nil)
	// The emitter publishes to the frontend once Wails supplies a context in
	// OnStartup; before that it drops events, which is correct because a window
	// that does not exist yet has no listener.
	emitter := NewWailsEmitter()
	backend.setEventEmitter(emitter)

	dir, err := supportDir()
	if err != nil {
		return err
	}
	// Capture ownership is requested at startup because this process is the
	// one the user launched; a second instance loses the race and reports
	// isCaptureOwner false.
	store, openErr := storage.Open(ctx, storage.Options{Dir: dir, CaptureOwnerRequested: true})
	if openErr != nil {
		// Keep the reason available to diagnostics without aborting startup.
		backend.setStorageError(openErr)
	} else {
		backend.attachStorage(store)
		defer func() { _ = store.Close() }()

		// Maintenance is owned by this context, so cancelling it at shutdown
		// stops the goroutine. There is no global scheduler to leak
		// (docs/modules/data.md).
		maintainer := storage.NewMaintainer(store, storage.MaintainerOptions{BackupDir: dir})
		go maintainer.Run(ctx)
	}

	err = wails.Run(&options.App{
		Title:     "Daygo",
		Width:     1180,
		Height:    760,
		MinWidth:  880,
		MinHeight: 600,
		/*
		 * Not frameless: a frameless NSWindow drops the standard window frame,
		 * and with it both the traffic-light controls and the native rounded
		 * window corners. Hiding the titlebar instead (mac.TitleBarHiddenInset)
		 * keeps content edge-to-edge while leaving those two to the OS.
		 */
		CSSDragProperty: "--wails-draggable",
		CSSDragValue:    "drag",
		// Shown only between window creation and the first webview paint. The
		// light palette base is the least jarring default: macOS defaults to a
		// light system appearance, and the app follows it until the user says
		// otherwise.
		BackgroundColour: &options.RGBA{R: 233, G: 240, B: 251, A: 1},
		AssetServer: &assetserver.Options{
			Assets: frontend.Assets,
		},
		// system is nil until the native adapter exists: capability and day
		// methods work truthfully, permission methods return native_unavailable.
		Bind: []any{backend},
		// OnStartup hands over the Wails context the event emitter needs. It is
		// installed here rather than at construction because runtime events
		// require a live context.
		OnStartup: func(ctx context.Context) {
			emitter.SetContext(ctx)
		},
		Mac: &mac.Options{
			/*
			 * TitleBarHidden (not TitleBarHiddenInset): both keep the native
			 * traffic lights, but HiddenInset also attaches an empty NSToolbar,
			 * and on macOS 26 a window with a toolbar is drawn with a noticeably
			 * larger corner radius than a titlebar-only window. Dropping the
			 * toolbar is what keeps the window corners tight.
			 */
			TitleBar:             mac.TitleBarHidden(),
			WebviewIsTransparent: false,
		},
	})
	if err != nil {
		return fmt.Errorf("run Daygo desktop shell: %w", err)
	}
	return nil
}
