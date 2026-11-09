package assets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"testing"
	"testing/iotest"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestScreenshotUploadSurvivesTransientSettlementRead(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")
	t.Cleanup(asc.SetMediaPollIntervalsForTest(time.Millisecond, time.Millisecond))
	root := t.TempDir()
	path := writeAssetsTestPNG(t, root, "01-home.png")
	size := fileSize(t, path)
	reads := 0
	var writes []string
	transport := assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			writes = append(writes, req.Method)
		}
		switch req.Method + " " + req.URL.Path {
		case "POST /v1/appScreenshots":
			return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"screenshot-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/upload","length":%d,"offset":0}]}}}`, size))
		case "PUT /upload":
			return assetsJSONResponse(http.StatusOK, "{}")
		case "PATCH /v1/appScreenshots/screenshot-1":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"screenshot-1","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`)
		case "GET /v1/appScreenshots/screenshot-1":
			reads++
			if reads == 1 {
				return nil, io.ErrUnexpectedEOF
			}
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"screenshot-1","attributes":{"sourceFileChecksum":"committed","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	client := newAssetsUploadTestClient(t)
	synctest.Test(t, func(t *testing.T) {
		result, pending, err := uploadScreenshotAsset(context.Background(), client, "set-1", root, path)
		if err != nil || reads != 2 || result.AssetID != "screenshot-1" || result.State != "COMPLETE" || pending.AssetID != "" {
			t.Fatalf("result=%+v pending=%+v reads=%d err=%v, want completed upload after two reads", result, pending, reads, err)
		}
		if !slices.Equal(writes, []string{http.MethodPost, http.MethodPut, http.MethodPatch}) {
			t.Fatalf("write methods = %v, want one reserve, upload, and commit", writes)
		}
	})
}

func TestScreenshotUploadRecoversLostCommitResponse(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")
	root := t.TempDir()
	path := writeAssetsTestPNG(t, root, "01-home.png")
	size := fileSize(t, path)
	checksum, err := computeFileChecksumInRoot(root, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, id, state, checksum string
		noWait, resume            bool
		wantState                 string
	}{
		{name: "confirmed", id: "screenshot-1", state: "COMPLETE", checksum: checksum, wantState: "COMPLETE"},
		{name: "no wait", id: "screenshot-1", state: "UPLOAD_COMPLETE", noWait: true, wantState: "UPLOAD_COMPLETE"},
		{name: "resume", id: "screenshot-1", state: "COMPLETE", checksum: checksum, resume: true, wantState: "COMPLETE"},
		{name: "different checksum", id: "screenshot-1", state: "COMPLETE", checksum: "different"},
		{name: "different asset", id: "screenshot-2", state: "COMPLETE", checksum: checksum},
		{name: "not committed", id: "screenshot-1", state: "AWAITING_UPLOAD", checksum: checksum},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var methods []string
			transport := assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				methods = append(methods, req.Method)
				switch req.Method + " " + req.URL.Path {
				case "POST /v1/appScreenshots":
					return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"screenshot-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/upload","length":%d,"offset":0}]}}}`, size))
				case "PUT /upload":
					return assetsJSONResponse(http.StatusOK, "{}")
				case "PATCH /v1/appScreenshots/screenshot-1":
					resp, _ := assetsJSONResponse(http.StatusOK, "")
					resp.Body = io.NopCloser(iotest.ErrReader(io.ErrUnexpectedEOF))
					return resp, nil
				case "GET /v1/appScreenshots/screenshot-1":
					if tc.resume && len(methods) == 1 {
						return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"screenshot-1","attributes":{"assetDeliveryState":{"state":"AWAITING_UPLOAD"}}}}`)
					}
					return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":%q,"attributes":{"sourceFileChecksum":%q,"assetDeliveryState":{"state":%q}}}}`, tc.id, tc.checksum, tc.state))
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
			})
			previous := http.DefaultTransport
			http.DefaultTransport = transport
			t.Cleanup(func() { http.DefaultTransport = previous })

			ctx := withScreenshotNoWait(context.Background(), tc.noWait)
			client := newAssetsUploadTestClient(t)
			var result asc.AssetUploadResultItem
			var pending screenshotPendingAsset
			var err error
			wantMethods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodGet}
			if tc.resume {
				wantMethods = []string{http.MethodGet, http.MethodPatch, http.MethodGet}
				var retry bool
				result, pending, retry, err = reconcilePendingScreenshotAsset(ctx, client, screenshotPendingAsset{
					AssetID: "screenshot-1", FileName: "01-home.png", FilePath: path, Checksum: checksum, State: "UPLOADED",
				}, root)
				if retry {
					t.Fatal("resume tried to reserve a new asset")
				}
			} else {
				result, pending, err = uploadScreenshotAsset(ctx, client, "set-1", root, path)
			}
			if tc.wantState != "" {
				if err != nil || pending.AssetID != "" || result.AssetID != "screenshot-1" || result.State != tc.wantState {
					t.Fatalf("result=%+v pending=%+v err=%v, want completed upload", result, pending, err)
				}
			} else if !errors.Is(err, io.ErrUnexpectedEOF) || pending.AssetID != "screenshot-1" || pending.State != "UPLOADED" {
				t.Fatalf("pending=%+v err=%v, want original commit error and resumable upload", pending, err)
			}
			if !slices.Equal(methods, wantMethods) {
				t.Fatalf("methods = %v, want %v", methods, wantMethods)
			}
		})
	}
}
