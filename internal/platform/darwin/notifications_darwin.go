//go:build darwin && cgo

package darwin

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework UserNotifications
#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>

// Manual reference counting: cgo merges #cgo CFLAGS across the package, so
// turning on ARC here would silently change how the adapter's other
// Objective-C compiles. Every object this file creates is released below.

// Result codes shared with notifications_darwin.go.
enum {
    DG_NOTIFY_OK = 0,
    DG_NOTIFY_UNAVAILABLE = 1, // no app bundle: UNUserNotificationCenter would throw
    DG_NOTIFY_DENIED = 2,      // not authorized (denied, or the one prompt went unanswered)
    DG_NOTIFY_FAILED = 3,
};

// Banners stay visible while Daygo is frontmost; without a delegate the system
// drops a notification whose app is active. Taps only dismiss for now
// (EventNotificationClick is reserved, docs/decisions/notifications-journal-reminder.md).
@interface DGNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation DGNotificationDelegate
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
    completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList |
                      UNNotificationPresentationOptionSound);
}
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
didReceiveNotificationResponse:(UNNotificationResponse *)response
         withCompletionHandler:(void (^)(void))completionHandler {
    completionHandler();
}
@end

static DGNotificationDelegate *dg_notification_delegate;
static BOOL dg_authorization_requested = NO;

// UNUserNotificationCenter raises an exception in a process without a bundle
// identifier (a bare binary, go test), so every entry point checks first.
static BOOL dg_has_bundle(void) {
    return [[NSBundle mainBundle] bundleIdentifier] != nil;
}

static int dg_notifications_available(void) { return dg_has_bundle() ? 1 : 0; }

static UNUserNotificationCenter *dg_center(void) {
    UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        dg_notification_delegate = [DGNotificationDelegate new];
        center.delegate = dg_notification_delegate;
    });
    return center;
}

static UNAuthorizationStatus dg_authorization_status(UNUserNotificationCenter *center) {
    __block UNAuthorizationStatus status = UNAuthorizationStatusNotDetermined;
    dispatch_semaphore_t done = dispatch_semaphore_create(0);
    [center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
        status = settings.authorizationStatus;
        dispatch_semaphore_signal(done);
    }];
    dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC));
    dispatch_release(done);
    return status;
}

// 0 not determined, 1 granted, 2 denied, -1 unavailable.
static int dg_notification_permission(void) {
    if (!dg_has_bundle()) return -1;
    switch (dg_authorization_status(dg_center())) {
        case UNAuthorizationStatusAuthorized:
        case UNAuthorizationStatusProvisional:
            return 1;
        case UNAuthorizationStatusDenied:
            return 2;
        default:
            return 0;
    }
}

// Requests authorization at most once per process: the first delivery shows the
// system prompt; afterwards a refusal is respected until the user changes it in
// System Settings, which the next reconcile tick then picks up.
static int dg_ensure_authorized(UNUserNotificationCenter *center) {
    UNAuthorizationStatus status = dg_authorization_status(center);
    if (status == UNAuthorizationStatusAuthorized || status == UNAuthorizationStatusProvisional) return DG_NOTIFY_OK;
    if (status == UNAuthorizationStatusDenied || dg_authorization_requested) return DG_NOTIFY_DENIED;
    dg_authorization_requested = YES;
    __block BOOL granted = NO;
    dispatch_semaphore_t done = dispatch_semaphore_create(0);
    [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                          completionHandler:^(BOOL ok, NSError *error) {
        granted = ok;
        dispatch_semaphore_signal(done);
    }];
    long timedOut = dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 120 * NSEC_PER_SEC));
    dispatch_release(done);
    if (timedOut != 0) return DG_NOTIFY_DENIED;
    return granted ? DG_NOTIFY_OK : DG_NOTIFY_DENIED;
}

