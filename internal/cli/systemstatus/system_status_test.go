package systemstatus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const healthyAndOutagePayload = `jsonCallback({
  "drMessage": null,
  "services": [
    {
      "serviceName": "App Store Connect API",
      "redirectUrl": "https://developer.apple.com/app-store-connect/api/",
      "events": [{
        "messageId": "2000005599",
        "statusType": "Outage",
        "message": "Users are experiencing a problem with this service.",
        "datePosted": "08/18/2026 13:01 PDT",
        "startDate": "08/18/2026 12:59 PDT",
        "endDate": null,
        "epochStartDate": 1787083140000,
        "epochEndDate": null,
        "usersAffected": "Some users are affected",
        "affectedServices": null,
        "eventStatus": "ongoing"
      }]
    },
    {
      "serviceName": "TestFlight",
      "redirectUrl": null,
      "events": []
    }
  ]
});`

type countingTransport struct {
	requests atomic.Int32
}

func (transport *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.requests.Add(1)
	return nil, errors.New("unexpected HTTP request")
}

func TestSystemStatusCommandJSONFiltersServices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if got := r.Header.Get("Accept"); !strings.Contains(got, "application/json") {
			t.Fatalf("Accept = %q, want JSON support", got)
		}
		_, _ = io.WriteString(w, healthyAndOutagePayload)
	}))
	defer server.Close()

	stdout, stderr, err := runCommand(t, server.Client(), server.URL,
		"--service", "App Store Connect API", "--output", "json")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	var report struct {
		Source  string `json:"source"`
		Summary struct {
			Status              string `json:"status"`
			TotalServices       int    `json:"totalServices"`
			OperationalServices int    `json:"operationalServices"`
			AffectedServices    int    `json:"affectedServices"`
			ActiveIncidents     int    `json:"activeIncidents"`
		} `json:"summary"`
		Services []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Events []struct {
				Active bool `json:"active"`
			} `json:"events"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, stdout)
	}
	if report.Source != statusPageURL {
		t.Fatalf("source = %q, want %q", report.Source, statusPageURL)
	}
	if report.Summary.Status != "issues" || report.Summary.TotalServices != 1 ||
		report.Summary.AffectedServices != 1 || report.Summary.OperationalServices != 0 ||
		report.Summary.ActiveIncidents != 1 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if len(report.Services) != 1 || report.Services[0].Name != "App Store Connect API" ||
		report.Services[0].Status != "issues" || len(report.Services[0].Events) != 1 ||
		!report.Services[0].Events[0].Active {
		t.Fatalf("unexpected services: %#v", report.Services)
	}
}

func TestSystemStatusCommandIssuesOnlyRetainsMatchedSummary(t *testing.T) {
	server := statusServer(t, healthyAndOutagePayload)
	defer server.Close()

	stdout, _, err := runCommand(t, server.Client(), server.URL, "--issues-only", "--output", "json")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	var report struct {
		Summary struct {
			TotalServices       int `json:"totalServices"`
			OperationalServices int `json:"operationalServices"`
			AffectedServices    int `json:"affectedServices"`
		} `json:"summary"`
		Services []struct {
			Name string `json:"name"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if report.Summary.TotalServices != 2 || report.Summary.OperationalServices != 1 || report.Summary.AffectedServices != 1 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if len(report.Services) != 1 || report.Services[0].Name != "App Store Connect API" {
		t.Fatalf("unexpected services: %#v", report.Services)
	}
}

func TestSystemStatusCommandRejectsUnknownService(t *testing.T) {
	server := statusServer(t, healthyAndOutagePayload)
	defer server.Close()

	for _, test := range []struct {
		filter string
		want   string
	}{
		{filter: "No Such Service", want: `no services matched --service "No Such Service"`},
		{filter: "TestFlight,Unknown", want: `no services matched --service "Unknown"`},
	} {
		t.Run(test.filter, func(t *testing.T) {
			_, _, err := runCommand(t, server.Client(), server.URL, "--service", test.filter, "--output", "json")
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want classified error containing %q", err, test.want)
			}
		})
	}
}

func TestSystemStatusCommandRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "positional", args: []string{"App Store Connect"}, want: "system-status does not accept positional arguments"},
		{name: "non-positive interval", args: []string{"--watch", "--poll-interval", "0s"}, want: "--poll-interval must be greater than 0"},
		{name: "negative max polls", args: []string{"--watch", "--max-polls", "-1"}, want: "--max-polls must be greater than or equal to 0"},
		{name: "max polls without watch", args: []string{"--max-polls", "2"}, want: "--max-polls requires --watch"},
		{name: "poll interval without watch", args: []string{"--poll-interval", "1s"}, want: "--poll-interval requires --watch"},
		{name: "invalid output before network", args: []string{"--output", "yaml"}, want: `--output must be one of: json, table, markdown (got "yaml")`},
		{name: "pretty JSON watch", args: []string{"--watch", "--output", "json", "--pretty"}, want: "--pretty is not supported with --watch JSON output"},
		{name: "empty service", args: []string{"--service", ""}, want: "--service must not contain empty service names"},
		{name: "trailing empty service", args: []string{"--service", "TestFlight, "}, want: "--service must not contain empty service names"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transport := &countingTransport{}
			client := &http.Client{Transport: transport}
			_, stderr, err := runCommand(t, client, "https://example.invalid", test.args...)
			if err == nil {
				t.Fatalf("error = nil, want %q", test.want)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want flag.ErrHelp classification", err)
			}
			if err.Error() != test.want {
				t.Fatalf("error = %q, want %q", err.Error(), test.want)
			}
			if wantStderr := "Error: " + test.want + "\n"; stderr != wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
			}
			if requests := transport.requests.Load(); requests != 0 {
				t.Fatalf("HTTP requests = %d, want 0", requests)
			}
		})
	}
}

func TestSystemStatusCommandWatchPrintsOnlyChanges(t *testing.T) {
	var requests atomic.Int32
	healthy := `{"drMessage":null,"services":[{"serviceName":"App Store Connect API","redirectUrl":null,"events":[]}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		poll := requests.Add(1)
		if poll < 3 {
			_, _ = io.WriteString(w, healthy)
			return
		}
		_, _ = io.WriteString(w, healthyAndOutagePayload)
	}))
	defer server.Close()

	stdout, stderr, err := runCommand(t, server.Client(), server.URL,
		"--service", "App Store Connect API", "--watch", "--poll-interval", "1ms", "--max-polls", "3", "--output", "json")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("watch lines = %d, want 2; output=%q", len(lines), stdout)
	}
	if requests.Load() != 3 {
		t.Fatalf("requests = %d, want 3", requests.Load())
	}
	for index, line := range lines {
		var report map[string]any
		if err := json.Unmarshal([]byte(line), &report); err != nil {
			t.Fatalf("line %d is not JSON: %v\n%s", index, err, line)
		}
	}
}

func TestSystemStatusCommandWatchRetriesTransientFailures(t *testing.T) {
	var requests atomic.Int32
	healthy := `{"drMessage":null,"services":[{"serviceName":"App Store Connect API","redirectUrl":null,"events":[]}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, healthy)
	}))
	defer server.Close()

	stdout, stderr, err := runCommand(t, server.Client(), server.URL,
		"--watch", "--poll-interval", "1ms", "--max-polls", "2", "--output", "json")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want 2", requests.Load())
	}
	if !strings.Contains(stderr, "poll 1 failed") || !strings.Contains(stderr, "retrying in 1ms") {
		t.Fatalf("stderr = %q, want retry warning", stderr)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("watch output is not JSON: %v\n%s", err, stdout)
	}
}

func TestSystemStatusCommandWatchStopsAfterRepeatedFailures(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	stdout, stderr, err := runCommand(t, server.Client(), server.URL,
		"--watch", "--poll-interval", "1ms", "--output", "json")
	if err == nil {
		t.Fatal("run error = nil, want repeated failure")
	}
	if requests.Load() != maxWatchFailures {
		t.Fatalf("requests = %d, want %d", requests.Load(), maxWatchFailures)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if warnings := strings.Count(stderr, "retrying in 1ms"); warnings != maxWatchFailures-1 {
		t.Fatalf("retry warnings = %d, want %d; stderr=%q", warnings, maxWatchFailures-1, stderr)
	}
}

func TestFetchDeveloperSystemStatusErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       string
	}{
		{name: "http", statusCode: http.StatusServiceUnavailable, body: "unavailable", want: "HTTP 503"},
		{name: "unknown wrapper", statusCode: http.StatusOK, body: `otherCallback({"services":[]})`, want: "unsupported response wrapper"},
		{name: "invalid JSON", statusCode: http.StatusOK, body: `jsonCallback({)`, want: "decode response"},
		{name: "empty services", statusCode: http.StatusOK, body: `{"services":[]}`, want: "contained no services"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.statusCode)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()

			_, err := fetchDeveloperSystemStatus(context.Background(), server.Client(), server.URL)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func statusServer(t *testing.T, payload string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, payload)
	}))
}

