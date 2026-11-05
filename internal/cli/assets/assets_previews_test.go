package assets

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAssetsPreviewsUploadCommandRejectsSkipExistingWithReplace(t *testing.T) {
	cmd := AssetsPreviewsUploadCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--version-localization", "LOC_ID",
		"--path", "./previews",
		"--device-type", "IPHONE_65",
		"--skip-existing",
		"--replace",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), cmd.FlagSet.Args())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--skip-existing and --replace are mutually exclusive") {
		t.Fatalf("expected mutually exclusive error in stderr, got %q", stderr)
	}
}

func TestAssetsPreviewsUploadCommandRejectsUnsupportedFileBeforeAuth(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a-preview.mov", "b-poster.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("not-empty"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	clientCalled := false
	cmd := assetsPreviewsUploadCommandWithDependencies(previewUploadDependencies{
		GetClient: func() (*asc.Client, error) {
			clientCalled = true
			return &asc.Client{}, nil
		},
	})
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--version-localization", "LOC_ID",
		"--path", dir,
		"--device-type", "IPHONE_65",
		"--replace",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, _ := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), cmd.FlagSet.Args())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if runErr == nil {
		t.Fatal("expected unsupported preview file error")
	}
	if !strings.Contains(runErr.Error(), `unsupported preview file extension ".png"`) {
		t.Fatalf("expected local file-type error before auth, got %v", runErr)
	}
	if clientCalled {
		t.Fatal("expected preview file validation before auth/client creation")
	}
}

func TestAssetsPreviewsUploadCommandRejectsMoreThanThreeFilesBeforeAuth(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 4; i++ {
		name := fmt.Sprintf("preview-%d.mov", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	clientCalled := false
	cmd := assetsPreviewsUploadCommandWithDependencies(previewUploadDependencies{
		GetClient: func() (*asc.Client, error) {
			clientCalled = true
			return &asc.Client{}, nil
		},
	})
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--version-localization", "LOC_ID",
		"--path", dir,
		"--device-type", "IPHONE_65",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, _ := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), cmd.FlagSet.Args())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if runErr == nil {
		t.Fatal("expected preview capacity error")
	}
	if !strings.Contains(runErr.Error(), "at most 3 files") {
		t.Fatalf("expected preview capacity error before auth, got %v", runErr)
	}
	if clientCalled {
		t.Fatal("expected preview capacity validation before auth/client creation")
	}
}

func TestDetectPreviewMimeType(t *testing.T) {
	tests := []struct {
		path    string
		want    string
		wantErr string
	}{
		{path: "preview.mov", want: "video/quicktime"},
		{path: "preview.m4v", want: "video/x-m4v"},
		{path: "preview.MP4", want: "video/mp4"},
		{path: "poster.png", wantErr: `unsupported preview file extension ".png"`},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, err := detectPreviewMimeType(tt.path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("detectPreviewMimeType(%q) error = %v, want %q", tt.path, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("detectPreviewMimeType(%q) error: %v", tt.path, err)
			}
			if got != tt.want {
				t.Fatalf("detectPreviewMimeType(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestUploadPreviewsRejectsUnsupportedFileBeforeRequests(t *testing.T) {
	tests := []struct {
		name    string
		replace bool
		dryRun  bool
	}{
		{name: "replace", replace: true},
		{name: "dry run", dryRun: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(t.TempDir(), "poster.png")
			if err := os.WriteFile(filePath, []byte("not-empty"), 0o600); err != nil {
				t.Fatalf("write invalid preview: %v", err)
			}

			requests := 0
			deleteRequests := 0
			client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method == http.MethodDelete {
					deleteRequests++
				}
				writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected request"}]}`)
			}))

			_, err := uploadPreviews(
				context.Background(),
				client,
				"LOC_ID",
				"IPHONE_65",
				[]string{filePath},
				false,
				tt.replace,
				tt.dryRun,
				"",
			)
			if err == nil {
				t.Fatal("expected unsupported preview file error")
			}
			if !strings.Contains(err.Error(), `unsupported preview file extension ".png"`) {
				t.Fatalf("unexpected error: %v", err)
			}
			if requests != 0 {
				t.Fatalf("requests = %d, want 0", requests)
			}
			if deleteRequests != 0 {
				t.Fatalf("DELETE requests = %d, want 0", deleteRequests)
			}
		})
	}
}

