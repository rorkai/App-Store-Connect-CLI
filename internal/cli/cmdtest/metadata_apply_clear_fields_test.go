package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func assertMetadataPatchRawAttributes(t *testing.T, req *http.Request, resourceType, id string, want map[string]string) {
	t.Helper()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read PATCH body: %v", err)
	}
	var payload struct {
		Data struct {
			Type       string                     `json:"type"`
			ID         string                     `json:"id"`
			Attributes map[string]json.RawMessage `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode PATCH body %s: %v", body, err)
	}
	if payload.Data.Type != resourceType || payload.Data.ID != id {
		t.Fatalf("unexpected PATCH identity: %+v", payload.Data)
	}
	if len(payload.Data.Attributes) != len(want) {
		t.Fatalf("PATCH attributes = %s, want exactly %v", body, want)
	}
	for field, wantValue := range want {
		raw, ok := payload.Data.Attributes[field]
		if !ok {
			t.Fatalf("PATCH attributes = %s, want field %q", body, field)
		}
		if string(raw) != wantValue {
			t.Fatalf("PATCH attribute %q = %s, want %s", field, raw, wantValue)
		}
	}
}

func writeMetadataFile(t *testing.T, dir string, parts []string, body string) {
	t.Helper()
	target := filepath.Join(append([]string{dir}, parts...)...)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(target), err)
	}
	if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", target, err)
	}
}

func TestMetadataApplySendsNullForExplicitlyClearedAppInfoFields(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	writeMetadataFile(t, dir, []string{"app-info", "en-US.json"}, `{"name":"Outslept","subtitle":null,"privacyPolicyUrl":null}`)

	patches := 0
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","subtitle":"Sleep tracker","privacyPolicyUrl":"https://example.com/privacy"}}],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appInfoLocalizations/loc-en":
			if req.Method != http.MethodPatch {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			}
			patches++
			assertMetadataPatchRawAttributes(t, req, "appInfoLocalizations", "loc-en", map[string]string{
				"name":             `"Outslept"`,
				"subtitle":         "null",
				"privacyPolicyUrl": "null",
			})
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","subtitle":null,"privacyPolicyUrl":null}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	stdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--confirm",
		"--output", "json",
	})

	var result struct {
		Applied   bool `json:"applied"`
		Succeeded int  `json:"succeeded"`
		Failed    int  `json:"failed"`
		Updates   []struct {
			Key string `json:"key"`
		} `json:"updates"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse apply output: %v\n%s", err, stdout)
	}
	if !result.Applied || result.Succeeded != 1 || result.Failed != 0 || patches != 1 {
		t.Fatalf("unexpected apply result: %+v patches=%d", result, patches)
	}
	if len(result.Updates) != 2 {
		t.Fatalf("expected planned clears for both fields, got %+v", result.Updates)
	}
}

func TestMetadataMutationRequiresConfirmForPlannedClear(t *testing.T) {
	for _, operation := range []string{"push", "apply"} {
		t.Run(operation, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")

			dir := t.TempDir()
			writeMetadataFile(t, dir, []string{"app-info", "en-US.json"}, `{"subtitle":null}`)

			mutations := 0
			withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					mutations++
				}
				switch req.URL.Path {
				case "/v1/apps/app-1/appInfos":
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
				case "/v1/apps/app-1/appStoreVersions":
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
				case "/v1/appInfos/appinfo-1/appInfoLocalizations":
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","subtitle":"Sleep tracker"}}],"links":{"next":""}}`), nil
				case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			})

			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run([]string{"metadata", operation, "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir, "--output", "json"}, "1.2.3")
				if code != cmd.ExitUsage {
					t.Fatalf("expected usage exit, got %d", code)
				}
			})
			if stdout != "" || !strings.Contains(stderr, "--confirm is required when applying field clear operations") {
				t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout, stderr)
			}
			if mutations != 0 {
				t.Fatalf("mutations = %d, want zero", mutations)
			}
		})
	}
}

