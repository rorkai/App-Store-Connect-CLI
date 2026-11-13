package cmdtest

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func readCSVRecords(t *testing.T, path string) [][]string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv.ReadAll() error: %v", err)
	}
	return records
}

func TestTestFlightBetaTestersExport_WritesDeterministicCSV(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters" {
			t.Fatalf("expected path /v1/betaTesters, got %s", req.URL.Path)
		}
		q := req.URL.Query()
		if q.Get("filter[apps]") != "app-1" {
			t.Fatalf("expected filter[apps]=app-1, got %q", q.Get("filter[apps]"))
		}
		if q.Get("limit") != "200" {
			t.Fatalf("expected limit=200, got %q", q.Get("limit"))
		}

		body := `{"data":[` +
			`{"type":"betaTesters","id":"tester-2","attributes":{"email":"b@example.com","firstName":"B","lastName":"Bee"}},` +
			`{"type":"betaTesters","id":"tester-1","attributes":{"email":"a@example.com","firstName":"A","lastName":"Aye"}}` +
			`]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	outPath := filepath.Join(t.TempDir(), "testers.csv")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type exportSummary struct {
		AppID      string `json:"appId"`
		OutputFile string `json:"outputFile"`
		Total      int    `json:"total"`
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "export", "--app", "app-1", "--output", outPath}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var summary exportSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("failed to parse JSON summary: %v (stdout=%q)", err, stdout)
	}
	if summary.AppID != "app-1" {
		t.Fatalf("expected appId app-1, got %q", summary.AppID)
	}
	if summary.OutputFile == "" || !strings.HasSuffix(summary.OutputFile, "testers.csv") {
		t.Fatalf("expected outputFile to end with testers.csv, got %q", summary.OutputFile)
	}
	if summary.Total != 2 {
		t.Fatalf("expected total 2, got %d", summary.Total)
	}

	records := readCSVRecords(t, outPath)
	want := [][]string{
		{"email", "first_name", "last_name"},
		{"a@example.com", "A", "Aye"},
		{"b@example.com", "B", "Bee"},
	}
	if got := strings.TrimSpace(csvRecordsToString(records)); got != strings.TrimSpace(csvRecordsToString(want)) {
		t.Fatalf("CSV records mismatch\nwant:\n%s\ngot:\n%s", csvRecordsToString(want), csvRecordsToString(records))
	}
}

func TestTestFlightBetaTestersExport_IncludeGroupsAddsColumn(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	// Group membership reads run concurrently, so match requests by path and
	// avoid t.Fatalf on transport goroutines.
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body string
		switch {
		case req.Method != http.MethodGet:
		case req.URL.Path == "/v1/apps/app-1/betaGroups":
			if req.URL.Query().Get("limit") != "200" {
				t.Errorf("expected limit=200, got %q", req.URL.Query().Get("limit"))
				return nil, fmt.Errorf("unexpected limit")
			}
			body = `{"data":[` +
				`{"type":"betaGroups","id":"group-1","attributes":{"name":"Alpha"}},` +
				`{"type":"betaGroups","id":"group-2","attributes":{"name":"Beta"}}` +
				`]}`
		case req.URL.Path == "/v1/betaTesters":
			body = `{"data":[` +
				`{"type":"betaTesters","id":"tester-2","attributes":{"email":"b@example.com","firstName":"B","lastName":"Bee"}},` +
				`{"type":"betaTesters","id":"tester-1","attributes":{"email":"a@example.com","firstName":"A","lastName":"Aye"}}` +
				`]}`
		case req.URL.Path == "/v1/betaGroups/group-1/betaTesters":
			body = `{"data":[{"type":"betaTesters","id":"tester-1"}]}`
		case req.URL.Path == "/v1/betaGroups/group-2/betaTesters":
			body = `{"data":[{"type":"betaTesters","id":"tester-1"},{"type":"betaTesters","id":"tester-2"}]}`
		}
		if body == "" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL)
			return nil, fmt.Errorf("unexpected request")
		}
		return jsonHTTPResponse(http.StatusOK, body), nil
	})

	outPath := filepath.Join(t.TempDir(), "testers.csv")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "export", "--app", "app-1", "--output", outPath, "--include-groups"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"includeGroups":true`) {
		t.Fatalf("expected includeGroups true in summary, got %q", stdout)
	}

	records := readCSVRecords(t, outPath)
	want := [][]string{
		{"email", "first_name", "last_name", "groups"},
		{"a@example.com", "A", "Aye", "Alpha;Beta"},
		{"b@example.com", "B", "Bee", "Beta"},
	}
	if got := strings.TrimSpace(csvRecordsToString(records)); got != strings.TrimSpace(csvRecordsToString(want)) {
		t.Fatalf("CSV records mismatch\nwant:\n%s\ngot:\n%s", csvRecordsToString(want), csvRecordsToString(records))
	}
}

