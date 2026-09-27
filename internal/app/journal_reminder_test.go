package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
	"github.com/Jwz-git/Daygo/internal/settings"
)

// reminderBackend builds a read-write backend on a fresh store with the given
// fake system and clock, so a reminder sync runs against real settings storage
// but a deterministic platform and time.
func reminderBackend(t *testing.T, system platform.System, now time.Time) *Backend {
	t.Helper()
	store := openTestStore(t, t.TempDir(), false)
	return newBackend(fixedClock{now: now}, system, store, true, true)
}

// reminderSettings returns the typed settings accessor for a test backend, so a
// test can seed or change the reminder settings directly.
func reminderSettings(t *testing.T, backend *Backend) *settings.Settings {
	t.Helper()
	access, err := backend.settingsAccess()
	if err != nil {
		t.Fatalf("settingsAccess: %v", err)
	}
	return access
}

func reminderLabels() NativeUiLabelsDTO {
	labels := defaultNativeUiLabels()
	labels.JournalReminderTitle = "写今天的日记"
	labels.JournalReminderBody = "花几分钟记下今天的进展。"
	return labels
}

// A disabled reminder schedules nothing: the default is off, so the first sync
// must be a no-op rather than arming a notification the user never asked for.
func TestJournalReminderDisabledSchedulesNothing(t *testing.T) {
	system := fake.NewSystem()
	backend := reminderBackend(t, system, time.Date(2026, 9, 27, 12, 0, 0, 0, time.Local))
	backend.nativeLabels.set(reminderLabels())

	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("journalReminderSync: %v", err)
	}
	if calls := system.ScheduleCalls(); calls != 0 {
		t.Fatalf("ScheduleNotification called %d times while disabled, want 0", calls)
	}
}

// Enabling the reminder schedules one notification for the next occurrence of
// the configured wall-clock time, carrying the localized copy and the stable id.
func TestJournalReminderSchedulesAtConfiguredTime(t *testing.T) {
	system := fake.NewSystem()
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.Local)
	backend := reminderBackend(t, system, now)
	backend.nativeLabels.set(reminderLabels())
	enableReminder(t, backend, "18:00")

	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("journalReminderSync: %v", err)
	}
	n, ok := system.ScheduledNotification(journalReminderID)
	if !ok {
		t.Fatal("no notification scheduled for an enabled reminder")
	}
	if n.Title != "写今天的日记" || n.Body != "花几分钟记下今天的进展。" {
		t.Fatalf("notification copy = %q / %q, want the pushed labels", n.Title, n.Body)
	}
	want := time.Date(2026, 9, 27, 18, 0, 0, 0, time.Local)
	if n.DeliverAt == nil || !n.DeliverAt.Equal(want) {
		t.Fatalf("DeliverAt = %v, want %v", n.DeliverAt, want)
	}
}

// Once the configured time has passed today, the next occurrence is tomorrow —
// the reminder fires daily, so a sync after 18:00 must not schedule in the past.
func TestJournalReminderRollsToTomorrow(t *testing.T) {
	system := fake.NewSystem()
	now := time.Date(2026, 9, 27, 20, 0, 0, 0, time.Local)
	backend := reminderBackend(t, system, now)
	backend.nativeLabels.set(reminderLabels())
	enableReminder(t, backend, "18:00")

	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("journalReminderSync: %v", err)
	}
	n, ok := system.ScheduledNotification(journalReminderID)
	if !ok || n.DeliverAt == nil {
		t.Fatal("no notification scheduled")
	}
	want := time.Date(2026, 9, 28, 18, 0, 0, 0, time.Local)
	if !n.DeliverAt.Equal(want) {
		t.Fatalf("DeliverAt = %v, want tomorrow %v", *n.DeliverAt, want)
	}
}

// The deliverAt exactly at the configured minute counts as passed: a sync at
// 18:00:00 must target tomorrow, not fire a notification for this instant.
func TestJournalReminderAtExactTimeRollsForward(t *testing.T) {
	loc := time.Local
	now := time.Date(2026, 9, 27, 18, 0, 0, 0, loc)
	next := nextReminderAt(now, "18:00", loc)
	want := time.Date(2026, 9, 28, 18, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("nextReminderAt at the boundary = %v, want %v", next, want)
	}
}

// A repeated sync with the same settings is idempotent: the app must not
// re-issue the platform call, which would risk a duplicate or a reset of the
// system's own delivery bookkeeping.
func TestJournalReminderSyncIsIdempotent(t *testing.T) {
	system := fake.NewSystem()
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.Local)
	backend := reminderBackend(t, system, now)
	backend.nativeLabels.set(reminderLabels())
	enableReminder(t, backend, "18:00")

	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if calls := system.ScheduleCalls(); calls != 1 {
		t.Fatalf("ScheduleNotification called %d times across two identical syncs, want 1", calls)
	}
}

