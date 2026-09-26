package cmdtest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type matchExtensionsTarget struct {
	bundleResourceID string
	identifier       string
	profileID        string
	certificates     string
}

func matchExtensionsCertificate(id, serial, content string) string {
	return fmt.Sprintf(`{"type":"certificates","id":%q,"attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":%q,"certificateContent":%q,"activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}`, id, serial, content)
}

// startMatchExtensionsStub serves com.app and its extension bundle IDs. The
// bundle ID list mimics App Store Connect's substring filter[identifier] by
// also returning an unrelated com.apple.other and a wildcard com.app.*.
func startMatchExtensionsStub(t *testing.T, targets []matchExtensionsTarget) *atomic.Int32 {
	t.Helper()
	setupAuth(t)

	var mutations atomic.Int32
	byResource := make(map[string]matchExtensionsTarget, len(targets))
	byProfile := make(map[string]matchExtensionsTarget, len(targets))
	bundles := make([]string, 0, len(targets)+2)
	for _, target := range targets {
		byResource[target.bundleResourceID] = target
		byProfile[target.profileID] = target
		bundles = append(bundles, fmt.Sprintf(`{"type":"bundleIds","id":%q,"attributes":{"identifier":%q,"platform":"IOS"}}`, target.bundleResourceID, target.identifier))
	}
	bundles = append(
		bundles,
		`{"type":"bundleIds","id":"bundle-other","attributes":{"identifier":"com.apple.other","platform":"IOS"}}`,
		`{"type":"bundleIds","id":"bundle-wild","attributes":{"identifier":"com.app.*","platform":"IOS"}}`,
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			mutations.Add(1)
			t.Errorf("unexpected mutation: %s %s", req.Method, req.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
			return
		}
		switch {
		case req.URL.Path == "/v1/bundleIds":
			if got := req.URL.Query().Get("filter[identifier]"); got != "com.app" {
				t.Errorf("filter[identifier] = %q, want com.app", got)
			}
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[`+strings.Join(bundles, ",")+`],"links":{}}`)
		case strings.HasPrefix(req.URL.Path, "/v1/bundleIds/") && strings.HasSuffix(req.URL.Path, "/profiles"):
			resourceID := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1/bundleIds/"), "/profiles")
			target, ok := byResource[resourceID]
			if !ok {
				t.Errorf("profiles requested for unmatched bundle %s", resourceID)
				http.Error(w, "unexpected", http.StatusNotFound)
				return
			}
			content := base64.StdEncoding.EncodeToString([]byte("profile-" + target.identifier))
			writeSigningFetchOutputJSON(t, w, http.StatusOK, fmt.Sprintf(
				`{"data":[{"type":"profiles","id":%q,"attributes":{"name":%q,"profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2100-01-01T00:00:00Z","profileContent":%q}}],"links":{}}`,
				target.profileID, "Profile "+target.identifier, content,
			))
		case strings.HasPrefix(req.URL.Path, "/v1/profiles/") && strings.HasSuffix(req.URL.Path, "/certificates"):
			profileID := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1/profiles/"), "/certificates")
			target, ok := byProfile[profileID]
			if !ok {
				t.Errorf("certificates requested for unknown profile %s", profileID)
				http.Error(w, "unexpected", http.StatusNotFound)
				return
			}
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[`+target.certificates+`],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	transport := server.Client().Transport
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			cloned := req.Clone(req.Context())
			cloned.URL.Scheme = serverURL.Scheme
			cloned.URL.Host = serverURL.Host
			return transport.RoundTrip(cloned)
		})},
	)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))
	return &mutations
}

func runMatchExtensionsFetch(t *testing.T, outputDir, format string) (int, string, string) {
	t.Helper()
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"signing", "fetch",
			"--bundle-id", "com.app",
			"--profile-type", "IOS_APP_STORE",
			"--match-extensions",
			"--output", outputDir,
			"--format", format,
		}, "test")
	})
	return code, stdout, stderr
}

type matchExtensionsReceipt struct {
	MatchedBundleIDs []string `json:"matchedBundleIds"`
	Results          []struct {
		BundleID         string   `json:"bundleId"`
		CertificateFiles []string `json:"certificateFiles"`
		ProfileFile      string   `json:"profileFile"`
	} `json:"results"`
	Failures []struct {
		BundleID string `json:"bundleId"`
		Error    string `json:"error"`
	} `json:"failures"`
}

func TestSigningFetchMatchExtensionsSharesCertificateAcrossTargets(t *testing.T) {
	sharedCert := matchExtensionsCertificate("cert-1", "SER1", base64.StdEncoding.EncodeToString([]byte("cert-one")))
	mutations := startMatchExtensionsStub(t, []matchExtensionsTarget{
		{bundleResourceID: "bundle-app", identifier: "com.app", profileID: "profile-app", certificates: sharedCert},
		{bundleResourceID: "bundle-widget", identifier: "com.app.widget", profileID: "profile-widget", certificates: sharedCert},
		{bundleResourceID: "bundle-clip", identifier: "com.app.clip", profileID: "profile-clip", certificates: sharedCert},
	})
	outputDir := t.TempDir()

	code, stdout, stderr := runMatchExtensionsFetch(t, outputDir, "json")
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, stderr = %q, stdout = %q", code, stderr, stdout)
	}
	var receipt matchExtensionsReceipt
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	if got := strings.Join(receipt.MatchedBundleIDs, ","); got != "com.app,com.app.widget,com.app.clip" {
		t.Fatalf("matchedBundleIds = %q; the wildcard and com.apple.other must not match", got)
	}
	if len(receipt.Failures) != 0 || len(receipt.Results) != 3 {
		t.Fatalf("receipt = %+v, want 3 results and no failures", receipt)
	}
	certPath := filepath.Join(outputDir, "SER1.cer")
	for _, result := range receipt.Results {
		if len(result.CertificateFiles) != 1 || result.CertificateFiles[0] != certPath {
			t.Fatalf("%s certificateFiles = %v, want [%s]", result.BundleID, result.CertificateFiles, certPath)
		}
		if _, err := os.Stat(result.ProfileFile); err != nil {
			t.Fatalf("%s profile file: %v", result.BundleID, err)
		}
	}
	if data, err := os.ReadFile(certPath); err != nil || string(data) != "cert-one" {
		t.Fatalf("shared certificate = %q, %v", data, err)
	}
	if got := mutations.Load(); got != 0 {
		t.Fatalf("mutations = %d, want 0", got)
	}
}

func TestSigningFetchMatchExtensionsFailedTargetKeepsSharedCertificate(t *testing.T) {
	sharedCert := matchExtensionsCertificate("cert-1", "SER1", base64.StdEncoding.EncodeToString([]byte("cert-one")))
	badCert := matchExtensionsCertificate("cert-2", "SER2", "!!not-base64!!")
	startMatchExtensionsStub(t, []matchExtensionsTarget{
		{bundleResourceID: "bundle-app", identifier: "com.app", profileID: "profile-app", certificates: sharedCert},
		{bundleResourceID: "bundle-widget", identifier: "com.app.widget", profileID: "profile-widget", certificates: sharedCert + "," + badCert},
	})
	outputDir := t.TempDir()

	code, stdout, _ := runMatchExtensionsFetch(t, outputDir, "json")
	if code == rootcmd.ExitSuccess {
		t.Fatal("expected a failure exit code when one target fails")
	}
	var receipt matchExtensionsReceipt
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	if len(receipt.Results) != 1 || receipt.Results[0].BundleID != "com.app" {
		t.Fatalf("results = %+v, want only com.app", receipt.Results)
	}
	if len(receipt.Failures) != 1 || receipt.Failures[0].BundleID != "com.app.widget" || receipt.Failures[0].Error == "" {
		t.Fatalf("failures = %+v, want com.app.widget", receipt.Failures)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "SER1.cer")); err != nil {
		t.Fatalf("shared certificate removed by the failed target: %v", err)
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "widget") {
			t.Fatalf("failed target left %s behind", entry.Name())
		}
	}
}

func TestSigningFetchMatchExtensionsTableOutputIsATable(t *testing.T) {
	sharedCert := matchExtensionsCertificate("cert-1", "SER1", base64.StdEncoding.EncodeToString([]byte("cert-one")))
	startMatchExtensionsStub(t, []matchExtensionsTarget{
		{bundleResourceID: "bundle-app", identifier: "com.app", profileID: "profile-app", certificates: sharedCert},
		{bundleResourceID: "bundle-widget", identifier: "com.app.widget", profileID: "profile-widget", certificates: sharedCert},
	})

	code, stdout, stderr := runMatchExtensionsFetch(t, t.TempDir(), "table")
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	if strings.HasPrefix(strings.TrimSpace(stdout), "{") || !strings.Contains(stdout, "com.app.widget") || !strings.Contains(stdout, "Bundle ID") {
		t.Fatalf("table output = %q, want a rendered table", stdout)
	}
}

func TestSigningFetchMatchExtensionsRejectsCreateMissingCertificate(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"signing", "fetch",
			"--bundle-id", "com.app",
			"--profile-type", "IOS_APP_STORE",
			"--match-extensions",
			"--create-missing",
			"--create-missing-certificate",
			"--identity-password-file", "password",
			"--output", t.TempDir(),
		}, "test")
	})
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if stdout != "" || !strings.Contains(stderr, "--match-extensions cannot be combined with --create-missing-certificate") {
		t.Fatalf("stdout = %q, stderr = %q", stdout, stderr)
	}
}

func TestSigningSyncPushRejectsMatchExtensionsWithTargetsFile(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"signing", "sync", "push",
			"--targets-file", "targets.json",
			"--match-extensions",
			"--profile-type", "IOS_APP_STORE",
			"--repo", "git@example.com:org/signing.git",
		}, "test")
	})
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, rootcmd.ExitUsage, stderr)
	}
	if stdout != "" || !strings.Contains(stderr, "--match-extensions requires --bundle-id") {
		t.Fatalf("stdout = %q, stderr = %q", stdout, stderr)
	}
}

func TestSigningFetchMatchExtensionsCreatesDistinctProfilePerTarget(t *testing.T) {
	setupAuth(t)
	certContent := base64.StdEncoding.EncodeToString([]byte("cert-one"))
	var created []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds":
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[
				{"type":"bundleIds","id":"bundle-app","attributes":{"identifier":"com.app","platform":"IOS"}},
				{"type":"bundleIds","id":"bundle-widget","attributes":{"identifier":"com.app.widget","platform":"IOS"}}
			],"links":{}}`)
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/profiles"):
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[`+matchExtensionsCertificate("cert-1", "SER1", certContent)+`],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			var body struct {
				Data struct {
					Attributes struct {
						Name string `json:"name"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Errorf("decode create body: %v", err)
			}
			name := body.Data.Attributes.Name
			for _, existing := range created {
				if existing == name {
					writeSigningFetchOutputJSON(t, w, http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR","title":"duplicate name","detail":"Multiple profiles found with the name"}]}`)
					return
				}
			}
			created = append(created, name)
			writeSigningFetchOutputJSON(t, w, http.StatusCreated, fmt.Sprintf(
				`{"data":{"type":"profiles","id":"profile-%d","attributes":{"name":%q,"profileType":"IOS_APP_STORE","profileState":"ACTIVE","profileContent":%q}}}`,
				len(created), name, base64.StdEncoding.EncodeToString([]byte(name)),
			))
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := server.Client().Transport
	client, err := asc.NewClientWithHTTPClient(os.Getenv("ASC_KEY_ID"), os.Getenv("ASC_ISSUER_ID"), os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			cloned := req.Clone(req.Context())
			cloned.URL.Scheme = serverURL.Scheme
			cloned.URL.Host = serverURL.Host
			return transport.RoundTrip(cloned)
		})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	outputDir := t.TempDir()
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"signing", "fetch",
			"--bundle-id", "com.app",
			"--profile-type", "IOS_APP_STORE",
			"--match-extensions", "--create-missing",
			"--output", outputDir,
			"--format", "json",
		}, "test")
	})
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, stderr = %q, stdout = %q", code, stderr, stdout)
	}
	if len(created) != 2 || created[0] == created[1] {
		t.Fatalf("created profile names = %v, want two distinct names", created)
	}
	for i, identifier := range []string{"com.app", "com.app.widget"} {
		if !strings.Contains(created[i], identifier) {
			t.Fatalf("profile name %q does not identify %s", created[i], identifier)
		}
	}
}

