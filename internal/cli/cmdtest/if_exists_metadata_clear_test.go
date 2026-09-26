package cmdtest

import (
	"net/http"
	"strings"
	"testing"
)

// metadataPushClearConflictHandler replays a version localization create for
// "ja" that fails with the recorded duplicate-locale 409. The locale is absent
// from the plan read; afterwards it exists with remoteDescription and a
// non-empty promotionalText, until a PATCH lands and the clear is visible.
func metadataPushClearConflictHandler(t *testing.T, remoteDescription string, patchBody *string) func(ifExistsRequest) (*http.Response, error) {
	t.Helper()
	posts := 0
	patched := false
	return func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, metadataPushVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, metadataPushAppInfosList)
		case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			switch {
			case posts == 0:
				return jsonResponse(http.StatusOK, metadataPushEmptyList)
			case patched:
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Planned JA description"}}],"links":{"next":""}}`)
			default:
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"`+remoteDescription+`","promotionalText":"Old JA promo"}}],"links":{"next":""}}`)
			}
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
			posts++
			return jsonResponse(http.StatusConflict, metadataVersionLocaleDuplicate409)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersionLocalizations/loc-ja":
			patched = true
			if patchBody != nil {
				*patchBody = req.Body
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Planned JA description"}}}`)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	}
}

func TestMetadataPushIfExistsUpdateCarriesExplicitClearsToExistingLocale(t *testing.T) {
	for _, tc := range []struct {
		name              string
		remoteDescription string
	}{
		// The create read-back only compares the created fields, so a remote
		// that already carries the set values must not hide a pending clear.
		{name: "set values already match", remoteDescription: "Planned JA description"},
		{name: "set values differ", remoteDescription: "Remote JA description"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description","promotionalText":null}`)
			patchBody := ""
			stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
				"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
				"--if-exists", "update", "--confirm", "--output", "json",
			}, metadataPushClearConflictHandler(t, tc.remoteDescription, &patchBody))

			if runErr != nil {
				t.Fatalf("expected exit 0, got %v (stderr %q)", runErr, stderr)
			}
			if countRequests(seen, http.MethodPatch, "/v1/appStoreVersionLocalizations/loc-ja") != 1 {
				t.Fatalf("want one PATCH carrying the clear, requests=%+v", seen)
			}
			if !strings.Contains(patchBody, `"promotionalText":null`) {
				t.Fatalf("PATCH body = %q, want the explicit promotionalText clear", patchBody)
			}
			if !strings.Contains(patchBody, `"description":"Planned JA description"`) {
				t.Fatalf("PATCH body = %q, want the planned description", patchBody)
			}
			actions := metadataPushActions(t, stdout)
			if len(actions) != 1 || actions[0]["action"] != "update" || actions[0]["status"] != "succeeded" || actions[0]["alreadyExists"] != true {
				t.Fatalf("actions = %v, want one successful update of the existing locale", actions)
			}
		})
	}
}

func TestMetadataPushIfExistsUpdateRequiresConfirmForPlannedCreateClears(t *testing.T) {
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description","promotionalText":null}`)
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "update", "--output", "json",
	}, metadataPushClearConflictHandler(t, "Remote JA description", nil))

	if runErr == nil || !strings.Contains(runErr.Error(), "--confirm is required") {
		t.Fatalf("expected a usage error requiring --confirm, got %v (stderr %q)", runErr, stderr)
	}
	if countRequests(seen, http.MethodPost, "/v1/appStoreVersionLocalizations") != 0 || countRequests(seen, http.MethodPatch, "/v1/appStoreVersionLocalizations/loc-ja") != 0 {
		t.Fatalf("no mutation may run before the --confirm check, requests=%+v", seen)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
}

func TestMetadataPushIfExistsSkipDoesNotRequireConfirmForPlannedCreateClears(t *testing.T) {
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description","promotionalText":null}`)
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "skip", "--output", "json",
	}, metadataPushClearConflictHandler(t, "Remote JA description", nil))

	if runErr != nil {
		t.Fatalf("expected exit 0 with --if-exists skip, got %v (stderr %q)", runErr, stderr)
	}
	if countRequests(seen, http.MethodPatch, "/v1/appStoreVersionLocalizations/loc-ja") != 0 {
		t.Fatalf("--if-exists skip must not PATCH, requests=%+v", seen)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 1 || actions[0]["status"] != "skipped" {
		t.Fatalf("actions = %v, want one skipped action", actions)
	}
}

func TestMetadataPushIfExistsUpdateCarriesClearOnlyAppInfoFileToLateLocale(t *testing.T) {
	dir := writeMetadataAppInfoFixture(t, `{"subtitle":null}`)
	localizationReads := 0
	patchBody := ""
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "update", "--confirm", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, metadataPushVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, metadataPushAppInfosList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			localizationReads++
			switch {
			case localizationReads == 1:
				return jsonResponse(http.StatusOK, metadataPushEmptyList)
			case patchBody != "":
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-ja","attributes":{"locale":"ja","name":"Remote JA name"}}],"links":{"next":""}}`)
			default:
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-ja","attributes":{"locale":"ja","name":"Remote JA name","subtitle":"Old JA subtitle"}}],"links":{"next":""}}`)
			}
		case req.Method == http.MethodPatch && req.Path == "/v1/appInfoLocalizations/loc-ja":
			patchBody = req.Body
			return jsonResponse(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-ja","attributes":{"locale":"ja","name":"Remote JA name"}}}`)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})

	if runErr != nil {
		t.Fatalf("expected exit 0, got %v (stderr %q)", runErr, stderr)
	}
	if !strings.Contains(patchBody, `"subtitle":null`) {
		t.Fatalf("PATCH body = %q, want the explicit subtitle clear (requests=%+v)", patchBody, seen)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 1 || actions[0]["scope"] != "app-info" || actions[0]["action"] != "update" || actions[0]["status"] != "succeeded" {
		t.Fatalf("actions = %v, want one successful app-info update", actions)
	}
}
