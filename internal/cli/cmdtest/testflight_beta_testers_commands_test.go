package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestFlightBetaTestersViewPreservesEmptyAppDevices(t *testing.T) {
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

	stdout, stderr, err := runRootWithDeps(t, stubServerClient(t, server), "testflight", "testers", "view", "--id", "tester-1", "--output", "json")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
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
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/betaTesters", "filter[email]=tester@example.com", "filter[apps]=app-1", "fields[betaTesters]=email,firstName,lastName,inviteType,state", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Beta"}}]}`))
	reg.Register(asctest.POST("/v1/betaTesters").JSONBody(`{"data":{"type":"betaTesters","attributes":{"email":"tester@example.com"},"relationships":{"betaGroups":{"data":[{"type":"betaGroups","id":"group-1"}]}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "testflight", "testers", "add", "--app", "app-1", "--email", "tester@example.com", "--group", "Beta")
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
	t.Setenv("ASC_TIMEOUT", "100ms")
	t.Setenv("ASC_MAX_RETRIES", "0")
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
			wantRequests: []string{"GET /v1/apps/app-1/betaGroups", "GET /v1/betaTesters", "POST /v1/betaTesters"},
			want:         `{` + groups + `}`,
		},
		{
			name:         "group name and ID resolve to one relationship",
			args:         []string{"--app", "app-1", "--group", "Beta,group-1"},
			wantRequests: []string{"GET /v1/apps/app-1/betaGroups", "GET /v1/betaTesters", "POST /v1/betaTesters"},
			want:         `{` + groups + `}`,
		},
		{
			name:         "groups and builds",
			args:         []string{"--app", "app-1", "--group", "Beta", "--build-id", "build-1, build-2,build-1"},
			wantRequests: []string{"GET /v1/builds/build-1/relationships/app", "GET /v1/builds/build-2/relationships/app", "GET /v1/apps/app-1/betaGroups", "GET /v1/apps/app-1/betaGroups", "GET /v1/betaTesters", "POST /v1/betaTesters"},
			want:         `{` + groups + `,` + builds + `}`,
		},
		{
			name:         "builds alone need neither a group nor an app",
			args:         []string{"--build-id", "build-1,build-2"},
			wantRequests: []string{"GET /v1/builds/build-1/relationships/app", "GET /v1/builds/build-2/relationships/app", "GET /v1/betaTesters", "POST /v1/betaTesters"},
			want:         `{` + builds + `}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			if slices.Contains(test.args, "--build-id") {
				for _, id := range []string{"build-1", "build-2"} {
					reg.Register(asctest.GET("/v1/builds/"+id+"/relationships/app"), asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`))
				}
			}
			fields := "email"
			if slices.Contains(test.args, "--group") {
				fields = "email,firstName,lastName,inviteType,state"
			}
			reg.Register(asctest.GET("/v1/betaTesters", "filter[email]=tester@example.com", "filter[apps]=app-1", "fields[betaTesters]="+fields, "limit=200"), asctest.JSON(http.StatusOK, `{"data":[]}`))
			if slices.Contains(test.args, "--group") {
				page := asctest.GET("/v1/apps/app-1/betaGroups", "limit=200")
				if slices.Contains(test.args, "--build-id") {
					reg.Register(page, asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":"/v1/apps/app-1/betaGroups?cursor=two"}}`))
					page = asctest.GET("/v1/apps/app-1/betaGroups", "cursor=two")
				}
				reg.Register(page, asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Beta"}}]}`))
			}
			reg.Register(asctest.POST("/v1/betaTesters").JSONBody(`{"data":{"type":"betaTesters","attributes":{"email":"tester@example.com"},"relationships":`+test.want+`}}`),
				asctest.JSON(http.StatusCreated, `{"data":{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}}`))

			args := append([]string{"testflight", "testers", "add", "--email", "tester@example.com"}, test.args...)
			// Each response fits ASC_TIMEOUT while the complete add takes longer.
			client := stubClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				select {
				case <-req.Context().Done():
					return nil, req.Context().Err()
				case <-time.After(60 * time.Millisecond):
					return reg.RoundTrip(req)
				}
			}))
			d, _, stderr := newTestDeps(t, client)
			if code := runWithDepsOnFakeClock(t, d, "1.2.3", args...); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
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
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

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
			assertUsageExitWithDeps(t, nil, test.args, test.wantErr)
		})
	}
}

