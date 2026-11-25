package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAgeRatingEditSendsSocialMediaFields(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/ageRatingDeclarations/age-441" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		var payload asc.AgeRatingDeclarationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Attributes.SocialMedia == nil || payload.Data.Attributes.SocialMedia.Value == nil || !*payload.Data.Attributes.SocialMedia.Value {
			t.Fatalf("expected socialMedia=true, got %#v", payload.Data.Attributes.SocialMedia)
		}
		if payload.Data.Attributes.SocialMediaAgeRestricted == nil || payload.Data.Attributes.SocialMediaAgeRestricted.Value == nil || *payload.Data.Attributes.SocialMediaAgeRestricted.Value {
			t.Fatalf("expected socialMediaAgeRestricted=false, got %#v", payload.Data.Attributes.SocialMediaAgeRestricted)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"type":"ageRatingDeclarations","id":"age-441","attributes":{"socialMedia":true,"socialMediaAgeRestricted":false}}}`)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	client := stubClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	}))

	stdout, stderr, runErr := runRootWithDeps(t, client,
		"age-rating", "edit",
		"--id", "age-441",
		"--social-media", "true",
		"--social-media-age-restricted", "false",
		"--output", "json",
	)
	if runErr != nil {
		t.Fatalf("run: %v; stderr=%q", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var response asc.AgeRatingDeclarationResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse stdout: %v; stdout=%q", err, stdout)
	}
	if response.Data.Attributes.SocialMedia == nil || response.Data.Attributes.SocialMedia.Value == nil || !*response.Data.Attributes.SocialMedia.Value {
		t.Fatalf("expected socialMedia=true output, got %#v", response.Data.Attributes.SocialMedia)
	}
	if response.Data.Attributes.SocialMediaAgeRestricted == nil || response.Data.Attributes.SocialMediaAgeRestricted.Value == nil || *response.Data.Attributes.SocialMediaAgeRestricted.Value {
		t.Fatalf("expected socialMediaAgeRestricted=false output, got %#v", response.Data.Attributes.SocialMediaAgeRestricted)
	}
}

func TestAgeRatingEditRejectsDirectIDWithScopedSelectorsBeforeRequests(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		envApp    string
		configApp bool
		wantErr   string
	}{
		{
			name:    "app flag",
			args:    []string{"age-rating", "edit", "--id", "age-441", "--app", "100000001", "--gambling", "true"},
			wantErr: "--id cannot be combined with --app",
		},
		{
			name:    "empty app flag",
			args:    []string{"age-rating", "edit", "--id", "age-441", "--app", "", "--gambling", "true"},
			wantErr: "--app must not be empty",
		},
		{
			name:    "empty id with app info flag",
			args:    []string{"age-rating", "edit", "--id", "", "--app-info-id", "info-1", "--gambling", "true"},
			wantErr: "--id cannot be combined with --app-info-id",
		},
		{
			name:    "app info flag",
			args:    []string{"age-rating", "edit", "--id", "age-441", "--app-info-id", "info-1", "--gambling", "true"},
			wantErr: "--id cannot be combined with --app-info-id",
		},
		{
			name:    "empty app info flag",
			args:    []string{"age-rating", "edit", "--id", "age-441", "--app-info-id", "", "--gambling", "true"},
			wantErr: "--id cannot be combined with --app-info-id",
		},
		{
			name:    "version flag",
			args:    []string{"age-rating", "edit", "--id", "age-441", "--version-id", "version-1", "--gambling", "true"},
			wantErr: "--id cannot be combined with --version-id",
		},
		{
			name:    "empty version flag",
			args:    []string{"age-rating", "edit", "--id", "age-441", "--version-id", "", "--gambling", "true"},
			wantErr: "--id cannot be combined with --version-id",
		},
		{
			name:    "configured app",
			args:    []string{"age-rating", "edit", "--id", "age-441", "--gambling", "true"},
			envApp:  "100000001",
			wantErr: "--id cannot be combined with --app, ASC_APP_ID, or a configured app",
		},
		{
			name:      "stored configured app",
			args:      []string{"age-rating", "edit", "--id", "age-441", "--gambling", "true"},
			configApp: true,
			wantErr:   "--id cannot be combined with --app, ASC_APP_ID, or a configured app",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			if test.configApp {
				if err := os.Unsetenv("ASC_APP_ID"); err != nil {
					t.Fatalf("unset ASC_APP_ID: %v", err)
				}
				configPath := filepath.Join(t.TempDir(), "config.json")
				if err := os.WriteFile(configPath, []byte(`{"app_id":"100000001"}`), 0o600); err != nil {
					t.Fatalf("write config: %v", err)
				}
				t.Setenv("ASC_CONFIG_PATH", configPath)
			} else {
				t.Setenv("ASC_APP_ID", test.envApp)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			}
			reg := asctest.NewRegistry(t)
			reg.Install()

			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), test.args...)
			if code != cmd.ExitUsage || stdout != "" || !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("exit=%d stdout=%q stderr=%q, want usage containing %q", code, stdout, stderr, test.wantErr)
			}
			if requests := reg.Requests(); len(requests) != 0 {
				t.Fatalf("requests=%v, want zero requests for a preflight conflict", requests)
			}
		})
	}
}

func TestAgeRatingEditScopedAppInfoWritesAfterOwnershipCheck(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appInfos/info-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"appInfos","id":"info-1","relationships":{"app":{"data":{"type":"apps","id":"100000001"}}}}}`))
	reg.Register(asctest.GET("/v1/appInfos/info-1/ageRatingDeclaration"), asctest.JSON(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"age-441","attributes":{"gambling":false}}}`))
	reg.Register(asctest.PATCH("/v1/ageRatingDeclarations/age-441"), asctest.JSON(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"age-441","attributes":{"gambling":true}}}`))

	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "age-rating", "edit", "--app", "100000001", "--app-info-id", "info-1", "--gambling", "true", "--output", "json")
	if code != cmd.ExitSuccess || stderr != "" || !strings.Contains(stdout, `"id":"age-441"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q, want successful scoped edit", code, stdout, stderr)
	}
	requests := reg.Requests()
	if len(requests) != 3 || requests[0].URL.Path != "/v1/appInfos/info-1" || requests[1].URL.Path != "/v1/appInfos/info-1/ageRatingDeclaration" || requests[2].Method != http.MethodPatch {
		t.Fatalf("requests=%v, want ownership GET, declaration GET, and one PATCH", requests)
	}
}

func TestAgeRatingEditRefusesWrongDeclarationTypeBeforePatch(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appInfos/info-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"appInfos","id":"info-1","relationships":{"app":{"data":{"type":"apps","id":"100000001"}}}}}`))
	reg.Register(asctest.GET("/v1/appInfos/info-1/ageRatingDeclaration"), asctest.JSON(http.StatusOK, `{"data":{"type":"appInfos","id":"age-441"}}`))

	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "age-rating", "edit", "--app", "100000001", "--app-info-id", "info-1", "--gambling", "true", "--output", "json")
	if code != cmd.ExitError || stdout != "" || !strings.Contains(stderr, "expected age rating declaration") {
		t.Fatalf("exit=%d stdout=%q stderr=%q, want declaration type refusal", code, stdout, stderr)
	}
	requests := reg.Requests()
	if len(requests) != 2 || requests[0].Method != http.MethodGet || requests[0].URL.Path != "/v1/appInfos/info-1" || requests[1].Method != http.MethodGet || requests[1].URL.Path != "/v1/appInfos/info-1/ageRatingDeclaration" {
		t.Fatalf("requests=%v, want only the ownership and declaration GETs", requests)
	}
}

