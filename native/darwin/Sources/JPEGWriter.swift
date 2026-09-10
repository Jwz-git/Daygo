import CoreGraphics
import Darwin
import Foundation
import ImageIO
import UniformTypeIdentifiers

func writeJPEGAtomically(
    _ image: CGImage,
    outputPath: String,
    quality: Int,
    cancellation: CaptureCancellation
) throws -> UInt64 {
    let fileManager = FileManager.default
    let outputURL = URL(fileURLWithPath: outputPath)
    let directoryURL = outputURL.deletingLastPathComponent()
    let temporaryURL = directoryURL.appendingPathComponent(
        ".\(outputURL.lastPathComponent).daygo-\(UUID().uuidString).partial"
    )

    guard fileManager.createFile(
        atPath: temporaryURL.path,
        contents: nil,
        attributes: [.posixPermissions: NSNumber(value: 0o600)]
    ) else {
        throw ScreenshotFailure.io(errno)
    }
    var temporaryExists = true
    defer {
        if temporaryExists {
            try? fileManager.removeItem(at: temporaryURL)
        }
    }

    guard let destination = CGImageDestinationCreateWithURL(
        temporaryURL as CFURL,
        UTType.jpeg.identifier as CFString,
        1,
        nil
    ) else {
        throw ScreenshotFailure.io(EIO)
    }
    let properties = [
        kCGImageDestinationLossyCompressionQuality as String: Double(quality) / 100
    ] as CFDictionary
    CGImageDestinationAddImage(destination, image, properties)
    guard CGImageDestinationFinalize(destination) else {
        throw ScreenshotFailure.io(EIO)
    }

    try cancellation.publish {
        if Darwin.link(temporaryURL.path, outputURL.path) != 0 {
            throw ScreenshotFailure.io(errno)
        }
    }
    if Darwin.unlink(temporaryURL.path) != 0 {
        let failure = errno
        _ = Darwin.unlink(outputURL.path)
        throw ScreenshotFailure.io(failure)
    }
    temporaryExists = false

    do {
        let attributes = try fileManager.attributesOfItem(atPath: outputURL.path)
        guard let fileSize = attributes[.size] as? NSNumber else {
            _ = Darwin.unlink(outputURL.path)
            throw ScreenshotFailure.io(EIO)
        }
        return fileSize.uint64Value
    } catch let failure as ScreenshotFailure {
        throw failure
    } catch {
        _ = Darwin.unlink(outputURL.path)
        throw ScreenshotFailure.io(Int32((error as NSError).code))
    }
}
