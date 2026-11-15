package cmdtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Both read paths must decode numeric versions without losing integer precision.
func TestAppClipsAdvancedExperiencesPreserveVersionJSON(t *testing.T) {
	const version = `9007199254740993`
	for _, command := range []struct {
		name     string
		selector string
		id       string
		path     string
	}{
		{"list", "--app-clip-id", "clip-1", "/v1/appClips/clip-1/appClipAdvancedExperiences"},
		{"view", "--experience-id", "adv-1", "/v1/appClipAdvancedExperiences/adv-1"},
	} {
		t.Run(command.name, func(t *testing.T) {
			data := `{"type":"appClipAdvancedExperiences","id":"adv-1","attributes":{"link":"https://example.com","version":` + version + `}}`
			if command.name == "list" {
				data = "[" + data + "]"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != command.path {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":`+data+`,"links":{}}`)
			}))
			t.Cleanup(server.Close)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			keyPath := filepath.Join(t.TempDir(), "key.p8")
			writeECDSAPEM(t, keyPath)
			client, err := newAppClipsTestServerClient(t, server, keyPath)
			if err != nil {
				t.Fatalf("new client: %v", err)
			}
			stdout, stderr, code := runWithDeps(t, func() (*asc.Client, error) { return client, nil }, "app-clips", "advanced-experiences", command.name, command.selector, command.id, "--output", "json")
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			var response struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &response); err != nil {
				t.Fatalf("unmarshal stdout: %v\nstdout=%q", err, stdout)
			}
			if command.name == "list" {
				var items []json.RawMessage
				if err := json.Unmarshal(response.Data, &items); err != nil || len(items) != 1 {
					t.Fatalf("expected one resource, got %s (error: %v)", response.Data, err)
				}
				response.Data = items[0]
			}
			var item struct {
				ID         string                     `json:"id"`
				Attributes map[string]json.RawMessage `json:"attributes"`
			}
			if err := json.Unmarshal(response.Data, &item); err != nil {
				t.Fatalf("unmarshal resource: %v", err)
			}
			if item.ID != "adv-1" || string(item.Attributes["version"]) != version {
				t.Fatalf("resource = %s, want id adv-1 and version %q", response.Data, version)
			}
		})
	}
}

