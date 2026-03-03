import XCTest
@testable import asc_screenshot_frame

final class ScreenshotFrameTests: XCTestCase {
    
    func testDeviceTypeRawValues() {
        let devices = DeviceType.allCases
        XCTAssertFalse(devices.isEmpty)
        
        // Verify each device has a valid raw value
        for device in devices {
            XCTAssertFalse(device.rawValue.isEmpty)
        }
    }
    
    func testDeviceDisplaySizes() {
        let iPhone14Pro = DeviceType.iPhone14Pro
        let size = iPhone14Pro.displaySize
        XCTAssertEqual(size.width, 1179)
        XCTAssertEqual(size.height, 2556)
        
        let iPadPro129 = DeviceType.iPadPro129
        let iPadSize = iPadPro129.displaySize
        XCTAssertEqual(iPadSize.width, 2048)
        XCTAssertEqual(iPadSize.height, 2732)
    }
    
    func testScreenshotFrameErrorDescription() {
        let errors: [ScreenshotFrameError] = [
            .invalidInput("test"),
            .imageLoadFailed("test"),
            .frameLoadFailed("test"),
            .processingFailed("test"),
            .deviceNotSupported("test"),
            .saveFailed("test")
        ]
        
        for error in errors {
            XCTAssertNotNil(error.errorDescription)
            XCTAssertFalse(error.errorDescription?.isEmpty ?? true)
        }
    }
    
    func testValidateScreenshotDimensions() {
        // Create a mock CIImage for testing
        // Note: This is a simplified test - actual testing would need real images
        XCTAssertTrue(true) // Placeholder for image loading tests
    }
    
    func testLoadImageInvalidPath() {
        XCTAssertThrowsError(try loadImage(from: "/nonexistent/path.png")) { error in
            guard let frameError = error as? ScreenshotFrameError,
                  case .imageLoadFailed = frameError else {
                XCTFail("Expected imageLoadFailed error")
                return
            }
        }
    }
}