func TestMetadataApplySendsNullForExplicitlyClearedPromotionalText(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	writeMetadataFile(t, dir, []string{"version", "1.2.3", "en-US.json"}, `{"promotionalText":null}`)

	patches := 0
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Remote description","promotionalText":"Old promo"}}],"links":{"next":""}}`), nil
		case "/v1/appStoreVersionLocalizations/loc-en":
			if req.Method != http.MethodPatch {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			}
			patches++
			assertMetadataPatchRawAttributes(t, req, "appStoreVersionLocalizations", "loc-en", map[string]string{
				"promotionalText": "null",
			})
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Remote description","promotionalText":null}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	stdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--confirm",
		"--output", "json",
	})

	var result struct {
		Applied   bool `json:"applied"`
		Succeeded int  `json:"succeeded"`
		Failed    int  `json:"failed"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse apply output: %v\n%s", err, stdout)
	}
	if !result.Applied || result.Succeeded != 1 || result.Failed != 0 || patches != 1 {
		t.Fatalf("unexpected apply result: %+v patches=%d", result, patches)
	}
}

func TestMetadataApplySkipsClearWhenRemoteFieldIsAlreadyEmpty(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	writeMetadataFile(t, dir, []string{"app-info", "en-US.json"}, `{"name":"Outslept","subtitle":null}`)

	mutations := 0
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			mutations++
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept"}}],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	stdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--output", "json",
	})

	var result struct {
		Total   int `json:"total"`
		Updates []struct {
			Key string `json:"key"`
		} `json:"updates"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse apply output: %v\n%s", err, stdout)
	}
	if result.Total != 0 || len(result.Updates) != 0 || mutations != 0 {
		t.Fatalf("expected no-op apply, got %+v mutations=%d", result, mutations)
	}
}

func TestMetadataApplyOmitsNoopClearFromMixedPatchWithoutConfirm(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	writeMetadataFile(t, dir, []string{"app-info", "en-US.json"}, `{"subtitle":null,"privacyPolicyUrl":"https://example.com/new"}`)

	patches := 0
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			if patches == 0 {
				return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","privacyPolicyUrl":"https://example.com/old"}}],"links":{"next":""}}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","privacyPolicyUrl":"https://example.com/new"}}],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appInfoLocalizations/loc-en":
			if req.Method != http.MethodPatch {
				t.Fatalf("unexpected localization request: %s %s", req.Method, req.URL.Path)
			}
			patches++
			assertMetadataPatchRawAttributes(t, req, "appInfoLocalizations", "loc-en", map[string]string{
				"privacyPolicyUrl": `"https://example.com/new"`,
			})
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","privacyPolicyUrl":"https://example.com/new"}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	stdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--output", "json",
	})

	var result struct {
		Applied bool `json:"applied"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse apply output: %v\n%s", err, stdout)
	}
	if !result.Applied || patches != 1 {
		t.Fatalf("unexpected mixed set/no-op clear apply: applied=%t patches=%d", result.Applied, patches)
	}
}

func TestMetadataClearOnlyPatchRejectsMissingVersionLocaleBeforeMutation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	writeMetadataFile(t, dir, []string{"version", "1.2.3", "en-US.json"}, `{"promotionalText":null}`)

	mutations := 0
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			mutations++
			t.Fatalf("clear-only patch must not mutate a missing locale: %s %s", req.Method, req.URL.Path)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	for _, command := range []string{"plan", "apply"} {
		t.Run(command, func(t *testing.T) {
			args := []string{
				"metadata", command,
				"--app", "app-1",
				"--version", "1.2.3",
				"--platform", "IOS",
				"--dir", dir,
				"--output", "json",
			}
			if command == "plan" {
				args = append(args, "--review-dir", t.TempDir())
			}
			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(args, "1.2.3"); code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want usage exit code %d", code, cmd.ExitUsage)
				}
			})
			if stdout != "" || !strings.Contains(stderr, `version localization "en-US" cannot be cleared because no existing localization was found`) {
				t.Fatalf("expected only the usage error on stderr, got stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
	if mutations != 0 {
		t.Fatalf("mutation count = %d, want 0", mutations)
	}
}

func TestMetadataClearOnlyAppInfoRejectsMissingLocaleBeforeMutation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	writeMetadataFile(t, dir, []string{"app-info", "en-US.json"}, `{"subtitle":null}`)

	mutations := 0
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			mutations++
			t.Fatalf("clear-only patch must not mutate a missing locale: %s %s", req.Method, req.URL.Path)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	for _, command := range []string{"plan", "apply"} {
		t.Run(command, func(t *testing.T) {
			args := []string{
				"metadata", command,
				"--app", "app-1",
				"--version", "1.2.3",
				"--platform", "IOS",
				"--dir", dir,
				"--output", "json",
			}
			if command == "plan" {
				args = append(args, "--review-dir", t.TempDir())
			}
			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(args, "1.2.3"); code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want usage exit code %d", code, cmd.ExitUsage)
				}
			})
			if stdout != "" || !strings.Contains(stderr, `app-info localization "en-US" requires name when creating a new locale`) {
				t.Fatalf("expected clear-only missing-locale usage error, got stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
	if mutations != 0 {
		t.Fatalf("mutation count = %d, want 0", mutations)
	}
}

func TestMetadataPlanAndApplyRejectClearFieldsInDefaultFilesBeforeRequests(t *testing.T) {
	cases := []struct {
		name    string
		path    []string
		body    string
		wantErr string
	}{
		{
			name:    "app-info",
			path:    []string{"app-info", "default.json"},
			body:    `{"name":"Outslept","subtitle":null}`,
			wantErr: "clear fields in app-info/default.json are not supported",
		},
		{
			name:    "version",
			path:    []string{"version", "1.2.3", "default.json"},
			body:    `{"description":"English description","promotionalText":null}`,
			wantErr: "clear fields in version/1.2.3/default.json are not supported",
		},
	}

	for _, tc := range cases {
		for _, command := range []string{"plan", "apply"} {
			t.Run(tc.name+"/"+command, func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
				t.Setenv("ASC_APP_ID", "")

				dir := t.TempDir()
				writeMetadataFile(t, dir, tc.path, tc.body)
				requests := 0
				withMetadataReviewHTTP(t, func(*http.Request) (*http.Response, error) {
					requests++
					return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","title":"unexpected request"}]}`), nil
				})

				args := []string{
					"metadata", command,
					"--app", "app-1",
					"--version", "1.2.3",
					"--platform", "IOS",
					"--dir", dir,
					"--output", "json",
				}
				if command == "plan" {
					args = append(args, "--review-dir", t.TempDir())
				}
				code := 0
				stdout, stderr := captureOutput(t, func() {
					code = cmd.Run(args, "1.2.3")
				})
				if code != cmd.ExitUsage {
					t.Errorf("exit code = %d, want usage exit code %d; stderr=%q", code, cmd.ExitUsage, stderr)
				}
				if stdout != "" || !strings.Contains(stderr, tc.wantErr) {
					t.Errorf("expected default clear usage error, got stdout=%q stderr=%q", stdout, stderr)
				}
				if requests != 0 {
					t.Errorf("default clear validation must happen before network requests, got %d", requests)
				}
			})
		}
	}
}

