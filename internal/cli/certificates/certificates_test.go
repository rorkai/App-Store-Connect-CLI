package certificates

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestCertificatesCreateCommand_MissingType(t *testing.T) {
	cmd := CertificatesCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--csr", "./cert.csr"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --certificate-type is missing, got %v", err)
	}
}

func TestCertificatesCreateCommand_MissingCSR(t *testing.T) {
	cmd := CertificatesCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--certificate-type", "IOS_DISTRIBUTION"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --csr is missing, got %v", err)
	}
}

func TestCertificatesCreateCommand_CSRAndGenerateCSRAreMutuallyExclusive(t *testing.T) {
	cmd := CertificatesCreateCommand()

	if err := cmd.FlagSet.Parse([]string{
		"--certificate-type", "IOS_DISTRIBUTION",
		"--csr", "./cert.csr",
		"--generate-csr",
		"--key-out", "./cert.key",
		"--csr-out", "./generated.csr",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for mutually exclusive CSR flags, got %v", err)
	}
}

func TestCertificatesCreateCommand_PassTypeCertificateRequiresPassTypeIDBeforeCSRGeneration(t *testing.T) {
	dir := t.TempDir()
	keyOut := filepath.Join(dir, "pass.key")
	csrOut := filepath.Join(dir, "pass.csr")

	originalGetClient := getCertificatesASCClient
	getCertificatesASCClient = func() (*asc.Client, error) {
		t.Fatal("ASC client should not be resolved when --pass-type-id is missing")
		return nil, nil
	}
	t.Cleanup(func() { getCertificatesASCClient = originalGetClient })

	cmd := CertificatesCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--certificate-type", "PASS_TYPE_ID",
		"--generate-csr",
		"--key-out", keyOut,
		"--csr-out", csrOut,
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --pass-type-id is missing, got %v", err)
	}
	if _, err := os.Stat(keyOut); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("private key should not be generated, stat error: %v", err)
	}
	if _, err := os.Stat(csrOut); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("CSR should not be generated, stat error: %v", err)
	}
}

func TestCertificatesCreateCommand_PassTypeIDRejectsIncompatibleTypeBeforeCSRGeneration(t *testing.T) {
	dir := t.TempDir()
	keyOut := filepath.Join(dir, "distribution.key")
	csrOut := filepath.Join(dir, "distribution.csr")

	originalGetClient := getCertificatesASCClient
	getCertificatesASCClient = func() (*asc.Client, error) {
		t.Fatal("ASC client should not be resolved for an incompatible --pass-type-id")
		return nil, nil
	}
	t.Cleanup(func() { getCertificatesASCClient = originalGetClient })

	cmd := CertificatesCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--certificate-type", "IOS_DISTRIBUTION",
		"--pass-type-id", "pass-123",
		"--generate-csr",
		"--key-out", keyOut,
		"--csr-out", csrOut,
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for an incompatible --pass-type-id, got %v", err)
	}
	if _, err := os.Stat(keyOut); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("private key should not be generated, stat error: %v", err)
	}
	if _, err := os.Stat(csrOut); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("CSR should not be generated, stat error: %v", err)
	}
}

