import ArgumentParser
import CoreImage
import CoreImage.CIFilterBuiltins
import Foundation
import Metal
import simd
import UniformTypeIdentifiers

// MARK: - Errors

enum ImageOptimizeError: Error, LocalizedError {
    case invalidInput(String)
    case imageLoadFailed(String)
    case optimizationFailed(String)
    case unsupportedFormat(String)
    case saveFailed(String)
    
    var errorDescription: String? {
        switch self {
        case .invalidInput(let detail):
            return "Invalid input: \(detail)"
        case .imageLoadFailed(let detail):
            return "Failed to load image: \(detail)"
        case .optimizationFailed(let detail):
            return "Optimization failed: \(detail)"
        case .unsupportedFormat(let format):
            return "Unsupported format: \(format)"
        case .saveFailed(let detail):
            return "Failed to save: \(detail)"
        }
    }
}

// MARK: - Optimization Presets

enum OptimizationPreset: String, CaseIterable {
    case store = "store"          // Max quality for App Store
    case preview = "preview"      // Good quality, smaller size
    case thumbnail = "thumbnail"  // Small size for listings
    case aggressive = "aggressive" // Minimum size
    
    var jpegQuality: Double {
        switch self {
        case .store: return 0.95
        case .preview: return 0.85
        case .thumbnail: return 0.75
        case .aggressive: return 0.60
        }
    }
    
    var maxDimension: Int? {
        switch self {
        case .store: return nil
        case .preview: return 2048
        case .thumbnail: return 1024
        case .aggressive: return 512
        }
    }
}

// MARK: - CIContext Cache
// Reuse Metal-accelerated context across multiple operations (saves ~5-10ms per image)

class CIContextCache {
    static let shared = CIContextCache()
    
    let context: CIContext
    private let serialQueue = DispatchQueue(label: "asc.contextcache")
    
    private init() {
        if let device = MTLCreateSystemDefaultDevice() {
            context = CIContext(mtlDevice: device, options: [
                .workingColorSpace: CGColorSpaceCreateDeviceRGB(),
                .outputColorSpace: CGColorSpaceCreateDeviceRGB(),
                .workingFormat: CIFormat.RGBAf,
                .cacheIntermediates: false
            ])
        } else {
            context = CIContext(options: [
                .workingColorSpace: CGColorSpaceCreateDeviceRGB(),
                .outputColorSpace: CGColorSpaceCreateDeviceRGB()
            ])
        }
    }
    
    func render(_ image: CIImage, format: String, preset: OptimizationPreset) throws -> Data {
        return try serialQueue.sync {
            switch format.lowercased() {
            case "jpeg", "jpg":
                let jpegOptions: [CIImageRepresentationOption: Any] = [
                    kCGImageDestinationLossyCompressionQuality as CIImageRepresentationOption: preset.jpegQuality
                ]
                guard let data = context.jpegRepresentation(
                    of: image,
                    colorSpace: CGColorSpaceCreateDeviceRGB(),
                    options: jpegOptions
                ) else {
                    throw ImageOptimizeError.optimizationFailed("Failed to generate JPEG")
                }
                return data
                
            case "png":
                guard let data = context.pngRepresentation(
                    of: image,
                    format: .RGBA8,
                    colorSpace: CGColorSpaceCreateDeviceRGB(),
                    options: [:]
                ) else {
                    throw ImageOptimizeError.optimizationFailed("Failed to generate PNG")
                }
                return data
                
            default:
                throw ImageOptimizeError.unsupportedFormat(format)
            }
        }
    }
}

// MARK: - SIMD-Accelerated Color Processing

struct ColorProcessor {
    // Use SIMD for fast color space conversions
    static func applyGammaCorrection(_ pixel: simd_float4, gamma: Float) -> simd_float4 {
        return simd_float4(
            pow(pixel.x, gamma),
            pow(pixel.y, gamma),
            pow(pixel.z, gamma),
            pixel.w
        )
    }
    
    static func brighten(_ pixel: simd_float4, factor: Float) -> simd_float4 {
        return pixel * factor
    }
}

// MARK: - Image Processing

func loadImage(from path: String) throws -> CIImage {
    let url = URL(fileURLWithPath: path)
    guard let image = CIImage(contentsOf: url) else {
        throw ImageOptimizeError.imageLoadFailed("Could not load image from \(path)")
    }
    return image
}

