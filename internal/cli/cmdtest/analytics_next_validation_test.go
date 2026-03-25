package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func runAnalyticsInvalidNextURLCases(
	t *testing.T,
	argsPrefix []string,
	wantErrPrefix string,
) {
	t.Helper()

	tests := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name:    "invalid scheme",
			next:    "http://api.appstoreconnect.apple.com/v1/analyticsReportRequests?cursor=AQ",
			wantErr: wantErrPrefix + " must be an App Store Connect URL",
		},
		{
			name:    "malformed URL",
			next:    "https://api.appstoreconnect.apple.com/%zz",
			wantErr: wantErrPrefix + " must be a valid URL:",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append(append([]string{}, argsPrefix...), "--next", test.next)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("expected error %q, got %v", test.wantErr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func runAnalyticsPaginateFromNext(
	t *testing.T,
	argsPrefix []string,
	firstURL string,
	secondURL string,
	firstBody string,
	secondBody string,
	wantIDs ...string,
) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(firstBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(secondBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	args := append(append([]string{}, argsPrefix...), "--paginate", "--next", firstURL)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, id := range wantIDs {
		needle := `"id":"` + id + `"`
		if !strings.Contains(stdout, needle) {
			t.Fatalf("expected output to contain %q, got %q", needle, stdout)
		}
	}
}

func TestAnalyticsRequestsRejectsInvalidNextURL(t *testing.T) {
	runAnalyticsInvalidNextURLCases(
		t,
		[]string{"analytics", "requests"},
		"analytics requests: --next",
	)
}

func TestAnalyticsRequestsPaginateFromNextWithoutApp(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/analyticsReportRequests?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/analyticsReportRequests?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"analyticsReportRequests","id":"analytics-request-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"analyticsReportRequests","id":"analytics-request-next-2"}],"links":{"next":""}}`

	runAnalyticsPaginateFromNext(
		t,
		[]string{"analytics", "requests"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"analytics-request-next-1",
		"analytics-request-next-2",
	)
}

func TestAnalyticsInstancesRelationshipsRejectsInvalidNextURL(t *testing.T) {
	runAnalyticsInvalidNextURLCases(
		t,
		[]string{"analytics", "instances", "links"},
		"analytics instances links: --next",
	)
}

func TestAnalyticsInstancesRelationshipsPaginateFromNextWithoutInstanceID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/analyticsReportInstances/instance-1/relationships/segments?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/analyticsReportInstances/instance-1/relationships/segments?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"analyticsReportSegments","id":"analytics-segment-link-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"analyticsReportSegments","id":"analytics-segment-link-next-2"}],"links":{"next":""}}`

	runAnalyticsPaginateFromNext(
		t,
		[]string{"analytics", "instances", "links"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"analytics-segment-link-next-1",
		"analytics-segment-link-next-2",
	)
}

func TestAnalyticsReportsRelationshipsRejectsInvalidNextURL(t *testing.T) {
	runAnalyticsInvalidNextURLCases(
		t,
		[]string{"analytics", "reports", "links"},
		"analytics reports links: --next",
	)
}

func TestAnalyticsReportsRelationshipsPaginateFromNextWithoutReportID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/analyticsReports/report-1/relationships/instances?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/analyticsReports/report-1/relationships/instances?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"analyticsReportInstances","id":"analytics-report-instance-link-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"analyticsReportInstances","id":"analytics-report-instance-link-next-2"}],"links":{"next":""}}`

	runAnalyticsPaginateFromNext(
		t,
		[]string{"analytics", "reports", "links"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"analytics-report-instance-link-next-1",
		"analytics-report-instance-link-next-2",
	)
}

func TestAnalyticsGetRejectsInvalidNextURL(t *testing.T) {
	runAnalyticsInvalidNextURLCases(
		t,
		[]string{"analytics", "view"},
		"analytics view: --next",
	)
}

func TestAnalyticsGetPaginateFromNextWithoutRequestID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const reportsURL = "https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/request-1/reports?cursor=AQ&limit=200"
	const instancesURL = "https://api.appstoreconnect.apple.com/v1/analyticsReports/analytics-report-next-1/instances?limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != reportsURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"analyticsReports","id":"analytics-report-next-1","attributes":{"name":"Retention","category":"APP_USAGE","granularity":"DAILY"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != instancesURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"analyticsReportInstances","id":"analytics-instance-next-1","attributes":{"reportDate":"2024-01-01","processingDate":"2024-01-02T00:00:00Z","granularity":"DAILY","version":"1"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"analytics", "view", "--paginate", "--next", reportsURL}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"analytics-report-next-1"`) || !strings.Contains(stdout, `"id":"analytics-instance-next-1"`) {
		t.Fatalf("expected report and instance IDs in output, got %q", stdout)
	}
}
