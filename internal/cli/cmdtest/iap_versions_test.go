package cmdtest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	iapcli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/iap"
)

func TestIAPVersionsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	next := "https://api.appstoreconnect.apple.com/v2/inAppPurchases/iap-1/versions?cursor=next"
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"create requires iap", []string{"iap", "versions", "create"}, "--iap-id is required"},
		{"list requires iap", []string{"iap", "versions", "list"}, "--iap-id is required"},
		{"list validates state", []string{"iap", "versions", "list", "--iap-id", "iap-1", "--state", "NOPE"}, "--state must be one of"},
		{"list validates version fields before auth", []string{"iap", "versions", "list", "--iap-id", "iap-1", "--version-fields", "nope"}, "--version-fields must be one of"},
		{"view validates iap fields before auth", []string{"iap", "versions", "view", "--version-id", "version-1", "--iap-fields", "nope"}, "--iap-fields must be one of"},
		{"primary image validates fields before auth", []string{"iap", "versions", "image", "--version-id", "version-1", "--image-fields", "nope"}, "--image-fields must be one of"},
		{"image list validates fields before auth", []string{"iap", "versions", "images", "list", "--version-id", "version-1", "--image-fields", "nope"}, "--image-fields must be one of"},
		{"image detail validates fields before auth", []string{"iap", "versions", "images", "view", "--image-id", "image-1", "--image-fields", "nope"}, "--image-fields must be one of"},
		{"localization list validates fields before auth", []string{"iap", "versions", "localizations", "list", "--version-id", "version-1", "--localization-fields", "nope"}, "--localization-fields must be one of"},
		{"localization detail validates version fields before auth", []string{"iap", "versions", "localizations", "view", "--localization-id", "loc-1", "--version-fields", "nope"}, "--version-fields must be one of"},
		{"iap list validates propagated version fields before auth", []string{"iap", "list", "--app", "app-1", "--version-fields", "nope"}, "--version-fields must be one of"},
		{"iap view validates propagated version fields before auth", []string{"iap", "view", "--id", "iap-1", "--version-fields", "nope"}, "--version-fields must be one of"},
		{"iap list validates parent fields before auth", []string{"iap", "list", "--app", "app-1", "--fields", "nope"}, "--fields must be one of"},
		{"iap view validates parent fields before auth", []string{"iap", "view", "--id", "iap-1", "--fields", "nope"}, "--fields must be one of"},
		{"iap list rejects parent fields on legacy endpoint", []string{"iap", "list", "--app", "app-1", "--legacy", "--fields", "name"}, "--fields requires the v2 endpoint"},
		{"version localization update rejects name and clear-name", []string{"iap", "versions", "localizations", "update", "--localization-id", "loc-1", "--name", "Name", "--clear-name"}, "--name and --clear-name are mutually exclusive"},
		{"version localization update rejects description and clear-description", []string{"iap", "versions", "localizations", "update", "--localization-id", "loc-1", "--description", "Description", "--clear-description"}, "--description and --clear-description are mutually exclusive"},
		{"legacy localization update rejects name and clear-name", []string{"iap", "localizations", "update", "--localization-id", "loc-1", "--name", "Name", "--clear-name"}, "--name and --clear-name are mutually exclusive"},
		{"legacy localization update rejects description and clear-description", []string{"iap", "localizations", "update", "--localization-id", "loc-1", "--description", "Description", "--clear-description"}, "--description and --clear-description are mutually exclusive"},
		{"version list rejects next with selectors", []string{"iap", "versions", "list", "--next", next, "--state", "READY_FOR_REVIEW"}, "--next cannot be combined"},
		{"image list rejects next with fields", []string{"iap", "versions", "images", "list", "--next", next, "--image-fields", "fileName"}, "--next cannot be combined"},
		{"localization list rejects next with include", []string{"iap", "versions", "localizations", "list", "--next", next, "--include", "version"}, "--next cannot be combined"},
		{"iap list rejects next with version selectors", []string{"iap", "list", "--next", next, "--include-versions"}, "--next cannot be combined"},
		{"version create rejects positional args", []string{"iap", "versions", "create", "--iap-id", "iap-1", "junk"}, "unexpected argument(s): junk"},
		{"version image rejects positional args", []string{"iap", "versions", "image", "--version-id", "version-1", "junk"}, "unexpected argument(s): junk"},
		{"version image detail rejects positional args", []string{"iap", "versions", "images", "view", "--image-id", "image-1", "junk"}, "unexpected argument(s): junk"},
		{"version localization detail rejects positional args", []string{"iap", "versions", "localizations", "view", "--localization-id", "loc-1", "junk"}, "unexpected argument(s): junk"},
		{"version submit rejects positional args", []string{"iap", "versions", "submit", "--version-id", "version-1", "--submission", "submission-1", "--confirm", "junk"}, "unexpected argument(s): junk"},
		{"version links reject positional args", []string{"iap", "versions", "links", "image", "--version-id", "version-1", "junk"}, "unexpected argument(s): junk"},
		{"view requires version", []string{"iap", "versions", "view"}, "--version-id is required"},
		{"image requires version", []string{"iap", "versions", "image"}, "--version-id is required"},
		{"localization create requires version", []string{"iap", "versions", "localizations", "create", "--name", "Name", "--locale", "en-US"}, "--version-id is required"},
		{"localization view requires id", []string{"iap", "versions", "localizations", "view"}, "--localization-id is required"},
		{"localization delete requires confirm", []string{"iap", "versions", "localizations", "delete", "--localization-id", "loc-1"}, "--confirm is required"},
		{"image create requires version", []string{"iap", "versions", "images", "create", "--file", "image.png"}, "--version-id is required"},
		{"image delete requires confirm", []string{"iap", "versions", "images", "delete", "--image-id", "img-1"}, "--confirm is required"},
		{"submit requires submission", []string{"iap", "versions", "submit", "--version-id", "version-1", "--confirm"}, "--submission is required"},
		{"submit requires confirm", []string{"iap", "versions", "submit", "--version-id", "version-1", "--submission", "submission-1"}, "--confirm is required"},
		{"links version requires iap", []string{"iap", "versions", "links", "versions"}, "--iap-id is required"},
		{"links image requires version", []string{"iap", "versions", "links", "image"}, "--version-id is required"},
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
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestIAPNextConflictsFailBeforeClientFactory(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v2/inAppPurchases/iap-next/versions?cursor=next"
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "iap list rejects owner with next",
			args:    []string{"iap", "list", "--app", "app-1", "--next", next},
			wantErr: "--next cannot be combined with --app",
		},
		{
			name:    "iap list rejects explicit empty owner with next",
			args:    []string{"iap", "list", "--app", "", "--next", next},
			wantErr: "--next cannot be combined with --app",
		},
		{
			name:    "iap list rejects limit with next",
			args:    []string{"iap", "list", "--limit", "5", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "iap list rejects explicit zero limit with next",
			args:    []string{"iap", "list", "--limit", "0", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "versions list rejects owner with next",
			args:    []string{"iap", "versions", "list", "--iap-id", "iap-1", "--next", next},
			wantErr: "--next cannot be combined with --iap-id",
		},
		{
			name:    "versions list rejects explicit empty owner with next",
			args:    []string{"iap", "versions", "list", "--iap-id", "", "--next", next},
			wantErr: "--next cannot be combined with --iap-id",
		},
		{
			name:    "versions list rejects limit with next",
			args:    []string{"iap", "versions", "list", "--limit", "5", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "versions list rejects explicit zero limit with next",
			args:    []string{"iap", "versions", "list", "--limit", "0", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "versions list rejects explicit zero images limit with next",
			args:    []string{"iap", "versions", "list", "--images-limit", "0", "--next", next},
			wantErr: "--next cannot be combined with --images-limit",
		},
		{
			name:    "versions list rejects explicit zero localizations limit with next",
			args:    []string{"iap", "versions", "list", "--localizations-limit", "0", "--next", next},
			wantErr: "--next cannot be combined with --localizations-limit",
		},
		{
			name:    "versions list rejects explicit empty state with next",
			args:    []string{"iap", "versions", "list", "--state", "", "--next", next},
			wantErr: "--next cannot be combined with --state",
		},
		{
			name:    "versions list rejects explicit empty include with next",
			args:    []string{"iap", "versions", "list", "--include", "", "--next", next},
			wantErr: "--next cannot be combined with --include",
		},
		{
			name:    "versions list rejects explicit empty sparse fields with next",
			args:    []string{"iap", "versions", "list", "--version-fields", "", "--next", next},
			wantErr: "--next cannot be combined with --version-fields",
		},
		{
			name:    "versions list rejects explicit empty iap fields with next",
			args:    []string{"iap", "versions", "list", "--iap-fields", "", "--next", next},
			wantErr: "--next cannot be combined with --iap-fields",
		},
		{
			name:    "versions list rejects explicit empty image fields with next",
			args:    []string{"iap", "versions", "list", "--image-fields", "", "--next", next},
			wantErr: "--next cannot be combined with --image-fields",
		},
		{
			name:    "versions list rejects explicit empty localization fields with next",
			args:    []string{"iap", "versions", "list", "--localization-fields", "", "--next", next},
			wantErr: "--next cannot be combined with --localization-fields",
		},
		{
			name:    "images list rejects owner with next",
			args:    []string{"iap", "versions", "images", "list", "--version-id", "version-1", "--next", next},
			wantErr: "--next cannot be combined with --version-id",
		},
		{
			name:    "images list rejects explicit empty owner with next",
			args:    []string{"iap", "versions", "images", "list", "--version-id", "", "--next", next},
			wantErr: "--next cannot be combined with --version-id",
		},
		{
			name:    "images list rejects limit with next",
			args:    []string{"iap", "versions", "images", "list", "--limit", "5", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "images list rejects explicit zero limit with next",
			args:    []string{"iap", "versions", "images", "list", "--limit", "0", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "images list rejects explicit empty fields with next",
			args:    []string{"iap", "versions", "images", "list", "--image-fields", "", "--next", next},
			wantErr: "--next cannot be combined with --image-fields",
		},
		{
			name:    "localizations list rejects owner with next",
			args:    []string{"iap", "versions", "localizations", "list", "--version-id", "version-1", "--next", next},
			wantErr: "--next cannot be combined with --version-id",
		},
		{
			name:    "localizations list rejects explicit empty owner with next",
			args:    []string{"iap", "versions", "localizations", "list", "--version-id", "", "--next", next},
			wantErr: "--next cannot be combined with --version-id",
		},
		{
			name:    "localizations list rejects limit with next",
			args:    []string{"iap", "versions", "localizations", "list", "--limit", "5", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "localizations list rejects explicit zero limit with next",
			args:    []string{"iap", "versions", "localizations", "list", "--limit", "0", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "localizations list rejects explicit empty include with next",
			args:    []string{"iap", "versions", "localizations", "list", "--include", "", "--next", next},
			wantErr: "--next cannot be combined with --include",
		},
		{
			name:    "localizations list rejects explicit empty localization fields with next",
			args:    []string{"iap", "versions", "localizations", "list", "--localization-fields", "", "--next", next},
			wantErr: "--next cannot be combined with --localization-fields",
		},
		{
			name:    "localizations list rejects explicit empty version fields with next",
			args:    []string{"iap", "versions", "localizations", "list", "--version-fields", "", "--next", next},
			wantErr: "--next cannot be combined with --version-fields",
		},
		{
			name:    "version linkages reject owner with next",
			args:    []string{"iap", "versions", "links", "versions", "--iap-id", "iap-1", "--next", next},
			wantErr: "--next cannot be combined with --iap-id",
		},
		{
			name:    "version linkages reject explicit empty owner with next",
			args:    []string{"iap", "versions", "links", "versions", "--iap-id", "", "--next", next},
			wantErr: "--next cannot be combined with --iap-id",
		},
		{
			name:    "image linkages reject owner with next",
			args:    []string{"iap", "versions", "links", "images", "--version-id", "version-1", "--next", next},
			wantErr: "--next cannot be combined with --version-id",
		},
		{
			name:    "image linkages reject explicit empty owner with next",
			args:    []string{"iap", "versions", "links", "images", "--version-id", "", "--next", next},
			wantErr: "--next cannot be combined with --version-id",
		},
		{
			name:    "localization linkages reject owner with next",
			args:    []string{"iap", "versions", "links", "localizations", "--version-id", "version-1", "--next", next},
			wantErr: "--next cannot be combined with --version-id",
		},
		{
			name:    "localization linkages reject explicit empty owner with next",
			args:    []string{"iap", "versions", "links", "localizations", "--version-id", "", "--next", next},
			wantErr: "--next cannot be combined with --version-id",
		},
		{
			name:    "version linkages reject limit with next",
			args:    []string{"iap", "versions", "links", "versions", "--limit", "5", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "image linkages reject limit with next",
			args:    []string{"iap", "versions", "links", "images", "--limit", "5", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "localization linkages reject limit with next",
			args:    []string{"iap", "versions", "links", "localizations", "--limit", "5", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "version linkages reject explicit zero limit with next",
			args:    []string{"iap", "versions", "links", "versions", "--limit", "0", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "image linkages reject explicit zero limit with next",
			args:    []string{"iap", "versions", "links", "images", "--limit", "0", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "localization linkages reject explicit zero limit with next",
			args:    []string{"iap", "versions", "links", "localizations", "--limit", "0", "--next", next},
			wantErr: "--next cannot be combined with --limit",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			restore := iapcli.SetVersionClientFactory(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})
			t.Cleanup(restore)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})
			if factoryCalled {
				t.Fatal("client factory called before validation")
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestIAPVersionLinkagesPaginateAllPages(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name         string
		args         []string
		firstPath    string
		resourceType string
	}{
		{
			name:         "versions",
			args:         []string{"iap", "versions", "links", "versions", "--iap-id", "iap-1", "--limit", "2", "--paginate", "--output", "json"},
			firstPath:    "/v2/inAppPurchases/iap-1/relationships/versions",
			resourceType: "inAppPurchaseVersions",
		},
		{
			name:         "images",
			args:         []string{"iap", "versions", "links", "images", "--version-id", "version-1", "--limit", "2", "--paginate", "--output", "json"},
			firstPath:    "/v1/inAppPurchaseVersions/version-1/relationships/images",
			resourceType: "inAppPurchaseImages",
		},
		{
			name:         "localizations",
			args:         []string{"iap", "versions", "links", "localizations", "--version-id", "version-1", "--limit", "2", "--paginate", "--output", "json"},
			firstPath:    "/v1/inAppPurchaseVersions/version-1/relationships/localizations",
			resourceType: "inAppPurchaseLocalizations",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requestCount++
				w.Header().Set("Content-Type", "application/json")
				switch requestCount {
				case 1:
					if req.Method != http.MethodGet || req.URL.Path != test.firstPath {
						t.Fatalf("first request = %s %s, want GET %s", req.Method, req.URL.Path, test.firstPath)
					}
					if got := req.URL.Query().Get("limit"); got != "2" {
						t.Fatalf("first request limit = %q, want 2", got)
					}
					nextURL := "https://api.appstoreconnect.apple.com" + test.firstPath + "?cursor=next"
					_, _ = fmt.Fprintf(w, `{"data":[{"type":%q,"id":"%s-1"}],"links":{"next":%q}}`, test.resourceType, test.name, nextURL)
				case 2:
					if req.Method != http.MethodGet || req.URL.Path != test.firstPath || req.URL.Query().Get("cursor") != "next" {
						t.Fatalf("second request = %s %s?%s, want opaque next URL", req.Method, req.URL.Path, req.URL.RawQuery)
					}
					_, _ = fmt.Fprintf(w, `{"data":[{"type":%q,"id":"%s-2"}],"links":{"next":""}}`, test.resourceType, test.name)
				default:
					t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL)
				}
			}))
			t.Cleanup(server.Close)
			setIAPVersionTestServerClient(t, server)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("unexpected stderr: %s", stderr)
			}
			var response asc.LinkagesResponse
			if err := json.Unmarshal([]byte(stdout), &response); err != nil {
				t.Fatalf("invalid JSON output %q: %v", stdout, err)
			}
			if requestCount != 2 {
				t.Fatalf("request count = %d, want 2", requestCount)
			}
			if len(response.Data) != 2 || response.Data[0].ID != test.name+"-1" || response.Data[1].ID != test.name+"-2" {
				t.Fatalf("unexpected aggregated data: %#v", response.Data)
			}
		})
	}
}

func TestIAPVersionPrincipalCLIFlowsUseExactHTTPContracts(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name       string
		args       []string
		method     string
		path       string
		query      map[string]string
		wantBody   string
		statusCode int
		response   string
	}{
		{
			name: "create version", args: []string{"iap", "versions", "create", "--iap-id", "iap-1", "--output", "json"},
			method: http.MethodPost, path: "/v1/inAppPurchaseVersions", statusCode: http.StatusCreated,
			wantBody: `{"data":{"type":"inAppPurchaseVersions","relationships":{"inAppPurchase":{"data":{"type":"inAppPurchases","id":"iap-1"}}}}}`,
			response: `{"data":{"type":"inAppPurchaseVersions","id":"version-1","attributes":{"version":1}}}`,
		},
		{
			name: "list versions with sparse fields", args: []string{"iap", "versions", "list", "--iap-id", "iap-1", "--state", "READY_FOR_REVIEW", "--include", "images", "--version-fields", "version,state", "--iap-fields", "name", "--image-fields", "fileName", "--localization-fields", "locale", "--limit", "3", "--images-limit", "2", "--localizations-limit", "4", "--output", "json"},
			method: http.MethodGet, path: "/v2/inAppPurchases/iap-1/versions",
			query:    map[string]string{"filter[state]": "READY_FOR_REVIEW", "fields[inAppPurchaseVersions]": "version,state", "fields[inAppPurchases]": "name", "fields[inAppPurchaseImages]": "fileName", "fields[inAppPurchaseLocalizations]": "locale", "include": "images", "limit": "3", "limit[images]": "2", "limit[localizations]": "4"},
			response: `{"data":[{"type":"inAppPurchaseVersions","id":"version-1","attributes":{"version":1}}]}`,
		},
		{
			name: "view version with sparse fields", args: []string{"iap", "versions", "view", "--version-id", "version-1", "--include", "localizations", "--version-fields", "version,state", "--iap-fields", "name", "--image-fields", "fileName", "--localization-fields", "name,locale", "--localizations-limit", "4", "--output", "json"},
			method: http.MethodGet, path: "/v1/inAppPurchaseVersions/version-1",
			query:    map[string]string{"fields[inAppPurchaseVersions]": "version,state", "fields[inAppPurchases]": "name", "fields[inAppPurchaseImages]": "fileName", "fields[inAppPurchaseLocalizations]": "name,locale", "include": "localizations", "limit[localizations]": "4"},
			response: `{"data":{"type":"inAppPurchaseVersions","id":"version-1","attributes":{"version":1}}}`,
		},
		{
			name: "existing IAP list propagates version fields", args: []string{"iap", "list", "--app", "app-1", "--include-versions", "--version-fields", "version,state", "--versions-limit", "5", "--output", "json"},
			method: http.MethodGet, path: "/v1/apps/app-1/inAppPurchasesV2",
			query:    map[string]string{"fields[inAppPurchaseVersions]": "version,state", "include": "versions", "limit[versions]": "5"},
			response: `{"data":[{"type":"inAppPurchases","id":"iap-1"}]}`,
		},
		{
			name: "existing IAP list propagates parent fields", args: []string{"iap", "list", "--app", "app-1", "--fields", "name,versions", "--output", "json"},
			method: http.MethodGet, path: "/v1/apps/app-1/inAppPurchasesV2",
			query:    map[string]string{"fields[inAppPurchases]": "name,versions"},
			response: `{"data":[{"type":"inAppPurchases","id":"iap-1"}]}`,
		},
		{
			name: "existing IAP view propagates version fields", args: []string{"iap", "view", "--id", "iap-1", "--include-versions", "--version-fields", "version,state", "--versions-limit", "5", "--output", "json"},
			method: http.MethodGet, path: "/v2/inAppPurchases/iap-1",
			query:    map[string]string{"fields[inAppPurchaseVersions]": "version,state", "include": "versions", "limit[versions]": "5"},
			response: `{"data":{"type":"inAppPurchases","id":"iap-1"}}`,
		},
		{
			name: "existing IAP view propagates parent fields", args: []string{"iap", "view", "--id", "iap-1", "--fields", "name,versions", "--output", "json"},
			method: http.MethodGet, path: "/v2/inAppPurchases/iap-1",
			query:    map[string]string{"fields[inAppPurchases]": "name,versions"},
			response: `{"data":{"type":"inAppPurchases","id":"iap-1"}}`,
		},
		{
			name: "view primary image fields", args: []string{"iap", "versions", "image", "--version-id", "version-1", "--image-fields", "fileName", "--output", "json"},
			method: http.MethodGet, path: "/v1/inAppPurchaseVersions/version-1/image",
			query:    map[string]string{"fields[inAppPurchaseImages]": "fileName"},
			response: `{"data":{"type":"inAppPurchaseImages","id":"image-1"}}`,
		},
		{
			name: "list version images fields", args: []string{"iap", "versions", "images", "list", "--version-id", "version-1", "--image-fields", "fileName,assetDeliveryState", "--limit", "7", "--output", "json"},
			method: http.MethodGet, path: "/v1/inAppPurchaseVersions/version-1/images",
			query:    map[string]string{"fields[inAppPurchaseImages]": "fileName,assetDeliveryState", "limit": "7"},
			response: `{"data":[{"type":"inAppPurchaseImages","id":"image-1"}]}`,
		},
		{
			name: "view v2 image fields", args: []string{"iap", "versions", "images", "view", "--image-id", "image-1", "--image-fields", "fileName,assetDeliveryState", "--output", "json"},
			method: http.MethodGet, path: "/v2/inAppPurchaseImages/image-1",
			query:    map[string]string{"fields[inAppPurchaseImages]": "fileName,assetDeliveryState"},
			response: `{"data":{"type":"inAppPurchaseImages","id":"image-1"}}`,
		},
		{
			name: "create localization", args: []string{"iap", "versions", "localizations", "create", "--version-id", "version-1", "--name", "Name", "--locale", "en-US", "--description", "Description", "--output", "json"},
			method: http.MethodPost, path: "/v2/inAppPurchaseLocalizations", statusCode: http.StatusCreated,
			wantBody: `{"data":{"type":"inAppPurchaseLocalizations","attributes":{"name":"Name","locale":"en-US","description":"Description"},"relationships":{"version":{"data":{"type":"inAppPurchaseVersions","id":"version-1"}}}}}`,
			response: `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1"}}`,
		},
		{
			name: "list localizations fields", args: []string{"iap", "versions", "localizations", "list", "--version-id", "version-1", "--include", "version", "--localization-fields", "name,locale", "--version-fields", "version,state", "--limit", "9", "--output", "json"},
			method: http.MethodGet, path: "/v1/inAppPurchaseVersions/version-1/localizations",
			query:    map[string]string{"fields[inAppPurchaseLocalizations]": "name,locale", "fields[inAppPurchaseVersions]": "version,state", "include": "version", "limit": "9"},
			response: `{"data":[{"type":"inAppPurchaseLocalizations","id":"loc-1"}]}`,
		},
		{
			name: "view localization fields", args: []string{"iap", "versions", "localizations", "view", "--localization-id", "loc-1", "--include", "version", "--localization-fields", "name,description", "--version-fields", "version,state", "--output", "json"},
			method: http.MethodGet, path: "/v2/inAppPurchaseLocalizations/loc-1",
			query:    map[string]string{"fields[inAppPurchaseLocalizations]": "name,description", "fields[inAppPurchaseVersions]": "version,state", "include": "version"},
			response: `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1"}}`,
		},
		{
			name: "update localization trims values", args: []string{"iap", "versions", "localizations", "update", "--localization-id", "loc-1", "--name", " Updated ", "--description", " Description ", "--output", "json"},
			method: http.MethodPatch, path: "/v2/inAppPurchaseLocalizations/loc-1",
			wantBody: `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1","attributes":{"name":"Updated","description":"Description"}}}`,
			response: `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1"}}`,
		},
		{
			name: "update localization clears nullable values", args: []string{"iap", "versions", "localizations", "update", "--localization-id", "loc-1", "--clear-name", "--clear-description", "--output", "json"},
			method: http.MethodPatch, path: "/v2/inAppPurchaseLocalizations/loc-1",
			wantBody: `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1","attributes":{"name":null,"description":null}}}`,
			response: `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1"}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requestCount++
				if req.Method != test.method || req.URL.Path != test.path {
					t.Fatalf("request = %s %s, want %s %s", req.Method, req.URL.Path, test.method, test.path)
				}
				wantQuery := url.Values{}
				for key, value := range test.query {
					wantQuery.Set(key, value)
				}
				if got := req.URL.Query(); !reflect.DeepEqual(got, wantQuery) {
					t.Fatalf("query = %v, want %v", got, wantQuery)
				}
				if test.wantBody != "" {
					assertJSONDocument(t, req.Body, test.wantBody)
				}
				status := test.statusCode
				if status == 0 {
					status = http.StatusOK
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, test.response)
			}))
			t.Cleanup(server.Close)
			setIAPVersionTestServerClient(t, server)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatal(err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatal(err)
				}
			})
			if stderr != "" {
				t.Fatalf("unexpected stderr: %s", stderr)
			}
			var output any
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatalf("invalid JSON output %q: %v", stdout, err)
			}
			if requestCount != 1 {
				t.Fatalf("request count = %d, want 1", requestCount)
			}
		})
	}
}

func TestIAPVersionImagesCreateRunsV2UploadLifecycle(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(t.TempDir(), "review.png")
	if err := os.WriteFile(imagePath, pngBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	requestCount := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		body := ""
		status := http.StatusOK
		switch requestCount {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v2/inAppPurchaseImages" {
				t.Fatalf("unexpected reservation request: %s %s", req.Method, req.URL)
			}
			assertJSONDocument(t, req.Body, fmt.Sprintf(`{"data":{"type":"inAppPurchaseImages","attributes":{"fileSize":%d,"fileName":"review.png"},"relationships":{"version":{"data":{"type":"inAppPurchaseVersions","id":"version-1"}}}}}`, len(pngBytes)))
			status = http.StatusCreated
			body = `{"data":{"type":"inAppPurchaseImages","id":"image-1","attributes":{"fileName":"review.png","fileSize":` + stringValue(len(pngBytes)) + `,"uploadOperations":[{"method":"PUT","url":"` + server.URL + `/upload/image-1","offset":0,"length":` + stringValue(len(pngBytes)) + `}]}}}`
		case 2:
			if req.Method != http.MethodPut || req.URL.Path != "/upload/image-1" {
				t.Fatalf("unexpected upload request: %s %s", req.Method, req.URL)
			}
			uploadedBody, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(uploadedBody) != string(pngBytes) {
				t.Fatalf("uploaded bytes differ")
			}
			status = http.StatusOK
		case 3:
			if req.Method != http.MethodPatch || req.URL.Path != "/v2/inAppPurchaseImages/image-1" {
				t.Fatalf("unexpected commit request: %s %s", req.Method, req.URL)
			}
			assertJSONDocument(t, req.Body, `{"data":{"type":"inAppPurchaseImages","id":"image-1","attributes":{"uploaded":true}}}`)
			body = `{"data":{"type":"inAppPurchaseImages","id":"image-1","attributes":{"assetDeliveryState":{"state":"PROCESSING"}}}}`
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchaseImages/image-1" {
				t.Fatalf("unexpected final fetch: %s %s", req.Method, req.URL)
			}
			body = `{"data":{"type":"inAppPurchaseImages","id":"image-1","attributes":{"fileName":"review.png","fileSize":` + stringValue(len(pngBytes)) + `,"assetDeliveryState":{"state":"COMPLETE"}}}}`
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	setIAPVersionTestServerClient(t, server)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "versions", "images", "create", "--version-id", "version-1", "--file", imagePath, "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("invalid JSON output %q: %v", stdout, err)
	}
	data, _ := response["data"].(map[string]any)
	if data["id"] != "image-1" {
		t.Fatalf("unexpected output: %s", stdout)
	}
	if requestCount != 4 {
		t.Fatalf("request count = %d, want 4", requestCount)
	}
}

func TestIAPVersionImagesCreateReportsReservedIDOnPostReservationFailures(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(t.TempDir(), "review.png")
	if err := os.WriteFile(imagePath, pngBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, stage := range []string{"no operations", "upload", "commit", "final fetch"} {
		t.Run(stage, func(t *testing.T) {
			requestCount := 0
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requestCount++
				w.Header().Set("Content-Type", "application/json")
				switch requestCount {
				case 1:
					if req.Method != http.MethodPost || req.URL.Path != "/v2/inAppPurchaseImages" {
						t.Fatalf("unexpected reservation request: %s %s", req.Method, req.URL)
					}
					operations := ""
					if stage != "no operations" {
						operations = `,"uploadOperations":[{"method":"PUT","url":"` + server.URL + `/upload/image-reserved","offset":0,"length":` + stringValue(len(pngBytes)) + `}]`
					}
					w.WriteHeader(http.StatusCreated)
					_, _ = io.WriteString(w, `{"data":{"type":"inAppPurchaseImages","id":"image-reserved","attributes":{"fileName":"review.png","fileSize":`+stringValue(len(pngBytes))+operations+`}}}`)
				case 2:
					if req.Method != http.MethodPut || req.URL.Path != "/upload/image-reserved" {
						t.Fatalf("unexpected upload request: %s %s", req.Method, req.URL)
					}
					if stage == "upload" {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					w.WriteHeader(http.StatusOK)
				case 3:
					if req.Method != http.MethodPatch || req.URL.Path != "/v2/inAppPurchaseImages/image-reserved" {
						t.Fatalf("unexpected commit request: %s %s", req.Method, req.URL)
					}
					if stage == "commit" {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = io.WriteString(w, `{"errors":[{"status":"500","detail":"commit failed"}]}`)
						return
					}
					w.WriteHeader(http.StatusOK)
					_, _ = io.WriteString(w, `{"data":{"type":"inAppPurchaseImages","id":"image-reserved"}}`)
				case 4:
					if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchaseImages/image-reserved" {
						t.Fatalf("unexpected final request: %s %s", req.Method, req.URL)
					}
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = io.WriteString(w, `{"errors":[{"status":"500","detail":"fetch failed"}]}`)
				default:
					t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL)
				}
			}))
			t.Cleanup(server.Close)
			setIAPVersionTestServerClient(t, server)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			_, _ = captureOutput(t, func() {
				if err := root.Parse([]string{"iap", "versions", "images", "create", "--version-id", "version-1", "--file", imagePath, "--output", "json"}); err != nil {
					t.Fatal(err)
				}
				runErr = root.Run(context.Background())
			})
			if runErr == nil {
				t.Fatal("expected post-reservation failure")
			}
			if !strings.Contains(runErr.Error(), "image-reserved") {
				t.Fatalf("error must include reserved image ID: %v", runErr)
			}
		})
	}
}

func TestIAPVersionsSubmitUsesExplicitReviewSubmissionRelationship(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissionItems" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		assertJSONDocument(t, req.Body, `{"data":{"type":"reviewSubmissionItems","relationships":{"reviewSubmission":{"data":{"type":"reviewSubmissions","id":"submission-1"}},"inAppPurchaseVersion":{"data":{"type":"inAppPurchaseVersions","id":"version-1"}}}}}`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"type":"reviewSubmissionItems","id":"item-1","attributes":{"state":"READY_FOR_REVIEW"}}}`)
	}))
	t.Cleanup(server.Close)
	setIAPVersionTestServerClient(t, server)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "versions", "submit", "--version-id", "version-1", "--submission", "submission-1", "--confirm", "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, `"id":"item-1"`) {
		t.Fatalf("unexpected output: %s", stdout)
	}
}

func stringValue(value int) string {
	return fmt.Sprintf("%d", value)
}

func assertJSONDocument(t *testing.T, reader io.Reader, want string) {
	t.Helper()
	var gotJSON, wantJSON any
	if err := json.NewDecoder(reader).Decode(&gotJSON); err != nil {
		t.Fatalf("invalid request JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantJSON); err != nil {
		t.Fatalf("invalid expected JSON %q: %v", want, err)
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		got, _ := json.Marshal(gotJSON)
		t.Fatalf("request body = %s, want %s", got, want)
	}
}

func setIAPVersionTestServerClient(t *testing.T, server *httptest.Server) {
	t.Helper()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	restore := iapcli.SetVersionClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restore)
}
