package app

import (
	"context"
	"sync"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/chat"
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/recorder"
	"github.com/Jwz-git/Daygo/internal/storage"
	daytime "github.com/Jwz-git/Daygo/internal/timeutil"
)

const (
	apiRevision = 1
	// TODO(m1): inject from build info once the signing pipeline exists.
	appVersion = "0.0.0"
)

// EventEmitter publishes an event to the frontend. It is an interface so the
// binding layer can be tested without a Wails runtime: docs/02 §2.1 keeps the
// Go core testable with no GUI, and a direct runtime.EventsEmit call would
// break that for every test that exercises an event-producing method.
type EventEmitter interface {
	Emit(name EventName, payload any)
}

// Clock makes GetDayContext deterministic without giving the binding layer a
// second source of timezone truth.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// nopEmitter discards events. It is the default so a Backend built without an
// emitter (a test, or a non-Wails host) does not panic on the first write.
type nopEmitter struct{}

func (nopEmitter) Emit(EventName, any) {}

// Backend is the Wails-bound surface. It coordinates pure Go policies, the
// storage foundation and platform ports; it contains no native implementation
// itself.
//
// EXPORTED METHODS ARE THE FRONTEND API. Wails binds every exported method on
// this type, so an exported helper silently becomes part of the contract in
// docs/05 §5.5.1 and lands in the generated Backend.d.ts. Anything that is not
// a binding method stays unexported, however inconvenient; bindings_test.go
// fails when the two disagree.
type Backend struct {
	clock                Clock
	system               platform.System
	capture              platform.Capture
	applicationInspector platform.ApplicationInspector
	applicationPicker    applicationPicker
	secrets              platform.Secrets
	storage              *storage.Store
	recorder             *recorder.Recorder
	recorderMu           sync.Mutex
	systemEventMu        sync.Mutex
	systemEventBuffer    []platform.SystemEvent
	statusActionMu       sync.RWMutex
	statusAction         func(string)
	statusUpdaterMu      sync.RWMutex
	statusUpdater        func(recorder.State)
	// no store is attached. With a store present they are ignored in favor of
	// the real instance locks, so the reported ownership cannot drift from
	// the locks actually held (docs/modules/data.md).
	canWrite       bool
	isCaptureOwner bool

	// storageErr records why the database is unavailable when Open failed. It
	// is reported through diagnostics rather than thrown at startup, so a
	// second instance or a damaged file still leaves a usable window.
	storageMu  sync.RWMutex
	storageErr error

	// captureMu serializes the test binding's path allocation and native call.
	// It prevents two rapid UI clicks from racing over a generated output name.
	captureMu sync.Mutex

	// emitter publishes frontend events. It is never nil after construction:
	// a Backend built through NewBackend or newBackend gets at least the
	// no-op emitter.
	emitter EventEmitter

	// chat is the chat service, wired lazily on first use by chatService.
	// chatMu guards the wiring; the service itself is concurrency-safe.
	chat   *chat.Service
	chatMu sync.Mutex

	// timelineEvents tracks pending merged timeline:updated emits, one timer
	// per day within the 200 ms merge window (docs/05 §5.5.3).
	timelineEvents   map[string]*time.Timer
	timelineEventsMu sync.Mutex
}

// setEventEmitter installs the Wails-backed emitter. It is called once during
// startup, before the window exists, so no synchronization is needed.
//
// Unexported on purpose: it is composition, not a binding. Exporting it would
// hand the frontend a way to replace the event emitter.
func (b *Backend) setEventEmitter(emitter EventEmitter) {
	if emitter == nil {
		emitter = nopEmitter{}
	}
	b.emitter = emitter
}

// setCapture installs the platform adapter at the composition root. It stays
// unexported because the adapter is an implementation detail, not a binding.
func (b *Backend) setCapture(capture platform.Capture) {
	b.capture = capture
}

// setApplicationInspector installs the native identity resolver used by the
// temporary capture test picker.
func (b *Backend) setApplicationInspector(inspector platform.ApplicationInspector) {
	b.applicationInspector = inspector
}

// setApplicationPicker installs UI interaction after Wails provides its live
// runtime context. It stays separate from identity inspection for headless tests.
func (b *Backend) setApplicationPicker(picker applicationPicker) {
	b.applicationPicker = picker
}

// setSecrets installs the keychain adapter at the composition root. Unexported
// for the same reason as setCapture: the adapter is not a binding surface.
func (b *Backend) setSecrets(s platform.Secrets) {
	b.secrets = s
}

