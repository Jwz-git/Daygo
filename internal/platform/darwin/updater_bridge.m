//go:build darwin && cgo && daygo_updater

#import <Foundation/Foundation.h>
#import <Sparkle/Sparkle.h>
#include "updater_bridge.h"

extern void dgGoUpdaterFound(char *version);
extern int32_t dgGoUpdaterCanInstall(void);
extern int32_t dgGoUpdaterPrepare(void);

/* Localized by the frontend and pushed through Go (docs/05 §5.5.1): no copy of
   our own lives here, and when the push has not happened yet Sparkle falls back
   to its own localized error text instead of an English-only sentence. */
static NSString *installRefusedMessage;

@interface DGDaygoUpdaterDelegate : NSObject <SPUUpdaterDelegate>
@end

@implementation DGDaygoUpdaterDelegate
- (void)updater:(SPUUpdater *)updater didFindValidUpdate:(SUAppcastItem *)item {
    NSString *version = item.displayVersionString ?: item.versionString;
    dgGoUpdaterFound((char *)version.UTF8String);
}
- (BOOL)updater:(SPUUpdater *)updater
        shouldProceedWithUpdate:(SUAppcastItem *)item
        updateCheck:(SPUUpdateCheck)updateCheck
        error:(NSError * __autoreleasing *)error {
    if (dgGoUpdaterCanInstall() != 0) return YES;
    if (error != NULL) {
        NSMutableDictionary *info = [NSMutableDictionary dictionary];
        if (installRefusedMessage.length > 0) {
            info[NSLocalizedDescriptionKey] = installRefusedMessage;
        }
        *error = [NSError errorWithDomain:@"io.github.jwz-git.Daygo.updater"
                                     code:1
                                 userInfo:info];
    }
    return NO;
}
- (void)updater:(SPUUpdater *)updater willInstallUpdate:(SUAppcastItem *)item {
    (void)dgGoUpdaterPrepare();
}
@end

static SPUStandardUpdaterController *controller;
static DGDaygoUpdaterDelegate *delegate;

static void on_main_sync(dispatch_block_t block) {
    if ([NSThread isMainThread]) block(); else dispatch_sync(dispatch_get_main_queue(), block);
}

int32_t dg_updater_start(void) {
    __block int32_t status = 0;
    on_main_sync(^{
        delegate = [DGDaygoUpdaterDelegate new];
        controller = [[SPUStandardUpdaterController alloc] initWithStartingUpdater:NO updaterDelegate:delegate userDriverDelegate:nil];
        if (controller == nil) status = -1;
    });
    return status;
}
void dg_updater_activate(void) { on_main_sync(^{ [controller startUpdater]; }); }
void dg_updater_stop(void) { on_main_sync(^{ controller = nil; delegate = nil; }); }
void dg_updater_check(int32_t interactive) {
    on_main_sync(^{
        if (interactive) [controller checkForUpdates:nil];
        else [controller.updater checkForUpdatesInBackground];
    });
}
int32_t dg_updater_automatic(void) { __block BOOL value; on_main_sync(^{ value = controller.updater.automaticallyChecksForUpdates; }); return value ? 1 : 0; }
void dg_updater_set_automatic(int32_t enabled) { on_main_sync(^{ controller.updater.automaticallyChecksForUpdates = enabled != 0; }); }
int32_t dg_updater_checking(void) { __block BOOL value; on_main_sync(^{ value = controller.updater.sessionInProgress; }); return value ? 1 : 0; }
int64_t dg_updater_last_checked(void) { __block NSDate *date; on_main_sync(^{ date = controller.updater.lastUpdateCheckDate; }); return date == nil ? 0 : (int64_t)date.timeIntervalSince1970; }

void dg_updater_set_install_refused_message(const char *message) {
    if (message == NULL) return;
    NSString *copy = [NSString stringWithUTF8String:message];
    /* An empty or undecodable push keeps the previous copy rather than blanking
       the dialog. The assignment runs on the main thread, which is also where
       the delegate reads it. */
    if (copy.length == 0) return;
    on_main_sync(^{ installRefusedMessage = copy; });
}
