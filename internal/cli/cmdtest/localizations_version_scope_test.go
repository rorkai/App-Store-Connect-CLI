package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestLocalizationsListSelectedVersionChecksOwnership(t *testing.T) {
	for _, test := range []struct {
		name      string
		owner     string
		wantError string
	}{
		{name: "matching selected app", owner: "100000001"},
		{name: "foreign selected app", owner: "100000002", wantError: `belongs to app "100000002", not "100000001"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(
				asctest.GET("/v1/appStoreVersions/version-selected", "include=app"),
				asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-selected","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"`+test.owner+`"}}}}}`),
			)
			if test.wantError == "" {
				reg.Register(
					asctest.GET("/v1/appStoreVersions/version-selected/appStoreVersionLocalizations"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-selected","attributes":{"locale":"en-US"}}],"links":{}}`),
				)
			}
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "localizations", "list", "--app", "100000001", "--version", "version-selected", "--output", "json")
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) || stdout != "" || stderr != "" {
					t.Fatalf("err=%v stdout=%q stderr=%q, want selected-app refusal", err, stdout, stderr)
				}
				return
			}
			if err != nil || stderr != "" || !strings.Contains(stdout, `"id":"loc-selected"`) {
				t.Fatalf("err=%v stdout=%q stderr=%q, want matching localization output", err, stdout, stderr)
			}
		})
	}
}

func TestLocalizationsListSelectedCursorChecksVersionOwnership(t *testing.T) {
	nextURL := "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-cursor/appStoreVersionLocalizations?cursor=next"
	for _, test := range []struct {
		name      string
		owner     string
		wantError string
	}{
		{name: "matching selected app", owner: "100000001"},
		{name: "foreign cursor version", owner: "100000002", wantError: `belongs to app "100000002", not "100000001"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(
				asctest.GET("/v1/appStoreVersions/version-cursor", "include=app"),
				asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-cursor","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"`+test.owner+`"}}}}}`),
			)
			if test.wantError == "" {
				reg.Register(
					asctest.GET("/v1/appStoreVersions/version-cursor/appStoreVersionLocalizations", "cursor=next"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-cursor","attributes":{"locale":"en-US"}}],"links":{}}`),
				)
			}
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "localizations", "list", "--app", "100000001", "--next", nextURL, "--output", "json")
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) || stdout != "" || stderr != "" {
					t.Fatalf("err=%v stdout=%q stderr=%q, want cursor refusal", err, stdout, stderr)
				}
				return
			}
			if err != nil || stderr != "" || !strings.Contains(stdout, `"id":"loc-cursor"`) {
				t.Fatalf("err=%v stdout=%q stderr=%q, want matching cursor output", err, stdout, stderr)
			}
		})
	}
}

func TestLocalizationsListRejectsMismatchedVersionAndCursor(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	nextURL := "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-cursor/appStoreVersionLocalizations?cursor=next"
	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "localizations", "list", "--app", "100000001", "--version", "version-other", "--next", nextURL, "--output", "json")
	if err == nil || !strings.Contains(err.Error(), "does not match") || stdout != "" || !strings.Contains(stderr, "does not match") {
		t.Fatalf("err=%v stdout=%q stderr=%q, want selector mismatch before provider reads", err, stdout, stderr)
	}
	if requests := reg.Requests(); len(requests) != 0 {
		t.Fatalf("requests=%v, want no provider reads for mismatched selectors", requests)
	}
}

func TestLocalizationsListStandaloneSelectorsRemainUnscoped(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(
		asctest.GET("/v1/appStoreVersions/version-direct/appStoreVersionLocalizations"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-direct","attributes":{"locale":"en-US"}}],"links":{}}`),
	)
	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "localizations", "list", "--version", "version-direct", "--output", "json")
	if err != nil || stderr != "" || !strings.Contains(stdout, `"id":"loc-direct"`) {
		t.Fatalf("err=%v stdout=%q stderr=%q, want standalone direct-ID output", err, stdout, stderr)
	}
	if requests := reg.Requests(); len(requests) != 1 || requests[0].URL.Path != "/v1/appStoreVersions/version-direct/appStoreVersionLocalizations" {
		t.Fatalf("requests=%v, want one standalone direct-ID read", requests)
	}

	reg.Register(
		asctest.GET("/v1/appStoreVersions/version-cursor/appStoreVersionLocalizations", "cursor=standalone"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-standalone","attributes":{"locale":"en-US"}}],"links":{}}`),
	)
	nextURL := "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-cursor/appStoreVersionLocalizations?cursor=standalone"
	stdout, stderr, err = runRootWithDeps(t, stubClient(t, reg), "localizations", "list", "--next", nextURL, "--output", "json")
	if err != nil || stderr != "" || !strings.Contains(stdout, `"id":"loc-standalone"`) {
		t.Fatalf("err=%v stdout=%q stderr=%q, want standalone cursor output", err, stdout, stderr)
	}
	requests := reg.Requests()
	if len(requests) != 2 || requests[1].URL.Path != "/v1/appStoreVersions/version-cursor/appStoreVersionLocalizations" {
		t.Fatalf("requests=%v, want direct-ID then standalone cursor reads", requests)
	}
}

func TestLocalizationsListVersionStringRejectsUnverifiedSingleton(t *testing.T) {
	for _, test := range []struct {
		name      string
		platform  string
		data      string
		wantError string
		query     []string
	}{
		{name: "wrong resource type", data: `{"type":"appInfos","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}`, wantError: "expected resource type", query: []string{"filter[versionString]=1.2.3", "limit=200"}},
		{name: "missing ID", data: `{"type":"appStoreVersions","id":"","attributes":{"platform":"IOS","versionString":"1.2.3"}}`, wantError: "empty ID", query: []string{"filter[versionString]=1.2.3", "limit=200"}},
		{name: "different version string", data: `{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.17.0"}}`, wantError: "returned version", query: []string{"filter[versionString]=1.2.3", "limit=200"}},
		{name: "different platform", platform: "IOS", data: `{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"MAC_OS","versionString":"1.2.3"}}`, wantError: "returned platform", query: []string{"filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=200"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(
				asctest.GET("/v1/apps/100000001/appStoreVersions", test.query...),
				asctest.JSON(http.StatusOK, `{"data":[`+test.data+`],"links":{}}`),
			)
			args := []string{"localizations", "list", "--app", "100000001", "--version", "1.2.3"}
			if test.platform != "" {
				args = append(args, "--platform", test.platform)
			}
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), args...)
			if err == nil || !strings.Contains(err.Error(), test.wantError) || stdout != "" || stderr != "" {
				t.Fatalf("err=%v stdout=%q stderr=%q, want singleton validation failure", err, stdout, stderr)
			}
		})
	}
}
