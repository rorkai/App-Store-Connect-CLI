package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func installLocalizationsDefaultVersionTransport(t *testing.T, versionsByQuery map[string]string, log *[]string) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		*log = append(*log, req.URL.Path)
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			body, ok := versionsByQuery[appStoreVersionsQueryKey(req.URL.Query())]
			if !ok {
				t.Fatalf("unexpected versions query %q", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, body)
		case "/v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}],"links":{"next":""}}`)
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})
}

func runLocalizationsList(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func TestLocalizationsListDefaultsToEditableVersionWhenVersionOmitted(t *testing.T) {
	var log []string
	installLocalizationsDefaultVersionTransport(t, map[string]string{
		editableVersionStateQuery: `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.2.3","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}}],"links":{"next":""}}`,
	}, &log)

	stdout, stderr, runErr := runLocalizationsList(t, "localizations", "list", "--app", "app-1")
	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}
	wantNote := "Using version 1.2.3 (PREPARE_FOR_SUBMISSION) for platform IOS; pass --version to override\n"
	if stderr != wantNote {
		t.Fatalf("stderr = %q, want %q", stderr, wantNote)
	}
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("expected localization envelope, got %q", stdout)
	}
	if strings.Join(log, ",") != "/v1/apps/app-1/appStoreVersions,/v1/appStoreVersions/ver-1/appStoreVersionLocalizations" {
		t.Fatalf("unexpected request sequence: %v", log)
	}
}

func TestLocalizationsListDefaultsToLiveVersionWithPlatform(t *testing.T) {
	var log []string
	installLocalizationsDefaultVersionTransport(t, map[string]string{
		editableVersionStateQuery + "&filter[platform]=MAC_OS":   `{"data":[],"links":{"next":""}}`,
		liveVersionStateQuery + "&filter[platform]=MAC_OS":       `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"MAC_OS","versionString":"2.0.0","appStoreState":"READY_FOR_SALE","createdDate":"2026-01-01T00:00:00Z"}}],"links":{"next":""}}`,
		liveVersionModernStateQuery + "&filter[platform]=MAC_OS": `{"data":[],"links":{"next":""}}`,
	}, &log)

	_, stderr, runErr := runLocalizationsList(t, "localizations", "list", "--app", "app-1", "--platform", "MAC_OS")
	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}
	wantNote := "Using version 2.0.0 (READY_FOR_SALE) for platform MAC_OS; pass --version to override\n"
	if stderr != wantNote {
		t.Fatalf("stderr = %q, want %q", stderr, wantNote)
	}
}

func TestLocalizationsListDefaultVersionUsageErrors(t *testing.T) {
	var log []string
	installLocalizationsDefaultVersionTransport(t, map[string]string{
		editableVersionStateQuery: `{"data":[
			{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.2.3","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}},
			{"type":"appStoreVersions","id":"ver-2","attributes":{"platform":"TV_OS","versionString":"1.2.3","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}}
		],"links":{"next":""}}`,
	}, &log)

	tests := []struct {
		name       string
		args       []string
		wantStderr string
		wantExit   int
	}{
		{
			name:       "missing app and version",
			args:       []string{"localizations", "list"},
			wantStderr: "--version is required for version localizations (or pass --app to use the app's editable or live version)",
			wantExit:   rootcmd.ExitUsage,
		},
		{
			name:       "platform with explicit version",
			args:       []string{"localizations", "list", "--version", "ver-1", "--platform", "IOS"},
			wantStderr: "--platform only applies when --version is omitted",
			wantExit:   rootcmd.ExitUsage,
		},
		{
			name:       "platform with app-info type",
			args:       []string{"localizations", "list", "--app", "app-1", "--type", "app-info", "--platform", "IOS"},
			wantStderr: "--platform requires --type version",
			wantExit:   rootcmd.ExitUsage,
		},
		{
			name:       "platform with next continuation",
			args:       []string{"localizations", "list", "--app", "app-1", "--platform", "IOS", "--next", "https://api.appstoreconnect.apple.com/v1/appStoreVersions/ver-1/appStoreVersionLocalizations?cursor=NEXT"},
			wantStderr: "--platform cannot be combined with --next",
			wantExit:   rootcmd.ExitUsage,
		},
		{
			name:       "ambiguous platforms",
			args:       []string{"localizations", "list", "--app", "app-1"},
			wantStderr: "IOS 1.2.3 (PREPARE_FOR_SUBMISSION), TV_OS 1.2.3 (PREPARE_FOR_SUBMISSION)",
			wantExit:   rootcmd.ExitUsage,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runLocalizationsList(t, test.args...)
			if runErr == nil {
				t.Fatal("expected error")
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != test.wantExit {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, test.wantExit, runErr)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("expected %q in stderr %q", test.wantStderr, stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			_ = errors.Is(runErr, flag.ErrHelp)
		})
	}
}

// A --next continuation already encodes its version, and the client ignores the
// version ID when a next URL is set, so the default lookup must be skipped: it
// would waste a request, could announce a version the page does not belong to,
// and could fail a valid continuation when the app's defaults are ambiguous.
func TestLocalizationsListNextContinuationSkipsDefaultVersionLookup(t *testing.T) {
	var log []string
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		log = append(log, req.URL.Path)
		if req.URL.Path != "/v1/appStoreVersions/ver-page2/appStoreVersionLocalizations" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-2","attributes":{"locale":"ja"}}],"links":{"next":""}}`)
	})

	nextURL := "https://api.appstoreconnect.apple.com/v1/appStoreVersions/ver-page2/appStoreVersionLocalizations?cursor=NEXT"
	stdout, stderr, runErr := runLocalizationsList(t, "localizations", "list", "--app", "app-1", "--next", nextURL)
	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}
	for _, path := range log {
		if strings.HasSuffix(path, "/appStoreVersions") {
			t.Fatalf("continuation must not resolve a default version, requests: %v", log)
		}
	}
	if stderr != "" {
		t.Fatalf("expected no default-version note for --next, got %q", stderr)
	}
	if !strings.Contains(stdout, "loc-2") {
		t.Fatalf("expected the continuation page, got %q", stdout)
	}
}
