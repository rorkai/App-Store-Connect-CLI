package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func ageRatingAuditTransport() http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps":
			return jsonHTTPResponse(http.StatusOK, `{"data":[
				{"type":"apps","id":"app-ready","attributes":{"name":"Ready App","bundleId":"com.example.ready"}},
				{"type":"apps","id":"app-social","attributes":{"name":"Social App","bundleId":"com.example.social"}},
				{"type":"apps","id":"app-unset","attributes":{"name":"Unset App","bundleId":"com.example.unset"}}
			],"links":{}}`), nil
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/apps/") && strings.HasSuffix(req.URL.Path, "/appInfos"):
			appID := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1/apps/"), "/appInfos")
			return jsonHTTPResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appInfos","id":"info-%s"}],"links":{}}`, appID)), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appInfos/info-app-ready/ageRatingDeclaration":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-1","attributes":{"socialMedia":false,"socialMediaAgeRestricted":false,"messagingAndChat":false,"ageAssurance":false}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appInfos/info-app-social/ageRatingDeclaration":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-2","attributes":{"socialMedia":true,"messagingAndChat":true,"ageAssurance":true,"userGeneratedContent":true}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appInfos/info-app-unset/ageRatingDeclaration":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-3","attributes":{}}}`), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})
}

func TestAgeRatingAuditReportsMissingSocialMediaResponses(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = ageRatingAuditTransport()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"age-rating", "audit", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result struct {
		Apps []struct {
			AppID                string   `json:"appId"`
			SocialMedia          string   `json:"socialMedia"`
			UserGeneratedContent string   `json:"userGeneratedContent"`
			MissingResponses     []string `json:"missingResponses"`
			Ready                bool     `json:"ready"`
		} `json:"apps"`
		ReadyCount   int `json:"readyCount"`
		MissingCount int `json:"missingCount"`
		ErrorCount   int `json:"errorCount"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if result.ReadyCount != 1 || result.MissingCount != 2 || result.ErrorCount != 0 {
		t.Fatalf("counts = ready %d missing %d error %d, want 1/2/0", result.ReadyCount, result.MissingCount, result.ErrorCount)
	}
	rows := map[string][]string{}
	userGeneratedContent := map[string]string{}
	for _, row := range result.Apps {
		rows[row.AppID] = row.MissingResponses
		userGeneratedContent[row.AppID] = row.UserGeneratedContent
	}
	if got := userGeneratedContent["app-social"]; got != "true" {
		t.Fatalf("app-social user-generated content = %q, want true", got)
	}
	if got := userGeneratedContent["app-unset"]; got != "UNSET" {
		t.Fatalf("app-unset user-generated content = %q, want UNSET", got)
	}
	if len(rows["app-ready"]) != 0 {
		t.Fatalf("app-ready missing = %v, want none", rows["app-ready"])
	}
	if got := strings.Join(rows["app-social"], ","); got != "socialMediaAgeRestricted" {
		t.Fatalf("app-social missing = %q, want socialMediaAgeRestricted", got)
	}
	if got := strings.Join(rows["app-unset"], ","); got != "socialMedia,messagingAndChat" {
		t.Fatalf("app-unset missing = %q, want socialMedia,messagingAndChat", got)
	}
	if !strings.Contains(stderr, "September 2026") {
		t.Fatalf("stderr missing deadline notice, got %q", stderr)
	}
}

func TestAgeRatingAuditAppFilterRestrictsSweep(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = ageRatingAuditTransport()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"age-rating", "audit", "--app", "app-social", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result struct {
		Apps []struct {
			AppID string `json:"appId"`
			Name  string `json:"name"`
		} `json:"apps"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Apps) != 1 || result.Apps[0].AppID != "app-social" || result.Apps[0].Name != "Social App" {
		t.Fatalf("unexpected filtered rows: %+v", result.Apps)
	}
}

