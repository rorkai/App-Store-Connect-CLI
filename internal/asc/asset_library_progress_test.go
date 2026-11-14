package asc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAssetLibraryImageProgress(t *testing.T) {
	for _, outcome := range []string{"ready", "failed", "canceled", "ready-on-commit", "commit-reply-lost", "commit-still-awaiting", "commit-rejected"} {
		t.Run(outcome, func(t *testing.T) {
			setFastAssetUploadRetries(t, "1")
			t.Cleanup(SetMediaPollIntervalsForTest(time.Nanosecond, time.Nanosecond))
			attempts, commits, polls := 0, 0, 0
			commitReplyLost := outcome == "commit-reply-lost" || outcome == "commit-still-awaiting"
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case "POST":
					fmt.Fprintf(w, `{"data":{"id":"secret-image","type":"appAssetLibraryImages","attributes":{"state":"AWAITING_UPLOAD","uploadOperations":[{"method":"PUT","url":%q,"length":3,"offset":0}]}}}`, server.URL+"/secret-upload?token=secret-token")
				case "PUT":
					attempts++
					_, _ = io.Copy(io.Discard, r.Body)
					if attempts == 1 {
						w.WriteHeader(503)
						fmt.Fprint(w, "secret-body")
						return
					}
					w.WriteHeader(204)
				case "PATCH":
					commits++
					if commitReplyLost {
						w.Header().Set("Content-Length", "128")
						_, _ = fmt.Fprint(w, `{"data":`)
						return
					}
					if outcome == "commit-rejected" {
						w.WriteHeader(http.StatusConflict)
						fmt.Fprint(w, `{"errors":[{"status":"409","code":"STATE_ERROR","title":"Image commit rejected"}]}`)
						return
					}
					if outcome == "ready-on-commit" {
						fmt.Fprint(w, `{"data":{"id":"secret-image","type":"appAssetLibraryImages","attributes":{"state":"PREPARE_FOR_SUBMISSION","specId":"fixture-spec","imageAsset":{"width":10,"height":20}}}}`)
						return
					}
					fmt.Fprint(w, `{"data":{"id":"secret-image"}}`)
				case "GET":
					polls++
					if outcome == "ready-on-commit" {
						w.WriteHeader(http.StatusNotFound)
						fmt.Fprint(w, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Resource not found"}]}`)
						return
					}
					if outcome == "commit-still-awaiting" {
						fmt.Fprint(w, `{"data":{"id":"secret-image","type":"appAssetLibraryImages","attributes":{"state":"AWAITING_UPLOAD"}}}`)
						return
					}
					state := "PROCESSING"
					if outcome == "commit-reply-lost" {
						state = "UPLOAD_COMPLETE"
					}
					asset := ""
					if polls > 1 && (outcome != "commit-reply-lost" || polls > 2) {
						if outcome == "failed" {
							state = "FAILED"
						} else {
							state = "PREPARE_FOR_SUBMISSION"
							asset = `,"imageAsset":{"width":10,"height":20}`
						}
					}
					fmt.Fprintf(w, `{"data":{"id":"secret-image","type":"appAssetLibraryImages","attributes":{"state":%q%s}}}`, state, asset)
				default:
					t.Errorf("unexpected method %s", r.Method)
					w.WriteHeader(400)
				}
			}))
			defer server.Close()
			target, _ := url.Parse(server.URL)
			client := newTestClient(t, nil, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)})
			client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				clone := r.Clone(r.Context())
				clone.URL.Scheme = target.Scheme
				clone.URL.Host = target.Host
				return server.Client().Transport.RoundTrip(clone)
			})}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var events []AssetLibraryImageUploadEvent
			ctx = WithAssetLibraryImageUploadProgress(ctx, func(e AssetLibraryImageUploadEvent) {
				events = append(events, e)
				if outcome == "canceled" && e.Stage == "processing" && e.Status == "started" && polls == 1 {
					cancel()
				}
			})
			file := createTempAssetFile(t, []byte("abc"))
			defer file.Close()
			result, err := client.UploadAssetLibraryImageWithCategory(ctx, "secret-library", "secret-name.png", file, 3, "CREATIVE_ASSETS", "", nil)
			if outcome == "commit-still-awaiting" || outcome == "commit-rejected" {
				wantPolls := 0
				if outcome == "commit-still-awaiting" {
					wantPolls = 1
					if !errors.Is(err, io.ErrUnexpectedEOF) {
						t.Fatalf("inconclusive readback lost original transport error: %v", err)
					}
				} else {
					var statusErr interface{ HTTPStatusCode() int }
					if !errors.As(err, &statusErr) || statusErr.HTTPStatusCode() != http.StatusConflict {
						t.Fatalf("definite rejection lost HTTP 409: %v", err)
					}
				}
				if commits != 1 || polls != wantPolls || result.ImageID != "secret-image" || result.Uploaded || result.Ready {
					t.Fatalf("commits=%d reads=%d result=%+v, want one commit, %d reads and only the reserved ID", commits, polls, result, wantPolls)
				}
				return
			}
			ready := outcome == "ready" || outcome == "ready-on-commit" || outcome == "commit-reply-lost"
			if (err == nil) != ready {
				t.Fatalf("outcome %s error %v", outcome, err)
			}
			if !result.Uploaded || result.Ready != ready {
				t.Fatalf("incorrect uploaded/ready: %+v", result)
			}
			if len(result.Diagnostics) != 4 {
				t.Fatalf("diagnostics = %+v", result.Diagnostics)
			}
			wantStatus, wantCode := "completed", ""
			if outcome == "failed" {
				wantStatus, wantCode = "failed", "processing_failed"
			}
			if outcome == "canceled" {
				wantStatus, wantCode = "failed", "canceled"
			}
			final := result.Diagnostics[3]
			if outcome == "canceled" && final.HTTPStatus != 0 {
				t.Fatalf("cancellation retained unrelated HTTP status: %+v", final)
			}
			if final.Stage != "processing" || final.Status != wantStatus || final.ErrorCode != wantCode {
				t.Fatalf("final = %+v", final)
			}
			if outcome == "commit-reply-lost" && (commits != 1 || polls != 3) {
				t.Fatalf("commit reply lost: %d commits, %d readbacks, want 1 commit and 3 reads", commits, polls)
			}
			if result.Diagnostics[1].RetryCount != 1 || attempts != 2 {
				t.Fatalf("retry diagnostics %+v attempts %d", result.Diagnostics[1], attempts)
			}
			sawRetry, sawWait := false, false
			for _, e := range events {
				if e.Stage == "upload" && e.Status == "retrying" && e.HTTPStatus == 503 && e.RetryCount == 1 {
					sawRetry = true
				}
				if e.Stage == "processing" && e.Status == "started" && e.HTTPStatus == 200 {
					sawWait = true
				}
			}
			if outcome == "ready-on-commit" {
				if polls != 0 || result.SpecID != "fixture-spec" || result.Width != 10 || result.Height != 20 {
					t.Fatalf("ready commit issued %d reads or lost processed image metadata: %+v", polls, result)
				}
			}
			if !sawRetry || (outcome != "ready-on-commit" && !sawWait) {
				t.Fatalf("missing retry/wait events: %+v", events)
			}
			encoded, _ := json.Marshal(events)
			for _, forbidden := range []string{"secret", "http://", "?token", "Authorization", "body"} {
				if strings.Contains(string(encoded), forbidden) {
					t.Errorf("progress contains %q: %s", forbidden, encoded)
				}
			}
		})
	}
}