func TestTestFlightBetaTestersRemoveOutput(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "filter[email]=tester@example.com"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-2","attributes":{"email":"tester@example.com"}}]}`))
	reg.Register(asctest.DELETE("/v1/betaTesters/tester-2"),
		asctest.JSON(http.StatusNoContent, ""))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "testflight", "testers", "remove", "--app", "app-1", "--email", "tester@example.com", "--confirm")
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
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Beta"}}]}`))
	reg.Register(asctest.GET("/v1/betaTesters", "filter[betaGroups]=group-1"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}]}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "testflight", "testers", "list", "--app", "app-1", "--group", "Beta")
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
			name:    "AddRejectsEmailListBeforeAccountLookup",
			args:    []string{"testflight", "testers", "add", "--email", "one@example.com,two@example.com", "--build-id", "build-1"},
			deps:    true,
			wantErr: "Error: invalid email format",
		},
		{
			name:    "ListRejectsGroupAndBuildID",
			args:    []string{"testflight", "testers", "list", "--app", "app-1", "--group", "group-1", "--build-id", "build-1"},
			deps:    true,
			wantErr: "Error: --group cannot be combined with --build-id",
		},
		{
			name:    "ExportRejectsGroupAndBuildID",
			args:    []string{"testflight", "testers", "export", "--app", "app-1", "--group", "group-1", "--build-id", "build-1", "--output", "/tmp/testers.csv"},
			deps:    true,
			wantErr: "Error: --group cannot be combined with --build-id",
		},
	})
}