// Changing the time re-schedules: the notification must follow the new setting.
func TestJournalReminderReschedulesOnTimeChange(t *testing.T) {
	system := fake.NewSystem()
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.Local)
	backend := reminderBackend(t, system, now)
	backend.nativeLabels.set(reminderLabels())
	enableReminder(t, backend, "18:00")
	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	if _, _, err := reminderSettings(t, backend).Apply(context.Background(), settings.Patch{JournalReminderTime: ptrString("20:30")}); err != nil {
		t.Fatalf("change reminder time: %v", err)
	}
	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	n, ok := system.ScheduledNotification(journalReminderID)
	if !ok || n.DeliverAt == nil {
		t.Fatal("no notification scheduled after a time change")
	}
	want := time.Date(2026, 9, 27, 20, 30, 0, 0, time.Local)
	if !n.DeliverAt.Equal(want) {
		t.Fatalf("DeliverAt = %v, want %v", *n.DeliverAt, want)
	}
	if calls := system.ScheduleCalls(); calls != 2 {
		t.Fatalf("ScheduleNotification called %d times, want 2 (initial + reschedule)", calls)
	}
}

// A language change re-issues the notification so the delivered copy follows the
// interface language rather than staying on whatever was pushed first.
func TestJournalReminderReschedulesOnCopyChange(t *testing.T) {
	system := fake.NewSystem()
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.Local)
	backend := reminderBackend(t, system, now)
	backend.nativeLabels.set(reminderLabels())
	enableReminder(t, backend, "18:00")
	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	labels := reminderLabels()
	labels.JournalReminderTitle = "Write today's journal"
	labels.JournalReminderBody = "Take a few minutes to log your progress."
	backend.nativeLabels.set(labels)
	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	n, ok := system.ScheduledNotification(journalReminderID)
	if !ok || n.Body != labels.JournalReminderBody {
		t.Fatalf("notification body = %q, want the new copy %q", n.Body, labels.JournalReminderBody)
	}
	if calls := system.ScheduleCalls(); calls != 2 {
		t.Fatalf("ScheduleNotification called %d times, want 2", calls)
	}
}

// Disabling cancels: the reminder must stop, and no further schedule is issued.
func TestJournalReminderDisableCancels(t *testing.T) {
	system := fake.NewSystem()
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.Local)
	backend := reminderBackend(t, system, now)
	backend.nativeLabels.set(reminderLabels())
	enableReminder(t, backend, "18:00")
	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	if _, _, err := reminderSettings(t, backend).Apply(context.Background(), settings.Patch{JournalReminderEnabled: ptrBool(false)}); err != nil {
		t.Fatalf("disable reminder: %v", err)
	}
	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if _, ok := system.ScheduledNotification(journalReminderID); ok {
		t.Fatal("notification still scheduled after disabling the reminder")
	}
	calls, ids := system.CancelCalls()
	if calls != 1 || len(ids) != 1 || ids[0] != journalReminderID {
		t.Fatalf("CancelNotifications calls=%d ids=%v, want 1 call for [%s]", calls, ids, journalReminderID)
	}
}

// A disable sync repeats must not cancel twice: the second sync has nothing left
// to cancel, so it stays a no-op.
func TestJournalReminderDisableIsIdempotent(t *testing.T) {
	system := fake.NewSystem()
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.Local)
	backend := reminderBackend(t, system, now)
	backend.nativeLabels.set(reminderLabels())
	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("sync while disabled: %v", err)
	}
	if calls, _ := system.CancelCalls(); calls != 0 {
		t.Fatalf("CancelNotifications called %d times for a reminder that was never enabled, want 0", calls)
	}
}

// A read-only instance schedules nothing: it holds neither the write lock nor
// the capture ownership that the reminder is scoped to.
func TestJournalReminderReadOnlyInstanceSchedulesNothing(t *testing.T) {
	system := fake.NewSystem()
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.Local)
	dir := t.TempDir()
	// A first store holds the write + capture lock, so the second open on the
	// same directory is forced read-only by the connection layer.
	writerStore := openTestStore(t, dir, true)
	writer := newBackend(fixedClock{now: now}, system, writerStore, true, true)
	writer.nativeLabels.set(reminderLabels())
	enableReminder(t, writer, "18:00")

	readerStore := openTestStore(t, dir, true)
	readOnly := newBackend(fixedClock{now: now}, system, readerStore, false, false)
	readOnly.nativeLabels.set(reminderLabels())
	if err := readOnly.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("journalReminderSync on reader: %v", err)
	}
	if calls := system.ScheduleCalls(); calls != 0 {
		t.Fatalf("read-only instance called ScheduleNotification %d times, want 0", calls)
	}
}