func TestCertificatesCreateCommand_PassTypeIDRelationship(t *testing.T) {
	csrPath := filepath.Join(t.TempDir(), "pass.csr")
	if err := os.WriteFile(csrPath, []byte("CSR_CONTENT"), 0o600); err != nil {
		t.Fatalf("write CSR: %v", err)
	}

	var got asc.CertificateCreateRequest
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/certificates" {
			t.Errorf("expected POST /v1/certificates, got %s %s", req.Method, req.URL.Path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
			t.Errorf("decode request body: %v", err)
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"type":"certificates","id":"cert-1","attributes":{"name":"Pass Cert","certificateType":"PASS_TYPE_ID"}}}`)
	}))
	t.Cleanup(server.Close)

	transport, ok := server.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("server transport type = %T, want *http.Transport", server.Client().Transport)
	}
	transport = transport.Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}
	client := newCertificatesTestClient(t, transport)

	originalGetClient := getCertificatesASCClient
	getCertificatesASCClient = func() (*asc.Client, error) { return client, nil }
	t.Cleanup(func() { getCertificatesASCClient = originalGetClient })

	cmd := CertificatesCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--certificate-type", "pass_type_id",
		"--pass-type-id", "  pass-123  ",
		"--csr", csrPath,
		"--output", "json",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != nil {
		t.Fatalf("exec error: %v", err)
	}
	if got.Data.Relationships == nil || got.Data.Relationships.PassTypeID == nil {
		t.Fatal("expected passTypeId relationship")
	}
	if got.Data.Relationships.PassTypeID.Data.Type != asc.ResourceTypePassTypeIds {
		t.Fatalf("expected passTypeIds relationship type, got %q", got.Data.Relationships.PassTypeID.Data.Type)
	}
	if got.Data.Relationships.PassTypeID.Data.ID != "pass-123" {
		t.Fatalf("expected trimmed pass type ID, got %q", got.Data.Relationships.PassTypeID.Data.ID)
	}
}

func TestCertificatesCreateCommand_GenerateCSRCreatesFilesAndPostsCSR(t *testing.T) {
	dir := t.TempDir()
	keyOut := filepath.Join(dir, "dist.key")
	csrOut := filepath.Join(dir, "dist.csr")

	var got asc.CertificateCreateRequest
	client := newCertificatesTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/certificates" {
			t.Fatalf("expected /v1/certificates, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"certificates","id":"cert-1","attributes":{"name":"Cert","certificateType":"IOS_DISTRIBUTION"}}}`), nil
	}))

	originalGetClient := getCertificatesASCClient
	getCertificatesASCClient = func() (*asc.Client, error) { return client, nil }
	t.Cleanup(func() { getCertificatesASCClient = originalGetClient })

	cmd := CertificatesCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--certificate-type", "IOS_DISTRIBUTION",
		"--generate-csr",
		"--key-out", keyOut,
		"--csr-out", csrOut,
		"--common-name", "ASC Signing",
		"--output", "json",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != nil {
		t.Fatalf("exec error: %v", err)
	}

	if got.Data.Attributes.CertificateType != "IOS_DISTRIBUTION" {
		t.Fatalf("expected certificate type IOS_DISTRIBUTION, got %q", got.Data.Attributes.CertificateType)
	}
	csrPEM, err := os.ReadFile(csrOut)
	if err != nil {
		t.Fatalf("read generated CSR: %v", err)
	}
	csrBlock, _ := pem.Decode(csrPEM)
	if csrBlock == nil {
		t.Fatalf("generated CSR is not PEM")
	}
	if want := base64.StdEncoding.EncodeToString(csrBlock.Bytes); got.Data.Attributes.CSRContent != want {
		t.Fatalf("expected generated CSR content to be posted")
	}
	if _, err := os.Stat(keyOut); err != nil {
		t.Fatalf("expected generated private key: %v", err)
	}

	csr, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	if err != nil {
		t.Fatalf("parse generated CSR: %v", err)
	}
	if err := csr.CheckSignature(); err != nil {
		t.Fatalf("generated CSR signature invalid: %v", err)
	}
	if csr.Subject.CommonName != "ASC Signing" {
		t.Fatalf("expected generated CSR CN ASC Signing, got %q", csr.Subject.CommonName)
	}
}

