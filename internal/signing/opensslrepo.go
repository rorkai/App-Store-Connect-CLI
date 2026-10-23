package signing

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"hash"

	"golang.org/x/crypto/pbkdf2"
)

// ErrOpenSSLRepositoryDecrypt reports that a file of an OpenSSL-encrypted
// signing repository could not be decrypted with the supplied password.
var ErrOpenSSLRepositoryDecrypt = errors.New("wrong password or not an OpenSSL-encrypted signing file")

const (
	openSSLV1Header        = "Salted__"
	openSSLSaltSize        = 8
	openSSLV2MarkerSize    = 20
	openSSLV2TagSize       = 16
	openSSLV2Iterations    = 10_000
	openSSLV2NonceSize     = 12
	openSSLV2AdditionalLen = 24
)

// DecryptOpenSSLRepositoryFile decrypts one file of an OpenSSL-encrypted
// signing repository. Each file holds standard base64 text (line breaks
// allowed) that decodes to one of two layouts:
//
//	v1: "Salted__" || salt (8) || AES-256-CBC ciphertext with PKCS#7 padding.
//	    Key (32) and IV (16) come from OpenSSL EVP_BytesToKey with one
//	    iteration over password || salt, using MD5 (older OpenSSL defaults)
//	    or SHA-256 (OpenSSL 1.1.0 and later).
//	v2: 20-byte ASCII format marker || salt (8) || GCM tag (16) || AES-256-GCM
//	    ciphertext. PBKDF2-HMAC-SHA256 with 10,000 iterations derives 68
//	    bytes from password and salt: key (32), nonce (12), and additional
//	    authenticated data (24).
//
// v1 has no integrity check, so a wrong password or digest can produce
// garbage with valid padding. accept reports whether a candidate plaintext is
// the expected artifact; every v1 digest is tried until one is accepted. v2
// is authenticated by its tag, and its plaintext must also be accepted.
func DecryptOpenSSLRepositoryFile(data []byte, password string, accept func([]byte) bool) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(bytes.Join(bytes.Fields(data), nil)))
	if err != nil {
		return nil, ErrOpenSSLRepositoryDecrypt
	}
	if bytes.HasPrefix(decoded, []byte(openSSLV1Header)) {
		for _, digest := range []func() hash.Hash{md5.New, sha256.New} {
			plaintext, ok := decryptOpenSSLV1(decoded, password, digest)
			if ok && accept(plaintext) {
				return plaintext, nil
			}
		}
		return nil, ErrOpenSSLRepositoryDecrypt
	}
	plaintext, ok := decryptOpenSSLV2(decoded, password)
	if !ok || !accept(plaintext) {
		return nil, ErrOpenSSLRepositoryDecrypt
	}
	return plaintext, nil
}

func decryptOpenSSLV1(data []byte, password string, digest func() hash.Hash) ([]byte, bool) {
	headerSize := len(openSSLV1Header) + openSSLSaltSize
	if len(data) < headerSize+aes.BlockSize || (len(data)-headerSize)%aes.BlockSize != 0 {
		return nil, false
	}
	salt := data[len(openSSLV1Header):headerSize]
	material := openSSLBytesToKey([]byte(password), salt, digest, keySize+aes.BlockSize)
	block, err := aes.NewCipher(material[:keySize])
	if err != nil {
		return nil, false
	}
	plaintext := make([]byte, len(data)-headerSize)
	cipher.NewCBCDecrypter(block, material[keySize:]).CryptBlocks(plaintext, data[headerSize:])
	padding := int(plaintext[len(plaintext)-1])
	if padding == 0 || padding > aes.BlockSize {
		return nil, false
	}
	for _, value := range plaintext[len(plaintext)-padding:] {
		if int(value) != padding {
			return nil, false
		}
	}
	return plaintext[:len(plaintext)-padding], true
}

// openSSLBytesToKey implements EVP_BytesToKey with an iteration count of one.
func openSSLBytesToKey(password, salt []byte, digest func() hash.Hash, size int) []byte {
	var material, previous []byte
	for len(material) < size {
		h := digest()
		h.Write(previous)
		h.Write(password)
		h.Write(salt)
		previous = h.Sum(nil)
		material = append(material, previous...)
	}
	return material[:size]
}

func decryptOpenSSLV2(data []byte, password string) ([]byte, bool) {
	headerSize := openSSLV2MarkerSize + openSSLSaltSize + openSSLV2TagSize
	if len(data) < headerSize {
		return nil, false
	}
	salt := data[openSSLV2MarkerSize : openSSLV2MarkerSize+openSSLSaltSize]
	tag := data[openSSLV2MarkerSize+openSSLSaltSize : headerSize]
	material := pbkdf2.Key([]byte(password), salt, openSSLV2Iterations, keySize+openSSLV2NonceSize+openSSLV2AdditionalLen, sha256.New)
	block, err := aes.NewCipher(material[:keySize])
	if err != nil {
		return nil, false
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, false
	}
	sealed := append(append([]byte{}, data[headerSize:]...), tag...)
	plaintext, err := gcm.Open(nil, material[keySize:keySize+openSSLV2NonceSize], sealed, material[keySize+openSSLV2NonceSize:])
	if err != nil {
		return nil, false
	}
	return plaintext, true
}
