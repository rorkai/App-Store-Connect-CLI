package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestVideoPreviewsUploadReportsUploadedPreviewsWhenALaterFileFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	pathDir := t.TempDir()
	firstSize := writePreviewFile(t, filepath.Join(pathDir, "01-first.mov"))
	secondSize := writePreviewFile(t, filepath.Join(pathDir, "02-second.mov"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appPreviewSets":
			return statusJSONResponse(`{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviewSets/set-1/appPreviews":
			return statusJSONResponse(`{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appPreviews":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read create preview body: %v", err)
			}
			if strings.Contains(string(body), "02-second.mov") {
				return statusJSONResponse(fmt.Sprintf(
					`{"data":{"type":"appPreviews","id":"preview-second","attributes":{"fileName":"02-second.mov","assetDeliveryState":{"state":"AWAITING_UPLOAD"},"uploadOperations":[{"method":"PUT","url":"https://upload.example/preview-second","length":%d,"offset":0}]}}}`,
					secondSize,
				)), nil
			}
			return statusJSONResponse(fmt.Sprintf(
				`{"data":{"type":"appPreviews","id":"preview-first","attributes":{"fileName":"01-first.mov","uploadOperations":[{"method":"PUT","url":"https://upload.example/preview-first","length":%d,"offset":0}]}}}`,
				firstSize,
			)), nil
		case req.Method == http.MethodPut && req.URL.Host == "upload.example" && req.URL.Path == "/preview-second":
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("preview upload failed")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appPreviews/preview-first":
			return statusJSONResponse(`{"data":{"type":"appPreviews","id":"preview-first","attributes":{"uploaded":true}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviews/preview-first":
			return statusJSONResponse(`{"data":{"type":"appPreviews","id":"preview-first","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`), nil
		case req.Method == http.MethodDelete && (req.URL.Path == "/v1/appPreviews/preview-first" || req.URL.Path == "/v1/appPreviews/preview-second"):
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	exitCode := rootcmd.ExitSuccess
	stdout, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run([]string{
			"video-previews", "upload",
			"--version-localization", "LOC_123",
			"--path", pathDir,
			"--device-type", "IPHONE_65",
			"--output", "json",
		}, "1.2.3")
	})

	if exitCode == rootcmd.ExitSuccess {
		t.Fatal("expected a non-zero exit code when a preview upload fails")
	}
	if !strings.Contains(stderr, "video-previews upload") {
		t.Fatalf("expected the failure to be attributed to video-previews upload, got stderr=%q", stderr)
	}

	var payload struct {
		SetID   string `json:"setId"`
		Results []struct {
			FileName string `json:"fileName"`
			AssetID  string `json:"assetId"`
			State    string `json:"state"`
		} `json:"results"`
		Failures []struct {
			FileName string `json:"fileName"`
			FilePath string `json:"filePath"`
			Error    string `json:"error"`
		} `json:"failures"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode partial receipt: %v\nstdout=%s", err, stdout)
	}
	if payload.SetID != "set-1" {
		t.Fatalf("setId = %q, want set-1", payload.SetID)
	}

	uploaded := make(map[string]string, len(payload.Results))
	for _, item := range payload.Results {
		uploaded[item.FileName] = item.AssetID
	}
	if uploaded["01-first.mov"] != "preview-first" {
		t.Fatalf("expected the already uploaded preview in the receipt, got %s", stdout)
	}

	if len(payload.Failures) != 1 {
		t.Fatalf("expected exactly one failure entry, got %s", stdout)
	}
	failure := payload.Failures[0]
	if failure.FileName != "02-second.mov" {
		t.Fatalf("failure fileName = %q, want 02-second.mov", failure.FileName)
	}
	if !strings.Contains(failure.FilePath, "02-second.mov") {
		t.Fatalf("failure filePath = %q, want the failing preview path", failure.FilePath)
	}
	if strings.TrimSpace(failure.Error) == "" {
		t.Fatalf("expected a failure message, got %s", stdout)
	}

	rolledBackStates := 0
	for _, item := range payload.Results {
		if (item.AssetID == "preview-first" || item.AssetID == "preview-second") && item.State == "rolled-back" {
			rolledBackStates++
		}
	}
	if rolledBackStates != 2 {
		t.Fatalf("expected both receipt items to be marked rolled-back after cleanup, got %s", stdout)
	}
}

func TestVideoPreviewsUploadPosterFrame(t *testing.T) {
	const appleDefault = "00:00:05:01"
	const timeCode = "00:00:07:00"
	tests := []struct {
		name         string
		timeCode     string
		outcomes     []string // per file: "ok", "error" (PATCH 409), "stale" (PATCH and read-back keep Apple's default), "late" (read-back matches after a stale PATCH echo)
		video        []string // videoDeliveryState on each file's successive GETs, the first being the delivery poll; the last repeats
		cancelOnGet  int      // cancel the run on this GET of the first file
		wantFailures []string
	}{
		{name: "omitted flag sends no poster frame", outcomes: []string{"ok"}},
		{name: "requested frame waits for video processing", timeCode: timeCode, outcomes: []string{"ok"}, video: []string{"PROCESSING", "PROCESSING", "COMPLETE"}},
		{name: "failed video processing keeps the preview", timeCode: timeCode, outcomes: []string{"ok"}, video: []string{"PROCESSING", "FAILED"}, wantFailures: []string{"VIDEO_ERROR: video could not be processed"}},
		{name: "unfinished video processing keeps the preview", timeCode: timeCode, outcomes: []string{"ok"}, video: []string{"PROCESSING"}, cancelOnGet: 2, wantFailures: []string{"timed out"}},
		{name: "read-back match after a stale echo succeeds", timeCode: timeCode, outcomes: []string{"late"}},
		{name: "patch failure keeps the preview and reports the retry", timeCode: timeCode, outcomes: []string{"error"}, wantFailures: []string{"poster frame not changed"}},
		{name: "read-back mismatch is a failure", timeCode: timeCode, outcomes: []string{"stale"}, wantFailures: []string{`read back "` + appleDefault + `"`}},
		{name: "one file's failure does not hide or stop another's", timeCode: timeCode, outcomes: []string{"error", "stale"}, wantFailures: []string{"poster frame not changed", `read back "` + appleDefault + `"`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")

			dir := t.TempDir()
			sizes := make([]int64, len(test.outcomes))
			linkages := make([]string, len(test.outcomes))
			readBack := make([]string, len(test.outcomes))
			gets := make([]int, len(test.outcomes))
			lastVideo := make([]string, len(test.outcomes))
			video := test.video
			if video == nil {
				video = []string{"PROCESSING", "COMPLETE"}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			for i := range test.outcomes {
				sizes[i] = writePreviewFile(t, filepath.Join(dir, fmt.Sprintf("%02d.mov", i+1)))
				linkages[i] = fmt.Sprintf(`{"type":"appPreviews","id":"preview-%d"}`, i+1)
				readBack[i] = appleDefault
			}

			var created []map[string]any
			var framePatches []string
			var deleted []string
			previewIndex := func(path string) int {
				index, err := strconv.Atoi(strings.TrimPrefix(path, "/v1/appPreviews/preview-"))
				if err != nil {
					t.Fatalf("preview path %s: %v", path, err)
				}
				return index - 1
			}
			previewJSON := func(i int, frame string) *http.Response {
				return statusJSONResponse(fmt.Sprintf(`{"data":{"type":"appPreviews","id":"preview-%d","attributes":{"assetDeliveryState":{"state":"COMPLETE"},"videoDeliveryState":{"state":%q,"errors":[{"code":"VIDEO_ERROR","description":"video could not be processed"}]},"previewFrameTimeCode":%q}}}`, i+1, lastVideo[i], frame))
			}
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appPreviewSets":
					return statusJSONResponse(`{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}],"links":{}}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviewSets/set-1/appPreviews":
					return statusJSONResponse(`{"data":[],"links":{}}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviewSets/set-1/relationships/appPreviews":
					return statusJSONResponse(`{"data":[` + strings.Join(linkages, ",") + `],"links":{}}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/appPreviews":
					var body struct {
						Data struct {
							Attributes map[string]any `json:"attributes"`
						} `json:"data"`
					}
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatalf("decode create preview body: %v", err)
					}
					created = append(created, body.Data.Attributes)
					id := len(created)
					return statusJSONResponse(fmt.Sprintf(
						`{"data":{"type":"appPreviews","id":"preview-%d","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/preview-%d","length":%d,"offset":0}]}}}`,
						id, id, sizes[id-1],
					)), nil
				case req.Method == http.MethodPut && req.URL.Host == "upload.example":
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
				case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appPreviews/preview-"):
					i := previewIndex(req.URL.Path)
					var body struct {
						Data struct {
							Attributes map[string]any `json:"attributes"`
						} `json:"data"`
					}
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatalf("decode patch preview body: %v", err)
					}
					frame, isFramePatch := body.Data.Attributes["previewFrameTimeCode"]
					if !isFramePatch {
						return statusJSONResponse(`{"data":{"type":"appPreviews","id":"preview","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`), nil
					}
					if lastVideo[i] != "COMPLETE" {
						t.Fatalf("poster frame patched before a GET showed preview-%d videoDeliveryState COMPLETE", i+1)
					}
					framePatches = append(framePatches, fmt.Sprintf("preview-%d=%v", i+1, frame))
					switch test.outcomes[i] {
					case "error":
						return jsonHTTPResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR","title":"poster frame not changed","detail":"poster frame not changed"}]}`), nil
					case "ok":
						readBack[i] = timeCode
					case "late":
						readBack[i] = timeCode
						return previewJSON(i, appleDefault), nil
					}
					return previewJSON(i, readBack[i]), nil
				case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appPreviews/preview-"):
					i := previewIndex(req.URL.Path)
					lastVideo[i] = video[min(gets[i], len(video)-1)]
					gets[i]++
					if i == 0 && gets[i] == test.cancelOnGet {
						cancel()
					}
					return previewJSON(i, readBack[i]), nil
				case req.Method == http.MethodDelete:
					deleted = append(deleted, req.URL.Path)
					return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})

			args := []string{"video-previews", "upload", "--version-localization", "LOC_123", "--path", dir, "--device-type", "IPHONE_65", "--output", "json"}
			if test.timeCode != "" {
				args = append(args, "--time-code", test.timeCode)
			}
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var err error
			stdout, stderr := captureOutput(t, func() {
				if parseErr := root.Parse(args); parseErr != nil {
					t.Fatalf("parse error: %v", parseErr)
				}
				err = root.Run(ctx)
			})

			if len(deleted) != 0 {
				t.Fatalf("deleted delivered previews: %v", deleted)
			}
			for _, attributes := range created {
				if got, sent := attributes["previewFrameTimeCode"]; sent {
					t.Fatalf("create sent previewFrameTimeCode %v; Apple ignores it on POST", got)
				}
			}
			var wantPatches []string
			if test.timeCode != "" && video[len(video)-1] == "COMPLETE" {
				for i := range test.outcomes {
					wantPatches = append(wantPatches, fmt.Sprintf("preview-%d=%s", i+1, test.timeCode))
				}
			}
			if !reflect.DeepEqual(framePatches, wantPatches) {
				t.Fatalf("poster frame patches = %v, want %v", framePatches, wantPatches)
			}
			var receipt struct {
				PreviewFrameTimeCode string `json:"previewFrameTimeCode"`
				Results              []struct {
					AssetID string `json:"assetId"`
					State   string `json:"state"`
				} `json:"results"`
				Failures []struct {
					FileName string `json:"fileName"`
					Error    string `json:"error"`
				} `json:"failures"`
			}
			if decodeErr := json.Unmarshal([]byte(stdout), &receipt); decodeErr != nil {
				t.Fatalf("decode receipt: %v\nstdout=%s\nstderr=%s\nerr=%v", decodeErr, stdout, stderr, err)
			}
			if receipt.PreviewFrameTimeCode != test.timeCode {
				t.Fatalf("receipt previewFrameTimeCode = %q, want %q: %s", receipt.PreviewFrameTimeCode, test.timeCode, stdout)
			}
			if len(receipt.Results) != len(test.outcomes) {
				t.Fatalf("receipt results: %s", stdout)
			}
			for i, item := range receipt.Results {
				if item.AssetID != fmt.Sprintf("preview-%d", i+1) || item.State != "COMPLETE" {
					t.Fatalf("receipt results: %s", stdout)
				}
			}
			if len(receipt.Failures) != len(test.wantFailures) || (err != nil) != (len(test.wantFailures) > 0) {
				t.Fatalf("err = %v, stderr = %q, stdout = %s", err, stderr, stdout)
			}
			for i, want := range test.wantFailures {
				retry := fmt.Sprintf("asc video-previews set-poster-frame --id preview-%d --time-code %s", i+1, test.timeCode)
				failure := receipt.Failures[i]
				if failure.FileName != fmt.Sprintf("%02d.mov", i+1) || !strings.Contains(failure.Error, want) || !strings.Contains(failure.Error, retry) || !strings.Contains(err.Error(), want) {
					t.Fatalf("failure %d = %+v, err = %v, want %q and %q", i, failure, err, want, retry)
				}
			}
		})
	}
}
