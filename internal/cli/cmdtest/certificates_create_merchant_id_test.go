package cmdtest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// applePayCSRPEM wraps applePayCSRBody so the create command encodes the PEM
// block body as csrContent.
const (
	applePayCSRBody = "MERCHANT_CSR_CONTENT"
	applePayCSRPEM  = `-----BEGIN CERTIFICATE REQUEST-----
TUVSQ0hBTlRfQ1NSX0NPTlRFTlQ=
-----END CERTIFICATE REQUEST-----
`
)

func writeApplePayCSR(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "merchant.csr")
	if err := os.WriteFile(path, []byte(applePayCSRPEM), 0o600); err != nil {
		t.Fatalf("write CSR: %v", err)
	}
	return path
}

// captureCertificateCreateRequest serves POST /v1/certificates and records the
// decoded request body so the tests can assert the merchantId relationship.
func captureCertificateCreateRequest(t *testing.T, payload *asc.CertificateCreateRequest) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/certificates" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(req.Body).Decode(payload); err != nil {
			t.Errorf("decode request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"type":"certificates","id":"cert-1","attributes":{"certificateType":"APPLE_PAY_MERCHANT_IDENTITY","displayName":"Merchant Cert"}}}`)
	}))
	t.Cleanup(server.Close)

	installDefaultTransportForServer(t, server)
}

func assertMerchantIDRelationship(t *testing.T, payload asc.CertificateCreateRequest, certificateType, merchantID string) {
	t.Helper()

	if payload.Data.Attributes.CertificateType != certificateType {
		t.Fatalf("expected attributes.certificateType %q, got %q", certificateType, payload.Data.Attributes.CertificateType)
	}
	decoded, err := base64.StdEncoding.DecodeString(payload.Data.Attributes.CSRContent)
	if err != nil {
		t.Fatalf("decode csrContent: %v", err)
	}
	if string(decoded) != applePayCSRBody {
		t.Fatalf("expected the CSR file contents to be sent, got %q", decoded)
	}
	if payload.Data.Relationships == nil || payload.Data.Relationships.MerchantID == nil {
		t.Fatal("expected relationships.merchantId in the request body")
	}
	if got := payload.Data.Relationships.MerchantID.Data.Type; got != asc.ResourceTypeMerchantIds {
		t.Fatalf("expected relationships.merchantId.data.type merchantIds, got %q", got)
	}
	if got := payload.Data.Relationships.MerchantID.Data.ID; got != merchantID {
		t.Fatalf("expected relationships.merchantId.data.id %q, got %q", merchantID, got)
	}
	if payload.Data.Relationships.PassTypeID != nil {
		t.Fatalf("expected passTypeId to be omitted, got %#v", payload.Data.Relationships.PassTypeID)
	}
}

func TestCertificatesCreate_SendsMerchantIDRelationship(t *testing.T) {
	setupAuth(t)

	var payload asc.CertificateCreateRequest
	captureCertificateCreateRequest(t, &payload)

	csrPath := writeApplePayCSR(t)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"certificates", "create",
			"--certificate-type", "APPLE_PAY_MERCHANT_IDENTITY",
			"--merchant-id", "merchant-1",
			"--csr", csrPath,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run: %v; stderr=%q", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	assertMerchantIDRelationship(t, payload, "APPLE_PAY_MERCHANT_IDENTITY", "merchant-1")

	var response asc.CertificateResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse stdout: %v; stdout=%q", err, stdout)
	}
	if response.Data.ID != "cert-1" {
		t.Fatalf("expected the created certificate on stdout, got %q", stdout)
	}
}

func TestMerchantIDsCertificatesCreate_SendsMerchantIDRelationship(t *testing.T) {
	setupAuth(t)

	var payload asc.CertificateCreateRequest
	captureCertificateCreateRequest(t, &payload)

	csrPath := writeApplePayCSR(t)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"merchant-ids", "certificates", "create",
			"--merchant-id", "merchant-2",
			"--certificate-type", "apple-pay",
			"--csr", csrPath,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run: %v; stderr=%q", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	assertMerchantIDRelationship(t, payload, "APPLE_PAY", "merchant-2")

	var response asc.CertificateResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse stdout: %v; stdout=%q", err, stdout)
	}
	if response.Data.ID != "cert-1" {
		t.Fatalf("expected the created certificate on stdout, got %q", stdout)
	}
}

