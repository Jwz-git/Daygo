package app

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform"
)

// journalReminderID is the single, stable identifier of the daily journal
// reminder notification. A re-schedule with the same id replaces the previous
// one, so the app never accumulates duplicates; a cancel passes the same id.
// See docs/decisions/notifications-journal-reminder.md.
const journalReminderID = "journal-reminder"

const (
	// journalReminderInterval is how often the resident agent re-reads the
	// reminder settings and reconciles them with the platform. The platform owns
	// the actual delivery time via DeliverAt, so this only needs to catch two
	// changes: the user editing the setting, and the clock crossing the
	// configured minute. A minute-level cadence is unnecessary; each tick is one
	// indexed settings read plus, at most, one platform call.
	journalReminderInterval = 5 * time.Minute
)

// journalReminderState is the last reconciled reminder, kept so a tick that
// finds nothing changed makes no platform call. It is process-local: after a
// restart the first tick re-issues the schedule once, which is harmless because
// the platform keys by id.
type journalReminderState struct {
	armed     bool // a schedule has been issued and not cancelled
	enabled   bool
	clockTime string
	deliverAt time.Time
	title     string
	body      string
}

// runJournalReminder keeps the daily journal reminder in sync with the user's
// settings for the life of ctx. Like runStandupBackfill it is launched from the
// read-write startup block: only the capture owner schedules notifications, and
// a read-only second instance must not arm one.
func (b *Backend) runJournalReminder(ctx context.Context) {
	ticker := time.NewTicker(journalReminderInterval)
	defer ticker.Stop()
	for {
		if err := b.journalReminderSync(ctx); err != nil && ctx.Err() == nil {
			// A sync-level error (settings unreadable) is not fatal: the next
			// tick retries. Log the code only — apperr messages carry no user data.
			log.Printf("journal reminder: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// journalReminderSync reconciles the platform notification with the current
// reminder settings. It is the unit under test.
//
// The reconcile is idempotent: it computes the desired state (armed with a
// given time and copy, or disarmed) and only calls the platform when that
// differs from what was last issued. Re-issuing an unchanged schedule would
// risk resetting the system's own delivery bookkeeping, and cancelling twice
// would be a pointless platform call on every tick.
func (b *Backend) journalReminderSync(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil
	}
	if err := b.requireTimelineWrite(); err != nil {
		// A read-only instance has nothing to do; not an error worth logging.
		return nil
	}
	if b.system == nil {
		// No platform: headless construction or a build without a System adapter.
		return nil
	}
	access, err := b.settingsAccess()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, settingsTimeout)
	defer cancel()
	snapshot, err := access.Load(ctx)
	if err != nil {
		return mapStorageError("read reminder settings", err)
	}

	labels := b.nativeLabels.get()
	title, body := labels.JournalReminderTitle, labels.JournalReminderBody
	if title == "" || body == "" {
		// The frontend has not pushed copy yet. Scheduling an untitled
		// notification is worse than waiting one tick, since the next tick
		// re-issues with the real copy.
		return nil
	}

	b.reminderMu.Lock()
	defer b.reminderMu.Unlock()
	state := &b.reminder

	if !snapshot.ReminderEnabled {
		if state.armed {
			if err := b.system.CancelNotifications(ctx, []string{journalReminderID}); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return apperr.E(apperr.NativeUnavailable, "cancel journal reminder", err)
			}
			*state = journalReminderState{}
		}
		return nil
	}

	deliverAt := nextReminderAt(b.clock.Now(), snapshot.ReminderTime, b.clock.Now().Location())
	if state.armed &&
		state.enabled &&
		state.clockTime == snapshot.ReminderTime &&
		state.deliverAt.Equal(deliverAt) &&
		state.title == title &&
		state.body == body {
		return nil
	}

	if err := b.system.ScheduleNotification(ctx, platform.Notification{
		ID:        journalReminderID,
		Title:     title,
		Body:      body,
		DeliverAt: &deliverAt,
	}); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		// A platform refusal (e.g. notifications not authorized) is retried on
		// the next tick; the state stays unarmed so the retry re-issues.
		return apperr.E(apperr.NativeUnavailable, "schedule journal reminder", err)
	}
	*state = journalReminderState{
		armed:     true,
		enabled:   true,
		clockTime: snapshot.ReminderTime,
		deliverAt: deliverAt,
		title:     title,
		body:      body,
	}
	return nil
}

// nextReminderAt returns the next local occurrence of the HH:mm clock time at or
// after now. A time later today is used as-is; a time already passed (or exactly
// now) rolls to tomorrow. Construction goes through time.Date in loc, so a DST
// transition between now and the target resolves to the correct local wall
// clock rather than a fixed 24-hour offset.
//
// clockTime is assumed already normalized to "HH:mm" by internal/settings; an
// unparseable value falls back to the settings default so a corrupt stored value
// cannot wedge the scheduler.
func nextReminderAt(now time.Time, clockTime string, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	hour, minute, err := parseClockTime(clockTime)
	if err != nil {
		hour, minute, _ = parseClockTime("18:00")
	}
	local := now.In(loc)
	candidate := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
	if !candidate.After(local) {
		next := local.AddDate(0, 0, 1)
		candidate = time.Date(next.Year(), next.Month(), next.Day(), hour, minute, 0, 0, loc)
	}
	return candidate
}

// parseClockTime parses a normalized "HH:mm" clock time.
func parseClockTime(value string) (hour, minute int, err error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, 0, errors.New("clock time is not HH:mm")
	}
	hour, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	minute, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, errors.New("clock time out of range")
	}
	return hour, minute, nil
}
