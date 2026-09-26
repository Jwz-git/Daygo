@preconcurrency import AppKit
import Foundation

@MainActor private var currentMessage: ResidentMessageController?

@MainActor private final class ResidentMessageController: NSObject, NSWindowDelegate {
  let alert = NSAlert()
  init(title: String, message: String, button: String) {
    super.init()
    alert.alertStyle = .warning
    alert.messageText = title
    alert.informativeText = message
    let acknowledge = alert.addButton(withTitle: button)
    acknowledge.target = self
    acknowledge.action = #selector(dismiss(_:))
    alert.window.delegate = self
  }
  func show() {
    alert.window.center()
    alert.window.makeKeyAndOrderFront(nil)
  }
  @objc func dismiss(_ sender: Any?) { dismissResidentMessage() }
  func windowWillClose(_ notification: Notification) { dismissResidentMessage() }
}

@MainActor func dismissResidentMessage() {
  currentMessage?.alert.window.orderOut(nil)
  currentMessage = nil
}

@_cdecl("dg_status_message_show")
func dg_status_message_show(_ requested: UInt32, _ raw: UnsafePointer<CChar>?) -> Int32 {
  guard requested == DG_SYSTEM_ABI_MAJOR, let raw else { return -1 }
  let length = strnlen(raw, 8193)
  guard length > 0, length <= 8192 else { return -1 }
  let data = Data(bytes: raw, count: length)
  guard let copy = (try? JSONSerialization.jsonObject(with: data)) as? [String: String],
    copy.count == 3,
    let title = copy["title"], !title.isEmpty, title.utf8.count <= 512,
    let message = copy["message"], !message.isEmpty, message.utf8.count <= 4096,
    let button = copy["button"], !button.isEmpty, button.utf8.count <= 512
  else { return -1 }
  let show: @Sendable @MainActor () -> Void = {
    dismissResidentMessage()
    let controller = ResidentMessageController(title: title, message: message, button: button)
    currentMessage = controller
    controller.show()
  }
  if Thread.isMainThread {
    MainActor.assumeIsolated { show() }
  } else {
    DispatchQueue.main.async { MainActor.assumeIsolated { show() } }
  }
  return 0
}

#if DAYGO_NATIVE_TESTS
  @MainActor func residentMessageForSmoke() -> NSAlert? { currentMessage?.alert }
#endif
