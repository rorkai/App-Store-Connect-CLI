package assets

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// installAssetsTestTransport swaps http.DefaultTransport for the test and
// restores it on cleanup.
func installAssetsTestTransport(t *testing.T, fn assetsUploadRoundTripFunc) {
	t.Helper()
	origTransport := http.DefaultTransport
	http.DefaultTransport = fn
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})
}

func TestUploadScreenshotsToSetParallelCompletionPreservesFileOrder(t *testing.T) {
	dir := t.TempDir()
	files := make([]string, 0, assetTransferWorkerLimit)
	for i := 1; i <= assetTransferWorkerLimit; i++ {
		files = append(files, writeAssetsTestPNG(t, dir, fmt.Sprintf("%02d-screen.png", i)))
	}
	sizeBytes := fileSize(t, files[0])

	// The first file's binary upload blocks until the last file has fully
	// completed, so completion order is deliberately different from file
	// order.
	lastFileComplete := make(chan struct{})
	var lastFileCompleteOnce sync.Once
	var patchedIDs atomic.Pointer[[]string]

	installAssetsTestTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			for idx := range files {
				if strings.Contains(string(body), filepath.Base(files[idx])) {
					return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-%d","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-%d","length":%d,"offset":0}]}}}`, idx+1, idx+1, sizeBytes))
				}
			}
			return assetsJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"UNKNOWN_FILE","detail":"unknown file"}]}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			if req.URL.Path == "/new-1" {
				select {
				case <-lastFileComplete:
				case <-req.Context().Done():
					return nil, req.Context().Err()
				}
			}
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/new-"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/new-"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			if id == fmt.Sprintf("new-%d", len(files)) {
				lastFileCompleteOnce.Do(func() { close(lastFileComplete) })
			}
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`, id))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			var payload struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				return nil, err
			}
			ids := make([]string, 0, len(payload.Data))
			for _, item := range payload.Data {
				ids = append(ids, item.ID)
			}
			patchedIDs.Store(&ids)
			return assetsJSONResponse(http.StatusNoContent, "")
		default:
			return assetsJSONResponse(http.StatusNotFound, fmt.Sprintf(`{"errors":[{"status":"404","code":"UNEXPECTED","detail":"unexpected request %s %s"}]}`, req.Method, req.URL.String()))
		}
	})

	client := newAssetsUploadTestClient(t)
	results, err := UploadScreenshotsToSet(context.Background(), client, "set-1", files, false)
	if err != nil {
		t.Fatalf("UploadScreenshotsToSet() error: %v", err)
	}

	if len(results) != len(files) {
		t.Fatalf("expected %d results, got %d", len(files), len(results))
	}
	for idx, item := range results {
		wantID := fmt.Sprintf("new-%d", idx+1)
		if item.AssetID != wantID || item.FilePath != files[idx] {
			t.Fatalf("result %d = %#v, want asset %s for %s", idx, item, wantID, files[idx])
		}
	}

	got := patchedIDs.Load()
	if got == nil {
		t.Fatal("expected screenshot relationship reorder PATCH to be called")
	}
	want := []string{"new-1", "new-2", "new-3", "new-4"}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("relationship order = %v, want %v", *got, want)
	}
}

func TestUploadScreenshotsWithOrderStateFailureReportsPendingFilesInFileOrder(t *testing.T) {
	dir := t.TempDir()
	fileA := writeAssetsTestPNG(t, dir, "01-a.png")
	fileB := writeAssetsTestPNG(t, dir, "02-b.png")
	fileC := writeAssetsTestPNG(t, dir, "03-c.png")
	files := []string{fileA, fileB, fileC}
	sizeBytes := fileSize(t, fileA)

	// The middle file fails only after its two siblings have fully
	// completed, so the failure outcome is deterministic even though the
	// uploads run concurrently.
	siblingComplete := make(chan struct{}, 2)
	var relationshipPatchCalled atomic.Bool

	installAssetsTestTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			switch {
			case strings.Contains(string(body), "01-a.png"):
				return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-a","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-a","length":%d,"offset":0}]}}}`, sizeBytes))
			case strings.Contains(string(body), "03-c.png"):
				return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-c","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-c","length":%d,"offset":0}]}}}`, sizeBytes))
			default:
				for i := 0; i < 2; i++ {
					select {
					case <-siblingComplete:
					case <-req.Context().Done():
						return nil, req.Context().Err()
					}
				}
				return assetsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"upload create failed"}]}`)
			}
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/new-"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/new-"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			siblingComplete <- struct{}{}
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`, id))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			relationshipPatchCalled.Store(true)
			return assetsJSONResponse(http.StatusNoContent, "")
		default:
			return assetsJSONResponse(http.StatusNotFound, fmt.Sprintf(`{"errors":[{"status":"404","code":"UNEXPECTED","detail":"unexpected request %s %s"}]}`, req.Method, req.URL.String()))
		}
	})

	client := newAssetsUploadTestClient(t)
	progress, err := uploadScreenshotsWithOrderState(context.Background(), client, "set-1", nil, files, false, true)
	if err == nil {
		t.Fatal("expected uploadScreenshotsWithOrderState() error")
	}
	if !strings.Contains(err.Error(), "upload create failed") {
		t.Fatalf("expected create failure to propagate, got %v", err)
	}

	if progress.FailedFile != fileB {
		t.Fatalf("expected failed file %q, got %q", fileB, progress.FailedFile)
	}
	if !reflect.DeepEqual(progress.PendingFiles, []string{fileB}) {
		t.Fatalf("expected pending files [%s], got %#v", fileB, progress.PendingFiles)
	}
	if len(progress.Results) != 2 || progress.Results[0].AssetID != "new-a" || progress.Results[1].AssetID != "new-c" {
		t.Fatalf("expected completed sibling results in file order, got %#v", progress.Results)
	}
	if !reflect.DeepEqual(progress.OrderedIDs, []string{"new-a", "new-c"}) {
		t.Fatalf("expected ordered IDs of completed uploads, got %#v", progress.OrderedIDs)
	}
	if relationshipPatchCalled.Load() {
		t.Fatal("expected no relationship reorder PATCH after upload failure")
	}
}

