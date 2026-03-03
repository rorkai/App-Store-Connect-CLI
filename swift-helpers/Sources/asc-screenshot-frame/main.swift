import ArgumentParser
import CoreImage
import CoreImage.CIFilterBuiltins
import Foundation
import Metal
import UniformTypeIdentifiers

// MARK: - Errors

enum ScreenshotFrameError: Error, LocalizedError {
    case invalidInput(String)
    case imageLoadFailed(String)
    case frameLoadFailed(String)
    case processingFailed(String)
    case deviceNotSupported(String)
    case saveFailed(String)
    
    var errorDescription: String? {
        switch self {
        case .invalidInput(let detail):
            return "Invalid input: \(detail)"
        case .imageLoadFailed(let detail):
            return "Failed to load image: \(detail)"
        case .frameLoadFailed(let detail):
            return "Failed to load device frame: \(detail)"
        case .processingFailed(let detail):
            return "Image processing failed: \(detail)"
        case .deviceNotSupported(let device):
            return "Device type not supported: \(device)"
        case .saveFailed(let detail):
            return "Failed to save output: \(detail)"
        }
    }
}

// MARK: - Device Specifications

enum DeviceType: String, CaseIterable {
    case iPhone14Pro = "iphone-14-pro"
    case iPhone14ProMax = "iphone-14-pro-max"
    case iPhone15 = "iphone-15"
    case iPhone15Pro = "iphone-15-pro"
    case iPhone15ProMax = "iphone-15-pro-max"
    case iPhone16 = "iphone-16"
    case iPhone16Pro = "iphone-16-pro"
    case iPhone16ProMax = "iphone-16-pro-max"
    case iPhone16e = "iphone-16e"
    case iPadPro11 = "ipad-pro-11"
    case iPadPro129 = "ipad-pro-12-9"
    
    var displaySize: CGSize {
        switch self {
        case .iPhone14Pro:
            return CGSize(width: 1179, height: 2556)
        case .iPhone14ProMax:
            return CGSize(width: 1290, height: 2796)
        case .iPhone15:
            return CGSize(width: 1179, height: 2556)
        case .iPhone15Pro:
            return CGSize(width: 1179, height: 2556)
        case .iPhone15ProMax:
            return CGSize(width: 1290, height: 2796)
        case .iPhone16:
            return CGSize(width: 1179, height: 2556)
        case .iPhone16Pro:
            return CGSize(width: 1206, height: 2622)
        case .iPhone16ProMax:
            return CGSize(width: 1320, height: 2868)
        case .iPhone16e:
            return CGSize(width: 1170, height: 2532)
        case .iPadPro11:
            return CGSize(width: 1668, height: 2388)
        case .iPadPro129:
            return CGSize(width: 2048, height: 2732)
        }
    }
    
    var framePath: String? {
        // Returns path to built-in frame resources or nil if custom frame needed
        // For now, we'll support compositing without pre-made frames
        return nil
    }
}

// MARK: - CIContext Cache
// Reuse Metal-accelerated context across multiple operations

class CIContextCache {
    static let shared = CIContextCache()
    
    let context: CIContext
    
    private init() {
        if let device = MTLCreateSystemDefaultDevice() {
            context = CIContext(mtlDevice: device, options: [
                .workingColorSpace: CGColorSpaceCreateDeviceRGB(),
                .outputColorSpace: CGColorSpaceCreateDeviceRGB(),
                .cacheIntermediates: false
            ])
        } else {
            context = CIContext(options: [
                .workingColorSpace: CGColorSpaceCreateDeviceRGB(),
                .outputColorSpace: CGColorSpaceCreateDeviceRGB()
            ])
        }
    }
}

// MARK: - Image Processing

func loadImage(from path: String) throws -> CIImage {
    let url = URL(fileURLWithPath: path)
    guard let image = CIImage(contentsOf: url) else {
        throw ScreenshotFrameError.imageLoadFailed("Could not load image from \(path)")
    }
    return image
}

func resizeImage(_ image: CIImage, to size: CGSize) -> CIImage {
    let scaleX = size.width / image.extent.width
    let scaleY = size.height / image.extent.height
    let scale = min(scaleX, scaleY)
    
    let filter = CIFilter.lanczosScaleTransform()
    filter.inputImage = image
    filter.scale = Float(scale)
    filter.aspectRatio = 1.0
    
    return filter.outputImage ?? image
}

