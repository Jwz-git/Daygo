import Foundation
// AppKit's types (NSStatusItem, NSMenu) are main-actor isolated. This file has
// to state which of its own code runs on the main actor, because the C entry
// points below are called from Go, which promises no thread.
@preconcurrency import AppKit

/// The C struct flattened into Swift values.
///
/// Every field is copied out of the pointer while still on the caller's thread,
/// which is what keeps the non-Sendable pointer from crossing an isolation
/// boundary later.
private struct StatusSnapshot: Sendable, Equatable {
    var visible: Bool
    var icon: UInt32
    var pauseDurationsEnabled: Bool
    var primaryActionEnabled: Bool
    var title: String
    var tooltip: String
    var openLabel: String
    var recordingsLabel: String
    var quitLabel: String
    var pauseMenuLabel: String
    var pause15Label: String
    var pause30Label: String
    var pause60Label: String
    var pauseIndefiniteLabel: String
    var primaryActionLabel: String
}

/// The callback and its user data, which exist only to be handed back to the
/// action methods later.
///
/// A raw pointer cannot be proven safe to share across threads, so this wrapper
/// takes responsibility for it explicitly. The lifetime is the caller's:
/// dg_status_item.h requires the pointer to stay valid until dg_status_item_stop.
private struct StatusHandles: @unchecked Sendable {
    var callback: dg_status_item_action_callback_v1?
    var data: UnsafeMutableRawPointer?
}

private func snapshot(from state: UnsafePointer<dg_status_item_state_v1>) -> StatusSnapshot {
    StatusSnapshot(
        visible: state.pointee.visible != 0,
        icon: state.pointee.icon,
        pauseDurationsEnabled: state.pointee.pause_durations_enabled != 0,
        primaryActionEnabled: state.pointee.primary_action_enabled != 0,
        title: String(cString: state.pointee.title),
        tooltip: String(cString: state.pointee.tooltip),
        openLabel: String(cString: state.pointee.open_label),
        recordingsLabel: String(cString: state.pointee.recordings_label),
        quitLabel: String(cString: state.pointee.quit_label),
        pauseMenuLabel: String(cString: state.pointee.pause_menu_label),
        pause15Label: String(cString: state.pointee.pause_15_label),
        pause30Label: String(cString: state.pointee.pause_30_label),
        pause60Label: String(cString: state.pointee.pause_60_label),
        pauseIndefiniteLabel: String(cString: state.pointee.pause_indefinite_label),
        primaryActionLabel: String(cString: state.pointee.primary_action_label)
    )
}

@MainActor
private final class StatusController: NSObject {
    let item: NSStatusItem
    let menu = NSMenu()
    let handles: StatusHandles
    private var currentSnapshot: StatusSnapshot?

    init(snapshot: StatusSnapshot, handles: StatusHandles) {
        item = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        self.handles = handles
        super.init()
        // AppKit otherwise auto-enables items whose target responds to the
        // action, which would override primaryActionEnabled (disabled while
        // starting) and re-enable the disabled pause-duration header.
        menu.autoenablesItems = false
        item.menu = menu
        update(snapshot: snapshot)
    }

    func update(snapshot: StatusSnapshot) {
        // Successful captures often publish the same surface. Preserve the menu
        // and its tracking selection when nothing visible has changed.
        guard currentSnapshot != snapshot else { return }
        currentSnapshot = snapshot
        let symbol: String
        switch snapshot.icon {
        case UInt32(DG_STATUS_ICON_ACTIVE): symbol = "record.circle.fill"
        case UInt32(DG_STATUS_ICON_BUSY): symbol = "hourglass"
        case UInt32(DG_STATUS_ICON_PAUSED): symbol = "pause.circle"
        case UInt32(DG_STATUS_ICON_WARNING): symbol = "exclamationmark.circle"
        default: symbol = "record.circle"
        }
        item.button?.image = NSImage(systemSymbolName: symbol, accessibilityDescription: snapshot.title.isEmpty ? snapshot.tooltip : snapshot.title)
        item.button?.image?.isTemplate = true
        // squareLength sizes the button for an icon alone; also setting a title
        // crams text into that square and renders as clipped, garbled glyphs.
        // Recording state moves to the tooltip and the menu items instead.
        item.button?.title = ""
        item.button?.toolTip = snapshot.title.isEmpty ? snapshot.tooltip : "\(snapshot.tooltip) — \(snapshot.title)"
        // The pause region changes shape between states (inline durations
        // while capturing, a single action otherwise), so the whole menu is
        // rebuilt on each update rather than mutating items in place.
        rebuildMenu(snapshot: snapshot)
        item.isVisible = snapshot.visible
    }

