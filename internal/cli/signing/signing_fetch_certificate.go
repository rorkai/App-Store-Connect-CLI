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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	modernpkcs12 "software.sslmate.com/src/go-pkcs12"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type createdSigningIdentity struct {
	CertificateID        string
	CertificateSHA256    string
	PrivateKeyPath       string
	CSRPath              string
	P12Path              string
	CertificatePath      string
	CertificateAttempted bool
}

type signingCertificateCreateRequest struct {
	CertificateType string
	Outputs         *signingCertificateOutputs
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
	if len(request.Password) == 0 {
		return asc.Resource[asc.CertificateAttributes]{}, createdSigningIdentity{}, fmt.Errorf("identity password is empty")
	}
	outputPaths := []string{request.KeyPath, request.CSRPath, request.P12Path}
	outputs := request.Outputs
	ownedOutputs := false
	if outputs == nil {
		outputs = &signingCertificateOutputs{}
		ownedOutputs = true
	}
	if ownedOutputs {
		defer outputs.Close()
	}
	if err := outputs.Prepare(outputPaths, request.Force); err != nil {
		return asc.Resource[asc.CertificateAttributes]{}, createdSigningIdentity{}, err
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
	if err := outputs.Write(0, keyPEM); err != nil {
		return asc.Resource[asc.CertificateAttributes]{}, createdSigningIdentity{}, fmt.Errorf("write private key: %w", err)
	}
	if err := outputs.Write(1, csrPEM); err != nil {
		return asc.Resource[asc.CertificateAttributes]{}, createdSigningIdentity{}, fmt.Errorf("write certificate request: %w", err)
	}

	identity := createdSigningIdentity{
		PrivateKeyPath:       request.KeyPath,
		CSRPath:              request.CSRPath,
		CertificateAttempted: true,
	}
	created, err := client.CreateCertificate(ctx, base64.StdEncoding.EncodeToString(csrDER), certificateType)
	if err != nil {
		return asc.Resource[asc.CertificateAttributes]{}, identity, fmt.Errorf("create certificate: %w", err)
	}
	identity.CertificateID = created.Data.ID
	certDER, err := decodeBase64Content("certificate", created.Data.Attributes.CertificateContent)
	if err != nil {
		return created.Data, identity, err
	}
	certificate, err := x509.ParseCertificate(certDER)
	if err != nil {
		return created.Data, identity, fmt.Errorf("parse created certificate: %w", err)
	}
	if err := signingCertificateMatchesKey(certificate, privateKey); err != nil {
		return created.Data, identity, err
	}
	p12, err := modernpkcs12.Modern2023.WithRand(rand.Reader).Encode(privateKey, certificate, nil, string(request.Password))
	if err != nil {
		return created.Data, identity, fmt.Errorf("encode p12: %w", err)
	}
	if err := outputs.Write(2, p12); err != nil {
		return created.Data, identity, fmt.Errorf("write p12: %w", err)
	}
	sum := sha256.Sum256(certDER)
	identity.CertificateSHA256 = hex.EncodeToString(sum[:])
	identity.P12Path = request.P12Path
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
	return writeBinaryFile(path, data)
}

type signingProfilesMetadata struct {
	CertificateID     string `json:"certificateId,omitempty"`
	CertificateSHA256 string `json:"certificateSha256,omitempty"`
	P12Path           string `json:"p12Path,omitempty"`
	PrivateKeyPath    string `json:"privateKeyPath,omitempty"`
	ProfilePath       string `json:"profilePath,omitempty"`
}

type signingCertificateOutput struct {
	original  string
	canonical string
	rooted    string
	root      *rootfs.Root
}

// signingCertificateOutputs retains descriptor-backed roots from the final
// preflight before certificate creation through the key, CSR, and PKCS#12
// publications. Explicit output paths may have different operator-selected
// parents, so each distinct parent is anchored independently.
type signingCertificateOutputs struct {
	BasePath string
	prepared bool
	replace  bool
	paths    []signingCertificateOutput
	roots    []*rootfs.Root
}

func (outputs *signingCertificateOutputs) Prepare(paths []string, replace bool) error {
	if outputs == nil {
		return fmt.Errorf("certificate outputs are required")
	}
	if outputs.prepared {
		if outputs.replace != replace || len(outputs.paths) != len(paths) {
			return fmt.Errorf("certificate output plan changed after preflight")
		}
		for index, path := range paths {
			if outputs.paths[index].original != path {
				return fmt.Errorf("certificate output plan changed after preflight")
			}
		}
		return outputs.preflight()
	}

	basePath := strings.TrimSpace(outputs.BasePath)
	if basePath != "" {
		var err error
		basePath, err = filepath.Abs(basePath)
		if err != nil {
			return fmt.Errorf("resolve certificate output root %s: %w", outputs.BasePath, err)
		}
	}
	rootByParent := make(map[string]*rootfs.Root, len(paths))
	seen := make(map[string]string, len(paths))
	prepared := make([]signingCertificateOutput, 0, len(paths))
	closeOnError := func() {
		for _, root := range rootByParent {
			_ = root.Close()
		}
	}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			closeOnError()
			return fmt.Errorf("certificate output path is required")
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			closeOnError()
			return fmt.Errorf("resolve output path %s: %w", path, err)
		}
		parent := filepath.Dir(absolute)
		physicalParent, err := filepath.EvalSymlinks(parent)
		if err != nil {
			closeOnError()
			return fmt.Errorf("inspect output parent %s: %w", parent, err)
		}
		canonical := filepath.Join(physicalParent, filepath.Base(absolute))
		if previous, exists := seen[canonical]; exists {
			closeOnError()
			return fmt.Errorf("output paths must be distinct: %s aliases %s", path, previous)
		}
		seen[canonical] = path

		anchor := parent
		if basePath != "" && pathWithinDirectory(basePath, absolute) {
			anchor = basePath
		} else {
			parentInfo, err := os.Lstat(parent)
			if err != nil {
				closeOnError()
				return fmt.Errorf("inspect output parent %s: %w", parent, err)
			}
			if parentInfo.Mode()&os.ModeSymlink != 0 {
				closeOnError()
				return fmt.Errorf("inspect output parent %s: %w", parent, rootfs.ErrSymlink)
			}
		}
		root := rootByParent[anchor]
		if root == nil {
			selected, err := rootfs.New(anchor)
			if err != nil {
				closeOnError()
				return fmt.Errorf("anchor output parent %s: %w", anchor, err)
			}
			root = &selected
			rootByParent[anchor] = root
		}
		rooted, err := root.Resolve(absolute)
		if err != nil {
			closeOnError()
			return fmt.Errorf("resolve output path %s: %w", path, err)
		}
		prepared = append(prepared, signingCertificateOutput{
			original:  path,
			canonical: canonical,
			rooted:    rooted,
			root:      root,
		})
	}
	outputs.prepared = true
	outputs.replace = replace
	outputs.paths = prepared
	for _, root := range rootByParent {
		outputs.roots = append(outputs.roots, root)
	}
	if err := outputs.preflight(); err != nil {
		_ = outputs.Close()
		return err
	}
	return nil
}

