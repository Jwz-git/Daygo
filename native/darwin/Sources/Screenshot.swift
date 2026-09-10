import CoreGraphics
import Foundation
import Security
@preconcurrency import ScreenCaptureKit

enum ScreenshotFailure: Error, Sendable {
    case blocked
    case invalidArgument
    case unsupported
    case permissionDenied
    case privacyUnsupported
    case noDisplay
    case timeout
    case io(Int32)
    case native(Int64)
}

final class CaptureCancellation: @unchecked Sendable {
    private let lock = NSLock()
    private var cancelled = false
    private var published = false

    func cancel() -> Bool {
        lock.lock()
        defer { lock.unlock() }
        cancelled = true
        return published
    }

    func publish(_ operation: () throws -> Void) throws {
        lock.lock()
        defer { lock.unlock() }
        guard !cancelled else {
            throw ScreenshotFailure.timeout
        }
        try operation()
        published = true
    }
}

struct ScreenshotRequest: Sendable {
    let outputPath: String
    let targetHeight: Int
    let jpegQuality: Int
    let showsCursor: Bool
    let blockedApplicationIDs: Set<String>
    let cancellation: CaptureCancellation
}

struct ScreenshotResult: Sendable {
    let capturedAtUnixNS: Int64
    let fileSize: UInt64
    let width: Int
    let height: Int
}

func capturePrimaryDisplay(_ request: ScreenshotRequest) async throws -> ScreenshotResult {
    captureDiagnostic("permission.preflight.begin")
    guard CGPreflightScreenCaptureAccess() else {
        captureDiagnostic("permission.preflight.denied")
        throw ScreenshotFailure.permissionDenied
    }
    captureDiagnostic("permission.preflight.granted")
    if !request.blockedApplicationIDs.isEmpty {
        captureDiagnostic("privacy.preflight.begin")
        if try frontmostApplicationIsBlocked(request.blockedApplicationIDs) {
            captureDiagnostic("privacy.preflight.blocked")
            throw ScreenshotFailure.blocked
        }
        captureDiagnostic("privacy.preflight.passed")
    }
    do {
        try Task.checkCancellation()
        captureDiagnostic("content.query.begin")
        let content = try await SCShareableContent.excludingDesktopWindows(
            false,
            onScreenWindowsOnly: true
        )
        captureDiagnostic("content.query.completed")
        guard let display = content.displays.first(where: {
            $0.displayID == CGMainDisplayID()
        }) else {
            throw ScreenshotFailure.noDisplay
        }
        guard display.width > 0, display.height > 0 else {
            throw ScreenshotFailure.noDisplay
        }
        captureDiagnostic("display.resolved.\(display.width)x\(display.height)")

        if !request.blockedApplicationIDs.isEmpty {
            captureDiagnostic("privacy.final.begin")
            if try frontmostApplicationIsBlocked(request.blockedApplicationIDs) {
                captureDiagnostic("privacy.final.blocked")
                throw ScreenshotFailure.blocked
            }
            captureDiagnostic("privacy.final.passed")
        }
        let excludedApplications = content.applications.filter {
            request.blockedApplicationIDs.contains($0.bundleIdentifier)
        }
        let filter = SCContentFilter(
            display: display,
            excludingApplications: excludedApplications,
            exceptingWindows: []
        )
        let outputWidth = max(
            1,
            Int(
                (Double(display.width) * Double(request.targetHeight) / Double(display.height))
                    .rounded()
            )
        )
        let configuration = SCStreamConfiguration()
        configuration.width = outputWidth
        configuration.height = request.targetHeight
        configuration.scalesToFit = true
        configuration.showsCursor = request.showsCursor

        try Task.checkCancellation()
        captureDiagnostic("screenshot.capture.begin")
        let startedAt = Date()
        let image = try await SCScreenshotManager.captureImage(
            contentFilter: filter,
            configuration: configuration
        )
        let finishedAt = Date()
        captureDiagnostic("screenshot.capture.completed.\(image.width)x\(image.height)")
        try Task.checkCancellation()

        captureDiagnostic("jpeg.write.begin")
        let fileSize = try writeJPEGAtomically(
            image,
            outputPath: request.outputPath,
            quality: request.jpegQuality,
            cancellation: request.cancellation
        )
        captureDiagnostic("jpeg.write.completed.\(fileSize)")
        let midpoint = startedAt.timeIntervalSince1970
            + finishedAt.timeIntervalSince(startedAt) / 2
        return ScreenshotResult(
            capturedAtUnixNS: Int64((midpoint * 1_000_000_000).rounded()),
            fileSize: fileSize,
            width: image.width,
            height: image.height
        )
    } catch is CancellationError {
        captureDiagnostic("failure.cancelled")
        throw ScreenshotFailure.timeout
    } catch let failure as ScreenshotFailure {
        captureDiagnostic("failure.screenshot.\(failure)")
        throw failure
    } catch {
        let nativeCode = Int64((error as NSError).code)
        captureDiagnostic("failure.native.\(nativeCode)")
        if !CGPreflightScreenCaptureAccess() {
            throw ScreenshotFailure.permissionDenied
        }
        throw ScreenshotFailure.native(nativeCode)
    }
}

private func frontmostApplicationIsBlocked(_ blockedApplicationIDs: Set<String>) throws -> Bool {
    guard !blockedApplicationIDs.isEmpty else {
        return false
    }
    guard let identifier = frontmostVisibleApplicationIdentifier() else {
        throw ScreenshotFailure.privacyUnsupported
    }
    return blockedApplicationIDs.contains(identifier)
}

private func frontmostVisibleApplicationIdentifier() -> String? {
    let options: CGWindowListOption = [.optionOnScreenOnly, .excludeDesktopElements]
    guard let windows = CGWindowListCopyWindowInfo(options, kCGNullWindowID)
        as? [[CFString: Any]]
    else {
        return nil
    }

    for window in windows {
        guard let layer = window[kCGWindowLayer] as? Int,
              layer == 0,
              let pid = window[kCGWindowOwnerPID] as? Int32
        else {
            continue
        }
        var dynamicCode: SecCode?
        let attributes = [kSecGuestAttributePid: pid] as CFDictionary
        guard SecCodeCopyGuestWithAttributes(nil, attributes, [], &dynamicCode) == errSecSuccess,
              let dynamicCode
        else {
            continue
        }
        var staticCode: SecStaticCode?
        guard SecCodeCopyStaticCode(dynamicCode, [], &staticCode) == errSecSuccess,
              let staticCode
        else {
            continue
        }
        var information: CFDictionary?
        guard SecCodeCopySigningInformation(
            staticCode,
            SecCSFlags(rawValue: kSecCSSigningInformation),
            &information
        ) == errSecSuccess,
            let dictionary = information as? [CFString: Any],
            let identifier = dictionary[kSecCodeInfoIdentifier] as? String
        else {
            continue
        }
        return identifier
    }
    return nil
}
