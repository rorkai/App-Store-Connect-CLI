package cmdtest

import (
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyticsRankedStringAliasesMatchCanonicalCommands(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = analyticsAliasTransport(t)

	tests := []struct {
		name          string
		canonicalArgs []string
		aliasArgs     []string
		alias         string
		canonical     string
	}{
		{
			name:          "versions view id",
			canonicalArgs: []string{"versions", "view", "--version-id", "version-1"},
			aliasArgs:     []string{"versions", "view", "--id", "version-1"},
			alias:         "id",
			canonical:     "version-id",
		},
		{
			name:          "versions attach build id",
			canonicalArgs: []string{"versions", "attach-build", "--version-id", "version-1", "--build-id", "build-1"},
			aliasArgs:     []string{"versions", "attach-build", "--version-id", "version-1", "--build", "build-1"},
			alias:         "build",
			canonical:     "build-id",
		},
		{
			name:          "localizations list version id",
			canonicalArgs: []string{"localizations", "list", "--version", "version-1"},
			aliasArgs:     []string{"localizations", "list", "--version-id", "version-1"},
			alias:         "version-id",
			canonical:     "version",
		},
		{
			name:          "subscriptions view subscription id",
			canonicalArgs: []string{"subscriptions", "view", "--id", "sub-1"},
			aliasArgs:     []string{"subscriptions", "view", "--subscription-id", "sub-1"},
			alias:         "subscription-id",
			canonical:     "id",
		},
		{
			name:          "testflight groups view group id",
			canonicalArgs: []string{"testflight", "groups", "view", "--id", "group-1"},
			aliasArgs:     []string{"testflight", "groups", "view", "--group-id", "group-1"},
			alias:         "group-id",
			canonical:     "id",
		},
		{
			name:          "iap localization id",
			canonicalArgs: []string{"iap", "localizations", "update", "--localization-id", "iap-loc-1", "--name", "Updated"},
			aliasArgs:     []string{"iap", "localizations", "update", "--id", "iap-loc-1", "--name", "Updated"},
			alias:         "id",
			canonical:     "localization-id",
		},
		{
			name:          "subscription screenshot id",
			canonicalArgs: []string{"subscriptions", "review", "screenshots", "delete", "--screenshot-id", "shot-1", "--confirm"},
			aliasArgs:     []string{"subscriptions", "review", "screenshots", "delete", "--id", "shot-1", "--confirm"},
			alias:         "id",
			canonical:     "screenshot-id",
		},
		{
			name:          "versions update id",
			canonicalArgs: []string{"versions", "update", "--version-id", "version-1", "--copyright", "2026 Example"},
			aliasArgs:     []string{"versions", "update", "--id", "version-1", "--copyright", "2026 Example"},
			alias:         "id",
			canonical:     "version-id",
		},
		{
			name:          "localizations update version id",
			canonicalArgs: []string{"localizations", "update", "--version", "version-1", "--locale", "en-US", "--description", "Updated"},
			aliasArgs:     []string{"localizations", "update", "--version-id", "version-1", "--locale", "en-US", "--description", "Updated"},
			alias:         "version-id",
			canonical:     "version",
		},
		{
			name:          "apps view app",
			canonicalArgs: []string{"apps", "view", "--id", "app-1"},
			aliasArgs:     []string{"apps", "view", "--app", "app-1"},
			alias:         "app",
			canonical:     "id",
		},
		{
			name:          "builds list app id",
			canonicalArgs: []string{"builds", "list", "--app", "123456789"},
			aliasArgs:     []string{"builds", "list", "--app-id", "123456789"},
			alias:         "app-id",
			canonical:     "app",
		},
		{
			name:          "bundle capabilities bundle id",
			canonicalArgs: []string{"bundle-ids", "capabilities", "list", "--bundle", "bundle-1"},
			aliasArgs:     []string{"bundle-ids", "capabilities", "list", "--bundle-id", "bundle-1"},
			alias:         "bundle-id",
			canonical:     "bundle",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			canonicalStdout, canonicalStderr, canonicalErr := runCommand(t, test.canonicalArgs)
			if canonicalErr != nil {
				t.Fatalf("canonical command error: %v", canonicalErr)
			}
			assertOnlyDeprecatedCommandWarnings(t, canonicalStderr)

			aliasStdout, aliasStderr, aliasErr := runCommand(t, test.aliasArgs)
			if aliasErr != nil {
				t.Fatalf("alias command error: %v", aliasErr)
			}
			warning := "Warning: `--" + test.alias + "` is deprecated. Use `--" + test.canonical + "`."
			requireStderrContainsWarning(t, aliasStderr, warning)
			assertOnlyDeprecatedCommandWarnings(t, aliasStderr)
			if aliasStdout != canonicalStdout {
				t.Fatalf("alias stdout differs from canonical output:\ncanonical: %q\nalias: %q", canonicalStdout, aliasStdout)
			}
		})
	}
}