// A platform failure (e.g. notifications not authorized) surfaces as a
// retryable error for the runner to log rather than crashing anything, and is
// not remembered as applied — so once the platform recovers, the next tick
// re-issues the schedule.
func TestJournalReminderScheduleFailureIsRetried(t *testing.T) {
	system := fake.NewSystem()
	system.SetScheduleError(errors.New("notifications not authorized"))
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.Local)
	backend := reminderBackend(t, system, now)
	backend.nativeLabels.set(reminderLabels())
	enableReminder(t, backend, "18:00")

	err := backend.journalReminderSync(context.Background())
	assertAppCode(t, err, apperr.NativeUnavailable)
	// The failed schedule must not be remembered as applied, or a later tick
	// would believe the reminder is armed when the platform has nothing.
	if _, ok := system.ScheduledNotification(journalReminderID); ok {
		t.Fatal("a failed schedule was recorded as applied")
	}

	// The platform recovers: the next sync arms the reminder.
	system.SetScheduleError(nil)
	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("sync after the platform recovered: %v", err)
	}
	if _, ok := system.ScheduledNotification(journalReminderID); !ok {
		t.Fatal("reminder was not scheduled after the platform recovered")
	}
}

// nextReminderAt uses local wall-clock construction, so the result is right
// across a DST transition rather than skewed by a fixed 24h offset.
func TestNextReminderAtAcrossDST(t *testing.T) {
	loc := mustLocation(t, "America/Los_Angeles")
	// Spring forward on 2026-03-08; a reminder on the day before and the day of
	// the transition must both land on their own local 18:00.
	before := time.Date(2026, time.March, 7, 12, 0, 0, 0, loc)
	next := nextReminderAt(before, "18:00", loc)
	if got := next.Format("2006-01-02 15:04"); got != "2026-03-07 18:00" {
		t.Fatalf("nextReminderAt before DST = %s, want 2026-03-07 18:00", got)
	}
	// 12:00 on the transition day is after the 02:00 jump; 18:00 is still ahead.
	onDay := time.Date(2026, time.March, 8, 12, 0, 0, 0, loc)
	next = nextReminderAt(onDay, "18:00", loc)
	if got := next.Format("2006-01-02 15:04"); got != "2026-03-08 18:00" {
		t.Fatalf("nextReminderAt on DST day = %s, want 2026-03-08 18:00", got)
	}
}

// A half-hour-offset zone still lands on the plain local wall clock.
func TestNextReminderAtHalfHourZone(t *testing.T) {
	loc := mustLocation(t, "Asia/Kolkata")
	now := time.Date(2026, 9, 27, 9, 15, 0, 0, loc)
	next := nextReminderAt(now, "18:30", loc)
	if got := next.Format("2006-01-02 15:04"); got != "2026-09-27 18:30" {
		t.Fatalf("nextReminderAt in a half-hour zone = %s, want 2026-09-27 18:30", got)
	}
}

// Before the frontend pushes localized copy, the sync schedules nothing: an
// untitled notification is worse than a one-tick delay, and the next tick
// re-issues once the copy arrives.
func TestJournalReminderWaitsForCopy(t *testing.T) {
	system := fake.NewSystem()
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.Local)
	backend := reminderBackend(t, system, now)
	// Clear the seeded labels to stand in for "frontend has not pushed yet".
	labels := defaultNativeUiLabels()
	labels.JournalReminderTitle = ""
	labels.JournalReminderBody = ""
	backend.nativeLabels.set(labels)
	enableReminder(t, backend, "18:00")

	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("journalReminderSync: %v", err)
	}
	if calls := system.ScheduleCalls(); calls != 0 {
		t.Fatalf("ScheduleNotification called %d times before copy was pushed, want 0", calls)
	}

	// Copy arrives: the next sync arms the reminder.
	backend.nativeLabels.set(reminderLabels())
	if err := backend.journalReminderSync(context.Background()); err != nil {
		t.Fatalf("sync after copy push: %v", err)
	}
	if _, ok := system.ScheduledNotification(journalReminderID); !ok {
		t.Fatal("reminder was not scheduled after the copy was pushed")
	}
}

// enableReminder persists an enabled reminder at the given time directly through
// the settings layer, bypassing the binding so the sync under test is the only
// thing exercised.
func enableReminder(t *testing.T, backend *Backend, at string) {
	t.Helper()
	if _, _, err := reminderSettings(t, backend).Apply(context.Background(), settings.Patch{
		JournalReminderEnabled: ptrBool(true),
		JournalReminderTime:    ptrString(at),
	}); err != nil {
		t.Fatalf("enable reminder: %v", err)
	}
}
