package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTestFlightBetaTestersViewPreservesEmptyAppDevices(t *testing.T) {
	setupAuth(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/betaTesters/tester-1" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com","appDevices":[]}}}`)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "view", "--id", "tester-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var response struct {
		Data struct {
			Attributes map[string]json.RawMessage `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode stdout: %v (stdout=%q)", err, stdout)
	}
	appDevices, present := response.Data.Attributes["appDevices"]
	if !present || string(appDevices) != `[]` {
		t.Fatalf("expected explicit empty appDevices array, got %s", stdout)
	}
}

func TestTestFlightBetaTestersAddOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("expected path /v1/apps/app-1/betaGroups, got %s", req.URL.Path)
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit 200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Beta"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("expected path /v1/betaTesters, got %s", req.URL.Path)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			if !strings.Contains(string(payload), `"email":"tester@example.com"`) {
				t.Fatalf("expected email in body, got %s", string(payload))
			}
			if !strings.Contains(string(payload), `"id":"group-1"`) {
				t.Fatalf("expected group id in body, got %s", string(payload))
			}
			body := `{"data":{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}}`
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

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "add", "--app", "app-1", "--email", "tester@example.com", "--group", "Beta"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"tester-1"`) {
		t.Fatalf("expected tester id in output, got %q", stdout)
	}
}

func TestTestFlightBetaTestersAddSendsRelationshipsInOneCreate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	const groups = `"betaGroups":{"data":[{"type":"betaGroups","id":"group-1"}]}`
	const builds = `"builds":{"data":[{"type":"builds","id":"build-1"},{"type":"builds","id":"build-2"}]}`
	tests := []struct {
		name         string
		args         []string
		wantRequests []string
		want         string
	}{
		{
			name:         "omitted --build-id sends only groups",
			args:         []string{"--app", "app-1", "--group", "Beta"},
			wantRequests: []string{"GET /v1/apps/app-1/betaGroups", "POST /v1/betaTesters"},
			want:         `{` + groups + `}`,
		},
		{
			name:         "groups and builds",
			args:         []string{"--app", "app-1", "--group", "Beta", "--build-id", "build-1, build-2,build-1"},
			wantRequests: []string{"GET /v1/apps/app-1/betaGroups", "POST /v1/betaTesters"},
			want:         `{` + groups + `,` + builds + `}`,
		},
		{
			name:         "builds alone need neither a group nor an app",
			args:         []string{"--build-id", "build-1,build-2"},
			wantRequests: []string{"POST /v1/betaTesters"},
			want:         `{` + builds + `}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requests []string
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req.Method+" "+req.URL.Path)
				body := `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Beta"}}]}`
				status := http.StatusOK
				if req.Method == http.MethodPost {
					if got := createRequestRelationships(t, req); got != test.want {
						t.Fatalf("relationships = %s, want %s", got, test.want)
					}
					body = `{"data":{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}}`
					status = http.StatusCreated
				}
				return &http.Response{
					StatusCode: status,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			})

			args := append([]string{"testflight", "testers", "add", "--email", "tester@example.com"}, test.args...)
			if _, _, err := runCommand(t, args); err != nil {
				t.Fatalf("run error: %v", err)
			}
			if !slices.Equal(requests, test.wantRequests) {
				t.Fatalf("requests = %v, want %v", requests, test.wantRequests)
			}
		})
	}
}

func TestBetaCreateRejectsEmptyRelationshipFlagsBeforeRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "group create --build-id",
			args:    []string{"testflight", "groups", "create", "--app", "app-1", "--name", "Beta", "--build-id", " , "},
			wantErr: "--build-id must include at least one ID",
		},
		{
			name:    "group create --tester",
			args:    []string{"testflight", "groups", "create", "--app", "app-1", "--name", "Beta", "--tester", ""},
			wantErr: "--tester must include at least one ID",
		},
		{
			name:    "tester add --build-id",
			args:    []string{"testflight", "testers", "add", "--app", "app-1", "--email", "tester@example.com", "--group", "Beta", "--build-id", ","},
			wantErr: "--build-id must include at least one ID",
		},
		{
			name:    "tester add blank --group beside builds",
			args:    []string{"testflight", "testers", "add", "--email", "tester@example.com", "--group", " ", "--build-id", "build-1"},
			wantErr: "--group is required",
		},
		{
			name:    "tester add group without app",
			args:    []string{"testflight", "testers", "add", "--email", "tester@example.com", "--group", "Beta", "--build-id", "build-1"},
			wantErr: "--app is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestTestFlightBetaTestersRemoveOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("expected path /v1/betaTesters, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[apps]") != "app-1" {
				t.Fatalf("expected app filter app-1, got %q", query.Get("filter[apps]"))
			}
			if query.Get("filter[email]") != "tester@example.com" {
				t.Fatalf("expected email filter tester@example.com, got %q", query.Get("filter[email]"))
			}
			body := `{"data":[{"type":"betaTesters","id":"tester-2","attributes":{"email":"tester@example.com"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodDelete {
				t.Fatalf("expected DELETE, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaTesters/tester-2" {
				t.Fatalf("expected path /v1/betaTesters/tester-2, got %s", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "remove", "--app", "app-1", "--email", "tester@example.com", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"deleted":true`) {
		t.Fatalf("expected deleted true in output, got %q", stdout)
	}
}