func TestTestFlightBetaTestersInviteOutput(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "filter[email]=tester@example.com"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-9","attributes":{"name":"Beta"}}]}`))
	reg.Register(asctest.POST("/v1/betaTesters").JSONBody(`{"data":{"type":"betaTesters","attributes":{"email":"tester@example.com"},"relationships":{"betaGroups":{"data":[{"type":"betaGroups","id":"group-9"}]}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"betaTesters","id":"tester-9","attributes":{"email":"tester@example.com"}}}`))
	reg.Register(asctest.POST("/v1/betaTesterInvitations").JSONBody(`{"data":{"type":"betaTesterInvitations","relationships":{"app":{"data":{"type":"apps","id":"app-1"}},"betaTester":{"data":{"type":"betaTesters","id":"tester-9"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"betaTesterInvitations","id":"invite-1"}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "testflight", "testers", "invite", "--app", "app-1", "--email", "tester@example.com", "--group", "Beta")
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

// Apple answers POST /v1/betaTesters with this 409 both when the email is
// already a tester in a requested group and when it cannot join the group at
// all (an internal group and an email outside the team). Captured live on
// 2026-11-22; the two cases return the same body.
const testersAddCannotAssignBody = `{"errors":[{"id":"76f5639e-74a9-425b-ba54-ebc8ba7b6dc8","status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"Tester(s) cannot be assigned","meta":{"associatedErrors":{}}}]}`

func TestTestFlightTestersAddConflictReadsTheTesterBack(t *testing.T) {
	const (
		testerList = `{"data":[{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com","state":"INVITED"}}],"links":{"self":"/v1/betaTesters"}}`
		noTesters  = `{"data":[],"links":{"self":"/v1/betaTesters"}}`
		groups     = `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Internal Testing","isInternalGroup":true}},{"type":"betaGroups","id":"group-2","attributes":{"name":"Beta"}}]}`
	)
	tests := []struct {
		name       string
		group      string
		testers    string
		memberOf   string
		addGroups  string
		groupRead  string
		buildID    string
		buildsOf   string
		wantCode   int
		wantStdout string
		wantStderr string
		notStderr  []string
	}{
		{
			name:       "already in every group",
			group:      "Beta",
			testers:    testerList,
			memberOf:   `{"data":[{"type":"betaGroups","id":"group-2"}],"links":{}}`,
			wantCode:   0,
			wantStdout: `"id":"tester-1"`,
			wantStderr: "testers add: tester@example.com is already tester tester-1 in every requested group and build; left unchanged\n",
		},
		{
			name:       "in one group, missing the other",
			group:      "Internal Testing,Beta",
			testers:    testerList,
			memberOf:   `{"data":[{"type":"betaGroups","id":"group-2"}],"links":{}}`,
			addGroups:  `{"data":[{"type":"betaGroups","id":"group-1"}]}`,
			wantCode:   0,
			wantStdout: `"id":"tester-1"`,
			wantStderr: "testers add: tester@example.com is already tester tester-1; added it to group(s) group-1\n",
		},
		{
			// A rerun adds only what is still missing, but the caller must
			// learn that the group add went through.
			name:       "group added, build add fails",
			group:      "Internal Testing,Beta",
			testers:    testerList,
			memberOf:   `{"data":[{"type":"betaGroups","id":"group-2"}],"links":{}}`,
			addGroups:  `{"data":[{"type":"betaGroups","id":"group-1"}]}`,
			buildID:    "build-1",
			buildsOf:   `{"data":[],"links":{}}`,
			wantCode:   cmd.ExitConflict,
			wantStderr: "tester tester-1 exists; added it to group(s) group-1, but adding build(s) build-1 failed: ",
		},
		{
			name:       "not a tester, internal group",
			group:      "Internal Testing",
			testers:    noTesters,
			groupRead:  `{"data":{"type":"betaGroups","id":"group-1","attributes":{"name":"Internal Testing","isInternalGroup":true}}}`,
			wantCode:   cmd.ExitConflict,
			wantStderr: "Tester(s) cannot be assigned\nHint: tester@example.com is not a tester of app app-1, and \"Internal Testing\" (group-1) is an internal group: internal groups take only users of your App Store Connect team who can see this app. To let the person test without joining the team, add them to an external group. Inviting them to the team (asc users invite) is a different, bigger action that gives them a team role; the Account Holder or an Admin decides it.\n",
			// The hint must not hand over a runnable team invite: an agent
			// asked to add a tester would grant a team role by following it.
			notStderr: []string{"asc users invite --", "--roles", "DEVELOPER"},
		},
		{
			name:       "not a tester, external group keeps Apple's error",
			group:      "Beta",
			testers:    noTesters,
			groupRead:  `{"data":{"type":"betaGroups","id":"group-2","attributes":{"name":"Beta","isInternalGroup":false}}}`,
			wantCode:   cmd.ExitConflict,
			wantStderr: "Error: testers add: failed to create: The request cannot be fulfilled because of the state of another resource.: Tester(s) cannot be assigned\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/apps/app-1/betaGroups", "limit=200"), asctest.JSON(http.StatusOK, groups))
			reg.Register(asctest.GET("/v1/betaTesters", "filter[email]=tester@example.com", "filter[apps]=app-1", "fields[betaTesters]=email,firstName,lastName,inviteType,state", "limit=200"), asctest.JSON(http.StatusOK, noTesters))
			reg.Register(asctest.POST("/v1/betaTesters"), asctest.JSON(http.StatusConflict, testersAddCannotAssignBody))
			reg.Register(asctest.GET("/v1/betaTesters", "filter[email]=tester@example.com", "filter[apps]=app-1", "fields[betaTesters]=email,firstName,lastName,inviteType,state", "limit=200"), asctest.JSON(http.StatusOK, test.testers))
			if test.memberOf != "" {
				reg.Register(asctest.GET("/v1/betaTesters/tester-1/relationships/betaGroups", "limit=200"), asctest.JSON(http.StatusOK, test.memberOf))
			}
			if test.addGroups != "" {
				reg.Register(asctest.POST("/v1/betaTesters/tester-1/relationships/betaGroups").JSONBody(test.addGroups), asctest.JSON(http.StatusNoContent, ""))
			}
			if test.buildsOf != "" {
				reg.Register(asctest.GET("/v1/builds/"+test.buildID+"/relationships/app"), asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`))
				reg.Register(asctest.GET("/v1/builds/"+test.buildID+"/relationships/individualTesters", "limit=200"), asctest.JSON(http.StatusOK, test.buildsOf))
				reg.Register(asctest.POST("/v1/betaTesters/tester-1/relationships/builds"), asctest.JSON(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","title":"Build cannot be assigned","detail":"Build cannot be assigned"}]}`))
			}
			if test.groupRead != "" {
				var group struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				}
				if err := json.Unmarshal([]byte(test.groupRead), &group); err != nil {
					t.Fatal(err)
				}
				reg.Register(asctest.GET("/v1/betaGroups/"+group.Data.ID), asctest.JSON(http.StatusOK, test.groupRead))
			}

			args := []string{"testflight", "testers", "add", "--app", "app-1", "--email", "tester@example.com", "--group", test.group, "--output", "json"}
			if test.buildID != "" {
				args = append(args, "--build-id", test.buildID)
			}
			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), args...)

			if code != test.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, test.wantCode, stderr)
			}
			if test.wantStdout == "" && stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("stdout = %q, want it to contain %q", stdout, test.wantStdout)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.wantStderr)
			}
			for _, unwanted := range test.notStderr {
				if strings.Contains(stderr, unwanted) {
					t.Fatalf("stderr = %q, must not contain %q", stderr, unwanted)
				}
			}
			if strings.Count(stderr, "Hint:") > 1 {
				t.Fatalf("stderr has more than one hint:\n%s", stderr)
			}
		})
	}
}

