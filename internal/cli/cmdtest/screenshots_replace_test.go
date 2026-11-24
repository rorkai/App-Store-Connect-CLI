package cmdtest

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func installScreenshotReplacementOwnershipFixture(t *testing.T) {
	t.Helper()
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/appStoreVersionLocalizations/LOC_123":
			query := req.URL.Query()
			if req.Method != http.MethodGet || query.Get("include") != "appStoreVersion" ||
				query.Get("fields[appStoreVersionLocalizations]") != "appStoreVersion" || query.Get("fields[appStoreVersions]") != "app" {
				t.Fatalf("ownership must request only documented linkage: %s %s", req.Method, req.URL.String())
			}
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"LOC_123","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"VERSION_123"}}}}}`)
		case "/v1/appStoreVersions/VERSION_123":
			if req.Method != http.MethodGet || req.URL.Query().Get("include") != "app" {
				t.Fatalf("unexpected version ownership request: %s %s", req.Method, req.URL.String())
			}
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"100"}}}}}`)
		default:
			return original.RoundTrip(req)
		}
	})
}

func TestRunScreenshotsReplaceRetainsVerifiedFiles(t *testing.T) {
	for _, tc := range []struct {
		name, state, checksum   string
		foreignOrder            bool
		stale                   bool
		appID, appEnv           string
		missingApp, noConfirm   bool
		versionMode, noRequests bool
		wantError               string
		wantCode                int
	}{
		{name: "already matches", state: "COMPLETE"},
		{name: "partial name with localization", state: "COMPLETE", appID: "Demo", noRequests: true, wantCode: cmd.ExitUsage, wantError: "canonical numeric app ID"},
		{name: "bundle label with localization", state: "COMPLETE", appID: "com.synthetic.demo", noRequests: true, wantCode: cmd.ExitUsage, wantError: "canonical numeric app ID"},
		{name: "partial name with version", state: "COMPLETE", appID: "Demo", versionMode: true, noRequests: true, wantCode: cmd.ExitUsage, wantError: "canonical numeric app ID"},
		{name: "bundle label with version", state: "COMPLETE", appID: "com.synthetic.demo", versionMode: true, noRequests: true, wantCode: cmd.ExitUsage, wantError: "canonical numeric app ID"},
		{name: "noncanonical numeric app", state: "COMPLETE", appID: "0100", noRequests: true, wantCode: cmd.ExitUsage, wantError: "canonical numeric app ID"},
		{name: "partial name from environment", state: "COMPLETE", missingApp: true, appEnv: "Demo", noRequests: true, wantCode: cmd.ExitUsage, wantError: "canonical numeric app ID"},
		{name: "wrong selected app", state: "COMPLETE", appID: "200", wantCode: -1, wantError: "belongs to app"},
		{name: "explicit app overrides environment", state: "COMPLETE", appID: "100", appEnv: "200"},
		{name: "environment selects app", state: "COMPLETE", missingApp: true, appEnv: "100"},
		{name: "missing independent app", state: "COMPLETE", missingApp: true, wantCode: -1, wantError: "--replace requires --app"},
		{name: "missing deletion consent", state: "COMPLETE", noConfirm: true, wantCode: -1, wantError: "--confirm is required"},
		{name: "inventory changed before delete", state: "COMPLETE", stale: true, wantCode: cmd.ExitError, wantError: "changed before replacement deletion"},
		{name: "processing", state: "UPLOAD_COMPLETE", wantCode: cmd.ExitError},
		{name: "checksum unavailable", state: "COMPLETE", checksum: "missing", wantCode: cmd.ExitError},
		{name: "unverified membership", state: "COMPLETE", foreignOrder: true, wantCode: cmd.ExitError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			t.Setenv("ASC_APP_ID", tc.appEnv)
			dir := t.TempDir()
			t.Chdir(dir)
			file := writeCmdtestScreenshotPNG(t, dir, "01-home.png")
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			checksum := fmt.Sprintf("%x", md5.Sum(data))
			if tc.checksum == "missing" {
				checksum = ""
			}
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			writes, inventoryReads := 0, 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					writes++
					return screenshotsUploadJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"UNEXPECTED_WRITE"}]}`)
				}
				switch req.URL.Path {
				case "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
					return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
				case "/v1/appScreenshotSets/set-1/appScreenshots":
					inventoryReads++
					if tc.stale {
						checksum = strings.Repeat("a", 32)
						if inventoryReads > 1 {
							checksum = strings.Repeat("b", 32)
						}
					}
					return screenshotsUploadJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":"kept","attributes":{"fileName":"old-name.png","sourceFileChecksum":%q,"assetDeliveryState":{"state":%q}}}],"links":{}}`, checksum, tc.state))
				case "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
					id := "kept"
					if tc.foreignOrder {
						id = "foreign"
					}
					return screenshotsUploadJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":%q}],"links":{}}`, id))
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
			})
			installScreenshotReplacementOwnershipFixture(t)
			providerRequests := 0
			fixtureTransport := http.DefaultTransport
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				providerRequests++
				return fixtureTransport.RoundTrip(req)
			})
			code := -1
			args := []string{"screenshots", "upload", "--version-localization", "LOC_123", "--path", dir, "--device-type", "IPHONE_65", "--replace", "--output", "json"}
			if tc.versionMode {
				args = []string{"screenshots", "upload", "--version", "1.0", "--locale", "en-US", "--path", dir, "--device-type", "IPHONE_65", "--replace", "--output", "json"}
			}
			if !tc.missingApp {
				appID := tc.appID
				if appID == "" {
					appID = "100"
				}
				args = append(args, "--app", appID)
			}
			if !tc.noConfirm {
				args = append(args, "--confirm")
			}
			stdout, stderr := captureOutput(t, func() { code = cmd.Run(args, "test") })
			if (tc.wantCode == -1 && code == 0) || (tc.wantCode != -1 && code != tc.wantCode) || writes != 0 {
				t.Fatalf("exit=%d want=%d writes=%d stderr=%s", code, tc.wantCode, writes, stderr)
			}
			if tc.noRequests && providerRequests != 0 {
				t.Fatalf("invalid replacement selector made %d provider requests", providerRequests)
			}
			if tc.wantError != "" && !strings.Contains(stderr, tc.wantError) {
				t.Fatalf("stderr=%q, want %q", stderr, tc.wantError)
			}
			if code == 0 {
				var result asc.AppScreenshotUploadResult
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatal(err)
				}
				if result.Uploaded != 0 || result.Skipped != 1 || len(result.Results) != 1 || result.Results[0].AssetID != "kept" {
					t.Fatalf("unexpected receipt: %+v", result)
				}
			}
		})
	}
}

