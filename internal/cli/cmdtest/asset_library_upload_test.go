package cmdtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/errfmt"
)

func creativeUploadFile(t *testing.T) (string, []byte) {
	t.Helper()
	var data bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "creative.png")
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, data.Bytes()
}

func TestAssetLibraryImageUpload(t *testing.T) {
	for _, mode := range []string{"success", "progress", "standalone", "transient-processing", "table", "invalid-reservation", "upload-failure", "commit-failure", "processing-failed", "processing-timeout", "reference-name"} {
		t.Run(mode, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_UPLOAD_TIMEOUT", "5s")
			if mode == "processing-timeout" {
				t.Setenv("ASC_UPLOAD_TIMEOUT", "100ms")
			}
			if mode == "processing-failed" {
				t.Setenv("ASC_UPLOAD_TIMEOUT", "1s")
			}
			category := "CREATIVE_ASSETS"
			if mode == "standalone" {
				category = "APP_SCREENSHOTS_AND_PREVIEWS"
			}
			path, content := creativeUploadFile(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			calls, reads, uploaded := 0, 0, 0
			var mu sync.Mutex
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				mu.Lock()
				defer mu.Unlock()
				calls++
				switch {
				case req.Method == "POST" && req.URL.Path == "/v1/appAssetLibraryImages":
					var body struct {
						Data struct {
							Type       string
							Attributes struct {
								FileName      string
								FileSize      int64
								Category      string
								ReferenceName *string
							}
							Relationships struct {
								AssetLibrary struct{ Data struct{ Type, ID string } }
							}
						}
					}
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if body.Data.Type != "appAssetLibraryImages" || body.Data.Attributes.FileName != "creative.png" || body.Data.Attributes.FileSize != int64(len(content)) || body.Data.Attributes.Category != category || body.Data.Relationships.AssetLibrary.Data.Type != "appAssetLibraries" || body.Data.Relationships.AssetLibrary.Data.ID != "lib" {
						t.Fatalf("wrong reservation: %+v", body)
					}
					if got := body.Data.Attributes.ReferenceName; (mode == "reference-name") != (got != nil) || (got != nil && *got != imageReferenceName) {
						t.Fatalf("wrong reservation referenceName: %v", got)
					}
					if mode == "invalid-reservation" {
						return jsonResponse(201, `{"data":{"type":"appAssetLibraryImages","id":"image","attributes":{"uploadOperations":"invalid"}}}`)
					}
					return jsonResponse(201, `{"data":{"type":"appAssetLibraryImages","id":"image","attributes":{"state":"AWAITING_UPLOAD","uploadOperations":[{"method":"PUT","url":"https://storage.example/first?token=secret-signed","offset":0,"length":`+jsonNumber(len(content)/2)+`,"requestHeaders":[{"name":"X-Part","value":"first"}]},{"method":"PUT","url":"https://storage.example/second","offset":`+jsonNumber(len(content)/2)+`,"length":`+jsonNumber(len(content)-len(content)/2)+`,"requestHeaders":[{"name":"X-Part","value":"second"}]}]}}}`)
				case req.Method == "PUT" && req.URL.Host == "storage.example":
					if req.Header.Get("Authorization") != "" {
						t.Fatal("ASC token sent to storage")
					}
					if mode == "upload-failure" {
						return jsonResponse(400, `{"error":"bad part"}`)
					}
					got, _ := io.ReadAll(req.Body)
					want := content[:len(content)/2]
					part := "first"
					if req.URL.Path == "/second" {
						want = content[len(content)/2:]
						part = "second"
					}
					if !bytes.Equal(got, want) || req.Header.Get("X-Part") != part {
						t.Fatal("wrong part bytes/header")
					}
					uploaded++
					return jsonResponse(200, "")
				case req.Method == "PATCH" && req.URL.Path == "/v1/appAssetLibraryImages/image":
					if mode == "upload-failure" {
						t.Fatal("upload failure committed the image")
					}
					var body map[string]any
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					expected := `{"data":{"type":"appAssetLibraryImages","id":"image","attributes":{"uploaded":true}}}`
					got, _ := json.Marshal(body)
					var expectedBody any
					_ = json.Unmarshal([]byte(expected), &expectedBody)
					want, _ := json.Marshal(expectedBody)
					if !bytes.Equal(got, want) {
						t.Fatalf("wrong commit: %s", got)
					}
					if mode == "commit-failure" {
						return jsonResponse(409, `{"errors":[{"status":"409","code":"ENTITY_ERROR","detail":"cannot commit"}]}`)
					}
					return jsonResponse(200, `{"data":{"type":"appAssetLibraryImages","id":"image","attributes":{"imageAsset":null}}}`)
				case req.Method == "GET" && req.URL.Path == "/v1/appAssetLibraryImages/image":
					reads++
					if mode == "processing-failed" {
						return jsonResponse(200, `{"data":{"type":"appAssetLibraryImages","id":"image","attributes":{"state":"FAILED","imageAsset":null,"stateDetails":[{"code":"INVALID_ASSET_FILE_FORMAT","description":"INVALID_ASSET_FILE_FORMAT"}]}}}`)
					}
					if mode == "transient-processing" && reads == 1 {
						return nil, context.DeadlineExceeded
					}
					if mode == "processing-timeout" {
						return jsonResponse(200, `{"data":{"type":"appAssetLibraryImages","id":"image","attributes":{"state":"FUTURE_PROCESSING","imageAsset":null}}}`)
					}
					if reads == 1 {
						return jsonResponse(200, `{"data":{"type":"appAssetLibraryImages","id":"image","attributes":{"state":"PROCESSING","imageAsset":null}}}`)
					}
					return jsonResponse(200, `{"data":{"type":"appAssetLibraryImages","id":"image","attributes":{"state":"PREPARE_FOR_SUBMISSION","specId":"spec","imageAsset":{"width":2,"height":2}}}}`)
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Redacted())
					return nil, nil
				}
			})
			output := "json"
			if mode == "table" {
				output = "table"
			}
			args := []string{"asset-library", "images", "upload", "--library-id", "lib", "--file", path, "--output", output, "--category", category}
			if mode == "reference-name" {
				args = append(args, "--reference-name", " "+imageReferenceName+" ")
			}
			if mode == "progress" {
				args = append(args, "--progress")
			}
			stdout, stderr, err := runAssetLibrary(t, args...)
			if mode == "progress" {
				lines := strings.Split(strings.TrimSpace(stderr), "\n")
				if len(lines) != 9 {
					t.Fatalf("progress lines = %d: %q", len(lines), stderr)
				}
				for i, line := range lines {
					var event struct {
						Event, Stage, Status   string
						DurationMs, RetryCount int
						HTTPStatus             int
					}
					if err := json.Unmarshal([]byte(line), &event); err != nil {
						t.Fatalf("invalid JSONL: %v", err)
					}
					if event.Event != "assetLibraryImageUpload" || event.DurationMs < 0 || event.RetryCount != 0 {
						t.Fatalf("invalid event %+v", event)
					}
					if i == 0 && (event.Stage != "reserve" || event.Status != "started") {
						t.Fatalf("first event %+v", event)
					}
					if i == 8 && (event.Stage != "processing" || event.Status != "completed" || event.HTTPStatus != 200) {
						t.Fatalf("last event %+v", event)
					}
				}
				if strings.Contains(stderr, "creative.png") || strings.Contains(stderr, "storage.example") || strings.Contains(stderr, "imageId") {
					t.Fatalf("identifying progress: %q", stderr)
				}
			}

			if mode == "table" {
				if err != nil || !strings.Contains(stdout, "Image ID") || !strings.Contains(stdout, "creative.png") || !strings.Contains(stdout, "PREPARE_FOR_SUBMISSION") || !strings.Contains(stdout, "true") {
					t.Fatalf("human output=%q err=%v", stdout, err)
				}
				return
			}
			var receipt struct {
				ImageID       string `json:"imageId"`
				LibraryID     string `json:"libraryId"`
				FileName      string `json:"fileName"`
				FileSize      int64  `json:"fileSize"`
				ReferenceName string `json:"referenceName"`
				State         string `json:"state"`
				Uploaded      bool   `json:"uploaded"`
				Ready         bool   `json:"ready"`
				Width         int    `json:"width"`
				Height        int    `json:"height"`
			}
			if decodeErr := json.Unmarshal([]byte(stdout), &receipt); decodeErr != nil {
				t.Fatalf("missing partial receipt: %s %v", stdout, decodeErr)
			}
			if receipt.ImageID != "image" || receipt.LibraryID != "lib" || receipt.FileName != "creative.png" || receipt.FileSize != int64(len(content)) {
				t.Fatalf("wrong receipt %+v", receipt)
			}
			if strings.Contains(stdout+stderr+stringError(err), "secret-signed") {
				t.Fatal("signed URL leaked")
			}
			if mode == "reference-name" && receipt.ReferenceName != imageReferenceName {
				t.Fatalf("receipt referenceName = %q", receipt.ReferenceName)
			}
			if mode == "progress" || mode == "success" || mode == "standalone" || mode == "transient-processing" || mode == "reference-name" {
				if err != nil || !receipt.Ready || !receipt.Uploaded || receipt.State != "PREPARE_FOR_SUBMISSION" || receipt.Width != 2 || receipt.Height != 2 || reads != 2 || uploaded != 2 {
					t.Fatalf("receipt=%+v err=%v calls=%d", receipt, err, calls)
				}
			} else if err == nil || receipt.Ready {
				t.Fatalf("false success receipt=%+v err=%v", receipt, err)
			}
			if mode == "processing-timeout" && (!receipt.Uploaded || receipt.State != "FUTURE_PROCESSING" || !errors.Is(err, context.DeadlineExceeded)) {
				t.Fatalf("unknown state did not remain pending: receipt=%+v err=%v", receipt, err)
			}
			if mode == "processing-timeout" {
				if hint := errfmt.Classify(err).Hint; hint != "Increase the upload timeout (e.g. set `ASC_UPLOAD_TIMEOUT=45m`)." {
					t.Fatalf("processing deadline hint = %q", hint)
				}
				if !strings.Contains(err.Error(), "may still finish processing") || !strings.Contains(err.Error(), "asc asset-library images view --id image") {
					t.Fatalf("processing deadline does not explain how to check the upload: %v", err)
				}
			}
			if mode == "processing-failed" {
				if !receipt.Uploaded || receipt.State != "FAILED" || errors.Is(err, context.DeadlineExceeded) || !strings.Contains(stringError(err), "FAILED: INVALID_ASSET_FILE_FORMAT") || reads != 1 || uploaded != 2 || calls != 5 {
					t.Fatalf("processing failure did not stop with partial receipt: receipt=%+v err=%v reads=%d calls=%d stderr=%q", receipt, err, reads, calls, stderr)
				}
			}
			if mode == "invalid-reservation" && calls != 1 {
				t.Fatalf("invalid reservation continued: %d calls", calls)
			}
			if mode == "upload-failure" && calls > 3 {
				t.Fatalf("upload failure continued: %d calls", calls)
			}
		})
	}
}