// A rejected chunk must retain its own status when another chunk finishes later.
func TestAssetLibraryImageProgressConcurrentFailureStatus(t *testing.T) {
	setFastAssetUploadRetries(t, "0")
	rejectedRead, successfulClose := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.URL.Path == "/rejected" {
			w.WriteHeader(http.StatusForbidden)
			w.(http.Flusher).Flush()
			<-successfulClose
			return
		}
		<-rejectedRead
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	original := http.DefaultTransport
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		response, err := transport.RoundTrip(r)
		if err == nil && r.URL.Path == "/rejected" {
			response.Body = &imageProgressReadBody{ReadCloser: response.Body, started: rejectedRead}
		}
		if err == nil && r.URL.Path == "/successful" {
			response.Body = &imageProgressCloseBody{ReadCloser: response.Body, closed: func() { close(successfulClose) }}
		}
		return response, err
	})
	t.Cleanup(func() { http.DefaultTransport = original })
	reservation := fmt.Sprintf(`{"data":{"id":"image","type":"appAssetLibraryImages","attributes":{"uploadOperations":[{"method":"PUT","url":%q,"length":1,"offset":0},{"method":"PUT","url":%q,"length":1,"offset":1}]}}}`, server.URL+"/rejected", server.URL+"/successful")
	client := newTestClient(t, nil, &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(reservation)), Header: make(http.Header)})
	var failed AssetLibraryImageUploadEvent
	ctx := WithAssetLibraryImageUploadProgress(context.Background(), func(event AssetLibraryImageUploadEvent) {
		if event.Status == "failed" {
			failed = event
		}
	})
	file := createTempAssetFile(t, []byte("ab"))
	defer file.Close()
	result, err := client.UploadAssetLibraryImageWithCategory(ctx, "library", "file.png", file, 2, "CREATIVE_ASSETS", "", nil)
	if err == nil || failed.Stage != "upload" || failed.HTTPStatus != 403 || failed.ErrorCode != "http_error" {
		t.Fatalf("wrong failure event: %+v, error %v", failed, err)
	}
	if got := result.Diagnostics[len(result.Diagnostics)-1]; got.HTTPStatus != 403 {
		t.Fatalf("wrong receipt failure status: %+v", got)
	}
}

type imageProgressCloseBody struct {
	io.ReadCloser
	closed func()
}

func (body *imageProgressCloseBody) Close() error {
	err := body.ReadCloser.Close()
	body.closed()
	return err
}

type imageProgressReadBody struct {
	io.ReadCloser
	started chan struct{}
	once    sync.Once
}

func (body *imageProgressReadBody) Read(data []byte) (int, error) {
	body.once.Do(func() { close(body.started) })
	return body.ReadCloser.Read(data)
}