func applyRoundedCorners(to image: CIImage, radius: CGFloat) -> CIImage {
    // Create a rounded rectangle mask
    let extent = image.extent
    let roundedRect = CIImage(color: CIColor(red: 1, green: 1, blue: 1))
        .cropped(to: extent)
    
    // Apply rounded corners using a simple mask approach
    let filter = CIFilter.gaussianBlur()
    filter.inputImage = roundedRect
    filter.radius = Float(radius)
    
    // For proper rounded corners, we'd need more complex masking
    // This is a simplified version
    return image
}

func createFramedScreenshot(
    screenshotPath: String,
    deviceType: DeviceType,
    outputPath: String,
    backgroundColor: CIColor? = nil,
    padding: Double = 40
) throws {
    // Load screenshot
    let screenshot = try loadImage(from: screenshotPath)
    
    // Get target dimensions
    let targetSize = deviceType.displaySize
    
    // Resize screenshot to fit device display area
    let resizedScreenshot = resizeImage(screenshot, to: targetSize)
    
    // Create output context
    let extent = CGRect(
        x: 0,
        y: 0,
        width: targetSize.width + (padding * 2),
        height: targetSize.height + (padding * 2)
    )
    
    // Create background
    let bgColor = backgroundColor ?? CIColor(red: 0.95, green: 0.95, blue: 0.97) // Light gray default
    var outputImage = CIImage(color: bgColor).cropped(to: extent)
    
    // Composite screenshot onto background, centered
    let screenshotX = (extent.width - resizedScreenshot.extent.width) / 2
    let screenshotY = (extent.height - resizedScreenshot.extent.height) / 2
    let transform = CGAffineTransform(translationX: screenshotX, y: screenshotY)
    let positionedScreenshot = resizedScreenshot.transformed(by: transform)
    
    // Use source-over compositing
    let composite = CIFilter.sourceOverCompositing()
    composite.inputImage = positionedScreenshot
    composite.backgroundImage = outputImage
    if let result = composite.outputImage {
        outputImage = result
    }
    
    // Export using cached context
    let cache = CIContextCache.shared
    
    guard let cgImage = cache.context.createCGImage(outputImage, from: outputImage.extent) else {
        throw ScreenshotFrameError.processingFailed("Failed to render final image")
    }
    
    let outputURL = URL(fileURLWithPath: outputPath)
    let destination = CGImageDestinationCreateWithURL(
        outputURL as CFURL,
        UTType.png.identifier as CFString,
        1,
        nil
    )
    
    guard let dest = destination else {
        throw ScreenshotFrameError.saveFailed("Failed to create image destination")
    }
    
    CGImageDestinationAddImage(dest, cgImage, nil)
    
    guard CGImageDestinationFinalize(dest) else {
        throw ScreenshotFrameError.saveFailed("Failed to write image data")
    }
}

func validateScreenshotDimensions(_ image: CIImage, for device: DeviceType) -> Bool {
    let size = device.displaySize
    let tolerance: Double = 0.05 // 5% tolerance
    
    // Check if aspect ratio matches (portrait or landscape)
    let targetAspect = size.width / size.height
    let imageAspect = image.extent.width / image.extent.height
    
    return abs(targetAspect - imageAspect) < tolerance
}

// MARK: - Commands

struct FrameCommand: ParsableCommand {
    static let configuration = CommandConfiguration(
        commandName: "frame",
        abstract: "Compose screenshots into device frames"
    )
    
    @Option(name: .long, help: "Input screenshot path")
    var input: String
    
    @Option(name: .long, help: "Output path")
    var output: String
    
    @Option(name: .long, help: "Device type (\(DeviceType.allCases.map { $0.rawValue }.joined(separator: ", ")))")
    var device: String
    
    @Option(name: .long, help: "Background color (hex, e.g., #F5F5F7)")
    var background: String?
    
    @Option(name: .long, help: "Padding around screenshot")
    var padding: Double = 40
    
    @Flag(name: .long, help: "Validate dimensions without processing")
    var validate: Bool = false
    
