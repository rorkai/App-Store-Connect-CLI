package signing

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/scrypt"
)

const (
	pbkdf2Iterations     = 100_000
	saltSize             = 16
	keySize              = 32 // AES-256
	encryptedFileMagic   = "ASCENC"
	encryptedFileVersion = 1
	encryptedFileKDF     = "scrypt"
	scryptN              = 32768
	scryptR              = 8
	scryptP              = 1
	// Signing identities are capped at 32 MiB before normalization. The
	// encrypted artifact limit leaves room for PKCS#12 and envelope overhead.
	maxEncryptedFileSize         = 34 << 20
	maxEncryptedFileMetadataSize = 64 << 10
)

// EncryptedFileMetadata is authenticated with a versioned encrypted signing
// artifact. It contains no secret material.
type EncryptedFileMetadata struct {
	Version           int    `json:"version"`
	Kind              string `json:"kind,omitempty"`
	RelativePath      string `json:"relativePath,omitempty"`
	Sensitive         bool   `json:"sensitive,omitempty"`
	CertificateSHA256 string `json:"certificateSha256,omitempty"`
	TeamID            string `json:"teamId,omitempty"`
	BundleID          string `json:"bundleId,omitempty"`
	ProfileType       string `json:"profileType,omitempty"`
	ProfileResourceID string `json:"profileResourceId,omitempty"`
	ProfileUUID       string `json:"profileUuid,omitempty"`
	ProfilePath       string `json:"profilePath,omitempty"`
	ProfileSHA256     string `json:"profileSha256,omitempty"`
	KDF               string `json:"kdf,omitempty"`
	ScryptN           int    `json:"scryptN,omitempty"`
	ScryptR           int    `json:"scryptR,omitempty"`
	ScryptP           int    `json:"scryptP,omitempty"`
}

// Encrypt encrypts plaintext using AES-256-GCM with a password-derived key.
// Output format: salt (16 bytes) || nonce (12 bytes) || ciphertext+tag.
func Encrypt(plaintext []byte, password string) ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	key := pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, keySize, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// salt || nonce || ciphertext+tag
	result := make([]byte, 0, saltSize+len(nonce)+len(ciphertext))
	result = append(result, salt...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)

	return result, nil
}

// Decrypt decrypts data encrypted by Encrypt.
func Decrypt(data []byte, password string) ([]byte, error) {
	if len(data) < saltSize+12 { // salt + minimum nonce
		return nil, fmt.Errorf("encrypted data too short")
	}

	salt := data[:saltSize]
	key := pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, keySize, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < saltSize+nonceSize {
		return nil, fmt.Errorf("encrypted data too short for nonce")
	}

	nonce := data[saltSize : saltSize+nonceSize]
	ciphertext := data[saltSize+nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed (wrong password?): %w", err)
	}

	return plaintext, nil
}

// EncryptFile encrypts plaintext and authenticates the versioned metadata as
// AES-GCM additional data. The envelope is magic || metadata length || metadata
// JSON || salt || nonce || ciphertext+tag.
func EncryptFile(plaintext []byte, password string, metadata EncryptedFileMetadata) ([]byte, error) {
	metadata.Version = encryptedFileVersion
	metadata.KDF = encryptedFileKDF
	metadata.ScryptN = scryptN
	metadata.ScryptR = scryptR
	metadata.ScryptP = scryptP
	authenticatedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal encrypted file metadata: %w", err)
	}
	if len(authenticatedMetadata) > maxEncryptedFileMetadataSize {
		return nil, fmt.Errorf("encrypted file metadata exceeds the %d-byte size limit", maxEncryptedFileMetadataSize)
	}

	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	key, err := scrypt.Key([]byte(password), salt, metadata.ScryptN, metadata.ScryptR, metadata.ScryptP, keySize)
	if err != nil {
		return nil, fmt.Errorf("derive encrypted file key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, authenticatedMetadata)

	result := bytes.NewBuffer(make([]byte, 0, len(encryptedFileMagic)+4+len(authenticatedMetadata)+len(salt)+len(nonce)+len(ciphertext)))
	result.WriteString(encryptedFileMagic)
	if err := binary.Write(result, binary.BigEndian, uint32(len(authenticatedMetadata))); err != nil {
		return nil, fmt.Errorf("encode encrypted file metadata length: %w", err)
	}
	result.Write(authenticatedMetadata)
	result.Write(salt)
	result.Write(nonce)
	result.Write(ciphertext)
	if result.Len() > maxEncryptedFileSize {
		return nil, fmt.Errorf("encrypted file exceeds the %d-byte size limit", maxEncryptedFileSize)
	}
	return result.Bytes(), nil
}

// DecryptFile decrypts a versioned envelope. Legacy unversioned ciphertext
// remains readable and returns zero-valued metadata.
func DecryptFile(data []byte, password string) ([]byte, EncryptedFileMetadata, error) {
	if len(data) > maxEncryptedFileSize {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("encrypted file exceeds the %d-byte size limit", maxEncryptedFileSize)
	}
	if !bytes.HasPrefix(data, []byte(encryptedFileMagic)) {
		plaintext, err := Decrypt(data, password)
		return plaintext, EncryptedFileMetadata{}, err
	}
	minimum := len(encryptedFileMagic) + 4
	if len(data) < minimum {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("encrypted file envelope is truncated")
	}
	metadataLength := int(binary.BigEndian.Uint32(data[len(encryptedFileMagic):minimum]))
	if metadataLength > maxEncryptedFileMetadataSize {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("encrypted file metadata exceeds the %d-byte size limit", maxEncryptedFileMetadataSize)
	}
	if metadataLength <= 0 || metadataLength > len(data)-minimum {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("encrypted file metadata length is invalid")
	}
	metadataBytes := data[minimum : minimum+metadataLength]
	var metadata EncryptedFileMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("decode encrypted file metadata: %w", err)
	}
	if metadata.Version != encryptedFileVersion {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("unsupported encrypted file version %d", metadata.Version)
	}
	if metadata.KDF != encryptedFileKDF || metadata.ScryptN != scryptN || metadata.ScryptR != scryptR || metadata.ScryptP != scryptP {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("unsupported encrypted file KDF parameters")
	}

	payload := data[minimum+metadataLength:]
	if len(payload) < saltSize+12 {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("encrypted file payload is truncated")
	}
	salt := payload[:saltSize]
	key, err := scrypt.Key([]byte(password), salt, metadata.ScryptN, metadata.ScryptR, metadata.ScryptP, keySize)
	if err != nil {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("derive encrypted file key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("create GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(payload) < saltSize+nonceSize {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("encrypted file payload is truncated")
	}
	nonce := payload[saltSize : saltSize+nonceSize]
	ciphertext := payload[saltSize+nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, metadataBytes)
	if err != nil {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("authenticate encrypted file (wrong password or modified metadata): %w", err)
	}
	return plaintext, metadata, nil
}
