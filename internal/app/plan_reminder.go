package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/insight"
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// The plan reminder (docs/modules/plan.md) keeps three kinds of system
// notification in sync with the plan and the recorded cards:
//
//   - a start notification per planned block with remind on, scheduled at its
//     start through DeliverAt so the system delivers it even if Daygo is busy;
//   - a one-off "distracted" alert when distraction inside the block in
//     progress reaches planDistractionMinMinutes and planDistractionShare of
//     the time elapsed so far;
//   - a one-off alert when the day's distraction passes the day goal's limit.
//
// Like the journal reminder it is a reconcile loop owned by the capture owner,
// with process-local bookkeeping: after a restart the schedules are re-issued
// once (the platform keys by id) and an alert already sent may repeat once.
// Distraction is measured from analysed cards, so it trails the screen by the
// analysis batch interval.

const (
	planReminderInterval      = 2 * time.Minute
	planStartHorizon          = 24 * time.Hour
	planDistractionMinMinutes = 10
	planDistractionShare      = 0.25
)

type planStartNotice struct {
	deliverAt int64
	title     string
	body      string
}

type planReminderState struct {
	starts  map[int64]planStartNotice
	day     string          // logical day the alerted set belongs to
	alerted map[string]bool // "block:<id>" / "day:<day>"
}

func planStartID(id int64) string       { return "plan-start-" + strconv.FormatInt(id, 10) }
func planDistractionID(id int64) string { return "plan-distraction-" + strconv.FormatInt(id, 10) }
func dayDistractionID(day string) string {
	return "plan-day-distraction-" + day
}

// nudgePlanReminder asks the loop to reconcile now, without blocking; a nudge
// already pending covers this one.
func (b *Backend) nudgePlanReminder() {
	if b.planNudge == nil {
		return
	}
	select {
	case b.planNudge <- struct{}{}:
	default:
	}
}

func (b *Backend) runPlanReminder(ctx context.Context) {
	ticker := time.NewTicker(planReminderInterval)
	defer ticker.Stop()
	for {
		if err := b.planReminderSync(ctx); err != nil && ctx.Err() == nil {
			log.Printf("plan reminder: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-b.planNudge:
		}
	}
}

// planReminderSync is one reconcile pass; it is the unit under test.
func (b *Backend) planReminderSync(ctx context.Context) error {
	if ctx.Err() != nil || b.requireTimelineWrite() != nil || b.system == nil {
		return nil
	}
	store := b.store()
	if store == nil {
		return nil
	}
	labels := b.nativeLabels.get()
	if labels.PlanStartTitle == "" || labels.PlanDistractionTitle == "" || labels.DayDistractionTitle == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, timelineTimeout)
	defer cancel()

	now := b.clock.Now()
	loc := now.Location()
	today := timeutil.LogicalDay(now, loc)
	tomorrow := timeutil.LogicalDay(now.Add(24*time.Hour), loc)

	b.planReminderMu.Lock()
	defer b.planReminderMu.Unlock()
	state := &b.planReminder
	if state.starts == nil {
		state.starts = map[int64]planStartNotice{}
	}
	if state.day != today {
		state.day, state.alerted = today, map[string]bool{}
	}

	todayBlocks, err := store.Plans().ForDay(ctx, today)
	if err != nil {
		return mapStorageError("read plan", err)
	}
	tomorrowBlocks, err := store.Plans().ForDay(ctx, tomorrow)
	if err != nil {
		return mapStorageError("read plan", err)
	}
	if err := b.syncPlanStarts(ctx, state, append(todayBlocks, tomorrowBlocks...), now, labels); err != nil {
		return err
	}
	return b.syncDistractionAlerts(ctx, store, state, todayBlocks, today, now, labels)
}

