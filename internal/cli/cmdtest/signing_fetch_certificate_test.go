package cmdtest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestSigningFetchCreateCertificateRequiresCreateMissingAtCommandBoundary(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"signing", "fetch",
			"--bundle-id", "com.example.app",
			"--profile-type", "IOS_APP_STORE",
			"--create-missing-certificate",
			"--identity-password-file", "password",
		}, "test")
	})
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--create-missing-certificate requires --create-missing") {
		t.Fatalf("stderr = %q, want create-missing diagnostic", stderr)
	}
}

func TestSigningFetchRejectsEmptyNormalizedIdentityPasswordAtCommandBoundary(t *testing.T) {
	passwordPath := t.TempDir() + "/identity-password"
	if err := os.WriteFile(passwordPath, []byte("\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"signing", "fetch",
			"--bundle-id", "com.example.app",
			"--profile-type", "IOS_APP_STORE",
			"--create-missing",
			"--create-missing-certificate",
			"--identity-password-file", passwordPath,
			"--output", t.TempDir(),
		}, "test")
	})
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "identity password file is empty") {
		t.Fatalf("stderr = %q, want empty identity password diagnostic", stderr)
	}
}

func TestSigningFetchCreateNoOpAndPartialReceiptsAtCommandBoundary(t *testing.T) {
	setupAuth(t)
	passwordPath := t.TempDir() + "/identity-password"
	if err := os.WriteFile(passwordPath, []byte("identity-password"), 0o600); err != nil {
		t.Fatal(err)
	}

	certificateContent := ""
	certificateCreated := false
	profileCreated := false
	failProfileCreate := false
	certificatePosts := 0
	profilePosts := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-1/profiles":
			if !profileCreated {
				return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"profiles","id":"profile-1","attributes":{"name":"App Store","profileType":"IOS_APP_STORE","profileState":"ACTIVE","profileContent":"cHJvZmlsZQ=="}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/profiles/profile-1/certificates":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"certificate-1","attributes":{"serialNumber":"serial-1","certificateType":"IOS_DISTRIBUTION","expirationDate":"2100-01-01T00:00:00Z","certificateContent":"`+certificateContent+`"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			if !certificateCreated {
				return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"certificate-1","attributes":{"serialNumber":"serial-1","certificateType":"IOS_DISTRIBUTION","expirationDate":"2100-01-01T00:00:00Z","certificateContent":"`+certificateContent+`"}}]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/certificates":
			certificatePosts++
			var payload struct {
				Data struct {
					Attributes struct {
						CSRContent string `json:"csrContent"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				return nil, err
			}
			csrDER, err := base64.StdEncoding.DecodeString(payload.Data.Attributes.CSRContent)
			if err != nil {
				return nil, err
			}
			csr, err := x509.ParseCertificateRequest(csrDER)
			if err != nil {
				return nil, err
			}
			certificateContent = cmdtestIssuedCertificateContent(t, csr)
			certificateCreated = true
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"certificates","id":"certificate-1","attributes":{"serialNumber":"serial-1","certificateType":"IOS_DISTRIBUTION","expirationDate":"2100-01-01T00:00:00Z","certificateContent":"`+certificateContent+`"}}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			profilePosts++
			if failProfileCreate {
				return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","title":"profile service unavailable"}]}`), nil
			}
			profileCreated = true
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"profiles","id":"profile-1","attributes":{"name":"App Store","profileType":"IOS_APP_STORE","profileState":"ACTIVE","profileContent":"cHJvZmlsZQ=="}}}`), nil
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			return jsonHTTPResponse(http.StatusNotFound, `{"errors":[{"title":"unexpected request"}]}`), nil
		}
	}))

	firstOutput := t.TempDir()
	firstRoot := RootCommand("test")
	firstRoot.FlagSet.SetOutput(io.Discard)
	firstArgs := []string{
		"signing", "fetch",
		"--bundle-id", "com.example.app",
		"--profile-type", "IOS_APP_STORE",
		"--create-missing",
		"--create-missing-certificate",
		"--identity-password-file", passwordPath,
		"--output", firstOutput,
		"--format", "json",
	}
	var firstCode int
	firstStdout, _ := captureOutput(t, func() {
		if err := firstRoot.Parse(firstArgs); err != nil {
			t.Fatalf("first parse error: %v", err)
		}
		firstCode = rootcmd.ExitCodeFromError(firstRoot.Run(context.Background()))
	})
	if firstCode != rootcmd.ExitSuccess || certificatePosts != 1 || profilePosts != 1 {
		t.Fatalf("first run code=%d certificatePosts=%d profilePosts=%d stdout=%q", firstCode, certificatePosts, profilePosts, firstStdout)
	}
	if strings.Contains(firstStdout, "PRIVATE KEY") {
		t.Fatalf("first receipt contains private key material: %q", firstStdout)
	}

	secondRoot := RootCommand("test")
	secondRoot.FlagSet.SetOutput(io.Discard)
	secondArgs := append([]string(nil), firstArgs...)
	secondArgs[11] = t.TempDir()
	var secondCode int
	secondStdout, _ := captureOutput(t, func() {
		if err := secondRoot.Parse(secondArgs); err != nil {
			t.Fatalf("second parse error: %v", err)
		}
		secondCode = rootcmd.ExitCodeFromError(secondRoot.Run(context.Background()))
	})
	if secondCode != rootcmd.ExitSuccess || certificatePosts != 1 || profilePosts != 1 {
		t.Fatalf("no-op run code=%d certificatePosts=%d profilePosts=%d stdout=%q", secondCode, certificatePosts, profilePosts, secondStdout)
	}
	if !strings.Contains(secondStdout, `"certificateCreated":false`) {
		t.Fatalf("no-op receipt = %q, want certificateCreated=false", secondStdout)
	}

	profileCreated = false
	certificateCreated = false
	failProfileCreate = true
	thirdRoot := RootCommand("test")
	thirdRoot.FlagSet.SetOutput(io.Discard)
	thirdArgs := append([]string(nil), firstArgs...)
	thirdArgs[11] = t.TempDir()
	var thirdCode int
	thirdStdout, _ := captureOutput(t, func() {
		if err := thirdRoot.Parse(thirdArgs); err != nil {
			t.Fatalf("third parse error: %v", err)
		}
		thirdCode = rootcmd.ExitCodeFromError(thirdRoot.Run(context.Background()))
	})
	if thirdCode == rootcmd.ExitSuccess || thirdCode == rootcmd.ExitUsage {
		t.Fatalf("partial run code=%d, want non-success API failure; stdout=%q", thirdCode, thirdStdout)
	}
	for _, expected := range []string{`"partial":true`, `"certificateCreationState":"created"`, `"profileCreationState":"unknown"`} {
		if !strings.Contains(thirdStdout, expected) {
			t.Fatalf("partial receipt missing %s: %q", expected, thirdStdout)
		}
	}
}

func cmdtestIssuedCertificateContent(t *testing.T, csr *x509.CertificateRequest) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: csr.Subject.CommonName, OrganizationalUnit: []string{"TEAM123"}},
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
