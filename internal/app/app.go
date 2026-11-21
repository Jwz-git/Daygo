//go:build daygo_desktop

package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/Jwz-git/Daygo/frontend"
	"github.com/Jwz-git/Daygo/internal/agentbridge"
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/factory"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
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
	if err := factory.PrepareDesktopHost(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	var closeResources []func()
	cleanup := sync.OnceFunc(func() {
		cancel()
		for i := len(closeResources) - 1; i >= 0; i-- {
			closeResources[i]()
		}
	})
	defer cleanup()

	system := factory.NewSystem()
	if closer, ok := system.(interface{ Close() }); ok {
		closeResources = append(closeResources, closer.Close)
	}
	backend := NewBackend(system, nil)
	backend.hostContext = ctx
	backend.setCapture(factory.NewCapture())
	backend.setApplicationInspector(factory.NewApplicationInspector())
	backend.setSecrets(secrets.New())
	emitter := NewWailsEmitter()
	backend.setEventEmitter(emitter)
	backend.startSystemEventPump()

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
		recordingsRoot, rootErr := backend.recordingRoot(ctx)
		if rootErr != nil {
			log.Printf("recording location unavailable: %v", rootErr)
		}
		// Frame playback serves screenshots from the recordings directory
		// next to the database (same root GetRecordingDirectory reports).
		backend.attachMedia(recordingsRoot)
		closeResources = append(closeResources, func() { _ = store.Close() })
		closeResources = append(closeResources, backend.stopTimelineTasks)
		backend.startCaptureOwnerServices(ctx, dir, recordingsRoot, rootErr, goruntime.GOOS)

		// Maintenance is owned by this context, so cancelling it at shutdown
		// stops the goroutine. There is no global scheduler to leak
		// (docs/modules/data.md).
		// The cleanup pass reads the recording limit live from settings: a
		// failed read returns 0 — the documented "no limit" — so the pass
		// skips rather than deleting on uncertain ground.
		if writable, _ := backend.instanceOwnership(); writable {
			audit, err := os.OpenFile(filepath.Join(dir, "agent-writes.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				log.Printf("agent audit unavailable: %v", err)
			} else {
				_ = audit.Chmod(0o600)
				bridge := agentbridge.NewServer(agentWriteHandler{backend: backend}, audit)
				if err := bridge.Start(ctx, filepath.Join(dir, "agent.sock")); err != nil {
					log.Printf("agent socket unavailable: %v", err)
					_ = audit.Close()
				} else {
					backend.agentSocketActive.Store(true)
					closeResources = append(closeResources, func() {
						backend.agentSocketActive.Store(false)
						_ = bridge.Close()
						_ = audit.Close()
					})
				}
			}
			maintainer := storage.NewMaintainer(store, storage.MaintainerOptions{
				BackupDir: dir,
				RecordingsRootFunc: func() string {
					root, err := backend.recordingRoot(ctx)
					if err != nil {
						return ""
					}
					info, err := os.Stat(root)
					if err != nil || !info.IsDir() {
						return ""
					}
					return root
				},
				CleanupLock: &backend.moveMu,
				RecordingsLimit: func() int64 {
					snapshot, err := backend.loadSettings(ctx)
					if err != nil {
						return 0
					}
					return snapshot.RecordingsLimitBytes
				},
			})
			go maintainer.Run(ctx)
		}

	}
	// Start the updater only after storage ownership is known. Sparkle and
	// WinSparkle may schedule a check immediately; an early update must not see
	// the default non-owner state or race database/recorder composition.
	backend.setUpdater(factory.NewUpdater())
	backend.startUpdaterEventPump(ctx)

	windowOptions := desktopWindowOptions(goruntime.GOOS)
	var host *application.App
	var window application.Window
	var quitPreparing, quitPrepared atomic.Bool
	showWindow := func() {
		if err := backend.exitBackground(ctx); err != nil {
			log.Printf("restore dock icon on reopen unavailable: %v", err)
		}
		if goruntime.GOOS == "darwin" {
			application.InvokeSync(host.Show)
		}
		window.Show()
		if goruntime.GOOS == "darwin" {
			// Unhiding and ordering a window front do not activate the application.
			window.Focus()
		}
		backend.setWindowHidden(false)
	}
	host = application.New(application.Options{
		Name:                        "Daygo",
		Mac:                         application.MacOptions{ActivationPolicy: application.ActivationPolicyRegular},
		Windows:                     application.WindowsOptions{DisableQuitOnLastWindowClosed: true},
		Linux:                       application.LinuxOptions{DisableQuitOnLastWindowClosed: true},
		DisableDefaultSignalHandler: true,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(frontend.Assets),
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					switch req.URL.Path {
					case "/media/frame", "/media/thumbnail", "/favicon":
						backend.serveAsset(w, req)
					default:
						next.ServeHTTP(w, req)
					}
				})
			},
		},
		Services: []application.Service{application.NewServiceWithOptions(backend, application.ServiceOptions{
			// Error() supplies the public message. Keep the internal cause off the wire.
			MarshalError: func(error) []byte { return []byte("null") },
		})},
		OnShutdown: func() {
			emitter.setPublisher(nil)
			cancel()
			backend.shutdown()
			cleanup()
		},
		ShouldQuit: func() bool {
			if quitPrepared.Load() {
				return true
			}
			if !quitPreparing.CompareAndSwap(false, true) {
				return false
			}
			// The native termination callback can run on the main thread.
			// Leave that thread available while capture or dialogs complete.
			go func() {
				defer quitPreparing.Store(false)
				if backend.quitIntent(true) == quitRelaunchForPermission {
					if err := backend.beginPermissionRestart(); err != nil {
						log.Printf("permission restart cancelled: %v", err)
						return
					}
				} else if err := backend.prepareTermination(backend.stopRecorder); err != nil {
					log.Printf("quit cancelled: recording finalization failed: %s", recordingFailureReason(err))
					showWindow()
					labels := backend.statusLabels.get()
					dialog := host.Dialog.Warning().SetTitle(labels.ActionFailedTitle).SetMessage(labels.QuitFailed)
					keepOpen := dialog.AddButton(labels.KeepOpen)
					quitAnyway := dialog.AddButton(labels.QuitAnyway)
					allow := false
					quitAnyway.OnClick(func() { allow = true })
					dialog.SetDefaultButton(keepOpen).SetCancelButton(keepOpen)
					dialog.Show()
					if !allow && !backend.systemShuttingDown.Load() {
						return
					}
				}
				quitPrepared.Store(true)
				host.Quit()
			}()
			return false
		},
	})
	installDesktopMenu(host, backend.nativeLabels.get().ApplicationMenu)
	window = host.Window.NewWithOptions(windowOptions)
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if quitPrepared.Load() {
			return
		}
		event.Cancel()
		backend.setWindowHidden(true)
		if goruntime.GOOS == "darwin" {
			application.InvokeSync(host.Hide)
		} else {
			window.Hide()
		}
		go func() {
			if err := backend.enterBackground(ctx); err != nil {
				log.Printf("drop dock icon on window close unavailable: %v", err)
			}
		}()
	})
	backend.paintWindowBackground = func(r, g, b uint8) {
		window.SetBackgroundColour(application.NewRGBA(r, g, b, 255))
	}
	backend.pickRecordingDirectory = func() (string, error) {
		return host.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).
			CanCreateDirectories(true).AttachToWindow(window).PromptForSingleSelection()
	}
	backend.chatgptOpenURL = host.Browser.OpenURL
	backend.setApplicationPicker(wailsApplicationPicker{host: host, window: window, labels: backend.nativeLabels.get})

	requestShutdown := func() {
		backend.requestQuit()
		host.Quit()
	}
	var ready atomic.Bool
	host.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		backend.configureUpdateInstall(requestShutdown)
		backend.setShutdownRequester(requestShutdown)
		emitter.setPublisher(func(name string, payload any) {
			host.Event.Emit(name, payload)
		})
		backend.pushApplicationMenuCopy()
		ready.Store(true)
		if backend.system != nil {
			backend.setStatusUpdater(func(item platform.StatusItemState) error {
				return backend.system.SetStatusItem(ctx, item)
			})
			backend.updateStatus(backend.recorderState())
		}
		backend.loadDockPreference(ctx)
		reportActionError := func(err error) {
			showWindow()
			labels := backend.statusLabels.get()
			if presenter, ok := backend.system.(platform.StatusMessagePresenter); ok {
				if err := presenter.ShowStatusMessage(ctx, platform.StatusMessage{Title: labels.ActionFailedTitle, Message: nativeActionErrorMessage(err, labels), Button: labels.OK}); err != nil {
					log.Printf("native action feedback unavailable")
				}
				return
			}
			dialog := host.Dialog.Error().SetTitle(labels.ActionFailedTitle).SetMessage(nativeActionErrorMessage(err, labels))
			ok := dialog.AddButton(labels.OK)
			dialog.SetDefaultButton(ok).SetCancelButton(ok)
			dialog.Show()
		}
		// Reopen a backgrounded window; leave ordinary activation to the system.
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
					reportActionError(err)
					return
				}
				// The directory only exists after the first capture; create
				// it so the folder always opens instead of failing silently.
				if err := os.MkdirAll(dir, 0o755); err != nil {
					reportActionError(err)
					return
				}
				// Finder, not BrowserOpenURL: Wails' URL validator rejects the
				// file:// scheme outright, so a file URL never opens the folder.
				if err := backend.system.RevealPath(ctx, dir); err != nil {
					reportActionError(err)
				}
			case "quit":
				requestShutdown()
			case "toggle_pause", "pause_15", "pause_30", "pause_60", "pause_indefinite":
				if err := backend.runStatusRecordingAction(action); err != nil {
					reportActionError(err)
				}
			}
		})
		// After the status action is installed so the status item reflects
		// the state the auto-start produces.
		backend.maybeAutoStartRecording()
	})

	signalCtx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	go func() {
		<-signalCtx.Done()
		if ctx.Err() != nil {
			return
		}
		if ready.Load() {
			backend.requestSystemShutdown()
			return
		}
		backend.shutdown()
		cleanup()
		os.Exit(0)
	}()

	err = host.Run()
	if err != nil {
		return fmt.Errorf("run Daygo desktop shell: %w", err)
	}
	return nil
}
