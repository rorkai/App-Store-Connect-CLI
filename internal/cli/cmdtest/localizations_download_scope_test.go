package cmdtest

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestLocalizationsDownloadSelectedAppRejectsForeignVersionBeforeLocalizationRead(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-foreign", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-foreign","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"100000002"}}}}}`))

	outputDir := filepath.Join(t.TempDir(), "localizations")
	stdout, _, runErr := runRootWithDeps(t, stubClient(t, reg), "localizations", "download", "--app", "100000001", "--version", "version-foreign", "--path", outputDir)
	if runErr == nil || stdout != "" || !strings.Contains(runErr.Error(), `belongs to app "100000002", not "100000001"`) {
		t.Fatalf("run error=%v stdout=%q, want selected-app ownership refusal", runErr, stdout)
	}
	requests := reg.Requests()
	if len(requests) != 1 || requests[0].URL.Path != "/v1/appStoreVersions/version-foreign" {
		t.Fatalf("requests=%v, want only typed version ownership read", requests)
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("output directory exists after refusal: %v", err)
	}
}

func TestLocalizationsDownloadSelectedAppReadsOwnedVersionAndBindsPagination(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	const nextURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=page-2"
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"100000001"}}}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"English"}}],"links":{"next":"`+nextURL+`"}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "cursor=page-2"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Japanese"}}],"links":{"next":""}}`))

	outputDir := filepath.Join(t.TempDir(), "localizations")
	stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "localizations", "download", "--app", "100000001", "--version", "version-1", "--path", outputDir)
	if runErr != nil || stderr != "" {
		t.Fatalf("run error=%v stderr=%q stdout=%q", runErr, stderr, stdout)
	}
	for _, name := range []string{"en-US.strings", "ja.strings"} {
		if _, err := os.Stat(filepath.Join(outputDir, name)); err != nil {
			t.Fatalf("expected %s to be written: %v", name, err)
		}
	}
	requests := reg.Requests()
	if len(requests) != 3 || requests[0].URL.Path != "/v1/appStoreVersions/version-1" || requests[1].URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" || requests[1].URL.Query().Get("limit") != "200" || requests[2].URL.String() != nextURL {
		t.Fatalf("requests=%v, want ownership then two bound localization reads", requests)
	}
}

func TestLocalizationsDownloadSelectedAppRejectsForeignOrRedirectedContinuation(t *testing.T) {
	tests := []struct {
		name       string
		nextURL    string
		wantErr    string
		wantTarget string
	}{
		{
			name:       "foreign version",
			nextURL:    "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-foreign/appStoreVersionLocalizations?cursor=next",
			wantErr:    "rejected scoped pagination URL path",
			wantTarget: "/v1/appStoreVersions/version-1/appStoreVersionLocalizations",
		},
		{
			name:       "redirect host",
			nextURL:    "https://offscope.example.invalid/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=next",
			wantErr:    "rejected scoped pagination URL origin",
			wantTarget: "/v1/appStoreVersions/version-1/appStoreVersionLocalizations",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"100000001"}}}}}`))
			reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"English"}}],"links":{"next":"`+test.nextURL+`"}}`))

			outputDir := filepath.Join(t.TempDir(), "localizations")
			stdout, _, runErr := runRootWithDeps(t, stubClient(t, reg), "localizations", "download", "--app", "100000001", "--version", "version-1", "--path", outputDir)
			if runErr == nil || stdout != "" || !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("run error=%v stdout=%q, want continuation refusal", runErr, stdout)
			}
			requests := reg.Requests()
			if len(requests) != 2 || requests[1].URL.Path != test.wantTarget {
				t.Fatalf("requests=%v, want ownership and initial page only", requests)
			}
			if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
				t.Fatalf("output directory exists after refusal: %v", err)
			}
		})
	}
}