func TestAgeRatingAuditSelectsCurrentAppInfo(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"apps","id":"app-1","attributes":{"name":"Current App"}}],"links":{}}`), nil
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-old","attributes":{"state":"REPLACED_WITH_NEW_INFO"}},{"type":"appInfos","id":"info-current","attributes":{"state":"READY_FOR_DISTRIBUTION"}}],"links":{}}`), nil
		case "/v1/appInfos/info-current/ageRatingDeclaration":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-current","attributes":{"socialMedia":false,"socialMediaAgeRestricted":false,"messagingAndChat":false,"ageAssurance":false}}}`), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"age-rating", "audit", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result struct {
		Apps []struct {
			Ready bool `json:"ready"`
		} `json:"apps"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Apps) != 1 || !result.Apps[0].Ready {
		t.Fatalf("unexpected audit result: %+v", result.Apps)
	}
}

func TestAgeRatingAuditChecksEveryCurrentAppInfo(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"apps","id":"app-1","attributes":{"name":"Multi-platform App"}}],"links":{}}`), nil
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-ios","attributes":{"state":"READY_FOR_DISTRIBUTION"}},{"type":"appInfos","id":"info-mac","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}],"links":{}}`), nil
		case "/v1/appInfos/info-ios/ageRatingDeclaration":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-ios","attributes":{"socialMedia":false,"socialMediaAgeRestricted":false,"messagingAndChat":false,"ageAssurance":false}}}`), nil
		case "/v1/appInfos/info-mac/ageRatingDeclaration":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-mac","attributes":{"socialMedia":true,"socialMediaAgeRestricted":false,"messagingAndChat":true,"ageAssurance":false,"userGeneratedContent":true}}}`), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"age-rating", "audit", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result struct {
		Apps []struct {
			AppInfoID    string `json:"appInfoId"`
			AppInfoState string `json:"appInfoState"`
			Ready        bool   `json:"ready"`
		} `json:"apps"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Apps) != 2 {
		t.Fatalf("audit rows = %+v, want one row per current app info", result.Apps)
	}
	want := []struct {
		id    string
		state string
	}{{"info-ios", "READY_FOR_DISTRIBUTION"}, {"info-mac", "PREPARE_FOR_SUBMISSION"}}
	for i, row := range result.Apps {
		if row.AppInfoID != want[i].id || row.AppInfoState != want[i].state || !row.Ready {
			t.Fatalf("audit row %d = %+v, want app info %q in state %q and ready", i, row, want[i].id, want[i].state)
		}
	}
}

func TestAgeRatingAuditReportsMissingReadinessResponses(t *testing.T) {
	tests := []struct {
		name        string
		attributes  string
		wantMissing string
	}{
		{
			name:        "requires age assurance",
			attributes:  `{"socialMedia":true,"socialMediaAgeRestricted":true,"messagingAndChat":true,"ageAssurance":false,"userGeneratedContent":true}`,
			wantMissing: "ageAssurance",
		},
		{
			name:        "rejects restriction without social media",
			attributes:  `{"socialMedia":false,"socialMediaAgeRestricted":true,"messagingAndChat":true,"ageAssurance":true,"userGeneratedContent":true}`,
			wantMissing: "socialMedia",
		},
		{
			name:        "social media requires user-generated content",
			attributes:  `{"socialMedia":true,"socialMediaAgeRestricted":false,"messagingAndChat":true,"ageAssurance":false,"userGeneratedContent":false}`,
			wantMissing: "userGeneratedContent",
		},
		{
			name:        "age-restricted social media requires user-generated content",
			attributes:  `{"socialMedia":false,"socialMediaAgeRestricted":true,"messagingAndChat":true,"ageAssurance":true,"userGeneratedContent":false}`,
			wantMissing: "socialMedia,userGeneratedContent",
		},
		{
			name:        "social media with user-generated content unset",
			attributes:  `{"socialMedia":true,"socialMediaAgeRestricted":false,"messagingAndChat":true,"ageAssurance":false}`,
			wantMissing: "userGeneratedContent",
		},
		{
			name:        "age-restricted social media with user-generated content unset",
			attributes:  `{"socialMedia":false,"socialMediaAgeRestricted":true,"messagingAndChat":true,"ageAssurance":true}`,
			wantMissing: "socialMedia,userGeneratedContent",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/apps":
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"apps","id":"app-1"}],"links":{}}`), nil
				case "/v1/apps/app-1/appInfos":
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-1","attributes":{"state":"READY_FOR_DISTRIBUTION"}}],"links":{}}`), nil
				case "/v1/appInfos/info-1/ageRatingDeclaration":
					return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-1","attributes":`+test.attributes+`}}`), nil
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
				}
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, _ := captureOutput(t, func() {
				if err := root.Parse([]string{"age-rating", "audit", "--output", "json"}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			var result struct {
				Apps []struct {
					MissingResponses []string `json:"missingResponses"`
					Ready            bool     `json:"ready"`
				} `json:"apps"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("unmarshal output: %v (%q)", err, stdout)
			}
			if len(result.Apps) != 1 || result.Apps[0].Ready || strings.Join(result.Apps[0].MissingResponses, ",") != test.wantMissing {
				t.Fatalf("unexpected readiness result: %+v", result.Apps)
			}
		})
	}
}

func TestAgeRatingAuditReturnsFailureAfterRenderingRowErrors(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"apps","id":"app-1"}],"links":{}}`), nil
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusForbidden, `{"errors":[{"status":"403","title":"Forbidden"}]}`), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"age-rating", "audit", "--output", "json"}, "1.2.3"); code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
	var result struct {
		Apps []struct {
			Error string `json:"error"`
		} `json:"apps"`
		ErrorCount int `json:"errorCount"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Apps) != 1 || result.Apps[0].Error == "" || result.ErrorCount != 1 {
		t.Fatalf("unexpected error result: %+v", result)
	}
	if !strings.Contains(stderr, "age-rating audit: 1 app info record could not be audited") {
		t.Fatalf("stderr = %q, want partial-audit diagnostic", stderr)
	}
}

func TestAgeRatingAuditRejectsExplicitEmptyApp(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"age-rating", "audit", "--app", " , \t"}, "1.2.3"); code != cmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--app must include at least one app ID") {
		t.Fatalf("stderr = %q, want explicit empty --app error", stderr)
	}
}

func TestAgeRatingAuditPaginationIsExplicit(t *testing.T) {
	for _, tt := range []struct {
		name           string
		args           []string
		wantApps       int
		wantSecondPage bool
	}{
		{name: "first page by default", args: []string{"age-rating", "audit", "--output", "json"}, wantApps: 1},
		{name: "all pages when requested", args: []string{"age-rating", "audit", "--paginate", "--output", "json"}, wantApps: 2, wantSecondPage: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			secondPage := false
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.URL.Path == "/v1/apps" && req.URL.Query().Get("cursor") == "next":
					secondPage = true
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"apps","id":"app-2"}],"links":{}}`), nil
				case req.URL.Path == "/v1/apps":
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"apps","id":"app-1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps?cursor=next"}}`), nil
				case strings.HasPrefix(req.URL.Path, "/v1/apps/") && strings.HasSuffix(req.URL.Path, "/appInfos"):
					appID := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1/apps/"), "/appInfos")
					return jsonHTTPResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appInfos","id":"info-%s","attributes":{"state":"READY_FOR_DISTRIBUTION"}}],"links":{}}`, appID)), nil
				case strings.HasPrefix(req.URL.Path, "/v1/appInfos/") && strings.HasSuffix(req.URL.Path, "/ageRatingDeclaration"):
					return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-1","attributes":{"socialMedia":false,"socialMediaAgeRestricted":false,"messagingAndChat":false,"ageAssurance":false}}}`), nil
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
				}
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(tt.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			var result struct {
				Apps []json.RawMessage `json:"apps"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("unmarshal output: %v (%q)", err, stdout)
			}
			if len(result.Apps) != tt.wantApps {
				t.Fatalf("app count = %d, want %d", len(result.Apps), tt.wantApps)
			}
			if secondPage != tt.wantSecondPage {
				t.Fatalf("second page requested = %t, want %t", secondPage, tt.wantSecondPage)
			}
			if !tt.wantSecondPage && !strings.Contains(stderr, "use --paginate to audit every app") {
				t.Fatalf("stderr = %q, want pagination warning", stderr)
			}
		})
	}
}

func TestAgeRatingAuditRejectsRepeatedAppPaginationURL(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "100ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	requests := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		requests++
		if requests > 2 {
			return nil, fmt.Errorf("pagination loop escaped repeated-link guard")
		}
		if req.URL.Query().Get("cursor") == "repeat" {
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps?cursor=repeat"}}`), nil
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"apps","id":"app-1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps?cursor=repeat"}}`), nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"age-rating", "audit", "--paginate", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := root.Run(context.Background())
	if !errors.Is(err, asc.ErrRepeatedPaginationURL) {
		t.Fatalf("run error = %v, want ErrRepeatedPaginationURL", err)
	}
	if requests != 2 {
		t.Fatalf("app requests = %d, want 2 before rejecting repeated link", requests)
	}
}
