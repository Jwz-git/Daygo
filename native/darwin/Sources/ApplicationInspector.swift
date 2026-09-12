import AppKit
import Foundation

private let maximumApplicationPathBytes = 32_768
private let maximumApplicationIdentifierBytes = 4_096
private let maximumApplicationNameBytes = 4_096
private let maximumApplicationIconBytes = 262_144
private let applicationIconPixelSize = 64

private enum ApplicationInspectionFailure: Error {
    case invalidArgument
    case abiMismatch
    case notApplication
    case notFound
    case native(OSStatus)
}

@_cdecl("dg_application_abi_version")
func dg_application_abi_version(
    _ major: UnsafeMutablePointer<UInt32>?,
    _ minor: UnsafeMutablePointer<UInt32>?
) {
    major?.pointee = UInt32(DG_APPLICATION_ABI_MAJOR)
    minor?.pointee = UInt32(DG_APPLICATION_ABI_MINOR)
}

@_cdecl("dg_application_inspect")
func dg_application_inspect(
    _ requestedABIMajor: UInt32,
    _ applicationPath: dg_application_string_view_v1,
    _ infoPointer: UnsafeMutablePointer<dg_application_info_v2>?,
    _ errorPointer: UnsafeMutablePointer<dg_application_error_v1>?
) -> Int32 {
    inspectApplication(requestedABIMajor, infoPointer, errorPointer) { path in
        let url = URL(fileURLWithPath: path).standardizedFileURL
        var isDirectory: ObjCBool = false
        guard url.pathExtension.lowercased() == "app",
              FileManager.default.fileExists(atPath: url.path, isDirectory: &isDirectory),
              isDirectory.boolValue,
              let bundle = Bundle(url: url),
              let bundleIdentifier = bundle.bundleIdentifier,
              !bundleIdentifier.isEmpty
        else {
            throw ApplicationInspectionFailure.notApplication
        }
        return applicationIdentity(bundleIdentifier: bundleIdentifier, bundle: bundle, path: url.path)
    } decode: {
        try decodeApplicationPath(applicationPath)
    }
}

@_cdecl("dg_application_lookup")
func dg_application_lookup(
    _ requestedABIMajor: UInt32,
    _ bundleIdentifier: dg_application_string_view_v1,
    _ infoPointer: UnsafeMutablePointer<dg_application_info_v2>?,
    _ errorPointer: UnsafeMutablePointer<dg_application_error_v1>?
) -> Int32 {
    inspectApplication(requestedABIMajor, infoPointer, errorPointer) { identifier in
        guard let url = NSWorkspace.shared.urlForApplication(withBundleIdentifier: identifier) else {
            throw ApplicationInspectionFailure.notFound
        }
        guard let bundle = Bundle(url: url),
              let resolvedIdentifier = bundle.bundleIdentifier,
              !resolvedIdentifier.isEmpty
        else {
            throw ApplicationInspectionFailure.notFound
        }
        return applicationIdentity(bundleIdentifier: resolvedIdentifier, bundle: bundle, path: url.path)
    } decode: {
        try decodeApplicationString(bundleIdentifier, maximumBytes: maximumApplicationIdentifierBytes)
    }
}