func TestTestFlightBetaTestersExport_IncludeGroupsRenewsRequestTimeout(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "500ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	t.Setenv("ASC_MAX_RETRIES", "0")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	// One group read as two sequential 300ms pages exceeds a single 500ms
	// timeout, so the export only succeeds if each request renews it.
	const requestDelay = 300 * time.Millisecond
	var callCount atomic.Int32
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount.Add(1)
		if req.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL)
			return nil, fmt.Errorf("unexpected request")
		}
		var body string
		switch {
		case req.URL.Path == "/v1/apps/app-1/betaGroups":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Alpha"}}]}`), nil
		case req.URL.Path == "/v1/betaTesters":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-1","attributes":{"email":"a@example.com","firstName":"A","lastName":"Aye"}}]}`), nil
		case req.URL.Path == "/v1/betaGroups/group-1/betaTesters" && req.URL.Query().Get("cursor") == "":
			body = `{"data":[{"type":"betaTesters","id":"tester-1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/betaGroups/group-1/betaTesters?cursor=page-2"}}`
		case req.URL.Path == "/v1/betaGroups/group-1/betaTesters":
			body = `{"data":[{"type":"betaTesters","id":"tester-2"}]}`
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL)
			return nil, fmt.Errorf("unexpected request")
		}
		select {
		case <-time.After(requestDelay):
			return jsonHTTPResponse(http.StatusOK, body), nil
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	})

	outPath := filepath.Join(t.TempDir(), "testers.csv")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "export", "--app", "app-1", "--output", outPath, "--include-groups"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"includeGroups":true`) {
		t.Fatalf("expected includeGroups true in summary, got %q", stdout)
	}
	if got := callCount.Load(); got != 4 {
		t.Fatalf("expected four HTTP requests, got %d", got)
	}

	records := readCSVRecords(t, outPath)
	want := [][]string{
		{"email", "first_name", "last_name", "groups"},
		{"a@example.com", "A", "Aye", "Alpha"},
	}
	if got := strings.TrimSpace(csvRecordsToString(records)); got != strings.TrimSpace(csvRecordsToString(want)) {
		t.Fatalf("CSV records mismatch\nwant:\n%s\ngot:\n%s", csvRecordsToString(want), csvRecordsToString(records))
	}
}

func TestTestFlightBetaTestersImport_InvalidSchemaReturnsUsageError(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	if err := os.WriteFile(csvPath, []byte("email,unknown\nx@example.com,value\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "import", "--app", "app-1", "--input", csvPath, "--dry-run"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "unknown CSV column") {
		t.Fatalf("expected unknown column error in stderr, got %q", stderr)
	}
}

func TestTestFlightBetaTestersImport_DryRun_NoMutations(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request 1: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Beta"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("unexpected request 2: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":[{"type":"betaTesters","id":"tester-1","attributes":{"email":"existing@example.com"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			if req.Method != http.MethodGet {
				t.Fatalf("unexpected mutation request %d: %s %s", callCount, req.Method, req.URL.Path)
			}
			t.Fatalf("unexpected request count %d: %s %s", callCount, req.Method, req.URL.Path)
			return nil, nil
		}
	})

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	csvBody := "" +
		"email,groups\n" +
		"new@example.com,Beta\n" +
		"existing@example.com,Beta\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type importSummary struct {
		DryRun  bool `json:"dryRun"`
		Total   int  `json:"total"`
		Created int  `json:"created"`
		Existed int  `json:"existed"`
		Updated int  `json:"updated"`
		Failed  int  `json:"failed"`
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "import", "--app", "app-1", "--input", csvPath, "--dry-run"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var summary importSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("failed to parse JSON summary: %v (stdout=%q)", err, stdout)
	}
	if !summary.DryRun {
		t.Fatalf("expected dryRun true")
	}
	if summary.Total != 2 || summary.Created != 1 || summary.Existed != 1 || summary.Updated != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestTestFlightBetaTestersImport_CreateAssignAndInvite(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request 1: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Beta"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("unexpected request 2: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("unexpected request 3: %s %s", req.Method, req.URL.Path)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}
			if !strings.Contains(string(payload), `"email":"new@example.com"`) {
				t.Fatalf("expected email in body, got %s", string(payload))
			}
			if !strings.Contains(string(payload), `"id":"group-1"`) {
				t.Fatalf("expected group id in body, got %s", string(payload))
			}
			body := `{"data":{"type":"betaTesters","id":"tester-new","attributes":{"email":"new@example.com"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/betaTesterInvitations" {
				t.Fatalf("unexpected request 4: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"betaTesterInvitations","id":"invite-1"}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	csvBody := "" +
		"email,first_name,last_name,groups\n" +
		"new@example.com,New,Tester,Beta\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "import", "--app", "app-1", "--input", csvPath, "--invite", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"created":1`) || !strings.Contains(stdout, `"invited":1`) {
		t.Fatalf("expected created=1 and invited=1, got %q", stdout)
	}
}

func TestTestFlightBetaTestersImport_RenewsRequestTimeoutForEachMutation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "500ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	t.Setenv("ASC_MAX_RETRIES", "0")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	const requestDelay = 300 * time.Millisecond
	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("unexpected request 1: %s %s", req.Method, req.URL.Path)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
		case 2, 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("unexpected request %d: %s %s", callCount, req.Method, req.URL.Path)
			}
			select {
			case <-time.After(requestDelay):
				testerID := "tester-1"
				if callCount == 3 {
					testerID = "tester-2"
				}
				return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"betaTesters","id":"`+testerID+`"}}`), nil
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	if err := os.WriteFile(csvPath, []byte("email\none@example.com\ntwo@example.com\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type importSummary struct {
		Total   int `json:"total"`
		Created int `json:"created"`
		Failed  int `json:"failed"`
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "import", "--app", "app-1", "--input", csvPath, "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if callCount != 3 {
		t.Fatalf("expected three HTTP requests, got %d", callCount)
	}

	var summary importSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("failed to parse JSON summary: %v (stdout=%q)", err, stdout)
	}
	if summary.Total != 2 || summary.Created != 2 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestTestFlightBetaTestersImport_FailedMutationsCountOnlyAsFailed(t *testing.T) {
	type step struct {
		method string
		path   string
		status int
		body   string
	}
	const boom = `{"errors":[{"status":"500","title":"boom"}]}`
	const betaGroup = `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Beta"}}]}`
	tests := []struct {
		name        string
		csv         string
		extraArgs   []string
		steps       []step
		wantSummary map[string]float64
	}{
		{
			name: "create failure does not increment created",
			csv:  "email\nnew@example.com\n",
			steps: []step{
				{http.MethodGet, "/v1/betaTesters", http.StatusOK, `{"data":[]}`},
				{http.MethodPost, "/v1/betaTesters", http.StatusInternalServerError, boom},
			},
			wantSummary: map[string]float64{"total": 1, "created": 0, "failed": 1},
		},
		{
			name:      "invite failure does not increment invited",
			csv:       "email\nnew@example.com\n",
			extraArgs: []string{"--invite"},
			steps: []step{
				{http.MethodGet, "/v1/betaTesters", http.StatusOK, `{"data":[]}`},
				{http.MethodPost, "/v1/betaTesters", http.StatusCreated, `{"data":{"type":"betaTesters","id":"tester-new","attributes":{"email":"new@example.com"}}}`},
				{http.MethodPost, "/v1/betaTesterInvitations", http.StatusInternalServerError, boom},
			},
			wantSummary: map[string]float64{"total": 1, "created": 1, "invited": 0, "failed": 1},
		},
		{
			name: "update failure does not increment updated",
			csv:  "email,groups\nexisting@example.com,Beta\n",
			steps: []step{
				{http.MethodGet, "/v1/apps/app-1/betaGroups", http.StatusOK, betaGroup},
				{http.MethodGet, "/v1/betaTesters", http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-1","attributes":{"email":"existing@example.com"}}]}`},
				{http.MethodPost, "/v1/betaTesters/tester-1/relationships/betaGroups", http.StatusInternalServerError, boom},
			},
			wantSummary: map[string]float64{"total": 1, "existed": 1, "updated": 0, "failed": 1},
		},
		{
			name: "invalid row fails while later rows are created",
			csv:  "email,groups\n,Beta\nok@example.com,Beta\n",
			steps: []step{
				{http.MethodGet, "/v1/apps/app-1/betaGroups", http.StatusOK, betaGroup},
				{http.MethodGet, "/v1/betaTesters", http.StatusOK, `{"data":[]}`},
				{http.MethodPost, "/v1/betaTesters", http.StatusCreated, `{"data":{"type":"betaTesters","id":"tester-1","attributes":{"email":"ok@example.com"}}}`},
			},
			wantSummary: map[string]float64{"created": 1, "failed": 1},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)

			callCount := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				callCount++
				if callCount > len(test.steps) {
					t.Fatalf("unexpected request %d: %s %s", callCount, req.Method, req.URL.Path)
				}
				want := test.steps[callCount-1]
				if req.Method != want.method || req.URL.Path != want.path {
					t.Fatalf("request %d = %s %s, want %s %s", callCount, req.Method, req.URL.Path, want.method, want.path)
				}
				return &http.Response{
					StatusCode: want.status,
					Body:       io.NopCloser(strings.NewReader(want.body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}))

			csvPath := filepath.Join(t.TempDir(), "input.csv")
			if err := os.WriteFile(csvPath, []byte(test.csv), 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			args := append([]string{"testflight", "testers", "import", "--app", "app-1", "--input", csvPath}, test.extraArgs...)
			args = append(args, "--confirm")
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if _, ok := errors.AsType[ReportedError](runErr); !ok {
				t.Fatalf("expected ReportedError, got %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if callCount != len(test.steps) {
				t.Fatalf("requests = %d, want %d", callCount, len(test.steps))
			}
			var summary map[string]any
			if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
				t.Fatalf("failed to parse JSON summary: %v (stdout=%q)", err, stdout)
			}
			for field, want := range test.wantSummary {
				if summary[field] != want {
					t.Fatalf("summary[%q] = %v, want %v (summary=%v)", field, summary[field], want, summary)
				}
			}
		})
	}
}

