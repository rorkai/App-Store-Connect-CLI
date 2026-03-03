import ArgumentParser
import CryptoKit
import Foundation

// MARK: - Errors

enum JWTSignError: Error, LocalizedError {
    case invalidPrivateKey(String)
    case keyFileReadError(String)
    case invalidIssuerID
    case invalidKeyID
    case signingFailed(String)
    
    var errorDescription: String? {
        switch self {
        case .invalidPrivateKey(let detail):
            return "Invalid private key: \(detail)"
        case .keyFileReadError(let detail):
            return "Failed to read key file: \(detail)"
        case .invalidIssuerID:
            return "Invalid or missing issuer ID"
        case .invalidKeyID:
            return "Invalid or missing key ID"
        case .signingFailed(let detail):
            return "Signing failed: \(detail)"
        }
    }
}

// MARK: - JWT Structures

struct JWTHeader: Encodable {
    let alg: String = "ES256"
    let kid: String
    let typ: String = "JWT"
}

struct JWTClaims: Encodable {
    let iss: String
    let iat: Int
    let exp: Int
    let aud: String
}

// MARK: - Helper Functions

/// Token lifetime: 10 minutes (matches Go implementation)
let jwtTokenLifetime: TimeInterval = 10 * 60

func base64URLEncode(_ data: Data) -> String {
    data.base64EncodedString()
        .replacingOccurrences(of: "+", with: "-")
        .replacingOccurrences(of: "/", with: "_")
        .replacingOccurrences(of: "=", with: "")
}

func loadPrivateKey(from path: String) throws -> P256.Signing.PrivateKey {
    let url = URL(fileURLWithPath: path)
    let pemData: Data
    do {
        pemData = try Data(contentsOf: url)
    } catch {
        throw JWTSignError.keyFileReadError(error.localizedDescription)
    }
    
    guard let pemString = String(data: pemData, encoding: .utf8) else {
        throw JWTSignError.keyFileReadError("File is not valid UTF-8")
    }
    
    // Extract base64 content from PEM
    let lines = pemString.components(separatedBy: .newlines)
    let base64Lines = lines.filter { !$0.hasPrefix("-") && !$0.isEmpty }
    let base64String = base64Lines.joined()
    
    guard let keyData = Data(base64Encoded: base64String) else {
        throw JWTSignError.invalidPrivateKey("Failed to decode base64 content")
    }
    
    // Try to parse as SEC1 format (raw EC private key) first
    // This is what OpenSSL ecparam generates
    do {
        // SEC1 format for P-256 is 32 bytes
        if keyData.count == 32 {
            return try P256.Signing.PrivateKey(rawRepresentation: keyData)
        }
    } catch {
        // Fall through to PKCS#8 parsing
    }
    
    // Try PKCS#8 format (what OpenSSL pkcs8 -topk8 generates)
    // PKCS#8 structure: version(1) + algorithmIdentifier + octetString containing SEC1 key
    do {
        let privateKeyBytes = try extractSEC1FromPKCS8(keyData)
        return try P256.Signing.PrivateKey(rawRepresentation: privateKeyBytes)
    } catch {
        throw JWTSignError.invalidPrivateKey("Key is not valid P-256 format: \(error)")
    }
}

