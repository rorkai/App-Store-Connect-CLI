package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
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
