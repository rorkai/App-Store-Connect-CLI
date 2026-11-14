package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestLocalizationsViewRequestsLocalizationAndPrintsEnvelope(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantQuery string
		body      string
	}{
		{
			name:     "version by default",
			args:     []string{"--id", "loc-1"},
			wantPath: "/v1/appStoreVersionLocalizations/loc-1",
			body:     `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":null}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-1"}}`,
		},
		{
			name:      "version fields",
			args:      []string{"--id", "loc-1", "--type", "version", "--fields", "description,whatsNew"},
			wantPath:  "/v1/appStoreVersionLocalizations/loc-1",
			wantQuery: "fields%5BappStoreVersionLocalizations%5D=description%2CwhatsNew",
			body:      `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"description":"Hi","whatsNew":null}}}`,
		},
		{
			name:      "app-info fields",
			args:      []string{"--id", "info-loc-1", "--type", "app-info", "--fields", "name,subtitle"},
			wantPath:  "/v1/appInfoLocalizations/info-loc-1",
			wantQuery: "fields%5BappInfoLocalizations%5D=name%2Csubtitle",
			body:      `{"data":{"type":"appInfoLocalizations","id":"info-loc-1","attributes":{"name":"App","subtitle":null}}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(test.wantPath, strings.Split(test.wantQuery, "&")...),
				asctest.JSON(http.StatusOK, test.body))

			stdout, stderr, err := runCommand(t, append([]string{"localizations", "view", "--output", "json"}, test.args...))
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			requests := reg.Requests()
			if requestCount := len(requests); requestCount != 1 {
				t.Fatalf("expected exactly 1 request, got %d", requestCount)
			}
			if got := requests[0].URL.RawQuery; got != test.wantQuery {
				t.Fatalf("expected query %q, got %q", test.wantQuery, got)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if strings.TrimSpace(stdout) != test.body {
				t.Fatalf("expected unmodified envelope %q, got %q", test.body, stdout)
			}
		})
	}
}

func TestLocalizationsViewMarkdownShowsRequestedFields(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersionLocalizations/loc-1", "fields[appStoreVersionLocalizations]=description,supportUrl,appScreenshotSets"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"description":"Long text","supportUrl":null},"relationships":{"appScreenshotSets":{"links":{"related":"https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets"}}}}}`))

	stdout, _, err := runCommand(t, []string{"localizations", "view", "--id", "loc-1", "--fields", "description,supportUrl,appScreenshotSets", "--output", "markdown"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	for _, row := range []string{
		"| id                | loc-1",
		"| description       | Long text",
		"| supportUrl        |  ",
		"| appScreenshotSets | https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets |",
	} {
		if !strings.Contains(stdout, row) {
			t.Fatalf("expected markdown row %q, got %q", row, stdout)
		}
	}
}

func TestLocalizationsViewRejectsInvalidFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "unknown type",
			args:       []string{"--id", "loc-1", "--type", "build"},
			wantStderr: `Error: --type must be "version" or "app-info"`,
		},
		{
			name:       "field from the other type",
			args:       []string{"--id", "loc-1", "--type", "app-info", "--fields", "keywords"},
			wantStderr: "Error: --fields must be one of: locale, name, subtitle",
		},
		{
			name:       "empty fields",
			args:       []string{"--id", "loc-1", "--fields", ","},
			wantStderr: "Error: --fields must not be empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			asctest.NewRegistry(t).Install()

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(append([]string{"localizations", "view"}, test.args...), "1.2.3")
			})

			if code != 2 {
				t.Fatalf("expected exit code 2, got %d (stderr %q)", code, stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantStderr, stderr)
			}
		})
	}
}