func TestTestFlightBetaTestersListWithGroupUsesOnlyGroupFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("expected path /v1/apps/app-1/betaGroups, got %s", req.URL.Path)
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit 200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Beta"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("expected path /v1/betaTesters, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[apps]") != "" {
				t.Fatalf("expected no app relationship filter when group filter is set, got %q", query.Get("filter[apps]"))
			}
			if query.Get("filter[betaGroups]") != "group-1" {
				t.Fatalf("expected beta group filter group-1, got %q", query.Get("filter[betaGroups]"))
			}
			body := `{"data":[{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "list", "--app", "app-1", "--group", "Beta"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"tester-1"`) {
		t.Fatalf("expected tester id in output, got %q", stdout)
	}
}

func TestTestFlightBetaTestersListRejectsGroupAndBuildID(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "list", "--app", "app-1", "--group", "group-1", "--build-id", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --group cannot be combined with --build-id") {
		t.Fatalf("expected conflicting filter error, got %q", stderr)
	}
}

func TestTestFlightBetaTestersExportRejectsGroupAndBuildID(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "export", "--app", "app-1", "--group", "group-1", "--build-id", "build-1", "--output", "/tmp/testers.csv"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --group cannot be combined with --build-id") {
		t.Fatalf("expected conflicting filter error, got %q", stderr)
	}
}

func TestTestFlightBetaTestersAddGroupsOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters/tester-1/relationships/betaGroups" {
			t.Fatalf("expected path /v1/betaTesters/tester-1/relationships/betaGroups, got %s", req.URL.Path)
		}
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		if !strings.Contains(string(payload), `"id":"group-1"`) {
			t.Fatalf("expected group id in body, got %s", string(payload))
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "add-groups", "--id", "tester-1", "--group", "group-1,group-2"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Successfully added tester tester-1") {
		t.Fatalf("expected success message, got %q", stderr)
	}
	if !strings.Contains(stdout, `"action":"added"`) {
		t.Fatalf("expected action added in output, got %q", stdout)
	}
}

func TestTestFlightBetaTestersRemoveGroupsOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters/tester-2/relationships/betaGroups" {
			t.Fatalf("expected path /v1/betaTesters/tester-2/relationships/betaGroups, got %s", req.URL.Path)
		}
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		if !strings.Contains(string(payload), `"id":"group-3"`) {
			t.Fatalf("expected group id in body, got %s", string(payload))
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "remove-groups", "--id", "tester-2", "--group", "group-3", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Successfully removed tester tester-2") {
		t.Fatalf("expected success message, got %q", stderr)
	}
	if !strings.Contains(stdout, `"action":"removed"`) {
		t.Fatalf("expected action removed in output, got %q", stdout)
	}
}

func TestTestFlightBetaTestersInviteOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("expected path /v1/betaTesters, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[apps]") != "app-1" {
				t.Fatalf("expected app filter app-1, got %q", query.Get("filter[apps]"))
			}
			if query.Get("filter[email]") != "tester@example.com" {
				t.Fatalf("expected email filter tester@example.com, got %q", query.Get("filter[email]"))
			}
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("expected path /v1/apps/app-1/betaGroups, got %s", req.URL.Path)
			}
			body := `{"data":[{"type":"betaGroups","id":"group-9","attributes":{"name":"Beta"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("expected path /v1/betaTesters, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"betaTesters","id":"tester-9","attributes":{"email":"tester@example.com"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaTesterInvitations" {
				t.Fatalf("expected path /v1/betaTesterInvitations, got %s", req.URL.Path)
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

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "invite", "--app", "app-1", "--email", "tester@example.com", "--group", "Beta"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"invitationId":"invite-1"`) {
		t.Fatalf("expected invitation id in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"testerId":"tester-9"`) {
		t.Fatalf("expected tester id in output, got %q", stdout)
	}
}