type matchStaleStub struct {
	mu       sync.Mutex
	requests []string
}

func (s *matchStaleStub) add(r string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, r)
}

func (s *matchStaleStub) count(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, r := range s.requests {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

// startMatchStaleStub serves com.app and com.app.widget, each with one expired
// and one current IOS_APP_STORE profile that share certificate cert-1.
func startMatchStaleStub(t *testing.T, failDelete string) *matchStaleStub {
	t.Helper()
	setupAuth(t)
	stub := &matchStaleStub{}
	cert := matchExtensionsCertificate("cert-1", "SER1", base64.StdEncoding.EncodeToString([]byte("cert-one")))
	profiles := func(prefix string) string {
		content := base64.StdEncoding.EncodeToString([]byte("profile-" + prefix))
		return fmt.Sprintf(`{"data":[
			{"type":"profiles","id":"stale-%[1]s","attributes":{"name":"Old %[1]s","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2000-01-01T00:00:00Z"}},
			{"type":"profiles","id":"live-%[1]s","attributes":{"name":"Live %[1]s","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2100-01-01T00:00:00Z","profileContent":%[2]q}}
		],"links":{}}`, prefix, content)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		stub.add(req.Method + " " + req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds":
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[
				{"type":"bundleIds","id":"bundle-app","attributes":{"identifier":"com.app","platform":"IOS"}},
				{"type":"bundleIds","id":"bundle-widget","attributes":{"identifier":"com.app.widget","platform":"IOS"}}
			],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-app/profiles":
			writeSigningFetchOutputJSON(t, w, http.StatusOK, profiles("app"))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-widget/profiles":
			writeSigningFetchOutputJSON(t, w, http.StatusOK, profiles("widget"))
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/certificates"):
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[`+cert+`],"links":{}}`)
		case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/v1/profiles/"):
			if strings.TrimPrefix(req.URL.Path, "/v1/profiles/") == failDelete {
				writeSigningFetchOutputJSON(t, w, http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR","title":"Conflict","detail":"cannot delete"}]}`)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := server.Client().Transport
	client, err := asc.NewClientWithHTTPClient(os.Getenv("ASC_KEY_ID"), os.Getenv("ASC_ISSUER_ID"), os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			cloned := req.Clone(req.Context())
			cloned.URL.Scheme = serverURL.Scheme
			cloned.URL.Host = serverURL.Host
			return transport.RoundTrip(cloned)
		})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))
	return stub
}

