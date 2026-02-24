# Swift Helper Tools

Native macOS helper tools providing hardware-accelerated operations for the App Store Connect CLI.

## Overview

These Swift helpers leverage native macOS frameworks where they provide a genuine performance benefit over Go or system CLI invocations:

| Helper | Framework | Purpose |
|--------|-----------|---------|
| `asc-jwt-sign` | CryptoKit | Hardware-accelerated ES256 JWT signing |
| `asc-screenshot-frame` | Core Image/Metal | GPU-accelerated screenshot composition |
| `asc-image-optimize` | Core Image/Metal | GPU-accelerated image optimization |
| `asc-video-encode` | AVFoundation | Native video encoding for App Store previews |

## Requirements

- macOS 14.0+
- Swift 5.9+
- Xcode Command Line Tools

## Building

```bash
cd swift-helpers
swift build                    # Debug
swift build -c release         # Release
swift test                     # Run tests
```

## Usage

Each helper is a standalone CLI tool that can be called directly or via the Go integration.

### asc-jwt-sign

Generate JWT tokens using CryptoKit's hardware-accelerated P-256 signing:

```bash
asc-jwt-sign \
  --issuer-id "YOUR_ISSUER_ID" \
  --key-id "YOUR_KEY_ID" \
  --private-key-path "/path/to/key.p8"
```

Output formats:
- `token` (default): Raw JWT string
- `json`: `{"token": "...", "expires_in": 600}`

### asc-screenshot-frame

Core Image/Metal-accelerated screenshot composition:

```bash
# Frame a single screenshot
asc-screenshot-frame frame \
  --input screenshot.png \
  --output framed.png \
  --device iphone-16-pro

# Batch process directory
asc-screenshot-frame batch \
  --input-dir ./screenshots \
  --output-dir ./framed \
  --device iphone-16-pro

# Validate dimensions only
asc-screenshot-frame frame \
  --input screenshot.png \
  --device iphone-16-pro \
  --validate
```

Supported device types:
- `iphone-14-pro`, `iphone-14-pro-max`
- `iphone-15`, `iphone-15-pro`, `iphone-15-pro-max`
- `iphone-16`, `iphone-16-pro`, `iphone-16-pro-max`, `iphone-16e`
- `ipad-pro-11`, `ipad-pro-12-9`

### asc-image-optimize

Metal-accelerated image optimization for App Store assets:

```bash
# Optimize a single image
asc-image-optimize optimize \
  --input screenshot.png \
  --output optimized.jpg \
  --preset preview

# Batch optimize directory
asc-image-optimize batch \
  --input-dir ./screenshots \
  --output-dir ./optimized \
  --preset thumbnail \
  --format jpeg

# Get image info
asc-image-optimize info image.png
```

Presets: `store` (95% quality), `preview` (85%), `thumbnail` (75%), `aggressive` (60%)

### asc-video-encode

Video encoding optimized for App Store app previews:

```bash
# Encode with preset
asc-video-encode encode \
  --input raw_video.mov \
  --output preview.mp4 \
  --preset preview

# Get video info
asc-video-encode info video.mov
```

Presets: `store` (6Mbps), `preview` (4Mbps), `compact` (2Mbps)

## Go Integration

The Go CLI automatically uses Swift helpers when available on macOS:

```go
import "github.com/rudrankriyam/app-store-connect-cli/internal/swifthelpers"

// Check if helpers are available
if swifthelpers.IsAvailable() {
    // Use hardware-accelerated JWT signing
    resp, err := swifthelpers.SignJWT(ctx, swifthelpers.JWTSignRequest{
        IssuerID:       issuerID,
        KeyID:          keyID,
        PrivateKeyPath: keyPath,
    })

    // Use Core Image framing
    resp, err := swifthelpers.FrameScreenshot(ctx, swifthelpers.ScreenshotFrameRequest{
        InputPath:  "screenshot.png",
        OutputPath: "framed.png",
        DeviceType: "iphone-16-pro",
    })

    // Optimize images
    result, err := swifthelpers.OptimizeImage(ctx, swifthelpers.ImageOptimizeRequest{
        InputPath:  "screenshot.png",
        OutputPath: "optimized.jpg",
        Preset:     "preview",
        Format:     "jpeg",
    })

    // Encode video
    videoResult, err := swifthelpers.EncodeVideo(ctx, "input.mov", "output.mp4", "preview")
}
```

## Architecture

The helpers follow a simple CLI contract:

1. **Input**: Command-line flags
2. **Processing**: Native macOS frameworks (CryptoKit, CoreImage, AVFoundation, Security)
3. **Output**: JSON to stdout
4. **Errors**: Human-readable to stderr, non-zero exit code

This design allows:
- Easy testing of helpers in isolation
- Go integration via `exec.Command`
- Shell script compatibility

## Development

Adding a new helper:

1. Create `Sources/asc-your-helper/main.swift`
2. Add executable target in `Package.swift`
3. Implement ArgumentParser subcommands
4. Add tests in `Tests/YourHelperTests/`
5. Update Go integration in `internal/swifthelpers/`

**Note**: Only introduce Swift helpers for operations that leverage native frameworks
(CryptoKit, CoreImage, AVFoundation, Security.framework). For operations that would
simply wrap system CLIs (zip, unzip, codesign, xcrun), call them directly from Go.

## License

Same as parent project (MIT).
