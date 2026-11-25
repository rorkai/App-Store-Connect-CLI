package cmdtest

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func writeMetadataNewLocaleFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		filepath.Join("app-info", "ja.json"):         `{"name":"Planned JA name"}`,
		filepath.Join("version", "1.2.3", "ja.json"): `{"description":"Planned JA description"}`,
	}
	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, path), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return dir
}

// Creating an app-info localization makes App Store Connect create an empty
// version localization for the same locale, so the version family must update
// it instead of replaying the planned create into a duplicate-locale 409.
func TestMetadataPushNewLocaleUpdatesVersionLocalizationCreatedByAppInfoCreate(t *testing.T) {
	dir := writeMetadataNewLocaleFixture(t)

	appInfoCreated := false
	versionPatched := false
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "123456789", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/apps/123456789/appStoreVersions":
			return jsonResponse(http.StatusOK, metadataPushVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/123456789/appInfos":
			return jsonResponse(http.StatusOK, metadataPushAppInfosList)
		case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			if appInfoCreated {
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"info-ja","attributes":{"locale":"ja","name":"Planned JA name"}}],"links":{"next":""}}`)
			}
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodPost && req.Path == "/v1/appInfoLocalizations":
			appInfoCreated = true
			return jsonResponse(http.StatusCreated, `{"data":{"type":"appInfoLocalizations","id":"info-ja","attributes":{"locale":"ja","name":"Planned JA name"}}}`)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			switch {
			case versionPatched:
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Planned JA description"}}],"links":{"next":""}}`)
			case appInfoCreated:
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja"}}],"links":{"next":""}}`)
			default:
				return jsonResponse(http.StatusOK, metadataPushEmptyList)
			}
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusConflict, metadataVersionLocaleDuplicate409)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersionLocalizations/loc-ja":
			versionPatched = true
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Planned JA description"}}}`)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})

	if runErr != nil {
		t.Fatalf("expected success, got %v (stderr %q)", runErr, stderr)
	}
	if got := countRequests(seen, http.MethodPost, "/v1/appStoreVersionLocalizations"); got != 0 {
		t.Fatalf("version localization POSTs = %d, want 0: %v", got, seen)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 2 {
		t.Fatalf("actions = %v, want two", actions)
	}
	if actions[0]["scope"] != "app-info" || actions[0]["action"] != "create" || actions[0]["status"] != "succeeded" {
		t.Fatalf("app-info action = %v, want a succeeded create", actions[0])
	}
	if actions[1]["scope"] != "version" || actions[1]["action"] != "update" || actions[1]["status"] != "succeeded" || actions[1]["localizationId"] != "loc-ja" {
		t.Fatalf("version action = %v, want a succeeded update of loc-ja", actions[1])
	}
	apiCalls, err := json.Marshal(metadataPushResult(t, stdout)["apiCalls"])
	if err != nil {
		t.Fatalf("marshal apiCalls: %v", err)
	}
	wantCalls := `[{"count":1,"operation":"create_localization","scope":"app-info"},{"count":1,"operation":"list_localizations","scope":"version"},{"count":1,"operation":"update_localization","scope":"version"}]`
	if string(apiCalls) != wantCalls {
		t.Fatalf("apiCalls = %s, want %s", apiCalls, wantCalls)
	}
}

// An app-info locale that another writer created between planning and apply
// reconciles as a duplicate; the version create for it must still fail.
func TestMetadataPushNewLocaleKeepsVersionConflictWhenAppInfoReconciledDuplicate(t *testing.T) {
	dir := writeMetadataNewLocaleFixture(t)

	appInfoPosted := false
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "123456789", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/apps/123456789/appStoreVersions":
			return jsonResponse(http.StatusOK, metadataPushVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/123456789/appInfos":
			return jsonResponse(http.StatusOK, metadataPushAppInfosList)
		case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			if appInfoPosted {
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"info-ja","attributes":{"locale":"ja","name":"Planned JA name"}}],"links":{"next":""}}`)
			}
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodPost && req.Path == "/v1/appInfoLocalizations":
			appInfoPosted = true
			return jsonResponse(http.StatusConflict, metadataAppInfoLocaleDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			if appInfoPosted {
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja"}}],"links":{"next":""}}`)
			}
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusConflict, metadataVersionLocaleDuplicate409)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})

	if runErr == nil {
		t.Fatalf("expected the version create conflict to fail the push (stderr %q)", stderr)
	}
	if got := countRequests(seen, http.MethodPatch, "/v1/appStoreVersionLocalizations/loc-ja"); got != 0 {
		t.Fatalf("version localization PATCHes = %d, want 0: %v", got, seen)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 2 || actions[1]["scope"] != "version" || actions[1]["action"] != "create" || actions[1]["status"] != "failed" {
		t.Fatalf("actions = %v, want a failed version create", actions)
	}
}
