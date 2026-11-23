package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

const stagedItemsDraftItemsJSON = `{"data":[` +
	`{"type":"reviewSubmissionItems","id":"item-v","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}},` +
	`{"type":"reviewSubmissionItems","id":"item-s","relationships":{"appStoreVersion":{"data":null},"subscriptionVersion":{"data":{"type":"subscriptionVersions","id":"sub-ver-1"}}}},` +
	`{"type":"reviewSubmissionItems","id":"item-g","relationships":{"appStoreVersion":{"data":null},"subscriptionGroupVersion":{"data":{"type":"subscriptionGroupVersions","id":"group-ver-1"}}}}` +
	`],"links":{"self":"/v1/reviewSubmissions/draft-1/items"}}`

// TestReviewSubmitNamesDraftItemsItDidNotName covers a draft that holds the
// version plus subscription items someone else staged. Review submit still
// submits the whole draft, but it must name each item the command line did not
// name on stderr, with how to remove it, and list every item in the receipt
// with named=false for the staged ones. --dry-run must do the same without
// writing; the registry fails on any request a row does not register.
func TestReviewSubmitNamesDraftItemsItDidNotName(t *testing.T) {
	const itemLines = "Not named: subscriptionVersions sub-ver-1 (review item item-s). To leave it out, remove it before submitting: asc review items remove --id \"item-s\" --confirm\n" +
		"Not named: subscriptionGroupVersions group-ver-1 (review item item-g). To leave it out, remove it before submitting: asc review items remove --id \"item-g\" --confirm\n"
	wantItems := []map[string]any{
		{"type": "appStoreVersions", "id": "version-1", "reviewSubmissionItemId": "item-v", "named": true},
		{"type": "subscriptionVersions", "id": "sub-ver-1", "reviewSubmissionItemId": "item-s", "named": false},
		{"type": "subscriptionGroupVersions", "id": "group-ver-1", "reviewSubmissionItemId": "item-g", "named": false},
	}

	tests := []struct {
		name       string
		args       []string
		register   func(*asctest.Registry)
		wantReuse  string
		wantHeader string
		wantSubmit string
	}{
		{
			name: "dry-run names the items and writes nothing",
			args: []string{"--dry-run"},
			register: func(reg *asctest.Registry) {
				reg.Register(asctest.GET("/v1/appStoreVersions/version-1/build"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
			},
			wantReuse:  "Would reuse review submission draft-1 because the target version is already attached.\n",
			wantHeader: "Review submission draft-1 holds 2 items this command did not name; submitting it would send them to App Review with version version-1.\n",
		},
		{
			name: "confirm names the items before submitting them",
			args: []string{"--confirm"},
			register: func(reg *asctest.Registry) {
				reg.Register(asctest.GET("/v1/appStoreVersions/version-1/build"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
				reg.Register(asctest.PATCH("/v1/appStoreVersions/version-1/relationships/build").JSONBody(`{"data":{"type":"builds","id":"build-1"}}`), asctest.JSON(http.StatusNoContent, ""))
				reg.Register(asctest.GET("/v1/reviewSubmissions/draft-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"draft-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
				reg.Register(asctest.GET("/v1/reviewSubmissions/draft-1/items", "fields[reviewSubmissionItems]=appStoreVersion,subscriptionGroupVersion,subscriptionVersion", "include=appStoreVersion,subscriptionGroupVersion,subscriptionVersion", "limit=200"), asctest.JSON(http.StatusOK, stagedItemsDraftItemsJSON))
				reg.Register(asctest.PATCH("/v1/reviewSubmissions/draft-1").JSONBody(`{"data":{"type":"reviewSubmissions","id":"draft-1","attributes":{"submitted":true}}}`), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"draft-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-04-08T00:00:00Z"}}}`))
			},
			wantReuse:  "Reusing existing review submission draft-1 because the target version is already attached.\n",
			wantHeader: "Review submission draft-1 holds 2 items this command did not name; submitting it sends them to App Review with version version-1.\n",
			wantSubmit: "draft-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`))
			reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionSubmission"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
			reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support"}}]}`))
			reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
			reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
			reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[]}`))
			registerSubmitPreflightAppInfos(reg, completeAgeRatingAttributes)
			reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "filter[platform]=IOS", "filter[state]=UNRESOLVED_ISSUES", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`))
			reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "filter[platform]=IOS", "filter[state]=READY_FOR_REVIEW", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"draft-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`))
			reg.Register(asctest.GET("/v1/reviewSubmissions/draft-1/items", "fields[reviewSubmissionItems]=appStoreVersion,subscriptionGroupVersion,subscriptionVersion", "include=appStoreVersion,subscriptionGroupVersion,subscriptionVersion", "limit=200"), asctest.JSON(http.StatusOK, stagedItemsDraftItemsJSON))
			if tt.register != nil {
				tt.register(reg)
			}

			args := append([]string{"review", "submit", "--app", "app-1", "--version", "1.2.3", "--build-id", "build-1", "--output", "json"}, tt.args...)
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), args...)
			if err != nil {
				t.Fatalf("run error: %v\nstderr=%s", err, stderr)
			}
			if !strings.Contains(stderr, tt.wantReuse) {
				t.Fatalf("stderr = %q, want %q", stderr, tt.wantReuse)
			}
			if !strings.Contains(stderr, tt.wantHeader+itemLines) {
				t.Fatalf("stderr = %q, want %q", stderr, tt.wantHeader+itemLines)
			}
			var payload struct {
				SubmissionID string           `json:"submissionId"`
				WouldSubmit  bool             `json:"wouldSubmit"`
				ReviewItems  []map[string]any `json:"reviewItems"`
			}
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
			}
			if payload.SubmissionID != tt.wantSubmit || payload.WouldSubmit != (tt.wantSubmit == "") {
				t.Fatalf("submissionId=%q wouldSubmit=%t, want %q / %t", payload.SubmissionID, payload.WouldSubmit, tt.wantSubmit, tt.wantSubmit == "")
			}
			gotItems, _ := json.Marshal(payload.ReviewItems)
			wantJSON, _ := json.Marshal(wantItems)
			if string(gotItems) != string(wantJSON) {
				t.Fatalf("reviewItems = %s, want %s", gotItems, wantJSON)
			}
		})
	}
}