func TestUploadPreviewsSkipExistingSyncsSortedOrder(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "01-first.mov")
	second := filepath.Join(dir, "02-second.mov")
	if err := os.WriteFile(first, []byte("first-preview"), 0o600); err != nil {
		t.Fatalf("write first preview: %v", err)
	}
	if err := os.WriteFile(second, []byte("second-preview"), 0o600); err != nil {
		t.Fatalf("write second preview: %v", err)
	}
	firstChecksum, err := computeFileChecksum(first)
	if err != nil {
		t.Fatalf("checksum first preview: %v", err)
	}
	secondChecksum, err := computeFileChecksum(second)
	if err != nil {
		t.Fatalf("checksum second preview: %v", err)
	}

	patched := make([][]string, 0, 1)
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/appStoreVersionLocalizations/LOC_ID/appPreviewSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}],"links":{}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/appPreviewSets/set-1/appPreviews":
			writeAssetsTestJSON(w, http.StatusOK, fmt.Sprintf(
				`{"data":[{"type":"appPreviews","id":"preview-second","attributes":{"fileName":"02-second.mov","sourceFileChecksum":"%s"}},{"type":"appPreviews","id":"preview-first","attributes":{"fileName":"01-first.mov","sourceFileChecksum":"%s"}}],"links":{}}`,
				secondChecksum, firstChecksum,
			))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/appPreviewSets/set-1/relationships/appPreviews":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appPreviews","id":"preview-second"},{"type":"appPreviews","id":"preview-first"}],"links":{}}`)
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/appPreviewSets/set-1/relationships/appPreviews":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read relationship body: %v", err)
			}
			var payload asc.RelationshipRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode relationship body: %v", err)
			}
			ids := make([]string, 0, len(payload.Data))
			for _, item := range payload.Data {
				ids = append(ids, item.ID)
			}
			patched = append(patched, ids)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected request"}]}`)
		}
	}))

	result, err := uploadPreviews(
		context.Background(),
		client,
		"LOC_ID",
		"IPHONE_65",
		[]string{first, second},
		true,
		false,
		false,
		"",
	)
	if err != nil {
		t.Fatalf("uploadPreviews() error: %v", err)
	}
	if len(patched) != 1 {
		t.Fatalf("expected exactly one preview order PATCH, got %d (%v)", len(patched), patched)
	}
	want := []string{"preview-first", "preview-second"}
	if !reflect.DeepEqual(patched[0], want) {
		t.Fatalf("preview order PATCH = %v, want %v", patched[0], want)
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected 2 skipped results, got %#v", result.Results)
	}
	for _, item := range result.Results {
		if !item.Skipped || strings.TrimSpace(item.AssetID) == "" {
			t.Fatalf("expected skipped results to carry the existing preview ID, got %#v", item)
		}
	}
}

func TestUploadPreviewsRejectsAppendThatWouldExceedSetCapacityBeforeMutation(t *testing.T) {
	dir := t.TempDir()
	files := make([]string, 0, 2)
	for _, name := range []string{"new-1.mov", "new-2.mov"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		files = append(files, path)
	}

	requests := make([]string, 0, 2)
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/v1/appStoreVersionLocalizations/LOC_ID/appPreviewSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}]}`)
		case "/v1/appPreviewSets/set-1/appPreviews":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appPreviews","id":"existing-1","attributes":{"fileName":"old-1.mov"}},{"type":"appPreviews","id":"existing-2","attributes":{"fileName":"old-2.mov"}}]}`)
		default:
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected mutation"}]}`)
		}
	}))

	_, err := uploadPreviews(context.Background(), client, "LOC_ID", "IPHONE_65", files, false, false, false, "")
	if err == nil {
		t.Fatal("expected preview capacity error")
	}
	if !strings.Contains(err.Error(), "would exceed the preview set limit of 3") {
		t.Fatalf("unexpected error: %v", err)
	}
	wantRequests := []string{
		"GET /v1/appStoreVersionLocalizations/LOC_ID/appPreviewSets",
		"GET /v1/appPreviewSets/set-1/appPreviews",
	}
	if fmt.Sprint(requests) != fmt.Sprint(wantRequests) {
		t.Fatalf("requests = %v, want %v", requests, wantRequests)
	}
}