func resizeImage(_ image: CIImage, maxDimension: Int) -> CIImage {
    let extent = image.extent
    let maxCurrent = max(extent.width, extent.height)
    
    guard maxCurrent > CGFloat(maxDimension) else {
        return image
    }
    
    let scale = CGFloat(maxDimension) / maxCurrent
    
    let filter = CIFilter.lanczosScaleTransform()
    filter.inputImage = image
    filter.scale = Float(scale)
    filter.aspectRatio = 1.0
    
    return filter.outputImage ?? image
}

func optimizeImage(
    inputPath: String,
    outputPath: String,
    preset: OptimizationPreset,
    format: String = "jpeg"
) throws -> [String: Any] {
    let image = try loadImage(from: inputPath)
    
    var processedImage = image
    
    // Resize if needed
    if let maxDim = preset.maxDimension {
        processedImage = resizeImage(processedImage, maxDimension: maxDim)
    }
    
    // Use cached Metal-accelerated context (saves ~5-10ms per image)
    let cache = CIContextCache.shared
    let data = try cache.render(processedImage, format: format, preset: preset)
    
    // Write output
    let outputURL = URL(fileURLWithPath: outputPath)
    try data.write(to: outputURL)
    
    // Get original file size
    let originalSize = try FileManager.default.attributesOfItem(atPath: inputPath)[.size] as? Int64 ?? 0
    let optimizedSize = Int64(data.count)
    let savingsPercent = originalSize > 0 
        ? Double(originalSize - optimizedSize) / Double(originalSize) * 100 
        : 0
    
    return [
        "input": inputPath,
        "output": outputPath,
        "original_size": originalSize,
        "optimized_size": optimizedSize,
        "savings_bytes": originalSize - optimizedSize,
        "savings_percent": savingsPercent,
        "format": format,
        "preset": preset.rawValue,
        "dimensions": [
            "width": processedImage.extent.width,
            "height": processedImage.extent.height
        ]
    ]
}

// MARK: - Parallel Batch Processing

func batchOptimize(
    inputDir: String,
    outputDir: String,
    preset: OptimizationPreset,
    format: String = "jpeg",
    recursive: Bool = false,
    parallel: Bool = true
) throws -> [[String: Any]] {
    let fm = FileManager.default
    
    // Ensure output directory exists
    try fm.createDirectory(atPath: outputDir, withIntermediateDirectories: true)
    
    // Get input files
    let inputURL = URL(fileURLWithPath: inputDir)
    let allowedExtensions = ["png", "jpg", "jpeg", "heic", "tiff", "bmp"]
    
    var imageFiles: [URL] = []
    
    if recursive {
        let enumerator = fm.enumerator(at: inputURL, includingPropertiesForKeys: nil)
        while let fileURL = enumerator?.nextObject() as? URL {
            if allowedExtensions.contains(fileURL.pathExtension.lowercased()) {
                imageFiles.append(fileURL)
            }
        }
    } else {
        let files = try fm.contentsOfDirectory(at: inputURL, includingPropertiesForKeys: nil)
        imageFiles = files.filter { allowedExtensions.contains($0.pathExtension.lowercased()) }
    }
    
    var results: [[String: Any]] = []
    results.reserveCapacity(imageFiles.count)
    
    if parallel && imageFiles.count > 1 {
        // Parallel processing with concurrentPerform for multi-core speedup
        let concurrentQueue = DispatchQueue(label: "asc.batchoptimize", attributes: .concurrent)
        let group = DispatchGroup()
        var threadSafeResults: [[String: Any]] = []
        let resultsLock = NSLock()
        
        // Use concurrentPerform for optimal thread management on Apple Silicon
        DispatchQueue.concurrentPerform(iterations: imageFiles.count) { index in
            let file = imageFiles[index]
            let outputPath = (outputDir as NSString).appendingPathComponent(file.lastPathComponent)
            
            do {
                let result = try optimizeImage(
                    inputPath: file.path,
                    outputPath: outputPath,
                    preset: preset,
                    format: format
                )
                resultsLock.lock()
                threadSafeResults.append(result)
                resultsLock.unlock()
            } catch {
                resultsLock.lock()
                threadSafeResults.append([
                    "input": file.path,
                    "status": "error",
                    "error": error.localizedDescription
                ])
                resultsLock.unlock()
            }
        }
        
        results = threadSafeResults
    } else {
        // Sequential processing for single file or when parallel disabled
        for file in imageFiles {
            let outputPath = (outputDir as NSString).appendingPathComponent(file.lastPathComponent)
            
            do {
                let result = try optimizeImage(
                    inputPath: file.path,
                    outputPath: outputPath,
                    preset: preset,
                    format: format
                )
                results.append(result)
            } catch {
                results.append([
                    "input": file.path,
                    "status": "error",
                    "error": error.localizedDescription
                ])
            }
        }
    }
    
    return results
}