func TestMetadataApplyMixedSetAndClearCreatesWithoutNullAttribute(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	writeMetadataFile(t, dir, []string{"version", "1.2.3", "fr-FR.json"}, `{"description":"Nouvelle description","keywords":"sommeil,sante","supportUrl":"https://example.com/support","whatsNew":"Premiere version","promotionalText":null}`)

	creates := 0
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			if req.Method != http.MethodGet {
				t.Fatalf("unexpected collection method: %s", req.Method)
			}
			if creates == 0 {
				return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","description":"Nouvelle description","keywords":"sommeil,sante","supportUrl":"https://example.com/support","whatsNew":"Premiere version"}}],"links":{"next":""}}`), nil
		case "/v1/appStoreVersionLocalizations":
			if req.Method != http.MethodPost {
				t.Fatalf("unexpected localization request: %s", req.Method)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read POST body: %v", err)
			}
			var payload struct {
				Data struct {
					Attributes map[string]json.RawMessage `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode POST body %s: %v", body, err)
			}
			if string(payload.Data.Attributes["locale"]) != `"fr-FR"` ||
				string(payload.Data.Attributes["description"]) != `"Nouvelle description"` ||
				string(payload.Data.Attributes["keywords"]) != `"sommeil,sante"` ||
				string(payload.Data.Attributes["supportUrl"]) != `"https://example.com/support"` ||
				string(payload.Data.Attributes["whatsNew"]) != `"Premiere version"` {
				t.Fatalf("unexpected create attributes: %s", body)
			}
			if _, exists := payload.Data.Attributes["promotionalText"]; exists {
				t.Fatalf("create payload must omit cleared promotionalText, got %s", body)
			}
			creates++
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","description":"Nouvelle description","keywords":"sommeil,sante","supportUrl":"https://example.com/support","whatsNew":"Premiere version"}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	stdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--output", "json",
	})
	var result struct {
		Applied   bool `json:"applied"`
		Succeeded int  `json:"succeeded"`
		Failed    int  `json:"failed"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse apply output: %v\n%s", err, stdout)
	}
	if !result.Applied || result.Succeeded != 1 || result.Failed != 0 || creates != 1 {
		t.Fatalf("unexpected mixed create result: %+v creates=%d", result, creates)
	}
}
