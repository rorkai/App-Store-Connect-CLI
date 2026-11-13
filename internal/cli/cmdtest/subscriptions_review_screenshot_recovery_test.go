package cmdtest

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/errfmt"
)

func TestSubscriptionsReviewScreenshotCreateSkipsCompleteMatchingAsset(t *testing.T) {
	setupAuth(t)
	path, content, checksum := writeSubscriptionReviewScreenshotFixture(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet {
			t.Errorf("expected state reads only, got %s %s", req.Method, req.URL.Path)
			return nil, errors.New("test transport assertion failed")
		}
		if req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1" {
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotFullResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "COMPLETE")), nil
		}
		if req.URL.Path != "/v1/subscriptions/8000000001/appStoreReviewScreenshot" {
			t.Errorf("unexpected state read: %s", req.URL.Path)
			return nil, errors.New("test transport assertion failed")
		}
		assertSubscriptionReviewScreenshotSparseFields(t, req)
		return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), strings.ToUpper(checksum), "COMPLETE", false)), nil
	})

	stdout, stderr, err := runSubscriptionReviewScreenshotCreate(t, path)
	if err != nil || stderr != "" || requests != 2 {
		t.Fatalf("unexpected skip result: requests=%d stdout=%q stderr=%q err=%v", requests, stdout, stderr, err)
	}
	attributes := subscriptionReviewScreenshotOutputAttributes(t, stdout)
	imageAsset, ok := attributes["imageAsset"].(map[string]any)
	if attributes["assetToken"] != "asset-token" || !ok || imageAsset["templateUrl"] != "https://example.test/image/{w}x{h}.png" {
		t.Fatalf("unexpected skip attributes: %#v", attributes)
	}
}

func TestSubscriptionsReviewScreenshotCreateResumesMatchingReservation(t *testing.T) {
	setupAuth(t)
	path, content, checksum := writeSubscriptionReviewScreenshotFixture(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	sequence := make([]string, 0, 4)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
			sequence = append(sequence, "read")
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), "", "", true)), nil
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			sequence = append(sequence, "upload")
			body, err := io.ReadAll(req.Body)
			if err != nil || string(body) != string(content) {
				t.Errorf("unexpected upload body: %q err=%v", body, err)
				return nil, errors.New("test transport assertion failed")
			}
			return jsonHTTPResponse(http.StatusOK, ``), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			sequence = append(sequence, "commit")
			assertSubscriptionReviewScreenshotCommit(t, req, checksum)
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "PROCESSING", false)), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			sequence = append(sequence, "poll")
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotFullResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "COMPLETE")), nil
		default:
			t.Errorf("unexpected request: %s %s host=%s", req.Method, req.URL.Path, req.URL.Host)
			return nil, errors.New("unexpected test transport request")
		}
	})

	stdout, stderr, err := runSubscriptionReviewScreenshotCreate(t, path)
	if err != nil || stderr != "" {
		t.Fatalf("unexpected resume result: stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
	if want := []string{"read", "upload", "commit", "poll"}; !reflect.DeepEqual(sequence, want) {
		t.Fatalf("expected sequence %v, got %v", want, sequence)
	}
	attributes := subscriptionReviewScreenshotOutputAttributes(t, stdout)
	imageAsset, ok := attributes["imageAsset"].(map[string]any)
	if attributes["assetToken"] != "asset-token" || !ok || imageAsset["templateUrl"] != "https://example.test/image/{w}x{h}.png" {
		t.Fatalf("unexpected upload attributes: %#v", attributes)
	}
}

func TestSubscriptionsReviewScreenshotCreateRejectsConflictingReservationDuringReconciliation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	path, content, _ := writeSubscriptionReviewScreenshotFixture(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	reads := 0
	posts := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
			reads++
			if reads == 1 {
				return jsonHTTPResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-2", "other.png", int64(len(content)), "", "", true)), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots":
			posts++
			return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"ambiguous"}]}`), nil
		default:
			t.Errorf("conflicting reservation must stop recovery: %s %s", req.Method, req.URL.Path)
			return nil, errors.New("unexpected test transport request")
		}
	})

	stdout, stderr, err := runSubscriptionReviewScreenshotCreate(t, path)
	if err == nil || !strings.Contains(err.Error(), "different incomplete") {
		t.Fatalf("expected conflicting reservation error, got %v", err)
	}
	if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitConflict {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitConflict, err)
	}
	if stdout != "" || stderr != "" || posts != 1 || reads != 2 {
		t.Fatalf("unexpected conflict result: reads=%d posts=%d stdout=%q stderr=%q", reads, posts, stdout, stderr)
	}
}

func TestSubscriptionsReviewScreenshotCreateRejectsConflictingCommitDuringReconciliation(t *testing.T) {
	tests := []struct {
		name             string
		readbackID       string
		readbackChecksum string
		want             string
	}{
		{name: "reservation replaced", readbackID: "shot-2", readbackChecksum: "different", want: "instead of upload reservation"},
		{name: "checksum changed", readbackID: "shot-1", readbackChecksum: "different", want: "checksum changed while committing upload"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_MAX_RETRIES", "0")
			path, content, _ := writeSubscriptionReviewScreenshotFixture(t)
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			reads := 0
			puts := 0
			patches := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
					reads++
					if reads == 1 {
						return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), "", "", true)), nil
					}
					return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse(tt.readbackID, filepath.Base(path), int64(len(content)), tt.readbackChecksum, "PROCESSING", false)), nil
				case req.Method == http.MethodPut && req.URL.Host == "upload.example":
					puts++
					return jsonHTTPResponse(http.StatusOK, ``), nil
				case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
					patches++
					return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"ambiguous"}]}`), nil
				default:
					t.Errorf("conflicting commit must stop recovery: %s %s", req.Method, req.URL.Path)
					return nil, errors.New("unexpected test transport request")
				}
			})

			stdout, stderr, err := runSubscriptionReviewScreenshotCreate(t, path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected conflicting commit error containing %q, got %v", tt.want, err)
			}
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitConflict {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitConflict, err)
			}
			if stdout != "" || stderr != "" || puts != 1 || patches != 1 || reads != 2 {
				t.Fatalf("unexpected conflict result: reads=%d puts=%d patches=%d stdout=%q stderr=%q", reads, puts, patches, stdout, stderr)
			}
		})
	}
}

