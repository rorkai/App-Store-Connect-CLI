package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// staleProfilesStub serves a bundle ID whose profiles endpoint returns the
// configured pages and records every request in order.
type staleProfilesStub struct {
	mu       sync.Mutex
	requests []string
}

func (s *staleProfilesStub) record(req *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, req.Method+" "+req.URL.Path)
}

func (s *staleProfilesStub) snapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

func (s *staleProfilesStub) count(prefix string) int {
	n := 0
	for _, request := range s.snapshot() {
		if strings.HasPrefix(request, prefix) {
			n++
		}
	}
	return n
}

type staleProfilesStubConfig struct {
	profilePages []string
	deleteStatus map[string]int
	certificates string
}

func startStaleProfilesStub(t *testing.T, cfg staleProfilesStubConfig) *staleProfilesStub {
	t.Helper()
	setupAuth(t)

	stub := &staleProfilesStub{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		stub.record(req)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds":
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-main","attributes":{"identifier":"com.example.app"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			page := 0
			if value := req.URL.Query().Get("cursor"); value != "" {
				_, _ = fmt.Sscanf(value, "%d", &page)
			}
			if page >= len(cfg.profilePages) {
				t.Errorf("unexpected profile page %d", page)
				http.Error(w, "unexpected", http.StatusInternalServerError)
				return
			}
			next := ""
			if page+1 < len(cfg.profilePages) {
				next = fmt.Sprintf(`"next":"https://api.appstoreconnect.apple.com/v1/bundleIds/bundle-main/profiles?cursor=%d"`, page+1)
			}
			writeSigningFetchOutputJSON(t, w, http.StatusOK, fmt.Sprintf(`{"data":[%s],"links":{%s}}`, cfg.profilePages[page], next))
		case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/v1/profiles/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/profiles/")
			if status, ok := cfg.deleteStatus[id]; ok && status != http.StatusNoContent {
				writeSigningFetchOutputJSON(t, w, status, `{"errors":[{"status":"409","code":"STATE_ERROR","title":"Conflict","detail":"profile cannot be deleted"}]}`)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			writeSigningFetchOutputJSON(t, w, http.StatusOK, cfg.certificates)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)
	installStaleProfilesClient(t, server)
	return stub
}

type staleFetchReceipt struct {
	ProfileID     string `json:"profileId"`
	StaleProfiles *struct {
		DryRun  bool `json:"dryRun"`
		Planned []struct {
			ID string `json:"id"`
		} `json:"planned"`
		Deleted []struct {
			ID string `json:"id"`
		} `json:"deleted"`
		Failed []struct {
			ID    string `json:"id"`
			Error string `json:"error"`
		} `json:"failed"`
	} `json:"staleProfiles"`
}

func decodeStaleFetchReceipt(t *testing.T, stdout string) staleFetchReceipt {
	t.Helper()
	var receipt staleFetchReceipt
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt %q: %v", stdout, err)
	}
	if receipt.StaleProfiles == nil {
		t.Fatalf("receipt has no staleProfiles: %s", stdout)
	}
	return receipt
}

func staleIDs[T any](items []T, id func(T) string) string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, id(item))
	}
	return strings.Join(ids, ",")
}

func runStaleSigningFetch(t *testing.T, extra ...string) (int, string, string) {
	t.Helper()
	args := append([]string{
		"signing", "fetch",
		"--bundle-id", "com.example.app",
		"--profile-type", "IOS_APP_STORE",
		"--format", "json",
	}, extra...)
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run(args, "test")
	})
	return code, stdout, stderr
}

func TestSigningFetchStaleFlagsRequireDeleteStaleProfiles(t *testing.T) {
	for _, tc := range []struct {
		flag string
		want string
	}{
		{flag: "--confirm", want: "--confirm requires --delete-stale-profiles"},
		{flag: "--dry-run", want: "--dry-run requires --delete-stale-profiles"},
	} {
		t.Run(tc.flag, func(t *testing.T) {
			stub := startStaleProfilesStub(t, staleProfilesStubConfig{})
			code, stdout, stderr := runStaleSigningFetch(t, tc.flag, "--output", t.TempDir())
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("stderr = %q, want %q", stderr, tc.want)
			}
			if got := stub.snapshot(); len(got) != 0 {
				t.Fatalf("requests = %v, want none", got)
			}
		})
	}
}

func TestSigningFetchDeleteStaleWithoutConfirmMakesNoRequests(t *testing.T) {
	stub := startStaleProfilesStub(t, staleProfilesStubConfig{})
	code, stdout, stderr := runStaleSigningFetch(t, "--delete-stale-profiles", "--output", t.TempDir())
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if stdout != "" || !strings.Contains(stderr, "--confirm is required with --delete-stale-profiles") {
		t.Fatalf("stdout = %q, stderr = %q", stdout, stderr)
	}
	if got := stub.snapshot(); len(got) != 0 {
		t.Fatalf("requests = %v, want none", got)
	}
}