/// Shared entry point for both ABIs: decode one caller-owned input, resolve the
/// application identity, and write the caller-owned output.
private func inspectApplication(
    _ requestedABIMajor: UInt32,
    _ infoPointer: UnsafeMutablePointer<dg_application_info_v2>?,
    _ errorPointer: UnsafeMutablePointer<dg_application_error_v1>?,
    resolve: (String) throws -> (identifier: String, name: String, iconPNG: Data?),
    decode: () throws -> String
) -> Int32 {
    guard let infoPointer else {
        return Int32(DG_APPLICATION_E_INVALID_ARGUMENT)
    }

    let expectedInfoSize = UInt32(MemoryLayout<dg_application_info_v2>.size)
    guard infoPointer.pointee.struct_size == expectedInfoSize,
          infoPointer.pointee.reserved0 == 0
    else {
        return writeApplicationFailure(.invalidArgument, to: errorPointer)
    }
    clearApplicationOutput(infoPointer)
    zeroApplicationError(errorPointer)

    guard requestedABIMajor == UInt32(DG_APPLICATION_ABI_MAJOR) else {
        return writeApplicationFailure(.abiMismatch, to: errorPointer)
    }

    do {
        let input = try decode()
        let application = try resolve(input)
        try writeApplicationString(
            application.identifier,
            maximumBytes: maximumApplicationIdentifierBytes,
            to: &infoPointer.pointee.identifier
        )
        try writeApplicationString(
            application.name,
            maximumBytes: maximumApplicationNameBytes,
            to: &infoPointer.pointee.name
        )
        try writeApplicationData(application.iconPNG ?? Data(), to: &infoPointer.pointee.icon_png)
        return Int32(DG_APPLICATION_OK)
    } catch let failure as ApplicationInspectionFailure {
        clearApplicationOutput(infoPointer)
        return writeApplicationFailure(failure, to: errorPointer)
    } catch {
        clearApplicationOutput(infoPointer)
        return writeApplicationFailure(.native(OSStatus((error as NSError).code)), to: errorPointer)
    }
}

private func clearApplicationOutput(_ infoPointer: UnsafeMutablePointer<dg_application_info_v2>) {
    infoPointer.pointee.identifier.len = 0
    infoPointer.pointee.name.len = 0
    infoPointer.pointee.icon_png.len = 0
}

/// Resolves the display identity of one installed bundle. The icon is optional:
/// a bundle without a loadable icon still yields a usable identity.
private func applicationIdentity(
    bundleIdentifier: String,
    bundle: Bundle,
    path: String
) -> (identifier: String, name: String, iconPNG: Data?) {
    let displayName = bundle.object(forInfoDictionaryKey: "CFBundleDisplayName") as? String
    let bundleName = bundle.object(forInfoDictionaryKey: "CFBundleName") as? String
    let name = [displayName, bundleName, URL(fileURLWithPath: path).deletingPathExtension().lastPathComponent]
        .compactMap { $0?.trimmingCharacters(in: .whitespacesAndNewlines) }
        .first { !$0.isEmpty } ?? bundleIdentifier
    return (bundleIdentifier, name, applicationIconPNG(at: path))
}

/// Renders the Finder icon of the bundle as a square PNG. Drawing happens into
/// a private bitmap so it stays safe off the main thread; any failure yields no
/// icon rather than failing the identity lookup.
private func applicationIconPNG(at path: String) -> Data? {
    let image = NSWorkspace.shared.icon(forFile: path)
    let size = NSSize(width: applicationIconPixelSize, height: applicationIconPixelSize)
    guard let representation = NSBitmapImageRep(
        bitmapDataPlanes: nil,
        pixelsWide: applicationIconPixelSize,
        pixelsHigh: applicationIconPixelSize,
        bitsPerSample: 8,
        samplesPerPixel: 4,
        hasAlpha: true,
        isPlanar: false,
        colorSpaceName: .deviceRGB,
        bytesPerRow: 0,
        bitsPerPixel: 0
    ), let context = NSGraphicsContext(bitmapImageRep: representation)
    else {
        return nil
    }
    representation.size = size
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = context
    image.draw(in: NSRect(origin: .zero, size: size), from: .zero, operation: .sourceOver, fraction: 1)
    context.flushGraphics()
    NSGraphicsContext.restoreGraphicsState()

    guard let data = representation.representation(using: .png, properties: [:]),
          !data.isEmpty,
          data.count <= maximumApplicationIconBytes
    else {
        return nil
    }
    return data
}

