package signing

import (
	"bytes"
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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"howett.net/plist"
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
	// Flag validation must fail before any client is built; an unusable client
	// keeps the test hermetic if that ever regresses.
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return nil, errors.New("App Store Connect client must not be created during flag validation")
	}))

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

func TestSigningFetchWarnsForDeviceWithoutCreateMissing(t *testing.T) {
	clientCalls := 0
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientCalls++
		return nil, errors.New("client reached after validation")
	}))

	cmd := SigningFetchCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{
		"--bundle-id", "com.example.app",
		"--profile-type", "IOS_APP_DEVELOPMENT",
		"--device", "DEVICE1,DEVICE2",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		runErr = cmd.Run(context.Background())
	})

	if runErr == nil || runErr.Error() != "signing fetch: client reached after validation" {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("deprecated input must not return a usage error: %v", runErr)
	}
	if clientCalls != 1 {
		t.Fatalf("client factory calls = %d, want 1", clientCalls)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	wantWarning := "Warning: --device without --create-missing is deprecated and ignored because device IDs are only applied when creating a profile. Add --create-missing so they can be applied if a profile must be created. This combination will be rejected in 5.0.0.\n"
	if stderr != wantWarning {
		t.Fatalf("stderr = %q, want %q", stderr, wantWarning)
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

func TestSigningFetchHelpPairsDeviceWithCreateMissing(t *testing.T) {
	cmd := SigningFetchCommand()

	deviceFlag := cmd.FlagSet.Lookup("device")
	if deviceFlag == nil {
		t.Fatal("expected --device flag")
	}
	if !strings.Contains(deviceFlag.Usage, "--create-missing") {
		t.Fatalf("--device usage = %q, want it to name --create-missing", deviceFlag.Usage)
	}
	if !strings.Contains(deviceFlag.Usage, "deprecated") || !strings.Contains(deviceFlag.Usage, "5.0.0") {
		t.Fatalf("--device usage = %q, want the transition and rejection release", deviceFlag.Usage)
	}
	if !strings.Contains(cmd.LongHelp, "warning and ignores the device IDs; 5.0.0 will reject") {
		t.Fatalf("long help must describe the --device transition, got %q", cmd.LongHelp)
	}

	for _, line := range strings.Split(cmd.LongHelp, "\n") {
		if !strings.Contains(line, "--device") {
			continue
		}
		if !strings.Contains(line, "--create-missing") {
			t.Fatalf("example uses --device without --create-missing: %q", line)
		}
	}
}

func TestSigningFetchFormatUsesSharedOutputDefault(t *testing.T) {
	tests := []struct {
		name         string
		defaultValue string
	}{
		{name: "resolved table default", defaultValue: "table"},
		{name: "resolved JSON default", defaultValue: "json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shared.ResetDefaultOutputFormat()
			t.Cleanup(shared.ResetDefaultOutputFormat)
			t.Setenv("ASC_DEFAULT_OUTPUT", tt.defaultValue)

			cmd := SigningFetchCommand()
			formatFlag := cmd.FlagSet.Lookup("format")
			if formatFlag == nil {
				t.Fatal("expected --format flag")
			}
			if got := formatFlag.Value.String(); got != tt.defaultValue {
				t.Fatalf("default --format = %q, want %q", got, tt.defaultValue)
			}

			if err := cmd.FlagSet.Parse([]string{"--format", "markdown"}); err != nil {
				t.Fatalf("parse explicit --format: %v", err)
			}
			if got := formatFlag.Value.String(); got != "markdown" {
				t.Fatalf("explicit --format = %q, want markdown", got)
			}
		})
	}
}

func TestSigningOutputPathsCoverProfileAndCertificateFiles(t *testing.T) {
	dir := filepath.Join("tmp", "signing")
	paths := signingOutputPaths(dir, "Created Profile", "profile-created", []asc.Resource[asc.CertificateAttributes]{
		{ID: "cert-1", Attributes: asc.CertificateAttributes{SerialNumber: "CERT1"}},
		{ID: "cert-2", Attributes: asc.CertificateAttributes{}},
	})

	want := []string{
		filepath.Join(dir, "Created Profile.mobileprovision"),
		filepath.Join(dir, "CERT1.cer"),
		filepath.Join(dir, "cert-2.cer"),
	}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("signingOutputPaths() = %v, want %v", paths, want)
	}
}

func TestEnsureOutputPathsAreFreeReportsCollisions(t *testing.T) {
	dir := t.TempDir()
	free := filepath.Join(dir, "free.cer")
	existing := filepath.Join(dir, "existing.cer")
	danglingSymlink := filepath.Join(dir, "dangling.cer")

	if err := os.WriteFile(existing, []byte("certificate"), 0o600); err != nil {
		t.Fatalf("write existing certificate: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "missing.cer"), danglingSymlink); err != nil {
		t.Fatalf("create dangling symlink: %v", err)
	}

	if err := ensureOutputPathsAreFree([]string{free}); err != nil {
		t.Fatalf("ensureOutputPathsAreFree() error for free path: %v", err)
	}

	for _, path := range []string{existing, danglingSymlink} {
		err := ensureOutputPathsAreFree([]string{free, path})
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("ensureOutputPathsAreFree(%q) error = %v, want os.ErrExist", path, err)
		}
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("ensureOutputPathsAreFree(%q) error = %v, want the colliding path named", path, err)
		}
	}
}

