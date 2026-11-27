package asc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCreativeVideoPlacementReadinessPreservesStandaloneUploads(t *testing.T) {
	for _, tc := range []struct {
		name                                                   string
		placement                                              bool
		category, assetField, frameState, frameCode, wantError string
		polls                                                  int
	}{
		{name: "standalone waits for URI", assetField: `,"videoAsset":null`, polls: 2},
		{name: "creative explicit null", placement: true, assetField: `,"videoAsset":null`, polls: 1},
		{name: "creative missing field waits", placement: true, polls: 2},
		{name: "creative empty string waits", placement: true, assetField: `,"videoAsset":""`, polls: 2},
		{name: "creative poster processing waits", placement: true, assetField: `,"videoAsset":null`, frameState: "PROCESSING", polls: 2},
		{name: "native preview keeps URI readiness", placement: true, category: "APP_SCREENSHOTS_AND_PREVIEWS", assetField: `,"videoAsset":null`, polls: 2},
		{name: "missing poster identity", placement: true, assetField: `,"videoAsset":null`, frameCode: "missing", wantError: "poster identity is unavailable", polls: 1},
		{name: "failed poster", placement: true, assetField: `,"videoAsset":null`, frameState: "FAILED", wantError: "preview frame reached FAILED", polls: 1},
		{name: "different requested poster", placement: true, assetField: `,"videoAsset":null`, frameCode: "00:00:01:00", wantError: "poster differs", polls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(SetMediaPollIntervalsForTest(time.Nanosecond, time.Nanosecond))
			var transfers atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, _ := io.ReadAll(r.Body)
				if r.Method != "PUT" || string(data) != "abc" {
					t.Errorf("transfer %s %q", r.Method, data)
				}
				transfers.Add(1)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			resource := func(asset, state, code string) string {
				if state == "" {
					state = "COMPLETE"
				}
				if code == "" {
					code = "00:00:00:00"
				}
				codeField := fmt.Sprintf(`,"previewFrameTimeCode":%q`, code)
				if code == "missing" {
					codeField = ""
				}
				return fmt.Sprintf(`{"data":{"type":"appAssetLibraryVideos","id":"video1","attributes":{"state":"PREPARE_FOR_SUBMISSION","specId":"video-spec"%s%s,"previewFrameImage":{"state":{"state":%q},"image":{"width":176,"height":144}}}}}`, asset, codeField, state)
			}
			response := func(status int, body string) *http.Response {
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
			}
			calls := []string{}
			client := newTestClient(t, func(r *http.Request) { calls = append(calls, r.Method+" "+r.URL.Path) },
				response(201, fmt.Sprintf(`{"data":{"type":"appAssetLibraryVideos","id":"video1","attributes":{"uploadOperations":[{"method":"PUT","url":%q,"length":3,"offset":0}]}}}`, server.URL)),
				response(200, `{"data":{"id":"video1"}}`),
				response(200, resource(tc.assetField, tc.frameState, tc.frameCode)),
				response(200, resource(`,"videoAsset":"https://example.invalid/video.mp4"`, "COMPLETE", "00:00:00:00")))
			ctx := context.Background()
			if tc.placement {
				ctx = WithCreativeVideoPlacementReadiness(ctx)
			}
			category := tc.category
			if category == "" {
				category = "CREATIVE_ASSETS"
			}
			file := createTempAssetFile(t, []byte("abc"))
			defer file.Close()
			result, err := client.UploadAssetLibraryVideoWithCategory(ctx, io.Discard, "library", "clip.mp4", file, 3, category, "", "00:00:00:00", nil)
			if (tc.wantError == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("err=%v want=%q", err, tc.wantError)
			}
			if result.VideoID != "video1" || !result.Uploaded || result.Ready != (err == nil) || len(calls) != 2+tc.polls || transfers.Load() != 1 {
				t.Fatalf("result=%+v calls=%v transfers=%d", result, calls, transfers.Load())
			}
		})
	}
}