// emitSettingsChanged publishes the keys a settings write committed.
func (b *Backend) emitSettingsChanged(keys []string) {
	if b == nil || b.emitter == nil {
		return
	}
	b.emitter.Emit(EventSettingsChanged, SettingsChangedPayload{Keys: keys})
}

// SettingsChangedPayload is the settings:changed event body (docs/05 §5.5.3).
// It carries only key names: the frontend re-reads through GetSettings rather
// than applying an optimistic update from the event.
type SettingsChangedPayload struct {
	Keys []string `json:"keys"`
}

// NewBackend wires the binding surface. system may be nil while the native
// adapter is unavailable; the permission methods then return native_unavailable
// instead of pretending a platform result exists.
//
// store may be nil too: without a database the app is still able to report day
// context, and it must not claim write ownership it does not have.
func NewBackend(system platform.System, store *storage.Store) *Backend {
	return newBackend(systemClock{}, system, store, false, false)
}

func newBackend(clock Clock, system platform.System, store *storage.Store, canWrite, isCaptureOwner bool) *Backend {
	return &Backend{
		clock:          clock,
		system:         system,
		storage:        store,
		canWrite:       canWrite,
		isCaptureOwner: isCaptureOwner,
		emitter:        nopEmitter{},
	}
}

// attachStorage binds an open store. It is called once during startup, before
// the window is created, so no locking is needed against readers; the mutex
// exists for setStorageError, which can run while bindings are serving.
func (b *Backend) attachStorage(store *storage.Store) {
	b.storage = store
}

// setStorageError records a failed open. The error stays internal: diagnostics
// reports the class of failure, never a path or a driver message that could
// carry user data (docs/07).
func (b *Backend) setStorageError(err error) {
	b.storageMu.Lock()
	defer b.storageMu.Unlock()
	b.storageErr = err
}

// storageFailure reports the recorded open failure, if any.
func (b *Backend) storageFailure() error {
	b.storageMu.RLock()
	defer b.storageMu.RUnlock()
	return b.storageErr
}

// store exposes the open store to other binding methods in this package. It is
// nil when the database is unavailable; callers must handle that rather than
// assuming persistence exists.
//
// startSystemEventPump owns the single platform event subscription and fans
// events into both the recorder and diagnostic/test consumers.
func (b *Backend) startSystemEventPump() {
	if b == nil || b.system == nil {
		return
	}
	go func(events <-chan platform.SystemEvent) {
		for event := range events {
			b.systemEventMu.Lock()
			if len(b.systemEventBuffer) >= 64 {
				b.systemEventBuffer = b.systemEventBuffer[1:]
			}
			b.systemEventBuffer = append(b.systemEventBuffer, event)
			b.systemEventMu.Unlock()
			b.recorderMu.Lock()
			r := b.recorder
			b.recorderMu.Unlock()
			if r != nil {
				r.HandleSystemEvent(event)
			}
			if event.Data.StatusItemID != nil {
				b.statusActionMu.RLock()
				handler := b.statusAction
				b.statusActionMu.RUnlock()
				if handler != nil {
					handler(*event.Data.StatusItemID)
				}
			}
		}
	}(b.system.Events())
}
func (b *Backend) setStatusUpdater(updater func(recorder.State)) {
	b.statusUpdaterMu.Lock()
	b.statusUpdater = updater
	b.statusUpdaterMu.Unlock()
}

func (b *Backend) updateStatus(state recorder.State) {
	b.statusUpdaterMu.RLock()
	updater := b.statusUpdater
	b.statusUpdaterMu.RUnlock()
	if updater != nil {
		updater(state)
	}
}
func (b *Backend) setStatusAction(handler func(string)) {
	b.statusActionMu.Lock()
	b.statusAction = handler
	b.statusActionMu.Unlock()
}

// Unexported on purpose: while it was exported, Wails bound it and pulled
// storage.Store into the generated models. Nothing outside this package may
// hold the database handle anyway (docs/02 §2.2 rule 5).
func (b *Backend) store() *storage.Store {
	return b.storage
}

// instanceOwnership reports what this process actually holds. With a store the
// answer comes from the instance locks; without one, the caller-supplied
// fallback applies. Both values are derived in one place so GetCapabilities and
// GetRecordingState can never disagree.
func (b *Backend) instanceOwnership() (canWrite, isCaptureOwner bool) {
	if b.storage == nil {
		return b.canWrite, b.isCaptureOwner
	}
	instance := b.storage.Instance()
	return instance.Mode == storage.ModeReadWrite, instance.CaptureOwner
}

