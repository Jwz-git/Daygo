import AppKit
import Foundation

private final class ObservedEvents: @unchecked Sendable {
    let lock = NSLock()
    var shutdowns = 0
    func record(_ kind: UInt32) {
        lock.lock(); defer { lock.unlock() }
        if kind == UInt32(DG_SYSTEM_SHUTDOWN) { shutdowns += 1 }
    }
    func count() -> Int {
        lock.lock(); defer { lock.unlock() }
        return shutdowns
    }
}
private let observed = ObservedEvents()
private func onEvent(_ kind: UInt32, _ at: Int64, _ data: UnsafeMutableRawPointer?) {
    observed.record(kind)
}
private func onAction(_ action: UInt32, _ data: UnsafeMutableRawPointer?) {}

@main
struct SystemSmoke {
    @MainActor static func main() {
        let app = NSApplication.shared
        app.finishLaunching()
        precondition(dg_activation_policy_set(99) < 0)
        precondition(dg_activation_policy_set(UInt32(DG_ACTIVATION_ACCESSORY)) == 0)
        precondition(app.activationPolicy() == .accessory)
        precondition(dg_status_item_is_available() == 0)
        let text = strdup("Fixture")!
        defer { free(text) }
        let p = UnsafePointer(text)
        var status = dg_status_item_state_v1()
        status.visible = 1
        status.title = p; status.tooltip = p; status.open_label = p
        status.recordings_label = p; status.quit_label = p; status.pause_menu_label = p
        status.pause_15_label = p; status.pause_30_label = p; status.pause_60_label = p
        status.pause_indefinite_label = p; status.primary_action_label = p
        precondition(dg_status_item_set(DG_STATUS_ITEM_ABI_MAJOR, &status, onAction, nil) == 0)
        precondition(dg_status_item_is_available() == 1)
        status.visible = 0
        precondition(dg_status_item_set(DG_STATUS_ITEM_ABI_MAJOR, &status, onAction, nil) == 0)
        precondition(dg_status_item_is_available() == 0)
        dg_status_item_stop(); dg_status_item_stop()
        precondition(dg_status_item_is_available() == 0)
        precondition(dg_activation_policy_set(UInt32(DG_ACTIVATION_REGULAR)) == 0)
        precondition(app.activationPolicy() == .regular)
        precondition(dg_system_start(DG_SYSTEM_ABI_MAJOR, onEvent, nil) == 0)
        NSWorkspace.shared.notificationCenter.post(name: NSWorkspace.willPowerOffNotification, object: NSWorkspace.shared)
        precondition(observed.count() == 1)
        dg_system_stop()
        NSWorkspace.shared.notificationCenter.post(name: NSWorkspace.willPowerOffNotification, object: NSWorkspace.shared)
        precondition(observed.count() == 1)
        print("native system smoke: policy, status availability, shutdown routing and teardown passed")
    }
}
