package signing

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	modernpkcs12 "software.sslmate.com/src/go-pkcs12"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type createdSigningIdentity struct {
	CertificateID     string
	CertificateSHA256 string
	PrivateKeyPath    string
	CSRPath           string
	P12Path           string
	CertificatePath   string
}

type signingCertificateCreateRequest struct {
	CertificateType string
	KeyPath         string
	CSRPath         string
	P12Path         string
	Password        []byte
	Force           bool
}

func createMissingSigningCertificate(ctx context.Context, client *asc.Client, request signingCertificateCreateRequest) (asc.Resource[asc.CertificateAttributes], createdSigningIdentity, error) {
	certificateType := strings.TrimSpace(request.CertificateType)
	if certificateType == "" {
		return asc.Resource[asc.CertificateAttributes]{}, createdSigningIdentity{}, fmt.Errorf("certificate type is required")
	}
	for _, path := range []string{request.KeyPath, request.CSRPath, request.P12Path} {
		if strings.TrimSpace(path) == "" {
			return asc.Resource[asc.CertificateAttributes]{}, createdSigningIdentity{}, fmt.Errorf("certificate output path is required")
		}
	}
	if !request.Force {
		if err := ensureOutputPathsAreFree([]string{request.KeyPath, request.CSRPath, request.P12Path}); err != nil {
			return asc.Resource[asc.CertificateAttributes]{}, createdSigningIdentity{}, err
		}
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return asc.Resource[asc.CertificateAttributes]{}, createdSigningIdentity{}, fmt.Errorf("generate private key: %w", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: "asc"},
		SignatureAlgorithm: x509.SHA256WithRSA,
	}, privateKey)
	if err != nil {
		return asc.Resource[asc.CertificateAttributes]{}, createdSigningIdentity{}, fmt.Errorf("generate certificate request: %w", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return asc.Resource[asc.CertificateAttributes]{}, createdSigningIdentity{}, fmt.Errorf("marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	if err := writeBinaryFileReplacing(request.KeyPath, keyPEM, request.Force); err != nil {
		return asc.Resource[asc.CertificateAttributes]{}, createdSigningIdentity{}, fmt.Errorf("write private key: %w", err)
	}
	if err := writeBinaryFileReplacing(request.CSRPath, csrPEM, request.Force); err != nil {
		return asc.Resource[asc.CertificateAttributes]{}, createdSigningIdentity{}, fmt.Errorf("write certificate request: %w", err)
	}

	created, err := client.CreateCertificate(ctx, base64.StdEncoding.EncodeToString(csrDER), certificateType)
	if err != nil {
		return asc.Resource[asc.CertificateAttributes]{}, createdSigningIdentity{
			PrivateKeyPath: request.KeyPath,
			CSRPath:        request.CSRPath,
		}, fmt.Errorf("create certificate: %w", err)
	}
	certDER, err := decodeBase64Content("certificate", created.Data.Attributes.CertificateContent)
	if err != nil {
		return created.Data, createdSigningIdentity{CertificateID: created.Data.ID, PrivateKeyPath: request.KeyPath, CSRPath: request.CSRPath}, err
	}
	certificate, err := x509.ParseCertificate(certDER)
	if err != nil {
		return created.Data, createdSigningIdentity{CertificateID: created.Data.ID, PrivateKeyPath: request.KeyPath, CSRPath: request.CSRPath}, fmt.Errorf("parse created certificate: %w", err)
	}
	if err := signingCertificateMatchesKey(certificate, privateKey); err != nil {
		return created.Data, createdSigningIdentity{CertificateID: created.Data.ID, PrivateKeyPath: request.KeyPath, CSRPath: request.CSRPath}, err
	}
	p12, err := modernpkcs12.Modern2023.WithRand(rand.Reader).Encode(privateKey, certificate, nil, string(request.Password))
	if err != nil {
		return created.Data, createdSigningIdentity{CertificateID: created.Data.ID, PrivateKeyPath: request.KeyPath, CSRPath: request.CSRPath}, fmt.Errorf("encode p12: %w", err)
	}
	if err := writeBinaryFileReplacing(request.P12Path, p12, request.Force); err != nil {
		return created.Data, createdSigningIdentity{CertificateID: created.Data.ID, PrivateKeyPath: request.KeyPath, CSRPath: request.CSRPath}, fmt.Errorf("write p12: %w", err)
	}
	sum := sha256.Sum256(certDER)
	identity := createdSigningIdentity{
		CertificateID:     created.Data.ID,
		CertificateSHA256: hex.EncodeToString(sum[:]),
		PrivateKeyPath:    request.KeyPath,
		CSRPath:           request.CSRPath,
		P12Path:           request.P12Path,
	}
	if created.Data.Attributes.ExpirationDate == "" {
		created.Data.Attributes.ExpirationDate = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	}
	return created.Data, identity, nil
}

func signingCertificateMatchesKey(certificate *x509.Certificate, privateKey *rsa.PrivateKey) error {
	publicDER, err := x509.MarshalPKIXPublicKey(certificate.PublicKey)
	if err != nil {
		return fmt.Errorf("read created certificate public key: %w", err)
	}
	privateDER, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return fmt.Errorf("read generated public key: %w", err)
	}
	if string(publicDER) != string(privateDER) {
		return fmt.Errorf("created certificate does not match the generated private key")
	}
	return nil
}

func writeSigningProfilesMetadata(path string, metadata signingProfilesMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeBinaryFileReplacing(path, data, true)
}

type signingProfilesMetadata struct {
	CertificateID     string `json:"certificateId,omitempty"`
	CertificateSHA256 string `json:"certificateSha256,omitempty"`
	P12Path           string `json:"p12Path,omitempty"`
	PrivateKeyPath    string `json:"privateKeyPath,omitempty"`
	ProfilePath       string `json:"profilePath,omitempty"`
}

func writeBinaryFileReplacing(path string, data []byte, replace bool) error {
	if replace {
		info, err := os.Lstat(path)
		switch {
		case err == nil && info.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("output file is a symlink: %s", path)
		case err == nil:
			if err := os.Remove(path); err != nil {
				return err
			}
		case !os.IsNotExist(err):
			return err
		}
	}
	return writeBinaryFile(path, data)
}

func primarySigningCertificateType(profileType, explicit string) (string, error) {
	value := strings.TrimSpace(explicit)
	if value == "" {
		inferred, err := inferCertificateType(profileType)
		if err != nil {
			return "", err
		}
		value = inferred
	}
	primary := strings.TrimSpace(strings.Split(value, ",")[0])
	if primary == "" {
		return "", fmt.Errorf("certificate type is required")
	}
	return primary, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func applyCreatedIdentity(result *asc.SigningFetchResult, identity createdSigningIdentity, created bool) {
	if result == nil || !created {
		return
	}
	value := true
	result.CertificateCreated = &value
	result.CertificateSHA256 = identity.CertificateSHA256
	result.PrivateKeyPath = identity.PrivateKeyPath
	result.CSRPath = identity.CSRPath
	result.P12Path = identity.P12Path
	if identity.CertificatePath != "" {
		result.CertificateFiles = append(result.CertificateFiles, identity.CertificatePath)
	}
}

func noSigningCertificates(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no certificates found for type")
}
