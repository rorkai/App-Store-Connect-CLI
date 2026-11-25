package assets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestConcurrentScreenshotUploadPreservesOrderAndSpeedsUp(t *testing.T) {
	files := make([]string, 40)
	for i := range files {
		files[i] = fmt.Sprintf("%02d.png", i)
	}
	previous := uploadOneScreenshot
	t.Cleanup(func() { uploadOneScreenshot = previous })
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	uploadOneScreenshot = func(_ context.Context, _ io.Writer, _ *asc.Client, _, filePath, _ string, _ openedScreenshotFiles) (asc.AssetUploadResultItem, screenshotPendingAsset, error) {
		current := inFlight.Add(1)
		for {
			seen := maxInFlight.Load()
			if current <= seen || maxInFlight.CompareAndSwap(seen, current) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		inFlight.Add(-1)
		return asc.AssetUploadResultItem{AssetID: filePath, FileName: filePath}, screenshotPendingAsset{}, nil
	}
	var progress screenshotUploadProgress
	var err error
	var elapsed time.Duration
	synctest.Test(t, func(*testing.T) {
		start := time.Now()
		progress, err = uploadScreenshotsWithOrderStateWithOpenedFiles(withScreenshotUploadConcurrency(context.Background(), 4), nil, shared.ContextWithTimeout, nil, "set", nil, files, "", false, false, nil)
		elapsed = time.Since(start)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(progress.Results) != len(files) || len(progress.OrderedIDs) != len(files) {
		t.Fatalf("progress = %+v", progress)
	}
	for index, file := range files {
		if progress.OrderedIDs[index] != file {
			t.Fatalf("order[%d]=%s, want %s", index, progress.OrderedIDs[index], file)
		}
	}
	if max := maxInFlight.Load(); max != 4 {
		t.Fatalf("max in-flight = %d, want 4", max)
	}
	if elapsed != 50*time.Millisecond {
		t.Fatalf("elapsed %s, want 40 delayed uploads in 10 rounds of 5ms", elapsed)
	}
}

// Concurrent upload workers share the command's stderr, which may be a writer
// that is not safe for concurrent use. Run under -race.
func TestConcurrentScreenshotUploadsSerializeStderrWrites(t *testing.T) {
	files := make([]string, 10)
	for i := range files {
		files[i] = fmt.Sprintf("%02d.png", i)
	}
	previous := uploadOneScreenshot
	t.Cleanup(func() { uploadOneScreenshot = previous })
	uploadOneScreenshot = func(_ context.Context, stderr io.Writer, _ *asc.Client, _, filePath, _ string, _ openedScreenshotFiles) (asc.AssetUploadResultItem, screenshotPendingAsset, error) {
		fmt.Fprintf(stderr, "notice %s\n", filePath)
		return asc.AssetUploadResultItem{AssetID: filePath, FileName: filePath}, screenshotPendingAsset{}, nil
	}
	var stderr strings.Builder
	if _, err := uploadScreenshotsWithOrderStateWithOpenedFiles(withScreenshotUploadConcurrency(context.Background(), len(files)), &stderr, shared.ContextWithTimeout, nil, "set", nil, files, "", false, false, nil); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
	slices.Sort(lines)
	want := make([]string, len(files))
	for i, file := range files {
		want[i] = "notice " + file
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("stderr lines = %q, want %q", lines, want)
	}
}

func TestConcurrentScreenshotUploadAggregatesCleanupFailures(t *testing.T) {
	for _, resumed := range []bool{false, true} {
		t.Run(fmt.Sprintf("resumed=%t", resumed), func(t *testing.T) {
			uploadErr := errors.New("upload failed")
			files := []string{"01.png", "02.png", "03.png", "04.png"}
			previous := uploadOneScreenshot
			t.Cleanup(func() { uploadOneScreenshot = previous })
			uploadOneScreenshot = func(_ context.Context, _ io.Writer, _ *asc.Client, _, filePath, _ string, _ openedScreenshotFiles) (asc.AssetUploadResultItem, screenshotPendingAsset, error) {
				if filePath == files[0] {
					return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, uploadErr
				}
				return asc.AssetUploadResultItem{AssetID: "asset-" + filePath, FileName: filePath, FilePath: filePath}, screenshotPendingAsset{}, nil
			}

			var deleted []string
			client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodDelete {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
				deleted = append(deleted, id)
				if id == "asset-02.png" || id == "asset-04.png" {
					writeAssetsTestJSON(w, http.StatusInternalServerError, fmt.Sprintf(`{"errors":[{"status":"500","detail":"delete %s failed"}]}`, id))
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))

			ctx := withScreenshotUploadConcurrency(context.Background(), 4)
			if resumed {
				ctx = context.WithValue(ctx, screenshotResumeKey{}, true)
			}
			progress, err := uploadScreenshotsWithOrderStateWithOpenedFiles(
				ctx,
				nil,
				shared.ContextWithTimeout,
				client, "set", nil, files, "", false, false, nil,
			)
			if !errors.Is(err, uploadErr) {
				t.Fatalf("error = %v, want original upload error", err)
			}
			if resumed {
				if len(deleted) != 0 || len(progress.CleanupFailures) != 3 || !strings.Contains(err.Error(), "receipts do not authorize screenshot deletion") {
					t.Fatalf("resume error=%v deleted=%v cleanup=%+v", err, deleted, progress.CleanupFailures)
				}
				return
			}
			for _, detail := range []string{"delete asset-02.png failed", "delete asset-04.png failed"} {
				if !strings.Contains(err.Error(), detail) {
					t.Fatalf("error = %q, want cleanup detail %q", err, detail)
				}
			}
			if len(deleted) != 3 {
				t.Fatalf("deleted assets = %v, want all 3 completed siblings", deleted)
			}
			if len(progress.CleanupFailures) != 2 {
				t.Fatalf("cleanup failures = %#v, want 2 unresolved assets", progress.CleanupFailures)
			}
			if progress.CleanupFailures[0].AssetID != "asset-04.png" || progress.CleanupFailures[1].AssetID != "asset-02.png" {
				t.Fatalf("cleanup failures = %#v, want reverse cleanup order", progress.CleanupFailures)
			}
		})
	}
}

func TestCleanupScreenshotAssetsUsesFreshBoundedContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	requests := make(chan struct{}, 1)
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete || req.URL.Path != "/v1/appScreenshots/asset-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		requests <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))

	remaining, err := cleanupScreenshotAssets(ctx, shared.ContextWithTimeout, client, []screenshotPendingAsset{{AssetID: "asset-1"}})
	if err != nil {
		t.Fatalf("cleanupScreenshotAssets() error: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("remaining assets = %#v, want none", remaining)
	}
	select {
	case <-requests:
	default:
		t.Fatal("expected cleanup DELETE after caller cancellation")
	}
}

func TestCleanupScreenshotAssetsUsesBoundedTimeout(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "20ms")
	requestCanceled := make(chan struct{}, 1)
	client := newAssetsUploadTestHandlerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete || req.URL.Path != "/v1/appScreenshots/asset-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		<-req.Context().Done()
		requestCanceled <- struct{}{}
	}))

	var remaining []screenshotPendingAsset
	var err error
	var elapsed time.Duration
	synctest.Test(t, func(*testing.T) {
		started := time.Now()
		remaining, err = cleanupScreenshotAssets(context.Background(), shared.ContextWithTimeout, client, []screenshotPendingAsset{{AssetID: "asset-1"}})
		elapsed = time.Since(started)
	})
	if err == nil {
		t.Fatal("cleanupScreenshotAssets() error = nil, want bounded timeout error")
	}
	if len(remaining) != 1 || remaining[0].AssetID != "asset-1" {
		t.Fatalf("remaining assets = %#v, want asset-1", remaining)
	}
	if elapsed > time.Second {
		t.Fatalf("cleanupScreenshotAssets() took %s, want bounded timeout", elapsed)
	}
	select {
	case <-requestCanceled:
	default:
		t.Fatal("server request did not observe cleanup cancellation")
	}
}

