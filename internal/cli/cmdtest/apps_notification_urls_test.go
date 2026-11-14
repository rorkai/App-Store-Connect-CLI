package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAppsUpdateNotificationURLs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want map[string]any
	}{
		{"both endpoints", []string{"--subscription-status-url", "https://example.com/production", "--sandbox-subscription-status-url", "https://example.com/sandbox"}, map[string]any{"subscriptionStatusUrl": "https://example.com/production", "subscriptionStatusUrlVersion": "V2", "subscriptionStatusUrlForSandbox": "https://example.com/sandbox", "subscriptionStatusUrlVersionForSandbox": "V2"}},
		{"production only", []string{"--subscription-status-url", "https://example.com/production"}, map[string]any{"subscriptionStatusUrl": "https://example.com/production", "subscriptionStatusUrlVersion": "V2"}},
		{"sandbox with metadata", []string{"--sandbox-subscription-status-url", "https://example.com/sandbox?source=apple", "--primary-locale", "en-GB"}, map[string]any{"subscriptionStatusUrlForSandbox": "https://example.com/sandbox?source=apple", "subscriptionStatusUrlVersionForSandbox": "V2", "primaryLocale": "en-GB"}},
		{"ordinary metadata leaves endpoints unchanged", []string{"--bundle-id", "com.example.app", "--content-rights", "DOES_NOT_USE_THIRD_PARTY_CONTENT"}, map[string]any{"bundleId": "com.example.app", "contentRightsDeclaration": "DOES_NOT_USE_THIRD_PARTY_CONTENT"}},
		{"accessibility URL", []string{"--accessibility-url", "http://example.com/accessibility"}, map[string]any{"accessibilityUrl": "http://example.com/accessibility"}},
		{"streamlined purchasing on", []string{"--streamlined-purchasing-enabled", "true"}, map[string]any{"streamlinedPurchasingEnabled": true}},
		{"streamlined purchasing explicit false", []string{"--streamlined-purchasing-enabled", "false"}, map[string]any{"streamlinedPurchasingEnabled": false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			attributes, err := json.Marshal(tt.want)
			if err != nil {
				t.Fatal(err)
			}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.PATCH("/v1/apps/app-1").JSONBody(`{"data":{"type":"apps","id":"app-1","attributes":`+string(attributes)+`}}`),
				asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{"name":"Example","subscriptionStatusUrl":"https://example.com/production","subscriptionStatusUrlVersion":"V2","subscriptionStatusUrlForSandbox":"https://example.com/sandbox","subscriptionStatusUrlVersionForSandbox":"V1","streamlinedPurchasingEnabled":false}}}`))
			root := RootCommand("test")
			args := append([]string{"apps", "update", "--id", "app-1", "--output", "json"}, tt.args...)
			var runErr error
			stdout, stderr := captureOutput(t, func() { runErr = root.ParseAndRun(context.Background(), args) })
			if runErr != nil {
				t.Fatal(runErr)
			}
			if calls := len(reg.Requests()); calls != 1 || stderr != "" {
				t.Fatalf("calls=%d stderr=%q", calls, stderr)
			}
			var output struct {
				Data struct {
					Type       string         `json:"type"`
					ID         string         `json:"id"`
					Attributes map[string]any `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatal(err)
			}
			if output.Data.Type != "apps" || output.Data.ID != "app-1" {
				t.Fatalf("lost resource envelope: %s", stdout)
			}
			for key, want := range map[string]any{"name": "Example", "subscriptionStatusUrl": "https://example.com/production", "subscriptionStatusUrlVersion": "V2", "subscriptionStatusUrlForSandbox": "https://example.com/sandbox", "subscriptionStatusUrlVersionForSandbox": "V1", "streamlinedPurchasingEnabled": false} {
				if output.Data.Attributes[key] != want {
					t.Errorf("output %s=%v, want %v", key, output.Data.Attributes[key], want)
				}
			}
		})
	}
}

func TestAppsUpdateNotificationURLRejectsInvalid(t *testing.T) {
	for _, name := range []string{"--subscription-status-url", "--sandbox-subscription-status-url"} {
		for _, value := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com/#fragment", "https://example.com/#", "https:///path", "https://:443/path", "https://example.com:invalid/path", "not-a-url", "", "   "} {
			t.Run(name+"/"+value, func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
				asctest.NewRegistry(t).Install()
				root := RootCommand("test")
				var err error
				stdout, stderr := captureOutput(t, func() {
					err = root.ParseAndRun(context.Background(), []string{"apps", "update", "--id", "app-1", name, value})
				})
				if !errors.Is(err, flag.ErrHelp) || stdout != "" || !strings.Contains(stderr, name+": must be an absolute HTTPS URL without credentials or fragments") {
					t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout, stderr)
				}
			})
		}
	}
}

