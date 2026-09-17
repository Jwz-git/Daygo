import AVFoundation
import CoreGraphics
import CoreMedia
import CoreVideo
import Foundation
import ImageIO
import UniformTypeIdentifiers
import VideoToolbox

final class SegmentReader: @unchecked Sendable {
    static let shared = SegmentReader()

    private let lock = NSLock()

    private struct CachedAsset {
        let url: URL
        let asset: AVURLAsset
        let track: AVAssetTrack
        var lastAccessed: Date
    }

    private var cache: [String: CachedAsset] = [:]
    private let maxCacheEntries = 4

    func decodeFrame(
        recordingsDir: String,
        segmentRelPath: String,
        frameIndex: Int,
        maxPixelSize: Int
    ) throws -> Data {
        let recordingsURL = URL(fileURLWithPath: recordingsDir)
        let fileURL = recordingsURL.appendingPathComponent(segmentRelPath)
        let fullPath = fileURL.path

        guard FileManager.default.fileExists(atPath: fullPath) else {
            throw ScreenshotFailure.io(ENOENT)
        }

        let ext = fileURL.pathExtension.lowercased()

        // Legacy JPEG staging fallback
        if ext == "jpg" || ext == "jpeg" {
            let data = try Data(contentsOf: fileURL)
            if maxPixelSize <= 0 {
                return data
            }
            let sourceOptions = [kCGImageSourceShouldCache: false] as CFDictionary
            guard let source = CGImageSourceCreateWithData(data as CFData, sourceOptions) else {
                return data
            }
            let downsampleOptions = [
                kCGImageSourceCreateThumbnailFromImageAlways: true,
                kCGImageSourceShouldCacheImmediately: true,
                kCGImageSourceCreateThumbnailWithTransform: true,
                kCGImageSourceThumbnailMaxPixelSize: maxPixelSize,
            ] as CFDictionary
            guard let thumbnail = CGImageSourceCreateThumbnailAtIndex(source, 0, downsampleOptions) else {
                return data
            }
            return try encodeCGImageToJPEG(thumbnail)
        }

        // Active segment cannot be read until finalized
        if SegmentWriter.shared.isActiveSegment(fullPath: fullPath) {
            throw ScreenshotFailure.unsupported
        }

        let cached = try getOrCreateCachedAsset(for: fileURL)
        let asset = cached.asset
        let track = cached.track

        let reader = try AVAssetReader(asset: asset)
        let outputSettings: [String: Any] = [
            kCVPixelBufferPixelFormatTypeKey as String: Int(kCVPixelFormatType_32BGRA),
        ]
        let trackOutput = AVAssetReaderTrackOutput(track: track, outputSettings: outputSettings)
        trackOutput.alwaysCopiesSampleData = false

        guard reader.canAdd(trackOutput) else {
            throw ScreenshotFailure.unsupported
        }
        reader.add(trackOutput)

        let targetTime = CMTime(value: CMTimeValue(frameIndex), timescale: 1)
        reader.timeRange = CMTimeRange(start: targetTime, duration: CMTime(value: 1, timescale: 1))

        guard reader.startReading() else {
            let err = reader.error as? NSError
            throw ScreenshotFailure.native(Int64(err?.code ?? Int(EIO)))
        }

        guard let sampleBuffer = trackOutput.copyNextSampleBuffer(),
              let pixelBuffer = CMSampleBufferGetImageBuffer(sampleBuffer) else {
            throw ScreenshotFailure.io(ENOENT)
        }

        var cgImage: CGImage?
        let status = VTCreateCGImageFromCVPixelBuffer(pixelBuffer, options: nil, imageOut: &cgImage)
        guard status == noErr, var image = cgImage else {
            throw ScreenshotFailure.io(Int32(status))
        }

        if maxPixelSize > 0 {
            image = downscale(image: image, maxPixelSize: maxPixelSize)
        }

        return try encodeCGImageToJPEG(image)
    }

