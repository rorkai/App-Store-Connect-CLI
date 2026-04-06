package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestRunMetadataKeywordsAuditRejectsEmptyBlockedTermReturnsUsageExitCode(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	}))

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"metadata", "keywords", "audit",
			"--app", "app-1",
			"--version-id", "ver-1",
			"--blocked-term", "   ",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if !strings.Contains(stderr, "--blocked-term must not be empty") {
		t.Fatalf("expected invalid blocked-term stderr, got %q", stderr)
	}
}

func TestRunMetadataKeywordsAuditRejectsEmptyBlockedTermsFileReturnsUsageExitCode(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	blockedTermsPath := filepath.Join(t.TempDir(), "blocked-terms.txt")
	if err := os.WriteFile(blockedTermsPath, []byte("# comment only\n   \n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	}))

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"metadata", "keywords", "audit",
			"--app", "app-1",
			"--version-id", "ver-1",
			"--blocked-terms-file", blockedTermsPath,
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if !strings.Contains(stderr, "--blocked-terms-file must include at least one blocked term") {
		t.Fatalf("expected invalid blocked-terms-file stderr, got %q", stderr)
	}
}

func TestMetadataKeywordsAuditReportsBlockedTermAndStrictBlocking(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	installDefaultTransport(t, metadataKeywordsAuditTransport(t, "free trial,habit tracker"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "audit",
			"--app", "app-1",
			"--version-id", "ver-1",
			"--blocked-term", "free",
			"--strict",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError under --strict, got %T: %v", runErr, runErr)
	}

	var payload struct {
		Strict       bool     `json:"strict"`
		BlockedTerms []string `json:"blockedTerms"`
		Summary      struct {
			Blocking int `json:"blocking"`
		} `json:"summary"`
		Checks []struct {
			ID string `json:"id"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Strict {
		t.Fatalf("expected strict report, got %+v", payload)
	}
	if payload.Summary.Blocking == 0 {
		t.Fatalf("expected blocking findings under strict mode, got %+v", payload.Summary)
	}
	if len(payload.BlockedTerms) != 1 || payload.BlockedTerms[0] != "free" {
		t.Fatalf("expected blocked terms [free], got %+v", payload.BlockedTerms)
	}
	if !hasAuditCheckID(payload.Checks, "metadata.keywords.blocked_term") {
		t.Fatalf("expected blocked-term check, got %+v", payload.Checks)
	}
}

func TestMetadataKeywordsAuditLoadsBlockedTermsFile(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	blockedTermsPath := filepath.Join(t.TempDir(), "blocked-terms.txt")
	if err := os.WriteFile(blockedTermsPath, []byte("free,premium\n# ignore me\nsale\nPremium\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	installDefaultTransport(t, metadataKeywordsAuditTransport(t, "premium features,habit tracker"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "audit",
			"--app", "app-1",
			"--version-id", "ver-1",
			"--blocked-terms-file", blockedTermsPath,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		BlockedTerms []string `json:"blockedTerms"`
		Checks       []struct {
			ID string `json:"id"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if got := strings.Join(payload.BlockedTerms, ","); got != "free,premium,sale" {
		t.Fatalf("expected deduped blocked terms, got %q", got)
	}
	if !hasAuditCheckID(payload.Checks, "metadata.keywords.blocked_term") {
		t.Fatalf("expected blocked-term check, got %+v", payload.Checks)
	}
}

func TestRunMetadataKeywordsAuditSuggestsAppInfoOverrideWhenResolutionIsAmbiguous(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	installDefaultTransport(t, metadataKeywordsAuditAmbiguousAppInfoTransport(t))

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"metadata", "keywords", "audit",
			"--app", "app-1",
			"--version-id", "ver-1",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if !strings.Contains(stderr, `multiple app infos found for app "app-1"`) {
		t.Fatalf("expected ambiguous app-info stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "--app-info") {
		t.Fatalf("expected app-info guidance in stderr, got %q", stderr)
	}
}

func TestMetadataKeywordsAuditUsesAppInfoOverride(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	installDefaultTransport(t, metadataKeywordsAuditAmbiguousAppInfoTransport(t))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "audit",
			"--app", "app-1",
			"--version-id", "ver-1",
			"--app-info", "info-override",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Checks []struct {
			ID string `json:"id"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !hasAuditCheckID(payload.Checks, "metadata.keywords.overlap_name") {
		t.Fatalf("expected overlap_name check when --app-info override is used, got %+v", payload.Checks)
	}
}

func metadataKeywordsAuditTransport(t *testing.T, keywords string) http.RoundTripper {
	t.Helper()

	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/ver-1":
			return jsonResponse(http.StatusOK, `{
				"data":{
					"type":"appStoreVersions",
					"id":"ver-1",
					"attributes":{"platform":"IOS","versionString":"1.2.3","appVersionState":"PREPARE_FOR_SUBMISSION"},
					"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, `{
				"data":[
					{"type":"appInfos","id":"info-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}
				]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appInfos/info-1/appInfoLocalizations":
			return jsonResponse(http.StatusOK, `{
				"data":[
					{
						"type":"appInfoLocalizations",
						"id":"info-loc-en",
						"attributes":{"locale":"en-US","name":"Habit Tracker","subtitle":"Daily progress"}
					}
				],
				"links":{}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, `{
				"data":[
					{
						"type":"appStoreVersionLocalizations",
						"id":"ver-loc-en",
						"attributes":{"locale":"en-US","keywords":"`+keywords+`"}
					}
				],
				"links":{}
			}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})
}

func metadataKeywordsAuditAmbiguousAppInfoTransport(t *testing.T) http.RoundTripper {
	t.Helper()

	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/ver-1":
			return jsonResponse(http.StatusOK, `{
				"data":{
					"type":"appStoreVersions",
					"id":"ver-1",
					"attributes":{"platform":"IOS","versionString":"1.2.3","appVersionState":"PREPARE_FOR_SUBMISSION"},
					"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, `{
				"data":[
					{"type":"appInfos","id":"info-a","attributes":{"state":"PREPARE_FOR_SUBMISSION"}},
					{"type":"appInfos","id":"info-override","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}
				]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appInfos/info-override/appInfoLocalizations":
			return jsonResponse(http.StatusOK, `{
				"data":[
					{
						"type":"appInfoLocalizations",
						"id":"info-loc-en",
						"attributes":{"locale":"en-US","name":"Habit Tracker","subtitle":"Daily progress"}
					}
				],
				"links":{}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, `{
				"data":[
					{
						"type":"appStoreVersionLocalizations",
						"id":"ver-loc-en",
						"attributes":{"locale":"en-US","keywords":"habit tracker,focus"}
					}
				],
				"links":{}
			}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})
}

func hasAuditCheckID(checks []struct {
	ID string `json:"id"`
}, id string,
) bool {
	for _, check := range checks {
		if check.ID == id {
			return true
		}
	}
	return false
}