func TestFindActiveProfilesUseBundleIDRelationship(t *testing.T) {
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

	profiles, err := findActiveProfiles(
		context.Background(),
		client,
		"bundle-main",
		"IOS_APP_STORE",
	)
	if err != nil {
		t.Fatalf("findActiveProfiles() error: %v", err)
	}
	if len(profiles) != 1 || profiles[0].ID != "profile-main" {
		t.Fatalf("expected exact bundle profile, got %#v", profiles)
	}

	if len(requestPaths) != 1 || requestPaths[0] != "/v1/bundleIds/bundle-main/profiles" {
		t.Fatalf("expected Bundle ID scoped profile lookup, got %v", requestPaths)
	}
}

func TestResolveSigningAssetsUsesOnlyExistingProfileCertificates(t *testing.T) {
	profileContent := base64.StdEncoding.EncodeToString([]byte("profile"))
	profileCertificateContent := base64.StdEncoding.EncodeToString([]byte("profile-certificate"))
	requestPaths := []string{}
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		requestPaths = append(requestPaths, req.URL.Path)

		switch req.URL.Path {
		case "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(
				http.StatusOK,
				fmt.Sprintf(
					`{"data":[{"type":"profiles","id":"profile-main","attributes":{"name":"Main App Store","profileType":"IOS_APP_STORE","profileState":"ACTIVE","profileContent":%q}}]}`,
					profileContent,
				),
			)
		case "/v1/profiles/profile-main/certificates":
			if req.URL.Query().Get("cursor") == "next" {
				return signingFetchJSONResponse(
					http.StatusOK,
					fmt.Sprintf(
						`{"data":[{"type":"certificates","id":"cert-profile-2","attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":"PROFILE2","certificateContent":%q}}]}`,
						profileCertificateContent,
					),
				)
			}
			return signingFetchJSONResponse(
				http.StatusOK,
				fmt.Sprintf(
					`{"data":[{"type":"certificates","id":"cert-profile","attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":"PROFILE","certificateContent":%q}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/profiles/profile-main/certificates?cursor=next"}}`,
					profileCertificateContent,
				),
			)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})

	profile, certificates, created, err := resolveSigningAssets(
		context.Background(),
		client,
		signingAssetsOptions{
			BundleIDResourceID: "bundle-main",
			BundleIdentifier:   "com.example.signing.profile",
			ProfileType:        "IOS_APP_STORE",
		},
	)
	if err != nil {
		t.Fatalf("resolveSigningAssets() error: %v", err)
	}
	if created {
		t.Fatal("expected existing profile, got created profile")
	}
	if profile.Data.ID != "profile-main" {
		t.Fatalf("expected profile-main, got %s", profile.Data.ID)
	}
	if got := strings.Join(extractIDs(certificates.Data), ","); got != "cert-profile,cert-profile-2" {
		t.Fatalf("expected only the paginated profile certificates, got %s", got)
	}
	if strings.Join(requestPaths, ",") != "/v1/bundleIds/bundle-main/profiles,/v1/profiles/profile-main/certificates,/v1/profiles/profile-main/certificates" {
		t.Fatalf("expected profile-scoped certificate lookup, got %v", requestPaths)
	}
}

func TestResolveSigningAssetsFiltersExistingProfileCertificatesByRequestedType(t *testing.T) {
	tests := []struct {
		name            string
		certificateType string
		wantID          string
		wantErr         string
	}{
		{
			name:            "matching associated certificate",
			certificateType: "ios_distribution",
			wantID:          "cert-ios",
		},
		{
			name:            "comma separated types include matching certificate",
			certificateType: "DEVELOPER_ID_APPLICATION, ios_distribution",
			wantID:          "cert-ios",
		},
		{
			name:            "no associated certificate matches",
			certificateType: "DEVELOPER_ID_APPLICATION",
			wantErr:         "profile profile-main has no associated certificates of type DEVELOPER_ID_APPLICATION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
				switch req.URL.Path {
				case "/v1/bundleIds/bundle-main/profiles":
					return signingFetchJSONResponse(
						http.StatusOK,
						`{"data":[{"type":"profiles","id":"profile-main","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}]}`,
					)
				case "/v1/profiles/profile-main/certificates":
					return signingFetchJSONResponse(
						http.StatusOK,
						`{"data":[{"type":"certificates","id":"cert-mac","attributes":{"certificateType":"MAC_APP_DISTRIBUTION"}},{"type":"certificates","id":"cert-ios","attributes":{"certificateType":"IOS_DISTRIBUTION"}}]}`,
					)
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
				}
			})

			_, certificates, _, err := resolveSigningAssets(
				context.Background(),
				client,
				signingAssetsOptions{
					BundleIDResourceID: "bundle-main",
					BundleIdentifier:   "com.example.signing.profile",
					ProfileType:        "IOS_APP_STORE",
					CertificateType:    tt.certificateType,
				},
			)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSigningAssets() error: %v", err)
			}
			if got := extractIDs(certificates.Data); len(got) != 1 || got[0] != tt.wantID {
				t.Fatalf("expected only %s, got %v", tt.wantID, got)
			}
		})
	}
}

