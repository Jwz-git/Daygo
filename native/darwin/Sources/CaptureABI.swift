import Dispatch
import Foundation

private let maximumPathBytes = 32_768
private let maximumApplicationIDBytes = 4_096
private let maximumApplicationIDs = 4_096
private let maximumApplicationIDTotalBytes = 1 << 20

final class BlockingCaptureResultBox<T: Sendable>: @unchecked Sendable {
    private let lock = NSLock()
    private let semaphore = DispatchSemaphore(value: 0)
    private var result: Result<T, ScreenshotFailure>?

    func complete(_ result: Result<T, ScreenshotFailure>) {
        lock.lock()
        self.result = result
        lock.unlock()
        semaphore.signal()
    }

    func wait(until deadline: DispatchTime) -> Bool {
        semaphore.wait(timeout: deadline) == .success
    }

    func take() -> Result<T, ScreenshotFailure> {
        lock.lock()
        defer { lock.unlock() }
        return result!
    }
}
private typealias BlockingCaptureResult = BlockingCaptureResultBox<ScreenshotResult>

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

private func zeroAppendResult(
    _ pointer: UnsafeMutablePointer<dg_frame_append_result_v1>,
    preservingSize size: UInt32
) {
    pointer.pointee.outcome = 0
    pointer.pointee.frame_index = 0
    pointer.pointee.width = 0
    pointer.pointee.height = 0
    pointer.pointee.reserved0 = 0
    pointer.pointee.captured_at_unix_ns = 0
    pointer.pointee.file_size = 0
    pointer.pointee.struct_size = size
    _ = withUnsafeMutableBytes(of: &pointer.pointee.segment_rel_path) { ptr in
        ptr.initializeMemory(as: UInt8.self, repeating: 0)
    }
}

private func copyString<T>(_ source: String, to destination: inout T) {
    withUnsafeMutableBytes(of: &destination) { ptr in
        ptr.initializeMemory(as: UInt8.self, repeating: 0)
        let utf8 = source.utf8
        let count = min(utf8.count, ptr.count - 1)
        for (index, byte) in utf8.prefix(count).enumerated() {
            ptr[index] = byte
        }
    }
}

private func decodeBlockedApplicationIDs(
    count: Int,
    pointer: UnsafePointer<dg_capture_string_view_v1>?
) throws -> Set<String> {
    guard count == 0 || pointer != nil else {
        throw ScreenshotFailure.invalidArgument
    }
    var totalBytes = 0
    var ids = Set<String>()
    if count > 0, let pointer {
        let views = UnsafeBufferPointer(start: pointer, count: count)
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
            ids.insert(identifier)
        }
    }
    return ids
}

private struct FrameAppendResult: Sendable {
    let outcome: UInt32
    let frameIndex: Int
    let width: Int
    let height: Int
    let capturedAtUnixNS: Int64
    let fileSize: UInt64
    let segmentRelPath: String
}