func TestUploadScreenshotsRollsBackCreatedItemCanceledBySiblingFailure(t *testing.T) {
	dir := t.TempDir()
	fileA := writeAssetsTestPNG(t, dir, "01-created.png")
	fileB := writeAssetsTestPNG(t, dir, "02-fails.png")
	sizeBytes := fileSize(t, fileA)
	createdA := make(chan struct{})
	deletedIDs := make([]string, 0, 1)

	installAssetsTestTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			if strings.Contains(string(body), filepath.Base(fileA)) {
				close(createdA)
				return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-a","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-a","length":%d,"offset":0}]}}}`, sizeBytes))
			}
			select {
			case <-createdA:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			return assetsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"sibling create failed"}]}`)
		case req.Method == http.MethodPut && req.URL.Path == "/new-a":
			<-req.Context().Done()
			return nil, req.Context().Err()
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appScreenshots/new-a":
			deletedIDs = append(deletedIDs, "new-a")
			return assetsJSONResponse(http.StatusNoContent, "")
		default:
			return assetsJSONResponse(http.StatusNotFound, fmt.Sprintf(`{"errors":[{"status":"404","code":"UNEXPECTED","detail":"unexpected request %s %s"}]}`, req.Method, req.URL.String()))
		}
	})

	client := newAssetsUploadTestClient(t)
	progress, err := uploadScreenshotsWithOrderState(context.Background(), client, "set-1", nil, []string{fileA, fileB}, false, true)
	if err == nil || !strings.Contains(err.Error(), "sibling create failed") {
		t.Fatalf("expected sibling create failure, got %v", err)
	}
	if !reflect.DeepEqual(progress.PendingFiles, []string{fileA, fileB}) {
		t.Fatalf("pending files = %v, want both files in input order", progress.PendingFiles)
	}
	if !reflect.DeepEqual(deletedIDs, []string{"new-a"}) {
		t.Fatalf("rolled back screenshot IDs = %v, want [new-a]", deletedIDs)
	}
}

func TestUploadPreviewsParallelCompletionPreservesFileOrder(t *testing.T) {
	if err := mime.AddExtensionType(".mov", "video/quicktime"); err != nil {
		t.Fatalf("register .mov mime type: %v", err)
	}
	dir := t.TempDir()
	names := []string{"01-intro.mov", "02-detail.mov", "03-outro.mov"}
	files := make([]string, 0, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("preview-video-bytes"), 0o600); err != nil {
			t.Fatalf("write preview file: %v", err)
		}
		files = append(files, path)
	}
	sizeBytes := fileSize(t, files[0])

	firstFileGate := make(chan struct{})
	var firstFileGateOnce sync.Once
	var patchedIDs atomic.Pointer[[]string]

	installAssetsTestTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appPreviewSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appPreviews":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			for idx, name := range names {
				if strings.Contains(string(body), name) {
					return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appPreviews","id":"preview-%d","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/preview-%d","length":%d,"offset":0}]}}}`, idx+1, idx+1, sizeBytes))
				}
			}
			return assetsJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"UNKNOWN_FILE","detail":"unknown file"}]}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			if req.URL.Path == "/preview-1" {
				select {
				case <-firstFileGate:
				case <-req.Context().Done():
					return nil, req.Context().Err()
				}
			}
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appPreviews/preview-"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appPreviews/")
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appPreviews","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appPreviews/preview-"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appPreviews/")
			if id == fmt.Sprintf("preview-%d", len(names)) {
				firstFileGateOnce.Do(func() { close(firstFileGate) })
			}
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appPreviews","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`, id))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviewSets/set-1/relationships/appPreviews":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appPreviews","id":"existing-1"},{"type":"appPreviews","id":"preview-3"},{"type":"appPreviews","id":"preview-2"},{"type":"appPreviews","id":"preview-1"}],"links":{}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appPreviewSets/set-1/relationships/appPreviews":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			var payload struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				return nil, err
			}
			ids := make([]string, 0, len(payload.Data))
			for _, item := range payload.Data {
				ids = append(ids, item.ID)
			}
			patchedIDs.Store(&ids)
			return assetsJSONResponse(http.StatusNoContent, "")
		default:
			return assetsJSONResponse(http.StatusNotFound, fmt.Sprintf(`{"errors":[{"status":"404","code":"UNEXPECTED","detail":"unexpected request %s %s"}]}`, req.Method, req.URL.String()))
		}
	})

	client := newAssetsUploadTestClient(t)
	result, err := uploadPreviews(context.Background(), client, "LOC_123", "IPHONE_65", files, false, false, false)
	if err != nil {
		t.Fatalf("uploadPreviews() error: %v", err)
	}

	if len(result.Results) != len(files) {
		t.Fatalf("expected %d results, got %d", len(files), len(result.Results))
	}
	for idx, item := range result.Results {
		wantID := fmt.Sprintf("preview-%d", idx+1)
		if item.AssetID != wantID || item.FilePath != files[idx] {
			t.Fatalf("result %d = %#v, want asset %s for %s", idx, item, wantID, files[idx])
		}
	}

	got := patchedIDs.Load()
	if got == nil {
		t.Fatal("expected preview relationship reorder PATCH to be called")
	}
	want := []string{"existing-1", "preview-1", "preview-2", "preview-3"}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("relationship order = %v, want %v", *got, want)
	}
}

func TestUploadPreviewsPropagatesFirstUploadFailure(t *testing.T) {
	if err := mime.AddExtensionType(".mov", "video/quicktime"); err != nil {
		t.Fatalf("register .mov mime type: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "01-intro.mov")
	if err := os.WriteFile(path, []byte("preview-video-bytes"), 0o600); err != nil {
		t.Fatalf("write preview file: %v", err)
	}

	installAssetsTestTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appPreviewSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appPreviews":
			return assetsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"preview create failed"}]}`)
		default:
			return assetsJSONResponse(http.StatusNotFound, fmt.Sprintf(`{"errors":[{"status":"404","code":"UNEXPECTED","detail":"unexpected request %s %s"}]}`, req.Method, req.URL.String()))
		}
	})

	client := newAssetsUploadTestClient(t)
	_, err := uploadPreviews(context.Background(), client, "LOC_123", "IPHONE_65", []string{path}, false, false, false)
	if err == nil {
		t.Fatal("expected uploadPreviews() error")
	}
	if !strings.Contains(err.Error(), "preview create failed") {
		t.Fatalf("expected create failure to propagate, got %v", err)
	}
}