func TestTestFlightBetaTestersAddRejectsBuildFromAnotherApp(t *testing.T) {
	for _, test := range []struct {
		name, owner, message string
		code                 int
	}{
		{"other app", `{"data":{"type":"apps","id":"other-app"}}`, "does not belong", 2},
		{"missing app", `{"data":null}`, "no verified owning app", 1},
		{"wrong resource", `{"data":{"type":"builds","id":"app-1"}}`, "no verified owning app", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/builds/other-build/relationships/app"), asctest.JSON(http.StatusOK, test.owner))
			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "testflight", "testers", "add", "--app", "app-1", "--email", "tester@example.com", "--group", "New Group", "--create-missing-groups", "--build-id", "other-build")
			if code != test.code || stdout != "" || !strings.Contains(stderr, test.message) {
				t.Fatalf("stdout=%q stderr=%q code=%d", stdout, stderr, code)
			}
			if len(reg.Requests()) != 1 {
				t.Fatalf("expected only build ownership lookup, got %v", reg.Requests())
			}
		})
	}
}

func TestTestFlightTestersAddBuildOnlyReconcilesExistingAndConcurrentTester(t *testing.T) {
	for _, scenario := range []string{"existing", "concurrent", "new", "foreign", "duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			visible := scenario == "existing" || scenario == "duplicate"
			creates, relationshipWrites := 0, 0
			builds := map[string]bool{"build-1": true}
			tester := map[string]any{"type": "betaTesters", "id": "tester-1", "attributes": map[string]any{
				"email": "Tester@Example.com", "firstName": "Private profile", "appDevices": []any{}, "futureAttribute": "private-value",
			}}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				var data any
				switch req.Method + " " + req.URL.Path {
				case "GET /v1/builds/build-1/relationships/app":
					data = map[string]string{"type": "apps", "id": "app-1"}
				case "GET /v1/builds/build-2/relationships/app":
					data = map[string]string{"type": "apps", "id": "app-2"}
				case "GET /v1/betaTesters":
					query := req.URL.Query()
					if query.Get("filter[email]") != "tester@example.com" || !slices.Contains([]string{"app-1", "app-2"}, query.Get("filter[apps]")) || query.Get("fields[betaTesters]") != "email" {
						t.Errorf("unexpected tester scope: %s", req.URL.RawQuery)
					}
					data = []any{}
					if visible {
						data = []any{tester}
					}
					if scenario == "duplicate" {
						data = []any{tester, map[string]any{"type": "betaTesters", "id": "tester-2", "attributes": map[string]string{"email": "tester@example.com"}}}
					}
				case "POST /v1/betaTesters":
					creates++
					if scenario == "concurrent" || scenario == "new" {
						visible = true
					}
					if scenario == "new" {
						builds["build-2"] = true
						w.WriteHeader(http.StatusCreated)
						data = tester
					} else {
						w.WriteHeader(http.StatusConflict)
						if _, err := io.WriteString(w, testersAddCannotAssignBody); err != nil {
							t.Error(err)
						}
						return
					}
				case "GET /v1/builds/build-1/relationships/individualTesters", "GET /v1/builds/build-2/relationships/individualTesters":
					if !visible || scenario == "duplicate" {
						t.Error("read membership without a unique in-scope tester")
					}
					data = []map[string]string{}
					if builds[strings.Split(req.URL.Path, "/")[3]] {
						data = []map[string]string{{"type": "betaTesters", "id": "tester-1"}}
					}
				case "POST /v1/betaTesters/tester-1/relationships/builds":
					relationshipWrites++
					var body struct{ Data []struct{ ID, Type string } }
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if len(body.Data) != 1 || body.Data[0].ID != "build-2" || body.Data[0].Type != "builds" {
						t.Errorf("must add only the missing named build: %+v", body.Data)
					}
					builds["build-2"] = true
					w.WriteHeader(http.StatusNoContent)
					return
				default:
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			for attempt := 0; attempt < 2; attempt++ {
				stdout, stderr, err := runRootWithDeps(t, stubServerClient(t, server), "testflight", "testers", "add",
					"--email", "tester@example.com", "--build-id", "build-1,build-2", "--first-name", "Do not overwrite", "--output", "json")
				if scenario == "foreign" || scenario == "duplicate" {
					wantCreates := 1
					if scenario == "duplicate" {
						wantCreates = 0
					}
					if err == nil || stdout != "" || creates != wantCreates || relationshipWrites != 0 {
						t.Fatalf("unsafe refusal: creates=%d writes=%d stdout=%q stderr=%q err=%v", creates, relationshipWrites, stdout, stderr, err)
					}
					break
				}
				if err != nil {
					t.Fatalf("stdout=%q stderr=%q err=%v", stdout, stderr, err)
				}
				var response struct{ Data json.RawMessage }
				if err := json.Unmarshal([]byte(stdout), &response); err != nil {
					t.Fatal(err)
				}
				var actual, expected any
				if err := json.Unmarshal(response.Data, &actual); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(`{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}`), &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual, expected) {
					t.Fatalf("build-only response must project only scoped identity: %s", response.Data)
				}
				wantCreates, wantWrites := 0, 1
				if scenario == "concurrent" || scenario == "new" {
					wantCreates = 1
				}
				if scenario == "new" {
					wantWrites = 0
				}
				if creates != wantCreates || relationshipWrites != wantWrites {
					t.Fatalf("attempt %d: creates=%d relationship writes=%d", attempt, creates, relationshipWrites)
				}
			}
		})
	}
}
