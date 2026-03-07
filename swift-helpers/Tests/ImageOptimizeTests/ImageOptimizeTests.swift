import Foundation
import XCTest
@testable import asc_image_optimize

final class ImageOptimizeTests: XCTestCase {
    func testBatchOptimizeRecursivePreservesRelativePaths() throws {
        let tempDir = URL(fileURLWithPath: NSTemporaryDirectory())
            .appendingPathComponent(UUID().uuidString, isDirectory: true)
        let inputDir = tempDir.appendingPathComponent("input", isDirectory: true)
        let outputDir = tempDir.appendingPathComponent("output", isDirectory: true)

        try FileManager.default.createDirectory(at: inputDir.appendingPathComponent("en", isDirectory: true), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: inputDir.appendingPathComponent("fr", isDirectory: true), withIntermediateDirectories: true)

        defer { try? FileManager.default.removeItem(at: tempDir) }

        let pngData = Data(base64Encoded: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a9X8AAAAASUVORK5CYII=")!
        try pngData.write(to: inputDir.appendingPathComponent("en/screenshot.png"))
        try pngData.write(to: inputDir.appendingPathComponent("fr/screenshot.png"))

        let results = try batchOptimize(
            inputDir: inputDir.path,
            outputDir: outputDir.path,
            preset: .store,
            format: "png",
            recursive: true,
            parallel: true
        )

        XCTAssertEqual(results.count, 2)
        XCTAssertTrue(FileManager.default.fileExists(atPath: outputDir.appendingPathComponent("en/screenshot.png").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: outputDir.appendingPathComponent("fr/screenshot.png").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: outputDir.appendingPathComponent("screenshot.png").path))
    }

    func testBatchOptimizeUsesRequestedOutputExtension() throws {
        let tempDir = URL(fileURLWithPath: NSTemporaryDirectory())
            .appendingPathComponent(UUID().uuidString, isDirectory: true)
        let inputDir = tempDir.appendingPathComponent("input", isDirectory: true)
        let outputDir = tempDir.appendingPathComponent("output", isDirectory: true)

        try FileManager.default.createDirectory(at: inputDir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: tempDir) }

        let pngData = Data(base64Encoded: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a9X8AAAAASUVORK5CYII=")!
        try pngData.write(to: inputDir.appendingPathComponent("screenshot.png"))

        let results = try batchOptimize(
            inputDir: inputDir.path,
            outputDir: outputDir.path,
            preset: .preview,
            format: "jpeg",
            recursive: false,
            parallel: false
        )

        XCTAssertEqual(results.count, 1)
        XCTAssertTrue(FileManager.default.fileExists(atPath: outputDir.appendingPathComponent("screenshot.jpeg").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: outputDir.appendingPathComponent("screenshot.png").path))
    }
}
