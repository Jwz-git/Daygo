import AppKit
import Foundation
import CoreGraphics

private final class SystemState: @unchecked Sendable {
    let lock = NSLock()
    var callback: dg_system_event_callback_v1?
    var callbackData: UnsafeMutableRawPointer?
    var observers: [NSObjectProtocol] = []
    var lastDisplayEventNs: Int64 = 0
    var lastLocked: Bool?
    var lockTimer: DispatchSourceTimer?
}
private let state = SystemState()

private func emit(_ kind: UInt32) {
    let now = Int64(Date().timeIntervalSince1970 * 1_000_000_000)
    state.lock.lock()
    if kind == UInt32(DG_SYSTEM_DISPLAYS_CHANGED) && now - state.lastDisplayEventNs < 500_000_000 { state.lock.unlock(); return }
    if kind == UInt32(DG_SYSTEM_DISPLAYS_CHANGED) { state.lastDisplayEventNs = now }
    let cb = state.callback; let data = state.callbackData
    state.lock.unlock()
    cb?(kind, now, data)
}

private func currentScreenLockState() -> Bool? {
    guard let session = CGSessionCopyCurrentDictionary() as NSDictionary? else { return nil }
    return session["CGSSessionScreenIsLocked"] as? Bool
}



@_cdecl("dg_system_start")
func dg_system_start(_ requested: UInt32, _ cb: dg_system_event_callback_v1?, _ data: UnsafeMutableRawPointer?) -> Int32 {
    guard requested == DG_SYSTEM_ABI_MAJOR, let cb else { return -1 }
    dg_system_stop()
    state.lock.lock(); state.callback = cb; state.callbackData = data; state.lock.unlock()
    let workspace = NSWorkspace.shared.notificationCenter
    let distributed = DistributedNotificationCenter.default()
    func add(_ name: Notification.Name, _ center: NotificationCenter) {
        let token = center.addObserver(forName: name, object: nil, queue: nil) { _ in emit(kind(for: name)) }
        state.lock.lock(); state.observers.append(token); state.lock.unlock()
    }
    add(NSWorkspace.willSleepNotification, workspace)
    add(NSWorkspace.didWakeNotification, workspace)
    add(NSApplication.didChangeScreenParametersNotification, NotificationCenter.default)
    for (name, value) in [("com.apple.screenIsLocked", UInt32(DG_SYSTEM_SCREEN_LOCKED)), ("com.apple.screenIsUnlocked", UInt32(DG_SYSTEM_SCREEN_UNLOCKED)), ("com.apple.screensaver.didstart", UInt32(DG_SYSTEM_SCREENSAVER_START)), ("com.apple.screensaver.didstop", UInt32(DG_SYSTEM_SCREENSAVER_STOP))] {
        let token = distributed.addObserver(forName: Notification.Name(name), object: nil, queue: nil) { _ in emit(value) }
        state.lock.lock(); state.observers.append(token); state.lock.unlock()
    }
    let timer = DispatchSource.makeTimerSource(queue: DispatchQueue.global(qos: .utility))
    timer.schedule(deadline: .now(), repeating: .seconds(1))
    timer.setEventHandler {
        guard let locked = currentScreenLockState() else { return }
        state.lock.lock(); let previous = state.lastLocked; state.lastLocked = locked; state.lock.unlock()
        if previous != nil && previous != locked { emit(locked ? UInt32(DG_SYSTEM_SCREEN_LOCKED) : UInt32(DG_SYSTEM_SCREEN_UNLOCKED)) }
    }
    state.lock.lock(); state.lockTimer = timer; state.lock.unlock()
    timer.resume()
    return 0
}

private func kind(for name: Notification.Name) -> UInt32 {
    if name == NSWorkspace.willSleepNotification { return UInt32(DG_SYSTEM_SLEEP) }
    if name == NSWorkspace.didWakeNotification { return UInt32(DG_SYSTEM_WAKE) }
    return UInt32(DG_SYSTEM_DISPLAYS_CHANGED)
}

private func activationRunOnMain(_ body: @Sendable @escaping @MainActor () -> Void) {
    if Thread.isMainThread {
        MainActor.assumeIsolated { body() }
    } else {
        DispatchQueue.main.async { MainActor.assumeIsolated { body() } }
    }
}

@_cdecl("dg_activation_policy_set")
func dg_activation_policy_set(_ policy: UInt32) -> Int32 {
    let target: NSApplication.ActivationPolicy
    switch policy {
    case UInt32(DG_ACTIVATION_REGULAR): target = .regular
    case UInt32(DG_ACTIVATION_ACCESSORY): target = .accessory
    case UInt32(DG_ACTIVATION_PROHIBITED): target = .prohibited
    default: return -1
    }
    activationRunOnMain { NSApp.setActivationPolicy(target) }
    return 0
}

@_cdecl("dg_screen_recording_permission_query")
func dg_screen_recording_permission_query() -> Int32 {
    // Preflight reflects only kTCCServiceScreenCapture and never prompts. A
    // false result may be a fresh not-determined state or a prior denial;
    // macOS gives no way to tell them apart here, so both map to
    // NOT_DETERMINED and the request path handles either.
    return CGPreflightScreenCaptureAccess()
        ? Int32(DG_PERMISSION_GRANTED)
        : Int32(DG_PERMISSION_NOT_DETERMINED)
}

@_cdecl("dg_screen_recording_permission_request")
func dg_screen_recording_permission_request() -> Int32 {
    // CGRequestScreenCaptureAccess blocks until the user answers the first-use
    // prompt and is a no-op once the choice is recorded. Run it off-thread and
    // return at once: a granted permission only applies after relaunch, so
    // there is nothing to observe by waiting.
    DispatchQueue.global(qos: .userInitiated).async {
        _ = CGRequestScreenCaptureAccess()
    }
    return 0
}

@_cdecl("dg_open_system_settings")
func dg_open_system_settings(_ pane: UInt32) -> Int32 {
    let urlString: String
    switch pane {
    case UInt32(DG_SETTINGS_PANE_SCREEN_RECORDING):
        urlString = "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture"
    case UInt32(DG_SETTINGS_PANE_NOTIFICATIONS):
        urlString = "x-apple.systempreferences:com.apple.preference.notifications"
    case UInt32(DG_SETTINGS_PANE_LOGIN_ITEMS):
        urlString = "x-apple.systempreferences:com.apple.LoginItems-Settings.extension"
    default:
        return -1
    }
    guard let url = URL(string: urlString) else { return -1 }
    activationRunOnMain { NSWorkspace.shared.open(url) }
    return 0
}

@_cdecl("dg_system_stop")
func dg_system_stop() {
    let workspace = NSWorkspace.shared.notificationCenter
    let distributed = DistributedNotificationCenter.default()
    state.lock.lock(); let old = state.observers; let timer = state.lockTimer; state.observers.removeAll(); state.lockTimer = nil; state.lastLocked = nil; state.callback = nil; state.callbackData = nil; state.lock.unlock()
    timer?.cancel()
    for token in old { workspace.removeObserver(token); distributed.removeObserver(token); NotificationCenter.default.removeObserver(token) }
}