func TestTestFlightBetaTestersImport_FastlaneHeaderAndSemicolonGroups(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request 1: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":[` +
				`{"type":"betaGroups","id":"group-1","attributes":{"name":"Beta"}},` +
				`{"type":"betaGroups","id":"group-2","attributes":{"name":"Core, Team"}}` +
				`]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("unexpected request 2: %s %s", req.Method, req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("unexpected request 3: %s %s", req.Method, req.URL.Path)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}
			bodyText := string(payload)
			if !strings.Contains(bodyText, `"email":"grace@example.com"`) {
				t.Fatalf("expected email in payload, got %s", bodyText)
			}
			if !strings.Contains(bodyText, `"id":"group-1"`) || !strings.Contains(bodyText, `"id":"group-2"`) {
				t.Fatalf("expected both groups in payload, got %s", bodyText)
			}
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(`{"data":{"type":"betaTesters","id":"tester-new","attributes":{"email":"grace@example.com"}}}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	csvPath := filepath.Join(t.TempDir(), "fastlane.csv")
	csvBody := "" +
		"First,Last,Email,Groups\n" +
		"Grace,Hopper,grace@example.com,\"Beta;Core, Team\"\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "import", "--app", "app-1", "--input", csvPath, "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"created":1`) || strings.Contains(stdout, `"failed":1`) {
		t.Fatalf("expected created=1 and failed=0, got %q", stdout)
	}
}

func TestTestFlightBetaTestersImport_HeaderlessFastlaneFormat(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("unexpected request 1: %s %s", req.Method, req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("unexpected request 2: %s %s", req.Method, req.URL.Path)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}
			bodyText := string(payload)
			if !strings.Contains(bodyText, `"firstName":"Linus"`) || !strings.Contains(bodyText, `"lastName":"Torvalds"`) {
				t.Fatalf("expected names in payload, got %s", bodyText)
			}
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(`{"data":{"type":"betaTesters","id":"tester-linus","attributes":{"email":"linus@example.com"}}}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	csvPath := filepath.Join(t.TempDir(), "headerless.csv")
	csvBody := "Linus,Torvalds,linus@example.com\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "import", "--app", "app-1", "--input", csvPath, "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"created":1`) || !strings.Contains(stdout, `"failed":0`) {
		t.Fatalf("expected created=1 and failed=0, got %q", stdout)
	}
}

func TestTestFlightBetaTestersImport_InvalidEmailFailsRow(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		if callCount > 1 {
			t.Fatalf("unexpected request count %d: %s %s", callCount, req.Method, req.URL.Path)
		}
		if req.Method != http.MethodGet || req.URL.Path != "/v1/betaTesters" {
			t.Fatalf("unexpected request 1: %s %s", req.Method, req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	csvPath := filepath.Join(t.TempDir(), "invalid-email.csv")
	csvBody := "" +
		"email\n" +
		"not-an-email\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "import", "--app", "app-1", "--input", csvPath, "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error for invalid email")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"failed":1`) || !strings.Contains(strings.ToLower(stdout), "invalid email") {
		t.Fatalf("expected failed row with invalid email error, got %q", stdout)
	}
}

func csvRecordsToString(records [][]string) string {
	var b strings.Builder
	for _, r := range records {
		b.WriteString(strings.Join(r, ","))
		b.WriteString("\n")
	}
	return b.String()
}