func TestResolveSigningAssetsSkipsUnusableExistingProfileCertificates(t *testing.T) {
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		switch req.URL.Path {
		case "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(
				http.StatusOK,
				`{"data":[{"type":"profiles","id":"profile-main","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}]}`,
			)
		case "/v1/profiles/profile-main/certificates":
			return signingFetchJSONResponse(
				http.StatusOK,
				`{"data":[
					{"type":"certificates","id":"cert-expired","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2000-01-01T00:00:00Z"}},
					{"type":"certificates","id":"cert-deactivated","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":false,"expirationDate":"2100-01-01T00:00:00Z"}},
					{"type":"certificates","id":"cert-undated","attributes":{"certificateType":"IOS_DISTRIBUTION"}},
					{"type":"certificates","id":"cert-valid","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}
				]}`,
			)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})

	_, certificates, created, err := resolveSigningAssets(
		context.Background(),
		client,
		signingAssetsOptions{
			BundleIDResourceID: "bundle-main",
			BundleIdentifier:   "com.example.signing.profile",
			ProfileType:        "IOS_APP_STORE",
		},
	)
	if err != nil {
		t.Fatalf("resolveSigningAssets() error: %v", err)
	}
	if created {
		t.Fatal("expected the existing profile to be reused")
	}
	// Certificates whose metadata does not prove they are unusable are kept.
	want := "cert-undated,cert-valid"
	if got := strings.Join(extractIDs(certificates.Data), ","); got != want {
		t.Fatalf("certificate IDs = %q, want %q", got, want)
	}
}

func TestResolveSigningAssetsTreatsProfileWithOnlyUnusableCertificatesAsNoMatch(t *testing.T) {
	requestPaths := []string{}
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		requestPaths = append(requestPaths, req.Method+" "+req.URL.Path)
		switch {
		case req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(
				http.StatusOK,
				`{"data":[{"type":"profiles","id":"profile-main","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}]}`,
			)
		case req.URL.Path == "/v1/profiles/profile-main/certificates":
			return signingFetchJSONResponse(
				http.StatusOK,
				`{"data":[{"type":"certificates","id":"cert-expired","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2000-01-01T00:00:00Z"}}]}`,
			)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			return signingFetchJSONResponse(
				http.StatusOK,
				`{"data":[{"type":"certificates","id":"cert-fresh","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}]}`,
			)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			return signingFetchJSONResponse(
				http.StatusCreated,
				`{"data":{"type":"profiles","id":"profile-created","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}}`,
			)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})

	_, _, _, err := resolveSigningAssets(
		context.Background(),
		client,
		signingAssetsOptions{
			BundleIDResourceID: "bundle-main",
			BundleIdentifier:   "com.example.signing.profile",
			ProfileType:        "IOS_APP_STORE",
		},
	)
	if err == nil || !strings.Contains(err.Error(), "no active, unexpired associated certificates") {
		t.Fatalf("resolveSigningAssets() error = %v, want unusable certificate error", err)
	}

	requestPaths = requestPaths[:0]
	profile, certificates, created, err := resolveSigningAssets(
		context.Background(),
		client,
		signingAssetsOptions{
			BundleIDResourceID: "bundle-main",
			BundleIdentifier:   "com.example.signing.profile",
			ProfileType:        "IOS_APP_STORE",
			CreateMissing:      true,
		},
	)
	if err != nil {
		t.Fatalf("resolveSigningAssets() error: %v", err)
	}
	if !created || profile.Data.ID != "profile-created" {
		t.Fatalf("expected a new profile, got created=%v profile=%#v", created, profile)
	}
	if got := strings.Join(extractIDs(certificates.Data), ","); got != "cert-fresh" {
		t.Fatalf("certificate IDs = %q, want cert-fresh", got)
	}
	wantPaths := "GET /v1/bundleIds/bundle-main/profiles,GET /v1/profiles/profile-main/certificates,GET /v1/certificates,POST /v1/profiles"
	if strings.Join(requestPaths, ",") != wantPaths {
		t.Fatalf("unexpected lookup order: %v", requestPaths)
	}
}

func TestResolveSigningAssetsRejectsUnknownCertificateTypesBeforeLookup(t *testing.T) {
	requests := 0
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		requests++
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
	})

	_, _, _, err := resolveSigningAssets(
		context.Background(),
		client,
		signingAssetsOptions{
			BundleIDResourceID: "bundle-main",
			BundleIdentifier:   "com.example.signing.profile",
			ProfileType:        "IOS_APP_STORE",
			CertificateType:    "IOS_DISTRIBUTION,BOGUS",
		},
	)
	if err == nil || !strings.Contains(err.Error(), "unsupported certificate type BOGUS") {
		t.Fatalf("resolveSigningAssets() error = %v, want unsupported certificate type", err)
	}
	if requests != 0 {
		t.Fatalf("expected validation before lookup, got %d requests", requests)
	}
}

