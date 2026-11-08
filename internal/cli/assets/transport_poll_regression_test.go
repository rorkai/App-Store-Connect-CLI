package assets

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"testing"
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