func TestConcurrentScreenshotUploadReservesScreenshotsInFileOrder(t *testing.T) {
	sourceRoot := t.TempDir()
	files := make([]string, 4)
	for i := range files {
		files[i] = writeAssetsTestPNG(t, sourceRoot, fmt.Sprintf("%02d.png", i+1))
	}
	size := fileSize(t, files[0])
	var mu sync.Mutex
	var created []string

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read create screenshot body: %v", err)
			}
			var payload struct {
				Data struct {
					Attributes struct {
						FileName string `json:"fileName"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode create screenshot body: %v", err)
			}
			name := payload.Data.Attributes.FileName
			// App Store Connect appends screenshots in creation order, so a slow first
			// reservation must not let later files claim earlier positions.
			if name == "01.png" {
				synctest.Wait()
			}
			mu.Lock()
			created = append(created, name)
			mu.Unlock()
			return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/%s","length":%d,"offset":0}]}}}`, name, name, size))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`, id))
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() { http.DefaultTransport = origTransport })

	uploads := map[string]func(context.Context, *asc.Client) error{
		"upload": func(ctx context.Context, client *asc.Client) error {
			_, err := uploadScreenshotsWithOrderStateWithOpenedFiles(withScreenshotUploadConcurrency(ctx, 4), nil, shared.ContextWithTimeout, client, "set", nil, files, sourceRoot, false, false, nil)
			return err
		},
		// migrate rebuilds each upload context from a parent without the reservation turn.
		"migrate": func(ctx context.Context, client *asc.Client) error {
			_, _, err := UploadScreenshotAssetsConcurrently(ctx, shared.DefaultDeps(), client, "set", files, func(context.Context) (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			}, func(string) (*os.File, error) { return nil, nil })
			return err
		},
	}
	for name, upload := range uploads {
		t.Run(name, func(t *testing.T) {
			created = nil
			var err error
			synctest.Test(t, func(t *testing.T) { err = upload(context.Background(), newAssetsUploadTestClient(t)) })
			if err != nil {
				t.Fatal(err)
			}
			if want := []string{"01.png", "02.png", "03.png", "04.png"}; !reflect.DeepEqual(created, want) {
				t.Fatalf("creation order = %v, want %v", created, want)
			}
		})
	}
}