    func run() throws {
        // Parse device type
        guard let deviceType = DeviceType(rawValue: device) else {
            throw ScreenshotFrameError.deviceNotSupported(device)
        }
        
        // Parse background color if provided
        var bgColor: CIColor?
        if let bgHex = background {
            bgColor = parseColor(hex: bgHex)
        }
        
        if validate {
            let image = try loadImage(from: input)
            let isValid = validateScreenshotDimensions(image, for: deviceType)
            let dict: [String: Any] = [
                "valid": isValid,
                "device": device,
                "screenshot_size": ["width": image.extent.width, "height": image.extent.height],
                "expected_size": ["width": deviceType.displaySize.width, "height": deviceType.displaySize.height]
            ]
            let data = try JSONSerialization.data(withJSONObject: dict, options: .sortedKeys)
            print(String(data: data, encoding: .utf8)!)
            return
        }
        
        // Process the screenshot
        try createFramedScreenshot(
            screenshotPath: input,
            deviceType: deviceType,
            outputPath: output,
            backgroundColor: bgColor,
            padding: padding
        )
        
        let dict: [String: String] = [
            "status": "success",
            "output": output,
            "device": device
        ]
        let data = try JSONSerialization.data(withJSONObject: dict, options: .sortedKeys)
        print(String(data: data, encoding: .utf8)!)
    }
    
    func parseColor(hex: String) -> CIColor? {
        var hexString = hex.trimmingCharacters(in: .whitespaces)
        if hexString.hasPrefix("#") {
            hexString.removeFirst()
        }
        
        guard hexString.count == 6,
              let value = Int(hexString, radix: 16) else {
            return nil
        }
        
        let r = CGFloat((value >> 16) & 0xFF) / 255.0
        let g = CGFloat((value >> 8) & 0xFF) / 255.0
        let b = CGFloat(value & 0xFF) / 255.0
        
        return CIColor(red: r, green: g, blue: b)
    }
}

struct BatchCommand: ParsableCommand {
    static let configuration = CommandConfiguration(
        commandName: "batch",
        abstract: "Process multiple screenshots in parallel"
    )
    
    @Option(name: .long, help: "Input directory containing screenshots")
    var inputDir: String
    
    @Option(name: .long, help: "Output directory for framed screenshots")
    var outputDir: String
    
    @Option(name: .long, help: "Device type")
    var device: String
    
    @Option(name: .long, help: "File extension filter (default: png)")
    var ext: String = "png"
    
    @Flag(name: .long, help: "Disable parallel processing")
    var sequential: Bool = false
    
    func run() throws {
        let fm = FileManager.default
        
        guard let deviceType = DeviceType(rawValue: device) else {
            throw ScreenshotFrameError.deviceNotSupported(device)
        }
        
        // Ensure output directory exists
        try fm.createDirectory(atPath: outputDir, withIntermediateDirectories: true, attributes: nil)
        
        // Get input files
        let inputURL = URL(fileURLWithPath: inputDir)
        let files = try fm.contentsOfDirectory(at: inputURL, includingPropertiesForKeys: nil)
            .filter { $0.pathExtension.lowercased() == ext.lowercased() }
        
        var results: [[String: String]] = []
        results.reserveCapacity(files.count)
        
        let startTime = Date()
        
        if !sequential && files.count > 1 {
            // Parallel processing with concurrentPerform for multi-core speedup
            let resultsLock = NSLock()
            var threadSafeResults: [[String: String]] = []
            
            DispatchQueue.concurrentPerform(iterations: files.count) { index in
                let file = files[index]
                let outputPath = (outputDir as NSString).appendingPathComponent(file.lastPathComponent)
                do {
                    try createFramedScreenshot(
                        screenshotPath: file.path,
                        deviceType: deviceType,
                        outputPath: outputPath
                    )
                    resultsLock.lock()
                    threadSafeResults.append([
                        "input": file.lastPathComponent,
                        "output": outputPath,
                        "status": "success"
                    ])
                    resultsLock.unlock()
                } catch {
                    resultsLock.lock()
                    threadSafeResults.append([
                        "input": file.lastPathComponent,
                        "status": "error",
                        "error": error.localizedDescription
                    ])
                    resultsLock.unlock()
                }
            }
            
            results = threadSafeResults
        } else {
            // Sequential processing
            for file in files {
                let outputPath = (outputDir as NSString).appendingPathComponent(file.lastPathComponent)
                do {
                    try createFramedScreenshot(
                        screenshotPath: file.path,
                        deviceType: deviceType,
                        outputPath: outputPath
                    )
                    results.append([
                        "input": file.lastPathComponent,
                        "output": outputPath,
                        "status": "success"
                    ])
                } catch {
                    results.append([
                        "input": file.lastPathComponent,
                        "status": "error",
                        "error": error.localizedDescription
                    ])
                }
            }
        }
        
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

@main
struct ScreenshotFrameCommand: ParsableCommand {
    static let configuration = CommandConfiguration(
        commandName: "asc-screenshot-frame",
        abstract: "Native screenshot framing using Core Image/Metal acceleration",
        version: "0.1.0",
        subcommands: [FrameCommand.self, BatchCommand.self]
    )
}