// GetCapabilities reports only features that are actually usable. Ownership is
// read from the instance locks when a store is attached, so a read-only second
// instance reports canWrite false rather than the value it was constructed
// with.
func (b *Backend) GetCapabilities() (CapabilitiesDTO, error) {
	canWrite, isCaptureOwner := b.instanceOwnership()
	return CapabilitiesDTO{
		CanWrite:       canWrite,
		IsCaptureOwner: isCaptureOwner,
		Features:       b.features(),
		AppVersion:     appVersion,
		APIRevision:    apiRevision,
	}, nil
}

// features lists the feature flags this build can honestly advertise. A feature
// appears only when its bindings exist and return real results; the frontend
// decides what to render from this list rather than probing calls and
// interpreting failures (docs/05 §5.2.1).
func (b *Backend) features() []string {
	features := []string{"settings"}
	if b.storage != nil {
		// Persistence is real only when a database is actually open.
		features = append(features, "storage", "timeline")
	}
	return features
}

// GetDayContext resolves an empty day to the current logical day. A non-empty
// value must be strict yyyy-MM-dd; aliases such as "today" are rejected.
func (b *Backend) GetDayContext(day string) (DayContextDTO, error) {
	now := b.clock.Now()
	loc := now.Location()
	if loc == nil {
		return DayContextDTO{}, apperr.E(apperr.Internal, "local time zone is unavailable", nil)
	}
	standupDay := day
	if day == "" {
		day = daytime.LogicalDay(now, loc)
		standupDay = daytime.CalendarDay(now, loc)
	}

	start, end, err := daytime.DayWindow(day, loc)
	if err != nil {
		return DayContextDTO{}, apperr.E(apperr.InvalidArgument, "day must use yyyy-MM-dd", err)
	}
	weekStart, err := daytime.WeekStart(day, loc)
	if err != nil {
		return DayContextDTO{}, apperr.E(apperr.InvalidArgument, "day must use yyyy-MM-dd", err)
	}
	return DayContextDTO{
		Day:             day,
		StandupDay:      standupDay,
		WeekStart:       weekStart,
		DayStartTs:      start.Unix(),
		DayEndTs:        end.Unix(),
		NowTs:           now.Unix(),
		TimeZone:        daytime.ZoneName(loc),
		DayBoundaryHour: daytime.BoundaryHour,
	}, nil
}

// GetRecordingState reports permission, ownership, and the live Go recorder state.
func (b *Backend) GetRecordingState() (RecordingStateDTO, error) {
	permission, err := b.recordingPermission()
	if err != nil {
		return RecordingStateDTO{}, err
	}
	_, isCaptureOwner := b.instanceOwnership()
	state := RecordingState(b.recorderState())
	return RecordingStateDTO{State: state, Permission: permission, IsCaptureOwner: isCaptureOwner}, nil
}

// recordingPermission is GetRecordingState's single query: system unavailability
// is a legitimate answer here (state becomes unknown), not an error.
func (b *Backend) recordingPermission() (string, error) {
	if b.system == nil {
		return "", apperr.E(apperr.NativeUnavailable, "platform services are unavailable", nil)
	}
	return b.permissionState("screen recording", b.system.ScreenRecordingPermission)
}

// GetPermissionState returns the currently observed platform permissions.
func (b *Backend) GetPermissionState() (PermissionDTO, error) {
	if b.system == nil {
		return PermissionDTO{}, apperr.E(apperr.NativeUnavailable, "platform services are unavailable", nil)
	}
	screenRecording, err := b.permissionState("screen recording", b.system.ScreenRecordingPermission)
	if err != nil {
		return PermissionDTO{}, err
	}
	notifications, err := b.permissionState("notifications", b.system.NotificationsPermission)
	if err != nil {
		return PermissionDTO{}, err
	}
	return PermissionDTO{
		ScreenRecording: screenRecording,
		Notifications:   notifications,
		CanRequest:      screenRecording == string(platform.PermissionNotDetermined),
	}, nil
}

// permissionState calls one platform permission query under the binding-layer
// timeout, validating the returned state and mapping failures to apperr.
func (b *Backend) permissionState(op string, query func(ctx context.Context) (platform.PermissionState, error)) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	permission, err := query(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return "", apperr.E(apperr.NativeUnavailable, "platform request timed out", ctx.Err())
		}
		return "", apperr.E(apperr.NativeUnavailable, op+" permission query failed", err)
	}
	if !permission.Valid() {
		return "", apperr.E(apperr.Internal, "platform returned an invalid "+op+" permission state", nil)
	}
	return string(permission), nil
}
