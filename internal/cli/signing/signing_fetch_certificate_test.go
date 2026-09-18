package signing

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSigningFetchCreateMissingCertificateUsage(t *testing.T) {
	cmd := SigningFetchCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{
		"--bundle-id", "com.example.app",
		"--profile-type", "IOS_APP_STORE",
		"--create-missing-certificate",
		"--identity-password-file", "password",
	}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--create-missing-certificate requires --create-missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestSigningFetchCreatesCertificateThenSkipsOnRerun(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	var posted int
	var certificateContent string
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/profiles") && strings.Contains(req.URL.Path, "/certificates"):
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-1","attributes":{"displayName":"iOS Distribution","expirationDate":"2099-01-01T00:00:00Z","certificateContent":"`+certificateContent+`"}}]}`)
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/profiles"):
			if posted == 0 {
				return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"profiles","id":"profile-1","attributes":{"name":"App Store","profileType":"IOS_APP_STORE","profileState":"ACTIVE","profileContent":"`+base64.StdEncoding.EncodeToString([]byte("profile"))+`"}}]}`)
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/bundleIds"):
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app"}}]}`)
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/certificates"):
			if posted == 0 {
				return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-1","attributes":{"displayName":"iOS Distribution","expirationDate":"2099-01-01T00:00:00Z","certificateContent":"`+certificateContent+`"}}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/certificates":
			posted++
			body, _ := io.ReadAll(req.Body)
			var payload struct {
				Data struct {
					Attributes struct {
						CSRContent      string `json:"csrContent"`
						CertificateType string `json:"certificateType"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode csr: %v", err)
			}
			if payload.Data.Attributes.CertificateType != "IOS_DISTRIBUTION" {
				t.Fatalf("certificate type = %s", payload.Data.Attributes.CertificateType)
			}
			der, err := base64.StdEncoding.DecodeString(payload.Data.Attributes.CSRContent)
			if err != nil {
				t.Fatal(err)
			}
			csr, err := x509.ParseCertificateRequest(der)
			if err != nil {
				t.Fatal(err)
			}
			certificateContent = issuedCertificateContent(t, csr)
			return signingFetchJSONResponse(http.StatusCreated, `{"data":{"type":"certificates","id":"cert-1","attributes":{"displayName":"iOS Distribution","expirationDate":"2099-01-01T00:00:00Z","certificateContent":"`+certificateContent+`"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			return signingFetchJSONResponse(http.StatusCreated, `{"data":{"type":"profiles","id":"profile-1","attributes":{"name":"App Store","profileContent":"`+base64.StdEncoding.EncodeToString([]byte("profile"))+`","profileType":"IOS_APP_STORE"}}}`)
		default:
			t.Fatalf("unexpected %s %s", req.Method, req.URL.Path)
			return nil
		}
	})
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))
	output := t.TempDir()
	password := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(password, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	firstOutput := filepath.Join(output, "first")
	secondOutput := filepath.Join(output, "second")
	run := func(dir string) (string, error) {
		cmd := SigningFetchCommand()
		cmd.FlagSet.SetOutput(io.Discard)
		if err := cmd.Parse([]string{
			"--bundle-id", "com.example.app",
			"--profile-type", "IOS_APP_STORE",
			"--create-missing",
			"--create-missing-certificate",
			"--identity-password-file", password,
			"--output", dir,
			"--format", "json",
		}); err != nil {
			t.Fatal(err)
		}
		var runErr error
		stdout, _ := captureOutput(t, func() {
			runErr = cmd.Run(context.Background())
		})
		return stdout, runErr
	}
	stdout, err := run(firstOutput)
	if err != nil {
		t.Fatalf("first run: %v\n%s", err, stdout)
	}
	if strings.Contains(stdout, "PRIVATE KEY") {
		t.Fatal("stdout included the private key")
	}
	var result struct {
		CertificateCreated *bool  `json:"certificateCreated"`
		P12Path            string `json:"p12Path"`
		PrivateKeyPath     string `json:"privateKeyPath"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result.CertificateCreated == nil || !*result.CertificateCreated || result.P12Path == "" || result.PrivateKeyPath == "" {
		t.Fatalf("result = %#v", result)
	}
	if posted != 1 {
		t.Fatalf("posted %d certificates", posted)
	}
	stdout, err = run(secondOutput)
	if err != nil {
		t.Fatalf("rerun: %v\n%s", err, stdout)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result.CertificateCreated == nil || *result.CertificateCreated {
		t.Fatalf("rerun certificateCreated = %#v", result.CertificateCreated)
	}
	if posted != 1 {
		t.Fatal("rerun posted another certificate")
	}
}

func TestSigningFetchReportsPartialCertificateCreate(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/bundleIds"):
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app"}}]}`)
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/certificates"):
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/certificates":
			body, _ := io.ReadAll(req.Body)
			var payload struct {
				Data struct {
					Attributes struct {
						CSRContent string `json:"csrContent"`
					} `json:"attributes"`
				} `json:"data"`
			}
			_ = json.Unmarshal(body, &payload)
			der, _ := base64.StdEncoding.DecodeString(payload.Data.Attributes.CSRContent)
			csr, err := x509.ParseCertificateRequest(der)
			if err != nil {
				t.Fatal(err)
			}
			content := issuedCertificateContent(t, csr)
			return signingFetchJSONResponse(http.StatusCreated, `{"data":{"type":"certificates","id":"cert-1","attributes":{"displayName":"iOS Distribution","expirationDate":"2099-01-01T00:00:00Z","certificateContent":"`+content+`"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			return signingFetchJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED","detail":"profile create failed"}]}`)
		default:
			t.Fatalf("unexpected %s %s", req.Method, req.URL.Path)
			return nil
		}
	})
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))
	password := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(password, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := SigningFetchCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{
		"--bundle-id", "com.example.app",
		"--profile-type", "IOS_APP_STORE",
		"--create-missing",
		"--create-missing-certificate",
		"--identity-password-file", password,
		"--output", t.TempDir(),
		"--format", "json",
	}); err != nil {
		t.Fatal(err)
	}
	var runErr error
	stdout, _ := captureOutput(t, func() { runErr = cmd.Run(context.Background()) })
	if runErr == nil {
		t.Fatal("expected profile create failure")
	}
	if !strings.Contains(stdout, `"certificateCreated":true`) {
		t.Fatalf("partial receipt missing certificateCreated: %s", stdout)
	}
}

func issuedCertificateContent(t *testing.T, csr *x509.CertificateRequest) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      csr.Subject,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, csr.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}