type matchStaleReceipt struct {
	Results []struct {
		BundleID      string `json:"bundleId"`
		ProfileID     string `json:"profileId"`
		StaleProfiles struct {
			DryRun  bool `json:"dryRun"`
			Planned []struct {
				ID string `json:"id"`
			} `json:"planned"`
			Deleted []struct {
				ID string `json:"id"`
			} `json:"deleted"`
		} `json:"staleProfiles"`
	} `json:"results"`
	Failures []struct {
		BundleID      string `json:"bundleId"`
		StaleProfiles *struct {
			Deleted []struct {
				ID string `json:"id"`
			} `json:"deleted"`
			Failed []struct {
				ID string `json:"id"`
			} `json:"failed"`
		} `json:"staleProfiles"`
	} `json:"failures"`
}

func runMatchStaleFetch(t *testing.T, outputDir string, extra ...string) (int, matchStaleReceipt, string) {
	t.Helper()
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run(append([]string{
			"signing", "fetch",
			"--bundle-id", "com.app",
			"--profile-type", "IOS_APP_STORE",
			"--match-extensions",
			"--delete-stale-profiles",
			"--output", outputDir,
			"--format", "json",
		}, extra...), "test")
	})
	var receipt matchStaleReceipt
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode %q (stderr %q): %v", stdout, stderr, err)
	}
	return code, receipt, stderr
}