func TestUploadPreviewsCountsExistingPreviewsAcrossAllPages(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "new.mov")
	if err := os.WriteFile(filePath, []byte("new-preview"), 0o600); err != nil {
		t.Fatalf("write preview: %v", err)
	}

	requests := make([]string, 0, 3)
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		switch {
		case r.URL.Path == "/v1/appStoreVersionLocalizations/LOC_ID/appPreviewSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}]}`)
		case r.URL.Path == "/v1/appPreviewSets/set-1/appPreviews" && r.URL.Query().Get("cursor") == "next":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appPreviews","id":"existing-3","attributes":{"fileName":"old-3.mov"}}]}`)
		case r.URL.Path == "/v1/appPreviewSets/set-1/appPreviews":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appPreviews","id":"existing-1","attributes":{"fileName":"old-1.mov"}},{"type":"appPreviews","id":"existing-2","attributes":{"fileName":"old-2.mov"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appPreviewSets/set-1/appPreviews?cursor=next"}}`)
		default:
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected mutation"}]}`)
		}
	}))

	_, err := uploadPreviews(context.Background(), client, "LOC_ID", "IPHONE_65", []string{filePath}, false, false, true, "")
	if err == nil {
		t.Fatal("expected preview capacity error")
	}
	if !strings.Contains(err.Error(), "already contains 3 preview(s)") {
		t.Fatalf("unexpected error: %v", err)
	}
	wantRequests := []string{
		"GET /v1/appStoreVersionLocalizations/LOC_ID/appPreviewSets",
		"GET /v1/appPreviewSets/set-1/appPreviews",
		"GET /v1/appPreviewSets/set-1/appPreviews?cursor=next",
	}
	if fmt.Sprint(requests) != fmt.Sprint(wantRequests) {
		t.Fatalf("requests = %v, want %v", requests, wantRequests)
	}
}

func TestUploadPreviewsSkipsChecksumMatchFromLaterPage(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "matching.mov")
	if err := os.WriteFile(filePath, []byte("matching-preview"), 0o600); err != nil {
		t.Fatalf("write preview: %v", err)
	}
	checksum, err := computeFileChecksum(filePath)
	if err != nil {
		t.Fatalf("checksum preview: %v", err)
	}

	previewPageRequests := 0
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/appStoreVersionLocalizations/LOC_ID/appPreviewSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}]}`)
		case r.URL.Path == "/v1/appPreviewSets/set-1/appPreviews" && r.URL.Query().Get("cursor") == "next":
			previewPageRequests++
			writeAssetsTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appPreviews","id":"existing-match","attributes":{"fileName":"matching.mov","sourceFileChecksum":%q}}]}`, checksum))
		case r.URL.Path == "/v1/appPreviewSets/set-1/appPreviews":
			previewPageRequests++
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appPreviews","id":"existing-1","attributes":{"fileName":"old.mov","sourceFileChecksum":"other"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appPreviewSets/set-1/appPreviews?cursor=next"}}`)
		default:
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected request"}]}`)
		}
	}))

	result, err := uploadPreviews(context.Background(), client, "LOC_ID", "IPHONE_65", []string{filePath}, true, false, true, "")
	if err != nil {
		t.Fatalf("uploadPreviews() error: %v", err)
	}
	if previewPageRequests != 2 {
		t.Fatalf("preview page requests = %d, want 2", previewPageRequests)
	}
	if len(result.Results) != 1 || !result.Results[0].Skipped || result.Results[0].State != "skipped" {
		t.Fatalf("results = %+v, want one skipped checksum match", result.Results)
	}
}