func TestAppClipsAdvancedExperiencesCreateSupportsMultipleInlineLocalizations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/appClipAdvancedExperiences" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		var payload asc.AppClipAdvancedExperienceCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Relationships.AppClip.Data.ID != "clip-1" {
			t.Fatalf("expected app clip id clip-1, got %s", payload.Data.Relationships.AppClip.Data.ID)
		}
		if payload.Data.Relationships.HeaderImage.Data.ID != "img-1" {
			t.Fatalf("expected header image id img-1, got %s", payload.Data.Relationships.HeaderImage.Data.ID)
		}
		if len(payload.Data.Relationships.Localizations.Data) != 2 ||
			payload.Data.Relationships.Localizations.Data[0].ID != "${localization-1}" ||
			payload.Data.Relationships.Localizations.Data[1].ID != "${localization-2}" {
			t.Fatalf("unexpected localization linkage: %#v", payload.Data.Relationships.Localizations.Data)
		}
		if len(payload.Included) != 2 ||
			payload.Included[0].ID != payload.Data.Relationships.Localizations.Data[0].ID ||
			payload.Included[1].ID != payload.Data.Relationships.Localizations.Data[1].ID {
			t.Fatalf("included localization must use the relationship local ID: %#v", payload.Included)
		}
		english := payload.Included[0].Attributes
		if english.Language != asc.AppClipAdvancedExperienceLanguageEN || english.Title != "Order ahead" || english.Subtitle != "Ready when you arrive" {
			t.Fatalf("unexpected English localization: %#v", english)
		}
		french := payload.Included[1].Attributes
		if french.Language != asc.AppClipAdvancedExperienceLanguageFR || french.Title != "Commander" || french.Subtitle != "Prêt à votre arrivée" {
			t.Fatalf("unexpected French localization: %#v", french)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"type":"appClipAdvancedExperiences","id":"adv-1","attributes":{"link":"https://example.com"}},"links":{}}`))
	}))
	defer server.Close()
	client := newDefaultExperiencesCreateClient(t, server)

	stdout, stderr, err := runRootWithDeps(t, client,
		"app-clips", "advanced-experiences", "create",
		"--app-clip-id", "clip-1",
		"--link", "https://example.com",
		"--default-language", "EN",
		"--is-powered-by",
		"--header-image-id", "img-1",
		"--inline-localization", `{"language":"EN","title":"Order ahead","subtitle":"Ready when you arrive"}`,
		"--inline-localization", `{"language":"FR","title":"Commander","subtitle":"Prêt à votre arrivée"}`,
	)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var response asc.AppClipAdvancedExperienceResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("unmarshal stdout: %v\nstdout=%q", err, stdout)
	}
	if response.Data.ID != "adv-1" {
		t.Fatalf("expected advanced experience id adv-1, got %s", response.Data.ID)
	}
}

func TestAppClipsAdvancedExperiencesCreateRejectsInvalidInlineLocalizationsAsUsage(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unknown JSON field",
			args:    []string{"--inline-localization", `{"locale":"en-US","title":"Order ahead"}`},
			wantErr: `unknown field "locale"`,
		},
		{
			name:    "multiple JSON objects",
			args:    []string{"--inline-localization", `{"language":"EN","title":"Order ahead"}{"language":"FR","title":"Commander"}`},
			wantErr: "must contain exactly one JSON object",
		},
		{
			name:    "missing JSON language",
			args:    []string{"--inline-localization", `{"title":"Order ahead"}`},
			wantErr: "language is required",
		},
		{
			name:    "unsupported JSON language",
			args:    []string{"--inline-localization", `{"language":"en-US","title":"Order ahead"}`},
			wantErr: `invalid default language "en-US"`,
		},
		{
			name:    "missing JSON title",
			args:    []string{"--inline-localization", `{"language":"EN"}`},
			wantErr: "title is required",
		},
		{
			name:    "unsupported singular language",
			args:    []string{"--language", "en-US", "--title", "Order ahead"},
			wantErr: `invalid default language "en-US"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := []string{
				"app-clips", "advanced-experiences", "create",
				"--app-clip-id", "clip-1",
				"--link", "https://example.com",
				"--default-language", "EN",
				"--is-powered-by",
				"--header-image-id", "img-1",
			}
			args = append(args, test.args...)

			stdout, stderr, code := runWithDeps(t, nil, args...)

			if code != rootcmd.ExitUsage {
				t.Fatalf("expected exit code %d, got %d", rootcmd.ExitUsage, code)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAppClipsAdvancedExperienceImagesCreateSupportsUnattachedAndAttachedUploads(t *testing.T) {
	for _, test := range []struct {
		name             string
		experienceID     string
		wantExperienceID string
		wantAttach       bool
		attachStatus     int
		wantError        string
	}{
		{name: "unattached"},
		{name: "attached", experienceID: "adv-1", wantExperienceID: "adv-1", wantAttach: true},
		{name: "attachment failure preserves image id", experienceID: "adv-1", wantAttach: true, attachStatus: http.StatusUnprocessableEntity, wantError: `failed to attach uploaded image "img-1"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			imageData := []byte("advanced-image")
			filePath := filepath.Join(t.TempDir(), "advanced.png")
			if err := os.WriteFile(filePath, imageData, 0o600); err != nil {
				t.Fatalf("write image: %v", err)
			}

			var mu sync.Mutex
			var requests []string
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests = append(requests, r.Method+" "+r.URL.Path)
				mu.Unlock()

				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/v1/appClipAdvancedExperienceImages":
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					_, _ = fmt.Fprintf(w, `{"data":{"type":"appClipAdvancedExperienceImages","id":"img-1","attributes":{"uploadOperations":[{"method":"PUT","url":%q,"length":%d,"offset":0}]}},"links":{}}`, server.URL+"/upload", len(imageData))
				case r.Method == http.MethodPut && r.URL.Path == "/upload":
					_, _ = io.Copy(io.Discard, r.Body)
					w.WriteHeader(http.StatusOK)
				case r.Method == http.MethodPatch && r.URL.Path == "/v1/appClipAdvancedExperienceImages/img-1":
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"data":{"type":"appClipAdvancedExperienceImages","id":"img-1","attributes":{"fileName":"advanced.png","fileSize":14,"assetDeliveryState":{"state":"COMPLETE"}}},"links":{}}`))
				case r.Method == http.MethodPatch && r.URL.Path == "/v1/appClipAdvancedExperiences/adv-1":
					var payload asc.AppClipAdvancedExperienceUpdateRequest
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Fatalf("decode attachment payload: %v", err)
					}
					if payload.Data.Relationships == nil || payload.Data.Relationships.HeaderImage == nil ||
						payload.Data.Relationships.HeaderImage.Data.Type != asc.ResourceTypeAppClipAdvancedExperienceImages ||
						payload.Data.Relationships.HeaderImage.Data.ID != "img-1" {
						t.Fatalf("unexpected header image relationship: %#v", payload.Data.Relationships)
					}
					w.Header().Set("Content-Type", "application/json")
					if test.attachStatus != 0 {
						w.WriteHeader(test.attachStatus)
						_, _ = w.Write([]byte(`{"errors":[{"status":"422","title":"Invalid relationship"}]}`))
						return
					}
					_, _ = w.Write([]byte(`{"data":{"type":"appClipAdvancedExperiences","id":"adv-1"},"links":{}}`))
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
			}))
			defer server.Close()
			client := newDefaultExperiencesCreateClient(t, server)

			args := []string{"app-clips", "advanced-experiences", "images", "create", "--file", filePath}
			if test.experienceID != "" {
				args = append(args, "--experience-id", test.experienceID)
			}
			stdout, stderr, runErr := runRootWithDeps(t, client, args...)
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if test.wantError != "" {
				if runErr == nil || !strings.Contains(runErr.Error(), test.wantError) {
					t.Fatalf("run error = %v, want %q", runErr, test.wantError)
				}
				if stdout != "" {
					t.Fatalf("expected no success output, got %q", stdout)
				}
				return
			}
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}

			var result asc.AppClipAdvancedExperienceImageUploadResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("unmarshal stdout: %v\nstdout=%q", err, stdout)
			}
			if result.ID != "img-1" || result.ExperienceID != test.wantExperienceID || !result.Uploaded {
				t.Fatalf("unexpected upload result: %#v", result)
			}
			mu.Lock()
			defer mu.Unlock()
			attached := false
			for _, request := range requests {
				attached = attached || request == "PATCH /v1/appClipAdvancedExperiences/adv-1"
			}
			if attached != test.wantAttach {
				t.Fatalf("attach request = %t, want %t; requests=%v", attached, test.wantAttach, requests)
			}
		})
	}
}

