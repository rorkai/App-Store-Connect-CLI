package shared

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"testing"
	"time"

	modernpkcs12 "software.sslmate.com/src/go-pkcs12"
)

func TestEncodePKCS12IdentitySelectsAlgorithms(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "pkcs12 encoding"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	sha1MAC := asn1.ObjectIdentifier{1, 3, 14, 3, 2, 26}
	sha256MAC := asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	pbes2 := asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	pbeSHA3DES := asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 1, 3}

	for _, test := range []struct {
		encoding PKCS12Encoding
		mac      asn1.ObjectIdentifier
		bagAlg   asn1.ObjectIdentifier
		otherAlg asn1.ObjectIdentifier
	}{
		{encoding: "", mac: sha256MAC, bagAlg: pbes2, otherAlg: pbeSHA3DES},
		{encoding: PKCS12EncodingModern, mac: sha256MAC, bagAlg: pbes2, otherAlg: pbeSHA3DES},
		{encoding: PKCS12EncodingLegacy, mac: sha1MAC, bagAlg: pbeSHA3DES, otherAlg: pbes2},
	} {
		t.Run(string(test.encoding), func(t *testing.T) {
			data, err := EncodePKCS12Identity(test.encoding, privateKey, certificate, "secret")
			if err != nil {
				t.Fatalf("EncodePKCS12Identity() error = %v", err)
			}
			decodedKey, decodedCertificate, err := modernpkcs12.Decode(data, "secret")
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !decodedCertificate.Equal(certificate) || !decodedKey.(*rsa.PrivateKey).Equal(privateKey) {
				t.Fatal("decoded identity does not match the input")
			}
			if got := pkcs12MACAlgorithm(t, data); !got.Equal(test.mac) {
				t.Fatalf("MAC algorithm = %v, want %v", got, test.mac)
			}
			if !containsOID(t, data, test.bagAlg) || containsOID(t, data, test.otherAlg) {
				t.Fatalf("bag encryption: want %v and not %v", test.bagAlg, test.otherAlg)
			}
		})
	}

	if _, err := EncodePKCS12Identity("rc2", privateKey, certificate, "secret"); err == nil {
		t.Fatal("EncodePKCS12Identity accepted an unsupported encoding")
	}
}

func pkcs12MACAlgorithm(t *testing.T, data []byte) asn1.ObjectIdentifier {
	t.Helper()
	var pfx struct {
		Version  int
		AuthSafe asn1.RawValue
		MacData  struct {
			Mac struct {
				Algorithm pkix.AlgorithmIdentifier
				Digest    []byte
			}
			MacSalt    []byte
			Iterations int `asn1:"optional,default:1"`
		}
	}
	if _, err := asn1.Unmarshal(data, &pfx); err != nil {
		t.Fatalf("parse PFX: %v", err)
	}
	return pfx.MacData.Mac.Algorithm.Algorithm
}

func containsOID(t *testing.T, data []byte, oid asn1.ObjectIdentifier) bool {
	t.Helper()
	encoded, err := asn1.Marshal(oid)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Contains(data, encoded)
}
