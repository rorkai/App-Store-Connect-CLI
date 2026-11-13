package publish

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestLoadPublishVersionMetadataValuesReadsOnlyVersionLocalizationFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("create app-info dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"Ignored App Name"}`), 0o600); err != nil {
		t.Fatalf("write app-info fixture: %v", err)
	}

	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("create version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"description":"Updated description","whatsNew":"Bug fixes"}`), 0o600); err != nil {
		t.Fatalf("write en-US fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "fr-FR.json"), []byte(`{"keywords":"one,two","marketingUrl":"https://example.com/fr"}`), 0o600); err != nil {
		t.Fatalf("write fr-FR fixture: %v", err)
	}

	values, err := loadPublishVersionMetadataValues(dir, "1.2.3")
	if err != nil {
		t.Fatalf("loadPublishVersionMetadataValues() error: %v", err)
	}

	if len(values) != 2 {
		t.Fatalf("expected 2 locales, got %d: %+v", len(values), values)
	}
	if got := values["en-US"]["description"]; got != "Updated description" {
		t.Fatalf("expected en-US description, got %q", got)
	}
	if got := values["en-US"]["whatsNew"]; got != "Bug fixes" {
		t.Fatalf("expected en-US whatsNew, got %q", got)
	}
	if _, ok := values["en-US"]["keywords"]; ok {
		t.Fatalf("did not expect omitted en-US keywords to be populated: %+v", values["en-US"])
	}
	if got := values["fr-FR"]["keywords"]; got != "one,two" {
		t.Fatalf("expected fr-FR keywords, got %q", got)
	}
	if got := values["fr-FR"]["marketingUrl"]; got != "https://example.com/fr" {
		t.Fatalf("expected fr-FR marketingUrl, got %q", got)
	}
}

func TestApplyPublishVersionMetadataRetainsPartialResultsAndArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("ASC_MAX_RETRIES", "0")
	patches := 0
	swapTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return publishJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"en-id","attributes":{"locale":"en-US","description":"Old English"}},{"type":"appStoreVersionLocalizations","id":"fr-id","attributes":{"locale":"fr-FR","description":"Old French"}}],"links":{"next":""}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionLocalizations/en-id":
			patches++
			return publishJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"en-id","attributes":{"locale":"en-US","description":"New English","whatsNew":"English notes"}}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionLocalizations/fr-id":
			patches++
			return publishJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"ENTITY_ERROR","detail":"French rejected"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))
	client := newPublishCommandTestClient(t)

	results, err := applyPublishVersionMetadata(context.Background(), client, publishVersionMetadataOptions{
		VersionID: "version-1",
		Dir:       "metadata",
		ValuesByLocale: map[string]map[string]string{
			"en-US": {"description": "New English", "whatsNew": "English notes"},
			"fr-FR": {"description": "New French", "whatsNew": "French notes"},
		},
	})
	if err == nil || patches != 2 || len(results) != 2 {
		t.Fatalf("expected visible partial publish result, results=%+v patches=%d err=%v", results, patches, err)
	}
	if !strings.Contains(err.Error(), "retry artifact: .asc/reports/localizations-upload/") {
		t.Fatalf("expected retry artifact in publish error: %v", err)
	}
	artifacts, globErr := filepath.Glob(filepath.Join(".asc", "reports", "localizations-upload", "failures-*.json"))
	if globErr != nil || len(artifacts) != 1 {
		t.Fatalf("expected one publish artifact, paths=%v err=%v", artifacts, globErr)
	}
	payload, readErr := os.ReadFile(artifacts[0])
	if readErr != nil || !strings.Contains(string(payload), `"description": "New French"`) {
		t.Fatalf("publish artifact is not resumable: payload=%s err=%v", payload, readErr)
	}
	var apiErr *asc.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected underlying API failure to remain inspectable: %T %v", err, err)
	}
}

func TestApplyPublishVersionMetadataPreservesZeroResultInputError(t *testing.T) {
	workDir := t.TempDir()
	t.Chdir(workDir)
	swapTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return publishJSONResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
	}))
	client := newPublishCommandTestClient(t)

	results, err := applyPublishVersionMetadata(context.Background(), client, publishVersionMetadataOptions{
		VersionID: "version-1",
		Dir:       "metadata",
		ValuesByLocale: map[string]map[string]string{
			"en-US": {"promotionalText": ""},
		},
	})
	if err == nil || !shared.IsLocalizationInputError(err) {
		t.Fatalf("expected localization input error, got %T: %v", err, err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no mutation results, got %+v", results)
	}
	if _, statErr := os.Stat(filepath.Join(workDir, ".asc")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected no bogus retry artifact, stat error=%v", statErr)
	}
}

func TestApplyPublishVersionMetadataPreservesZeroResultAPIError(t *testing.T) {
	workDir := t.TempDir()
	t.Chdir(workDir)
	t.Setenv("ASC_MAX_RETRIES", "0")
	swapTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return publishJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","detail":"snapshot conflict"}]}`), nil
	}))
	client := newPublishCommandTestClient(t)

	results, err := applyPublishVersionMetadata(context.Background(), client, publishVersionMetadataOptions{
		VersionID: "version-1",
		Dir:       "metadata",
		ValuesByLocale: map[string]map[string]string{
			"en-US": {"description": "Desired"},
		},
	})
	if len(results) != 0 || err == nil {
		t.Fatalf("expected zero-result API failure, results=%+v err=%v", results, err)
	}
	var apiErr *asc.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusConflict {
		t.Fatalf("expected typed conflict, got %T: %v", err, err)
	}
	if strings.Contains(err.Error(), "0 locale(s) failed") {
		t.Fatalf("unexpected zero-failure summary: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(workDir, ".asc")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected no retry artifact, stat error=%v", statErr)
	}
}

func TestApplyPublishVersionMetadataUsesFreshDeadlinePerLocale(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "100ms")
	t.Setenv("ASC_MAX_RETRIES", "0")
	patches := 0
	swapTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return publishJSONResponse(http.StatusOK, `{"data":[
				{"type":"appStoreVersionLocalizations","id":"en-id","attributes":{"locale":"en-US","description":"Old English"}},
				{"type":"appStoreVersionLocalizations","id":"fr-id","attributes":{"locale":"fr-FR","description":"Old French"}}
			],"links":{"next":""}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionLocalizations/en-id":
			patches++
			time.Sleep(60 * time.Millisecond)
			return publishJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"en-id","attributes":{"description":"New English"}}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionLocalizations/fr-id":
			patches++
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) < 70*time.Millisecond {
				t.Fatalf("expected fresh second-locale deadline, remaining=%s", time.Until(deadline))
			}
			return publishJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"fr-id","attributes":{"description":"New French"}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	}))
	client := newPublishCommandTestClient(t)

	results, err := applyPublishVersionMetadata(context.Background(), client, publishVersionMetadataOptions{
		VersionID: "version-1",
		ValuesByLocale: map[string]map[string]string{
			"en-US": {"description": "New English"},
			"fr-FR": {"description": "New French"},
		},
	})
	if err != nil || patches != 2 || len(results) != 2 {
		t.Fatalf("unexpected metadata batch: results=%+v patches=%d err=%v", results, patches, err)
	}
}

func publishJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func TestLoadPublishVersionMetadataValuesRequiresVersionFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("create version dir: %v", err)
	}

	_, err := loadPublishVersionMetadataValues(dir, "1.2.3")
	if err == nil {
		t.Fatal("expected missing version metadata JSON files to fail")
	}
	if !strings.Contains(err.Error(), "no version metadata JSON files found") {
		t.Fatalf("expected missing version metadata files error, got %v", err)
	}
}
