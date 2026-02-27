package asc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func TestNewClientFromPEM(t *testing.T) {
	privateKeyPEM := mustGenerateECDSAPEM(t)

	client, err := NewClientFromPEM("KEY123", "ISS456", privateKeyPEM)
	if err != nil {
		t.Fatalf("NewClientFromPEM() error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.privateKey == nil {
		t.Fatal("expected private key to be initialized")
	}
}

func TestNewClientFromPEM_InvalidKey(t *testing.T) {
	_, err := NewClientFromPEM("KEY123", "ISS456", "not-a-valid-key")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid private key") {
		t.Fatalf("expected invalid private key error, got %v", err)
	}
}

func mustGenerateECDSAPEM(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error: %v", err)
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	encoded := pem.EncodeToMemory(block)
	if encoded == nil {
		t.Fatal("pem.EncodeToMemory() returned nil")
	}
	return string(encoded)
}
