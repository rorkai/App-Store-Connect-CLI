package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Beta"}}]}`))
	reg.Register(asctest.POST("/v1/betaTesters").JSONBody(`{"data":{"type":"betaTesters","attributes":{"email":"tester@example.com"},"relationships":{"betaGroups":{"data":[{"type":"betaGroups","id":"group-1"}]}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "testers", "add", "--app", "app-1", "--email", "tester@example.com", "--group", "Beta"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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
			name:         "group name and ID resolve to one relationship",
			args:         []string{"--app", "app-1", "--group", "Beta,group-1"},
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
			reg := asctest.NewRegistry(t)
			reg.Install()
			if len(test.wantRequests) > 1 {
				reg.Register(asctest.GET("/v1/apps/app-1/betaGroups", "limit=200"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Beta"}}]}`))
			}
			reg.Register(asctest.POST("/v1/betaTesters").JSONBody(`{"data":{"type":"betaTesters","attributes":{"email":"tester@example.com"},"relationships":`+test.want+`}}`),
				asctest.JSON(http.StatusCreated, `{"data":{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}}`))

			args := append([]string{"testflight", "testers", "add", "--email", "tester@example.com"}, test.args...)
			if _, _, err := runCommand(t, args); err != nil {
				t.Fatalf("run error: %v", err)
			}
			var requests []string
			for _, req := range reg.Requests() {
				requests = append(requests, req.Method+" "+req.URL.Path)
				if req.Method == http.MethodPost {
					if got := createRequestRelationships(t, req.Body); got != test.want {
						t.Fatalf("relationships = %s, want %s", got, test.want)
					}
				}
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

	asctest.NewRegistry(t).Install()

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "filter[email]=tester@example.com"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-2","attributes":{"email":"tester@example.com"}}]}`))
	reg.Register(asctest.DELETE("/v1/betaTesters/tester-2"),
		asctest.JSON(http.StatusNoContent, ""))

	stdout, stderr, err := runCommand(t, []string{"testflight", "testers", "remove", "--app", "app-1", "--email", "tester@example.com", "--confirm"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Beta"}}]}`))
	reg.Register(asctest.GET("/v1/betaTesters", "filter[betaGroups]=group-1"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}]}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "testers", "list", "--app", "app-1", "--group", "Beta"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"tester-1"`) {
		t.Fatalf("expected tester id in output, got %q", stdout)
	}
}

func TestTestFlightBetaTestersSelectorUsageErrors(t *testing.T) {
	runUsageExitCases(t, []usageExitCase{
		{
			name:    "ListRejectsGroupAndBuildID",
			args:    []string{"testflight", "testers", "list", "--app", "app-1", "--group", "group-1", "--build-id", "build-1"},
			wantErr: "Error: --group cannot be combined with --build-id",
		},
		{
			name:    "ExportRejectsGroupAndBuildID",
			args:    []string{"testflight", "testers", "export", "--app", "app-1", "--group", "group-1", "--build-id", "build-1", "--output", "/tmp/testers.csv"},
			wantErr: "Error: --group cannot be combined with --build-id",
		},
	})
}

func TestTestFlightBetaTestersInviteOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "filter[email]=tester@example.com"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-9","attributes":{"name":"Beta"}}]}`))
	reg.Register(asctest.POST("/v1/betaTesters").JSONBody(`{"data":{"type":"betaTesters","attributes":{"email":"tester@example.com"},"relationships":{"betaGroups":{"data":[{"type":"betaGroups","id":"group-9"}]}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"betaTesters","id":"tester-9","attributes":{"email":"tester@example.com"}}}`))
	reg.Register(asctest.POST("/v1/betaTesterInvitations").JSONBody(`{"data":{"type":"betaTesterInvitations","relationships":{"app":{"data":{"type":"apps","id":"app-1"}},"betaTester":{"data":{"type":"betaTesters","id":"tester-9"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"betaTesterInvitations","id":"invite-1"}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "testers", "invite", "--app", "app-1", "--email", "tester@example.com", "--group", "Beta"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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
