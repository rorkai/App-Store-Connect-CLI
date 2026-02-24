// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "asc-helpers",
    platforms: [
        .macOS(.v14)
    ],
    products: [
        .executable(
            name: "asc-jwt-sign",
            targets: ["asc-jwt-sign"]
        ),
        .executable(
            name: "asc-screenshot-frame",
            targets: ["asc-screenshot-frame"]
        ),
        .executable(
            name: "asc-image-optimize",
            targets: ["asc-image-optimize"]
        ),
        .executable(
            name: "asc-video-encode",
            targets: ["asc-video-encode"]
        ),
        .executable(
            name: "asc-swift-daemon",
            targets: ["asc-swift-daemon"]
        )
    ],
    dependencies: [
        .package(url: "https://github.com/apple/swift-argument-parser.git", from: "1.3.0")
    ],
    targets: [
        // JWT signing helper - CryptoKit hardware-accelerated
        .executableTarget(
            name: "asc-jwt-sign",
            dependencies: [
                .product(name: "ArgumentParser", package: "swift-argument-parser")
            ],
            path: "Sources/asc-jwt-sign"
        ),

        // Screenshot framing helper - Core Image/Metal accelerated
        .executableTarget(
            name: "asc-screenshot-frame",
            dependencies: [
                .product(name: "ArgumentParser", package: "swift-argument-parser")
            ],
            path: "Sources/asc-screenshot-frame"
        ),

        // Image optimization helper - Core Image/Metal accelerated
        .executableTarget(
            name: "asc-image-optimize",
            dependencies: [
                .product(name: "ArgumentParser", package: "swift-argument-parser")
            ],
            path: "Sources/asc-image-optimize"
        ),

        // Video encoding helper - AVFoundation
        .executableTarget(
            name: "asc-video-encode",
            dependencies: [
                .product(name: "ArgumentParser", package: "swift-argument-parser")
            ],
            path: "Sources/asc-video-encode"
        ),
        
        // Swift daemon - eliminates subprocess overhead entirely
        .executableTarget(
            name: "asc-swift-daemon",
            dependencies: [
                .product(name: "ArgumentParser", package: "swift-argument-parser")
            ],
            path: "Sources/asc-swift-daemon"
        ),

        // Test targets
        .testTarget(
            name: "JWTHelperTests",
            dependencies: ["asc-jwt-sign"],
            path: "Tests/JWTHelperTests"
        ),
        .testTarget(
            name: "ScreenshotFrameTests",
            dependencies: ["asc-screenshot-frame"],
            path: "Tests/ScreenshotFrameTests"
        )
    ]
)