/// Extract SEC1 private key bytes from PKCS#8 container
func extractSEC1FromPKCS8(_ data: Data) throws -> Data {
    // PKCS#8 format: SEQUENCE { version, algorithmIdentifier, privateKey[OCTET STRING] }
    // The privateKey OCTET STRING contains ECPrivateKey structure (RFC 5915)
    
    var index = 0
    
    // Helper to read ASN.1 length
    func readLength(from data: Data, at index: inout Int) throws -> Int {
        guard index < data.count else {
            throw JWTSignError.invalidPrivateKey("Unexpected end of data")
        }
        let byte = data[index]
        if byte & 0x80 == 0 {
            // Short form
            index += 1
            return Int(byte)
        } else {
            // Long form
            let numBytes = Int(byte & 0x7F)
            index += 1
            var length = 0
            for _ in 0..<numBytes {
                guard index < data.count else {
                    throw JWTSignError.invalidPrivateKey("Unexpected end of data reading length")
                }
                length = (length << 8) + Int(data[index])
                index += 1
            }
            return length
        }
    }
    
    // Helper to skip an ASN.1 element
    func skipElement(from data: Data, at index: inout Int) throws {
        guard index < data.count else { return }
        let tag = data[index]
        index += 1
        let length = try readLength(from: data, at: &index)
        index += length
    }
    
    // Skip outer SEQUENCE
    guard index < data.count && data[index] == 0x30 else {
        throw JWTSignError.invalidPrivateKey("Expected SEQUENCE")
    }
    index += 1
    _ = try readLength(from: data, at: &index)
    
    // Skip version INTEGER
    try skipElement(from: data, at: &index)
    
    // Skip algorithmIdentifier SEQUENCE
    try skipElement(from: data, at: &index)
    
    // Read privateKey OCTET STRING
    guard index < data.count && data[index] == 0x04 else {
        throw JWTSignError.invalidPrivateKey("Expected OCTET STRING for privateKey")
    }
    index += 1
    let privateKeyLength = try readLength(from: data, at: &index)
    
    guard index + privateKeyLength <= data.count else {
        throw JWTSignError.invalidPrivateKey("Private key length exceeds data")
    }
    
    let ecPrivateKeyData = data.subdata(in: index..<(index + privateKeyLength))
    
    // ECPrivateKey structure: SEQUENCE { version, privateKey[OCTET STRING], parameters, publicKey }
    // We need to extract the privateKey field (32 bytes for P-256)
    var ecIndex = 0
    
    // Skip outer SEQUENCE
    guard ecIndex < ecPrivateKeyData.count && ecPrivateKeyData[ecIndex] == 0x30 else {
        throw JWTSignError.invalidPrivateKey("Expected SEQUENCE for ECPrivateKey")
    }
    ecIndex += 1
    _ = try readLength(from: ecPrivateKeyData, at: &ecIndex)
    
    // Skip version INTEGER
    try skipElement(from: ecPrivateKeyData, at: &ecIndex)
    
    // Read privateKey OCTET STRING (this contains the 32 bytes we need)
    guard ecIndex < ecPrivateKeyData.count && ecPrivateKeyData[ecIndex] == 0x04 else {
        throw JWTSignError.invalidPrivateKey("Expected OCTET STRING for EC privateKey")
    }
    ecIndex += 1
    let sec1KeyLength = try readLength(from: ecPrivateKeyData, at: &ecIndex)
    
    guard ecIndex + sec1KeyLength <= ecPrivateKeyData.count else {
        throw JWTSignError.invalidPrivateKey("SEC1 key length exceeds data")
    }
    
    return ecPrivateKeyData.subdata(in: ecIndex..<(ecIndex + sec1KeyLength))
}

func generateJWT(issuerID: String, keyID: String, privateKey: P256.Signing.PrivateKey) throws -> String {
    let now = Date()
    let iat = Int(now.timeIntervalSince1970)
    let exp = Int(now.addingTimeInterval(jwtTokenLifetime).timeIntervalSince1970)
    
    let header = JWTHeader(kid: keyID)
    let claims = JWTClaims(
        iss: issuerID,
        iat: iat,
        exp: exp,
        aud: "appstoreconnect-v1"
    )
    
    let encoder = JSONEncoder()
    encoder.outputFormatting = .sortedKeys
    
    let headerData = try encoder.encode(header)
    let payloadData = try encoder.encode(claims)
    
    let headerEncoded = base64URLEncode(headerData)
    let payloadEncoded = base64URLEncode(payloadData)
    let signingInput = "\(headerEncoded).\(payloadEncoded)"
    
    guard let data = signingInput.data(using: .utf8) else {
        throw JWTSignError.signingFailed("Failed to encode signing input")
    }
    
    let signature = try privateKey.signature(for: data)
    let signatureEncoded = base64URLEncode(signature.rawRepresentation)
    
    return "\(signingInput).\(signatureEncoded)"
}

/// Signs a string payload using ES256 (ECDSA P-256 + SHA-256).
func sign(payload: String, with privateKey: P256.Signing.PrivateKey) throws -> String {
    guard let data = payload.data(using: .utf8) else {
        throw JWTSignError.signingFailed("Could not encode signing input as UTF-8")
    }
    do {
        let signature = try privateKey.signature(for: data)
        return base64URLEncode(signature.rawRepresentation)
    } catch {
        throw JWTSignError.signingFailed(error.localizedDescription)
    }
}

// MARK: - Command

@main
struct JWTSignCommand: ParsableCommand {
    static let configuration = CommandConfiguration(
        commandName: "asc-jwt-sign",
        abstract: "Generate JWT tokens for App Store Connect API authentication using CryptoKit hardware acceleration",
        version: "0.1.0"
    )
    