@_cdecl("dg_frame_append")
func dg_frame_append(
    _ requestedABIMajor: UInt32,
    _ requestPointer: UnsafePointer<dg_frame_append_request_v1>?,
    _ resultPointer: UnsafeMutablePointer<dg_frame_append_result_v1>?,
    _ errorPointer: UnsafeMutablePointer<dg_capture_error_v1>?
) -> Int32 {
    guard let requestPointer, let resultPointer else {
        return Int32(DG_CAPTURE_E_INVALID_ARGUMENT)
    }

    let suppliedResultSize = resultPointer.pointee.struct_size
    zeroAppendResult(resultPointer, preservingSize: suppliedResultSize)
    if let errorPointer {
        let suppliedErrorSize = errorPointer.pointee.struct_size
        zeroError(errorPointer, preservingSize: suppliedErrorSize)
    }

    guard requestedABIMajor == UInt32(DG_CAPTURE_ABI_MAJOR),
          requestPointer.pointee.struct_size >= MemoryLayout<dg_frame_append_request_v1>.size,
          suppliedResultSize >= MemoryLayout<dg_frame_append_result_v1>.size,
          errorPointer == nil || errorPointer!.pointee.struct_size >= MemoryLayout<dg_capture_error_v1>.size
    else {
        return Int32(DG_CAPTURE_E_ABI_MISMATCH)
    }

    let recordingsDir: String
    let blockedIDs: Set<String>
    do {
        recordingsDir = try decodeUTF8(requestPointer.pointee.recordings_dir, maximumBytes: maximumPathBytes, allowEmpty: false)
        guard (recordingsDir as NSString).isAbsolutePath else {
            throw ScreenshotFailure.invalidArgument
        }
        blockedIDs = try decodeBlockedApplicationIDs(
            count: Int(requestPointer.pointee.blocked_application_id_count),
            pointer: requestPointer.pointee.blocked_application_ids
        )
    } catch let failure as ScreenshotFailure {
        return writeFailure(failure, to: errorPointer)
    } catch {
        return Int32(DG_CAPTURE_E_INVALID_ARGUMENT)
    }

    // Synthetic frame path (for native smoke tests)
    if requestPointer.pointee.synthetic_width > 0 && requestPointer.pointee.synthetic_height > 0 {
        let width = Int(requestPointer.pointee.synthetic_width)
        let height = Int(requestPointer.pointee.synthetic_height)
        guard let img = createSyntheticImage(width: width, height: height, frameIndex: 0) else {
            return writeFailure(.invalidArgument, to: errorPointer)
        }
        let now = Date()
        do {
            let res = try SegmentWriter.shared.append(image: img, capturedAt: now, recordingsDir: recordingsDir)
            resultPointer.pointee.outcome = UInt32(DG_CAPTURE_OK)
            resultPointer.pointee.frame_index = UInt32(res.frameIndex)
            resultPointer.pointee.width = UInt32(res.width)
            resultPointer.pointee.height = UInt32(res.height)
            resultPointer.pointee.captured_at_unix_ns = Int64((now.timeIntervalSince1970 * 1_000_000_000).rounded())
            resultPointer.pointee.file_size = res.fileSize
            copyString(res.segmentRelPath, to: &resultPointer.pointee.segment_rel_path)
            return Int32(DG_CAPTURE_OK)
        } catch let failure as ScreenshotFailure {
            return writeFailure(failure, to: errorPointer)
        } catch {
            return Int32(DG_CAPTURE_E_IO)
        }
    }

    // Real screen capture path
    let targetHeight = Int(requestPointer.pointee.target_height)
    guard targetHeight >= 1 && targetHeight <= 16_384 else {
        return writeFailure(.invalidArgument, to: errorPointer)
    }
    let showsCursor = requestPointer.pointee.flags & UInt32(DG_CAPTURE_SHOWS_CURSOR) != 0

    let box = BlockingCaptureResultBox<FrameAppendResult>()
    let task = Task.detached(priority: .userInitiated) {
        do {
            if !blockedIDs.isEmpty {
                let isBlocked = try frontmostApplicationIsBlocked(blockedIDs)
                if isBlocked {
                    let targetWidth = max(1, targetHeight * 16 / 9)
                    guard let placeholder = createPlaceholderImage(width: targetWidth, height: targetHeight) else {
                        box.complete(.failure(.invalidArgument))
                        return
                    }
                    let now = Date()
                    try Task.checkCancellation()
                    let res = try SegmentWriter.shared.append(image: placeholder, capturedAt: now, recordingsDir: recordingsDir)
                    box.complete(.success(FrameAppendResult(
                        outcome: UInt32(DG_CAPTURE_BLOCKED),
                        frameIndex: res.frameIndex,
                        width: res.width,
                        height: res.height,
                        capturedAtUnixNS: Int64((now.timeIntervalSince1970 * 1_000_000_000).rounded()),
                        fileSize: res.fileSize,
                        segmentRelPath: res.segmentRelPath
                    )))
                    return
                }
            }

            let (image, startedAt, finishedAt) = try await capturePrimaryDisplayCGImage(
                targetHeight: targetHeight,
                showsCursor: showsCursor,
                blockedApplicationIDs: blockedIDs
            )
            // The ABI may have timed out and cancelled us while the system
            // screenshot call — which ignores cooperative cancellation — ran to
            // completion. Don't append a frame the Go side already abandoned: it
            // would land out of order and keep this task's image alive across the
            // write.
            try Task.checkCancellation()
            let res = try SegmentWriter.shared.append(image: image, capturedAt: startedAt, recordingsDir: recordingsDir)
            let midpoint = startedAt.timeIntervalSince1970 + finishedAt.timeIntervalSince(startedAt) / 2
            box.complete(.success(FrameAppendResult(
                outcome: UInt32(DG_CAPTURE_OK),
                frameIndex: res.frameIndex,
                width: res.width,
                height: res.height,
                capturedAtUnixNS: Int64((midpoint * 1_000_000_000).rounded()),
                fileSize: res.fileSize,
                segmentRelPath: res.segmentRelPath
            )))
        } catch let failure as ScreenshotFailure {
            box.complete(.failure(failure))
        } catch {
            box.complete(.failure(.native(Int64((error as NSError).code))))
        }
    }

    let timeoutMs = requestPointer.pointee.timeout_ms > 0 ? Int(requestPointer.pointee.timeout_ms) : 10_000
    let timeout = DispatchTime.now() + .milliseconds(timeoutMs)
    if !box.wait(until: timeout) {
        task.cancel()
        return writeFailure(.timeout, to: errorPointer)
    }

    switch box.take() {
    case let .success(res):
        resultPointer.pointee.outcome = res.outcome
        resultPointer.pointee.frame_index = UInt32(res.frameIndex)
        resultPointer.pointee.width = UInt32(res.width)
        resultPointer.pointee.height = UInt32(res.height)
        resultPointer.pointee.captured_at_unix_ns = res.capturedAtUnixNS
        resultPointer.pointee.file_size = res.fileSize
        copyString(res.segmentRelPath, to: &resultPointer.pointee.segment_rel_path)
        return res.outcome == UInt32(DG_CAPTURE_BLOCKED) ? Int32(DG_CAPTURE_BLOCKED) : Int32(DG_CAPTURE_OK)
    case let .failure(failure):
        return writeFailure(failure, to: errorPointer)
    }
}

