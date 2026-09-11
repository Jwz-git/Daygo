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
private struct StatusSnapshot: Sendable {
    var visible: Bool
    var pauseEnabled: Bool
    var title: String
    var tooltip: String
    var openLabel: String
    var pauseLabel: String
    var quitLabel: String
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
        pauseEnabled: state.pointee.pause_enabled != 0,
        title: String(cString: state.pointee.title),
        tooltip: String(cString: state.pointee.tooltip),
        openLabel: String(cString: state.pointee.open_label),
        pauseLabel: String(cString: state.pointee.pause_label),
        quitLabel: String(cString: state.pointee.quit_label)
    )
}

@MainActor
private final class StatusController: NSObject {
    let item: NSStatusItem
    let menu = NSMenu()
    let pauseItem = NSMenuItem()
    let handles: StatusHandles

    init(snapshot: StatusSnapshot, handles: StatusHandles) {
        item = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        self.handles = handles
        super.init()
        let open = NSMenuItem(title: snapshot.openLabel, action: #selector(openAction(_:)), keyEquivalent: "")
        open.target = self
        pauseItem.action = #selector(pauseAction(_:)); pauseItem.target = self
        let quit = NSMenuItem(title: snapshot.quitLabel, action: #selector(quitAction(_:)), keyEquivalent: "")
        quit.target = self
        menu.addItem(open); menu.addItem(pauseItem); menu.addItem(.separator()); menu.addItem(quit)
        item.menu = menu
        update(snapshot: snapshot)
    }

    func update(snapshot: StatusSnapshot) {
        item.button?.image = NSImage(systemSymbolName: "clock", accessibilityDescription: snapshot.tooltip)
        item.button?.image?.isTemplate = true
        item.button?.toolTip = snapshot.tooltip
        item.button?.title = snapshot.title
        item.button?.target = self; item.button?.action = #selector(openAction(_:))
        pauseItem.title = snapshot.pauseLabel
        pauseItem.isEnabled = snapshot.pauseEnabled
        item.isVisible = snapshot.visible
    }

    @objc func openAction(_ sender: Any?) { handles.callback?(UInt32(DG_STATUS_ITEM_OPEN), handles.data) }
    @objc func pauseAction(_ sender: Any?) { handles.callback?(UInt32(DG_STATUS_ITEM_TOGGLE_PAUSE), handles.data) }
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
    statusController?.item.isVisible = false
    statusController = nil
    statusLock.unlock()
}

/// Runs body synchronously on the main actor from any thread.
///
/// The closure is @Sendable because it crosses an isolation boundary; the
/// callers below therefore hand it only Sendable values.
private func runOnMainActor(_ body: @Sendable @MainActor () -> Void) {
    if Thread.isMainThread {
        // Already on the main thread, so the main actor's executor is this
        // thread and no hop is needed.
        MainActor.assumeIsolated { body() }
    } else {
        DispatchQueue.main.sync { MainActor.assumeIsolated { body() } }
    }
}

@_cdecl("dg_status_item_set")
func dg_status_item_set(_ requested: UInt32, _ state: UnsafePointer<dg_status_item_state_v1>?, _ callback: dg_status_item_action_callback_v1?, _ data: UnsafeMutableRawPointer?) -> Int32 {
    guard requested == DG_STATUS_ITEM_ABI_MAJOR, let state else { return -1 }
    let snapshot = snapshot(from: state)
    let handles = StatusHandles(callback: callback, data: data)
    runOnMainActor { applyStatus(snapshot, handles) }
    return 0
}

@_cdecl("dg_status_item_stop")
func dg_status_item_stop() {
    runOnMainActor { clearStatus() }
}