func TestRunScreenshotsReplaceRequiresFreshConfirmationAfterFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("ASC_APP_ID", "")
	dir := t.TempDir()
	t.Chdir(dir)
	checksums := map[string]string{"old-a": strings.Repeat("a", 32), "old-b": strings.Repeat("b", 32), "foreign": strings.Repeat("c", 32)}
	for index, name := range []string{"01-home.png", "02-profile.png"} {
		path := writeCmdtestScreenshotPNG(t, dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		checksums[fmt.Sprintf("kept-%d", index+1)] = fmt.Sprintf("%x", md5.Sum(data))
	}
	order := []string{"old-a", "kept-2", "old-b", "kept-1"}
	failDelete := true
	var writes []string
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			writes = append(writes, req.Method+" "+req.URL.Path)
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			data := make([]map[string]any, 0, len(order))
			for _, id := range order {
				data = append(data, map[string]any{"type": "appScreenshots", "id": id, "attributes": map[string]any{"fileName": id + ".png", "sourceFileChecksum": checksums[id], "assetDeliveryState": map[string]string{"state": "COMPLETE"}}})
			}
			body, err := json.Marshal(map[string]any{"data": data})
			if err != nil {
				return nil, err
			}
			return screenshotsUploadJSONResponse(http.StatusOK, string(body))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			data := make([]map[string]string, 0, len(order))
			for _, id := range order {
				data = append(data, map[string]string{"type": "appScreenshots", "id": id})
			}
			body, err := json.Marshal(map[string]any{"data": data})
			if err != nil {
				return nil, err
			}
			return screenshotsUploadJSONResponse(http.StatusOK, string(body))
		case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/old-"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			if id == "old-b" && failDelete {
				return screenshotsUploadJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"FIXTURE_FAILURE"}]}`)
			}
			order = slices.DeleteFunc(order, func(candidate string) bool { return candidate == id })
			return screenshotsUploadJSONResponse(http.StatusNoContent, "")
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			var body struct{ Data []struct{ ID string } }
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return nil, err
			}
			order = nil
			for _, item := range body.Data {
				order = append(order, item.ID)
			}
			return screenshotsUploadJSONResponse(http.StatusNoContent, "")
		default:
			return nil, fmt.Errorf("unexpected replacement request: %s %s", req.Method, req.URL.Path)
		}
	})
	installScreenshotReplacementOwnershipFixture(t)
	run := func(args ...string) (int, asc.AppScreenshotUploadResult, string) {
		t.Helper()
		if len(args) > 0 && args[0] == "--resume" {
			args = append(args, "--app", "100", "--version-localization", "LOC_123", "--device-type", "IPHONE_65")
		}
		code := -1
		stdout, stderr := captureOutput(t, func() { code = cmd.Run(append([]string{"screenshots", "upload"}, args...), "test") })
		var result asc.AppScreenshotUploadResult
		if stdout != "" {
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("receipt decode: %v: %s", err, stdout)
			}
		}
		return code, result, stderr
	}
	args := []string{"--app", "100", "--version-localization", "LOC_123", "--path", dir, "--device-type", "IPHONE_65", "--replace", "--confirm", "--output", "json"}
	code, failed, stderr := run(args...)
	if code == 0 || failed.FailureArtifactPath == "" || failed.Skipped != 2 || !reflect.DeepEqual(order, []string{"kept-2", "old-b", "kept-1"}) {
		t.Fatalf("partial deletion: code=%d result=%+v order=%v stderr=%s", code, failed, order, stderr)
	}
	if !slices.ContainsFunc(failed.Results, func(item asc.AssetUploadResultItem) bool {
		return item.State == "deleted" && item.AssetID == "old-a"
	}) || len(failed.Failures) != 1 || failed.Failures[0].FileName != "old-b.png" {
		t.Fatalf("receipt did not distinguish completed and failed deletions: %+v", failed)
	}
	originalRecord, err := os.ReadFile(failed.FailureArtifactPath)
	if err != nil {
		t.Fatal(err)
	}
	for field, value := range map[string]any{
		"setId":                 "another-set",
		"versionLocalizationId": "another-localization",
		"displayType":           "APP_IPAD_PRO_3GEN_129",
		"replacement":           map[string]any{"remove": []map[string]string{{"assetId": "kept-1", "checksum": strings.Repeat("a", 32)}}},
	} {
		var record map[string]any
		if err := json.Unmarshal(originalRecord, &record); err != nil {
			t.Fatal(err)
		}
		record[field] = value
		payload, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		editedPath := filepath.Join(dir, "edited-"+field+".json")
		if err := os.WriteFile(editedPath, payload, 0o600); err != nil {
			t.Fatal(err)
		}
		before := len(writes)
		code, _, stderr := run("--resume", editedPath, "--output", "json")
		if code == 0 || len(writes) != before {
			t.Fatalf("edited %s authorized writes: exit=%d writes=%v stderr=%s", field, code, writes[before:], stderr)
		}
	}
	failDelete = false
	order = append(order, "foreign")
	writeCount := len(writes)
	code, _, stderr = run("--resume", failed.FailureArtifactPath, "--output", "json")
	if code == 0 || len(writes) != writeCount || !strings.Contains(stderr, "receipts do not authorize screenshot deletion") {
		t.Fatalf("foreign screenshot: code=%d writes=%v stderr=%s", code, writes, stderr)
	}
	order = slices.DeleteFunc(order, func(id string) bool { return id == "foreign" })
	code, _, stderr = run("--resume", failed.FailureArtifactPath, "--output", "json")
	if code == 0 || len(writes) != writeCount || !strings.Contains(stderr, "receipts do not authorize screenshot deletion") {
		t.Fatalf("receipt deletion: code=%d writes=%v stderr=%s", code, writes, stderr)
	}
	preview := slices.Replace(slices.Clone(args), slices.Index(args, "--confirm"), slices.Index(args, "--confirm")+1, "--dry-run")
	code, _, stderr = run(preview...)
	if code != 0 || len(writes) != writeCount {
		t.Fatalf("fresh preview: code=%d writes=%v stderr=%s", code, writes, stderr)
	}
	code, _, stderr = run(args...)
	if code != 0 || !reflect.DeepEqual(order, []string{"kept-1", "kept-2"}) {
		t.Fatalf("fresh replacement: code=%d order=%v stderr=%s", code, order, stderr)
	}
	wantWrites := []string{"DELETE /v1/appScreenshots/old-a", "DELETE /v1/appScreenshots/old-b", "DELETE /v1/appScreenshots/old-b", "PATCH /v1/appScreenshotSets/set-1/relationships/appScreenshots"}
	if !reflect.DeepEqual(writes, wantWrites) {
		t.Fatalf("writes=%v want=%v", writes, wantWrites)
	}
	code, _, stderr = run(args...)
	if code != 0 || !reflect.DeepEqual(writes, wantWrites) {
		t.Fatalf("repeat: code=%d writes=%v stderr=%s", code, writes, stderr)
	}
}

func TestRunScreenshotsReplaceResumeReconcilesSurvivorsAndLegacyScope(t *testing.T) {
	for _, tc := range []struct {
		name, kind, state                                  string
		legacy, wrongSet, foreign, missingScope, orderOnly bool
		mismatch, missingChecksum                          bool
		wantError                                          string
	}{
		{name: "complete cleanup survivor", kind: "cleanupFailures"},
		{name: "complete pending survivor", kind: "pendingAssets"},
		{name: "cleanup checksum changed", kind: "cleanupFailures", mismatch: true, wantError: "source checksum does not match"},
		{name: "cleanup checksum missing", kind: "cleanupFailures", missingChecksum: true, wantError: "requires COMPLETE assets with source checksums"},
		{name: "cleanup still processing", kind: "cleanupFailures", state: "UPLOAD_COMPLETE", wantError: "still processing"},
		{name: "legacy complete cleanup", kind: "cleanupFailures", legacy: true},
		{name: "legacy failed cleanup", kind: "cleanupFailures", legacy: true, state: "FAILED", wantError: "receipts do not authorize screenshot deletion"},
		{name: "failed pending reservation", kind: "pendingAssets", state: "FAILED", wantError: "receipts do not authorize screenshot deletion"},
		{name: "legacy wrong set", kind: "cleanupFailures", legacy: true, wrongSet: true, wantError: "selected localization and device"},
		{name: "legacy new member", kind: "cleanupFailures", legacy: true, foreign: true, wantError: "live screenshot membership differs"},
		{name: "legacy missing scope", kind: "cleanupFailures", legacy: true, missingScope: true, wantError: "cannot verify"},
		{name: "legacy order only without source", legacy: true, orderOnly: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			t.Setenv("ASC_APP_ID", "")
			dir := t.TempDir()
			file := writeCmdtestScreenshotPNG(t, dir, "01-home.png")
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			checksum := fmt.Sprintf("%x", md5.Sum(data))
			remoteChecksum := checksum
			if tc.mismatch {
				remoteChecksum = strings.Repeat("a", 32)
			}
			if tc.missingChecksum {
				remoteChecksum = ""
			}
			state := tc.state
			if state == "" {
				state = "COMPLETE"
			}
			artifact := map[string]any{
				"replace": true, "setId": "set-1", "versionLocalizationId": "LOC_123",
				"displayType": "APP_IPHONE_65", "rootPath": dir, "files": []string{file},
				"pendingFiles": []string{file},
			}
			if !tc.legacy {
				artifact["replacement"] = map[string]any{"checksums": map[string]string{file: checksum}}
			}
			if tc.kind != "" {
				artifact[tc.kind] = []map[string]string{{"assetId": "kept", "filePath": file, "fileName": "01-home.png", "state": "COMPLETE"}}
			}
			if tc.missingScope {
				delete(artifact, "versionLocalizationId")
			}
			if tc.orderOnly {
				delete(artifact, "pendingFiles")
				artifact["orderedIds"] = []string{"kept"}
				artifact["results"] = []map[string]string{{"assetId": "kept", "filePath": file, "fileName": "01-home.png", "state": "COMPLETE"}}
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			}
			payload, err := json.Marshal(artifact)
			if err != nil {
				t.Fatal(err)
			}
			artifactPath := filepath.Join(dir, "resume.json")
			if err := os.WriteFile(artifactPath, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			var writes []string
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					writes = append(writes, req.Method+" "+req.URL.Path)
					if tc.orderOnly && req.Method == http.MethodPatch {
						return screenshotsUploadJSONResponse(http.StatusNoContent, "")
					}
					return screenshotsUploadJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"UNEXPECTED_WRITE"}]}`)
				}
				switch req.URL.Path {
				case "/v1/appScreenshots/kept":
					return screenshotsUploadJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"kept","attributes":{"sourceFileChecksum":%q,"assetDeliveryState":{"state":%q}}}}`, remoteChecksum, state))
				case "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
					id := "set-1"
					if tc.wrongSet {
						id = "other-set"
					}
					return screenshotsUploadJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshotSets","id":%q,"attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`, id))
				case "/v1/appScreenshotSets/set-1/appScreenshots":
					items := []map[string]any{{"type": "appScreenshots", "id": "kept", "attributes": map[string]any{"fileName": "01-home.png", "sourceFileChecksum": remoteChecksum, "assetDeliveryState": map[string]string{"state": state}}}}
					if tc.foreign {
						items = append(items, map[string]any{"type": "appScreenshots", "id": "foreign", "attributes": map[string]any{"sourceFileChecksum": strings.Repeat("a", 32), "assetDeliveryState": map[string]string{"state": "COMPLETE"}}})
					}
					body, err := json.Marshal(map[string]any{"data": items})
					if err != nil {
						return nil, err
					}
					return screenshotsUploadJSONResponse(http.StatusOK, string(body))
				case "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
					items := []map[string]string{{"type": "appScreenshots", "id": "kept"}}
					if tc.foreign {
						items = append(items, map[string]string{"type": "appScreenshots", "id": "foreign"})
					}
					body, err := json.Marshal(map[string]any{"data": items})
					if err != nil {
						return nil, err
					}
					return screenshotsUploadJSONResponse(http.StatusOK, string(body))
				default:
					return nil, fmt.Errorf("unexpected resume request: %s %s", req.Method, req.URL.Path)
				}
			})
			installScreenshotReplacementOwnershipFixture(t)
			code := -1
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run([]string{"screenshots", "upload", "--resume", artifactPath, "--app", "100", "--version-localization", "LOC_123", "--device-type", "IPHONE_65", "--output", "json"}, "test")
			})
			if tc.wantError != "" {
				if code == 0 || len(writes) != 0 || !strings.Contains(stderr, tc.wantError) {
					t.Fatalf("exit=%d writes=%v stderr=%s", code, writes, stderr)
				}
				return
			}
			if code != 0 || (!tc.orderOnly && len(writes) != 0) {
				t.Fatalf("exit=%d writes=%v stderr=%s", code, writes, stderr)
			}
			var result asc.AppScreenshotUploadResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			if !result.Resumed || len(result.Results) != 1 || result.Results[0].AssetID != "kept" || result.Failed != 0 {
				t.Fatalf("unexpected resume receipt: %+v", result)
			}
		})
	}
}