// syncPlanStarts schedules the start notification of every upcoming planned
// block with remind on, and cancels the ones that no longer apply (deleted,
// done, moved, remind off, or already started).
func (b *Backend) syncPlanStarts(ctx context.Context, state *planReminderState, blocks []storage.PlanBlock,
	now time.Time, labels NativeUiLabelsDTO) error {
	loc := now.Location()
	desired := map[int64]planStartNotice{}
	for _, block := range blocks {
		if block.Status != storage.PlanStatusPlanned || !block.Remind {
			continue
		}
		start := time.Unix(block.StartTs, 0)
		if !start.After(now) || start.Sub(now) > planStartHorizon {
			continue
		}
		replacer := strings.NewReplacer(
			"{title}", block.Title,
			"{start}", start.In(loc).Format("15:04"),
			"{end}", time.Unix(block.EndTs, 0).In(loc).Format("15:04"),
		)
		desired[block.ID] = planStartNotice{
			deliverAt: block.StartTs,
			title:     replacer.Replace(labels.PlanStartTitle),
			body:      replacer.Replace(labels.PlanStartBody),
		}
	}

	var stale []string
	for id := range state.starts {
		if _, ok := desired[id]; !ok {
			stale = append(stale, planStartID(id))
		}
	}
	if len(stale) > 0 {
		if err := b.system.CancelNotifications(ctx, stale); err != nil && !errors.Is(err, platform.ErrCapabilityUnavailable) {
			return apperr.E(apperr.NativeUnavailable, "cancel plan notifications", err)
		}
		for id := range state.starts {
			if _, ok := desired[id]; !ok {
				delete(state.starts, id)
			}
		}
	}

	for id, notice := range desired {
		if state.starts[id] == notice {
			continue
		}
		deliverAt := time.Unix(notice.deliverAt, 0)
		err := b.system.ScheduleNotification(ctx, platform.Notification{
			ID: planStartID(id), Title: notice.title, Body: notice.body, DeliverAt: &deliverAt,
		})
		if errors.Is(err, platform.ErrCapabilityUnavailable) {
			return nil
		}
		if err != nil {
			return apperr.E(apperr.NativeUnavailable, "schedule plan notification", err)
		}
		state.starts[id] = notice
	}
	return nil
}

// syncDistractionAlerts sends each distraction alert at most once per block and
// once per day.
func (b *Backend) syncDistractionAlerts(ctx context.Context, store *storage.Store, state *planReminderState,
	blocks []storage.PlanBlock, today string, now time.Time, labels NativeUiLabelsDTO) error {
	loc := now.Location()
	goal, refs, goalFound, err := store.Goals().Get(ctx, today)
	if err != nil {
		return mapStorageError("read day goal", err)
	}
	var goalNames []string
	for _, ref := range refs {
		if ref.Role == storage.GoalRoleDistraction {
			goalNames = append(goalNames, ref.Name)
		}
	}
	dayLimit := 0
	if goalFound && !goal.IsSkipped {
		dayLimit = goal.DistractionLimitMinutes
	}

	var active []storage.PlanBlock
	for _, block := range blocks {
		if block.Status == storage.PlanStatusPlanned && block.StartTs <= now.Unix() && now.Unix() < block.EndTs &&
			!state.alerted["block:"+strconv.FormatInt(block.ID, 10)] {
			active = append(active, block)
		}
	}
	checkDay := dayLimit > 0 && !state.alerted["day:"+today]
	if len(active) == 0 && !checkDay {
		return nil
	}

	dayStart, dayEnd, _ := timeutil.DayWindow(today, loc)
	spans, err := store.Cards().CardSpansInRange(ctx, dayStart, dayEnd)
	if err != nil {
		return mapStorageError("read cards", err)
	}
	rule := insight.DistractionRule(goalNames)

	for _, block := range active {
		coverage := insight.PlanBlockCoverage(block.StartTs, block.EndTs, now.Unix(), block.CategoryName, spans, rule, loc)
		elapsed := float64(now.Unix()-block.StartTs) / 60
		if coverage.DistractionMinutes < planDistractionMinMinutes || coverage.DistractionMinutes < planDistractionShare*elapsed {
			continue
		}
		replacer := strings.NewReplacer("{title}", block.Title, "{minutes}", fmt.Sprintf("%.0f", coverage.DistractionMinutes))
		sent, err := b.notifyNow(ctx, planDistractionID(block.ID), replacer.Replace(labels.PlanDistractionTitle),
			replacer.Replace(labels.PlanDistractionBody))
		if err != nil || !sent {
			return err
		}
		state.alerted["block:"+strconv.FormatInt(block.ID, 10)] = true
	}

	if checkDay {
		minutes := insight.DistractionMinutes(dayStart.Unix(), now.Unix(), spans, rule, loc)
		if minutes >= float64(dayLimit) {
			replacer := strings.NewReplacer("{minutes}", fmt.Sprintf("%.0f", minutes), "{limit}", strconv.Itoa(dayLimit))
			sent, err := b.notifyNow(ctx, dayDistractionID(today), replacer.Replace(labels.DayDistractionTitle),
				replacer.Replace(labels.DayDistractionBody))
			if err != nil || !sent {
				return err
			}
			state.alerted["day:"+today] = true
		}
	}
	return nil
}

// notifyNow delivers an immediate notification (DeliverAt nil). sent is false
// when the platform has no notification capability, which is not an error.
func (b *Backend) notifyNow(ctx context.Context, id, title, body string) (bool, error) {
	err := b.system.ScheduleNotification(ctx, platform.Notification{ID: id, Title: title, Body: body})
	if errors.Is(err, platform.ErrCapabilityUnavailable) {
		return false, nil
	}
	if err != nil {
		return false, apperr.E(apperr.NativeUnavailable, "deliver plan alert", err)
	}
	return true, nil
}
