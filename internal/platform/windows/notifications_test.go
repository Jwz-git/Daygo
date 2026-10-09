package windows

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
)

type notificationFixtureDriver struct {
	state     platform.PermissionState
	err       error
	scheduled []platform.Notification
	cancelled []string
	closed    int
}

func (d *notificationFixtureDriver) permission() (platform.PermissionState, error) {
	return d.state, d.err
}
func (d *notificationFixtureDriver) schedule(n platform.Notification) error {
	d.scheduled = append(d.scheduled, n)
	return d.err
}
func (d *notificationFixtureDriver) cancel(id string) error {
	d.cancelled = append(d.cancelled, id)
	return d.err
}
func (d *notificationFixtureDriver) close() { d.closed++ }

func TestNotificationClientAvailableIsIndependentOfPermission(t *testing.T) {
	d := &notificationFixtureDriver{state: platform.PermissionDenied}
	n := &notificationClient{driver: d}
	if !n.available() {
		t.Fatal("native driver with denied permission must advertise delivery capability")
	}
	state, err := n.permission(context.Background())
	if err != nil || state != platform.PermissionDenied {
		t.Fatalf("permission = %q, %v", state, err)
	}
	n.close()
	n.close()
	if n.available() || d.closed != 1 {
		t.Fatalf("closed client available=%v closes=%d", n.available(), d.closed)
	}
}

func TestNotificationTokenFitsWindowsIdentifierLimit(t *testing.T) {
	if got := notificationToken("journal-reminder"); got != "5b8bcea554833d8b" {
		t.Fatalf("journal token=%q", got)
	}
	if len(notificationToken("plan-day-distraction-2026-10-09")) != 16 {
		t.Fatal("long business ID does not fit native ID")
	}
}

func TestNotificationClientPassesImmediateAndScheduledRequests(t *testing.T) {
	d := &notificationFixtureDriver{}
	n := &notificationClient{driver: d}
	at := time.Date(2026, 10, 9, 18, 0, 0, 0, time.FixedZone("fixture", 8*3600))
	requests := []platform.Notification{
		{ID: "journal-reminder", Title: "日记提醒", Body: "匿名 <文本> & 引号\"", DeliverAt: &at},
		{ID: "plan-distraction-42", Title: "匿名提醒", Body: "匿名正文"},
	}
	for _, request := range requests {
		if err := n.schedule(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(d.scheduled, requests) {
		t.Fatalf("requests = %#v", d.scheduled)
	}
	if err := n.cancel(context.Background(), []string{"journal-reminder", "plan-start-42"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.cancelled, []string{"journal-reminder", "plan-start-42"}) {
		t.Fatalf("cancelled=%v", d.cancelled)
	}
}

func TestNotificationClientRejectsInvalidInputBeforeNativeCall(t *testing.T) {
	valid := platform.Notification{ID: "journal-reminder", Title: "匿名标题", Body: "匿名正文"}
	for name, mutate := range map[string]func(*platform.Notification){
		"empty id":          func(n *platform.Notification) { n.ID = "" },
		"nul id":            func(n *platform.Notification) { n.ID = "bad\x00id" },
		"invalid utf8":      func(n *platform.Notification) { n.Body = string([]byte{0xff}) },
		"xml control":       func(n *platform.Notification) { n.Title = "bad\x01text" },
		"large body":        func(n *platform.Notification) { n.Body = strings.Repeat("a", 16385) },
		"large id":          func(n *platform.Notification) { n.ID = strings.Repeat("a", 4097) },
		"out of range time": func(n *platform.Notification) { at := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC); n.DeliverAt = &at },
	} {
		t.Run(name, func(t *testing.T) {
			d := &notificationFixtureDriver{}
			n := &notificationClient{driver: d}
			request := valid
			mutate(&request)
			if err := n.schedule(context.Background(), request); err == nil {
				t.Fatal("invalid input accepted")
			}
			if len(d.scheduled) != 0 {
				t.Fatal("invalid input reached native adapter")
			}
		})
	}
	d := &notificationFixtureDriver{}
	n := &notificationClient{driver: d}
	if err := n.cancel(context.Background(), []string{"valid", "bad\x00id"}); err == nil {
		t.Fatal("invalid cancel accepted")
	}
	if len(d.cancelled) != 0 {
		t.Fatal("partial cancel occurred before validation")
	}
}

func TestNotificationClientUnavailableCancelledAndNativeFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := &notificationFixtureDriver{}
	n := &notificationClient{driver: d}
	if err := n.schedule(ctx, platform.Notification{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled schedule=%v", err)
	}
	if err := n.cancel(ctx, []string{"journal-reminder"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled cancel=%v", err)
	}
	if len(d.scheduled) != 0 || len(d.cancelled) != 0 {
		t.Fatal("cancelled context reached driver")
	}
	failure := errors.New("anonymous native failure")
	d.err = failure
	if err := n.schedule(context.Background(), platform.Notification{ID: "fixture", Title: "匿名", Body: "匿名"}); !errors.Is(err, failure) {
		t.Fatalf("native failure=%v", err)
	}
	n.close()
	if err := n.schedule(context.Background(), platform.Notification{}); !errors.Is(err, platform.ErrCapabilityUnavailable) {
		t.Fatalf("closed schedule=%v", err)
	}
	if _, err := n.permission(context.Background()); !errors.Is(err, platform.ErrCapabilityUnavailable) {
		t.Fatalf("closed permission=%v", err)
	}
}
