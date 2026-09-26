import AppKit
import Foundation

private final class ObservedEvents: @unchecked Sendable {
  let lock = NSLock()
  var shutdowns = 0
  func record(_ kind: UInt32) {
    lock.lock()
    defer { lock.unlock() }
    if kind == UInt32(DG_SYSTEM_SHUTDOWN) { shutdowns += 1 }
  }
  func count() -> Int {
    lock.lock()
    defer { lock.unlock() }
    return shutdowns
  }
}
private let observed = ObservedEvents()
private func onEvent(_ kind: UInt32, _ at: Int64, _ data: UnsafeMutableRawPointer?) {
  observed.record(kind)
}
private func onAction(_ action: UInt32, _ data: UnsafeMutableRawPointer?) {}

private final class WorkerCompletion: @unchecked Sendable {
  let lock = NSLock()
  private var done = false
  func finish() {
    lock.lock()
    done = true
    lock.unlock()
  }
  func finished() -> Bool {
    lock.lock()
    defer { lock.unlock() }
    return done
  }
}

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
    status.title = p
    status.tooltip = p
    status.open_label = p
    status.recordings_label = p
    status.quit_label = p
    status.pause_menu_label = p
    status.pause_15_label = p
    status.pause_30_label = p
    status.pause_60_label = p
    status.pause_indefinite_label = p
    status.primary_action_label = p
    precondition(dg_status_item_set(DG_STATUS_ITEM_ABI_MAJOR, &status, onAction, nil) == 0)
    precondition(dg_status_item_is_available() == 1)
    let firstHeader = statusItemMenuForSmoke()!.items[0]
    precondition(firstHeader.title == "Fixture" && !firstHeader.isEnabled)
    precondition(dg_status_item_set(DG_STATUS_ITEM_ABI_MAJOR, &status, onAction, nil) == 0)
    precondition(statusItemMenuForSmoke()!.items[0] === firstHeader)
    status.icon = 99
    precondition(dg_status_item_set(DG_STATUS_ITEM_ABI_MAJOR, &status, onAction, nil) < 0)
    status.icon = UInt32(DG_STATUS_ICON_WARNING)
    precondition(dg_status_item_set(DG_STATUS_ITEM_ABI_MAJOR, &status, onAction, nil) == 0)
    status.visible = 0
    precondition(dg_status_item_set(DG_STATUS_ITEM_ABI_MAJOR, &status, onAction, nil) == 0)
    precondition(dg_status_item_is_available() == 0)
    dg_status_item_stop()
    dg_status_item_stop()
    precondition(dg_status_item_is_available() == 0)
    precondition(dg_activation_policy_set(UInt32(DG_ACTIVATION_REGULAR)) == 0)
    precondition(app.activationPolicy() == .regular)
    verifyWorkerThread(app)
    verifyApplicationMenu(app)
    let message = #"{"title":"Fixture title","message":"Fixture message","button":"Acknowledge"}"#
    precondition(message.withCString { dg_status_message_show(DG_SYSTEM_ABI_MAJOR, $0) } == 0)
    let alert = residentMessageForSmoke()!
    precondition(
      alert.messageText == "Fixture title" && alert.informativeText == "Fixture message"
        && alert.window.isVisible)
    alert.buttons[0].performClick(nil)
    precondition(residentMessageForSmoke() == nil)
    precondition("{}".withCString { dg_status_message_show(DG_SYSTEM_ABI_MAJOR, $0) } < 0)
    precondition(dg_system_start(DG_SYSTEM_ABI_MAJOR, onEvent, nil) == 0)
    // Wails sets regular during launch. Reapply an already requested policy
    // after that lifecycle boundary, independently of frontend startup timing.
    precondition(dg_activation_policy_set(UInt32(DG_ACTIVATION_ACCESSORY)) == 0)
    precondition(app.setActivationPolicy(.regular))
    NotificationCenter.default.post(name: NSApplication.didFinishLaunchingNotification, object: app)
    RunLoop.current.run(until: Date().addingTimeInterval(0.02))
    precondition(app.activationPolicy() == .accessory)
    NSWorkspace.shared.notificationCenter.post(
      name: NSWorkspace.willPowerOffNotification, object: NSWorkspace.shared)
    precondition(observed.count() == 1)
    precondition(message.withCString { dg_status_message_show(DG_SYSTEM_ABI_MAJOR, $0) } == 0)
    dg_system_stop()
    precondition(residentMessageForSmoke() == nil)
    NSWorkspace.shared.notificationCenter.post(
      name: NSWorkspace.willPowerOffNotification, object: NSWorkspace.shared)
    precondition(observed.count() == 1)
    print(
      "native system smoke: main/worker policy, status availability/deduplication, nonblocking feedback, main-menu localization, shutdown routing and teardown passed"
    )
  }

  @MainActor private static func verifyApplicationMenu(_ app: NSApplication) {
    let menu = NSMenu()
    let appRoot = NSMenuItem(title: "Fixture", action: nil, keyEquivalent: "")
    let appMenu = NSMenu(title: "Fixture")
    let target = NSObject()
    let quit = NSMenuItem(title: "Quit", action: NSSelectorFromString("Quit"), keyEquivalent: "q")
    quit.target = target
    appMenu.addItem(quit)
    appRoot.submenu = appMenu
    menu.addItem(appRoot)
    let editRoot = NSMenuItem(title: "Edit", action: nil, keyEquivalent: "")
    let edit = NSMenu(title: "Edit")
    let undo = NSMenuItem(title: "Undo", action: NSSelectorFromString("undo:"), keyEquivalent: "z")
    edit.addItem(undo)
    editRoot.submenu = edit
    menu.addItem(editRoot)
    app.mainMenu = menu
    let keys = [
      "hide", "hideOthers", "showAll", "background", "edit", "undo", "redo", "cut", "copy", "paste",
      "pasteMatch", "delete", "selectAll", "speech", "startSpeaking", "stopSpeaking", "window",
      "minimize", "zoom", "fullScreen",
    ]
    for prefix in ["First", "Second"] {
      let copy = Dictionary(uniqueKeysWithValues: keys.map { ($0, "\(prefix)-\($0)") })
      let data = try! JSONSerialization.data(withJSONObject: copy)
      let json = String(data: data, encoding: .utf8)!
      precondition(
        json.withCString { dg_application_menu_labels_set(DG_SYSTEM_ABI_MAJOR, $0) } == 0)
      precondition(
        quit.title == "\(prefix)-background" && editRoot.title == "\(prefix)-edit"
          && undo.title == "\(prefix)-undo")
      precondition(
        quit.action == NSSelectorFromString("Quit") && quit.keyEquivalent == "q"
          && quit.target === target)
      precondition(undo.action == NSSelectorFromString("undo:") && undo.keyEquivalent == "z")
    }
    precondition("{}".withCString { dg_application_menu_labels_set(DG_SYSTEM_ABI_MAJOR, $0) } < 0)
    precondition(
      String(repeating: "x", count: 8193).withCString {
        dg_application_menu_labels_set(DG_SYSTEM_ABI_MAJOR, $0)
      } < 0)
  }

  @MainActor private static func verifyWorkerThread(_ app: NSApplication) {
    let completion = WorkerCompletion()
    DispatchQueue.global().async {
      precondition(dg_activation_policy_set(UInt32(DG_ACTIVATION_ACCESSORY)) == 0)
      let text = strdup("Worker fixture")!
      let p = UnsafePointer(text)
      var status = dg_status_item_state_v1()
      status.visible = 1
      status.icon = UInt32(DG_STATUS_ICON_ACTIVE)
      status.title = p
      status.tooltip = p
      status.open_label = p
      status.recordings_label = p
      status.quit_label = p
      status.pause_menu_label = p
      status.pause_15_label = p
      status.pause_30_label = p
      status.pause_60_label = p
      status.pause_indefinite_label = p
      status.primary_action_label = p
      precondition(dg_status_item_set(DG_STATUS_ITEM_ABI_MAJOR, &status, onAction, nil) == 0)
      free(text)
      precondition(dg_status_item_is_available() == 1)
      dg_status_item_stop()
      precondition(dg_status_item_is_available() == 0)
      precondition(dg_activation_policy_set(UInt32(DG_ACTIVATION_REGULAR)) == 0)
      completion.finish()
    }
    let deadline = Date().addingTimeInterval(5)
    while !completion.finished() && Date() < deadline {
      RunLoop.current.run(until: Date().addingTimeInterval(0.01))
    }
    precondition(completion.finished(), "worker main-thread bridge deadlocked")
    precondition(app.activationPolicy() == .regular)
  }
}