const imageReferenceName = "Hero · en-US · 3.14"

func jsonNumber(n int) string { data, _ := json.Marshal(n); return string(data) }
func stringError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestAssetLibraryImageUploadValidation(t *testing.T) {
	for _, args := range [][]string{{}, {"--library-id", "lib"}, {"--file", "missing.png"}, {"--library-id", "lib", "--file", "missing.png"}, {"--library-id", "../apps", "--file", "missing.png"}} {
		_, stderr, err := runAssetLibrary(t, append([]string{"asset-library", "images", "upload"}, args...)...)
		if err == nil || stderr == "" {
			t.Fatalf("missing reported validation: %v %q", err, stderr)
		}
	}
}

func TestAssetLibraryImageUploadRejectsInvalidImageBeforeHTTP(t *testing.T) {
	setupAuth(t)
	_, pngContent := creativeUploadFile(t)
	var gifData bytes.Buffer
	if err := gif.Encode(&gifData, image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.White, color.Black}), nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		content []byte
	}{
		{"empty.png", nil}, {"not-image.png", []byte("text")}, {"unsupported.gif", gifData.Bytes()}, {"unknown.bin", pngContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name)
			if err := os.WriteFile(path, tc.content, 0o600); err != nil {
				t.Fatal(err)
			}
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("invalid image caused HTTP"); return nil, nil })
			stdout, stderr, err := runAssetLibrary(t, "asset-library", "images", "upload", "--library-id", "lib", "--file", path)
			if !isUsageClassError(err) || stdout != "" || stderr == "" {
				t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout, stderr)
			}
		})
	}
}

func TestAssetLibraryUploadRejectsBlankReferenceNameBeforeHTTP(t *testing.T) {
	setupAuth(t)
	imagePath, _ := creativeUploadFile(t)
	videoPath := filepath.Join(t.TempDir(), "video.mp4")
	if err := os.WriteFile(videoPath, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("blank reference name caused HTTP")
		return nil, nil
	})
	for _, tc := range []struct{ kind, file string }{{"images", imagePath}, {"videos", videoPath}} {
		t.Run(tc.kind, func(t *testing.T) {
			stdout, stderr, err := runAssetLibrary(t, "asset-library", tc.kind, "upload", "--library-id", "lib", "--file", tc.file, "--reference-name", "  ")
			want := "asset-library " + tc.kind + " upload: --reference-name must not be blank"
			if !isUsageClassError(err) || stdout != "" || !strings.Contains(stderr, want) {
				t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout, stderr)
			}
		})
	}
}
