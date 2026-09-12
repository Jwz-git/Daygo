package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Jwz-git/Daygo/frontend"
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/factory"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
	"github.com/Jwz-git/Daygo/internal/recorder"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
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

	system := factory.NewSystem()
	if closer, ok := system.(interface{ Close() }); ok {
		defer closer.Close()
	}
	backend := NewBackend(system, nil)
	backend.setCapture(factory.NewCapture())
	backend.setApplicationInspector(factory.NewApplicationInspector())
	backend.setSecrets(secrets.New())
	backend.startSystemEventPump()
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

		// The analysis pipeline runs only on the read-write instance; a
		// read-only second instance holds neither lock the pipeline's writes
		// need. Its recordings root is the staging directory the recorder
		// commits frames into.
		if _, err := startAnalysis(ctx, backend, store, filepath.Join(dir, "recordings")); err != nil {
			// Analysis failing to start must not take the shell down: the UI
			// still renders stored cards, and diagnostics reports the gap.
			log.Printf("analysis pipeline unavailable: %v", err)
		}
	}
	err = wails.Run(&options.App{
		Title:             "Daygo",
		Width:             1180,
		Height:            760,
		HideWindowOnClose: true,
		MinWidth:          880,
		MinHeight:         600,
		/*
		 * Not frameless: a frameless NSWindow drops the standard window frame,
		 * and with it both the traffic-light controls and the native rounded
		 * window corners. Hiding the titlebar instead (mac.TitleBarHiddenInset)
		 * keeps content edge-to-edge while leaving those two to the OS.
		 */
		CSSDragProperty: "--wails-draggable",
		CSSDragValue:    "drag",
		// No webview context menu in production. Debug builds force it on
		// regardless of this flag, so the frontend installs its own
		// contextmenu guard (frontend/src/main.ts) to keep both builds
		// behaving the same.
		EnableDefaultContextMenu: false,
		// Shown only between window creation and the first webview paint. The
		// light palette base is the least jarring default: macOS defaults to a
		// light system appearance, and the app follows it until the user says
		// otherwise.
		BackgroundColour: &options.RGBA{R: 233, G: 240, B: 251, A: 1},
		AssetServer: &assetserver.Options{
			Assets: frontend.Assets,
		},
		// System may still be nil when the current platform adapter cannot
		// start; capability and day methods remain available in that mode.
		Bind: []any{backend},
		// OnStartup hands over the Wails context the event emitter needs. It is
		// installed here rather than at construction because runtime events
		// require a live context.
		OnStartup: func(ctx context.Context) {
			emitter.SetContext(ctx)
			backend.setApplicationPicker(wailsApplicationPicker{ctx: ctx})
			updateStatus := func(state recorder.State) {
				// Status-item setup is an optional platform capability and must not
				// make the desktop shell's startup depend on native tray availability.
				if backend.system == nil {
					return
				}
				title, pause := "Not recording", "Start Recording"
				switch state {
				case recorder.StateStarting, recorder.StateCapturing:
					title, pause = "Recording", "Pause Recording"
				case recorder.StatePaused:
					title, pause = "Paused", "Resume Recording"
				}
				if err := backend.system.SetStatusItem(ctx, platform.StatusItemState{Visible: true, Title: title, Tooltip: "Daygo", OpenLabel: "Open Daygo", PauseLabel: pause, QuitLabel: "Quit Daygo", PauseEnabled: state != recorder.StateStarting}); err != nil {
					log.Printf("status item update unavailable: %v", err)
				}
			}
			backend.setStatusUpdater(updateStatus)
			updateStatus(backend.recorderState())
			backend.setStatusAction(func(action string) {
				switch action {
				case "open":
					runtime.WindowShow(ctx)
					runtime.Show(ctx)
				case "quit":
					runtime.Quit(ctx)
				case "toggle_pause":
					switch backend.recorderState() {
					case recorder.StateIdle:
						_ = backend.SetRecording(true)
					case recorder.StatePaused:
						_ = backend.ResumeRecording()
					case recorder.StateCapturing:
						_ = backend.PauseRecording(0)
					}
				}
			})
			// After the status action is installed so the status item reflects
			// the state the auto-start produces.
			backend.maybeAutoStartRecording()
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