func TestSubscriptionsReviewScreenshotCreatePollsMatchingProcessingAsset(t *testing.T) {
	setupAuth(t)
	path, content, checksum := writeSubscriptionReviewScreenshotFixture(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	reads := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Errorf("processing resume must not mutate: %s %s", req.Method, req.URL.Path)
			return nil, errors.New("test transport assertion failed")
		}
		reads++
		if req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot" {
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "PROCESSING", false)), nil
		}
		if req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1" {
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "COMPLETE", false)), nil
		}
		t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, errors.New("unexpected test transport request")
	})

	stdout, stderr, err := runSubscriptionReviewScreenshotCreate(t, path)
	if err != nil || stderr != "" || reads != 2 {
		t.Fatalf("unexpected processing result: reads=%d stdout=%q stderr=%q err=%v", reads, stdout, stderr, err)
	}
	attributes := subscriptionReviewScreenshotOutputAttributes(t, stdout)
	deliveryState, ok := attributes["assetDeliveryState"].(map[string]any)
	if !ok || deliveryState["state"] != "COMPLETE" {
		t.Fatalf("unexpected processing attributes: %#v", attributes)
	}
}

func TestSubscriptionsReviewScreenshotCreateRejectsChangedChecksumDuringPoll(t *testing.T) {
	setupAuth(t)
	path, content, checksum := writeSubscriptionReviewScreenshotFixture(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	reads := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Errorf("checksum race must not mutate: %s %s", req.Method, req.URL.Path)
			return nil, errors.New("test transport assertion failed")
		}
		reads++
		switch req.URL.Path {
		case "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "PROCESSING", false)), nil
		case "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), "different", "COMPLETE", false)), nil
		default:
			t.Errorf("unexpected request: %s", req.URL.Path)
			return nil, errors.New("unexpected test transport request")
		}
	})

	stdout, stderr, err := runSubscriptionReviewScreenshotCreate(t, path)
	if err == nil || !strings.Contains(err.Error(), "checksum changed") {
		t.Fatalf("expected checksum race error, got %v", err)
	}
	if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitConflict {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitConflict, err)
	}
	if stdout != "" || stderr != "" || reads != 2 {
		t.Fatalf("unexpected checksum race result: reads=%d stdout=%q stderr=%q", reads, stdout, stderr)
	}
}

