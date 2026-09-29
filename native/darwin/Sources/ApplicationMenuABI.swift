import AppKit
import Foundation

@MainActor private var applicationMenuCopy: [String: String] = [:]

@MainActor func applyApplicationMenuCopy() {
  guard let menu = NSApp.mainMenu else { return }
  let selectors = [
    "hide:": "hide", "hideOtherApplications:": "hideOthers", "unhideAllApplications:": "showAll",
    "Quit": "background",
    "undo:": "undo", "redo:": "redo", "cut:": "cut", "copy:": "copy", "paste:": "paste",
    "pasteAsRichText:": "pasteMatch",
    "delete:": "delete", "selectAll:": "selectAll", "startSpeaking:": "startSpeaking",
    "stopSpeaking:": "stopSpeaking",
    "performMiniaturize:": "minimize", "performZoom:": "zoom", "enterFullScreenMode:": "fullScreen",
  ]
  func translate(_ menu: NSMenu) {
    for item in menu.items {
      if let action = item.action, let key = selectors[NSStringFromSelector(action)],
        let label = applicationMenuCopy[key]
      {
        item.title = label
      }
      if let submenu = item.submenu {
        let actions = Set(submenu.items.compactMap { $0.action.map(NSStringFromSelector) })
        let key: String?
        if actions.contains("undo:") {
          key = "edit"
        } else if actions.contains("performMiniaturize:") {
          key = "window"
        } else if actions.contains("startSpeaking:") {
          key = "speech"
        } else {
          key = nil
        }
        if let key, let label = applicationMenuCopy[key] {
          item.title = label
          submenu.title = label
        }
        translate(submenu)
      }
    }
  }
  translate(menu)
}

@_cdecl("dg_application_menu_labels_set")
func dg_application_menu_labels_set(_ requested: UInt32, _ raw: UnsafePointer<CChar>?) -> Int32 {
  guard requested == DG_SYSTEM_ABI_MAJOR, let raw else { return -1 }
  let length = strnlen(raw, 8193)
  guard length > 0, length <= 8192 else { return -1 }
  let data = Data(bytes: raw, count: length)
  guard let copy = (try? JSONSerialization.jsonObject(with: data)) as? [String: String] else {
    return -1
  }
  let keys = [
    "hide", "hideOthers", "showAll", "background", "edit", "undo", "redo", "cut", "copy", "paste",
    "pasteMatch", "delete", "selectAll", "speech", "startSpeaking", "stopSpeaking", "window",
    "minimize", "zoom", "fullScreen",
  ]
  guard keys.allSatisfy({ key in copy[key].map { !$0.isEmpty && $0.utf8.count <= 512 } ?? false }),
    copy.count == keys.count
  else { return -1 }
  let apply: @Sendable @MainActor () -> Void = {
    applicationMenuCopy = copy
    applyApplicationMenuCopy()
  }
  if Thread.isMainThread {
    MainActor.assumeIsolated { apply() }
  } else {
    DispatchQueue.main.async { MainActor.assumeIsolated { apply() } }
  }
  return 0
}
