import XCTest
import CryptoKit
@testable import asc_jwt_sign

final class JWTHelperTests: XCTestCase {
    
    func testBase64URLEncode() {
        let data = Data("Hello+World/Path=test".utf8)
        let encoded = base64URLEncode(data)
        XCTAssertFalse(encoded.contains("+"))
        XCTAssertFalse(encoded.contains("/"))
        XCTAssertFalse(encoded.contains("="))
    }
    
    func testJWTHeaderEncoding() throws {
        // Test that header encodes correctly
        let header = JWTHeader(kid: "TEST123")
        let encoder = JSONEncoder()
        encoder.outputFormatting = .sortedKeys
        let data = try encoder.encode(header)
        let json = String(data: data, encoding: .utf8)!
        
        XCTAssertTrue(json.contains("\"alg\":\"ES256\""))
        XCTAssertTrue(json.contains("\"kid\":\"TEST123\""))
        XCTAssertTrue(json.contains("\"typ\":\"JWT\""))
    }
    
    func testJWTClaimsEncoding() throws {
        let claims = JWTClaims(
            iss: "test-issuer",
            iat: 1700000000,
            exp: 1700000600,
            aud: "appstoreconnect-v1"
        )
        
        let encoder = JSONEncoder()
        encoder.outputFormatting = .sortedKeys
        let data = try encoder.encode(claims)
        let json = String(data: data, encoding: .utf8)!
        
        XCTAssertTrue(json.contains("\"iss\":\"test-issuer\""))
        XCTAssertTrue(json.contains("\"aud\":\"appstoreconnect-v1\""))
    }
    
    func testLoadPrivateKey_InvalidPath() {
        XCTAssertThrowsError(try loadPrivateKey(from: "/nonexistent/path/key.p8")) { error in
            guard let jwtError = error as? JWTSignError else {
                XCTFail("Expected JWTSignError")
                return
            }
            if case .keyFileReadError = jwtError {
                // Expected
            } else {
                XCTFail("Expected keyFileReadError, got \(jwtError)")
            }
        }
    }
    
    func testGenerateJWTStructure() throws {
        // Generate a test P-256 key
        let privateKey = P256.Signing.PrivateKey()
        
        let token = try generateJWT(
            issuerID: "test-issuer",
            keyID: "TEST123",
            privateKey: privateKey
        )
        
        // Verify JWT structure (three parts separated by dots)
        let parts = token.split(separator: ".")
        XCTAssertEqual(parts.count, 3)
        
        // Verify each part is non-empty base64url
        XCTAssertFalse(parts[0].isEmpty)
        XCTAssertFalse(parts[1].isEmpty)
        XCTAssertFalse(parts[2].isEmpty)
        XCTAssertFalse(parts[0].contains("+"))
        XCTAssertFalse(parts[0].contains("/"))
    }
    
    func testSignWithValidKey() throws {
        let privateKey = P256.Signing.PrivateKey()
        let payload = "test.payload.data"
        
        let signature = try sign(payload: payload, with: privateKey)
        
        // Signature should be base64url encoded
        XCTAssertFalse(signature.isEmpty)
        XCTAssertFalse(signature.contains("+"))
        XCTAssertFalse(signature.contains("/"))
        XCTAssertFalse(signature.contains("="))
    }
}