func runCommand(t *testing.T, client *http.Client, endpoint string, args ...string) (stdout string, stderr string, runErr error) {
	t.Helper()
	command := commandWithClient(client, endpoint)
	command.FlagSet.SetOutput(io.Discard)
	if err := command.Parse(args); err != nil {
		return "", "", err
	}

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	copyErrors := make(chan error, 2)
	var copies sync.WaitGroup
	copies.Add(2)
	go func() {
		defer copies.Done()
		_, err := io.Copy(&stdoutBuffer, stdoutReader)
		copyErrors <- err
	}()
	go func() {
		defer copies.Done()
		_, err := io.Copy(&stderrBuffer, stderrReader)
		copyErrors <- err
	}()

	runErr = command.Run(context.Background())
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	copies.Wait()
	_ = stdoutReader.Close()
	_ = stderrReader.Close()
	close(copyErrors)
	for err := range copyErrors {
		if err != nil {
			t.Fatalf("capture command output: %v", err)
		}
	}
	return stdoutBuffer.String(), stderrBuffer.String(), runErr
}

func TestDecodeDeveloperSystemStatusFeedShapes(t *testing.T) {
	tests := []struct {
		name   string
		body   []byte
		assert func(*testing.T, *asc.DeveloperSystemStatusReport)
	}{
		{
			name: "plain JSON",
			body: []byte(`{"drMessage":"Maintenance window","services":[{"serviceName":"TestFlight","redirectUrl":null,"events":[]}]}`),
			assert: func(t *testing.T, report *asc.DeveloperSystemStatusReport) {
				if report.Message != "Maintenance window" || report.Summary.Status != "issues" {
					t.Fatalf("unexpected report: %#v", report)
				}
			},
		},
		{
			name: "UTF-8 BOM",
			body: append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"services":[{"serviceName":"TestFlight","events":[]}]}`)...),
			assert: func(t *testing.T, report *asc.DeveloperSystemStatusReport) {
				if len(report.Services) != 1 || report.Services[0].Name != "TestFlight" {
					t.Fatalf("unexpected services: %#v", report.Services)
				}
			},
		},
		{
			name: "affected services array",
			body: []byte(`{"services":[{"serviceName":"Xcode Cloud","events":[{"eventStatus":"ongoing","affectedServices":["App Store","TestFlight"]}]}]}`),
			assert: func(t *testing.T, report *asc.DeveloperSystemStatusReport) {
				got := report.Services[0].Events[0].AffectedServices
				if got != "App Store, TestFlight" {
					t.Fatalf("affected services = %q, want %q", got, "App Store, TestFlight")
				}
			},
		},
		{
			name: "resolved event",
			body: []byte(`{"services":[{"serviceName":"TestFlight","events":[{"eventStatus":"resolved","epochEndDate":1787086740000}]}]}`),
			assert: func(t *testing.T, report *asc.DeveloperSystemStatusReport) {
				service := report.Services[0]
				if service.Status != "operational" || service.Events[0].Active {
					t.Fatalf("resolved service = %#v, want inactive operational event", service)
				}
			},
		},
		{
			name: "completed event",
			body: []byte(`{"services":[{"serviceName":"App Store Connect","events":[{"messageId":"2000005610","eventStatus":"completed","startDate":"08/20/2026 05:00 PDT","endDate":"08/20/2026 06:00 PDT","epochStartDate":1787227200000,"epochEndDate":1787230800000}]}]}`),
			assert: func(t *testing.T, report *asc.DeveloperSystemStatusReport) {
				service := report.Services[0]
				if service.Status != "operational" || service.Events[0].Active {
					t.Fatalf("completed service = %#v, want inactive operational event", service)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report, err := decodeDeveloperSystemStatus(test.body)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			test.assert(t, report)
		})
	}
}

func TestFetchDeveloperSystemStatusRejectsOversizedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, strings.Repeat("x", maxStatusResponseBytes+1))
	}))
	defer server.Close()

	_, err := fetchDeveloperSystemStatus(context.Background(), server.Client(), server.URL)
	if err == nil || !strings.Contains(err.Error(), "response exceeds") {
		t.Fatalf("error = %v, want oversized response error", err)
	}
}

func TestDecodeDeveloperSystemStatusRejectsMissingOrNullEvents(t *testing.T) {
	for _, body := range []string{
		`{"services":[{"serviceName":"TestFlight"}]}`,
		`{"services":[{"serviceName":"TestFlight","events":null}]}`,
	} {
		_, err := decodeDeveloperSystemStatus([]byte(body))
		if err == nil || !strings.Contains(err.Error(), `service "TestFlight" is missing events`) {
			t.Fatalf("decode error = %v, want missing-events failure for %s", err, body)
		}
	}
}

func TestDecodeDeveloperSystemStatusRejectsUnknownEventStatus(t *testing.T) {
	_, err := decodeDeveloperSystemStatus([]byte(`{"services":[{"serviceName":"TestFlight","events":[{"eventStatus":"investigating"}]}]}`))
	if err == nil || !strings.Contains(err.Error(), `service "TestFlight" event 1 has unknown eventStatus "investigating"`) {
		t.Fatalf("decode error = %v, want unknown-event-status failure", err)
	}
}