    private func rebuildMenu(snapshot: StatusSnapshot) {
        menu.removeAllItems()

        let state = NSMenuItem(title: snapshot.title, action: nil, keyEquivalent: "")
        state.isEnabled = false
        menu.addItem(state)
        menu.addItem(.separator())

        let open = NSMenuItem(title: snapshot.openLabel, action: #selector(openAction(_:)), keyEquivalent: "")
        open.target = self
        menu.addItem(open)

        if snapshot.pauseDurationsEnabled {
            // Show the durations inline under a disabled header (like the legacy
            // picker) rather than hiding them one level down in a submenu, so
            // 15/30/60/∞ are visible the moment the menu opens while capturing.
            let header = NSMenuItem(title: snapshot.pauseMenuLabel, action: nil, keyEquivalent: "")
            header.isEnabled = false
            menu.addItem(header)
            let durations: [(String, Int)] = [
                (snapshot.pause15Label, Int(DG_STATUS_ITEM_PAUSE_15)),
                (snapshot.pause30Label, Int(DG_STATUS_ITEM_PAUSE_30)),
                (snapshot.pause60Label, Int(DG_STATUS_ITEM_PAUSE_60)),
                (snapshot.pauseIndefiniteLabel, Int(DG_STATUS_ITEM_PAUSE_INDEFINITE)),
            ]
            for (label, action) in durations {
                let entry = NSMenuItem(title: label, action: #selector(durationAction(_:)), keyEquivalent: "")
                entry.target = self
                entry.tag = action
                entry.indentationLevel = 1
                menu.addItem(entry)
            }
        } else {
            let primary = NSMenuItem(title: snapshot.primaryActionLabel, action: #selector(pauseAction(_:)), keyEquivalent: "")
            primary.target = self
            primary.isEnabled = snapshot.primaryActionEnabled
            menu.addItem(primary)
        }

        let recordings = NSMenuItem(title: snapshot.recordingsLabel, action: #selector(recordingsAction(_:)), keyEquivalent: "")
        menu.addItem(.separator())
        recordings.target = self
        menu.addItem(recordings)

        menu.addItem(.separator())

        let quit = NSMenuItem(title: snapshot.quitLabel, action: #selector(quitAction(_:)), keyEquivalent: "")
        quit.target = self
        menu.addItem(quit)
    }

    @objc func openAction(_ sender: Any?) { handles.callback?(UInt32(DG_STATUS_ITEM_OPEN), handles.data) }
    @objc func pauseAction(_ sender: Any?) { handles.callback?(UInt32(DG_STATUS_ITEM_TOGGLE_PAUSE), handles.data) }
    @objc func durationAction(_ sender: Any?) {
        guard let item = sender as? NSMenuItem else { return }
        handles.callback?(UInt32(item.tag), handles.data)
    }
    @objc func recordingsAction(_ sender: Any?) { handles.callback?(UInt32(DG_STATUS_ITEM_OPEN_RECORDINGS), handles.data) }
    @objc func quitAction(_ sender: Any?) { handles.callback?(UInt32(DG_STATUS_ITEM_QUIT), handles.data) }
}

private let statusLock = NSLock()
private nonisolated(unsafe) var statusController: StatusController?

@MainActor
private func applyStatus(_ snapshot: StatusSnapshot, _ handles: StatusHandles) {
    statusLock.lock()
    if let existing = statusController {
        existing.update(snapshot: snapshot)
    } else {
        statusController = StatusController(snapshot: snapshot, handles: handles)
    }
    statusLock.unlock()
}

@MainActor
private func clearStatus() {
    statusLock.lock()
    if let controller = statusController { NSStatusBar.system.removeStatusItem(controller.item) }
    statusController = nil
    statusLock.unlock()
}

/// Runs body on the main actor from any thread.
///
/// Off the main thread the hop is async, not sync, and that asymmetry is
/// load-bearing: a status repaint is fired from the recorder's event callback,
/// which runs on the goroutine draining Recorder.run. During shutdown the main
/// thread is inside Recorder.Stop blocked on <-done waiting for that same
/// goroutine to exit. A sync hop here would block the goroutine on the main
/// thread while the main thread blocks on the goroutine — a deadlock that hangs
/// the app on menu-bar Quit. Async lets the repaint queue and return, so the
/// goroutine finishes and Stop unblocks. Repaints carry no return value, so
/// nothing depends on the hop completing before this returns.
///
/// The closure is @Sendable because it crosses an isolation boundary; the
/// callers below therefore hand it only Sendable values.
private func runOnMainActor(_ body: @escaping @Sendable @MainActor () -> Void) {
    if Thread.isMainThread {
        // Already on the main thread, so the main actor's executor is this
        // thread and no hop is needed. Kept synchronous because startup builds
        // the item on the main thread before first paint, and a sync dispatch
        // onto the queue we are already on would itself deadlock.
        MainActor.assumeIsolated { body() }
    } else {
        DispatchQueue.main.async { MainActor.assumeIsolated { body() } }
    }
}

@_cdecl("dg_status_item_set")
func dg_status_item_set(_ requested: UInt32, _ state: UnsafePointer<dg_status_item_state_v1>?, _ callback: dg_status_item_action_callback_v1?, _ data: UnsafeMutableRawPointer?) -> Int32 {
    guard requested == DG_STATUS_ITEM_ABI_MAJOR, let state, state.pointee.icon <= UInt32(DG_STATUS_ICON_WARNING) else { return -1 }
    let snapshot = snapshot(from: state)
    let handles = StatusHandles(callback: callback, data: data)
    runOnMainActor { applyStatus(snapshot, handles) }
    return 0
}

@_cdecl("dg_status_item_stop")
func dg_status_item_stop() {
    runOnMainActor { clearStatus() }
}

@_cdecl("dg_status_item_is_available")
func dg_status_item_is_available() -> Int32 {
    let query: @Sendable @MainActor () -> Int32 = {
        guard let controller = statusController, controller.item.isVisible, controller.item.button != nil else { return 0 }
        return 1
    }
    if Thread.isMainThread { return MainActor.assumeIsolated { query() } }
    return DispatchQueue.main.sync { MainActor.assumeIsolated { query() } }
}

#if DAYGO_NATIVE_TESTS
@MainActor func statusItemMenuForSmoke() -> NSMenu? { statusController?.menu }
#endif
