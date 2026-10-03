package app

import (
	"context"
	"strings"
	"testing"

	"github.com/Jwz-git/Daygo/internal/storage"
)

func syncPlan(t *testing.T, backend *Backend) {
	t.Helper()
	if err := backend.planReminderSync(context.Background()); err != nil {
		t.Fatalf("planReminderSync: %v", err)
	}
}

func addBlock(t *testing.T, backend *Backend, start, end, title string, remind bool) int64 {
	t.Helper()
	id, err := backend.SavePlanBlock(PlanBlockInputDTO{Day: "2026-09-12", Start: start, End: end, Title: title, Remind: remind})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPlanStartNotificationsFollowThePlan(t *testing.T) {
	backend, system, _ := planBackend(t) // now 09:00
	upcoming := addBlock(t, backend, "09:30", "10:30", "Write API", true)
	silent := addBlock(t, backend, "11:00", "12:00", "Quiet block", false)
	past := addBlock(t, backend, "08:00", "08:30", "Earlier", true)
	syncPlan(t, backend)

	n, ok := system.ScheduledNotification(planStartID(upcoming))
	if !ok {
		t.Fatal("no start notification for the upcoming block")
	}
	if n.DeliverAt == nil || !n.DeliverAt.Equal(localAt(2026, 9, 12, 9, 30)) {
		t.Fatalf("deliverAt = %v, want the block start 09:30", n.DeliverAt)
	}
	if n.Title != "开始：Write API" || n.Body != "计划时间 09:30–10:30" {
		t.Fatalf("copy = %q / %q, want the filled templates", n.Title, n.Body)
	}
	for _, id := range []int64{silent, past} {
		if _, ok := system.ScheduledNotification(planStartID(id)); ok {
			t.Fatalf("block %d scheduled a start notification; want only remind-on upcoming blocks", id)
		}
	}

	calls := system.ScheduleCalls()
	syncPlan(t, backend)
	if system.ScheduleCalls() != calls {
		t.Fatal("an unchanged plan re-issued its schedule")
	}

	if err := backend.SetPlanBlockStatus(upcoming, storage.PlanStatusDone); err != nil {
		t.Fatal(err)
	}
	syncPlan(t, backend)
	if _, ok := system.ScheduledNotification(planStartID(upcoming)); ok {
		t.Fatal("a finished block kept its start notification")
	}
}

func TestPlanDistractionAlertFiresOncePerBlock(t *testing.T) {
	backend, system, _ := planBackend(t)
	id := addBlock(t, backend, "10:00", "11:00", "Write API", false)

	// 08 minutes of distraction after 20 minutes: below the 10-minute floor.
	backend.clock = fixedClock{now: localAt(2026, 9, 12, 10, 20)}
	seedCard(t, backend, "Distraction", localAt(2026, 9, 12, 10, 0), localAt(2026, 9, 12, 10, 8))
	syncPlan(t, backend)
	if _, ok := system.ScheduledNotification(planDistractionID(id)); ok {
		t.Fatal("alert fired below the threshold")
	}

	// 12 more minutes later the block has 20 minutes of distraction in 40.
	backend.clock = fixedClock{now: localAt(2026, 9, 12, 10, 40)}
	seedCard(t, backend, "Distraction", localAt(2026, 9, 12, 10, 25), localAt(2026, 9, 12, 10, 37))
	syncPlan(t, backend)
	n, ok := system.ScheduledNotification(planDistractionID(id))
	if !ok {
		t.Fatal("no distraction alert at 20 of 40 minutes")
	}
	if n.DeliverAt != nil || n.Title != "有点分心了" || !strings.Contains(n.Body, "Write API") || !strings.Contains(n.Body, "20") {
		t.Fatalf("alert = %+v, want an immediate alert naming the block and the minutes", n)
	}

	calls := system.ScheduleCalls()
	backend.clock = fixedClock{now: localAt(2026, 9, 12, 10, 50)}
	syncPlan(t, backend)
	if system.ScheduleCalls() != calls {
		t.Fatal("the alert repeated for the same block")
	}
}

func TestDayDistractionAlertUsesTheGoalLimit(t *testing.T) {
	backend, system, _ := planBackend(t)
	backend.clock = fixedClock{now: localAt(2026, 9, 12, 15, 0)}
	seedCard(t, backend, "Distraction", localAt(2026, 9, 12, 13, 0), localAt(2026, 9, 12, 13, 20))

	syncPlan(t, backend)
	if _, ok := system.ScheduledNotification(dayDistractionID("2026-09-12")); ok {
		t.Fatal("day alert fired without a goal limit")
	}

	if err := backend.SaveDayGoal(DayGoalDTO{Day: "2026-09-12", DistractionLimitMinutes: 15}); err != nil {
		t.Fatal(err)
	}
	syncPlan(t, backend)
	n, ok := system.ScheduledNotification(dayDistractionID("2026-09-12"))
	if !ok || !strings.Contains(n.Body, "20") || !strings.Contains(n.Body, "15") {
		t.Fatalf("day alert = %+v, %v; want one naming 20 of the 15-minute limit", n, ok)
	}
	calls := system.ScheduleCalls()
	syncPlan(t, backend)
	if system.ScheduleCalls() != calls {
		t.Fatal("the day alert repeated")
	}
}

func TestPlanReminderNeedsCopyAndAWriter(t *testing.T) {
	backend, system, _ := planBackend(t)
	addBlock(t, backend, "09:30", "10:30", "Write API", true)
	labels := backend.nativeLabels.get()
	labels.PlanStartTitle = ""
	backend.nativeLabels.set(labels)
	syncPlan(t, backend)
	if system.ScheduleCalls() != 0 {
		t.Fatal("scheduled before the frontend pushed copy")
	}
}
