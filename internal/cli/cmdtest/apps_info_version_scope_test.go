package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAppsInfoEditVersionIDScopesSelectedApp(t *testing.T) {
	const selectedAppID = "100000001"
	tests := []struct {
		name         string
		appFlag      string
		envApp       string
		versionID    string
		ownerAppID   string
		wantError    string
		wantRequests int
	}{
		{
			name:         "foreign version refuses before localization read",
			appFlag:      selectedAppID,
			versionID:    "version-foreign",
			ownerAppID:   "100000002",
			wantError:    `version "version-foreign" belongs to app "100000002", not "100000001"`,
			wantRequests: 1,
		},
		{
			name:         "matching selected app writes",
			appFlag:      selectedAppID,
			versionID:    "version-owned",
			ownerAppID:   selectedAppID,
			wantRequests: 3,
		},
		{
			name:         "configured app provenance writes",
			envApp:       "com.example.app",
			versionID:    "version-configured",
			ownerAppID:   selectedAppID,
			wantRequests: 4,
		},
		{
			name:         "unscoped direct version ID remains supported",
			versionID:    "version-direct",
			wantRequests: 3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			if test.envApp != "" {
				t.Setenv("ASC_APP_ID", test.envApp)
			}

			reg := asctest.NewRegistry(t)
			reg.Install()
			if test.envApp != "" {
				reg.Register(
					asctest.GET("/v1/apps", "filter[bundleId]="+test.envApp, "limit=200"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"100000001","attributes":{"bundleId":"com.example.app"}}],"links":{}}`),
				)
			}

			versionPath := "/v1/appStoreVersions/" + test.versionID
			versionBody := `{"data":{"type":"appStoreVersions","id":"` + test.versionID + `","attributes":{"platform":"IOS"}`
			if test.ownerAppID != "" {
				versionBody += `,"relationships":{"app":{"data":{"type":"apps","id":"` + test.ownerAppID + `"}}}`
			}
			versionBody += "}}"
			versionMatcher := asctest.GET(versionPath)
			if test.appFlag != "" || test.envApp != "" {
				versionMatcher = asctest.GET(versionPath, "include=app")
			}
			reg.Register(versionMatcher, asctest.JSON(http.StatusOK, versionBody))

			if test.wantError == "" {
				reg.Register(
					asctest.GET(versionPath+"/appStoreVersionLocalizations", "filter[locale]=en-US", "limit=200"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Old"}}],"links":{"next":""}}`),
				)
				reg.Register(
					asctest.PATCH("/v1/appStoreVersionLocalizations/loc-1"),
					asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Bug fixes"}}}`),
				)
			}

			args := []string{"apps", "info", "edit"}
			if test.appFlag != "" {
				args = append(args, "--app", test.appFlag)
			}
			args = append(args, "--version-id", test.versionID, "--locale", "en-US", "--whats-new", "Bug fixes", "--output", "json")
			stdout, _, err := runRootWithDeps(t, stubClient(t, reg), args...)

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("err = %v, want %q", err, test.wantError)
				}
				if stdout != "" {
					t.Fatalf("stdout = %q, want empty output", stdout)
				}
				for _, request := range reg.Requests() {
					if strings.Contains(request.URL.Path, "appStoreVersionLocalizations") {
						t.Fatalf("request %s reached localization path after ownership refusal", request.URL.Path)
					}
				}
			} else if err != nil {
				t.Fatalf("run error: %v", err)
			}
			requests := reg.Requests()
			if len(requests) != test.wantRequests {
				t.Fatalf("requests = %v, want %d requests", requests, test.wantRequests)
			}
			if test.wantError == "" && requests[len(requests)-1].Method != http.MethodPatch {
				t.Fatalf("last request = %v, want PATCH", requests[len(requests)-1])
			}
		})
	}
}

func TestAppsInfoViewVersionIDRefusesForeignVersionBeforeLocalizationRead(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(
		asctest.GET("/v1/appStoreVersions/version-foreign", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-foreign","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"100000002"}}}}}`),
	)

	stdout, _, err := runRootWithDeps(
		t,
		stubClient(t, reg),
		"apps", "info", "view",
		"--app", "100000001",
		"--version-id", "version-foreign",
		"--output", "json",
	)
	if err == nil || !strings.Contains(err.Error(), `version "version-foreign" belongs to app "100000002", not "100000001"`) {
		t.Fatalf("err = %v, want foreign-version refusal", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty output", stdout)
	}
	requests := reg.Requests()
	if len(requests) != 1 || requests[0].URL.Path != "/v1/appStoreVersions/version-foreign" {
		t.Fatalf("requests = %v, want only the ownership read", requests)
	}
}

func TestAppsInfoViewNextUsesOwnedCursorVersion(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(
		asctest.GET("/v1/appStoreVersions/version-old", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-old","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"100000001"}}}}}`),
	)
	reg.Register(
		asctest.GET("/v1/appStoreVersions/version-old/appStoreVersionLocalizations", "cursor=older"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-old","attributes":{"locale":"en-US"}}],"links":{}}`),
	)

	nextURL := "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-old/appStoreVersionLocalizations?cursor=older"
	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "apps", "info", "view", "--app", "100000001", "--next", nextURL, "--output", "json")
	if err != nil || stderr != "" || !strings.Contains(stdout, `"id":"loc-old"`) {
		t.Fatalf("run error=%v stdout=%q stderr=%q, want owned older-version cursor output", err, stdout, stderr)
	}
	requests := reg.Requests()
	if len(requests) != 2 || requests[0].URL.Path != "/v1/appStoreVersions/version-old" || requests[1].URL.Path != "/v1/appStoreVersions/version-old/appStoreVersionLocalizations" {
		t.Fatalf("requests=%v, want ownership then older-version localization reads", requests)
	}
}

