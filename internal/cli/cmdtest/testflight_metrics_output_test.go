package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const groupTesterMetricsFixture = `{
	"data":[
		{
			"type":"appsBetaTesterUsages",
			"dataPoints":[{
				"start":"2026-08-01T00:00:00Z",
				"end":"2026-08-02T00:00:00Z",
				"values":{"sessionCount":12,"crashCount":1,"feedbackCount":2}
			}],
			"dimensions":{"betaTesters":{"data":{"type":"betaTesters","id":"tester-1"}}}
		},
		{
			"type":"appsBetaTesterUsages",
			"dataPoints":[{
				"start":"2026-08-02T00:00:00Z",
				"end":"2026-08-03T00:00:00Z",
				"values":{"sessionCount":3,"crashCount":0,"feedbackCount":1}
			}],
			"dimensions":{"betaTesters":{"data":{"type":"betaTesters","id":"tester-2"}}}
		}
	],
	"links":{"self":"https://api.example.test/v1/betaGroups/group-1/metrics/betaTesterUsages?groupBy=betaTesters"}
}`

const publicLinkMetricsFixture = `{
	"data":[
		{
			"dataPoints":[
				{
					"start":"2026-08-01T00:00:00Z",
					"end":"2026-08-02T00:00:00Z",
					"values":{
						"viewCount":120,
						"acceptedCount":45,
						"didNotAcceptCount":30,
						"didNotMeetCriteriaCount":9,
						"notRelevantRatio":0.25,
						"notClearRatio":0.5,
						"notInterestingRatio":0.125
					}
				},
				{
					"start":"2026-08-02T00:00:00Z",
					"end":"2026-08-03T00:00:00Z",
					"values":{"viewCount":0}
				}
			]
		}
	],
	"links":{"self":"https://api.example.test/v1/betaGroups/group-1/metrics/publicLinkUsages"},
	"meta":{"paging":{"total":1,"limit":50}}
}`

func runPublicLinkMetrics(t *testing.T, extraArgs ...string) string {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/group-1/metrics/publicLinkUsages" {
			t.Fatalf("expected path /v1/betaGroups/group-1/metrics/publicLinkUsages, got %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(publicLinkMetricsFixture)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		args := append([]string{"testflight", "metrics", "public-link", "--group", "group-1"}, extraArgs...)
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
	return stdout
}

func TestTestFlightMetricsPublicLinkOutput(t *testing.T) {
	stdout := runPublicLinkMetrics(t)

	var got, want map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if err := json.Unmarshal([]byte(publicLinkMetricsFixture), &want); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected Apple's envelope unmodified\n got: %s\nwant: %s", stdout, publicLinkMetricsFixture)
	}
}

func TestTestFlightMetricsPublicLinkTableOutput(t *testing.T) {
	stdout := runPublicLinkMetrics(t, "--output", "table")

	for _, want := range []string{
		"Start", "End", "Views", "Accepted", "Did Not Accept", "Did Not Meet Criteria",
		"Not Relevant", "Not Clear", "Not Interesting",
		"2026-08-01T00:00:00Z", "120", "45", "30", "9", "0.25", "0.5", "0.125", "2026-08-02T00:00:00Z",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table output to contain %q, got %q", want, stdout)
		}
	}
}

func TestTestFlightMetricsTestersOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/group-1/metrics/betaTesterUsages" {
			t.Fatalf("expected path /v1/betaGroups/group-1/metrics/betaTesterUsages, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("groupBy") != "betaTesters" {
			t.Fatalf("expected groupBy=betaTesters, got %q", req.URL.Query().Get("groupBy"))
		}
		body := groupTesterMetricsFixture
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "metrics", "group-testers", "--group", "group-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		Data []struct {
			Type       string `json:"type"`
			DataPoints []struct {
				Start  string `json:"start"`
				End    string `json:"end"`
				Values struct {
					SessionCount  int `json:"sessionCount"`
					CrashCount    int `json:"crashCount"`
					FeedbackCount int `json:"feedbackCount"`
				} `json:"values"`
			} `json:"dataPoints"`
			Dimensions struct {
				BetaTesters struct {
					Data struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"betaTesters"`
			} `json:"dimensions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if len(out.Data) != 2 {
		t.Fatalf("unexpected output data length: %+v", out)
	}
	first := out.Data[0]
	if first.Type != "appsBetaTesterUsages" || len(first.DataPoints) != 1 || first.Dimensions.BetaTesters.Data.Type != "betaTesters" || first.Dimensions.BetaTesters.Data.ID != "tester-1" {
		t.Fatalf("unexpected first metric: %+v", first)
	}
	point := first.DataPoints[0]
	if point.Start != "2026-08-01T00:00:00Z" || point.End != "2026-08-02T00:00:00Z" || point.Values.SessionCount != 12 || point.Values.CrashCount != 1 || point.Values.FeedbackCount != 2 {
		t.Fatalf("unexpected first metric data point: %+v", point)
	}
	if out.Data[1].Dimensions.BetaTesters.Data.ID != "tester-2" {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestTestFlightMetricsTestersTableOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/group-1/metrics/betaTesterUsages" {
			t.Fatalf("expected path /v1/betaGroups/group-1/metrics/betaTesterUsages, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("groupBy") != "betaTesters" {
			t.Fatalf("expected groupBy=betaTesters, got %q", req.URL.Query().Get("groupBy"))
		}
		body := groupTesterMetricsFixture
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "metrics", "group-testers", "--group", "group-1", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{"Tester ID", "Start", "End", "Sessions", "Crashes", "Feedback", "tester-1", "2026-08-01T00:00:00Z", "12", "1", "2", "tester-2", "3", "0"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table output to contain %q, got %q", want, stdout)
		}
	}
}

func TestTestFlightMetricsTestersMarkdownOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/group-1/metrics/betaTesterUsages" {
			t.Fatalf("expected path /v1/betaGroups/group-1/metrics/betaTesterUsages, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("groupBy") != "betaTesters" {
			t.Fatalf("expected groupBy=betaTesters, got %q", req.URL.Query().Get("groupBy"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(groupTesterMetricsFixture)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "metrics", "group-testers", "--group", "group-1", "--output", "markdown"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{"| Tester ID", "| Start", "| Sessions", "| tester-1", "| 12", "| tester-2", "| 3"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected Markdown output to contain %q, got %q", want, stdout)
		}
	}
}

func TestTestFlightMetricsTestersEmptyResponse(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":[],"links":{"self":"https://api.example.test/metrics"}}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "metrics", "group-testers", "--group", "group-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var out struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if len(out.Data) != 0 {
		t.Fatalf("expected empty data, got %+v", out.Data)
	}
}

func TestTestFlightMetricsTestersMalformedResponse(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"data":[{"type":"appsBetaTesterUsages","dimensions":{"betaTesters":{"data":42}}}],"links":{"self":"https://api.example.test/metrics"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "metrics", "group-testers", "--group", "group-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected malformed response error")
	}
	if !strings.Contains(runErr.Error(), "testflight metrics group-testers: failed to fetch") || !strings.Contains(runErr.Error(), "betaTesters.data") {
		t.Fatalf("unexpected malformed response error: %v", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected no output for malformed response, stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestTestFlightMetricsPublicLinkReturnsFetchFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/group-1/metrics/publicLinkUsages" {
			t.Fatalf("expected path /v1/betaGroups/group-1/metrics/publicLinkUsages, got %s", req.URL.Path)
		}
		body := `{"errors":[{"status":"500","title":"Server Error","detail":"metrics unavailable"}]}`
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "metrics", "public-link", "--group", "group-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "testflight metrics public-link: failed to fetch") {
		t.Fatalf("expected public-link fetch failure, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestTestFlightMetricsAppTestersPaginateRejectsRepeatedNextURL(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/metrics/betaTesterUsages?limit=2"
	const repeatedNextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/metrics/betaTesterUsages?cursor=AQ"

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
			body := `{
				"data":[{"id":"usage-1"}],
				"links":{"next":"` + repeatedNextURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != repeatedNextURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":[{"id":"usage-2"}],
				"links":{"next":"` + repeatedNextURL + `"}
			}`
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

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "metrics", "app-testers",
			"--paginate",
			"--next", firstURL,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "detected repeated pagination URL") {
		t.Fatalf("expected repeated pagination URL error, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "testflight metrics app-testers:") {
		t.Fatalf("expected command context, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestTestFlightMetricsPublicLinkOutputErrors(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/group-1/metrics/publicLinkUsages" {
			t.Fatalf("expected path /v1/betaGroups/group-1/metrics/publicLinkUsages, got %s", req.URL.Path)
		}
		body := `{"data":[{"type":"betaGroupPublicLinkUsages","id":"public-usage-1"}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unsupported output",
			args:    []string{"testflight", "metrics", "public-link", "--group", "group-1", "--output", "yaml"},
			wantErr: `(got "yaml")`,
		},
		{
			name:    "pretty with table",
			args:    []string{"testflight", "metrics", "public-link", "--group", "group-1", "--output", "table", "--pretty"},
			wantErr: "--pretty is only valid with JSON output",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !isUsageClassError(runErr) {
				t.Fatalf("expected usage-class error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
