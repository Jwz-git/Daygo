import AppKit
import Foundation

private final class StatusController: NSObject {
    let item: NSStatusItem
    let menu = NSMenu()
    var callback: dg_status_item_action_callback_v1?
    var callbackData: UnsafeMutableRawPointer?
    let pauseItem = NSMenuItem()
    
    init(state: UnsafePointer<dg_status_item_state_v1>, callback: dg_status_item_action_callback_v1?, data: UnsafeMutableRawPointer?) {
        item = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        self.callback = callback
        self.callbackData = data
        super.init()
        let open = NSMenuItem(title: String(cString: state.pointee.open_label), action: #selector(openAction(_:)), keyEquivalent: "")
        open.target = self
        pauseItem.action = #selector(pauseAction(_:)); pauseItem.target = self
        let quit = NSMenuItem(title: String(cString: state.pointee.quit_label), action: #selector(quitAction(_:)), keyEquivalent: "")
        quit.target = self
        menu.addItem(open); menu.addItem(pauseItem); menu.addItem(.separator()); menu.addItem(quit)
        item.menu = menu
        update(state: state)
    }
    
    func update(state: UnsafePointer<dg_status_item_state_v1>) {
        let tooltip = String(cString: state.pointee.tooltip)
        item.button?.image = NSImage(systemSymbolName: "clock", accessibilityDescription: tooltip)
        item.button?.image?.isTemplate = true
        item.button?.toolTip = tooltip
        item.button?.title = String(cString: state.pointee.title)
        item.button?.target = self; item.button?.action = #selector(openAction(_:))
        pauseItem.title = String(cString: state.pointee.pause_label)
        pauseItem.isEnabled = state.pointee.pause_enabled != 0
        item.isVisible = state.pointee.visible != 0
    }
    @objc func openAction(_ sender: Any?) { callback?(UInt32(DG_STATUS_ITEM_OPEN), callbackData) }
    @objc func pauseAction(_ sender: Any?) { callback?(UInt32(DG_STATUS_ITEM_TOGGLE_PAUSE), callbackData) }
    @objc func quitAction(_ sender: Any?) { callback?(UInt32(DG_STATUS_ITEM_QUIT), callbackData) }
}

private let statusLock = NSLock()
nonisolated(unsafe) private var statusController: StatusController?
@_cdecl("dg_status_item_set")
func dg_status_item_set(_ requested: UInt32, _ state: UnsafePointer<dg_status_item_state_v1>?, _ callback: dg_status_item_action_callback_v1?, _ data: UnsafeMutableRawPointer?) -> Int32 {
    guard requested == DG_STATUS_ITEM_ABI_MAJOR, let state else { return -1 }
    let operation = {
        statusLock.lock()
        if let existing = statusController {
            existing.update(state: state)
        } else {
            statusController = StatusController(state: state, callback: callback, data: data)
        }
        statusLock.unlock()
    }
    if Thread.isMainThread { operation() } else { DispatchQueue.main.sync(execute: operation) }
    return 0
}

@_cdecl("dg_status_item_stop")
func dg_status_item_stop() {
    let operation = {
        statusLock.lock(); statusController?.item.isVisible = false; statusController = nil; statusLock.unlock()
    }
    if Thread.isMainThread { operation() } else { DispatchQueue.main.sync(execute: operation) }
}

