@preconcurrency import AppKit
import Foundation

private func dockShortcutURL() -> URL? {
    let url = Bundle.main.bundleURL
    guard url.pathExtension.lowercased() == "app",
          !url.pathComponents.contains("AppTranslocation") else { return nil }
    return url
}

@MainActor private var dockShortcutPanel: NSPanel?

@MainActor private final class DockShortcutIcon: NSImageView, NSDraggingSource {
    let appURL: URL

    init(appURL: URL, label: String) {
        self.appURL = appURL
        super.init(frame: .zero)
        image = NSWorkspace.shared.icon(forFile: appURL.path)
        imageScaling = .scaleProportionallyUpOrDown
        setAccessibilityLabel(label)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not supported") }

    override func mouseDown(with event: NSEvent) {}

    override func mouseDragged(with event: NSEvent) {
        let item = NSDraggingItem(pasteboardWriter: appURL as NSURL)
        let point = convert(event.locationInWindow, from: nil)
        let frame = bounds.offsetBy(dx: point.x - bounds.midX, dy: point.y - bounds.midY)
        item.setDraggingFrame(frame, contents: image)
        beginDraggingSession(with: [item], event: event, source: self)
    }

    func draggingSession(_ session: NSDraggingSession, sourceOperationMaskFor context: NSDraggingContext) -> NSDragOperation {
        .link
    }

    func draggingSession(_ session: NSDraggingSession, endedAt screenPoint: NSPoint, operation: NSDragOperation) {
        if !operation.isEmpty { window?.close() }
    }
}

@MainActor func dismissDockShortcut() {
    dockShortcutPanel?.close()
    dockShortcutPanel = nil
}

@_cdecl("dg_dock_shortcut_available")
func dg_dock_shortcut_available() -> Int32 { dockShortcutURL() == nil ? 0 : 1 }

@_cdecl("dg_dock_shortcut_show")
func dg_dock_shortcut_show(_ rawTitle: UnsafePointer<CChar>?, _ rawHint: UnsafePointer<CChar>?) -> Int32 {
    guard let rawTitle, let rawHint,
          (1...512).contains(strnlen(rawTitle, 513)), (1...512).contains(strnlen(rawHint, 513)),
          let title = String(validatingCString: rawTitle), let hint = String(validatingCString: rawHint),
          let appURL = dockShortcutURL() else { return -1 }
    let show: @Sendable @MainActor () -> Void = {
        dismissDockShortcut()
        let panel = NSPanel(contentRect: NSRect(x: 0, y: 0, width: 320, height: 160),
                            styleMask: [.titled, .closable], backing: .buffered, defer: false)
        panel.isReleasedWhenClosed = false
        panel.title = title
        let icon = DockShortcutIcon(appURL: appURL, label: title)
        let instructions = NSTextField(wrappingLabelWithString: hint)
        instructions.alignment = .center
        let stack = NSStackView(views: [icon, instructions])
        stack.orientation = .vertical
        stack.alignment = .centerX
        stack.spacing = 12
        stack.translatesAutoresizingMaskIntoConstraints = false
        let content = NSView()
        content.addSubview(stack)
        panel.contentView = content
        NSLayoutConstraint.activate([
            icon.widthAnchor.constraint(equalToConstant: 64),
            icon.heightAnchor.constraint(equalToConstant: 64),
            instructions.widthAnchor.constraint(equalToConstant: 280),
            stack.centerXAnchor.constraint(equalTo: content.centerXAnchor),
            stack.centerYAnchor.constraint(equalTo: content.centerYAnchor),
        ])
        panel.center()
        panel.makeKeyAndOrderFront(nil)
        dockShortcutPanel = panel
    }
    if Thread.isMainThread {
        MainActor.assumeIsolated { show() }
    } else {
        DispatchQueue.main.async { MainActor.assumeIsolated { show() } }
    }
    return 0
}

#if DAYGO_NATIVE_TESTS
@MainActor func dockShortcutForSmoke() -> (NSPanel, URL)? {
    guard let panel = dockShortcutPanel,
          let stack = panel.contentView?.subviews.first as? NSStackView,
          let icon = stack.arrangedSubviews.first as? DockShortcutIcon else { return nil }
    return (panel, icon.appURL)
}
#endif
