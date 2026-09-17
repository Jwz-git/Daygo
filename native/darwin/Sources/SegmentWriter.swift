@preconcurrency import AVFoundation
import CoreGraphics
import CoreMedia
import CoreVideo
import Foundation
import VideoToolbox

final class SegmentWriter: @unchecked Sendable {
    static let shared = SegmentWriter()

    private let lock = NSLock()

    private struct ActiveSegment: @unchecked Sendable {
        let writer: AVAssetWriter
        let input: AVAssetWriterInput
        let adaptor: AVAssetWriterInputPixelBufferAdaptor
        let segmentRelPath: String
        let fullPath: String
        let startedAt: Date
        let width: Int
        let height: Int
        var frameCount: Int
    }

    private var active: ActiveSegment?

    func isActiveSegment(fullPath: String) -> Bool {
        lock.lock()
        defer { lock.unlock() }
        guard let active else { return false }
        return active.fullPath == fullPath
    }

    func finishActive() throws {
        lock.lock()
        guard let current = active else {
            lock.unlock()
            return
        }
        active = nil
        lock.unlock()

        try finishSegment(current)
    }

    func append(
        image: CGImage,
        capturedAt: Date,
        recordingsDir: String
    ) throws -> (segmentRelPath: String, frameIndex: Int, fileSize: UInt64, width: Int, height: Int) {
        lock.lock()
        defer { lock.unlock() }

        let width = image.width
        let height = image.height

        // Check if active segment must roll over:
        // 1. Resolution change
        // 2. 600 frames reached
        // 3. 600 seconds reached
        if let current = active {
            let resolutionChanged = current.width != width || current.height != height
            let frameLimitReached = current.frameCount >= 600
            let timeLimitReached = capturedAt.timeIntervalSince(current.startedAt) >= 600

            if resolutionChanged || frameLimitReached || timeLimitReached {
                active = nil
                try finishSegment(current)
            }
        }

        // Start new segment if none active
        if active == nil {
            active = try startNewSegment(recordingsDir: recordingsDir, width: width, height: height, startedAt: capturedAt)
        }

        guard var current = active else {
            throw ScreenshotFailure.io(EIO)
        }

        let frameIndex = current.frameCount
        let presentationTime = CMTime(value: CMTimeValue(frameIndex), timescale: 1)

        let pixelBuffer = try createPixelBuffer(from: image, pool: current.adaptor.pixelBufferPool, width: width, height: height)

        var retry = 0
        while !current.input.isReadyForMoreMediaData && retry < 200 {
            Thread.sleep(forTimeInterval: 0.005)
            retry += 1
        }

        guard current.adaptor.append(pixelBuffer, withPresentationTime: presentationTime) else {
            let err = current.writer.error as? NSError
            throw ScreenshotFailure.native(Int64(err?.code ?? Int(EIO)))
        }

        current.frameCount += 1
        active = current

        let fileSize = (try? FileManager.default.attributesOfItem(atPath: current.fullPath)[.size] as? NSNumber)?.uint64Value ?? 0

        return (current.segmentRelPath, frameIndex, fileSize, width, height)
    }

    private func startNewSegment(recordingsDir: String, width: Int, height: Int, startedAt: Date) throws -> ActiveSegment {
        let segmentsDir = URL(fileURLWithPath: recordingsDir).appendingPathComponent("segments")
        try FileManager.default.createDirectory(at: segmentsDir, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])

        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = TimeZone(secondsFromGMT: 0)
        formatter.dateFormat = "yyyyMMdd-HHmmss"
        let baseName = formatter.string(from: startedAt)

        var fileName = "\(baseName).mp4"
        var targetURL = segmentsDir.appendingPathComponent(fileName)
        var counter = 1
        while FileManager.default.fileExists(atPath: targetURL.path) {
            fileName = "\(baseName)-\(counter).mp4"
            targetURL = segmentsDir.appendingPathComponent(fileName)
            counter += 1
        }

        let writer = try AVAssetWriter(url: targetURL, fileType: .mp4)
        let compressionProperties: [String: Any] = [
            AVVideoQualityKey: 0.55,
            AVVideoMaxKeyFrameIntervalKey: 30,
        ]

        let videoSettings: [String: Any] = [
            AVVideoCodecKey: AVVideoCodecType.hevc,
            AVVideoWidthKey: width,
            AVVideoHeightKey: height,
            AVVideoCompressionPropertiesKey: compressionProperties,
        ]

        let input = AVAssetWriterInput(mediaType: .video, outputSettings: videoSettings)
        input.expectsMediaDataInRealTime = false

        let sourcePixelBufferAttributes: [String: Any] = [
            kCVPixelBufferPixelFormatTypeKey as String: Int(kCVPixelFormatType_32BGRA),
            kCVPixelBufferWidthKey as String: width,
            kCVPixelBufferHeightKey as String: height,
        ]

        let adaptor = AVAssetWriterInputPixelBufferAdaptor(
            assetWriterInput: input,
            sourcePixelBufferAttributes: sourcePixelBufferAttributes
        )

        guard writer.canAdd(input) else {
            throw ScreenshotFailure.unsupported
        }
        writer.add(input)

        guard writer.startWriting() else {
            let err = writer.error as? NSError
            throw ScreenshotFailure.native(Int64(err?.code ?? Int(EIO)))
        }