func TestCertificatesCreateCommand_GenerateCSRPreservesUnicodeSubject(t *testing.T) {
	dir := t.TempDir()
	keyOut := filepath.Join(dir, "unicode.key")
	csrOut := filepath.Join(dir, "unicode.csr")

	var got asc.CertificateCreateRequest
	client := newCertificatesTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"certificates","id":"cert-1","attributes":{"name":"Cert","certificateType":"IOS_DISTRIBUTION"}}}`), nil
	}))

	originalGetClient := getCertificatesASCClient
	getCertificatesASCClient = func() (*asc.Client, error) { return client, nil }
	t.Cleanup(func() { getCertificatesASCClient = originalGetClient })

	const (
		commonName         = "Jos\u00e9 \u0141ukasz"
		organization       = "M\u00fcnchen Labs"
		organizationalUnit = "Cr\u00e9dentials"
	)

	cmd := CertificatesCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--certificate-type", "IOS_DISTRIBUTION",
		"--generate-csr",
		"--key-out", keyOut,
		"--csr-out", csrOut,
		"--common-name", commonName,
		"--organization", organization,
		"--organizational-unit", organizationalUnit,
		"--output", "json",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != nil {
		t.Fatalf("exec error: %v", err)
	}

	csrDER, err := base64.StdEncoding.DecodeString(got.Data.Attributes.CSRContent)
	if err != nil {
		t.Fatalf("decode posted CSR content: %v", err)
	}
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		t.Fatalf("parse posted CSR: %v", err)
	}
	if err := csr.CheckSignature(); err != nil {
		t.Fatalf("posted CSR signature invalid: %v", err)
	}
	if csr.Subject.CommonName != commonName {
		t.Fatalf("expected generated CSR CN %q, got %q", commonName, csr.Subject.CommonName)
	}
	if got := firstString(csr.Subject.Organization); got != organization {
		t.Fatalf("expected generated CSR organization %q, got %q", organization, got)
	}
	if got := firstString(csr.Subject.OrganizationalUnit); got != organizationalUnit {
		t.Fatalf("expected generated CSR organizational unit %q, got %q", organizationalUnit, got)
	}
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func TestCertificatesCreateCommand_GenerateCSRWriteFailures(t *testing.T) {
	tests := []struct {
		name string
		key  func(dir string, parentFile string) string
		csr  func(dir string, parentFile string) string
	}{
		{
			name: "key output parent is file",
			key:  func(dir string, parentFile string) string { return filepath.Join(parentFile, "dist.key") },
			csr:  func(dir string, parentFile string) string { return filepath.Join(dir, "dist.csr") },
		},
		{
			name: "csr output parent is file",
			key:  func(dir string, parentFile string) string { return filepath.Join(dir, "dist.key") },
			csr:  func(dir string, parentFile string) string { return filepath.Join(parentFile, "dist.csr") },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			parentFile := filepath.Join(dir, "parent")
			if err := os.WriteFile(parentFile, []byte{}, 0o644); err != nil {
				t.Fatalf("write parent file: %v", err)
			}
			keyOut := test.key(dir, parentFile)
			csrOut := test.csr(dir, parentFile)

			client := newCertificatesTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("ASC request should not be sent when CSR artifacts cannot be written")
				return nil, nil
			}))
			originalGetClient := getCertificatesASCClient
			getCertificatesASCClient = func() (*asc.Client, error) { return client, nil }
			t.Cleanup(func() { getCertificatesASCClient = originalGetClient })

			cmd := CertificatesCreateCommand()
			if err := cmd.FlagSet.Parse([]string{
				"--certificate-type", "IOS_DISTRIBUTION",
				"--generate-csr",
				"--key-out", keyOut,
				"--csr-out", csrOut,
			}); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			err := cmd.Exec(context.Background(), []string{})
			if err == nil {
				t.Fatalf("expected write failure")
			}
			if !strings.Contains(err.Error(), parentFile) {
				t.Fatalf("expected error to mention parent path %q, got %v", parentFile, err)
			}
			if _, err := os.Stat(keyOut); err == nil {
				t.Fatalf("expected key output to not be created")
			}
			if _, err := os.Stat(csrOut); err == nil {
				t.Fatalf("expected CSR output to not be created")
			}
		})
	}
}

func TestValidateCSRFileOutputPathRejectsAlternateWindowsSeparator(t *testing.T) {
	windowsPathSeparator := func(character uint8) bool {
		return character == '\\' || character == '/'
	}

	if _, err := validateCSRFileOutputPathWithSeparator("output/", windowsPathSeparator); err == nil {
		t.Fatal("expected alternate Windows path separator to be rejected")
	}
}

func TestCertificatesRevokeCommand_MissingID(t *testing.T) {
	cmd := CertificatesRevokeCommand()

	if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newCertificatesTestClient(t *testing.T, transport http.RoundTripper) *asc.Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("WriteFile(keyPath) error: %v", err)
	}

	client, err := asc.NewClientWithHTTPClient("TESTKEY123", "TESTISSUER", keyPath, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func jsonHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestCertificatesRevokeCommand_MissingConfirm(t *testing.T) {
	cmd := CertificatesRevokeCommand()

	if err := cmd.FlagSet.Parse([]string{"--id", "CERT_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
}

func TestCertificatesUpdateCommand_MissingID(t *testing.T) {
	cmd := CertificatesUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{"--activated", "true"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestCertificatesUpdateCommand_MissingActivated(t *testing.T) {
	cmd := CertificatesUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{"--id", "CERT_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --activated is missing, got %v", err)
	}
}

func TestCertificatesGetCommand_MissingID(t *testing.T) {
	cmd := CertificatesGetCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestCertificatesRelationshipsPassTypeIDCommand_MissingID(t *testing.T) {
	cmd := CertificatesRelationshipsPassTypeIDCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}