func TestAnalyticsRankedStringAliasesRejectConflictingValues(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		alias     string
		canonical string
	}{
		{name: "versions view", args: []string{"versions", "view", "--version-id", "one", "--id", "two"}, alias: "id", canonical: "version-id"},
		{name: "versions view empty canonical", args: []string{"versions", "view", "--version-id", "", "--id", "two"}, alias: "id", canonical: "version-id"},
		{name: "versions view empty alias", args: []string{"versions", "view", "--version-id", "one", "--id", ""}, alias: "id", canonical: "version-id"},
		{name: "versions attach-build", args: []string{"versions", "attach-build", "--version-id", "version-1", "--build-id", "one", "--build", "two"}, alias: "build", canonical: "build-id"},
		{name: "localizations list", args: []string{"localizations", "list", "--version", "one", "--version-id", "two"}, alias: "version-id", canonical: "version"},
		{name: "subscriptions view", args: []string{"subscriptions", "view", "--id", "one", "--subscription-id", "two"}, alias: "subscription-id", canonical: "id"},
		{name: "testflight groups view", args: []string{"testflight", "groups", "view", "--id", "one", "--group-id", "two"}, alias: "group-id", canonical: "id"},
		{name: "iap localizations update", args: []string{"iap", "localizations", "update", "--localization-id", "one", "--id", "two", "--name", "Updated"}, alias: "id", canonical: "localization-id"},
		{name: "subscriptions screenshots delete", args: []string{"subscriptions", "review", "screenshots", "delete", "--screenshot-id", "one", "--id", "two", "--confirm"}, alias: "id", canonical: "screenshot-id"},
		{name: "versions update", args: []string{"versions", "update", "--version-id", "one", "--id", "two", "--copyright", "2026 Example"}, alias: "id", canonical: "version-id"},
		{name: "localizations update", args: []string{"localizations", "update", "--version", "one", "--version-id", "two", "--locale", "en-US", "--description", "Updated"}, alias: "version-id", canonical: "version"},
		{name: "apps view", args: []string{"apps", "view", "--id", "one", "--app", "two"}, alias: "app", canonical: "id"},
		{name: "builds list", args: []string{"builds", "list", "--app", "one", "--app-id", "two"}, alias: "app-id", canonical: "app"},
		{name: "bundle capabilities", args: []string{"bundle-ids", "capabilities", "list", "--bundle", "one", "--bundle-id", "two"}, alias: "bundle-id", canonical: "bundle"},
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("conflicting aliases must fail before HTTP: %s %s", req.Method, req.URL.String())
		return nil, errors.New("unexpected request")
	})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runCommand(t, test.args)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("run error = %v, want usage error", runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			want := "Error: --" + test.alias + " conflicts with --" + test.canonical + "; use only --" + test.canonical
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want containing %q", stderr, want)
			}
			warning := "Warning: `--" + test.alias + "` is deprecated. Use `--" + test.canonical + "`."
			requireStderrContainsWarning(t, stderr, warning)
		})
	}
}

func analyticsAliasTransport(t *testing.T) http.RoundTripper {
	t.Helper()

	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			return analyticsAliasJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0"}}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return analyticsAliasJSONResponse(http.StatusNoContent, ""), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1":
			return analyticsAliasJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","copyright":"2026 Example"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return analyticsAliasJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Current"}}],"links":{}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-1":
			return analyticsAliasJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Updated"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1":
			return analyticsAliasJSONResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Subscription"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/betaGroups/group-1":
			return analyticsAliasJSONResponse(http.StatusOK, `{"data":{"type":"betaGroups","id":"group-1","attributes":{"name":"Group"}}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/inAppPurchaseLocalizations/iap-loc-1":
			return analyticsAliasJSONResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseLocalizations","id":"iap-loc-1","attributes":{"locale":"en-US","name":"Updated","description":"Description"}}}`), nil
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			return analyticsAliasJSONResponse(http.StatusNoContent, ""), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1":
			return analyticsAliasJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{"name":"App","bundleId":"com.example.app","sku":"APP","primaryLocale":"en-US"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			if got := req.URL.Query().Get("filter[app]"); got != "123456789" {
				t.Fatalf("build filter = %q, want 123456789", got)
			}
			return analyticsAliasJSONResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"1","uploadedDate":"2026-07-15T00:00:00Z","processingState":"VALID","expired":false}}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-1/bundleIdCapabilities":
			return analyticsAliasJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIdCapabilities","id":"cap-1","attributes":{"settings":[]}}],"links":{}}`), nil
		default:
			t.Fatalf("unexpected alias test request: %s %s", req.Method, req.URL.String())
			return nil, errors.New("unexpected request")
		}
	})
}

func analyticsAliasJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
