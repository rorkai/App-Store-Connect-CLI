package signing

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// isolateSigningFetchAppDefaults points HOME and the config path at a temp
// directory whose config.json sets a default app_id, so tests never read the
// operator's real ~/.asc configuration.
func isolateSigningFetchAppDefaults(t *testing.T, configAppID string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".asc")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"app_id":"`+configAppID+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_APP_ID", "")
	if err := os.Unsetenv("ASC_APP_ID"); err != nil {
		t.Fatal(err)
	}
}

func TestSigningFetchAppValidation(t *testing.T) {
	profileContent := base64.StdEncoding.EncodeToString([]byte("profile"))
	certificateContent := base64.StdEncoding.EncodeToString([]byte("certificate"))

	tests := []struct {
		name         string
		envAppID     string
		args         []string
		wantAppCalls int
		wantErr      string
	}{
		{
			name:         "config default app is not applied to an explicit bundle ID",
			wantAppCalls: 0,
		},
		{
			name:         "ASC_APP_ID default is not applied to an explicit bundle ID",
			envAppID:     "env-app",
			wantAppCalls: 0,
		},
		{
			name:         "explicit --app that owns another bundle ID is rejected",
			args:         []string{"--app", "flag-app"},
			wantAppCalls: 1,
			wantErr:      "bundle ID com.example.app does not match app flag-app (expected com.example.other)",
		},
		{
			name:         "explicit --app that owns the bundle ID is accepted",
			args:         []string{"--app", "owner-app"},
			wantAppCalls: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateSigningFetchAppDefaults(t, "config-app")
			if test.envAppID != "" {
				t.Setenv("ASC_APP_ID", test.envAppID)
			}

			appCalls := 0
			client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
				switch {
				case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/apps/"):
					appCalls++
					appID := strings.TrimPrefix(req.URL.Path, "/v1/apps/")
					bundle := "com.example.other"
					if appID == "owner-app" {
						bundle = "com.example.app"
					}
					return signingFetchJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"`+appID+`","attributes":{"bundleId":"`+bundle+`"}}}`)
				case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/profiles") && strings.Contains(req.URL.Path, "/certificates"):
					return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-1","attributes":{"displayName":"iOS Distribution","certificateType":"IOS_DISTRIBUTION","expirationDate":"2099-01-01T00:00:00Z","certificateContent":"`+certificateContent+`"}}]}`)
				case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/profiles"):
					return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"profiles","id":"profile-1","attributes":{"name":"App Store","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2099-01-01T00:00:00Z","profileContent":"`+profileContent+`"}}]}`)
				case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/bundleIds"):
					return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app"}}]}`)
				default:
					t.Fatalf("unexpected %s %s", req.Method, req.URL.Path)
					return nil
				}
			})
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				return client, nil
			}))

			cmd := SigningFetchCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			args := append([]string{
				"--bundle-id", "com.example.app",
				"--profile-type", "IOS_APP_STORE",
				"--output", t.TempDir(),
				"--format", "json",
			}, test.args...)
			if err := cmd.Parse(args); err != nil {
				t.Fatal(err)
			}
			var runErr error
			stdout, _ := captureOutput(t, func() {
				runErr = cmd.Run(context.Background())
			})

			if test.wantErr != "" {
				if runErr == nil || !strings.Contains(runErr.Error(), test.wantErr) {
					t.Fatalf("error = %v, want %q", runErr, test.wantErr)
				}
			} else if runErr != nil {
				t.Fatalf("run: %v\n%s", runErr, stdout)
			}
			if appCalls != test.wantAppCalls {
				t.Fatalf("app lookups = %d, want %d", appCalls, test.wantAppCalls)
			}
		})
	}
}