func TestSigningFetchDeleteStaleDryRunIsPlanOnly(t *testing.T) {
	stub := startStaleProfilesStub(t, staleProfilesStubConfig{
		profilePages: []string{staleProfileJSON("stale-1", "ACTIVE", "2000-01-01T00:00:00Z") + "," + staleProfileJSON("invalid-1", "INVALID", "2100-01-01T00:00:00Z")},
	})
	outputDir := filepath.Join(t.TempDir(), "signing")

	code, stdout, stderr := runStaleSigningFetch(t, "--delete-stale-profiles", "--dry-run", "--create-missing", "--output", outputDir)
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want success (stderr %q)", code, stderr)
	}
	receipt := decodeStaleFetchReceipt(t, stdout)
	if !receipt.StaleProfiles.DryRun {
		t.Fatalf("dryRun = false, want true: %s", stdout)
	}
	if got := staleIDs(receipt.StaleProfiles.Planned, func(p struct {
		ID string `json:"id"`
	},
	) string {
		return p.ID
	}); got != "stale-1,invalid-1" {
		t.Fatalf("planned = %q, want stale-1,invalid-1", got)
	}
	if len(receipt.StaleProfiles.Deleted) != 0 {
		t.Fatalf("deleted = %+v, want none in dry run", receipt.StaleProfiles.Deleted)
	}
	if n := stub.count("DELETE"); n != 0 {
		t.Fatalf("delete requests = %d, want 0", n)
	}
	if n := stub.count("POST"); n != 0 {
		t.Fatalf("create requests = %d, want 0", n)
	}
	if n := stub.count("GET /v1/certificates"); n != 0 {
		t.Fatalf("certificate lookups = %d, want 0 in a plan-only run", n)
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("output dir stat err = %v, want not created", err)
	}
	if !strings.Contains(stderr, "Dry run: would delete 2 stale profile(s)") {
		t.Fatalf("stderr = %q, want dry-run summary", stderr)
	}
}

func TestSigningFetchDeleteStaleRecordsFailuresSeparately(t *testing.T) {
	stub := startStaleProfilesStub(t, staleProfilesStubConfig{
		profilePages: []string{staleProfileJSON("stale-1", "ACTIVE", "2000-01-01T00:00:00Z") + "," + staleProfileJSON("stale-2", "INVALID", "2000-01-01T00:00:00Z")},
		deleteStatus: map[string]int{"stale-2": http.StatusConflict},
	})

	code, stdout, stderr := runStaleSigningFetch(t, "--delete-stale-profiles", "--confirm", "--output", t.TempDir())
	if code == rootcmd.ExitSuccess {
		t.Fatal("expected a failure exit code when a stale profile deletion fails")
	}
	receipt := decodeStaleFetchReceipt(t, stdout)
	if got := len(receipt.StaleProfiles.Planned); got != 2 {
		t.Fatalf("planned = %d, want 2", got)
	}
	if len(receipt.StaleProfiles.Deleted) != 1 || receipt.StaleProfiles.Deleted[0].ID != "stale-1" {
		t.Fatalf("deleted = %+v, want only stale-1", receipt.StaleProfiles.Deleted)
	}
	if len(receipt.StaleProfiles.Failed) != 1 || receipt.StaleProfiles.Failed[0].ID != "stale-2" || receipt.StaleProfiles.Failed[0].Error == "" {
		t.Fatalf("failed = %+v, want stale-2 with an error", receipt.StaleProfiles.Failed)
	}
	if !strings.Contains(stderr, "failed to delete 1 of 2 stale profile(s)") {
		t.Fatalf("stderr = %q, want deletion failure summary", stderr)
	}
	if n := stub.count("GET /v1/certificates"); n != 0 {
		t.Fatalf("certificate lookups = %d, want 0 after a failed deletion", n)
	}
}