func TestAgeRatingViewRejectsForeignVersionBeforeDeclarationRead(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-foreign", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-foreign","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"100000002"}}}}}`))

	stdout, _, runErr := runRootWithDeps(t, stubClient(t, reg), "age-rating", "view", "--app", "100000001", "--version-id", "version-foreign", "--output", "json")
	if runErr == nil || stdout != "" || !strings.Contains(runErr.Error(), `version "version-foreign" belongs to app "100000002", not "100000001"`) {
		t.Fatalf("run error=%v stdout=%q, want foreign-version refusal", runErr, stdout)
	}
	requests := reg.Requests()
	if len(requests) != 1 || requests[0].URL.Path != "/v1/appStoreVersions/version-foreign" {
		t.Fatalf("requests=%v, want only the ownership read", requests)
	}
}

func TestAgeRatingEditDirectIDWithEmptyAppEnvOverridesStoredConfig(t *testing.T) {
	setupAuth(t)
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{"app_id":"100000001"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v1/ageRatingDeclarations/age-441"), asctest.JSON(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"age-441","attributes":{"gambling":true}}}`))

	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "age-rating", "edit", "--id", "age-441", "--gambling", "true", "--output", "json")
	if code != cmd.ExitSuccess || stderr != "" || !strings.Contains(stdout, `"id":"age-441"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q, want direct-ID PATCH with empty ASC_APP_ID", code, stdout, stderr)
	}
	requests := reg.Requests()
	if len(requests) != 1 || requests[0].Method != http.MethodPatch || requests[0].URL.Path != "/v1/ageRatingDeclarations/age-441" {
		t.Fatalf("requests=%v, want one direct declaration PATCH", requests)
	}
}

func TestAgeRatingEditInvalidSocialMediaReturnsUsageExit(t *testing.T) {
	assertUsageExitWithDeps(t, stubClient(t, asctest.NewRegistry(t)), []string{
		"age-rating", "edit", "--id", "age-441", "--social-media", "sometimes",
	}, "--social-media must be true or false")
}

func TestAgeRatingEditRejectsExplicitDependencyContradictions(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "social media with user generated content disabled",
			args: []string{
				"age-rating", "edit", "--id", "age-441",
				"--social-media", "true",
				"--user-generated-content", "false",
			},
			wantErr: "--social-media true cannot be combined with --user-generated-content false",
		},
		{
			name: "age restricted social media with age assurance disabled",
			args: []string{
				"age-rating", "edit", "--id", "age-441",
				"--social-media-age-restricted", "true",
				"--age-assurance", "false",
			},
			wantErr: "--social-media-age-restricted true cannot be combined with --age-assurance false",
		},
		{
			name: "age restricted social media with social media disabled",
			args: []string{
				"age-rating", "edit", "--id", "age-441",
				"--social-media-age-restricted", "true",
				"--social-media", "false",
			},
			wantErr: "--social-media-age-restricted true cannot be combined with --social-media false",
		},
		{
			name: "age restricted social media with user generated content disabled",
			args: []string{
				"age-rating", "edit", "--id", "age-441",
				"--social-media-age-restricted", "true",
				"--user-generated-content", "false",
			},
			wantErr: "--social-media-age-restricted true cannot be combined with --user-generated-content false",
		},
		{
			name: "all none materializes conflicting false prerequisites",
			args: []string{
				"age-rating", "edit", "--id", "age-441",
				"--all-none",
				"--social-media", "true",
			},
			wantErr: "--social-media true cannot be combined with --user-generated-content false",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitWithDeps(t, stubClient(t, asctest.NewRegistry(t)), test.args, test.wantErr)
		})
	}
}
