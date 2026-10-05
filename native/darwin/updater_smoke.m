#import <Foundation/Foundation.h>
#import <Sparkle/Sparkle.h>

/* Invoke the production delegate with anonymous callback inputs. No updater
   scheduler, installation process, network request, screen capture, or Daygo
   data is opened. */
static BOOL owner;
static BOOL prepareFails;
static BOOL recording;
static BOOL prepared;
static NSUInteger prepareCalls;
static NSUInteger relaunchCalls;
static NSUInteger resumeCalls;
static SPUUpdater *fixtureUpdater;
static SUAppcastItem *fixtureItem;

void dgGoUpdaterFound(char *version) {}
int32_t dgGoUpdaterCanInstall(void) { return owner ? 1 : 0; }
int32_t dgGoUpdaterPrepare(void) {
    prepareCalls++;
    if (prepareFails) return -1;
    prepared = YES;
    recording = NO;
    return 0;
}
void dgGoUpdaterCancelled(void) {
    if (prepared) {
        prepared = NO;
        recording = YES;
        resumeCalls++;
    }
}
void dgGoUpdaterWillRelaunch(void) { relaunchCalls++; }

@interface DGFixtureUpdateState : NSObject
@property SPUUserUpdateStage stage;
@end
@implementation DGFixtureUpdateState
@end

static void reset(void) {
    owner = YES;
    prepareFails = NO;
    recording = YES;
    prepared = NO;
    prepareCalls = 0;
    relaunchCalls = 0;
    resumeCalls = 0;
}
static void require(BOOL condition, const char *message) {
    if (!condition) {
        fprintf(stderr, "updater fixture failed: %s\n", message);
        exit(1);
    }
}
static void choose(id<SPUUpdaterDelegate> delegate, SPUUserUpdateChoice choice, SPUUserUpdateStage stage) {
    DGFixtureUpdateState *state = [DGFixtureUpdateState new];
    state.stage = stage;
    [delegate updater:fixtureUpdater userDidMakeChoice:choice forUpdate:fixtureItem state:(SPUUserUpdateState *)state];
}

int main(void) {
    @autoreleasepool {
        id<SPUUpdaterDelegate> delegate = [NSClassFromString(@"DGDaygoUpdaterDelegate") new];
        require(delegate != nil, "production delegate must exist");
        // These callbacks do not inspect the updater/item. Use non-null marker
        // objects without constructing Sparkle's network or installation state.
        fixtureUpdater = (SPUUpdater *)[NSObject new];
        fixtureItem = (SUAppcastItem *)[NSObject new];

        for (NSNumber *kind in @[@(SPUUpdateCheckUpdates), @(SPUUpdateCheckUpdatesInBackground), @(SPUUpdateCheckUpdateInformation)]) {
            reset();
            NSError *error = nil;
            require([delegate updater:fixtureUpdater shouldProceedWithUpdate:fixtureItem updateCheck:kind.integerValue error:&error], "owner may discover an update");
            require(error == nil && prepareCalls == 0 && recording && !prepared,
                    "discovery must leave recording and its start gate unchanged");
        }

        reset();
        choose(delegate, SPUUserUpdateChoiceInstall, SPUUserUpdateStageNotDownloaded);
        require(prepareCalls == 0 && recording && !prepared, "choosing download must keep recording");
        choose(delegate, SPUUserUpdateChoiceInstall, SPUUserUpdateStageDownloaded);
        require(prepareCalls == 0 && recording && !prepared, "extracting a download must keep recording");
        choose(delegate, SPUUserUpdateChoiceDismiss, SPUUserUpdateStageInstalling);
        require(prepareCalls == 0 && recording && !prepared, "install on quit must not stop a running app");

        reset();
        choose(delegate, SPUUserUpdateChoiceSkip, SPUUserUpdateStageNotDownloaded);
        choose(delegate, SPUUserUpdateChoiceDismiss, SPUUserUpdateStageDownloaded);
        [delegate updater:fixtureUpdater didAbortWithError:[NSError errorWithDomain:@"invalid.daygo.fixture" code:1 userInfo:nil]];
        require(prepareCalls == 0 && recording && resumeCalls == 0, "cancel before installation must not restart recording");

        require([delegate respondsToSelector:@selector(updaterShouldRelaunchApplication:)], "installation needs a late refusal callback");
        reset();
        require([delegate updaterShouldRelaunchApplication:fixtureUpdater], "finalized owner may install");
        require(prepareCalls == 1 && prepared && !recording && relaunchCalls == 0, "installation must finalize before authorizing termination");
        [delegate updaterWillRelaunchApplication:fixtureUpdater];
        require(relaunchCalls == 1, "Sparkle termination must be authorized");
        [delegate updater:fixtureUpdater didAbortWithError:[NSError errorWithDomain:@"invalid.daygo.fixture" code:2 userInfo:nil]];
        [delegate updater:fixtureUpdater didAbortWithError:[NSError errorWithDomain:@"invalid.daygo.fixture" code:2 userInfo:nil]];
        require(recording && !prepared && resumeCalls == 1, "installation abort must resume once");

        reset();
        prepareFails = YES;
        require(![delegate updaterShouldRelaunchApplication:fixtureUpdater], "failed finalization must refuse installation");
        require(prepareCalls == 1 && recording && !prepared && relaunchCalls == 0, "failure must leave the app usable without authorizing termination");
        prepareFails = NO;
        require([delegate updaterShouldRelaunchApplication:fixtureUpdater], "a later installation attempt may retry finalization");

        reset();
        owner = NO;
        NSError *error = nil;
        require(![delegate updater:fixtureUpdater shouldProceedWithUpdate:fixtureItem updateCheck:SPUUpdateCheckUpdates error:&error] && error != nil, "discovery must retain the ownership refusal");
        require(![delegate updaterShouldRelaunchApplication:fixtureUpdater], "installation must recheck ownership");
        require(prepareCalls == 0 && recording && !prepared && relaunchCalls == 0, "non-owner must not stop recording or terminate");

        printf("Sparkle production delegate: anonymous callback scenarios passed\n");
    }
    return 0;
}