// deliverAt is Unix seconds; 0 (or a moment already past) delivers now.
static int dg_schedule_notification(const char *identifier, const char *title, const char *body, double deliverAt) {
    @autoreleasepool {
        if (!dg_has_bundle()) return DG_NOTIFY_UNAVAILABLE;
        UNUserNotificationCenter *center = dg_center();
        int auth = dg_ensure_authorized(center);
        if (auth != DG_NOTIFY_OK) return auth;

        UNMutableNotificationContent *content = [[[UNMutableNotificationContent alloc] init] autorelease];
        content.title = [NSString stringWithUTF8String:title] ?: @"";
        content.body = [NSString stringWithUTF8String:body] ?: @"";
        content.sound = [UNNotificationSound defaultSound];

        UNNotificationTrigger *trigger = nil;
        NSDate *date = [NSDate dateWithTimeIntervalSince1970:deliverAt];
        if (deliverAt > 0 && [date timeIntervalSinceNow] > 1) {
            // A calendar date (not an interval) so sleep or a clock change does
            // not shift the delivery.
            NSDateComponents *components = [[NSCalendar currentCalendar]
                components:(NSCalendarUnitYear | NSCalendarUnitMonth | NSCalendarUnitDay |
                            NSCalendarUnitHour | NSCalendarUnitMinute | NSCalendarUnitSecond)
                  fromDate:date];
            trigger = [UNCalendarNotificationTrigger triggerWithDateMatchingComponents:components repeats:NO];
        }
        UNNotificationRequest *request = [UNNotificationRequest
            requestWithIdentifier:[NSString stringWithUTF8String:identifier]
                          content:content
                          trigger:trigger];

        __block BOOL failed = NO;
        dispatch_semaphore_t done = dispatch_semaphore_create(0);
        [center addNotificationRequest:request withCompletionHandler:^(NSError *error) {
            failed = error != nil;
            dispatch_semaphore_signal(done);
        }];
        long timedOut = dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));
        dispatch_release(done);
        if (timedOut != 0) return DG_NOTIFY_FAILED;
        return failed ? DG_NOTIFY_FAILED : DG_NOTIFY_OK;
    }
}

static int dg_cancel_notifications(const char **identifiers, int count) {
    @autoreleasepool {
        if (!dg_has_bundle()) return DG_NOTIFY_UNAVAILABLE;
        NSMutableArray<NSString *> *ids = [NSMutableArray arrayWithCapacity:count];
        for (int i = 0; i < count; i++) {
            NSString *value = [NSString stringWithUTF8String:identifiers[i]];
            if (value != nil) [ids addObject:value];
        }
        [dg_center() removePendingNotificationRequestsWithIdentifiers:ids];
        return DG_NOTIFY_OK;
    }
}
*/
import "C"

import (
	"context"
	"errors"
	"unsafe"

	"github.com/Jwz-git/Daygo/internal/platform"
)

var errNotificationsDenied = errors.New("darwin: notifications are not authorized")

func notificationsAvailable() bool { return C.dg_notifications_available() != 0 }

func scheduleNotification(_ context.Context, n platform.Notification) error {
	id := C.CString(n.ID)
	title := C.CString(n.Title)
	body := C.CString(n.Body)
	defer C.free(unsafe.Pointer(id))
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(body))
	var at C.double
	if n.DeliverAt != nil {
		at = C.double(n.DeliverAt.Unix())
	}
	return notifyResult(C.dg_schedule_notification(id, title, body, at))
}

func cancelNotifications(_ context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	cIDs := make([]*C.char, len(ids))
	for i, id := range ids {
		cIDs[i] = C.CString(id)
	}
	defer func() {
		for _, p := range cIDs {
			C.free(unsafe.Pointer(p))
		}
	}()
	return notifyResult(C.dg_cancel_notifications((**C.char)(unsafe.Pointer(&cIDs[0])), C.int(len(cIDs))))
}

func notificationPermission(context.Context) (platform.PermissionState, error) {
	switch C.dg_notification_permission() {
	case 1:
		return platform.PermissionGranted, nil
	case 2:
		return platform.PermissionDenied, nil
	case -1:
		return platform.PermissionNotDetermined, platform.ErrCapabilityUnavailable
	default:
		return platform.PermissionNotDetermined, nil
	}
}

func notifyResult(code C.int) error {
	switch code {
	case C.DG_NOTIFY_OK:
		return nil
	case C.DG_NOTIFY_UNAVAILABLE:
		return platform.ErrCapabilityUnavailable
	case C.DG_NOTIFY_DENIED:
		return errNotificationsDenied
	default:
		return errors.New("darwin: notification request failed")
	}
}
