package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

const localizationsListVersionUUID = "3f0e8b8e-1c2d-4e5f-8a9b-0c1d2e3f4a5b"

func installLocalizationsVersionStringTransport(t *testing.T, versionsByQuery map[string]string, log *[]string) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/123456789/appStoreVersions":
			key := appStoreVersionsQueryKey(req.URL.Query())
			*log = append(*log, req.URL.Path+"?"+key)
			body, ok := versionsByQuery[key]
			if !ok {
				t.Fatalf("unexpected versions query %q", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, body)
		case "/v1/appStoreVersions/ver-1/appStoreVersionLocalizations",
			"/v1/appStoreVersions/" + localizationsListVersionUUID + "/appStoreVersionLocalizations":
			*log = append(*log, req.URL.Path)
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}],"links":{"next":""}}`)
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})
}

func TestLocalizationsListResolvesVersionStringWithApp(t *testing.T) {
	var log []string
	installLocalizationsVersionStringTransport(t, map[string]string{
		"filter[versionString]=1.0": `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION"}}],"links":{"next":""}}`,
	}, &log)

	stdout, stderr, runErr := runLocalizationsList(t, "localizations", "list", "--app", "123456789", "--version", "1.0", "--output", "json")
	if runErr != nil {
		t.Fatalf("Run() error = %v (stderr=%q)", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("expected localization envelope, got %q", stdout)
	}
	want := "/v1/apps/123456789/appStoreVersions?filter[versionString]=1.0,/v1/appStoreVersions/ver-1/appStoreVersionLocalizations"
	if got := strings.Join(log, ","); got != want {
		t.Fatalf("request sequence = %q, want %q", got, want)
	}
}

func TestLocalizationsListResolvesVersionStringWithPlatform(t *testing.T) {
	var log []string
	installLocalizationsVersionStringTransport(t, map[string]string{
		"filter[platform]=MAC_OS&filter[versionString]=1.0": `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"MAC_OS","versionString":"1.0"}}],"links":{"next":""}}`,
	}, &log)

	_, stderr, runErr := runLocalizationsList(t, "localizations", "list", "--app", "123456789", "--version", "1.0", "--platform", "mac_os")
	if runErr != nil {
		t.Fatalf("Run() error = %v (stderr=%q)", runErr, stderr)
	}
	want := "/v1/apps/123456789/appStoreVersions?filter[platform]=MAC_OS&filter[versionString]=1.0,/v1/appStoreVersions/ver-1/appStoreVersionLocalizations"
	if got := strings.Join(log, ","); got != want {
		t.Fatalf("request sequence = %q, want %q", got, want)
	}
}

func TestLocalizationsListPrefersVersionIDWhenValueIsResourceID(t *testing.T) {
	var log []string
	installLocalizationsVersionStringTransport(t, map[string]string{}, &log)

	_, stderr, runErr := runLocalizationsList(t, "localizations", "list", "--app", "123456789", "--version", localizationsListVersionUUID)
	if runErr != nil {
		t.Fatalf("Run() error = %v (stderr=%q)", runErr, stderr)
	}
	want := "/v1/appStoreVersions/" + localizationsListVersionUUID + "/appStoreVersionLocalizations"
	if got := strings.Join(log, ","); got != want {
		t.Fatalf("request sequence = %q, want %q", got, want)
	}
}

func TestLocalizationsListVersionStringErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		versions   map[string]string
		wantStderr string
		wantExit   int
	}{
		{
			name: "ambiguous across platforms",
			args: []string{"localizations", "list", "--app", "123456789", "--version", "1.0"},
			versions: map[string]string{
				"filter[versionString]=1.0": `{"data":[
					{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0"}},
					{"type":"appStoreVersions","id":"ver-2","attributes":{"platform":"MAC_OS","versionString":"1.0"}}
				],"links":{"next":""}}`,
			},
			wantStderr: "--platform",
			wantExit:   rootcmd.ExitUsage,
		},
		{
			name: "version string not found",
			args: []string{"localizations", "list", "--app", "123456789", "--version", "9.9"},
			versions: map[string]string{
				"filter[versionString]=9.9": `{"data":[],"links":{"next":""}}`,
			},
			wantStderr: `app store version not found for version "9.9"`,
			wantExit:   rootcmd.ExitNotFound,
		},
		{
			name:       "platform with version ID",
			args:       []string{"localizations", "list", "--app", "123456789", "--version", localizationsListVersionUUID, "--platform", "IOS"},
			versions:   map[string]string{},
			wantStderr: "--platform only applies when --version is omitted or is a version string with --app",
			wantExit:   rootcmd.ExitUsage,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var log []string
			installLocalizationsVersionStringTransport(t, test.versions, &log)
			stdout, stderr, runErr := runLocalizationsList(t, test.args...)
			if runErr == nil {
				t.Fatal("expected error")
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != test.wantExit {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, test.wantExit, runErr)
			}
			if !strings.Contains(stderr+runErr.Error(), test.wantStderr) {
				t.Fatalf("expected %q in stderr %q or error %v", test.wantStderr, stderr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}