func TestUploadPreviewsRollsBackCreatedItemsAfterPartialFailure(t *testing.T) {
	if err := mime.AddExtensionType(".mov", "video/quicktime"); err != nil {
		t.Fatalf("register .mov mime type: %v", err)
	}
	dir := t.TempDir()
	names := []string{"01-intro.mov", "02-fails.mov", "03-outro.mov"}
	files := make([]string, 0, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("preview-video-bytes"), 0o600); err != nil {
			t.Fatalf("write preview file: %v", err)
		}
		files = append(files, path)
	}
	sizeBytes := fileSize(t, files[0])

	// The failing create waits for both siblings to finish, proving rollback
	// covers previews that committed before another parallel worker failed.
	siblingComplete := make(chan struct{}, 2)
	deletedIDs := make([]string, 0, 2)
	var relationshipPatchCalled atomic.Bool

	installAssetsTestTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appPreviewSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appPreviews":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			switch {
			case strings.Contains(string(body), names[0]):
				return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appPreviews","id":"preview-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/preview-1","length":%d,"offset":0}]}}}`, sizeBytes))
			case strings.Contains(string(body), names[2]):
				return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appPreviews","id":"preview-3","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/preview-3","length":%d,"offset":0}]}}}`, sizeBytes))
			default:
				for range 2 {
					select {
					case <-siblingComplete:
					case <-req.Context().Done():
						return nil, req.Context().Err()
					}
				}
				return assetsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"preview create failed"}]}`)
			}
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appPreviews/preview-"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appPreviews/")
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appPreviews","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appPreviews/preview-"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appPreviews/")
			siblingComplete <- struct{}{}
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appPreviews","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`, id))
		case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/v1/appPreviews/preview-"):
			deletedIDs = append(deletedIDs, strings.TrimPrefix(req.URL.Path, "/v1/appPreviews/"))
			return assetsJSONResponse(http.StatusNoContent, "")
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appPreviewSets/set-1/relationships/appPreviews":
			relationshipPatchCalled.Store(true)
			return assetsJSONResponse(http.StatusNoContent, "")
		default:
			return assetsJSONResponse(http.StatusNotFound, fmt.Sprintf(`{"errors":[{"status":"404","code":"UNEXPECTED","detail":"unexpected request %s %s"}]}`, req.Method, req.URL.String()))
		}
	})

	client := newAssetsUploadTestClient(t)
	_, err := uploadPreviews(context.Background(), client, "LOC_123", "IPHONE_65", files, false, false, false)
	if err == nil || !strings.Contains(err.Error(), "preview create failed") {
		t.Fatalf("expected preview create failure, got %v", err)
	}
	if !reflect.DeepEqual(deletedIDs, []string{"preview-1", "preview-3"}) {
		t.Fatalf("rolled back preview IDs = %v, want [preview-1 preview-3]", deletedIDs)
	}
	if relationshipPatchCalled.Load() {
		t.Fatal("expected rollback instead of relationship reorder after partial failure")
	}
}