func TestResolveSigningCertificateTypesIncludesCompatibleCertificatesForIOSAndTVOSProfiles(t *testing.T) {
	tests := []struct {
		profileType string
		want        string
	}{
		{profileType: "IOS_APP_DEVELOPMENT", want: "IOS_DEVELOPMENT,DEVELOPMENT"},
		{profileType: "IOS_APP_STORE", want: "IOS_DISTRIBUTION,DISTRIBUTION"},
		{profileType: "IOS_APP_ADHOC", want: "IOS_DISTRIBUTION,DISTRIBUTION"},
		{profileType: "IOS_APP_INHOUSE", want: "IOS_DISTRIBUTION,DISTRIBUTION"},
		{profileType: "TVOS_APP_DEVELOPMENT", want: "IOS_DEVELOPMENT,DEVELOPMENT"},
		{profileType: "TVOS_APP_STORE", want: "IOS_DISTRIBUTION,DISTRIBUTION"},
		{profileType: "TVOS_APP_ADHOC", want: "IOS_DISTRIBUTION,DISTRIBUTION"},
		{profileType: "TVOS_APP_INHOUSE", want: "IOS_DISTRIBUTION,DISTRIBUTION"},
	}

	for _, tt := range tests {
		t.Run(tt.profileType, func(t *testing.T) {
			got, err := resolveSigningCertificateTypes(tt.profileType, "")
			if err != nil {
				t.Fatalf("resolveSigningCertificateTypes() error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveSigningCertificateTypes() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSigningCertificateTypesIncludesCompatibleCertificatesForMacProfiles(t *testing.T) {
	tests := []struct {
		profileType string
		want        string
	}{
		{profileType: "MAC_APP_DEVELOPMENT", want: "MAC_APP_DEVELOPMENT,DEVELOPMENT"},
		{profileType: "MAC_CATALYST_APP_DEVELOPMENT", want: "MAC_APP_DEVELOPMENT,DEVELOPMENT"},
		{profileType: "MAC_APP_STORE", want: "MAC_APP_DISTRIBUTION,DISTRIBUTION"},
		{profileType: "MAC_CATALYST_APP_STORE", want: "MAC_APP_DISTRIBUTION,DISTRIBUTION"},
	}

	for _, tt := range tests {
		t.Run(tt.profileType, func(t *testing.T) {
			got, err := resolveSigningCertificateTypes(tt.profileType, "")
			if err != nil {
				t.Fatalf("resolveSigningCertificateTypes() error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveSigningCertificateTypes() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSigningCertificateTypesPreservesExplicitSelection(t *testing.T) {
	got, err := resolveSigningCertificateTypes("MAC_APP_STORE", "mac_app_distribution")
	if err != nil {
		t.Fatalf("resolveSigningCertificateTypes() error: %v", err)
	}
	if got != "MAC_APP_DISTRIBUTION" {
		t.Fatalf("resolveSigningCertificateTypes() = %q, want %q", got, "MAC_APP_DISTRIBUTION")
	}
}

func TestResolveSigningAssetsChecksEveryActiveProfileForInferredCertificateType(t *testing.T) {
	requestPaths := []string{}
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		requestPaths = append(requestPaths, req.URL.Path)
		switch req.URL.Path {
		case "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"profiles","id":"profile-first","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}},{"type":"profiles","id":"profile-second","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}]}`)
		case "/v1/profiles/profile-first/certificates":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-mac","attributes":{"certificateType":"MAC_APP_DISTRIBUTION"}}]}`)
		case "/v1/profiles/profile-second/certificates":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-ios","attributes":{"certificateType":"IOS_DISTRIBUTION"}}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})

	profile, certificates, created, err := resolveSigningAssets(
		context.Background(),
		client,
		signingAssetsOptions{
			BundleIDResourceID: "bundle-main",
			BundleIdentifier:   "com.example.signing.profile",
			ProfileType:        "IOS_APP_STORE",
		},
	)
	if err != nil {
		t.Fatalf("resolveSigningAssets() error: %v", err)
	}
	if created || profile.Data.ID != "profile-second" {
		t.Fatalf("expected matching existing profile, got created=%v profile=%#v", created, profile)
	}
	if got := extractIDs(certificates.Data); len(got) != 1 || got[0] != "cert-ios" {
		t.Fatalf("expected cert-ios, got %v", got)
	}
	wantPaths := "/v1/bundleIds/bundle-main/profiles,/v1/profiles/profile-first/certificates,/v1/profiles/profile-second/certificates"
	if strings.Join(requestPaths, ",") != wantPaths {
		t.Fatalf("unexpected lookup order: %v", requestPaths)
	}
}

func TestResolveSigningAssetsCreatesWhenActiveProfilesLackRequestedCertificate(t *testing.T) {
	requestPaths := []string{}
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		requestPaths = append(requestPaths, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"profiles","id":"profile-first","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}},{"type":"profiles","id":"profile-second","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}]}`)
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/profiles/"):
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-mac","attributes":{"certificateType":"MAC_APP_DISTRIBUTION"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-ios","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			return signingFetchJSONResponse(http.StatusCreated, `{"data":{"type":"profiles","id":"profile-created","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})

	profile, certificates, created, err := resolveSigningAssets(
		context.Background(),
		client,
		signingAssetsOptions{
			BundleIDResourceID: "bundle-main",
			BundleIdentifier:   "com.example.signing.profile",
			ProfileType:        "IOS_APP_STORE",
			CertificateType:    "IOS_DISTRIBUTION",
			CreateMissing:      true,
		},
	)
	if err != nil {
		t.Fatalf("resolveSigningAssets() error: %v", err)
	}
	if !created || profile.Data.ID != "profile-created" {
		t.Fatalf("expected created profile, got created=%v profile=%#v", created, profile)
	}
	if got := extractIDs(certificates.Data); len(got) != 1 || got[0] != "cert-ios" {
		t.Fatalf("expected creation certificate cert-ios, got %v", got)
	}
	wantPaths := "GET /v1/bundleIds/bundle-main/profiles,GET /v1/profiles/profile-first/certificates,GET /v1/profiles/profile-second/certificates,GET /v1/certificates,POST /v1/profiles"
	if strings.Join(requestPaths, ",") != wantPaths {
		t.Fatalf("unexpected lookup and creation order: %v", requestPaths)
	}
}

func TestResolveSigningAssetsDoesNotRecreateAfterCreatedProfileUUIDMismatch(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 35)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
	artifacts, err := prepareSigningIdentityArtifacts(identity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	profilePlist, err := plist.Marshal(map[string]any{
		"UUID": "01234567-89ab-cdef-0123-456789abcdef", "TeamIdentifier": []string{"TEAM123"}, "ApplicationIdentifierPrefix": []string{"TEAM123"},
		"ExpirationDate": time.Now().Add(time.Hour), "DeveloperCertificates": [][]byte{certificate.Raw},
		"ProvisionedDevices": []string{"DEVICE1"}, "Entitlements": map[string]any{"application-identifier": "TEAM123.com.example.app", "get-task-allow": false},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	profileContent := mustSignedCMS(t, profilePlist, certificate, key)
	encodedProfile := base64.StdEncoding.EncodeToString(profileContent)
	encodedCertificate := base64.StdEncoding.EncodeToString(certificate.Raw)
	createCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			if createCount == 0 {
				signingFetchWriteJSON(t, w, http.StatusOK, `{"data":[]}`)
				return
			}
			signingFetchWriteJSON(t, w, http.StatusOK, fmt.Sprintf(`{"data":[{"type":"profiles","id":"profile-main","attributes":{"name":"Ad Hoc","uuid":"11234567-89ab-cdef-0123-456789abcdef","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","profileContent":%q}}]}`, encodedProfile))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			signingFetchWriteJSON(t, w, http.StatusOK, fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-main","attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":"SERIAL","activated":true,"expirationDate":%q,"certificateContent":%q}}]}`, certificate.NotAfter.Format(time.RFC3339), encodedCertificate))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			createCount++
			signingFetchWriteJSON(t, w, http.StatusCreated, fmt.Sprintf(`{"data":{"type":"profiles","id":"profile-main","attributes":{"name":"Ad Hoc","uuid":"11234567-89ab-cdef-0123-456789abcdef","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","profileContent":%q}}}`, encodedProfile))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/profiles/profile-main/certificates":
			signingFetchWriteJSON(t, w, http.StatusOK, fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-main","attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":"SERIAL","certificateContent":%q}}]}`, encodedCertificate))
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)
	client := newSigningFetchServerTestClient(t, server)
	options := signingAssetsOptions{
		BundleIDResourceID: "bundle-main",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_ADHOC",
		CertificateType:    "IOS_DISTRIBUTION",
		DeviceIDs:          []string{"DEVICE1"},
		CreateMissing:      true,
		CertificateFilter:  identityCertificateFilter(identity),
	}

	for attempt := 0; attempt < 2; attempt++ {
		profile, _, created, err := resolveSigningAssets(context.Background(), client, options)
		if err != nil {
			t.Fatalf("resolve attempt %d: %v", attempt+1, err)
		}
		if created != (attempt == 0) {
			t.Fatalf("resolve attempt %d created = %t", attempt+1, created)
		}
		if err := bindSigningIdentityProfile(artifacts, profile, "profiles/adhoc/profile.mobileprovision", profileContent); err == nil || !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("bind attempt %d error = %v, want UUID mismatch", attempt+1, err)
		}
	}
	if createCount != 1 {
		t.Fatalf("profile create requests = %d, want 1", createCount)
	}
}

func TestResolveSigningAssetsCreatesSingleCertificateProfileWithNewestEligibleCertificate(t *testing.T) {
	tests := []struct {
		name        string
		profileType string
		deviceIDs   []string
	}{
		{name: "App Store", profileType: "IOS_APP_STORE"},
		{name: "Ad Hoc", profileType: "IOS_APP_ADHOC", deviceIDs: []string{"device-1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var profileCreateBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
					signingFetchWriteJSON(t, w, http.StatusOK, `{"data":[]}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
					signingFetchWriteJSON(t, w, http.StatusOK, `{
						"data":[
							{"type":"certificates","id":"cert-inactive","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":false,"expirationDate":"2102-01-01T00:00:00Z"}},
							{"type":"certificates","id":"cert-expired","attributes":{"certificateType":"DISTRIBUTION","activated":true,"expirationDate":"2000-01-01T00:00:00Z"}},
							{"type":"certificates","id":"cert-older","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2100-01-01T00:00:00Z"}},
							{"type":"certificates","id":"cert-b","attributes":{"certificateType":"DISTRIBUTION","activated":true,"expirationDate":"2101-01-01T00:00:00Z"}},
							{"type":"certificates","id":"cert-a","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2101-01-01T00:00:00Z"}}
						]
					}`)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
					var err error
					profileCreateBody, err = io.ReadAll(req.Body)
					if err != nil {
						t.Errorf("read profile create request: %v", err)
						http.Error(w, "invalid request body", http.StatusBadRequest)
						return
					}
					signingFetchWriteJSON(t, w, http.StatusCreated, fmt.Sprintf(`{"data":{"type":"profiles","id":"profile-created","attributes":{"profileType":%q,"profileState":"ACTIVE"}}}`, tt.profileType))
				default:
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
					http.Error(w, "unexpected request", http.StatusInternalServerError)
				}
			}))
			t.Cleanup(server.Close)
			client := newSigningFetchServerTestClient(t, server)

			profile, certificates, created, err := resolveSigningAssets(
				context.Background(),
				client,
				signingAssetsOptions{
					BundleIDResourceID: "bundle-main",
					BundleIdentifier:   "com.example.signing.profile",
					ProfileType:        tt.profileType,
					DeviceIDs:          tt.deviceIDs,
					CreateMissing:      true,
				},
			)
			if err != nil {
				t.Fatalf("resolveSigningAssets() error: %v", err)
			}
			if !created || profile.Data.ID != "profile-created" {
				t.Fatalf("expected created profile, got created=%v profile=%#v", created, profile)
			}
			createdWithCertificateIDs := profileCreateCertificateIDs(t, bytes.NewReader(profileCreateBody))
			if got := strings.Join(createdWithCertificateIDs, ","); got != "cert-a" {
				t.Fatalf("profile certificate IDs = %q, want cert-a", got)
			}
			if got := strings.Join(extractIDs(certificates.Data), ","); got != "cert-a" {
				t.Fatalf("returned certificate IDs = %q, want cert-a", got)
			}
		})
	}
}

func TestResolveSigningAssetsPreservesEligibleDevelopmentCertificates(t *testing.T) {
	var profileCreateBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			signingFetchWriteJSON(t, w, http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			if got := req.URL.Query().Get("filter[certificateType]"); got != "IOS_DEVELOPMENT,DEVELOPMENT" {
				t.Errorf("certificate type filter = %q, want IOS_DEVELOPMENT,DEVELOPMENT", got)
			}
			signingFetchWriteJSON(t, w, http.StatusOK, `{
				"data":[
					{"type":"certificates","id":"cert-ios","attributes":{"certificateType":"IOS_DEVELOPMENT","activated":true,"expirationDate":"2100-01-01T00:00:00Z"}},
					{"type":"certificates","id":"cert-unified","attributes":{"certificateType":"DEVELOPMENT","activated":true,"expirationDate":"2101-01-01T00:00:00Z"}},
					{"type":"certificates","id":"cert-inactive","attributes":{"certificateType":"DEVELOPMENT","activated":false,"expirationDate":"2102-01-01T00:00:00Z"}}
				]
			}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			var err error
			profileCreateBody, err = io.ReadAll(req.Body)
			if err != nil {
				t.Errorf("read profile create request: %v", err)
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			signingFetchWriteJSON(t, w, http.StatusCreated, `{"data":{"type":"profiles","id":"profile-created","attributes":{"profileType":"IOS_APP_DEVELOPMENT","profileState":"ACTIVE"}}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)
	client := newSigningFetchServerTestClient(t, server)

	_, certificates, created, err := resolveSigningAssets(
		context.Background(),
		client,
		signingAssetsOptions{
			BundleIDResourceID: "bundle-main",
			BundleIdentifier:   "com.example.signing.profile",
			ProfileType:        "IOS_APP_DEVELOPMENT",
			DeviceIDs:          []string{"device-1"},
			CreateMissing:      true,
		},
	)
	if err != nil {
		t.Fatalf("resolveSigningAssets() error: %v", err)
	}
	if !created {
		t.Fatal("expected created profile")
	}
	createdWithCertificateIDs := profileCreateCertificateIDs(t, bytes.NewReader(profileCreateBody))
	want := "cert-ios,cert-unified"
	if got := strings.Join(createdWithCertificateIDs, ","); got != want {
		t.Fatalf("profile certificate IDs = %q, want %q", got, want)
	}
	if got := strings.Join(extractIDs(certificates.Data), ","); got != want {
		t.Fatalf("returned certificate IDs = %q, want %q", got, want)
	}
}

func TestResolveSigningAssetsRejectsProfileCreationWhenAllCertificatesAreIneligible(t *testing.T) {
	postCalled := false
	preflightCalled := false
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			return signingFetchJSONResponse(http.StatusOK, `{
				"data":[
					{"type":"certificates","id":"cert-inactive","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":false,"expirationDate":"2100-01-01T00:00:00Z"}},
					{"type":"certificates","id":"cert-expired","attributes":{"certificateType":"DISTRIBUTION","activated":true,"expirationDate":"2000-01-01T00:00:00Z"}}
				]
			}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			postCalled = true
			return signingFetchJSONResponse(http.StatusCreated, `{"data":{"type":"profiles","id":"profile-created"}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})

	_, _, _, err := resolveSigningAssets(
		context.Background(),
		client,
		signingAssetsOptions{
			BundleIDResourceID: "bundle-main",
			BundleIdentifier:   "com.example.signing.profile",
			ProfileType:        "IOS_APP_STORE",
			CreateMissing:      true,
			BeforeCreate: func(profileCreatePlan) error {
				preflightCalled = true
				return nil
			},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "no active, unexpired certificates available") {
		t.Fatalf("resolveSigningAssets() error = %v, want no eligible certificate error", err)
	}
	if preflightCalled {
		t.Fatal("repository preflight ran without an eligible certificate")
	}
	if postCalled {
		t.Fatal("profile creation ran without an eligible certificate")
	}
}

func TestCertificatesForProfileCreationAcceptsOmittedActivationButRequiresValidExpiration(t *testing.T) {
	now := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	active := true
	certificates := []asc.Resource[asc.CertificateAttributes]{
		{ID: "cert-missing-activation", Attributes: asc.CertificateAttributes{ExpirationDate: "2031-01-01T00:00:00Z"}},
		{ID: "cert-missing-expiration", Attributes: asc.CertificateAttributes{Activated: &active}},
		{ID: "cert-malformed-expiration", Attributes: asc.CertificateAttributes{Activated: &active, ExpirationDate: "not-a-date"}},
		{ID: "cert-expiring-now", Attributes: asc.CertificateAttributes{Activated: &active, ExpirationDate: "2030-01-01T00:00:00Z"}},
		{ID: "cert-valid", Attributes: asc.CertificateAttributes{Activated: &active, ExpirationDate: "2030-01-01T00:00:01Z"}},
	}

	got := certificatesForProfileCreation(certificates, "IOS_APP_DEVELOPMENT", now)
	if ids := strings.Join(extractIDs(got), ","); ids != "cert-missing-activation,cert-valid" {
		t.Fatalf("eligible certificate IDs = %q, want omitted-activation and valid certificates", ids)
	}
}

func TestIsSingleCertificateProfileMatchesDocumentedTypes(t *testing.T) {
	tests := []struct {
		profileType string
		want        bool
	}{
		{profileType: "IOS_APP_STORE", want: true},
		{profileType: "IOS_APP_ADHOC", want: true},
		{profileType: "IOS_APP_INHOUSE", want: true},
		{profileType: "TVOS_APP_STORE", want: true},
		{profileType: "TVOS_APP_ADHOC", want: true},
		{profileType: "TVOS_APP_INHOUSE", want: true},
		{profileType: "MAC_APP_STORE", want: true},
		{profileType: " mac_catalyst_app_store ", want: true},
		{profileType: "MAC_APP_DIRECT", want: false},
		{profileType: "MAC_CATALYST_APP_DIRECT", want: false},
		{profileType: "IOS_APP_DEVELOPMENT", want: false},
		{profileType: "TVOS_APP_DEVELOPMENT", want: false},
		{profileType: "MAC_APP_DEVELOPMENT", want: false},
		{profileType: "MAC_CATALYST_APP_DEVELOPMENT", want: false},
	}

	for _, tt := range tests {
		t.Run(strings.TrimSpace(tt.profileType), func(t *testing.T) {
			if got := isSingleCertificateProfile(tt.profileType); got != tt.want {
				t.Fatalf("isSingleCertificateProfile(%q) = %v, want %v", tt.profileType, got, tt.want)
			}
		})
	}
}

func TestResolveSigningAssetsPreflightsBeforeCreatingProfile(t *testing.T) {
	certificateContent := base64.StdEncoding.EncodeToString([]byte("certificate"))
	profileContent := base64.StdEncoding.EncodeToString([]byte("profile"))
	events := []string{}
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		events = append(events, req.Method+" "+req.URL.Path)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			return signingFetchJSONResponse(
				http.StatusOK,
				fmt.Sprintf(
					`{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":"CERT1","certificateContent":%q,"activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}]}`,
					certificateContent,
				),
			)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			return signingFetchJSONResponse(
				http.StatusCreated,
				fmt.Sprintf(
					`{"data":{"type":"profiles","id":"profile-created","attributes":{"name":"Created Profile","profileType":"IOS_APP_STORE","profileState":"ACTIVE","profileContent":%q}}}`,
					profileContent,
				),
			)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})

	profile, certificates, created, err := resolveSigningAssets(
		context.Background(),
		client,
		signingAssetsOptions{
			BundleIDResourceID: "bundle-main",
			BundleIdentifier:   "com.example.signing.profile",
			ProfileType:        "IOS_APP_STORE",
			CreateMissing:      true,
			BeforeCreate: func(plan profileCreatePlan) error {
				events = append(events, "repository preflight")
				if plan.ProfileName != profileCreateName("IOS_APP_STORE", time.Now()) {
					t.Errorf("preflight profile name = %q, want the name used to create the profile", plan.ProfileName)
				}
				if got := extractIDs(plan.Certificates); len(got) != 1 || got[0] != "cert-1" {
					t.Errorf("preflight certificates = %v, want the certificates used to create the profile", got)
				}
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("resolveSigningAssets() error: %v", err)
	}
	if !created || profile.Data.ID != "profile-created" {
		t.Fatalf("expected newly created profile, got created=%v id=%s", created, profile.Data.ID)
	}
	if got := extractIDs(certificates.Data); len(got) != 1 || got[0] != "cert-1" {
		t.Fatalf("expected creation certificate, got %v", got)
	}
	wantEvents := []string{
		"GET /v1/bundleIds/bundle-main/profiles",
		"GET /v1/certificates",
		"repository preflight",
		"POST /v1/profiles",
	}
	if strings.Join(events, ",") != strings.Join(wantEvents, ",") {
		t.Fatalf("unexpected operation order: got %v, want %v", events, wantEvents)
	}
}

func TestResolveSigningAssetsRefreshesCreateTimeoutAfterPreflight(t *testing.T) {
	requestCtx, expireRequest := context.WithCancel(context.Background())
	events := []string{}
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		events = append(events, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			return signingFetchJSONResponse(http.StatusCreated, `{"data":{"type":"profiles","id":"profile-created","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})

	profile, _, created, err := resolveSigningAssets(
		requestCtx,
		client,
		signingAssetsOptions{
			BundleIDResourceID: "bundle-main",
			BundleIdentifier:   "com.example.signing.profile",
			ProfileType:        "IOS_APP_STORE",
			CreateMissing:      true,
			BeforeCreate: func(profileCreatePlan) error {
				events = append(events, "repository preflight")
				expireRequest()
				return nil
			},
			CreateContext: func() (context.Context, context.CancelFunc) {
				events = append(events, "refresh request timeout")
				return context.WithCancel(context.Background())
			},
		},
	)
	if err != nil {
		t.Fatalf("resolveSigningAssets() error: %v", err)
	}
	if !created || profile.Data.ID != "profile-created" {
		t.Fatalf("expected created profile, got created=%v profile=%#v", created, profile)
	}
	wantEvents := []string{
		"GET /v1/bundleIds/bundle-main/profiles",
		"GET /v1/certificates",
		"repository preflight",
		"refresh request timeout",
		"POST /v1/profiles",
	}
	if strings.Join(events, ",") != strings.Join(wantEvents, ",") {
		t.Fatalf("unexpected operation order: got %v, want %v", events, wantEvents)
	}
}

type signingFetchRoundTripFunc func(*http.Request) *http.Response

func (fn signingFetchRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req), nil
}

func newSigningFetchTestClient(t *testing.T, fn signingFetchRoundTripFunc) *asc.Client {
	t.Helper()
	return newSigningFetchHTTPClient(t, &http.Client{Transport: fn})
}

type signingFetchServerRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn signingFetchServerRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newSigningFetchServerTestClient(t *testing.T, server *httptest.Server) *asc.Client {
	t.Helper()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	httpClient := server.Client()
	serverTransport := httpClient.Transport
	httpClient.Transport = signingFetchServerRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		routedReq := req.Clone(req.Context())
		routedURL := *req.URL
		routedURL.Scheme = serverURL.Scheme
		routedURL.Host = serverURL.Host
		routedReq.URL = &routedURL
		routedReq.Host = serverURL.Host
		return serverTransport.RoundTrip(routedReq)
	})
	return newSigningFetchHTTPClient(t, httpClient)
}

func newSigningFetchHTTPClient(t *testing.T, httpClient *http.Client) *asc.Client {
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

	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
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

func signingFetchWriteJSON(t *testing.T, w http.ResponseWriter, status int, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := io.WriteString(w, body); err != nil {
		t.Errorf("write JSON response: %v", err)
	}
}

func profileCreateCertificateIDs(t *testing.T, body io.Reader) []string {
	t.Helper()

	var payload asc.ProfileCreateRequest
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		t.Fatalf("decode profile create request: %v", err)
	}
	if payload.Data.Relationships == nil || payload.Data.Relationships.Certificates == nil {
		t.Fatal("profile create request is missing certificate relationships")
	}
	ids := make([]string, 0, len(payload.Data.Relationships.Certificates.Data))
	for _, certificate := range payload.Data.Relationships.Certificates.Data {
		ids = append(ids, certificate.ID)
	}
	return ids
}
