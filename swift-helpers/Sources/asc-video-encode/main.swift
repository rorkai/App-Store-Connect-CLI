import ArgumentParser
import AVFoundation
import Foundation
import UniformTypeIdentifiers

// MARK: - Errors

enum VideoEncodeError: Error, LocalizedError {
    case invalidInput(String)
    case videoLoadFailed(String)
    case encodingFailed(String)
    case unsupportedFormat(String)
    case saveFailed(String)
    
    var errorDescription: String? {
        switch self {
        case .invalidInput(let detail):
            return "Invalid input: \(detail)"
        case .videoLoadFailed(let detail):
            return "Failed to load video: \(detail)"
        case .encodingFailed(let detail):
            return "Encoding failed: \(detail)"
        case .unsupportedFormat(let format):
            return "Unsupported format: \(format)"
        case .saveFailed(let detail):
            return "Failed to save: \(detail)"
        }
    }
}

// MARK: - Encoding Presets

enum VideoPreset: String, CaseIterable {
    case store = "store"          // App Store preview quality
    case preview = "preview"      // High quality, reasonable size
    case compact = "compact"      // Smaller size for web
    
    var bitRate: Int {
        switch self {
        case .store: return 6_000_000      // 6 Mbps
        case .preview: return 4_000_000   // 4 Mbps
        case .compact: return 2_000_000   // 2 Mbps
        }
    }
    
    var maxResolution: CGSize? {
        switch self {
        case .store: return nil
        case .preview: return CGSize(width: 1920, height: 1080)
        case .compact: return CGSize(width: 1280, height: 720)
        }
    }
}

// MARK: - Video Processing

func encodeVideo(
    inputPath: String,
    outputPath: String,
    preset: VideoPreset
) throws -> [String: Any] {
    let inputURL = URL(fileURLWithPath: inputPath)
    let outputURL = URL(fileURLWithPath: outputPath)
    
    // Create asset
    let asset = AVAsset(url: inputURL)
    
    guard let videoTrack = asset.tracks(withMediaType: .video).first else {
        throw VideoEncodeError.videoLoadFailed("No video track found")
    }
    
    // Get original properties
    let originalDuration = asset.duration.seconds
    let originalSize = videoTrack.naturalSize
    let originalBitrate = videoTrack.estimatedDataRate
    
    // Create composition
    let composition = AVMutableComposition()
    guard let compositionTrack = composition.addMutableTrack(
        withMediaType: .video,
        preferredTrackID: kCMPersistentTrackID_Invalid
    ) else {
        throw VideoEncodeError.encodingFailed("Could not create composition track")
    }
    
    try compositionTrack.insertTimeRange(
        CMTimeRangeMake(start: .zero, duration: asset.duration),
        of: videoTrack,
        at: .zero
    )
    
    // Create export session
    guard let exportSession = AVAssetExportSession(
        asset: composition,
        presetName: AVAssetExportPresetHighestQuality
    ) else {
        throw VideoEncodeError.encodingFailed("Could not create export session")
    }
    
    // Configure export
    exportSession.outputURL = outputURL
    exportSession.outputFileType = .mp4
    
    // Apply video compression
    let videoComposition = AVMutableVideoComposition()
    videoComposition.renderSize = preset.maxResolution ?? originalSize
    videoComposition.frameDuration = CMTimeMake(value: 1, timescale: 30)
    
    let instruction = AVMutableVideoCompositionInstruction()
    instruction.timeRange = CMTimeRangeMake(start: .zero, duration: asset.duration)
    
    let layerInstruction = AVMutableVideoCompositionLayerInstruction(assetTrack: compositionTrack)
    instruction.layerInstructions = [layerInstruction]
    videoComposition.instructions = [instruction]
    
    exportSession.videoComposition = videoComposition
    
    // Export
    let semaphore = DispatchSemaphore(value: 0)
    var exportError: Error?
    
    exportSession.exportAsynchronously {
        if let error = exportSession.error {
            exportError = error
        }
        semaphore.signal()
    }
    
    semaphore.wait()
    
    if let error = exportError {
        throw VideoEncodeError.encodingFailed(error.localizedDescription)
    }
    
    // Get output size
    let fm = FileManager.default
    let originalFileSize = (try? fm.attributesOfItem(atPath: inputPath)[.size] as? Int64) ?? 0
    let outputFileSize = (try? fm.attributesOfItem(atPath: outputPath)[.size] as? Int64) ?? 0
    
    return [
        "input": inputPath,
        "output": outputPath,
        "preset": preset.rawValue,
        "original_duration": originalDuration,
        "original_size": ["width": originalSize.width, "height": originalSize.height],
        "original_bitrate": originalBitrate,
        "original_file_size": originalFileSize,
        "output_file_size": outputFileSize,
        "compression_ratio": originalFileSize > 0 ? Double(outputFileSize) / Double(originalFileSize) : 1.0
    ]
}

// MARK: - Commands

struct EncodeCommand: ParsableCommand {
    static let configuration = CommandConfiguration(
        commandName: "encode",
        abstract: "Encode a video with App Store optimized settings"
    )
    
    @Option(name: .long, help: "Input video path")
    var input: String
    
    @Option(name: .long, help: "Output video path")
    var output: String
    
    @Option(name: .long, help: "Encoding preset (\(VideoPreset.allCases.map { $0.rawValue }.joined(separator: ", ")))")
    var preset: String = "preview"
    
    mutating func run() throws {
        guard let presetEnum = VideoPreset(rawValue: preset) else {
            throw VideoEncodeError.invalidInput("Unknown preset: \(preset)")
        }
        
        let result = try encodeVideo(
            inputPath: input,
            outputPath: output,
            preset: presetEnum
        )
        
        let data = try JSONSerialization.data(withJSONObject: result, options: .sortedKeys)
        print(String(data: data, encoding: .utf8)!)
    }
}

struct InfoCommand: ParsableCommand {
    static let configuration = CommandConfiguration(
        commandName: "info",
        abstract: "Get video information"
    )
    
    @Argument(help: "Video path")
    var path: String
    
    mutating func run() throws {
        let asset = AVAsset(url: URL(fileURLWithPath: path))
        
        guard let videoTrack = asset.tracks(withMediaType: .video).first else {
            throw VideoEncodeError.videoLoadFailed("No video track found")
        }
        
        let fm = FileManager.default
        let fileSize = (try? fm.attributesOfItem(atPath: path)[.size] as? Int64) ?? 0
        
        let dict: [String: Any] = [
            "path": path,
            "duration_seconds": asset.duration.seconds,
            "dimensions": [
                "width": videoTrack.naturalSize.width,
                "height": videoTrack.naturalSize.height
            ],
            "frame_rate": videoTrack.nominalFrameRate,
            "bitrate": videoTrack.estimatedDataRate,
            "file_size": fileSize
        ]
        
        let data = try JSONSerialization.data(withJSONObject: dict, options: .sortedKeys)
        print(String(data: data, encoding: .utf8)!)
    }
}

@main
struct VideoEncodeCommand: ParsableCommand {
    static let configuration = CommandConfiguration(
        commandName: "asc-video-encode",
        abstract: "Video encoding optimized for App Store app previews",
        version: "0.1.0",
        subcommands: [EncodeCommand.self, InfoCommand.self]
    )
}
