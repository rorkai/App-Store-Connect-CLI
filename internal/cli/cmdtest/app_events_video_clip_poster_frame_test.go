package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// Catches a malformed time code reaching Apple, or leaving a half-created clip
// behind, because validation ran after the first request.
func TestAppEventVideoClipsTimeCodeUsageErrorsBeforeHTTP(t *testing.T) {
	create := []string{"app-events", "video-clips", "create", "--localization-id", "LOC_1", "--path", "./clip.mov", "--asset-type", "EVENT_CARD"}
	setPosterFrame := []string{"app-events", "video-clips", "set-poster-frame", "--clip-id", "clip-1"}

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"create rejects a malformed value on the older flag", append(create[:9:9], "--preview-frame-time-code", "5s"), "--preview-frame-time-code must be in HH:MM:SS:FF or HH:MM:SS.mmm format"},
		{"create does not ignore an explicitly empty time code", append(create[:9:9], "--time-code", " "), "--time-code must be in HH:MM:SS:FF or HH:MM:SS.mmm format"},
		{"create rejects both spellings", append(create[:9:9], "--time-code", "00:00:05:00", "--preview-frame-time-code", "00:00:06:00"), "--time-code and --preview-frame-time-code are mutually exclusive"},
		{"set-poster-frame rejects a malformed time code", append(setPosterFrame[:5:5], "--time-code", "00:00:60:00"), "--time-code must be in HH:MM:SS:FF or HH:MM:SS.mmm format"},
		{"set-poster-frame requires a time code", setPosterFrame, "--time-code is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				return nil, nil
			}))

			exitCode := rootcmd.ExitSuccess
			_, stderr := captureOutput(t, func() {
				exitCode = rootcmd.Run(test.args, "1.2.3")
			})

			if exitCode != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr=%q", exitCode, rootcmd.ExitUsage, stderr)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantErr)
			}
		})
	}
}

// Catches either spelling not reaching the POST, and an omitted flag sending a
// time code. Every status read reports no frame state, and a second read fails
// the test, so it also catches create polling for a frame.
func TestAppEventVideoClipsCreateTimeCodeRequestWiring(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		wantCode any
	}{
		{"time code", []string{"--time-code", " 00:00:05:00 "}, "00:00:05:00"},
		{"older spelling", []string{"--preview-frame-time-code", "00:00:05.000"}, "00:00:05.000"},
		{"omitted", nil, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			clipPath := filepath.Join(t.TempDir(), "clip.mov")
			if err := os.WriteFile(clipPath, []byte("clip-bytes"), 0o600); err != nil {
				t.Fatalf("write clip: %v", err)
			}

			var createAttributes map[string]any
			gets := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPost && req.URL.Path == "/v1/appEventVideoClips":
					var payload struct {
						Data struct {
							Attributes map[string]any `json:"attributes"`
						} `json:"data"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode create body: %v", err)
					}
					createAttributes = payload.Data.Attributes
					return statusJSONResponse(`{"data":{"type":"appEventVideoClips","id":"clip-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/clip-1","length":10,"offset":0}]}}}`), nil
				case req.Method == http.MethodPut && req.URL.Host == "upload.example":
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
				case req.Method == http.MethodPatch && req.URL.Path == "/v1/appEventVideoClips/clip-1":
					return statusJSONResponse(`{"data":{"type":"appEventVideoClips","id":"clip-1"}}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appEventVideoClips/clip-1":
					gets++
					if gets > 1 {
						t.Fatalf("polled again after delivery")
					}
					return statusJSONResponse(`{"data":{"type":"appEventVideoClips","id":"clip-1","attributes":{"videoDeliveryState":{"state":"COMPLETE"}}}}`), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}))

			args := append([]string{
				"app-events", "video-clips", "create",
				"--localization-id", "LOC_1",
				"--path", clipPath,
				"--asset-type", "EVENT_CARD",
				"--output", "json",
			}, test.flags...)
			exitCode := rootcmd.ExitError
			_, stderr := captureOutput(t, func() {
				exitCode = rootcmd.Run(args, "1.2.3")
			})

			if exitCode != rootcmd.ExitSuccess {
				t.Fatalf("exit code = %d, stderr=%q", exitCode, stderr)
			}
			if got := createAttributes["previewFrameTimeCode"]; got != test.wantCode {
				t.Fatalf("previewFrameTimeCode = %v, want %v", got, test.wantCode)
			}
		})
	}
}

// Catches set-poster-frame patching the wrong clip, or sending `uploaded` and
// re-committing an upload the user only wanted to retime.
func TestAppEventVideoClipsSetPosterFramePatchesOnlyTheTimeCode(t *testing.T) {
	setupAuth(t)

	var gotBody string
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/appEventVideoClips/clip-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		gotBody = strings.TrimSpace(string(body))
		return statusJSONResponse(`{"data":{"type":"appEventVideoClips","id":"clip-1"}}`), nil
	}))

	exitCode := rootcmd.ExitError
	_, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run([]string{
			"app-events", "video-clips", "set-poster-frame",
			"--clip-id", "clip-1",
			"--time-code", "00:00:05:00",
			"--output", "json",
		}, "1.2.3")
	})

	if exitCode != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, stderr=%q", exitCode, stderr)
	}
	const wantBody = `{"data":{"type":"appEventVideoClips","id":"clip-1","attributes":{"previewFrameTimeCode":"00:00:05:00"}}}`
	if gotBody != wantBody {
		t.Fatalf("body = %s, want %s", gotBody, wantBody)
	}
}