func TestMerchantIDsCertificatesCreate_GeneratesCSR(t *testing.T) {
	setupAuth(t)

	var payload asc.CertificateCreateRequest
	captureCertificateCreateRequest(t, &payload)

	dir := t.TempDir()
	keyOut := filepath.Join(dir, "merchant.key")
	csrOut := filepath.Join(dir, "merchant.csr")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"merchant-ids", "certificates", "create",
			"--merchant-id", "merchant-3",
			"--certificate-type", "APPLE_PAY_RSA",
			"--generate-csr",
			"--key-out", keyOut,
			"--csr-out", csrOut,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run: %v; stderr=%q", runErr, stderr)
	}
	if payload.Data.Relationships == nil || payload.Data.Relationships.MerchantID == nil {
		t.Fatal("expected relationships.merchantId in the request body")
	}
	if got := payload.Data.Relationships.MerchantID.Data.ID; got != "merchant-3" {
		t.Fatalf("expected relationships.merchantId.data.id merchant-3, got %q", got)
	}
	if payload.Data.Attributes.CSRContent == "" {
		t.Fatal("expected the generated CSR to be sent")
	}
	for _, path := range []string{keyOut, csrOut} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %q to be written: %v", path, err)
		}
	}
}

func TestCertificatesCreateMerchantIDUsageErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "merchant id rejected for non apple pay type",
			args:    []string{"certificates", "create", "--certificate-type", "IOS_DISTRIBUTION", "--merchant-id", "merchant-1", "--csr", "./missing.csr"},
			wantErr: "--merchant-id can only be used with",
		},
		{
			name:    "merchant id rejected with pass type id",
			args:    []string{"certificates", "create", "--certificate-type", "PASS_TYPE_ID", "--merchant-id", "merchant-1", "--pass-type-id", "pass-1", "--csr", "./missing.csr"},
			wantErr: "--merchant-id cannot be used with --pass-type-id",
		},
		{
			name:    "merchant ids create missing merchant id",
			args:    []string{"merchant-ids", "certificates", "create", "--certificate-type", "APPLE_PAY", "--csr", "./missing.csr"},
			wantErr: "--merchant-id is required",
		},
		{
			name:    "merchant ids create missing certificate type",
			args:    []string{"merchant-ids", "certificates", "create", "--merchant-id", "merchant-1", "--csr", "./missing.csr"},
			wantErr: "--certificate-type is required",
		},
		{
			name:    "merchant ids create missing csr",
			args:    []string{"merchant-ids", "certificates", "create", "--merchant-id", "merchant-1", "--certificate-type", "APPLE_PAY"},
			wantErr: "--csr is required",
		},
		{
			name:    "merchant ids create rejects non apple pay type",
			args:    []string{"merchant-ids", "certificates", "create", "--merchant-id", "merchant-1", "--certificate-type", "IOS_DISTRIBUTION", "--csr", "./missing.csr"},
			wantErr: "--merchant-id can only be used with",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
				if !isUsageClassError(err) {
					t.Fatalf("expected usage-class error, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

// Relationship validation must run before --generate-csr writes key material.
func TestMerchantIDsCertificatesCreate_RejectsNonApplePayTypeBeforeGeneratingFiles(t *testing.T) {
	dir := t.TempDir()
	keyOut := filepath.Join(dir, "merchant.key")
	csrOut := filepath.Join(dir, "merchant.csr")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"merchant-ids", "certificates", "create",
			"--merchant-id", "merchant-1",
			"--certificate-type", "IOS_DISTRIBUTION",
			"--generate-csr",
			"--key-out", keyOut,
			"--csr-out", csrOut,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); !isUsageClassError(err) {
			t.Fatalf("expected usage-class error, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--merchant-id can only be used with") {
		t.Fatalf("expected the Apple Pay pairing error, got %q", stderr)
	}
	for _, path := range []string{keyOut, csrOut} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected %q not to be written, stat error: %v", path, err)
		}
	}
}