func TestAppsUpdateAccessibilityURLRejectsInvalid(t *testing.T) {
	for _, value := range []string{"ftp://example.com", "https://user:pass@example.com", "https:///path", "not-a-url", "", "   "} {
		t.Run(value, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			asctest.NewRegistry(t).Install()
			root := RootCommand("test")
			var err error
			stdout, stderr := captureOutput(t, func() {
				err = root.ParseAndRun(context.Background(), []string{"apps", "update", "--id", "app-1", "--accessibility-url", value})
			})
			if !errors.Is(err, flag.ErrHelp) || stdout != "" || !strings.Contains(stderr, "--accessibility-url: must be an absolute HTTP or HTTPS URL without credentials") {
				t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout, stderr)
			}
		})
	}
}

func TestAppsViewNotificationURLFields(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	fields := "accessibilityUrl,subscriptionStatusUrl,subscriptionStatusUrlVersion,subscriptionStatusUrlForSandbox,subscriptionStatusUrlVersionForSandbox"
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1", "fields[apps]="+fields),
		asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{"accessibilityUrl":"https://example.com/accessibility","subscriptionStatusUrl":"https://example.com/production","subscriptionStatusUrlVersion":"V2","subscriptionStatusUrlForSandbox":"https://example.com/sandbox","subscriptionStatusUrlVersionForSandbox":"V2"}}}`))
	root := RootCommand("test")
	var err error
	stdout, stderr := captureOutput(t, func() {
		err = root.ParseAndRun(context.Background(), []string{"apps", "view", "--id", "app-1", "--fields", fields, "--output", "json"})
	})
	if err != nil || stderr != "" {
		t.Fatalf("err=%v stderr=%q", err, stderr)
	}
	var output struct {
		Data struct {
			Attributes map[string]any `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"accessibilityUrl":                       "https://example.com/accessibility",
		"subscriptionStatusUrl":                  "https://example.com/production",
		"subscriptionStatusUrlVersion":           "V2",
		"subscriptionStatusUrlForSandbox":        "https://example.com/sandbox",
		"subscriptionStatusUrlVersionForSandbox": "V2",
	}
	if !reflect.DeepEqual(output.Data.Attributes, want) {
		t.Fatalf("sparse readback changed returned attributes: %s", stdout)
	}
}

func TestAppsViewNotificationFieldsRejectsInvalid(t *testing.T) {
	for _, value := range []string{"", "unknownField"} {
		t.Run(value, func(t *testing.T) {
			root := RootCommand("test")
			var err error
			stdout, stderr := captureOutput(t, func() {
				err = root.ParseAndRun(context.Background(), []string{"apps", "view", "--id", "app-1", "--fields", value})
			})
			if !errors.Is(err, flag.ErrHelp) || stdout != "" || !strings.Contains(stderr, "--fields") {
				t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout, stderr)
			}
		})
	}
}

func TestAppsViewNotificationFieldsPreserveIncludedRelationships(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1", "fields[appInfos]=kidsAgeBand", "fields[apps]=subscriptionStatusUrl,appInfos,inAppPurchases,subscriptionGroups", "fields[inAppPurchases]=versions", "fields[subscriptionGroups]=versions", "include=appInfos,inAppPurchases,subscriptionGroups"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{"subscriptionStatusUrl":null}},"included":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Plus"}}]}`))
	root := RootCommand("test")
	var err error
	stdout, stderr := captureOutput(t, func() {
		err = root.ParseAndRun(context.Background(), []string{"apps", "view", "--id", "app-1", "--fields", "subscriptionStatusUrl", "--app-info-fields", "kidsAgeBand", "--iap-fields", "versions", "--subscription-group-fields", "versions", "--output", "json"})
	})
	if err != nil || !strings.HasPrefix(stderr, "Warning: AppInfo.kidsAgeBand is deprecated") || strings.Count(stderr, "\n") != 1 {
		t.Fatalf("err=%v stderr=%q", err, stderr)
	}
	var output struct {
		Included []struct {
			ID string `json:"id"`
		} `json:"included"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Included) != 1 || output.Included[0].ID != "group-1" {
		t.Fatalf("lost included resource: %s", stdout)
	}
}
