package cmdtest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	appclipscli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/appclips"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// The values and key a typed model would drop, rewrite, or refuse.
const appClipPlaceJSON = `{"placeId":"","names":[],"homePage":null,"displayPoint":{"coordinates":{"latitude":0}},"mapAction":"VALUE_APPLE_ADDS_LATER","fieldAppleAddsLater":1}`

func appClipExperienceUpdateArgs(extra ...string) []string {
	return append([]string{"app-clips", "advanced-experiences", "update", "--experience-id", "adv-1"}, extra...)
}

// The place must reach Apple on both verbs, from inline JSON or @file:, with
// its bytes untouched, and a command without --place must not send one.
func TestAppClipsAdvancedExperiencesPlaceRequestBody(t *testing.T) {
	placeFile := filepath.Join(t.TempDir(), "place.json")
	if err := os.WriteFile(placeFile, []byte("\n  "+appClipPlaceJSON+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		args       []string
		wantMethod string
		wantPlace  string
	}{
		{"create inline", appClipAdvancedExperienceCreateArgs("--app-clip-id", "clip-1", "--place", appClipPlaceJSON), http.MethodPost, appClipPlaceJSON},
		{"create from file", appClipAdvancedExperienceCreateArgs("--app-clip-id", "clip-1", "--place", "@file:"+placeFile), http.MethodPost, appClipPlaceJSON},
		{"create without place", appClipAdvancedExperienceCreateArgs("--app-clip-id", "clip-1"), http.MethodPost, ""},
		{"update with only place", appClipExperienceUpdateArgs("--place", appClipPlaceJSON), http.MethodPatch, appClipPlaceJSON},
		{"update from file", appClipExperienceUpdateArgs("--place", "@file:"+placeFile), http.MethodPatch, appClipPlaceJSON},
		{"update without place", appClipExperienceUpdateArgs("--action", "VIEW"), http.MethodPatch, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotMethod string
			var attributes map[string]json.RawMessage
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				var payload struct {
					Data struct {
						Attributes map[string]json.RawMessage `json:"attributes"`
					} `json:"data"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Errorf("decode payload: %v", err)
				}
				attributes = payload.Data.Attributes
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":{"type":"appClipAdvancedExperiences","id":"adv-1","attributes":{}},"links":{}}`)
			}))
			t.Cleanup(server.Close)
			keyPath := filepath.Join(t.TempDir(), "key.p8")
			writeECDSAPEM(t, keyPath)
			client, err := newAppClipsTestServerClient(t, server, keyPath)
			if err != nil {
				t.Fatalf("new client: %v", err)
			}
			factory := func() (*asc.Client, error) { return client, nil }
			t.Cleanup(appclipscli.SetClientFactory(factory))
			t.Cleanup(shared.SetASCClientFactoryForTesting(factory))

			_, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(append(test.args, "--output", "json"), "1.2.3"); code != 0 {
					t.Errorf("exit code = %d, want 0", code)
				}
			})
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if gotMethod != test.wantMethod {
				t.Fatalf("method = %q, want %q", gotMethod, test.wantMethod)
			}
			place, sent := attributes["place"]
			if test.wantPlace == "" {
				if sent {
					t.Fatalf("place = %s, want it omitted", place)
				}
				return
			}
			if !sent {
				t.Fatalf("attributes = %v, want place", attributes)
			}
			if string(place) != test.wantPlace {
				t.Fatalf("place = %s, want %s", place, test.wantPlace)
			}
		})
	}
}

// A bad place must stop with a usage error before any client or request exists.
func TestAppClipsAdvancedExperiencesPlaceValidatedBeforeClient(t *testing.T) {
	missingFile := filepath.Join(t.TempDir(), "missing.json")

	values := []struct {
		name       string
		value      string
		wantStderr string
	}{
		{"empty", "", "--place must be a JSON object"},
		{"malformed", "{", "--place must be a JSON object"},
		{"array", "[]", "--place must be a JSON object"},
		{"null", "null", "--place must be a JSON object"},
		{"second document", `{} {}`, "--place must contain exactly one JSON object"},
		{"trailing garbage", `{} x`, "--place must contain exactly one JSON object"},
		{"unreadable file", "@file:" + missingFile, "--place: cannot read file"},
	}
	commands := []struct {
		name string
		args func(...string) []string
	}{
		{"create", func(extra ...string) []string {
			return appClipAdvancedExperienceCreateArgs(append([]string{"--app-clip-id", "clip-1"}, extra...)...)
		}},
		{"update", appClipExperienceUpdateArgs},
	}

	for _, command := range commands {
		for _, value := range values {
			t.Run(command.name+"/"+value.name, func(t *testing.T) {
				clientCalls := 0
				factory := func() (*asc.Client, error) {
					clientCalls++
					return nil, errors.New("client factory must not run")
				}
				t.Cleanup(appclipscli.SetClientFactory(factory))
				t.Cleanup(shared.SetASCClientFactoryForTesting(factory))

				var exitCode int
				stdout, stderr := captureOutput(t, func() {
					exitCode = rootcmd.Run(command.args("--place", value.value), "1.2.3")
				})
				if exitCode != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d; stderr = %q", exitCode, rootcmd.ExitUsage, stderr)
				}
				if stdout != "" {
					t.Fatalf("stdout = %q, want empty", stdout)
				}
				if !strings.Contains(stderr, value.wantStderr) {
					t.Fatalf("stderr = %q, want it to contain %q", stderr, value.wantStderr)
				}
				if clientCalls != 0 {
					t.Fatalf("client factory calls = %d, want 0", clientCalls)
				}
			})
		}
	}
}
