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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBetaGroupsCreateInternalSetsAttributeOnCreate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/betaGroups").JSONBody(`{"data":{"type":"betaGroups","attributes":{"name":"Internal Testers","isInternalGroup":true},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"betaGroups","id":"bg-1","attributes":{"name":"Internal Testers","isInternalGroup":true}}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "groups", "create", "--app", "app-1", "--name", "Internal Testers", "--internal"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/betaGroups").JSONBody(`{"data":{"type":"betaGroups","attributes":{"name":"Beta"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"betaGroups","id":"bg-2","attributes":{"name":"Beta"}}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "groups", "create", "--app", "app-1", "--name", "Beta"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"bg-2"`) {
		t.Fatalf("expected beta group id in output, got %q", stdout)
	}
}

func TestBetaGroupsCreateSendsDistributionControls(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantAttributes string
	}{
		{
			name: "enabled",
			args: []string{
				"--name", "Public Preview",
				"--access-all-builds",
				"--public-link-enabled",
				"--public-link-limit-enabled",
				"--public-link-limit", "250",
				"--feedback-enabled",
			},
			wantAttributes: `{"name":"Public Preview","hasAccessToAllBuilds":true,"publicLinkEnabled":true,"publicLinkLimitEnabled":true,"publicLinkLimit":250,"feedbackEnabled":true}`,
		},
		{
			name: "explicit false",
			args: []string{
				"--name", "Explicit Defaults",
				"--internal=false",
				"--access-all-builds=false",
				"--public-link-enabled=false",
				"--public-link-limit-enabled=false",
				"--feedback-enabled=false",
			},
			wantAttributes: `{"name":"Explicit Defaults","isInternalGroup":false,"hasAccessToAllBuilds":false,"publicLinkEnabled":false,"publicLinkLimitEnabled":false,"feedbackEnabled":false}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.POST("/v1/betaGroups").JSONBody(`{"data":{"type":"betaGroups","attributes":`+test.wantAttributes+`,"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`),
				asctest.JSON(http.StatusCreated, `{"data":{"type":"betaGroups","id":"bg-3","attributes":{"name":"Public Preview"}}}`))

			_, stderr, err := runCommand(t, append([]string{"testflight", "groups", "create", "--app", "app-1"}, test.args...))
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if callCount := len(reg.Requests()); callCount != 1 {
				t.Fatalf("expected one create request, got %d", callCount)
			}
		})
	}
}

func TestBetaGroupsCreateRejectsInternalPublicLinkControlsBeforeRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()

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
			if callCount := len(reg.Requests()); callCount != 0 {
				t.Fatalf("expected no requests, got %d", callCount)
			}
		})
	}
}

func TestBetaGroupsCreateRejectsInvalidPublicLinkLimitBeforeRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	asctest.NewRegistry(t).Install()

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

func createRequestRelationships(t *testing.T, body []byte) string {
	t.Helper()

	var payload struct {
		Data struct {
			Relationships json.RawMessage `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode body error: %v", err)
	}
	return string(payload.Data.Relationships)
}

func TestBetaGroupsCreateSendsRelationshipsInOneRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

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
			reg := asctest.NewRegistry(t)
			reg.Install()
			if len(test.args) > 0 {
				reg.Register(asctest.GET("/v1/betaGroups", "filter[app]=app-1", "filter[name]=Beta", "limit=200"),
					asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
			}
			reg.Register(asctest.POST("/v1/betaGroups").JSONBody(`{"data":{"type":"betaGroups","attributes":{"name":"Beta"},"relationships":`+test.want+`}}`),
				asctest.JSON(http.StatusCreated, `{"data":{"type":"betaGroups","id":"bg-1","attributes":{"name":"Beta"}}}`))

			args := append([]string{"testflight", "groups", "create", "--app", "app-1", "--name", "Beta"}, test.args...)
			if _, _, err := runCommand(t, args); err != nil {
				t.Fatalf("run error: %v", err)
			}
			callCount := 0
			for _, req := range reg.Requests() {
				if req.Method != http.MethodPost {
					continue
				}
				callCount++
				if got := createRequestRelationships(t, req.Body); got != test.want {
					t.Fatalf("relationships = %s, want %s", got, test.want)
				}
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
			want: []string{
				"There is no resource of type 'builds' with id 'missing-build'",
				"App Store Connect may have created the group anyway",
				`asc testflight groups list --app "app-1" --name "Beta"`,
			},
			notWant: []string{"created group", "delete"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Install()
			wantBefore := max(test.getsBefore, 1)
			for index, lookup := range test.lookups {
				if index == wantBefore {
					registerBetaGroupsCreateWithRelationships(reg, test.createStatus, test.createBody)
				}
				query := []string{"filter[app]=app-1", "filter[name]=Beta", "limit=200"}
				if index > 0 && test.lookups[index-1] == oldPaged {
					query = []string{"filter[app]=app-1", "filter[name]=Beta", "cursor=2"}
				}
				status, body := http.StatusOK, lookup
				if body == "" {
					status, body = http.StatusInternalServerError, serverErr
				}
				reg.Register(asctest.GET("/v1/betaGroups", query...), asctest.JSON(status, body))
			}
			if len(test.lookups) == wantBefore {
				registerBetaGroupsCreateWithRelationships(reg, test.createStatus, test.createBody)
			}

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
			gets, posts := 0, 0
			for _, req := range reg.Requests() {
				switch req.Method {
				case http.MethodPost:
					posts++
					if gets != wantBefore {
						t.Errorf("expected %d lookups before the create, got %d", wantBefore, gets)
					}
				case http.MethodGet:
					gets++
				}
			}
			if posts != 1 || gets != test.wantGets {
				t.Fatalf("got %d creates and %d lookups, want 1 and %d", posts, gets, test.wantGets)
			}
		})
	}
}

func registerBetaGroupsCreateWithRelationships(reg *asctest.Registry, status int, body string) {
	reg.Register(asctest.POST("/v1/betaGroups").JSONBody(`{"data":{"type":"betaGroups","attributes":{"name":"Beta"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}},"builds":{"data":[{"type":"builds","id":"missing-build"}]},"betaTesters":{"data":[{"type":"betaTesters","id":"tester-1"}]}}}}`),
		asctest.JSON(status, body))
}
