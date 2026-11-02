package cmdtest

import (
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
	const failedFrame = "poster frame failed: FRAME: frame is out of range"
	const unfinishedFrame = "timed out waiting for preview preview-1 poster frame"
	tests := []struct {
		name          string
		timeCode      string
		frameStates   []string
		uploadTimeout string
		replace       bool
		wantFailures  []string
	}{
		{name: "omitted flag sends nothing and ignores frame state", frameStates: []string{"PROCESSING"}, uploadTimeout: "3s"},
		{name: "requested frame is sent and recorded", timeCode: "00:00:05:00", frameStates: []string{"COMPLETE"}, uploadTimeout: "3s"},
		{name: "failed frame keeps the delivered preview", timeCode: "00:00:05:00", frameStates: []string{"FAILED"}, uploadTimeout: "3s", wantFailures: []string{failedFrame}},
		{name: "unfinished frame keeps the delivered preview", timeCode: "00:00:05:00", frameStates: []string{"PROCESSING"}, uploadTimeout: "100ms", wantFailures: []string{unfinishedFrame}},
		{name: "failed frame after replace keeps the new preview", timeCode: "00:00:05:00", frameStates: []string{"FAILED"}, uploadTimeout: "3s", replace: true, wantFailures: []string{failedFrame}},
		{name: "unfinished frame does not hide a later file's failure", timeCode: "00:00:05:00", frameStates: []string{"PROCESSING", "FAILED"}, uploadTimeout: "100ms", wantFailures: []string{unfinishedFrame, "preview preview-2 " + failedFrame}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_UPLOAD_TIMEOUT", test.uploadTimeout)

			dir := t.TempDir()
			sizes := make([]int64, len(test.frameStates))
			linkages := make([]string, len(test.frameStates))
			for i := range test.frameStates {
				sizes[i] = writePreviewFile(t, filepath.Join(dir, fmt.Sprintf("%02d.mov", i+1)))
				linkages[i] = fmt.Sprintf(`{"type":"appPreviews","id":"preview-%d"}`, i+1)
			}
			existing := ""
			if test.replace {
				existing = `{"type":"appPreviews","id":"old-1","attributes":{"fileName":"old.mov"}}`
			}

			var created []map[string]any
			var deleted []string
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appPreviewSets":
					return statusJSONResponse(`{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}],"links":{}}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviewSets/set-1/appPreviews":
					return statusJSONResponse(`{"data":[` + existing + `],"links":{}}`), nil
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
				case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appPreviews/"):
					return statusJSONResponse(`{"data":{"type":"appPreviews","id":"preview","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`), nil
				case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appPreviews/preview-"):
					index, err := strconv.Atoi(strings.TrimPrefix(req.URL.Path, "/v1/appPreviews/preview-"))
					if err != nil {
						t.Fatalf("preview path %s: %v", req.URL.Path, err)
					}
					return statusJSONResponse(fmt.Sprintf(
						`{"data":{"type":"appPreviews","id":"preview-%d","attributes":{"assetDeliveryState":{"state":"COMPLETE"},"previewFrameImage":{"state":{"state":%q,"errors":[{"code":"FRAME","description":"frame is out of range"}]}}}}}`,
						index, test.frameStates[index-1],
					)), nil
				case req.Method == http.MethodDelete:
					deleted = append(deleted, strings.TrimPrefix(req.URL.Path, "/v1/appPreviews/"))
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
			wantDeleted := []string(nil)
			if test.replace {
				args = append(args, "--replace", "--confirm")
				wantDeleted = []string{"old-1"}
			}
			stdout, stderr, err := runRootCommand(t, args)

			if !reflect.DeepEqual(deleted, wantDeleted) {
				t.Fatalf("deleted = %v, want %v", deleted, wantDeleted)
			}
			for _, attributes := range created {
				got, sent := attributes["previewFrameTimeCode"]
				if sent != (test.timeCode != "") || (sent && got != test.timeCode) {
					t.Fatalf("create previewFrameTimeCode = %v, want %q", got, test.timeCode)
				}
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
			if len(receipt.Results) != len(test.frameStates) {
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