// TestPublishAppStoreSubmitNamesDraftItemsItDidNotName covers the same draft
// through `publish appstore --submit`: it still submits the draft, names the
// staged items on stderr before submitting, and lists every item in the
// receipt.
func TestPublishAppStoreSubmitNamesDraftItemsItDidNotName(t *testing.T) {
	const notice = "Review submission draft-1 holds 2 items this command did not name; submitting it sends them to App Review with version version-1.\n" +
		"Not named: subscriptionVersions sub-ver-1 (review item item-s). To leave it out, remove it before submitting: asc review items remove --id \"item-s\" --confirm\n" +
		"Not named: subscriptionGroupVersions group-ver-1 (review item item-g). To leave it out, remove it before submitting: asc review items remove --id \"item-g\" --confirm\n"
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	var writes []string
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if resp, err, ok := respondToPublishAppLookup(t, req); ok {
			return resp, err
		}
		query := req.URL.Query()
		route := req.Method + " " + req.URL.Path
		switch {
		case route == "POST /v1/buildUploads":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"}}}`)
		case route == "POST /v1/buildUploadFiles":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		case route == "PATCH /v1/buildUploadFiles/file-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		case route == "GET /v1/buildUploads/upload-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"},"relationships":{"build":{"data":{"type":"builds","id":"build-42"}}}}}`)
		case route == "GET /v1/builds/build-42":
			return jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}}`)
		case route == "GET /v1/apps/app-1/appStoreVersions" && query.Get("filter[versionString]") == "1.2.3":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
		case route == "GET /v1/apps/app-1/appStoreVersions" && isReleasedVersionStateQuery(query):
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case route == "GET /v1/appStoreVersions/version-1/appStoreVersionSubmission", route == "GET /v1/appStoreVersions/version-1/build":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case route == "GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)
		case route == "GET /v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, submitPreflightAppInfosJSON("app-1", completeAgeRatingAttributes))
		case route == "GET /v1/apps/app-1/subscriptionGroups":
			return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case route == "GET /v1/apps/app-1/reviewSubmissions" && query.Get("filter[state]") == "UNRESOLVED_ISSUES":
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
		case route == "GET /v1/apps/app-1/reviewSubmissions" && query.Get("filter[state]") == "READY_FOR_REVIEW":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"draft-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
		case route == "GET /v1/reviewSubmissions/draft-1/items":
			return jsonResponse(http.StatusOK, stagedItemsDraftItemsJSON)
		case route == "GET /v1/reviewSubmissions/draft-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"draft-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
		case route == "PATCH /v1/appStoreVersions/version-1/relationships/build":
			writes = append(writes, route)
			return jsonResponse(http.StatusNoContent, "")
		case route == "PATCH /v1/reviewSubmissions/draft-1":
			writes = append(writes, route)
			return jsonResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"draft-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-15T00:00:00Z"}}}`)
		default:
			t.Errorf("unexpected request: %s?%s", route, req.URL.RawQuery)
			return jsonResponse(http.StatusInternalServerError, `{"errors":[{"status":"500"}]}`)
		}
	})
	installDefaultTransport(t, rt)

	args := []string{"publish", "appstore", "--app", "app-1", "--ipa", ipaPath, "--version", "1.2.3", "--build-number", "42", "--submit", "--confirm", "--timeout", "1s", "--poll-interval", "1ms", "--output", "json"}
	var stdout, stderr string
	var err error
	synctest.Test(t, func(t *testing.T) {
		stdout, stderr, err = runRootWithDeps(t, stubClient(t, rt), args...)
	})
	if err != nil {
		t.Fatalf("run error: %v\nstderr=%s", err, stderr)
	}
	if !strings.Contains(stderr, notice) {
		t.Fatalf("stderr = %q, want %q", stderr, notice)
	}
	if want := []string{"PATCH /v1/appStoreVersions/version-1/relationships/build", "PATCH /v1/reviewSubmissions/draft-1"}; strings.Join(writes, ",") != strings.Join(want, ",") {
		t.Fatalf("writes = %v, want %v", writes, want)
	}
	var payload struct {
		ReviewItems []map[string]any `json:"reviewItems"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	got, _ := json.Marshal(payload.ReviewItems)
	want := `[{"id":"version-1","named":true,"reviewSubmissionItemId":"item-v","type":"appStoreVersions"},{"id":"sub-ver-1","named":false,"reviewSubmissionItemId":"item-s","type":"subscriptionVersions"},{"id":"group-ver-1","named":false,"reviewSubmissionItemId":"item-g","type":"subscriptionGroupVersions"}]`
	if string(got) != want {
		t.Fatalf("reviewItems = %s, want %s", got, want)
	}
}

// TestReviewSubmitDryRunFailsWhenDraftLookupFails pins that --dry-run now reads
// the ready-for-review drafts the real run would reuse, so a failed read fails
// the dry run (exit non-zero) instead of previewing a submit the real run would
// not make. The registry has no write stubs, so it also proves nothing was
// written.
func TestReviewSubmitDryRunFailsWhenDraftLookupFails(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionSubmission"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support"}}]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	registerSubmitPreflightAppInfos(reg, completeAgeRatingAttributes)
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "filter[platform]=IOS", "filter[state]=UNRESOLVED_ISSUES", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "filter[platform]=IOS", "filter[state]=READY_FOR_REVIEW", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"Forbidden"}]}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "submit", "--app", "app-1", "--version", "1.2.3", "--build-id", "build-1", "--dry-run", "--output", "json")
	if err == nil {
		t.Fatalf("expected the dry run to fail when the draft lookup fails\nstdout=%s\nstderr=%s", stdout, stderr)
	}
	if !strings.Contains(err.Error(), "prepare review submission") {
		t.Fatalf("err = %v, want the draft lookup failure", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want no receipt", stdout)
	}
}