    func probe(
        recordingsDir: String,
        segmentRelPath: String
    ) -> (frameCount: Int, width: Int, height: Int, readable: Bool) {
        let recordingsURL = URL(fileURLWithPath: recordingsDir)
        let fileURL = recordingsURL.appendingPathComponent(segmentRelPath)
        let fullPath = fileURL.path

        guard FileManager.default.fileExists(atPath: fullPath) else {
            return (0, 0, 0, false)
        }

        let ext = fileURL.pathExtension.lowercased()
        if ext == "jpg" || ext == "jpeg" {
            guard let source = CGImageSourceCreateWithURL(fileURL as CFURL, nil),
                  let properties = CGImageSourceCopyPropertiesAtIndex(source, 0, nil) as? [CFString: Any],
                  let width = properties[kCGImagePropertyPixelWidth] as? Int,
                  let height = properties[kCGImagePropertyPixelHeight] as? Int else {
                return (0, 0, 0, false)
            }
            return (1, width, height, true)
        }

        if ext == "mp4" {
            if SegmentWriter.shared.isActiveSegment(fullPath: fullPath) {
                return (0, 0, 0, false)
            }
            let asset = AVURLAsset(url: fileURL)
            guard let track = asset.tracks(withMediaType: .video).first else {
                return (0, 0, 0, false)
            }
            let durationSeconds = asset.duration.seconds
            if durationSeconds.isNaN || durationSeconds <= 0 {
                return (0, 0, 0, false)
            }
            let frameCount = max(1, Int(durationSeconds.rounded()))
            let size = track.naturalSize
            return (frameCount, Int(size.width), Int(size.height), true)
        }

        return (0, 0, 0, false)
    }

    private func getOrCreateCachedAsset(for fileURL: URL) throws -> CachedAsset {
        lock.lock()
        defer { lock.unlock() }

        let path = fileURL.path
        if var existing = cache[path] {
            existing.lastAccessed = Date()
            cache[path] = existing
            return existing
        }

        let asset = AVURLAsset(url: fileURL)
        guard let track = asset.tracks(withMediaType: .video).first else {
            throw ScreenshotFailure.io(ENOENT)
        }

        let cached = CachedAsset(
            url: fileURL,
            asset: asset,
            track: track,
            lastAccessed: Date()
        )

        if cache.count >= maxCacheEntries {
            if let oldest = cache.min(by: { $0.value.lastAccessed < $1.value.lastAccessed }) {
                cache.removeValue(forKey: oldest.key)
            }
        }

        cache[path] = cached
        return cached
    }
}

func downscale(image: CGImage, maxPixelSize: Int) -> CGImage {
    let width = image.width
    let height = image.height
    let maxDim = max(width, height)
    if maxDim <= maxPixelSize {
        return image
    }
    let scale = Double(maxPixelSize) / Double(maxDim)
    let newWidth = max(1, Int((Double(width) * scale).rounded()))
    let newHeight = max(1, Int((Double(height) * scale).rounded()))
    let colorSpace = image.colorSpace ?? CGColorSpaceCreateDeviceRGB()
    let bitmapInfo = CGBitmapInfo.byteOrder32Little.rawValue | CGImageAlphaInfo.premultipliedFirst.rawValue
    guard let context = CGContext(
        data: nil,
        width: newWidth,
        height: newHeight,
        bitsPerComponent: 8,
        bytesPerRow: 0,
        space: colorSpace,
        bitmapInfo: bitmapInfo
    ) else {
        return image
    }
    context.interpolationQuality = .high
    context.draw(image, in: CGRect(x: 0, y: 0, width: newWidth, height: newHeight))
    return context.makeImage() ?? image
}

func encodeCGImageToJPEG(_ image: CGImage, quality: Double = 0.85) throws -> Data {
    let data = NSMutableData()
    guard let destination = CGImageDestinationCreateWithData(
        data as CFMutableData,
        UTType.jpeg.identifier as CFString,
        1,
        nil
    ) else {
        throw ScreenshotFailure.io(EIO)
    }
    let properties = [
        kCGImageDestinationLossyCompressionQuality as String: quality
    ] as CFDictionary
    CGImageDestinationAddImage(destination, image, properties)
    guard CGImageDestinationFinalize(destination) else {
        throw ScreenshotFailure.io(EIO)
    }
    return data as Data
}