func TestUploadPreviewsRejectsLargeSkipExistingBatchBeforeCreatingMissingSet(t *testing.T) {
	dir := t.TempDir()
	files := make([]string, 0, 4)
	for i := 1; i <= 4; i++ {
		name := fmt.Sprintf("preview-%d.mov", i)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		files = append(files, path)
	}

	requests := make([]string, 0, 1)
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodGet || r.URL.Path != "/v1/appStoreVersionLocalizations/LOC_ID/appPreviewSets" {
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected mutation"}]}`)
			return
		}
		writeAssetsTestJSON(w, http.StatusOK, `{"data":[]}`)
	}))

	_, err := uploadPreviews(context.Background(), client, "LOC_ID", "IPHONE_65", files, true, false, false, "")
	if err == nil {
		t.Fatal("expected preview capacity error")
	}
	if !strings.Contains(err.Error(), "after checksum filtering") {
		t.Fatalf("unexpected error: %v", err)
	}
	wantRequests := []string{"GET /v1/appStoreVersionLocalizations/LOC_ID/appPreviewSets"}
	if fmt.Sprint(requests) != fmt.Sprint(wantRequests) {
		t.Fatalf("requests = %v, want %v", requests, wantRequests)
	}
}

func TestUploadPreviewsAppliesSkipExistingBeforeCapacityCheck(t *testing.T) {
	dir := t.TempDir()
	matchingPath := filepath.Join(dir, "matching.mov")
	matchingContents := []byte("matching-preview")
	if err := os.WriteFile(matchingPath, matchingContents, 0o600); err != nil {
		t.Fatalf("write matching preview: %v", err)
	}
	newPath := filepath.Join(dir, "new.mov")
	if err := os.WriteFile(newPath, []byte("new-preview"), 0o600); err != nil {
		t.Fatalf("write new preview: %v", err)
	}
	matchingChecksum, err := computeFileChecksum(matchingPath)
	if err != nil {
		t.Fatalf("checksum matching preview: %v", err)
	}

	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/appStoreVersionLocalizations/LOC_ID/appPreviewSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}]}`)
		case "/v1/appPreviewSets/set-1/appPreviews":
			writeAssetsTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appPreviews","id":"existing-1","attributes":{"fileName":"matching.mov","sourceFileChecksum":%q}},{"type":"appPreviews","id":"existing-2","attributes":{"fileName":"old.mov","sourceFileChecksum":"old-checksum"}}]}`, matchingChecksum))
		default:
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected request"}]}`)
		}
	}))

	result, err := uploadPreviews(context.Background(), client, "LOC_ID", "IPHONE_65", []string{matchingPath, newPath}, true, false, true, "")
	if err != nil {
		t.Fatalf("uploadPreviews() error: %v", err)
	}
	if len(result.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(result.Results))
	}
	states := map[string]string{}
	for _, item := range result.Results {
		states[item.FileName] = item.State
	}
	if states["matching.mov"] != "skipped" || states["new.mov"] != "would-upload" {
		t.Fatalf("unexpected result states: %v", states)
	}
}

func TestUploadPreviewsAllowsLargeCandidateSetWhenChecksumSkipsFitCapacity(t *testing.T) {
	dir := t.TempDir()
	files := make([]string, 0, 4)
	for _, file := range []struct {
		name    string
		content string
	}{
		{name: "matching-a-1.mov", content: "matching-a"},
		{name: "matching-a-2.mov", content: "matching-a"},
		{name: "matching-b.mov", content: "matching-b"},
		{name: "new.mov", content: "new"},
	} {
		path := filepath.Join(dir, file.name)
		if err := os.WriteFile(path, []byte(file.content), 0o600); err != nil {
			t.Fatalf("write %s: %v", file.name, err)
		}
		files = append(files, path)
	}
	checksumA, err := computeFileChecksum(files[0])
	if err != nil {
		t.Fatalf("checksum matching-a: %v", err)
	}
	checksumB, err := computeFileChecksum(files[2])
	if err != nil {
		t.Fatalf("checksum matching-b: %v", err)
	}

	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/appStoreVersionLocalizations/LOC_ID/appPreviewSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}]}`)
		case "/v1/appPreviewSets/set-1/appPreviews":
			writeAssetsTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appPreviews","id":"existing-a","attributes":{"sourceFileChecksum":%q}},{"type":"appPreviews","id":"existing-b","attributes":{"sourceFileChecksum":%q}}]}`, checksumA, checksumB))
		default:
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected request"}]}`)
		}
	}))

	result, err := uploadPreviews(context.Background(), client, "LOC_ID", "IPHONE_65", files, true, false, true, "")
	if err != nil {
		t.Fatalf("uploadPreviews() error: %v", err)
	}
	if len(result.Results) != 4 {
		t.Fatalf("results = %d, want 4", len(result.Results))
	}
	skipped := 0
	wouldUpload := 0
	for _, item := range result.Results {
		if item.Skipped {
			skipped++
		}
		if item.State == "would-upload" {
			wouldUpload++
		}
	}
	if skipped != 3 || wouldUpload != 1 {
		t.Fatalf("skipped = %d, would upload = %d; want 3 and 1", skipped, wouldUpload)
	}
}

