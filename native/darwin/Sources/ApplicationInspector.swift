import Foundation

private let maximumApplicationPathBytes = 32_768
private let maximumApplicationIdentifierBytes = 4_096
private let maximumApplicationNameBytes = 4_096

private enum ApplicationInspectionFailure: Error {
    case invalidArgument
    case abiMismatch
    case notApplication
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
    _ infoPointer: UnsafeMutablePointer<dg_application_info_v1>?,
    _ errorPointer: UnsafeMutablePointer<dg_application_error_v1>?
) -> Int32 {
    guard let infoPointer else {
        return Int32(DG_APPLICATION_E_INVALID_ARGUMENT)
    }

    let expectedInfoSize = UInt32(MemoryLayout<dg_application_info_v1>.size)
    guard infoPointer.pointee.struct_size == expectedInfoSize,
          infoPointer.pointee.reserved0 == 0
    else {
        return writeApplicationFailure(.invalidArgument, to: errorPointer)
    }
    infoPointer.pointee.identifier.len = 0
    infoPointer.pointee.name.len = 0
    zeroApplicationError(errorPointer)

    guard requestedABIMajor == UInt32(DG_APPLICATION_ABI_MAJOR) else {
        return writeApplicationFailure(.abiMismatch, to: errorPointer)
    }

    do {
        let path = try decodeApplicationPath(applicationPath)
        let application = try inspectApplication(at: path)
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
        return Int32(DG_APPLICATION_OK)
    } catch let failure as ApplicationInspectionFailure {
        infoPointer.pointee.identifier.len = 0
        infoPointer.pointee.name.len = 0
        return writeApplicationFailure(failure, to: errorPointer)
    } catch {
        infoPointer.pointee.identifier.len = 0
        infoPointer.pointee.name.len = 0
        return writeApplicationFailure(.native(OSStatus((error as NSError).code)), to: errorPointer)
    }
}

private func decodeApplicationPath(_ view: dg_application_string_view_v1) throws -> String {
    guard view.len > 0,
          view.len <= UInt64(maximumApplicationPathBytes),
          let data = view.data
    else {
        throw ApplicationInspectionFailure.invalidArgument
    }
    let bytes = Data(bytes: data, count: Int(view.len))
    guard let path = String(data: bytes, encoding: .utf8),
          path.utf8.count == Int(view.len),
          path.first == "/",
          !path.utf8.contains(0)
    else {
        throw ApplicationInspectionFailure.invalidArgument
    }
    return path
}

private func inspectApplication(at path: String) throws -> (identifier: String, name: String) {
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

    let displayName = bundle.object(forInfoDictionaryKey: "CFBundleDisplayName") as? String
    let bundleName = bundle.object(forInfoDictionaryKey: "CFBundleName") as? String
    let name = [displayName, bundleName, url.deletingPathExtension().lastPathComponent]
        .compactMap { $0?.trimmingCharacters(in: .whitespacesAndNewlines) }
        .first { !$0.isEmpty } ?? bundleIdentifier

    return (bundleIdentifier, name)
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