@_cdecl("dg_frame_decode")
func dg_frame_decode(
    _ recordingsDirView: dg_capture_string_view_v1,
    _ segmentRelPathView: dg_capture_string_view_v1,
    _ frameIndex: UInt32,
    _ maxPixelSize: UInt32,
    _ outData: UnsafeMutablePointer<UnsafeMutablePointer<UInt8>?>?,
    _ outLen: UnsafeMutablePointer<UInt64>?
) -> Int32 {
    guard let outData, let outLen else {
        return Int32(DG_CAPTURE_E_INVALID_ARGUMENT)
    }
    outData.pointee = nil
    outLen.pointee = 0

    do {
        let recordingsDir = try decodeUTF8(recordingsDirView, maximumBytes: maximumPathBytes, allowEmpty: false)
        let segmentRelPath = try decodeUTF8(segmentRelPathView, maximumBytes: maximumPathBytes, allowEmpty: false)
        let data = try SegmentReader.shared.decodeFrame(
            recordingsDir: recordingsDir,
            segmentRelPath: segmentRelPath,
            frameIndex: Int(frameIndex),
            maxPixelSize: Int(maxPixelSize)
        )
        let buffer = UnsafeMutablePointer<UInt8>.allocate(capacity: data.count)
        data.copyBytes(to: buffer, count: data.count)
        outData.pointee = buffer
        outLen.pointee = UInt64(data.count)
        return Int32(DG_CAPTURE_OK)
    } catch let failure as ScreenshotFailure {
        return writeFailure(failure, to: nil)
    } catch {
        return Int32(DG_CAPTURE_E_IO)
    }
}

@_cdecl("dg_frame_free")
func dg_frame_free(_ data: UnsafeMutablePointer<UInt8>?) {
    data?.deallocate()
}

@_cdecl("dg_segment_probe")
func dg_segment_probe(
    _ recordingsDirView: dg_capture_string_view_v1,
    _ segmentRelPathView: dg_capture_string_view_v1,
    _ outInfo: UnsafeMutablePointer<dg_segment_info_v1>?
) -> Int32 {
    guard let outInfo else {
        return Int32(DG_CAPTURE_E_INVALID_ARGUMENT)
    }
    outInfo.pointee.frame_count = 0
    outInfo.pointee.width = 0
    outInfo.pointee.height = 0
    outInfo.pointee.readable = 0

    do {
        let recordingsDir = try decodeUTF8(recordingsDirView, maximumBytes: maximumPathBytes, allowEmpty: false)
        let segmentRelPath = try decodeUTF8(segmentRelPathView, maximumBytes: maximumPathBytes, allowEmpty: false)
        let probe = SegmentReader.shared.probe(recordingsDir: recordingsDir, segmentRelPath: segmentRelPath)
        outInfo.pointee.frame_count = UInt32(probe.frameCount)
        outInfo.pointee.width = UInt32(probe.width)
        outInfo.pointee.height = UInt32(probe.height)
        outInfo.pointee.readable = probe.readable ? 1 : 0
        return Int32(DG_CAPTURE_OK)
    } catch {
        return Int32(DG_CAPTURE_E_INVALID_ARGUMENT)
    }
}

@_cdecl("dg_segment_close_active")
func dg_segment_close_active() -> Int32 {
    do {
        try SegmentWriter.shared.finishActive()
        return Int32(DG_CAPTURE_OK)
    } catch {
        return Int32(DG_CAPTURE_E_INTERNAL)
    }
}