func TestSigningFetchMatchExtensionsDeleteStaleDryRunPlansEveryTarget(t *testing.T) {
	stub := startMatchStaleStub(t, "")
	outputDir := filepath.Join(t.TempDir(), "signing")

	code, receipt, stderr := runMatchStaleFetch(t, outputDir, "--dry-run")
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d (stderr %q)", code, stderr)
	}
	if len(receipt.Results) != 2 {
		t.Fatalf("results = %+v, want a plan per target", receipt.Results)
	}
	for _, result := range receipt.Results {
		if !result.StaleProfiles.DryRun || len(result.StaleProfiles.Planned) != 1 || len(result.StaleProfiles.Deleted) != 0 || result.ProfileID != "" {
			t.Fatalf("%s plan = %+v", result.BundleID, result)
		}
	}
	if stub.count("DELETE") != 0 || stub.count("GET /v1/profiles/") != 0 {
		t.Fatalf("dry run deleted or fetched: %v", stub.requests)
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("dry run created the output dir: %v", err)
	}
	if !strings.Contains(stderr, "would delete 2 stale profile(s) across 2 bundle ID(s)") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestSigningFetchMatchExtensionsDeleteStaleConfirmDeletesPerTarget(t *testing.T) {
	stub := startMatchStaleStub(t, "")

	code, receipt, stderr := runMatchStaleFetch(t, t.TempDir(), "--confirm")
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d (stderr %q)", code, stderr)
	}
	if len(receipt.Results) != 2 {
		t.Fatalf("results = %+v", receipt.Results)
	}
	for _, result := range receipt.Results {
		if len(result.StaleProfiles.Deleted) != 1 || !strings.HasPrefix(result.StaleProfiles.Deleted[0].ID, "stale-") || !strings.HasPrefix(result.ProfileID, "live-") {
			t.Fatalf("%s result = %+v", result.BundleID, result)
		}
	}
	if got := stub.count("DELETE"); got != 2 {
		t.Fatalf("deletes = %d, want 2", got)
	}
}