func TestSubscriptionsReviewScreenshotCreateStopsWhenPollDeadlineExpires(t *testing.T) {
	tests := []struct {
		name          string
		uploadTimeout string
		minWait       time.Duration
		maxWait       time.Duration
		maxReads      int
	}{
		{name: "ASC_UPLOAD_TIMEOUT bounds the wait", uploadTimeout: "20ms", maxWait: 20 * time.Millisecond, maxReads: 2},
		// The fixture is under 1 MiB, so the default is 10m + 1m per file +
		// 30s per MiB. The wait slows down instead of polling every 2s.
		{name: "default scales with the screenshot", minWait: 11 * time.Minute, maxWait: 11*time.Minute + 30*time.Second, maxReads: 70},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_UPLOAD_TIMEOUT", test.uploadTimeout)
			t.Setenv("ASC_UPLOAD_TIMEOUT_SECONDS", "")
			t.Setenv("ASC_MAX_RETRIES", "0")
			t.Cleanup(asc.SetMediaPollIntervalsForTest(2*time.Second, 15*time.Second))
			path, content, checksum := writeSubscriptionReviewScreenshotFixture(t)
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			reads := 0
			var first, last time.Time
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Errorf("poll timeout must not mutate: %s %s", req.Method, req.URL.Path)
					return nil, errors.New("test transport assertion failed")
				}
				reads++
				if first.IsZero() {
					first = time.Now()
				}
				last = time.Now()
				return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "PROCESSING", false)), nil
			})

			stdout, stderr, err := runSubscriptionReviewScreenshotCreate(t, path)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected poll deadline exceeded, got %v", err)
			}
			for _, want := range []string{"screenshot shot-1", "delivery state PROCESSING", "may still finish processing", "rerun the same command"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("poll deadline error %q does not contain %q", err, want)
				}
			}
			if hint := errfmt.Classify(err).Hint; hint != "Increase the upload timeout (e.g. set `ASC_UPLOAD_TIMEOUT=45m`)." {
				t.Fatalf("poll deadline hint = %q", hint)
			}
			if stdout != "" || stderr != "" || reads < 2 || reads > test.maxReads {
				t.Fatalf("unexpected poll timeout result: reads=%d stdout=%q stderr=%q", reads, stdout, stderr)
			}
			if waited := last.Sub(first); waited < test.minWait || waited > test.maxWait {
				t.Fatalf("polled for %s, want between %s and %s", waited, test.minWait, test.maxWait)
			}
		})
	}
}