func pathWithinDirectory(directory, path string) bool {
	relative, err := filepath.Rel(directory, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (outputs *signingCertificateOutputs) preflight() error {
	for _, output := range outputs.paths {
		if err := output.root.CheckDirectoryWritable(filepath.Dir(output.rooted), 0o600); err != nil {
			return fmt.Errorf("inspect output parent %s: %w", filepath.Dir(output.rooted), err)
		}
		var err error
		if outputs.replace {
			err = output.root.CheckWriteFile(output.rooted)
		} else {
			err = output.root.CheckCreateNewFile(output.rooted)
		}
		if err != nil {
			if !outputs.replace && errors.Is(err, os.ErrExist) {
				return fmt.Errorf("output file already exists: %s: %w", output.original, err)
			}
			return fmt.Errorf("preflight output path %s: %w", output.original, err)
		}
	}
	return nil
}

func (outputs *signingCertificateOutputs) CheckDistinct(path string) error {
	if outputs == nil || !outputs.prepared {
		return fmt.Errorf("certificate outputs were not prepared")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve output path %s: %w", path, err)
	}
	physicalParent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return fmt.Errorf("inspect output parent %s: %w", filepath.Dir(absolute), err)
	}
	canonical := filepath.Join(physicalParent, filepath.Base(absolute))
	for _, output := range outputs.paths {
		if canonical == output.canonical {
			return fmt.Errorf("output paths must be distinct: %s aliases %s", path, output.original)
		}
	}
	return nil
}

func (outputs *signingCertificateOutputs) Write(index int, data []byte) error {
	if outputs == nil || !outputs.prepared || index < 0 || index >= len(outputs.paths) {
		return fmt.Errorf("certificate output plan is unavailable")
	}
	output := outputs.paths[index]
	if outputs.replace {
		return output.root.WriteFile(output.rooted, data, 0o600)
	}
	return output.root.CreateNewFile(output.rooted, data, 0o600)
}

func (outputs *signingCertificateOutputs) Close() error {
	if outputs == nil {
		return nil
	}
	var closeErr error
	for _, root := range outputs.roots {
		closeErr = errors.Join(closeErr, root.Close())
	}
	outputs.prepared = false
	outputs.paths = nil
	outputs.roots = nil
	return closeErr
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
	if result == nil {
		return
	}
	if identity.CertificateID != "" {
		seen := false
		for _, certificateID := range result.CertificateIDs {
			if certificateID == identity.CertificateID {
				seen = true
				break
			}
		}
		if !seen {
			result.CertificateIDs = append(result.CertificateIDs, identity.CertificateID)
		}
	}
	result.PrivateKeyPath = identity.PrivateKeyPath
	result.CSRPath = identity.CSRPath
	result.P12Path = identity.P12Path
	if !created {
		return
	}
	value := true
	result.CertificateCreated = &value
	result.CertificateSHA256 = identity.CertificateSHA256
	if identity.CertificatePath != "" {
		result.CertificateFiles = append(result.CertificateFiles, identity.CertificatePath)
	}
}

func applyCreatedIdentityToSyncResult(result *asc.SigningSyncResult, identity createdSigningIdentity, created bool) {
	if result == nil {
		return
	}
	if identity.CertificateID != "" {
		seen := false
		for _, certificateID := range result.CertificateIDs {
			if certificateID == identity.CertificateID {
				seen = true
				break
			}
		}
		if !seen {
			result.CertificateIDs = append(result.CertificateIDs, identity.CertificateID)
		}
	}
	if identity.CertificateAttempted {
		result.CertificateCreationState = "unknown"
	}
	if created {
		result.CertificateCreationState = "created"
	}
}

func noSigningCertificates(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no certificates found for type")
}