private func decodeApplicationPath(_ view: dg_application_string_view_v1) throws -> String {
    guard view.len > 0, view.len <= UInt64(maximumApplicationPathBytes), view.data != nil else {
        throw ApplicationInspectionFailure.invalidArgument
    }
    let path = try decodeApplicationString(view, maximumBytes: maximumApplicationPathBytes)
    guard path.first == "/" else {
        throw ApplicationInspectionFailure.invalidArgument
    }
    return path
}

private func decodeApplicationString(_ view: dg_application_string_view_v1, maximumBytes: Int) throws -> String {
    guard view.len > 0,
          view.len <= UInt64(maximumBytes),
          let data = view.data
    else {
        throw ApplicationInspectionFailure.invalidArgument
    }
    let bytes = Data(bytes: data, count: Int(view.len))
    guard let value = String(data: bytes, encoding: .utf8),
          value.utf8.count == Int(view.len),
          !value.utf8.contains(0)
    else {
        throw ApplicationInspectionFailure.invalidArgument
    }
    return value
}

private func writeApplicationString(
    _ value: String,
    maximumBytes: Int,
    to buffer: inout dg_application_buffer_v1
) throws {
    let bytes = Array(value.utf8)
    guard !bytes.isEmpty,
          bytes.count <= maximumBytes,
          UInt64(bytes.count) <= buffer.capacity,
          let destination = buffer.data
    else {
        throw ApplicationInspectionFailure.invalidArgument
    }
    bytes.withUnsafeBufferPointer { source in
        destination.update(from: source.baseAddress!, count: source.count)
    }
    buffer.len = UInt64(bytes.count)
}

/// Writes optional binary output. An empty payload or one that does not fit the
/// caller's buffer leaves len at 0 instead of failing the call.
private func writeApplicationData(
    _ value: Data,
    to buffer: inout dg_application_buffer_v1
) throws {
    guard !value.isEmpty else {
        buffer.len = 0
        return
    }
    guard UInt64(value.count) <= buffer.capacity, let destination = buffer.data else {
        buffer.len = 0
        return
    }
    value.withUnsafeBytes { source in
        guard let base = source.baseAddress else { return }
        destination.update(from: base.assumingMemoryBound(to: UInt8.self), count: value.count)
    }
    buffer.len = UInt64(value.count)
}

private func writeApplicationFailure(
    _ failure: ApplicationInspectionFailure,
    to errorPointer: UnsafeMutablePointer<dg_application_error_v1>?
) -> Int32 {
    let status: Int32
    let nativeCode: Int64

    switch failure {
    case .invalidArgument:
        status = Int32(DG_APPLICATION_E_INVALID_ARGUMENT)
        nativeCode = 0
    case .abiMismatch:
        status = Int32(DG_APPLICATION_E_ABI_MISMATCH)
        nativeCode = 0
    case .notApplication:
        status = Int32(DG_APPLICATION_E_NOT_APPLICATION)
        nativeCode = 0
    case .notFound:
        status = Int32(DG_APPLICATION_E_NOT_FOUND)
        nativeCode = 0
    case .native(let code):
        status = Int32(DG_APPLICATION_E_INTERNAL)
        nativeCode = Int64(code)
    }

    if let errorPointer,
       errorPointer.pointee.struct_size == UInt32(MemoryLayout<dg_application_error_v1>.size) {
        errorPointer.pointee.native_domain = nativeCode == 0
            ? UInt32(DG_APPLICATION_NATIVE_NONE)
            : UInt32(DG_APPLICATION_NATIVE_APPLE)
        errorPointer.pointee.native_code = nativeCode
    }
    return status
}

private func zeroApplicationError(_ pointer: UnsafeMutablePointer<dg_application_error_v1>?) {
    guard let pointer,
          pointer.pointee.struct_size == UInt32(MemoryLayout<dg_application_error_v1>.size)
    else {
        return
    }
    pointer.pointee.native_domain = UInt32(DG_APPLICATION_NATIVE_NONE)
    pointer.pointee.native_code = 0
}