func TestUploadPreviewsRejectsReplaceBatchAboveCapacityAfterFiltering(t *testing.T) {
	dir := t.TempDir()
	files := make([]string, 0, 4)
	for i := 1; i <= 4; i++ {
		name := fmt.Sprintf("preview-%d.mov", i)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		files = append(files, path)
	}

	requests := make([]string, 0, 2)
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/v1/appStoreVersionLocalizations/LOC_ID/appPreviewSets" {
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}]}`)
			return
		}
		if r.URL.Path == "/v1/appPreviewSets/set-1/appPreviews" {
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appPreviews","id":"existing-1","attributes":{"fileName":"old.mov"}}]}`)
			return
		}
		writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected mutation"}]}`)
	}))

	_, err := uploadPreviews(context.Background(), client, "LOC_ID", "IPHONE_65", files, true, true, false, "")
	if err == nil {
		t.Fatal("expected replace capacity error")
	}
	if !strings.Contains(err.Error(), "after checksum filtering") {
		t.Fatalf("unexpected error: %v", err)
	}
	wantRequests := []string{
		"GET /v1/appStoreVersionLocalizations/LOC_ID/appPreviewSets",
		"GET /v1/appPreviewSets/set-1/appPreviews",
	}
	if fmt.Sprint(requests) != fmt.Sprint(wantRequests) {
		t.Fatalf("requests = %v, want %v", requests, wantRequests)
	}
}