func TestUploadPreviewsRollsBackCreatedItemsAfterReorderFailure(t *testing.T) {
	if err := mime.AddExtensionType(".mov", "video/quicktime"); err != nil {
		t.Fatalf("register .mov mime type: %v", err)
	}
	dir := t.TempDir()
	names := []string{"01-intro.mov", "02-outro.mov"}
	files := make([]string, 0, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("preview-video-bytes"), 0o600); err != nil {
			t.Fatalf("write preview file: %v", err)
		}
		files = append(files, path)
	}
	sizeBytes := fileSize(t, files[0])
	deletedIDs := make([]string, 0, len(files))

	installAssetsTestTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appPreviewSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appPreviews":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			for idx, name := range names {
				if strings.Contains(string(body), name) {
					return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appPreviews","id":"preview-%d","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/preview-%d","length":%d,"offset":0}]}}}`, idx+1, idx+1, sizeBytes))
				}
			}
			return assetsJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"UNKNOWN_FILE","detail":"unknown file"}]}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appPreviews/preview-"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appPreviews/")
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appPreviews","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appPreviews/preview-"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appPreviews/")
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appPreviews","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`, id))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviewSets/set-1/relationships/appPreviews":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appPreviews","id":"preview-2"},{"type":"appPreviews","id":"preview-1"}],"links":{}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appPreviewSets/set-1/relationships/appPreviews":
			return assetsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"REORDER_FAILED","detail":"preview reorder failed"}]}`)
		case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/v1/appPreviews/preview-"):
			deletedIDs = append(deletedIDs, strings.TrimPrefix(req.URL.Path, "/v1/appPreviews/"))
			return assetsJSONResponse(http.StatusNoContent, "")
		default:
			return assetsJSONResponse(http.StatusNotFound, fmt.Sprintf(`{"errors":[{"status":"404","code":"UNEXPECTED","detail":"unexpected request %s %s"}]}`, req.Method, req.URL.String()))
		}
	})

	client := newAssetsUploadTestClient(t)
	_, err := uploadPreviews(context.Background(), client, "LOC_123", "IPHONE_65", files, false, false, false)
	if err == nil || !strings.Contains(err.Error(), "preview reorder failed") {
		t.Fatalf("expected preview reorder failure, got %v", err)
	}
	if !reflect.DeepEqual(deletedIDs, []string{"preview-1", "preview-2"}) {
		t.Fatalf("rolled back preview IDs = %v, want [preview-1 preview-2]", deletedIDs)
	}
}