// TestAppClipsAdvancedExperienceImagesDeleteIsRemoved locks the 5.0.0 removal
// of the unsupported `images delete` shim. App Store Connect has no delete
// endpoint for advanced experience images, so the verb is no longer
// registered and fails with the generic unknown-command usage error.
func TestAppClipsAdvancedExperienceImagesDeleteIsRemoved(t *testing.T) {
	clientFactoryCalls := 0
	clientFactory := func() (*asc.Client, error) {
		clientFactoryCalls++
		return nil, errors.New("must not authenticate")
	}

	root := RootCommand("5.0.0")
	if command := findSubcommand(root, "app-clips", "advanced-experiences", "images", "delete"); command != nil {
		t.Fatal("removed command `asc app-clips advanced-experiences images delete` is still registered")
	}
	images := findSubcommand(root, "app-clips", "advanced-experiences", "images")
	if images == nil || findSubcommand(images, "view") == nil || findSubcommand(images, "create") == nil {
		t.Fatal("`images view` and `images create` must stay registered")
	}
	if strings.Contains(images.LongHelp, "images delete") {
		t.Fatalf("images help still advertises the removed delete verb: %q", images.LongHelp)
	}

	stdout, stderr, code := runWithDeps(t, clientFactory, "app-clips", "advanced-experiences", "images", "delete", "--id", "img-1", "--confirm")

	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("expected no success output, got %q", stdout)
	}
	if !strings.HasPrefix(stderr, "Error: unknown command `asc app-clips advanced-experiences images delete`\n") {
		t.Fatalf("stderr = %q, want generic unknown-command failure", stderr)
	}
	if strings.Contains(stderr, "DEPRECATED") {
		t.Fatalf("stderr = %q, must not carry the retired shim guidance", stderr)
	}
	if clientFactoryCalls != 0 {
		t.Fatalf("expected no authentication or HTTP setup, got %d client factory calls", clientFactoryCalls)
	}
}