    @Option(name: .long, help: "App Store Connect Issuer ID")
    var issuerID: String
    
    @Option(name: .long, help: "App Store Connect Key ID")
    var keyID: String
    
    @Option(name: .long, help: "Path to private key file (.p8)")
    var privateKeyPath: String
    
    @Option(name: .long, help: "Output format: token (default), json")
    var output: String = "token"
    
    @Flag(name: .long, help: "Validate the generated token without output")
    var validate: Bool = false
    
    @Flag(name: .long, help: "Read batch requests from stdin (JSONL format)")
    var batch: Bool = false
    
    mutating func run() throws {
        // Handle batch mode for bulk operations
        if batch {
            try runBatch()
            return
        }
        
        try runSingle()
    }
    
    /// Run single JWT signing (original behavior)
    mutating func runSingle() throws {
        // Validate inputs
        guard !issuerID.isEmpty else {
            throw JWTSignError.invalidIssuerID
        }
        guard !keyID.isEmpty else {
            throw JWTSignError.invalidKeyID
        }
        
        // Load private key
        let privateKey = try loadPrivateKey(from: privateKeyPath)
        
        // Generate JWT
        let token = try generateJWT(issuerID: issuerID, keyID: keyID, privateKey: privateKey)
        
        // Output result
        switch output.lowercased() {
        case "json":
            let result: [String: Any] = [
                "token": token,
                "expires_in": Int(jwtTokenLifetime)
            ]
            let jsonData = try JSONSerialization.data(withJSONObject: result, options: .sortedKeys)
            print(String(data: jsonData, encoding: .utf8)!)
        default:
            print(token)
        }
    }
    
    /// Batch process multiple JWT signing requests from stdin
    /// Input format: JSON Lines (JSONL) - one JSON object per line
    /// Each line: {"issuer_id": "...", "key_id": "...", "private_key_path": "..."}
    /// Output: JSON array of results [{"token": "...", "expires_in": 600}, ...]
    /// Optimized: Caches private keys in memory to avoid reloading from disk
    func runBatch() throws {
        var results: [[String: Any]] = []
        let stdin = FileHandle.standardInput
        let data = stdin.readDataToEndOfFile()
        
        guard let input = String(data: data, encoding: .utf8), !input.isEmpty else {
            throw JWTSignError.invalidIssuerID // Reuse error - no input
        }
        
        let lines = input.components(separatedBy: .newlines)
        var processedCount = 0
        
        // Cache private keys to avoid reloading from disk (saves ~1-2ms per key)
        var keyCache: [String: P256.Signing.PrivateKey] = [:]
        
        for line in lines {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            guard !trimmed.isEmpty, trimmed != "[" && trimmed != "]" else { continue }
            
            // Parse each line as JSON
            guard let lineData = trimmed.data(using: .utf8),
                  let request = try? JSONSerialization.jsonObject(with: lineData) as? [String: String] else {
                results.append(["error": "Invalid JSON: \(trimmed.prefix(50))"])
                continue
            }
            
            guard let reqIssuerID = request["issuer_id"],
                  let reqKeyID = request["key_id"],
                  let reqKeyPath = request["private_key_path"] else {
                results.append(["error": "Missing required fields in: \(trimmed)"])
                continue
            }
            
            do {
                // Use cached key or load and cache it
                let privateKey: P256.Signing.PrivateKey
                if let cached = keyCache[reqKeyPath] {
                    privateKey = cached
                } else {
                    privateKey = try loadPrivateKey(from: reqKeyPath)
                    keyCache[reqKeyPath] = privateKey
                }
                
                let token = try generateJWT(issuerID: reqIssuerID, keyID: reqKeyID, privateKey: privateKey)
                results.append([
                    "token": token,
                    "expires_in": Int(jwtTokenLifetime),
                    "success": true
                ])
                processedCount += 1
            } catch {
                results.append([
                    "error": error.localizedDescription,
                    "success": false
                ])
            }
        }
        
        // Output results as JSON array
        let outputData = try JSONSerialization.data(withJSONObject: results, options: .sortedKeys)
        print(String(data: outputData, encoding: .utf8)!)
        
        // Exit with error if any failed
        if processedCount < results.count {
            throw ExitCode(code: EXIT_FAILURE)
        }
    }
}

// MARK: - Exit Code Support

struct ExitCode: Error {
    let code: Int32
}
