import AppKit
import Foundation
import CoreGraphics
import ServiceManagement

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
    add(NSApplication.didBecomeActiveNotification, NotificationCenter.default)
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
    if name == NSApplication.didBecomeActiveNotification { return UInt32(DG_SYSTEM_APPLICATION_ACTIVATED) }
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

@_cdecl("dg_launch_at_login_query")
func dg_launch_at_login_query() -> Int32 {
    // SMAppService.mainApp registers the app itself as a login item with no
    // helper or plist. status never prompts, so it is safe to read on load.
    guard #available(macOS 13, *) else { return Int32(DG_LAUNCH_AT_LOGIN_UNSUPPORTED) }
    switch SMAppService.mainApp.status {
    case .enabled:
        return Int32(DG_LAUNCH_AT_LOGIN_ENABLED)
    case .requiresApproval:
        return Int32(DG_LAUNCH_AT_LOGIN_REQUIRES_APPROVAL)
    case .notFound:
        return Int32(DG_LAUNCH_AT_LOGIN_NOT_FOUND)
    case .notRegistered:
        return Int32(DG_LAUNCH_AT_LOGIN_NOT_REGISTERED)
    @unknown default:
        return Int32(DG_LAUNCH_AT_LOGIN_NOT_REGISTERED)
    }
}

@_cdecl("dg_launch_at_login_set")
func dg_launch_at_login_set(_ enabled: UInt32) -> Int32 {
    guard #available(macOS 13, *) else { return -1 }
    do {
        if enabled != 0 {
            try SMAppService.mainApp.register()
        } else {
            try SMAppService.mainApp.unregister()
        }
        return 0
    } catch {
        // Unsigned/ad-hoc bundles fail here with "Operation not permitted"; the
        // Go layer logs it without failing the settings write.
        return -1
    }
}

@_cdecl("dg_relaunch")
func dg_relaunch() -> Int32 {
    // Schedule a fresh instance to start once THIS process has exited. macOS
    // caches the screen-recording (TCC) decision at launch, so only a real
    // relaunch picks up a newly granted permission; and the new instance must
    // wait for the old one to release the write/capture locks before it can
    // become the owner. A detached /bin/sh polls the parent PID, then `open -n`
    // starts a new instance. When we exit the helper is reparented to launchd,
    // so it outlives our termination. The bundle path is passed as $0 rather
    // than interpolated into the script, so a path with spaces is handled by
    // the shell without quoting games.
    let bundlePath = Bundle.main.bundlePath
    guard !bundlePath.isEmpty else { return -1 }
    let pid = ProcessInfo.processInfo.processIdentifier
    let script = "while kill -0 \(pid) 2>/dev/null; do sleep 0.2; done; sleep 0.5; exec open -n \"$0\""
    let task = Process()
    task.executableURL = URL(fileURLWithPath: "/bin/sh")
    task.arguments = ["-c", script, bundlePath]
    do {
        try task.run()
        return 0
    } catch {
        return -1
    }
}

@_cdecl("dg_system_stop")
func dg_system_stop() {
    let workspace = NSWorkspace.shared.notificationCenter
    let distributed = DistributedNotificationCenter.default()
    state.lock.lock(); let old = state.observers; let timer = state.lockTimer; state.observers.removeAll(); state.lockTimer = nil; state.lastLocked = nil; state.callback = nil; state.callbackData = nil; state.lock.unlock()
    timer?.cancel()
    for token in old { workspace.removeObserver(token); distributed.removeObserver(token); NotificationCenter.default.removeObserver(token) }
}
