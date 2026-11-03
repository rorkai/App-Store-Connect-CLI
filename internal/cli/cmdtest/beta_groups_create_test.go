package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBetaGroupsCreateInternalSetsAttributeOnCreate(t *testing.T) {
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
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaGroups" {
				t.Fatalf("expected path /v1/betaGroups, got %s", req.URL.Path)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			if !strings.Contains(string(payload), `"name":"Internal Testers"`) {
				t.Fatalf("expected group name in body, got %s", string(payload))
			}
			if !strings.Contains(string(payload), `"isInternalGroup":true`) {
				t.Fatalf("expected isInternalGroup=true in body, got %s", string(payload))
			}
			for _, field := range []string{"publicLinkEnabled", "publicLinkLimitEnabled", "publicLinkLimit"} {
				if strings.Contains(string(payload), field) {
					t.Fatalf("did not expect internal group create payload to include %s, got %s", field, string(payload))
				}
			}
			if !strings.Contains(string(payload), `"type":"apps"`) || !strings.Contains(string(payload), `"id":"app-1"`) {
				t.Fatalf("expected app relationship in body, got %s", string(payload))
			}

			body := `{"data":{"type":"betaGroups","id":"bg-1","attributes":{"name":"Internal Testers","isInternalGroup":true}}}`
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
		if err := root.Parse([]string{"testflight", "groups", "create", "--app", "app-1", "--name", "Internal Testers", "--internal"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"isInternalGroup":true`) {
		t.Fatalf("expected isInternalGroup in output, got %q", stdout)
	}
}

func TestBetaGroupsCreateWithoutInternalMakesOneCall(t *testing.T) {
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
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaGroups" {
				t.Fatalf("expected path /v1/betaGroups, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"betaGroups","id":"bg-2","attributes":{"name":"Beta"}}}`
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
		if err := root.Parse([]string{"testflight", "groups", "create", "--app", "app-1", "--name", "Beta"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"bg-2"`) {
		t.Fatalf("expected beta group id in output, got %q", stdout)
	}
}

func TestBetaGroupsCreateSendsDistributionControls(t *testing.T) {
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
		if req.Method != http.MethodPost || req.URL.Path != "/v1/betaGroups" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}

		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		body := string(payload)
		for _, attribute := range []string{
			`"hasAccessToAllBuilds":true`,
			`"publicLinkEnabled":true`,
			`"publicLinkLimitEnabled":true`,
			`"publicLinkLimit":250`,
			`"feedbackEnabled":true`,
		} {
			if !strings.Contains(body, attribute) {
				t.Fatalf("expected %s in body, got %s", attribute, body)
			}
		}

		response := `{"data":{"type":"betaGroups","id":"bg-3","attributes":{"name":"Public Preview","isInternalGroup":false,"hasAccessToAllBuilds":true,"feedbackEnabled":true}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(response)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "groups", "create",
			"--app", "app-1",
			"--name", "Public Preview",
			"--access-all-builds",
			"--public-link-enabled",
			"--public-link-limit-enabled",
			"--public-link-limit", "250",
			"--feedback-enabled",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if callCount != 1 {
		t.Fatalf("expected one create request, got %d", callCount)
	}
}

func TestBetaGroupsCreateSendsExplicitFalseControls(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		body := string(payload)
		for _, attribute := range []string{
			`"isInternalGroup":false`,
			`"hasAccessToAllBuilds":false`,
			`"publicLinkEnabled":false`,
			`"publicLinkLimitEnabled":false`,
			`"feedbackEnabled":false`,
		} {
			if !strings.Contains(body, attribute) {
				t.Errorf("expected %s in body, got %s", attribute, body)
			}
		}

		response := `{"data":{"type":"betaGroups","id":"bg-false","attributes":{"name":"Explicit Defaults"}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(response)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "groups", "create",
			"--app", "app-1",
			"--name", "Explicit Defaults",
			"--internal=false",
			"--access-all-builds=false",
			"--public-link-enabled=false",
			"--public-link-limit-enabled=false",
			"--feedback-enabled=false",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestBetaGroupsCreateRejectsInternalPublicLinkControlsBeforeRequest(t *testing.T) {
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
		return nil, errors.New("unexpected request")
	})

	tests := []struct {
		name string
		args []string
	}{
		{name: "enabled", args: []string{"--public-link-enabled"}},
		{name: "limit enabled", args: []string{"--public-link-limit-enabled"}},
		{name: "limit", args: []string{"--public-link-limit", "250"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			args := []string{"testflight", "groups", "create", "--app", "app-1", "--name", "Internal", "--internal"}
			args = append(args, tt.args...)
			var runErr error
			_, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			wantErr := shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--internal")
			if runErr.Error() != wantErr.Error() {
				t.Fatalf("error = %q, want %q", runErr, wantErr)
			}
			if !strings.Contains(stderr, "--internal cannot be combined with public link controls") {
				t.Fatalf("expected conflict diagnostic, got %q", stderr)
			}
			if callCount != 0 {
				t.Fatalf("expected no requests, got %d", callCount)
			}
		})
	}
}

func TestBetaGroupsCreateRejectsInvalidPublicLinkLimitBeforeRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "groups", "create",
			"--app", "app-1",
			"--name", "Beta",
			"--public-link-limit", "10001",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	wantErr := shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--public-link-limit")
	if runErr.Error() != wantErr.Error() {
		t.Fatalf("error = %q, want %q", runErr, wantErr)
	}
	if !strings.Contains(stderr, "--public-link-limit must be between 1 and 10000") {
		t.Fatalf("expected limit diagnostic, got %q", stderr)
	}
}

func createRequestRelationships(t *testing.T, req *http.Request) string {
	t.Helper()

	var payload struct {
		Data struct {
			Relationships json.RawMessage `json:"relationships"`
		} `json:"data"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body error: %v", err)
	}
	return string(payload.Data.Relationships)
}

func TestBetaGroupsCreateSendsRelationshipsInOneRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	const app = `"app":{"data":{"type":"apps","id":"app-1"}}`
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "omitted flags send only the app",
			want: `{` + app + `}`,
		},
		{
			name: "builds",
			args: []string{"--build-id", "build-1, build-2,build-1"},
			want: `{` + app + `,"builds":{"data":[{"type":"builds","id":"build-1"},{"type":"builds","id":"build-2"}]}}`,
		},
		{
			name: "testers",
			args: []string{"--tester", "tester-1,tester-2"},
			want: `{` + app + `,"betaTesters":{"data":[{"type":"betaTesters","id":"tester-1"},{"type":"betaTesters","id":"tester-2"}]}}`,
		},
		{
			name: "builds and testers",
			args: []string{"--build-id", "build-1", "--tester", "tester-1"},
			want: `{` + app + `,"builds":{"data":[{"type":"builds","id":"build-1"}]},"betaTesters":{"data":[{"type":"betaTesters","id":"tester-1"}]}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			callCount := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method == http.MethodGet && req.URL.Path == "/v1/betaGroups" && len(test.args) > 0 {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"data":[],"links":{}}`)),
						Header:     http.Header{"Content-Type": []string{"application/json"}},
					}, nil
				}
				callCount++
				if req.Method != http.MethodPost || req.URL.Path != "/v1/betaGroups" {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
				}
				if got := createRequestRelationships(t, req); got != test.want {
					t.Fatalf("relationships = %s, want %s", got, test.want)
				}
				return &http.Response{
					StatusCode: http.StatusCreated,
					Body:       io.NopCloser(strings.NewReader(`{"data":{"type":"betaGroups","id":"bg-1","attributes":{"name":"Beta"}}}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			})

			args := append([]string{"testflight", "groups", "create", "--app", "app-1", "--name", "Beta"}, test.args...)
			if _, _, err := runCommand(t, args); err != nil {
				t.Fatalf("run error: %v", err)
			}
			if callCount != 1 {
				t.Fatalf("expected one create request, got %d", callCount)
			}
		})
	}
}

// Live App Store Connect created the group even though the create request
// failed with 404 for a missing build, so the error must name that group. A
// same-name group that existed before the create must never be called new.
func TestBetaGroupsCreateFailureWithRelationshipsReportsPartialCreate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	const (
		notFound  = `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'builds' with id 'missing-build'"}]}`
		conflict  = `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE","title":"duplicate","detail":"A group with this name already exists"}]}`
		noGroups  = `{"data":[],"links":{}}`
		oldGroup  = `{"data":[{"type":"betaGroups","id":"bg-old","attributes":{"name":"Beta"}}],"links":{}}`
		oldPaged  = `{"data":[{"type":"betaGroups","id":"bg-old","attributes":{"name":"Beta"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/betaGroups?filter%5Bapp%5D=app-1&filter%5Bname%5D=Beta&cursor=2"}}`
		serverErr = `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"boom","detail":"boom"}]}`
	)
	tests := []struct {
		name         string
		lookups      []string // GET responses in order; "" answers 500
		createStatus int
		createBody   string
		wantErr      error
		wantGets     int
		getsBefore   int // lookups expected before the POST; 0 means 1
		want         []string
		notWant      []string
	}{
		{
			name:         "group appears only after the failed create",
			lookups:      []string{noGroups, `{"data":[{"type":"betaGroups","id":"bg-partial","attributes":{"name":"Beta"}}],"links":{}}`},
			createStatus: http.StatusNotFound,
			createBody:   notFound,
			wantErr:      asc.ErrNotFound,
			wantGets:     2,
			want: []string{
				"There is no resource of type 'builds' with id 'missing-build'",
				`App Store Connect created group "bg-partial" despite the failure`,
				`asc builds add-groups --build-id "missing-build" --group "bg-partial"`,
				`asc testflight groups add-testers --group "bg-partial" --tester "tester-1"`,
				`asc testflight groups delete --id "bg-partial" --confirm`,
			},
		},
		{
			name:         "same-name group existed before the create",
			lookups:      []string{oldGroup, oldGroup},
			createStatus: http.StatusConflict,
			createBody:   conflict,
			wantErr:      asc.ErrConflict,
			wantGets:     2,
			want:         []string{"A group with this name already exists"},
			notWant:      []string{"bg-old", "created", "delete"},
		},
		{
			name:         "new group on a later page",
			lookups:      []string{oldPaged, noGroups, oldPaged, `{"data":[{"type":"betaGroups","id":"bg-partial","attributes":{"name":"Beta"}}],"links":{}}`},
			createStatus: http.StatusNotFound,
			createBody:   notFound,
			wantErr:      asc.ErrNotFound,
			wantGets:     4,
			getsBefore:   2,
			want:         []string{`App Store Connect created group "bg-partial" despite the failure`},
			notWant:      []string{`"bg-old"`},
		},
		{
			name:         "post-create lookup fails",
			lookups:      []string{noGroups, ""},
			createStatus: http.StatusNotFound,
			createBody:   notFound,
			wantErr:      asc.ErrNotFound,
			wantGets:     2,
			want: []string{
				"There is no resource of type 'builds' with id 'missing-build'",
				"App Store Connect may have created the group anyway",
				`asc testflight groups list --app "app-1" --name "Beta"`,
			},
		},
		{
			name:         "pre-create lookup fails",
			lookups:      []string{""},
			createStatus: http.StatusNotFound,
			createBody:   notFound,
			wantErr:      asc.ErrNotFound,
			wantGets:     1,
			want:         []string{"There is no resource of type 'builds' with id 'missing-build'"},
			notWant:      []string{"created", "delete"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gets, posts := 0, 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/v1/betaGroups" {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
				}
				status, body := test.createStatus, test.createBody
				switch req.Method {
				case http.MethodPost:
					posts++
					wantBefore := max(test.getsBefore, 1)
					if gets != wantBefore {
						t.Errorf("expected %d lookups before the create, got %d", wantBefore, gets)
					}
				case http.MethodGet:
					if gets >= len(test.lookups) {
						t.Fatalf("unexpected lookup %d", gets+1)
					}
					query := req.URL.Query()
					if query.Get("filter[app]") != "app-1" || query.Get("filter[name]") != "Beta" {
						t.Fatalf("unexpected lookup query %s", req.URL.RawQuery)
					}
					status, body = http.StatusOK, test.lookups[gets]
					if body == "" {
						status, body = http.StatusInternalServerError, serverErr
					}
					gets++
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
				}
				return &http.Response{
					StatusCode: status,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			})

			_, _, err := runCommand(t, []string{
				"testflight", "groups", "create", "--app", "app-1", "--name", "Beta",
				"--build-id", "missing-build", "--tester", "tester-1",
			})
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("expected the create's error to be kept, got %v", err)
			}
			for _, want := range test.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error missing %q:\n%s", want, err)
				}
			}
			for _, notWant := range test.notWant {
				if strings.Contains(err.Error(), notWant) {
					t.Errorf("error should not contain %q:\n%s", notWant, err)
				}
			}
			if posts != 1 || gets != test.wantGets {
				t.Fatalf("got %d creates and %d lookups, want 1 and %d", posts, gets, test.wantGets)
			}
		})
	}
}
