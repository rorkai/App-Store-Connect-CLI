package cmdtest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const analyticsTimeoutRequestDelay = 10 * time.Millisecond

func TestAnalyticsViewRefreshesTimeoutForEachRequest(t *testing.T) {
	setupAnalyticsTimeoutTest(t)

	const (
		reportsNextURL   = "https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/" + analyticsViewRequestID + "/reports?cursor=reports-next"
		instancesNextURL = "https://api.appstoreconnect.apple.com/v1/analyticsReports/report-1/instances?cursor=instances-next"
		segmentsNextURL  = "https://api.appstoreconnect.apple.com/v1/analyticsReportInstances/instance-1/segments?cursor=segments-next"
	)
	var deadlines []time.Time
	requestCount := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		deadline := requireAnalyticsRequestDeadline(t, req)
		deadlines = append(deadlines, deadline)
		if req.Header.Get("Authorization") == "" {
			t.Fatal("expected App Store Connect request authorization")
		}
		time.Sleep(analyticsTimeoutRequestDelay)

		switch requestCount {
		case 1:
			assertAnalyticsTimeoutRequest(t, req, "/v1/analyticsReportRequests/"+analyticsViewRequestID+"/reports", "limit=200")
			return analyticsViewJSONResponse(`{
				"data":[{"type":"analyticsReports","id":"report-1","attributes":{"name":"App Sessions"}}],
				"links":{"next":"` + reportsNextURL + `"}
			}`), nil
		case 2:
			assertAnalyticsTimeoutRequest(t, req, "/v1/analyticsReportRequests/"+analyticsViewRequestID+"/reports", "cursor=reports-next")
			return analyticsViewJSONResponse(`{
				"data":[{"type":"analyticsReports","id":"report-2","attributes":{"name":"Store Discovery"}}],
				"links":{}
			}`), nil
		case 3:
			assertAnalyticsTimeoutRequest(t, req, "/v1/analyticsReports/report-1/instances", "limit=200")
			return analyticsViewJSONResponse(`{
				"data":[{"type":"analyticsReportInstances","id":"instance-1","attributes":{"processingDate":"2024-01-20"}}],
				"links":{"next":"` + instancesNextURL + `"}
			}`), nil
		case 4:
			assertAnalyticsTimeoutRequest(t, req, "/v1/analyticsReports/report-1/instances", "cursor=instances-next")
			return analyticsViewJSONResponse(`{
				"data":[{"type":"analyticsReportInstances","id":"instance-2","attributes":{"processingDate":"2024-01-21"}}],
				"links":{}
			}`), nil
		case 5:
			assertAnalyticsTimeoutRequest(t, req, "/v1/analyticsReportInstances/instance-1/segments", "limit=200")
			return analyticsViewJSONResponse(`{
				"data":[{"type":"analyticsReportSegments","id":"segment-1"}],
				"links":{"next":"` + segmentsNextURL + `"}
			}`), nil
		case 6:
			assertAnalyticsTimeoutRequest(t, req, "/v1/analyticsReportInstances/instance-1/segments", "cursor=segments-next")
			return analyticsViewJSONResponse(`{"data":[{"type":"analyticsReportSegments","id":"segment-2"}],"links":{}}`), nil
		case 7:
			assertAnalyticsTimeoutRequest(t, req, "/v1/analyticsReportInstances/instance-2/segments", "limit=200")
			return analyticsViewJSONResponse(`{"data":[],"links":{}}`), nil
		case 8:
			assertAnalyticsTimeoutRequest(t, req, "/v1/analyticsReports/report-2/instances", "limit=200")
			return analyticsViewJSONResponse(`{"data":[],"links":{}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	setAnalyticsTimeoutTestClient(t, transport)

	stdout, stderr, runErr := runCommand(t, []string{
		"analytics", "view",
		"--request-id", analyticsViewRequestID,
		"--include-segments",
		"--paginate",
		"--output", "json",
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"instance-1"`) || !strings.Contains(stdout, `"id":"report-2"`) {
		t.Fatalf("analytics view output lost selected resources: %s", stdout)
	}
	if requestCount != 8 {
		t.Fatalf("request count = %d, want 8", requestCount)
	}
	assertAnalyticsDeadlinesRefresh(t, deadlines)
}

func TestAnalyticsDownloadRefreshesTimeoutThroughBodyTransfer(t *testing.T) {
	setupAnalyticsTimeoutTest(t)

	const reportData = "analytics-timeout-report"
	var deadlines []time.Time
	var reportBody *analyticsDeadlineBody
	requestCount := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		deadline := requireAnalyticsRequestDeadline(t, req)
		deadlines = append(deadlines, deadline)
		time.Sleep(analyticsTimeoutRequestDelay)

		switch requestCount {
		case 1:
			assertAnalyticsTimeoutRequest(t, req, "/v1/analyticsReportRequests/"+analyticsViewRequestID+"/reports", "limit=200")
			requireAnalyticsRequestAuth(t, req, true)
			return analyticsViewJSONResponse(`{"data":[{"type":"analyticsReports","id":"report-1"}],"links":{}}`), nil
		case 2:
			assertAnalyticsTimeoutRequest(t, req, "/v1/analyticsReports/report-1/instances", "limit=200")
			requireAnalyticsRequestAuth(t, req, true)
			return analyticsViewJSONResponse(`{"data":[{"type":"analyticsReportInstances","id":"instance-1"}],"links":{}}`), nil
		case 3:
			assertAnalyticsTimeoutRequest(t, req, "/v1/analyticsReportInstances/instance-1/segments", "limit=200")
			requireAnalyticsRequestAuth(t, req, true)
			return analyticsViewJSONResponse(`{
				"data":[{"type":"analyticsReportSegments","id":"segment-1","attributes":{"url":"https://mzstatic.com/report.csv.gz"}}],
				"links":{}
			}`), nil
		case 4:
			assertAnalyticsTimeoutRequest(t, req, "/report.csv.gz", "")
			requireAnalyticsRequestAuth(t, req, false)
			reportBody = &analyticsDeadlineBody{
				ctx:    req.Context(),
				reader: strings.NewReader(reportData),
			}
			return &http.Response{
				StatusCode:    http.StatusOK,
				Body:          reportBody,
				ContentLength: int64(len(reportData)),
				Header:        http.Header{"Content-Type": []string{"application/a-gzip"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	setAnalyticsTimeoutTestClient(t, transport)

	outputPath := filepath.Join(t.TempDir(), "analytics-report.csv.gz")
	stdout, stderr, runErr := runCommand(t, []string{
		"analytics", "download",
		"--request-id", analyticsViewRequestID,
		"--instance-id", "instance-1",
		"--segment-id", "segment-1",
		"--output", outputPath,
		"--output-format", "json",
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"segmentId":"segment-1"`) {
		t.Fatalf("analytics download output lost selected segment: %s", stdout)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read downloaded report: %v", err)
	}
	if string(data) != reportData {
		t.Fatalf("downloaded report = %q, want %q", data, reportData)
	}
	if reportBody == nil || reportBody.reads == 0 {
		t.Fatal("report body was not read")
	}
	if reportBody.contextErr != nil {
		t.Fatalf("report body context ended before transfer completed: %v", reportBody.contextErr)
	}
	if requestCount != 4 {
		t.Fatalf("request count = %d, want 4", requestCount)
	}
	assertAnalyticsDeadlinesRefresh(t, deadlines)
}

func TestAnalyticsViewFreshTimeoutsPreserveParentCancellation(t *testing.T) {
	setupAnalyticsTimeoutTest(t)

	parentCtx, cancelParent := context.WithCancel(context.Background())
	requestCount := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount == 1 {
			requireAnalyticsRequestDeadline(t, req)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: &analyticsCancelOnEOFBody{
					reader: strings.NewReader(`{"data":[{"type":"analyticsReports","id":"report-1"}],"links":{}}`),
					cancel: cancelParent,
				},
				Header: http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
		if !errors.Is(req.Context().Err(), context.Canceled) {
			t.Fatalf("later request context error = %v, want parent cancellation", req.Context().Err())
		}
		return nil, req.Context().Err()
	})
	setAnalyticsTimeoutTestClient(t, transport)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "view",
			"--request-id", analyticsViewRequestID,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(parentCtx)
	})
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("run error = %v, want parent cancellation", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected no output after cancellation, got stdout=%q stderr=%q", stdout, stderr)
	}
	if requestCount > 2 {
		t.Fatalf("request count = %d, want cancellation before further work", requestCount)
	}
}

func setupAnalyticsTimeoutTest(t *testing.T) {
	t.Helper()
	setupAnalyticsResourceIDAuth(t)
	t.Setenv("ASC_TIMEOUT", "2s")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
}

func setAnalyticsTimeoutTestClient(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("new analytics timeout client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))
}

func requireAnalyticsRequestDeadline(t *testing.T, req *http.Request) time.Time {
	t.Helper()
	deadline, ok := req.Context().Deadline()
	if !ok {
		t.Fatalf("request has no timeout deadline: %s %s", req.Method, req.URL.String())
	}
	return deadline
}

func assertAnalyticsTimeoutRequest(t *testing.T, req *http.Request, wantPath, wantQuery string) {
	t.Helper()
	if req.Method != http.MethodGet || req.URL.Path != wantPath {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
	}
	if req.URL.RawQuery != wantQuery {
		t.Fatalf("query = %q, want %q", req.URL.RawQuery, wantQuery)
	}
}

func requireAnalyticsRequestAuth(t *testing.T, req *http.Request, want bool) {
	t.Helper()
	got := req.Header.Get("Authorization")
	if want && got == "" {
		t.Fatal("expected App Store Connect request authorization")
	}
	if !want && got != "" {
		t.Fatalf("unexpected authorization on report transfer: %q", got)
	}
}

func assertAnalyticsDeadlinesRefresh(t *testing.T, deadlines []time.Time) {
	t.Helper()
	if len(deadlines) < 2 {
		t.Fatalf("deadline count = %d, want at least 2", len(deadlines))
	}
	for i := 1; i < len(deadlines); i++ {
		if !deadlines[i].After(deadlines[i-1]) {
			t.Fatalf("request %d deadline = %s, want later than request %d deadline %s", i+1, deadlines[i].Format(time.RFC3339Nano), i, deadlines[i-1].Format(time.RFC3339Nano))
		}
	}
}

type analyticsDeadlineBody struct {
	ctx        context.Context
	reader     *strings.Reader
	reads      int
	contextErr error
}

func (b *analyticsDeadlineBody) Read(p []byte) (int, error) {
	b.reads++
	if err := b.ctx.Err(); err != nil {
		b.contextErr = err
		return 0, err
	}
	return b.reader.Read(p)
}

func (b *analyticsDeadlineBody) Close() error {
	return nil
}

var _ io.ReadCloser = (*analyticsDeadlineBody)(nil)

type analyticsCancelOnEOFBody struct {
	reader *strings.Reader
	cancel context.CancelFunc
}

func (b *analyticsCancelOnEOFBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	if errors.Is(err, io.EOF) {
		b.cancel()
	}
	return n, err
}

func (b *analyticsCancelOnEOFBody) Close() error {
	return nil
}