func TestUploadPreviewFilesRollsBackAllCreatedItemsOnPostCreateConflict(t *testing.T) {
	requests := make([]string, 0, 2)
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodDelete || (r.URL.Path != "/v1/appPreviews/created-1" && r.URL.Path != "/v1/appPreviews/created-2") {
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected request"}]}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	uploadCalls := 0
	capacityErr := &asc.APIError{
		Code:       "ENTITY_ERROR.RELATIONSHIP.INVALID",
		Detail:     "The preview set is full.",
		StatusCode: http.StatusConflict,
	}
	results, err := uploadPreviewFiles(
		context.Background(),
		context.Background(),
		client,
		"set-1",
		[]string{"first.mov", "second.mov"},
		func(_ context.Context, _ *asc.Client, _, filePath string) (asc.AssetUploadResultItem, error) {
			uploadCalls++
			if filePath == "first.mov" {
				return asc.AssetUploadResultItem{AssetID: "created-1"}, nil
			}
			return asc.AssetUploadResultItem{AssetID: "created-2"}, capacityErr
		},
	)
	if !errors.Is(err, asc.ErrConflict) {
		t.Fatalf("uploadPreviewFiles() error = %v, want conflict", err)
	}
	if !strings.Contains(err.Error(), "The preview set is full") {
		t.Fatalf("uploadPreviewFiles() error = %v, want original API detail", err)
	}
	if uploadCalls != 2 {
		t.Fatalf("upload calls = %d, want 2", uploadCalls)
	}
	if len(results) != 2 {
		t.Fatalf("uploadPreviewFiles() results = %d items, want 2", len(results))
	}
	if results[0].AssetID != "created-1" || results[1].AssetID != "created-2" {
		t.Fatalf("uploadPreviewFiles() results = %#v, want created asset IDs", results)
	}
	if results[0].State != "rolled-back" || results[1].State != "rolled-back" {
		t.Fatalf("uploadPreviewFiles() states = %#v, want rolled-back results", results)
	}
	wantRequests := []string{
		"DELETE /v1/appPreviews/created-2",
		"DELETE /v1/appPreviews/created-1",
	}
	if fmt.Sprint(requests) != fmt.Sprint(wantRequests) {
		t.Fatalf("requests = %v, want %v", requests, wantRequests)
	}
}

func TestUploadPreviewFilesPreservesResultsWhenRollbackFails(t *testing.T) {
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected request"}]}`)
			return
		}
		writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"rollback failed"}]}`)
	}))

	uploadErr := errors.New("preview upload failed")
	results, err := uploadPreviewFiles(
		context.Background(),
		context.Background(),
		client,
		"set-1",
		[]string{"first.mov", "second.mov"},
		func(_ context.Context, _ *asc.Client, _, filePath string) (asc.AssetUploadResultItem, error) {
			if filePath == "first.mov" {
				return asc.AssetUploadResultItem{AssetID: "created-1"}, nil
			}
			return asc.AssetUploadResultItem{AssetID: "created-2"}, uploadErr
		},
	)
	if !errors.Is(err, uploadErr) {
		t.Fatalf("uploadPreviewFiles() error = %v, want original upload error", err)
	}
	if !strings.Contains(err.Error(), "roll back previews created by this upload") {
		t.Fatalf("uploadPreviewFiles() error = %v, want rollback failure context", err)
	}
	if !strings.Contains(err.Error(), "rollback failed") {
		t.Fatalf("uploadPreviewFiles() error = %v, want rollback failure detail", err)
	}
	if len(results) != 2 {
		t.Fatalf("uploadPreviewFiles() results = %d items, want 2", len(results))
	}
	if results[0].AssetID != "created-1" || results[1].AssetID != "created-2" {
		t.Fatalf("uploadPreviewFiles() results = %#v, want created asset IDs", results)
	}
	if results[0].State != "rollback-failed" || results[1].State != "rollback-failed" {
		t.Fatalf("uploadPreviewFiles() states = %#v, want rollback-failed results", results)
	}
}

func TestUploadPreviewFilesRollsBackPostCreateFailureWithFreshContext(t *testing.T) {
	requests := make([]string, 0, 1)
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/appPreviews/created-1" {
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected request"}]}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	uploadCtx, cancelUpload := context.WithCancel(context.Background())
	cancelUpload()
	uploadErr := errors.New("upload failed after reservation")
	_, err := uploadPreviewFiles(
		uploadCtx,
		context.Background(),
		client,
		"set-1",
		[]string{"preview.mov"},
		func(ctx context.Context, _ *asc.Client, _, _ string) (asc.AssetUploadResultItem, error) {
			if ctx.Err() == nil {
				t.Fatal("expected upload context to be expired")
			}
			return asc.AssetUploadResultItem{AssetID: "created-1"}, uploadErr
		},
	)
	if !errors.Is(err, uploadErr) {
		t.Fatalf("uploadPreviewFiles() error = %v, want original upload error", err)
	}
	wantRequests := []string{"DELETE /v1/appPreviews/created-1"}
	if fmt.Sprint(requests) != fmt.Sprint(wantRequests) {
		t.Fatalf("requests = %v, want %v", requests, wantRequests)
	}
}

func TestUploadPreviewFilesStartsRollbackTimeoutAfterSlowUpload(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "20ms")
	requests := make([]string, 0, 1)
	client := newAssetsUploadTestHandlerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/appPreviews/created-1" {
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected request"}]}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	uploadErr := errors.New("slow upload failed after reservation")
	var err error
	synctest.Test(t, func(*testing.T) {
		_, err = uploadPreviewFiles(
			context.Background(),
			context.Background(),
			client,
			"set-1",
			[]string{"preview.mov"},
			func(context.Context, *asc.Client, string, string) (asc.AssetUploadResultItem, error) {
				time.Sleep(50 * time.Millisecond)
				return asc.AssetUploadResultItem{AssetID: "created-1"}, uploadErr
			},
		)
	})
	if !errors.Is(err, uploadErr) {
		t.Fatalf("uploadPreviewFiles() error = %v, want original upload error", err)
	}
	wantRequests := []string{"DELETE /v1/appPreviews/created-1"}
	if fmt.Sprint(requests) != fmt.Sprint(wantRequests) {
		t.Fatalf("requests = %v, want %v", requests, wantRequests)
	}
}

func TestUploadPreviewFilesDetachesRollbackFromCallerCancellation(t *testing.T) {
	requests := make([]string, 0, 1)
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/appPreviews/created-1" {
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected request"}]}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	rollbackBase, cancelRollback := context.WithCancel(context.Background())
	cancelRollback()
	uploadErr := errors.New("upload interrupted after reservation")
	_, err := uploadPreviewFiles(
		context.Background(),
		rollbackBase,
		client,
		"set-1",
		[]string{"preview.mov"},
		func(context.Context, *asc.Client, string, string) (asc.AssetUploadResultItem, error) {
			return asc.AssetUploadResultItem{AssetID: "created-1"}, uploadErr
		},
	)
	if !errors.Is(err, uploadErr) {
		t.Fatalf("uploadPreviewFiles() error = %v, want original upload error", err)
	}
	wantRequests := []string{"DELETE /v1/appPreviews/created-1"}
	if fmt.Sprint(requests) != fmt.Sprint(wantRequests) {
		t.Fatalf("requests = %v, want %v", requests, wantRequests)
	}
}

func TestNormalizePreviewType(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr string
	}{
		{input: "IPHONE_69", want: "IPHONE_67"},
		{input: "APP_IPHONE_69", want: "IPHONE_67"},
		{input: " app_iphone_69 ", want: "IPHONE_67"},
		{input: "IPHONE_70", wantErr: `unsupported preview type "IPHONE_70"`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := normalizePreviewType(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("normalizePreviewType(%q) error = %v, want %q", tt.input, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizePreviewType(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("normalizePreviewType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidPreviewFrameTimeCode(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "frame format", value: "00:00:05:00", want: true},
		{name: "millisecond format", value: "00:00:05.000", want: true},
		{name: "frame upper bound", value: "99:59:59:29", want: true},
		{name: "non numeric", value: "abc", want: false},
		{name: "missing component", value: "00:00:05", want: false},
		{name: "invalid minute", value: "00:60:05:00", want: false},
		{name: "invalid second", value: "00:00:60.000", want: false},
		{name: "invalid frame", value: "00:00:05:30", want: false},
		{name: "invalid millisecond width", value: "00:00:05.00", want: false},
		{name: "invalid separator", value: "00-00-05-00", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidPreviewFrameTimeCode(tt.value); got != tt.want {
				t.Fatalf("isValidPreviewFrameTimeCode(%q) = %t, want %t", tt.value, got, tt.want)
			}
		})
	}
}

func TestUploadPreviewFilesCleansNewSetWithBoundedDetachedContext(t *testing.T) {
	for _, reserved := range []bool{false, true} {
		t.Run(fmt.Sprint(reserved), func(t *testing.T) {
			t.Setenv("ASC_TIMEOUT", "1s")
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			var requests []string
			http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Context().Err() != nil {
					t.Fatalf("cleanup inherited cancellation: %v", req.Context().Err())
				}
				deadline, ok := req.Context().Deadline()
				if !ok || time.Until(deadline) > time.Second {
					t.Fatalf("cleanup lacks bounded timeout: %v", deadline)
				}
				requests = append(requests, req.Method+" "+req.URL.Path)
				return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
			})
			base, cancel := context.WithCancel(context.Background())
			cancel()
			results, err := uploadPreviewFilesWithSetCleanup(base, base, newAssetsUploadTestClient(t), "new-set", true, []string{"preview.mov"}, func(ctx context.Context, _ *asc.Client, _, _ string) (asc.AssetUploadResultItem, error) {
				if ctx.Err() != context.Canceled {
					t.Fatal("expected canceled upload")
				}
				if reserved {
					return asc.AssetUploadResultItem{AssetID: "reserved-1"}, context.Canceled
				}
				return asc.AssetUploadResultItem{}, context.Canceled
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("original cancellation cause lost: %v", err)
			}
			want := []string{"DELETE /v1/appPreviewSets/new-set"}
			if reserved {
				want = append([]string{"DELETE /v1/appPreviews/reserved-1"}, want...)
				if len(results) != 1 || results[0].AssetID != "reserved-1" || results[0].State != "rolled-back" {
					t.Fatalf("receipt=%#v", results)
				}
			}
			if !reflect.DeepEqual(requests, want) {
				t.Fatalf("requests=%v want=%v", requests, want)
			}
		})
	}
}
