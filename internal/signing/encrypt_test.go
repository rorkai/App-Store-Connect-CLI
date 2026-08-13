package signing

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := []byte("Hello, signing sync!")
	password := "test-password-123"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if bytes.Equal(encrypted, plaintext) {
		t.Fatal("encrypted data should differ from plaintext")
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted data doesn't match: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongPassword(t *testing.T) {
	plaintext := []byte("secret data")
	encrypted, err := Encrypt(plaintext, "correct-password")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = Decrypt(encrypted, "wrong-password")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestDecryptTooShort(t *testing.T) {
	_, err := Decrypt([]byte("short"), "password")
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestEncryptProducesDifferentOutput(t *testing.T) {
	plaintext := []byte("same input")
	password := "same-password"

	enc1, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("encrypt 1: %v", err)
	}

	enc2, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("encrypt 2: %v", err)
	}

	if bytes.Equal(enc1, enc2) {
		t.Fatal("two encryptions of same data should produce different output (random salt/nonce)")
	}
}

func TestEncryptEmptyData(t *testing.T) {
	encrypted, err := Encrypt([]byte{}, "password")
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}

	decrypted, err := Decrypt(encrypted, "password")
	if err != nil {
		t.Fatalf("decrypt empty: %v", err)
	}

	if len(decrypted) != 0 {
		t.Fatalf("expected empty, got %d bytes", len(decrypted))
	}
}

func TestEncryptedEnvelopeRoundTripAuthenticatesMetadata(t *testing.T) {
	metadata := EncryptedFileMetadata{
		Kind:              "pkcs12-identity",
		RelativePath:      "identities/distribution/ABC.p12",
		Sensitive:         true,
		CertificateSHA256: "ABC",
		TeamID:            "TEAM123",
		BundleID:          "com.example.app",
		ProfileType:       "IOS_APP_ADHOC",
	}

	encrypted, err := EncryptFile([]byte("secret identity"), "repository-password", metadata)
	if err != nil {
		t.Fatalf("EncryptFile() error = %v", err)
	}
	if !bytes.HasPrefix(encrypted, []byte(encryptedFileMagic)) {
		t.Fatalf("encrypted file is missing versioned envelope magic")
	}

	plaintext, gotMetadata, err := DecryptFile(encrypted, "repository-password")
	if err != nil {
		t.Fatalf("DecryptFile() error = %v", err)
	}
	if string(plaintext) != "secret identity" {
		t.Fatalf("plaintext = %q", plaintext)
	}
	if gotMetadata.Version != encryptedFileVersion || gotMetadata.Kind != metadata.Kind ||
		gotMetadata.RelativePath != metadata.RelativePath || !gotMetadata.Sensitive ||
		gotMetadata.CertificateSHA256 != metadata.CertificateSHA256 ||
		gotMetadata.TeamID != metadata.TeamID || gotMetadata.BundleID != metadata.BundleID ||
		gotMetadata.ProfileType != metadata.ProfileType ||
		gotMetadata.KDF != encryptedFileKDF || gotMetadata.ScryptN != scryptN ||
		gotMetadata.ScryptR != scryptR || gotMetadata.ScryptP != scryptP {
		t.Fatalf("metadata = %#v", gotMetadata)
	}

	tampered := bytes.Clone(encrypted)
	index := bytes.Index(tampered, []byte("ABC.p12"))
	if index < 0 {
		t.Fatal("test could not locate authenticated metadata")
	}
	tampered[index] = 'X'
	if _, _, err := DecryptFile(tampered, "repository-password"); err == nil || !strings.Contains(err.Error(), "authenticate") {
		t.Fatalf("tampered metadata error = %v, want authentication failure", err)
	}
}

func TestDecryptFileReadsLegacyCiphertext(t *testing.T) {
	legacy, err := Encrypt([]byte("legacy certificate"), "password")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	plaintext, metadata, err := DecryptFile(legacy, "password")
	if err != nil {
		t.Fatalf("DecryptFile() error = %v", err)
	}
	if string(plaintext) != "legacy certificate" {
		t.Fatalf("plaintext = %q", plaintext)
	}
	if metadata.Version != 0 {
		t.Fatalf("legacy metadata = %#v", metadata)
	}
}

func TestDecryptFileRejectsOversizedEnvelopeBeforeDecrypt(t *testing.T) {
	data := make([]byte, maxEncryptedFileSize+1)
	copy(data, encryptedFileMagic)
	if _, _, err := DecryptFile(data, "password"); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("DecryptFile() error = %v, want size limit", err)
	}
}

func TestDecryptFileRejectsOversizedMetadataHeaderBeforeJSON(t *testing.T) {
	data := make([]byte, len(encryptedFileMagic)+4)
	copy(data, encryptedFileMagic)
	binary.BigEndian.PutUint32(data[len(encryptedFileMagic):], uint32(maxEncryptedFileMetadataSize+1))
	if _, _, err := DecryptFile(data, "password"); err == nil || !strings.Contains(err.Error(), "metadata") || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("DecryptFile() error = %v, want metadata size limit", err)
	}
}