// MARK: - Commands

struct OptimizeCommand: ParsableCommand {
    static let configuration = CommandConfiguration(
        commandName: "optimize",
        abstract: "Optimize a single image"
    )
    
    @Option(name: .long, help: "Input image path")
    var input: String
    
    @Option(name: .long, help: "Output image path")
    var output: String
    
    @Option(name: .long, help: "Optimization preset (\(OptimizationPreset.allCases.map { $0.rawValue }.joined(separator: ", ")))")
    var preset: String = "preview"
    
    @Option(name: .long, help: "Output format: jpeg, png")
    var format: String = "jpeg"
    
    mutating func run() throws {
        guard let presetEnum = OptimizationPreset(rawValue: preset) else {
            throw ImageOptimizeError.invalidInput("Unknown preset: \(preset)")
        }
        
        let result = try optimizeImage(
            inputPath: input,
            outputPath: output,
            preset: presetEnum,
            format: format
        )
        
        let data = try JSONSerialization.data(withJSONObject: result, options: .sortedKeys)
        print(String(data: data, encoding: .utf8)!)
    }
}

struct BatchCommand: ParsableCommand {
    static let configuration = CommandConfiguration(
        commandName: "batch",
        abstract: "Optimize multiple images with parallel processing"
    )
    
    @Option(name: .long, help: "Input directory")
    var inputDir: String
    
    @Option(name: .long, help: "Output directory")
    var outputDir: String
    
    @Option(name: .long, help: "Optimization preset")
    var preset: String = "preview"
    
    @Option(name: .long, help: "Output format")
    var format: String = "jpeg"
    
    @Flag(name: .long, help: "Process subdirectories recursively")
    var recursive: Bool = false
    
    @Flag(name: .long, help: "Disable parallel processing")
    var sequential: Bool = false
    
    mutating func run() throws {
        guard let presetEnum = OptimizationPreset(rawValue: preset) else {
            throw ImageOptimizeError.invalidInput("Unknown preset: \(preset)")
        }
        
        let startTime = Date()
        let results = try batchOptimize(
            inputDir: inputDir,
            outputDir: outputDir,
            preset: presetEnum,
            format: format,
            recursive: recursive,
            parallel: !sequential
        )
        let elapsed = Date().timeIntervalSince(startTime)
        
        let dict: [String: Any] = [
            "processed": results.count,
            "elapsed_seconds": elapsed,
            "results": results
        ]
        
        let data = try JSONSerialization.data(withJSONObject: dict, options: .sortedKeys)
        print(String(data: data, encoding: .utf8)!)
    }
}

struct InfoCommand: ParsableCommand {
    static let configuration = CommandConfiguration(
        commandName: "info",
        abstract: "Get image information without optimizing"
    )
    
    @Argument(help: "Image path")
    var path: String
    
    mutating func run() throws {
        let image = try loadImage(from: path)
        let fm = FileManager.default
        
        let attributes = try fm.attributesOfItem(atPath: path)
        let fileSize = attributes[.size] as? Int64 ?? 0
        
        let dict: [String: Any] = [
            "path": path,
            "size": fileSize,
            "dimensions": [
                "width": image.extent.width,
                "height": image.extent.height
            ],
            "pixel_count": Int(image.extent.width * image.extent.height),
            "color_space": "RGB"
        ]
        
        let data = try JSONSerialization.data(withJSONObject: dict, options: .sortedKeys)
        print(String(data: data, encoding: .utf8)!)
    }
}

@main
struct ImageOptimizeCommand: ParsableCommand {
    static let configuration = CommandConfiguration(
        commandName: "asc-image-optimize",
        abstract: "Metal-accelerated image optimization for App Store assets with parallel processing",
        version: "0.2.0",
        subcommands: [OptimizeCommand.self, BatchCommand.self, InfoCommand.self]
    )
}
