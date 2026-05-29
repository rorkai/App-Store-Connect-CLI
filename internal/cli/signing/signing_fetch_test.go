package signing

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	outC := make(chan string)
	errC := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		_ = rOut.Close()
		outC <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		_ = rErr.Close()
		errC <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = wOut.Close()
		_ = wErr.Close()
	}()

	fn()

	_ = wOut.Close()
	_ = wErr.Close()

	stdout := <-outC
	stderr := <-errC

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return stdout, stderr
}

func TestSigningFetchValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing bundle-id",
			args:    []string{"signing", "fetch", "--profile-type", "IOS_APP_STORE"},
			wantErr: "Error: --bundle-id is required",
		},
		{
			name:    "missing profile-type",
			args:    []string{"signing", "fetch", "--bundle-id", "com.example.app"},
			wantErr: "Error: --profile-type is required",
		},
		{
			name:    "missing device for development profile",
			args:    []string{"signing", "fetch", "--bundle-id", "com.example.app", "--profile-type", "IOS_APP_DEVELOPMENT", "--create-missing"},
			wantErr: "Error: --device is required for development profiles",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := SigningFetchCommand()
			cmd.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				args := test.args
				if len(args) >= 2 && args[0] == "signing" && args[1] == "fetch" {
					args = args[2:]
				}
				if err := cmd.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := cmd.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
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

func TestSigningFetchWriteFiles_NoOverwrite(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "profile.mobileprovision")
	certPath := filepath.Join(dir, "cert.cer")

	profileContent := base64.StdEncoding.EncodeToString([]byte("profile"))
	certContent := base64.StdEncoding.EncodeToString([]byte("certificate"))

	profileData, err := decodeBase64Content("profile", profileContent)
	if err != nil {
		t.Fatalf("decode profile error: %v", err)
	}
	if err := shared.WriteProfileFile(profilePath, profileData); err != nil {
		t.Fatalf("writeProfileFile error: %v", err)
	}
	certData, err := decodeBase64Content("certificate", certContent)
	if err != nil {
		t.Fatalf("decode certificate error: %v", err)
	}
	if err := writeBinaryFile(certPath, certData); err != nil {
		t.Fatalf("writeBinaryFile error: %v", err)
	}

	if data, err := os.ReadFile(profilePath); err != nil {
		t.Fatalf("read profile error: %v", err)
	} else if string(data) != "profile" {
		t.Fatalf("unexpected profile content: %q", string(data))
	}

	if data, err := os.ReadFile(certPath); err != nil {
		t.Fatalf("read certificate error: %v", err)
	} else if string(data) != "certificate" {
		t.Fatalf("unexpected certificate content: %q", string(data))
	}

	if err := shared.WriteProfileFile(profilePath, profileData); err == nil {
		t.Fatal("expected error when overwriting profile file")
	} else if !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected ErrExist, got %v", err)
	}

	if err := writeBinaryFile(certPath, certData); err == nil {
		t.Fatal("expected error when overwriting certificate file")
	} else if !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected ErrExist, got %v", err)
	}
}

func TestFindOrCreateProfileUsesBundleIDRelationship(t *testing.T) {
	widgetProfileContent := base64.StdEncoding.EncodeToString([]byte("application-identifier=TEAM.com.example.signing.profile.widget"))
	mainProfileContent := base64.StdEncoding.EncodeToString([]byte("application-identifier=TEAM.com.example.signing.profile"))
	requestPaths := []string{}
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		requestPaths = append(requestPaths, req.URL.Path)

		switch req.URL.Path {
		case "/v1/profiles":
			return signingFetchJSONResponse(
				http.StatusOK,
				fmt.Sprintf(
					`{"data":[{"type":"profiles","id":"profile-widget","attributes":{"name":"Widget-stamped main profile","profileType":"IOS_APP_STORE","profileState":"ACTIVE","profileContent":%q}}]}`,
					widgetProfileContent,
				),
			)
		case "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(
				http.StatusOK,
				fmt.Sprintf(
					`{"data":[{"type":"profiles","id":"profile-main","attributes":{"name":"Main App Store","profileType":"IOS_APP_STORE","profileState":"ACTIVE","profileContent":%q}}]}`,
					mainProfileContent,
				),
			)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})

	profile, created, err := findOrCreateProfile(
		context.Background(),
		client,
		"bundle-main",
		"com.example.signing.profile",
		"IOS_APP_STORE",
		[]string{"cert-1"},
		nil,
		false,
	)
	if err != nil {
		t.Fatalf("findOrCreateProfile() error: %v", err)
	}
	if created {
		t.Fatal("expected existing profile, got created profile")
	}
	if profile.Data.ID != "profile-main" {
		t.Fatalf("expected exact bundle profile, got %s", profile.Data.ID)
	}

	if len(requestPaths) != 1 || requestPaths[0] != "/v1/bundleIds/bundle-main/profiles" {
		t.Fatalf("expected Bundle ID scoped profile lookup, got %v", requestPaths)
	}
}

type signingFetchRoundTripFunc func(*http.Request) *http.Response

func (fn signingFetchRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req), nil
}

func newSigningFetchTestClient(t *testing.T, fn signingFetchRoundTripFunc) *asc.Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "key.p8")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, &http.Client{
		Transport: fn,
	})
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func signingFetchJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
