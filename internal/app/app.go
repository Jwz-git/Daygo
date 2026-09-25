package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Jwz-git/Daygo/frontend"
	"github.com/Jwz-git/Daygo/internal/platform/factory"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
	"github.com/Jwz-git/Daygo/internal/recorder"
	"github.com/Jwz-git/Daygo/internal/settings"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
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
	// Site favicons for timeline cards are cached under the support dir. This
	// does not depend on the database, so a read-only second instance (which
	// still renders cards) resolves icons too.
	backend.attachFavicons(filepath.Join(dir, "favicons"))
	// Capture ownership is requested at startup because this process is the
	// one the user launched; a second instance loses the race and reports
	// isCaptureOwner false.
	store, openErr := storage.Open(ctx, storage.Options{Dir: dir, CaptureOwnerRequested: true})
	if openErr != nil {
		// Keep the reason available to diagnostics without aborting startup.
		backend.setStorageError(openErr)
	} else {
		backend.attachStorage(store)
		// Frame playback serves screenshots from the recordings directory
		// next to the database (same root GetRecordingDirectory reports).
		backend.attachMedia(filepath.Join(filepath.Dir(store.Path()), "recordings"))
		defer func() { _ = store.Close() }()

		// Maintenance is owned by this context, so cancelling it at shutdown
		// stops the goroutine. There is no global scheduler to leak
		// (docs/modules/data.md).
		// The cleanup pass reads the recording limit live from settings: a
		// failed read returns 0 — the documented "no limit" — so the pass
		// skips rather than deleting on uncertain ground.
		settingsAccess := settings.New(store.Settings())
		maintainer := storage.NewMaintainer(store, storage.MaintainerOptions{
			BackupDir:      dir,
			RecordingsRoot: filepath.Join(dir, "recordings"),
			RecordingsLimit: func() int64 {
				snapshot, err := settingsAccess.Load(ctx)
				if err != nil {
					return 0
				}
				return snapshot.RecordingsLimitBytes
			},
		})
		go maintainer.Run(ctx)

		// The analysis pipeline runs only on the read-write instance; a
		// read-only second instance holds neither lock the pipeline's writes
		// need. Its recordings root is the staging directory the recorder
		// commits frames into.
		recordingsRoot := filepath.Join(dir, "recordings")

		// Crash recovery first (docs/modules/recording): settle pending
		// capture intents against the filesystem before the analysis
		// scheduler looks at frames. A commit the process died before
		// making is completed here; an intent whose file never landed is
		// dropped. Failures are logged, not fatal — the UI still works on
		// committed data.
		reconcileCtx, reconcileCancel := context.WithTimeout(ctx, 30*time.Second)
		if err := store.Captures().Reconcile(reconcileCtx, recordingsRoot); err != nil {
			log.Printf("capture reconcile: %v", err)
		}
		reconcileCancel()

		if _, err := startAnalysis(ctx, backend, store, recordingsRoot); err != nil {
			// Analysis failing to start must not take the shell down: the UI
			// still renders stored cards, and diagnostics reports the gap.
			log.Printf("analysis pipeline unavailable: %v", err)
		}

		// Backfill standups for calendar days that completed while the agent
		// was not running, then re-scan on a ticker for newly completed days.
		// Owned by ctx like the analysis pipeline, so shutdown stops it.
		go backend.runStandupBackfill(ctx)
	}
	// Start the updater only after storage ownership is known. Sparkle and
	// WinSparkle may schedule a check immediately; an early update must not see
	// the default non-owner state or race database/recorder composition.
	backend.setUpdater(factory.NewUpdater())
	backend.startUpdaterEventPump(ctx)

	appOpts := &options.App{
		Title:             "Daygo",
		Width:             1180,
		Height:            760,
		Frameless:         platformFrameless(),
		HideWindowOnClose: true,
		MinWidth:          880,
		MinHeight:         600,
		/*
		 * Only Windows uses a frameless window with the Vue title bar.
		 * macOS keeps its native frame and uses mac.TitleBarHidden() to
		 * preserve traffic lights and rounded corners.
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
			// Numeric-ID screenshot frames for card playback; everything the
			// embedded bundle does not claim falls through to this handler.
			Handler: http.HandlerFunc(backend.serveAsset),
		},
		// System may still be nil when the current platform adapter cannot
		// start; capability and day methods remain available in that mode.
		Bind: []any{backend},
		// OnStartup hands over the Wails context the event emitter needs. It is
		// installed here rather than at construction because runtime events
		// require a live context.
		OnStartup: func(ctx context.Context) {
			requestShutdown := func() {
				backend.requestQuit()
				runtime.Quit(ctx)
			}
			backend.configureUpdateInstall(requestShutdown)
			backend.setShutdownRequester(requestShutdown)
			emitter.SetContext(ctx)
			backend.setWindowContext(ctx)
			backend.setApplicationPicker(wailsApplicationPicker{ctx: ctx, labels: backend.nativeLabels.get})
			updateStatus := func(state recorder.State) {
				// Status-item setup is an optional platform capability and must not
				// make the desktop shell's startup depend on native tray availability.
				if backend.system == nil {
					return
				}
				// Labels come from the frontend (vue-i18n); the state → surface
				// mapping stays here so the adapter never learns recorder states.
				item := statusItemState(state, backend.statusLabels.get())
				if err := backend.system.SetStatusItem(ctx, item); err != nil {
					log.Printf("status item update unavailable: %v", err)
				}
			}
			backend.setStatusUpdater(updateStatus)
			updateStatus(backend.recorderState())
			// showWindow restores the window on an explicit user request: the
			// status-bar "open" item. The application is unhidden before the
			// window is ordered front because a soft-quit orders the window
			// itself out, which unhiding the app does not undo.
			showWindow := func() {
				if err := backend.exitBackground(ctx); err != nil {
					log.Printf("restore dock icon on reopen unavailable: %v", err)
				}
				runtime.Show(ctx)
				runtime.WindowShow(ctx)
			}
			// Activation only has to undo a soft-quit. See restoreOnActivation
			// for why every other activation must be left to the system.
			backend.setActivationAction(func() { backend.restoreOnActivation(showWindow) })
			backend.setStatusAction(func(action string) {
				switch action {
				case "open":
					showWindow()
				case "open_recordings":
					if backend.system == nil {
						return
					}
					dir, err := backend.GetRecordingDirectory()
					if err != nil {
						log.Printf("open recordings folder unavailable: %v", err)
						return
					}
					// The directory only exists after the first capture; create
					// it so the folder always opens instead of failing silently.
					if err := os.MkdirAll(dir, 0o755); err != nil {
						log.Printf("create recordings folder unavailable: %v", err)
						return
					}
					// Finder, not BrowserOpenURL: Wails' URL validator rejects the
					// file:// scheme outright, so a file URL never opens the folder.
					if err := backend.system.RevealPath(ctx, dir); err != nil {
						log.Printf("open recordings folder unavailable: %v", err)
					}
				case "quit":
					backend.requestQuit()
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
				case "pause_15":
					_ = backend.PauseRecording(15)
				case "pause_30":
					_ = backend.PauseRecording(30)
				case "pause_60":
					_ = backend.PauseRecording(60)
				case "pause_indefinite":
					_ = backend.PauseRecording(0)
				}
			})
			// After the status action is installed so the status item reflects
			// the state the auto-start produces.
			backend.maybeAutoStartRecording()
		},
		OnShutdown: func(ctx context.Context) {
			backend.shutdown()
		},
		// Daygo is a resident agent: Cmd+Q and the Dock "Quit" item must not
		// end the process. Wails routes every quit attempt (window close is
		// intercepted separately by HideWindowOnClose) through this hook, and
		// returning true keeps the app alive. Only the status-bar Quit sets
		// requestQuit first, so that one path returns false and terminates.
		// The soft-quit hides the window and drops the Dock icon, leaving the
		// status item as the way back (docs/decisions/lifecycle-quit-model.md).
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			if backend.quitAllowed() {
				return false
			}
			// A permission-change restart must terminate for real and relaunch so
			// a freshly granted screen-recording permission takes effect. macOS's
			// own "Quit & Reopen" reaches this hook like any other quit; without
			// this branch it would be downgraded to a background hide and the new
			// grant would never apply (docs/decisions/
			// recording-screen-recording-permission.md).
			if backend.permissionRestartArmed() {
				if err := backend.beginPermissionRestart(); err != nil {
					log.Printf("permission restart cancelled: %v", err)
					return true
				}
				return false
			}
			runtime.WindowHide(ctx)
			if err := backend.enterBackground(ctx); err != nil {
				log.Printf("drop dock icon on background quit unavailable: %v", err)
			}
			return true
		},
		// Host window options are platform-specific; each lives in an
		// options_<goos>.go file under a matching build tag.
		Mac:     platformMacOptions(),
		Linux:   platformLinuxOptions(),
		Windows: platformWindowsOptions(),
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		// A terminal Ctrl-C or launchd SIGTERM is a real quit, not a soft
		// background hide: mark it allowed so OnBeforeClose lets it through, then
		// ask Wails to quit — OnShutdown runs backend.shutdown(). Before startup
		// hands over the runtime context there is no event loop to quit, so stop
		// work directly and exit.
		backend.requestQuit()
		if ctx := backend.windowContext(); ctx != nil {
			runtime.Quit(ctx)
			return
		}
		backend.shutdown()
		os.Exit(0)
	}()

	err = wails.Run(appOpts)
	if err != nil {
		return fmt.Errorf("run Daygo desktop shell: %w", err)
	}
	return nil
}
