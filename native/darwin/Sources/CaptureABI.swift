import Dispatch
import Foundation

private let maximumPathBytes = 32_768
private let maximumApplicationIDBytes = 4_096
private let maximumApplicationIDs = 4_096
private let maximumApplicationIDTotalBytes = 1 << 20

private final class BlockingCaptureResult: @unchecked Sendable {
    private let lock = NSLock()
    private let semaphore = DispatchSemaphore(value: 0)
    private var result: Result<ScreenshotResult, ScreenshotFailure>?

    func complete(_ result: Result<ScreenshotResult, ScreenshotFailure>) {
        lock.lock()
        self.result = result
        lock.unlock()
        semaphore.signal()
    }

    func wait(until deadline: DispatchTime) -> Bool {
        semaphore.wait(timeout: deadline) == .success
    }

    func take() -> Result<ScreenshotResult, ScreenshotFailure> {
        lock.lock()
        defer { lock.unlock() }
        return result!
    }
}

@_cdecl("dg_capture_abi_version")
func dg_capture_abi_version(
    _ major: UnsafeMutablePointer<UInt32>?,
    _ minor: UnsafeMutablePointer<UInt32>?
) {
    major?.pointee = UInt32(DG_CAPTURE_ABI_MAJOR)
    minor?.pointee = UInt32(DG_CAPTURE_ABI_MINOR)
}

@_cdecl("dg_capture_once")
func dg_capture_once(
    _ requestedABIMajor: UInt32,
    _ requestPointer: UnsafePointer<dg_capture_request_v1>?,
    _ resultPointer: UnsafeMutablePointer<dg_capture_result_v1>?,
    _ errorPointer: UnsafeMutablePointer<dg_capture_error_v1>?
) -> Int32 {
    captureDiagnostic("abi.enter")
    guard let requestPointer, let resultPointer else {
        return Int32(DG_CAPTURE_E_INVALID_ARGUMENT)
    }

    let suppliedResultSize = resultPointer.pointee.struct_size
    zeroResult(resultPointer, preservingSize: suppliedResultSize)
    if let errorPointer {
        let suppliedErrorSize = errorPointer.pointee.struct_size
        zeroError(errorPointer, preservingSize: suppliedErrorSize)
    }

    guard requestedABIMajor == UInt32(DG_CAPTURE_ABI_MAJOR),
          requestPointer.pointee.struct_size >= MemoryLayout<dg_capture_request_v1>.size,
          suppliedResultSize >= MemoryLayout<dg_capture_result_v1>.size,
          errorPointer == nil || errorPointer!.pointee.struct_size >= MemoryLayout<dg_capture_error_v1>.size
    else {
        return Int32(DG_CAPTURE_E_ABI_MISMATCH)
    }

    let decodedRequest: ScreenshotRequest
    do {
        decodedRequest = try decodeRequest(requestPointer.pointee)
    } catch let failure as ScreenshotFailure {
        return writeFailure(failure, to: errorPointer)
    } catch {
        return Int32(DG_CAPTURE_E_INVALID_ARGUMENT)
    }
    captureDiagnostic("abi.request.decoded")

    let box = BlockingCaptureResult()
    let task = Task.detached(priority: .userInitiated) {
        captureDiagnostic("task.started")
        do {
            box.complete(.success(try await capturePrimaryDisplay(decodedRequest)))
        } catch let failure as ScreenshotFailure {
            box.complete(.failure(failure))
        } catch {
            box.complete(.failure(.native(Int64((error as NSError).code))))
        }
        captureDiagnostic("task.completed")
    }

    let timeout = DispatchTime.now() + .milliseconds(Int(requestPointer.pointee.timeout_ms))
    if !box.wait(until: timeout) {
        captureDiagnostic("abi.timeout")
        task.cancel()
        let published = decodedRequest.cancellation.cancel()
        if published {
            try? FileManager.default.removeItem(atPath: decodedRequest.outputPath)
        }
        return writeFailure(.timeout, to: errorPointer)
    }

    captureDiagnostic("abi.result.available")
    switch box.take() {
    case let .success(result):
        resultPointer.pointee.image_format = UInt32(DG_CAPTURE_IMAGE_JPEG)
        resultPointer.pointee.captured_at_unix_ns = result.capturedAtUnixNS
        resultPointer.pointee.file_size = result.fileSize
        resultPointer.pointee.width = UInt32(result.width)
        resultPointer.pointee.height = UInt32(result.height)
        captureDiagnostic("abi.success")
        return Int32(DG_CAPTURE_OK)
    case let .failure(failure):
        captureDiagnostic("abi.failure.\(failure)")
        return writeFailure(failure, to: errorPointer)
    }
}