func TestSigningFetchDeleteStaleEmitsReceiptWhenFetchFails(t *testing.T) {
	stub := startStaleProfilesStub(t, staleProfilesStubConfig{
		profilePages: []string{staleProfileJSON("stale-1", "ACTIVE", "2000-01-01T00:00:00Z")},
		certificates: `{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":"CERT1","certificateContent":"Y2VydA==","activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}]}`,
	})

	code, stdout, stderr := runStaleSigningFetch(t, "--delete-stale-profiles", "--confirm", "--output", t.TempDir())
	if code == rootcmd.ExitSuccess {
		t.Fatalf("expected failure: the only profile was stale and --create-missing was not set (stdout %q)", stdout)
	}
	receipt := decodeStaleFetchReceipt(t, stdout)
	if len(receipt.StaleProfiles.Deleted) != 1 || receipt.StaleProfiles.Deleted[0].ID != "stale-1" {
		t.Fatalf("deleted = %+v, want stale-1 recorded despite the fetch failure", receipt.StaleProfiles.Deleted)
	}
	if receipt.ProfileID != "" {
		t.Fatalf("profileId = %q, want empty", receipt.ProfileID)
	}
	if stderr == "" {
		t.Fatal("stderr is empty, want the fetch error")
	}
	if n := stub.count("DELETE /v1/profiles/stale-1"); n != 1 {
		t.Fatalf("stale-1 deletes = %d, want 1", n)
	}
}

func TestSigningFetchDeleteStaleReadsEveryPageBeforeDeleting(t *testing.T) {
	startStaleProfilesStub(t, staleProfilesStubConfig{
		profilePages: []string{
			staleProfileJSON("stale-1", "ACTIVE", "2000-01-01T00:00:00Z"),
			staleProfileJSON("stale-2", "ACTIVE", "2000-01-01T00:00:00Z"),
		},
	})

	code, stdout, _ := runStaleSigningFetch(t, "--delete-stale-profiles", "--dry-run", "--output", t.TempDir())
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d", code)
	}
	receipt := decodeStaleFetchReceipt(t, stdout)
	if len(receipt.StaleProfiles.Planned) != 2 {
		t.Fatalf("planned = %+v, want both pages", receipt.StaleProfiles.Planned)
	}

	stub := startStaleProfilesStub(t, staleProfilesStubConfig{
		profilePages: []string{
			staleProfileJSON("stale-1", "ACTIVE", "2000-01-01T00:00:00Z"),
			staleProfileJSON("stale-2", "ACTIVE", "2000-01-01T00:00:00Z"),
		},
	})
	_, _, _ = runStaleSigningFetch(t, "--delete-stale-profiles", "--confirm", "--output", t.TempDir())
	requests := stub.snapshot()
	lastProfileRead, firstDelete := -1, -1
	for i, request := range requests {
		if request == "GET /v1/bundleIds/bundle-main/profiles" && firstDelete == -1 {
			lastProfileRead = i
		}
		if strings.HasPrefix(request, "DELETE") && firstDelete == -1 {
			firstDelete = i
		}
	}
	if firstDelete == -1 || lastProfileRead != firstDelete-1 {
		t.Fatalf("requests = %v, want both profile pages read before the first delete", requests)
	}
	if stub.count("DELETE") != 2 {
		t.Fatalf("requests = %v, want both stale profiles deleted", requests)
	}
}

func TestSigningFetchTableShowsStaleProfiles(t *testing.T) {
	startStaleProfilesStub(t, staleProfilesStubConfig{
		profilePages: []string{staleProfileJSON("stale-1", "ACTIVE", "2000-01-01T00:00:00Z")},
	})
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"signing", "fetch",
			"--bundle-id", "com.example.app",
			"--profile-type", "IOS_APP_STORE",
			"--delete-stale-profiles", "--dry-run",
			"--output", t.TempDir(),
			"--format", "table",
		}, "test")
	})
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, "Stale Planned") || !strings.Contains(stdout, "stale-1") {
		t.Fatalf("table output = %q, want stale columns", stdout)
	}
}

func TestSigningFetchDeleteStaleRejectsUnsupportedTypesBeforeRequests(t *testing.T) {
	for name, extra := range map[string][]string{
		"certificate type": {"--certificate-type", "NOT_A_TYPE"},
		"profile type":     {"--profile-type", "NOT_A_PROFILE_TYPE"},
	} {
		t.Run(name, func(t *testing.T) {
			stub := startStaleProfilesStub(t, staleProfilesStubConfig{})
			args := append([]string{"--delete-stale-profiles", "--confirm", "--output", t.TempDir()}, extra...)
			code, stdout, stderr := runStaleSigningFetch(t, args...)
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" || !strings.Contains(stderr, "--delete-stale-profiles:") {
				t.Fatalf("stdout = %q, stderr = %q", stdout, stderr)
			}
			if got := stub.snapshot(); len(got) != 0 {
				t.Fatalf("requests = %v, want none before validation", got)
			}
		})
	}
}

func TestSigningFetchDeleteStaleChecksOutputDirBeforeDeleting(t *testing.T) {
	stub := startStaleProfilesStub(t, staleProfilesStubConfig{
		profilePages: []string{staleProfileJSON("stale-1", "ACTIVE", "2000-01-01T00:00:00Z")},
	})
	outputPath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(outputPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runStaleSigningFetch(t, "--delete-stale-profiles", "--confirm", "--output", outputPath)
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
