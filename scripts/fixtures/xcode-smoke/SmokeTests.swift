import XCTest
final class SmokeTests: XCTestCase {
    func testPass() { XCTAssertEqual(2 + 2, 4) }
    func testSkip() throws { throw XCTSkip("smoke fixture") }
}