func TestSubscriptionsReviewScreenshotCreateReconcilesAmbiguousReservation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	path, content, checksum := writeSubscriptionReviewScreenshotFixture(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	reads := 0
	posts := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
			reads++
			if reads == 1 {
				return jsonHTTPResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), "", "", true)), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots":
			posts++
			return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"ambiguous"}]}`), nil
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return jsonHTTPResponse(http.StatusOK, ``), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "PROCESSING", false)), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "COMPLETE", false)), nil
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, errors.New("unexpected test transport request")
		}
	})

	_, stderr, err := runSubscriptionReviewScreenshotCreate(t, path)
	if err != nil || stderr != "" || posts != 1 || reads != 2 {
		t.Fatalf("unexpected reservation reconcile: reads=%d posts=%d stderr=%q err=%v", reads, posts, stderr, err)
	}
}

func TestSubscriptionsReviewScreenshotCreateReconcilesAmbiguousCommit(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	path, content, checksum := writeSubscriptionReviewScreenshotFixture(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	relationshipReads := 0
	patches := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
			relationshipReads++
			if relationshipReads == 1 {
				return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), "", "", true)), nil
			}
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "PROCESSING", false)), nil
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return jsonHTTPResponse(http.StatusOK, ``), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			patches++
			return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"ambiguous"}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "COMPLETE", false)), nil
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, errors.New("unexpected test transport request")
		}
	})

	_, stderr, err := runSubscriptionReviewScreenshotCreate(t, path)
	if err != nil || stderr != "" || patches != 1 || relationshipReads != 2 {
		t.Fatalf("unexpected commit reconcile: reads=%d patches=%d stderr=%q err=%v", relationshipReads, patches, stderr, err)
	}
}

func TestSubscriptionsReviewScreenshotCreateRejectsMismatchedCommitResponse(t *testing.T) {
	setupAuth(t)
	path, content, checksum := writeSubscriptionReviewScreenshotFixture(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	puts := 0
	patches := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), "", "", true)), nil
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			puts++
			return jsonHTTPResponse(http.StatusOK, ``), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			patches++
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-other", filepath.Base(path), int64(len(content)), checksum, "PROCESSING", false)), nil
		default:
			t.Errorf("mismatched commit must not be polled: %s %s", req.Method, req.URL.Path)
			return nil, errors.New("unexpected test transport request")
		}
	})

	stdout, stderr, err := runSubscriptionReviewScreenshotCreate(t, path)
	if err == nil || !strings.Contains(err.Error(), "instead of upload reservation") {
		t.Fatalf("expected mismatched commit error, got %v", err)
	}
	if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitConflict {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitConflict, err)
	}
	if stdout != "" || stderr != "" || puts != 1 || patches != 1 {
		t.Fatalf("unexpected mismatched commit result: puts=%d patches=%d stdout=%q stderr=%q", puts, patches, stdout, stderr)
	}
}

func TestSubscriptionsReviewScreenshotCreateTreatsNullRelationshipAsAbsentBeforeReservationReplay(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	path, content, checksum := writeSubscriptionReviewScreenshotFixture(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	sequence := make([]string, 0, 5)
	reads := 0
	posts := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
			reads++
			sequence = append(sequence, "read")
			return jsonHTTPResponse(http.StatusOK, `{"data":null,"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptions/8000000001/appStoreReviewScreenshot"}}`), nil
		case req.Method == http.MethodPost:
			posts++
			sequence = append(sequence, "post")
			if posts == 1 {
				return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR"}]}`), nil
			}
			return jsonHTTPResponse(http.StatusCreated, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), "", "", true)), nil
		case req.Method == http.MethodPut:
			return jsonHTTPResponse(http.StatusOK, ``), nil
		case req.Method == http.MethodPatch:
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "PROCESSING", false)), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "COMPLETE", false)), nil
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, errors.New("unexpected test transport request")
		}
	})

	_, _, err := runSubscriptionReviewScreenshotCreate(t, path)
	want := []string{"read", "post", "read", "read", "post"}
	if err != nil || reads != 3 || posts != 2 || !reflect.DeepEqual(sequence, want) {
		t.Fatalf("expected bounded replay %v, got sequence=%v reads=%d posts=%d err=%v", want, sequence, reads, posts, err)
	}
}

func TestSubscriptionsReviewScreenshotCreateReadsTwiceBeforeCommitReplay(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	path, content, checksum := writeSubscriptionReviewScreenshotFixture(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	relationshipReads := 0
	patches := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
			relationshipReads++
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), "", "", true)), nil
		case req.Method == http.MethodPut:
			return jsonHTTPResponse(http.StatusOK, ``), nil
		case req.Method == http.MethodPatch:
			patches++
			if patches == 1 {
				return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR"}]}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "PROCESSING", false)), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "COMPLETE", false)), nil
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, errors.New("unexpected test transport request")
		}
	})

	_, _, err := runSubscriptionReviewScreenshotCreate(t, path)
	if err != nil || relationshipReads != 3 || patches != 2 {
		t.Fatalf("expected two negative commit readbacks before replay, reads=%d patches=%d err=%v", relationshipReads, patches, err)
	}
}