        writer.startSession(atSourceTime: .zero)

        let segmentRelPath = "segments/\(fileName)"
        return ActiveSegment(
            writer: writer,
            input: input,
            adaptor: adaptor,
            segmentRelPath: segmentRelPath,
            fullPath: targetURL.path,
            startedAt: startedAt,
            width: width,
            height: height,
            frameCount: 0
        )
    }

    private func finishSegment(_ segment: ActiveSegment) throws {
        segment.input.markAsFinished()

        let writer = segment.writer
        let box = BlockingCaptureResultBox<Void>()
        writer.finishWriting {
            if writer.status == .failed {
                let code = Int64((writer.error as NSError?)?.code ?? 0)
                box.complete(.failure(.native(code)))
            } else {
                box.complete(.success(()))
            }
        }

        _ = box.wait(until: .distantFuture)
        switch box.take() {
        case .success:
            break
        case .failure(let error):
            throw error
        }
    }
}

func createPixelBuffer(from image: CGImage, pool: CVPixelBufferPool?, width: Int, height: Int) throws -> CVPixelBuffer {
    var pixelBufferOut: CVPixelBuffer?
    if let pool = pool {
        let status = CVPixelBufferPoolCreatePixelBuffer(kCFAllocatorDefault, pool, &pixelBufferOut)
        if status != kCVReturnSuccess {
            pixelBufferOut = nil
        }
    }
    if pixelBufferOut == nil {
        let attributes: [CFString: Any] = [
            kCVPixelBufferCGImageCompatibilityKey: true,
            kCVPixelBufferCGBitmapContextCompatibilityKey: true,
            kCVPixelBufferWidthKey: width,
            kCVPixelBufferHeightKey: height,
        ]
        let status = CVPixelBufferCreate(
            kCFAllocatorDefault,
            width,
            height,
            kCVPixelFormatType_32BGRA,
            attributes as CFDictionary,
            &pixelBufferOut
        )
        guard status == kCVReturnSuccess, let _ = pixelBufferOut else {
            throw ScreenshotFailure.io(Int32(status))
        }
    }
    guard let pixelBuffer = pixelBufferOut else {
        throw ScreenshotFailure.io(EIO)
    }

    CVPixelBufferLockBaseAddress(pixelBuffer, [])
    defer { CVPixelBufferUnlockBaseAddress(pixelBuffer, []) }

    guard let baseAddress = CVPixelBufferGetBaseAddress(pixelBuffer) else {
        throw ScreenshotFailure.io(EIO)
    }
    let bytesPerRow = CVPixelBufferGetBytesPerRow(pixelBuffer)
    let colorSpace = CGColorSpaceCreateDeviceRGB()
    let bitmapInfo = CGBitmapInfo.byteOrder32Little.rawValue | CGImageAlphaInfo.premultipliedFirst.rawValue

    guard let context = CGContext(
        data: baseAddress,
        width: width,
        height: height,
        bitsPerComponent: 8,
        bytesPerRow: bytesPerRow,
        space: colorSpace,
        bitmapInfo: bitmapInfo
    ) else {
        throw ScreenshotFailure.io(EIO)
    }

    context.draw(image, in: CGRect(x: 0, y: 0, width: width, height: height))
    return pixelBuffer
}

func createPlaceholderImage(width: Int, height: Int) -> CGImage? {
    let colorSpace = CGColorSpaceCreateDeviceRGB()
    let bitmapInfo = CGBitmapInfo.byteOrder32Little.rawValue | CGImageAlphaInfo.premultipliedFirst.rawValue
    guard let context = CGContext(
        data: nil,
        width: width,
        height: height,
        bitsPerComponent: 8,
        bytesPerRow: 0,
        space: colorSpace,
        bitmapInfo: bitmapInfo
    ) else {
        return nil
    }
    context.setFillColor(CGColor(red: 32.0 / 255.0, green: 32.0 / 255.0, blue: 32.0 / 255.0, alpha: 1.0))
    context.fill(CGRect(x: 0, y: 0, width: width, height: height))
    return context.makeImage()
}

func createSyntheticImage(width: Int, height: Int, frameIndex: Int = 0) -> CGImage? {
    let colorSpace = CGColorSpaceCreateDeviceRGB()
    let bitmapInfo = CGBitmapInfo.byteOrder32Little.rawValue | CGImageAlphaInfo.premultipliedFirst.rawValue
    guard let context = CGContext(
        data: nil,
        width: width,
        height: height,
        bitsPerComponent: 8,
        bytesPerRow: 0,
        space: colorSpace,
        bitmapInfo: bitmapInfo
    ) else {
        return nil
    }
    let r = CGFloat((frameIndex * 37) % 255) / 255.0
    let g = CGFloat((frameIndex * 73) % 255) / 255.0
    let b = CGFloat((frameIndex * 109) % 255) / 255.0
    context.setFillColor(CGColor(red: r, green: g, blue: b, alpha: 1.0))
    context.fill(CGRect(x: 0, y: 0, width: width, height: height))
    context.setFillColor(CGColor(red: 1.0 - r, green: 1.0 - g, blue: 1.0 - b, alpha: 1.0))
    context.fill(CGRect(x: width / 4, y: height / 4, width: width / 2, height: height / 2))
    return context.makeImage()
}