func TestSigningFetchMatchExtensionsDeleteStaleFailureStopsEveryTarget(t *testing.T) {
	stub := startMatchStaleStub(t, "stale-widget")

	code, receipt, stderr := runMatchStaleFetch(t, t.TempDir(), "--confirm")
	if code == rootcmd.ExitSuccess {
		t.Fatal("expected failure when a stale deletion fails")
	}
	if len(receipt.Results) != 0 || len(receipt.Failures) != 2 {
		t.Fatalf("receipt = %+v, want every target reported as failed", receipt)
	}
	for _, failure := range receipt.Failures {
		if failure.StaleProfiles == nil {
			t.Fatalf("%s failure has no stale receipt", failure.BundleID)
		}
	}
	if app := receipt.Failures[0]; app.BundleID != "com.app" || len(app.StaleProfiles.Deleted) != 1 {
		t.Fatalf("com.app receipt = %+v, want its completed deletion recorded", app)
	}
	if widget := receipt.Failures[1]; len(widget.StaleProfiles.Failed) != 1 || len(widget.StaleProfiles.Deleted) != 0 {
		t.Fatalf("widget receipt = %+v", widget)
	}
	if stub.count("GET /v1/profiles/") != 0 {
		t.Fatalf("fetched after a failed deletion: %v", stub.requests)
	}
	if !strings.Contains(stderr, "nothing was fetched or created") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestSigningFetchMatchExtensionsDeleteStaleChecksOutputDirBeforeDeleting(t *testing.T) {
	stub := startMatchStaleStub(t, "")
	outputPath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(outputPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runMatchStaleFetch(t, outputPath, "--confirm")
	if code == rootcmd.ExitSuccess {
		t.Fatal("expected failure for an unusable output directory")
	}
	if !strings.Contains(stderr, "no stale profiles were deleted") {
		t.Fatalf("stderr = %q", stderr)
	}
	if n := stub.count("DELETE"); n != 0 {
		t.Fatalf("delete requests = %d, want 0", n)
	}
}

func TestSigningFetchMatchExtensionsReportsCreatedProfileWhenWriteFails(t *testing.T) {
	setupAuth(t)
	certContent := base64.StdEncoding.EncodeToString([]byte("cert-one"))
	creates := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds":
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[
				{"type":"bundleIds","id":"bundle-app","attributes":{"identifier":"com.app","platform":"IOS"}},
				{"type":"bundleIds","id":"bundle-widget","attributes":{"identifier":"com.app.widget","platform":"IOS"}}
			],"links":{}}`)
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/profiles"):
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[`+matchExtensionsCertificate("cert-1", "SER1", certContent)+`],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			creates++
			content := base64.StdEncoding.EncodeToString([]byte("profile"))
			if creates == 2 {
				content = "!!not-base64!!"
			}
			writeSigningFetchOutputJSON(t, w, http.StatusCreated, fmt.Sprintf(
				`{"data":{"type":"profiles","id":"created-%d","attributes":{"name":"Created %d","profileType":"IOS_APP_STORE","profileState":"ACTIVE","profileContent":%q}}}`,
				creates, creates, content,
			))
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := server.Client().Transport
	client, err := asc.NewClientWithHTTPClient(os.Getenv("ASC_KEY_ID"), os.Getenv("ASC_ISSUER_ID"), os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			cloned := req.Clone(req.Context())
			cloned.URL.Scheme = serverURL.Scheme
			cloned.URL.Host = serverURL.Host
			return transport.RoundTrip(cloned)
		})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"signing", "fetch",
			"--bundle-id", "com.app",
			"--profile-type", "IOS_APP_STORE",
			"--match-extensions", "--create-missing",
			"--output", t.TempDir(),
			"--format", "json",
		}, "test")
	})
	if code == rootcmd.ExitSuccess {
		t.Fatalf("expected failure (stderr %q)", stderr)
	}
	var receipt struct {
		Failures []struct {
			BundleID             string `json:"bundleId"`
			ProfileID            string `json:"profileId"`
			ProfileCreationState string `json:"profileCreationState"`
		} `json:"failures"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	if len(receipt.Failures) != 1 {
		t.Fatalf("failures = %+v", receipt.Failures)
	}
	failure := receipt.Failures[0]
	if failure.BundleID != "com.app.widget" || failure.ProfileID != "created-2" || failure.ProfileCreationState != "created" {
		t.Fatalf("failure = %+v, want the created profile reported", failure)
	}
}