func TestSubscriptionsReviewScreenshotCreateUsesFreshStageDeadlines(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "500ms")
	t.Setenv("ASC_UPLOAD_TIMEOUT", "500ms")
	t.Setenv("ASC_MAX_RETRIES", "0")
	path, content, checksum := writeSubscriptionReviewScreenshotFixture(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		deadline, ok := req.Context().Deadline()
		if !ok || time.Until(deadline) < 350*time.Millisecond {
			t.Errorf("expected fresh deadline for %s %s, remaining=%s", req.Method, req.URL.Path, time.Until(deadline))
			return nil, errors.New("test transport assertion failed")
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
			time.Sleep(300 * time.Millisecond)
			return jsonHTTPResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`), nil
		case req.Method == http.MethodPost:
			time.Sleep(300 * time.Millisecond)
			return jsonHTTPResponse(http.StatusCreated, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), "", "", true)), nil
		case req.Method == http.MethodPut:
			time.Sleep(300 * time.Millisecond)
			return jsonHTTPResponse(http.StatusOK, ``), nil
		case req.Method == http.MethodPatch:
			time.Sleep(300 * time.Millisecond)
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "PROCESSING", false)), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "COMPLETE", false)), nil
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, errors.New("unexpected test transport request")
		}
	})

	_, _, err := runSubscriptionReviewScreenshotCreate(t, path)
	if err != nil {
		t.Fatalf("unexpected fresh-stage result: %v", err)
	}
}

func TestSubscriptionsReviewScreenshotCreateUsesFreshTimeoutForEachUploadPart(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_UPLOAD_TIMEOUT", "150ms")
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	path, content, checksum := writeSubscriptionReviewScreenshotFixture(t)
	firstLength := int64(len(content) / 2)
	operations := []map[string]any{
		{"method": "PUT", "url": "https://upload.example/part-1", "length": firstLength, "offset": int64(0)},
		{"method": "PUT", "url": "https://upload.example/part-2", "length": int64(len(content)) - firstLength, "offset": firstLength},
	}
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	sequence := newRequestLog(7)
	var partTwoAttempts lockedCounter
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
			sequence.Add("read")
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponseWithOperations("shot-1", filepath.Base(path), int64(len(content)), operations)), nil
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) < 100*time.Millisecond {
				t.Errorf("upload part inherited stale deadline: %s", time.Until(deadline))
				return nil, errors.New("test transport assertion failed")
			}
			sequence.Add(req.URL.Path)
			if req.URL.Path == "/part-1" {
				time.Sleep(75 * time.Millisecond)
				return jsonHTTPResponse(http.StatusOK, ``), nil
			}
			if partTwoAttempts.Inc() == 1 {
				return jsonHTTPResponse(http.StatusServiceUnavailable, ``), nil
			}
			return jsonHTTPResponse(http.StatusOK, ``), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			sequence.Add("commit")
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "PROCESSING", false)), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			sequence.Add("poll")
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", filepath.Base(path), int64(len(content)), checksum, "COMPLETE", false)), nil
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, errors.New("unexpected test transport request")
		}
	})

	_, _, err := runSubscriptionReviewScreenshotCreate(t, path)
	if err != nil {
		t.Fatalf("unexpected multipart result: %v", err)
	}
	got := sequence.Snapshot()
	if len(got) != 6 || got[0] != "read" || got[4] != "commit" || got[5] != "poll" {
		t.Fatalf("unexpected multipart sequence: %v", got)
	}
	uploadAttempts := make(map[string]int, 2)
	for _, step := range got[1:4] {
		uploadAttempts[step]++
	}
	wantUploadAttempts := map[string]int{"/part-1": 1, "/part-2": 2}
	if !reflect.DeepEqual(uploadAttempts, wantUploadAttempts) {
		t.Fatalf("upload attempts = %v, want %v", uploadAttempts, wantUploadAttempts)
	}
}

func TestSubscriptionsReviewScreenshotCreateDoesNotCommitAfterUploadFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	path, content, _ := writeSubscriptionReviewScreenshotFixture(t)
	firstLength := int64(len(content) / 2)
	operations := []map[string]any{
		{"method": "PUT", "url": "https://upload.example/part-1", "length": firstLength, "offset": int64(0)},
		{"method": "PUT", "url": "https://upload.example/part-2", "length": int64(len(content)) - firstLength, "offset": firstLength},
	}
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	var puts atomic.Int32
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
			return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponseWithOperations("shot-1", filepath.Base(path), int64(len(content)), operations)), nil
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			puts.Add(1)
			if req.URL.Path == "/part-2" {
				return jsonHTTPResponse(http.StatusBadRequest, ``), nil
			}
			return jsonHTTPResponse(http.StatusOK, ``), nil
		default:
			t.Errorf("failed upload must not be committed: %s %s", req.Method, req.URL.Path)
			return nil, errors.New("unexpected test transport request")
		}
	})

	stdout, stderr, err := runSubscriptionReviewScreenshotCreate(t, path)
	if err == nil || !strings.Contains(err.Error(), "upload request failed") {
		t.Fatalf("expected upload failure, got %v", err)
	}
	// Concurrent chunk workers cancel remaining PUTs after the first 400, so
	// only one part may land. The contract is "at least one failed upload and
	// no commit", not "every reserved part was attempted".
	if stdout != "" || stderr != "" || puts.Load() < 1 {
		t.Fatalf("unexpected failed upload result: puts=%d stdout=%q stderr=%q", puts.Load(), stdout, stderr)
	}
}

func TestSubscriptionsReviewScreenshotCreateRejectsUnsafeExistingState(t *testing.T) {
	tests := []struct {
		name     string
		checksum func(string) string
		state    string
		ops      bool
		fileName string
		status   int
		body     string
		want     string
		conflict bool
	}{
		{name: "complete conflict", checksum: func(string) string { return "different" }, state: "COMPLETE", want: "different checksum", conflict: true},
		{name: "processing checksum conflict", checksum: func(string) string { return "different" }, state: "PROCESSING", want: "different checksum", conflict: true},
		{name: "failed delivery", checksum: func(value string) string { return value }, state: "FAILED", want: "run asc subscriptions review screenshots delete --screenshot-id shot-1 --confirm", conflict: true},
		{name: "missing operations", checksum: func(string) string { return "" }, state: "", want: "no upload operations"},
		{name: "incomplete identity mismatch", checksum: func(string) string { return "" }, state: "", ops: true, fileName: "other.png", want: "different incomplete", conflict: true},
		{name: "forbidden read", status: http.StatusForbidden, body: `{"errors":[{"status":"403","code":"FORBIDDEN","detail":"denied"}]}`, want: "denied"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_MAX_RETRIES", "0")
			path, content, checksum := writeSubscriptionReviewScreenshotFixture(t)
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			requests := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != http.MethodGet {
					t.Errorf("unsafe state must not mutate: %s %s", req.Method, req.URL.Path)
					return nil, errors.New("test transport assertion failed")
				}
				if tt.status != 0 {
					return jsonHTTPResponse(tt.status, tt.body), nil
				}
				fileName := tt.fileName
				if fileName == "" {
					fileName = filepath.Base(path)
				}
				return jsonHTTPResponse(http.StatusOK, subscriptionReviewScreenshotResponse("shot-1", fileName, int64(len(content)), tt.checksum(checksum), tt.state, tt.ops)), nil
			})

			stdout, stderr, err := runSubscriptionReviewScreenshotCreate(t, path)
			if err == nil || !strings.Contains(err.Error(), tt.want) || stdout != "" || stderr != "" || requests != 1 {
				t.Fatalf("expected %q: requests=%d stdout=%q stderr=%q err=%v", tt.want, requests, stdout, stderr, err)
			}
			if tt.conflict {
				if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitConflict {
					t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitConflict, err)
				}
			}
		})
	}
}

func TestSubscriptionsReviewScreenshotCreateRejectsUnsafeInvocationBeforeHTTP(t *testing.T) {
	tests := []struct {
		name    string
		extra   []string
		wantErr string
	}{
		{name: "positional", extra: []string{"stray"}, wantErr: "does not accept positional arguments"},
		{name: "unsupported output", extra: []string{"--output", "yaml"}, wantErr: `(got "yaml")`},
		{name: "pretty table", extra: []string{"--output", "table", "--pretty"}, wantErr: "--pretty is only valid with JSON output"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			path, _, _ := writeSubscriptionReviewScreenshotFixture(t)
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			})
			args := []string{"subscriptions", "review", "screenshots", "create"}
			args = append(args, tt.extra...)
			args = append(args, "--subscription-id", "8000000001", "--file", path)
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected usage error")
				}
			})
			if stdout != "" || !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("expected %q, stdout=%q stderr=%q", tt.wantErr, stdout, stderr)
			}
		})
	}
}

func runSubscriptionReviewScreenshotCreate(t *testing.T, path string) (string, string, error) {
	t.Helper()
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "review", "screenshots", "create",
			"--subscription-id", "8000000001",
			"--file", path,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		synctest.Test(t, func(*testing.T) { runErr = root.Run(context.Background()) })
	})
	return stdout, stderr, runErr
}

func writeSubscriptionReviewScreenshotFixture(t *testing.T) (string, []byte, string) {
	t.Helper()
	content := reviewScreenshotPNG(t)
	path := filepath.Join(t.TempDir(), "review.png")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write screenshot fixture: %v", err)
	}
	sum := md5.Sum(content)
	return path, content, hex.EncodeToString(sum[:])
}

func subscriptionReviewScreenshotResponse(id, fileName string, fileSize int64, checksum, state string, uploadOperations bool) string {
	attributes := map[string]any{
		"fileName":           fileName,
		"fileSize":           fileSize,
		"sourceFileChecksum": checksum,
	}
	if state != "" {
		attributes["assetDeliveryState"] = map[string]any{"state": state, "errors": []map[string]string{{"code": "DELIVERY_ERROR", "message": "delivery failed"}}}
	}
	if uploadOperations {
		attributes["uploadOperations"] = []map[string]any{{
			"method": "PUT",
			"url":    "https://upload.example/part",
			"length": fileSize,
			"offset": int64(0),
		}}
	}
	payload := map[string]any{
		"data": map[string]any{
			"type":       "subscriptionAppStoreReviewScreenshots",
			"id":         id,
			"attributes": attributes,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func subscriptionReviewScreenshotFullResponse(id, fileName string, fileSize int64, checksum, state string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(subscriptionReviewScreenshotResponse(id, fileName, fileSize, checksum, state, false)), &payload); err != nil {
		panic(err)
	}
	data := payload["data"].(map[string]any)
	attributes := data["attributes"].(map[string]any)
	attributes["assetToken"] = "asset-token"
	attributes["imageAsset"] = map[string]any{
		"templateUrl": "https://example.test/image/{w}x{h}.png",
		"width":       1280,
		"height":      720,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func subscriptionReviewScreenshotResponseWithOperations(id, fileName string, fileSize int64, operations []map[string]any) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(subscriptionReviewScreenshotResponse(id, fileName, fileSize, "", "", false)), &payload); err != nil {
		panic(err)
	}
	data := payload["data"].(map[string]any)
	attributes := data["attributes"].(map[string]any)
	attributes["uploadOperations"] = operations
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func subscriptionReviewScreenshotOutputAttributes(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var payload struct {
		Data struct {
			Attributes map[string]any `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse screenshot output: %v\n%s", err, stdout)
	}
	if payload.Data.Attributes == nil {
		t.Fatalf("screenshot output is missing attributes: %s", stdout)
	}
	return payload.Data.Attributes
}

func assertSubscriptionReviewScreenshotSparseFields(t *testing.T, req *http.Request) {
	t.Helper()
	want := "fileName,fileSize,sourceFileChecksum,uploadOperations,assetDeliveryState"
	if got := req.URL.Query().Get("fields[subscriptionAppStoreReviewScreenshots]"); got != want {
		t.Fatalf("expected sparse fields %q, got %q", want, got)
	}
}

func assertSubscriptionReviewScreenshotCommit(t *testing.T, req *http.Request, checksum string) {
	t.Helper()
	var payload struct {
		Data struct {
			Attributes map[string]any `json:"attributes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		t.Fatalf("decode commit: %v", err)
	}
	want := map[string]any{"sourceFileChecksum": checksum, "uploaded": true}
	if !reflect.DeepEqual(payload.Data.Attributes, want) {
		t.Fatalf("expected commit attributes %#v, got %#v", want, payload.Data.Attributes)
	}
}