private func decodeRequest(_ request: dg_capture_request_v1) throws -> ScreenshotRequest {
    let knownFlags = UInt32(DG_CAPTURE_SHOWS_CURSOR)
    guard request.flags & ~knownFlags == 0,
          request.image_format == UInt32(DG_CAPTURE_IMAGE_JPEG)
    else {
        throw ScreenshotFailure.unsupported
    }
    guard request.target_height >= 1,
          request.target_height <= 16_384,
          request.jpeg_quality >= 1,
          request.jpeg_quality <= 100,
          request.timeout_ms >= 1,
          request.timeout_ms <= 60_000,
          request.blocked_application_id_count <= maximumApplicationIDs,
          request.reserved0 == 0
    else {
        throw ScreenshotFailure.invalidArgument
    }

    let outputPath = try decodeUTF8(
        request.output_path,
        maximumBytes: maximumPathBytes,
        allowEmpty: false
    )
    guard (outputPath as NSString).isAbsolutePath,
          ["jpg", "jpeg"].contains(URL(fileURLWithPath: outputPath).pathExtension.lowercased())
    else {
        throw ScreenshotFailure.invalidArgument
    }

    let count = Int(request.blocked_application_id_count)
    guard count == 0 || request.blocked_application_ids != nil else {
        throw ScreenshotFailure.invalidArgument
    }
    var totalBytes = 0
    var blockedApplicationIDs = Set<String>()
    if count > 0 {
        let views = UnsafeBufferPointer(start: request.blocked_application_ids, count: count)
        for view in views {
            let identifier = try decodeUTF8(
                view,
                maximumBytes: maximumApplicationIDBytes,
                allowEmpty: false
            )
            totalBytes += identifier.utf8.count
            guard totalBytes <= maximumApplicationIDTotalBytes else {
                throw ScreenshotFailure.invalidArgument
            }
            blockedApplicationIDs.insert(identifier)
        }
    }

    return ScreenshotRequest(
        outputPath: outputPath,
        targetHeight: Int(request.target_height),
        jpegQuality: Int(request.jpeg_quality),
        showsCursor: request.flags & knownFlags != 0,
        blockedApplicationIDs: blockedApplicationIDs,
        cancellation: CaptureCancellation()
    )
}

private func decodeUTF8(
    _ view: dg_capture_string_view_v1,
    maximumBytes: Int,
    allowEmpty: Bool
) throws -> String {
    guard view.len <= UInt64(maximumBytes), view.len <= UInt64(Int.max) else {
        throw ScreenshotFailure.invalidArgument
    }
    let count = Int(view.len)
    if count == 0 {
        guard allowEmpty else {
            throw ScreenshotFailure.invalidArgument
        }
        return ""
    }
    guard let data = view.data,
          let value = String(bytes: UnsafeBufferPointer(start: data, count: count), encoding: .utf8)
    else {
        throw ScreenshotFailure.invalidArgument
    }
    return value
}

private func writeFailure(
    _ failure: ScreenshotFailure,
    to errorPointer: UnsafeMutablePointer<dg_capture_error_v1>?
) -> Int32 {
    let status: Int32
    let domain: UInt32
    let nativeCode: Int64

    switch failure {
    case .blocked:
        status = Int32(DG_CAPTURE_BLOCKED)
        domain = UInt32(DG_CAPTURE_NATIVE_NONE)
        nativeCode = 0
    case .invalidArgument:
        status = Int32(DG_CAPTURE_E_INVALID_ARGUMENT)
        domain = UInt32(DG_CAPTURE_NATIVE_NONE)
        nativeCode = 0
    case .unsupported:
        status = Int32(DG_CAPTURE_E_UNSUPPORTED)
        domain = UInt32(DG_CAPTURE_NATIVE_NONE)
        nativeCode = 0
    case .permissionDenied:
        status = Int32(DG_CAPTURE_E_PERMISSION_DENIED)
        domain = UInt32(DG_CAPTURE_NATIVE_NONE)
        nativeCode = 0
    case .privacyUnsupported:
        status = Int32(DG_CAPTURE_E_PRIVACY_UNSUPPORTED)
        domain = UInt32(DG_CAPTURE_NATIVE_NONE)
        nativeCode = 0
    case .noDisplay:
        status = Int32(DG_CAPTURE_E_NO_DISPLAY)
        domain = UInt32(DG_CAPTURE_NATIVE_NONE)
        nativeCode = 0
    case .timeout:
        status = Int32(DG_CAPTURE_E_TIMEOUT)
        domain = UInt32(DG_CAPTURE_NATIVE_NONE)
        nativeCode = 0
    case let .io(code):
        status = Int32(DG_CAPTURE_E_IO)
        domain = UInt32(DG_CAPTURE_NATIVE_POSIX)
        nativeCode = Int64(code)
    case let .native(code):
        status = Int32(DG_CAPTURE_E_INTERNAL)
        domain = UInt32(DG_CAPTURE_NATIVE_APPLE)
        nativeCode = code
    }

    if let errorPointer {
        errorPointer.pointee.native_domain = domain
        errorPointer.pointee.native_code = nativeCode
    }
    return status
}

private func zeroResult(
    _ pointer: UnsafeMutablePointer<dg_capture_result_v1>,
    preservingSize size: UInt32
) {
    pointer.pointee.image_format = 0
    pointer.pointee.captured_at_unix_ns = 0
    pointer.pointee.file_size = 0
    pointer.pointee.width = 0
    pointer.pointee.height = 0
    pointer.pointee.struct_size = size
}

private func zeroError(
    _ pointer: UnsafeMutablePointer<dg_capture_error_v1>,
    preservingSize size: UInt32
) {
    pointer.pointee.native_domain = UInt32(DG_CAPTURE_NATIVE_NONE)
    pointer.pointee.native_code = 0
    pointer.pointee.struct_size = size
}