func TestAppsInfoViewNextRefusesForeignCursorVersionBeforeLocalizationRead(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(
		asctest.GET("/v1/appStoreVersions/version-foreign", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-foreign","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"100000002"}}}}}`),
	)

	nextURL := "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-foreign/appStoreVersionLocalizations?cursor=foreign"
	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "apps", "info", "view", "--app", "100000001", "--next", nextURL, "--output", "json")
	if err == nil || !strings.Contains(err.Error(), `version "version-foreign" belongs to app "100000002", not "100000001"`) {
		t.Fatalf("err=%v, want foreign cursor-version refusal", err)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout=%q stderr=%q, want no output", stdout, stderr)
	}
	requests := reg.Requests()
	if len(requests) != 1 || requests[0].URL.Path != "/v1/appStoreVersions/version-foreign" {
		t.Fatalf("requests=%v, want only foreign-version ownership read", requests)
	}
}

func TestAppsInfoViewNextSupportsStandaloneCursor(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(
		asctest.GET("/v1/appStoreVersions/version-old/appStoreVersionLocalizations", "cursor=standalone"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-standalone","attributes":{"locale":"en-US"}}],"links":{}}`),
	)

	nextURL := "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-old/appStoreVersionLocalizations?cursor=standalone"
	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "apps", "info", "view", "--next", nextURL, "--output", "json")
	if err != nil || stderr != "" || !strings.Contains(stdout, `"id":"loc-standalone"`) {
		t.Fatalf("run error=%v stdout=%q stderr=%q, want standalone cursor output", err, stdout, stderr)
	}
	requests := reg.Requests()
	if len(requests) != 1 || requests[0].URL.Path != "/v1/appStoreVersions/version-old/appStoreVersionLocalizations" {
		t.Fatalf("requests=%v, want one standalone cursor read", requests)
	}
}

func TestAppsInfoViewCursorHonorsPlatformAndStateFilters(t *testing.T) {
	const versionPath = "/v1/appStoreVersions/version-old"
	const localizationPath = versionPath + "/appStoreVersionLocalizations"
	for _, test := range []struct {
		name string
		args []string
		flag string
	}{
		{name: "platform mismatch", args: []string{"--platform", "MAC_OS"}, flag: "--platform"},
		{name: "legacy state mismatch", args: []string{"--state", "READY_FOR_SALE"}, flag: "--state"},
		{name: "modern state mismatch", args: []string{"--state", "PROCESSING_FOR_DISTRIBUTION"}, flag: "--state"},
		{name: "matching legacy state", args: []string{"--state", "PREPARE_FOR_SUBMISSION"}},
		{name: "matching modern state and platform", args: []string{"--state", "READY_FOR_DISTRIBUTION", "--platform", "IOS"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET(versionPath, "include=app"),
				asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-old","attributes":{"platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION","appVersionState":"READY_FOR_DISTRIBUTION"},"relationships":{"app":{"data":{"type":"apps","id":"100000001"}}}}}`))
			if test.flag == "" {
				reg.Register(asctest.GET(localizationPath, "cursor=older"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-old"}]}`))
			}
			args := []string{"apps", "info", "view", "--app", "100000001", "--next", "https://api.appstoreconnect.apple.com" + localizationPath + "?cursor=older"}
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), append(args, test.args...)...)
			count := len(reg.Requests())
			if test.flag == "" {
				if err != nil || count != 2 || !strings.Contains(stdout, `"id":"loc-old"`) {
					t.Fatalf("stdout=%q stderr=%q err=%v requests=%d, want filtered page", stdout, stderr, err, count)
				}
			} else if err == nil || stdout != "" || count != 1 || !strings.Contains(err.Error()+stderr, "does not match "+test.flag) {
				t.Fatalf("stdout=%q stderr=%q err=%v requests=%d, want filter refusal", stdout, stderr, err, count)
			}
		})
	}
}
