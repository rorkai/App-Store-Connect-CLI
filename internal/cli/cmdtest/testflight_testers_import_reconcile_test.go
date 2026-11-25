package cmdtest

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestTestersImportRetainsValidRowsAndOriginalFailureNumbers(t *testing.T) {
	for _, tc := range []struct {
		name, rows   string
		stop         bool
		total        int
		wantEmails   []string
		wantFailures []int
	}{
		{"continue", "not-an-email,Ignored\none@example.com,\nONE@example.com,Ignored\ntwo@example.com,\n", false, 4, []string{"one@example.com", "two@example.com"}, []int{1, 3}},
		{"unresolved CSV group", "unknown@example.com,Missing\ntwo@example.com,\n", false, 2, []string{"two@example.com"}, []int{1}},
		{"stop", "one@example.com,\nnot-an-email,Ignored\ntwo@example.com,SkippedGroup\n", true, 3, []string{"one@example.com"}, []int{2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "testers.csv")
			if err := os.WriteFile(path, []byte("email,groups\n"+tc.rows), 0o600); err != nil {
				t.Fatal(err)
			}
			var added []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "GET /v1/apps/a1/betaGroups":
					if tc.name != "unresolved CSV group" {
						t.Errorf("read groups for a rejected/skipped row")
					}
					_, _ = w.Write([]byte(`{"data":[]}`))
				case "GET /v1/betaTesters":
					if r.URL.Query().Get("filter[apps]") != "a1" || r.URL.Query().Get("filter[email]") != strings.Join(tc.wantEmails, ",") {
						t.Error(r.URL)
					}
					_, _ = w.Write([]byte(`{"data":[]}`))
				case "POST /v1/betaTesters":
					var body asc.BetaTesterCreateRequest
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					email := body.Data.Attributes.Email
					added = append(added, email)
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": email, "type": "betaTesters", "attributes": map[string]string{"email": email}}})
				default:
					t.Errorf("unexpected request for a rejected/skipped row: %s %s", r.Method, r.URL)
					w.WriteHeader(400)
				}
			}))
			defer srv.Close()
			args := []string{"testflight", "testers", "import", "--app", "a1", "--input", path, "--confirm"}
			if tc.name != "unresolved CSV group" {
				args = append(args, "--create-missing-groups")
			}
			if tc.stop {
				args = append(args, "--continue-on-error=false")
			}
			stdout, stderr, err := runRootWithDeps(t, stubServerClient(t, srv), args...)
			var result asc.BetaTestersImportSummary
			if _, ok := errors.AsType[ReportedError](err); !ok || json.Unmarshal([]byte(stdout), &result) != nil || result.Total != tc.total || result.Created != len(tc.wantEmails) || result.Failed != len(tc.wantFailures) || len(result.Failures) != len(tc.wantFailures) || strings.Join(added, ",") != strings.Join(tc.wantEmails, ",") {
				t.Fatalf("added=%v stdout=%s stderr=%s err=%v", added, stdout, stderr, err)
			}
			for i, row := range tc.wantFailures {
				if result.Failures[i].Row != row {
					t.Fatalf("failures=%+v", result.Failures)
				}
			}
			if tc.name == "continue" && !strings.Contains(result.Failures[1].Error, "already seen at row 2") {
				t.Fatalf("duplicate provenance lost: %+v", result.Failures)
			}
		})
	}
}

func TestTestersImportCreatesMissingGroupAndReconcilesMembership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testers.csv")
	if err := os.WriteFile(path, []byte("email,groups\nnew@example.com,Beta\nexisting@example.com,Beta\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	groups := []map[string]any{}
	testers := []map[string]any{{"id": "t1", "type": "betaTesters", "attributes": map[string]any{"email": "existing@example.com"}}}
	memberships := map[string]bool{}
	writes := 0
	lookupEmails := "existing@example.com,new@example.com"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var data any
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/apps/a1/betaGroups":
			data = groups
		case r.Method == "GET" && r.URL.Path == "/v1/betaTesters":
			if r.URL.Query().Get("filter[apps]") != "a1" || r.URL.Query().Get("filter[email]") != lookupEmails || r.URL.Query().Get("fields[betaTesters]") != "email,firstName,lastName,inviteType,state" {
				t.Errorf("lookup must use input emails within the selected app: %s", r.URL.RawQuery)
			}
			data = testers
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/relationships/betaGroups"):
			id := strings.Split(r.URL.Path, "/")[3]
			links := []map[string]string{}
			if memberships[id] {
				links = append(links, map[string]string{"type": "betaGroups", "id": "g1"})
			}
			data = links
		case r.Method == "POST":
			writes++
			var body struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			switch r.URL.Path {
			case "/v1/betaGroups":
				var group asc.BetaGroupCreateData
				if err := json.Unmarshal(body.Data, &group); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				if group.Attributes.IsInternalGroup == nil || *group.Attributes.IsInternalGroup || group.Attributes.PublicLinkEnabled == nil || *group.Attributes.PublicLinkEnabled {
					t.Error("group must be external with public link disabled")
				}
				groups = append(groups, map[string]any{"id": "g1", "type": "betaGroups", "attributes": map[string]any{"name": "Beta", "isInternalGroup": false}})
				data = groups[0]
			case "/v1/betaTesters":
				testers = append(testers, map[string]any{"id": "t2", "type": "betaTesters", "attributes": map[string]any{"email": "new@example.com"}})
				memberships["t2"] = true
				data = testers[1]
			case "/v1/betaTesters/t1/relationships/betaGroups":
				memberships["t1"] = true
				w.WriteHeader(http.StatusNoContent)
				return
			default:
				t.Errorf("unexpected POST %s", r.URL.Path)
				w.WriteHeader(500)
				return
			}
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client := stubServerClient(t, server)
	for _, tc := range []struct {
		mode                     string
		writes, created, updated int
	}{{"--dry-run", 0, 1, 1}, {"--confirm", 3, 1, 1}, {"--confirm", 3, 0, 0}} {
		stdout, stderr, err := runRootWithDeps(t, client, "testflight", "testers", "import", "--app", "a1", "--input", path, "--create-missing-groups", tc.mode)
		if err != nil {
			t.Fatalf("err=%v stderr=%s", err, stderr)
		}
		var summary asc.BetaTestersImportSummary
		if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
			t.Fatal(err)
		}
		if writes != tc.writes || summary.Created != tc.created || summary.Updated != tc.updated {
			t.Fatalf("summary=%+v writes=%d", summary, writes)
		}
	}
	groups = nil
	testers = testers[:1]
	memberships = map[string]bool{}
	writes = 0
	for _, email := range []string{"existing@example.com", "new@example.com"} {
		lookupEmails = email
		for attempt := 0; attempt < 2; attempt++ {
			before := writes
			stdout, stderr, err := runRootWithDeps(t, client, "testflight", "testers", "add", "--app", "a1", "--email", email, "--group", "Beta", "--create-missing-groups")
			if err != nil || !strings.Contains(stdout, email) {
				t.Fatalf("direct add: stdout=%q stderr=%q err=%v", stdout, stderr, err)
			}
			if attempt == 1 && writes != before {
				t.Fatalf("direct add repeat sent %d writes", writes-before)
			}
		}
	}
	if writes != 3 {
		t.Fatalf("direct add needs one group, one membership and one tester; got %d writes", writes)
	}
}

func TestTestersCreateMissingGroupsPreservesNameMatching(t *testing.T) {
	const overlappingGroups = `{"data":[{"type":"betaGroups","id":"g1","attributes":{"name":"Core Team"}},{"type":"betaGroups","id":"g2","attributes":{"name":"Core  Team"}}]}`
	for _, command := range []string{"add", "import"} {
		for _, tc := range []struct {
			name, selector, groups     string
			ambiguous, withoutCreation bool
		}{
			{name: "normalized whitespace", selector: "Core  Team", groups: `{"data":[{"type":"betaGroups","id":"g1","attributes":{"name":"Core Team"}}]}`},
			{name: "exact name wins", selector: "Core Team", groups: overlappingGroups},
			{name: "ambiguous normalization", selector: "Core   Team", groups: overlappingGroups, ambiguous: true},
			{name: "missing before ambiguous", selector: "Missing;Core   Team", groups: overlappingGroups, ambiguous: true, withoutCreation: true},
			{name: "ambiguous before missing", selector: "Core   Team;Missing", groups: overlappingGroups, ambiguous: true, withoutCreation: true},
		} {
			if tc.withoutCreation && command != "import" {
				continue
			}
			t.Run(command+"/"+tc.name, func(t *testing.T) {
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
				t.Setenv("ASC_APP_ID", "")
				reg := asctest.NewRegistry(t)
				reg.Register(asctest.GET("/v1/apps/a1/betaGroups", "limit=200"), asctest.JSON(http.StatusOK, tc.groups))
				if !tc.ambiguous {
					reg.Register(asctest.GET("/v1/betaTesters", "filter[email]=existing@example.com", "filter[apps]=a1", "fields[betaTesters]=email,firstName,lastName,inviteType,state", "limit=200"),
						asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"t1","attributes":{"email":"existing@example.com"}}]}`))
					reg.Register(asctest.GET("/v1/betaTesters/t1/relationships/betaGroups", "limit=200"),
						asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"g1"}]}`))
				}
				args := []string{"testflight", "testers", command, "--app", "a1"}
				if !tc.withoutCreation {
					args = append(args, "--create-missing-groups")
				}
				if command == "add" {
					selector := tc.selector
					if tc.ambiguous {
						selector += ",Unresolved"
					}
					args = append(args, "--email", "existing@example.com", "--group", selector)
				} else {
					path := filepath.Join(t.TempDir(), "testers.csv")
					rows := "existing@example.com," + tc.selector + "\n"
					if tc.withoutCreation {
						rows += "other@example.com,\n"
					}
					if err := os.WriteFile(path, []byte("email,groups\n"+rows), 0o600); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--input", path, "--confirm")
				}
				stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), args...)
				if tc.ambiguous {
					if err == nil || stdout != "" || !strings.Contains(err.Error(), "2 groups match") {
						t.Fatalf("stdout=%q stderr=%q err=%v", stdout, stderr, err)
					}
					ambiguous, ok := errors.AsType[*shared.AmbiguousSelectionError](err)
					if !ok {
						t.Fatalf("expected typed ambiguity, got %v", err)
					}
					if command == "import" {
						if ambiguous.Flag != "" || !strings.Contains(ambiguous.Hint, "CSV groups column") {
							t.Fatalf("CSV ambiguity gave flag guidance: %v", err)
						}
					} else if ambiguous.Flag != "--group" {
						t.Fatalf("flag ambiguity lost --group guidance: %v", err)
					}
				} else if err != nil || stdout == "" {
					t.Fatalf("stdout=%q stderr=%q err=%v", stdout, stderr, err)
				}
				for _, request := range reg.Requests() {
					if request.Method != http.MethodGet {
						t.Fatalf("unexpected mutation: %s %s", request.Method, request.URL)
					}
				}
			})
		}
	}
}

func TestTestersReconciliationPreservesSelectedAppScope(t *testing.T) {
	for _, command := range []string{"add", "import"} {
		for _, selector := range []string{"explicit", "environment", "configured"} {
			for _, scenario := range []string{"existing", "concurrent", "concurrent skip existing", "foreign", "duplicate"} {
				skipExisting := scenario == "concurrent skip existing"
				if command == "add" && skipExisting {
					continue
				}
				t.Run(command+"/"+selector+"/"+scenario, func(t *testing.T) {
					folder := t.TempDir()
					configPath := filepath.Join(folder, "config.json")
					t.Setenv("ASC_APP_ID", "")
					t.Setenv("ASC_CONFIG_PATH", configPath)
					args := []string{"testflight", "testers", command}
					switch selector {
					case "explicit":
						args = append(args, "--app", "app-1")
					case "environment":
						t.Setenv("ASC_APP_ID", "app-1")
					case "configured":
						if err := os.Unsetenv("ASC_APP_ID"); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(configPath, []byte(`{"app_id":"app-1"}`), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					if command == "add" {
						args = append(args, "--email", "Tester@Example.com", "--group", "Beta")
					} else {
						input := filepath.Join(folder, "testers.csv")
						if err := os.WriteFile(input, []byte(`email,groups
Tester@Example.com,Beta
`), 0o600); err != nil {
							t.Fatal(err)
						}
						args = append(args, "--input", input, "--confirm")
						if skipExisting {
							args = append(args, "--skip-existing", "--invite")
						}
					}
					visible := scenario == "existing" || scenario == "duplicate"
					member := scenario == "existing"
					creates, writes, membershipReads := 0, 0, 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						var data any
						switch r.Method + " " + r.URL.Path {
						case "GET /v1/apps/app-1/betaGroups":
							data = []any{map[string]any{"type": "betaGroups", "id": "g1", "attributes": map[string]any{"name": "Beta", "isInternalGroup": false}}}
						case "GET /v1/betaGroups/g1":
							data = map[string]any{"type": "betaGroups", "id": "g1", "attributes": map[string]any{"name": "Beta", "isInternalGroup": false}}
						case "GET /v1/betaTesters":
							if r.URL.Query().Get("filter[apps]") != "app-1" || r.URL.Query().Get("filter[email]") != "Tester@Example.com" {
								t.Errorf("lookup escaped selected app: %s", r.URL.RawQuery)
							}
							data = []any{}
							if visible {
								data = []any{map[string]any{"type": "betaTesters", "id": "selected-tester", "attributes": map[string]string{"email": "tester@example.com"}}}
							}
							if scenario == "duplicate" {
								data = append(data.([]any), map[string]any{"type": "betaTesters", "id": "another-selected-tester", "attributes": map[string]string{"email": "tester@example.com"}})
							}
						case "POST /v1/betaTesters":
							creates++
							if scenario == "concurrent" || skipExisting {
								visible = true
							}
							w.WriteHeader(http.StatusConflict)
							if _, err := w.Write([]byte(testersAddCannotAssignBody)); err != nil {
								t.Error(err)
							}
							return
						case "GET /v1/betaTesters/selected-tester/relationships/betaGroups":
							membershipReads++
							if !visible || scenario == "duplicate" {
								t.Error("read membership before unique scoped identity")
							}
							data = []map[string]string{}
							if member {
								data = []map[string]string{{"type": "betaGroups", "id": "g1"}}
							}
						case "POST /v1/betaTesters/selected-tester/relationships/betaGroups":
							writes++
							member = true
							w.WriteHeader(http.StatusNoContent)
							return
						default:
							t.Errorf("unrelated resource request: %s %s", r.Method, r.URL.String())
							http.Error(w, "unrelated resource", http.StatusBadRequest)
							return
						}
						if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
							t.Error(err)
						}
					}))
					defer server.Close()
					for attempt := 0; attempt < 2; attempt++ {
						stdout, stderr, err := runRootWithDeps(t, stubServerClient(t, server), args...)
						if scenario == "foreign" || scenario == "duplicate" {
							wantCreates := 1
							if scenario == "duplicate" {
								wantCreates = 0
							}
							if err == nil || creates != wantCreates || writes != 0 || membershipReads != 0 || strings.Contains(stdout+stderr, "foreign-tester") {
								t.Fatalf("unsafe refusal: creates=%d writes=%d reads=%d stdout=%s stderr=%s err=%v", creates, writes, membershipReads, stdout, stderr, err)
							}
							break
						}
						wantCreates, wantWrites := 0, 0
						if scenario == "concurrent" {
							wantCreates, wantWrites = 1, 1
						}
						if skipExisting {
							wantCreates = 1
							var summary asc.BetaTestersImportSummary
							if parseErr := json.Unmarshal([]byte(stdout), &summary); parseErr != nil {
								t.Fatal(parseErr)
							}
							if summary.Existed != 1 || summary.Created != 0 || summary.Updated != 0 || summary.Invited != 0 || summary.Failed != 0 || membershipReads != 0 {
								t.Fatalf("skip-existing receipt=%+v membershipReads=%d", summary, membershipReads)
							}
						}
						if err != nil || creates != wantCreates || writes != wantWrites {
							t.Fatalf("repeat %d: creates=%d writes=%d stdout=%s stderr=%s err=%v", attempt, creates, writes, stdout, stderr, err)
						}
					}
				})
			}
		}
	}
}

func TestTestersImportContinuesAfterGroupCreationFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testers.csv")
	if err := os.WriteFile(path, []byte("email,groups\none@example.com,Missing Group;Existing\ntwo@example.com,Existing\nthree@example.com,missing  group\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		args       []string
		created    int
		failedRows []int
	}{
		{name: "default continues", created: 1, failedRows: []int{1, 3}},
		{name: "global group blocks all rows", args: []string{"--group", "Missing Group"}, failedRows: []int{1, 2, 3}},
		{name: "fail fast stops before tester writes", args: []string{"--continue-on-error=false"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/apps/a1/betaGroups", "limit=200"), asctest.JSON(200, `{"data":[{"type":"betaGroups","id":"g1","attributes":{"name":"Existing"}}]}`))
			reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=a1", "filter[email]=one@example.com,three@example.com,two@example.com", "fields[betaTesters]=email,firstName,lastName,inviteType,state", "limit=200"), asctest.JSON(200, `{"data":[]}`))
			reg.Register(asctest.POST("/v1/betaGroups"), asctest.JSON(403, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Cannot create group"}]}`))
			if tc.created > 0 {
				reg.Register(asctest.POST("/v1/betaTesters").JSONBody(`{"data":{"type":"betaTesters","attributes":{"email":"two@example.com"},"relationships":{"betaGroups":{"data":[{"type":"betaGroups","id":"g1"}]}}}}`),
					asctest.JSON(201, `{"data":{"type":"betaTesters","id":"t2"}}`))
			}
			args := append([]string{"testflight", "testers", "import", "--app", "a1", "--input", path, "--create-missing-groups", "--confirm"}, tc.args...)
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), args...)
			var summary asc.BetaTestersImportSummary
			if parseErr := json.Unmarshal([]byte(stdout), &summary); parseErr != nil || err == nil {
				t.Fatalf("stdout=%q stderr=%q err=%v parseErr=%v", stdout, stderr, err, parseErr)
			}
			if len(summary.Groups) != 1 || summary.Groups[0].Name != "Missing Group" || summary.Groups[0].Status != "failed" || !strings.Contains(summary.Groups[0].Error, "Cannot create group") {
				t.Fatalf("unexpected group failure: %+v", summary.Groups)
			}
			if summary.Created != tc.created || summary.Failed != len(tc.failedRows) || len(summary.Failures) != len(tc.failedRows) {
				t.Fatalf("unexpected row results: %+v", summary)
			}
			emails := []string{"one@example.com", "two@example.com", "three@example.com"}
			for i, row := range tc.failedRows {
				if failure := summary.Failures[i]; failure.Row != row || failure.Email != emails[row-1] {
					t.Fatalf("unexpected failed row: %+v", failure)
				}
			}
		})
	}
}

func TestTestersImportResolvesCommaGroupBeforePlanningCreates(t *testing.T) {
	for _, tc := range []struct {
		name, groupResources, memberResources string
	}{
		{name: "exported single group", groupResources: `{"type":"betaGroups","id":"both","attributes":{"name":"Staff, Family & Friends"}}`, memberResources: `{"type":"betaGroups","id":"both"}`},
		{name: "legacy comma list", groupResources: `{"type":"betaGroups","id":"staff","attributes":{"name":"Staff"}},{"type":"betaGroups","id":"friends","attributes":{"name":"Family & Friends"}}`, memberResources: `{"type":"betaGroups","id":"staff"},{"type":"betaGroups","id":"friends"}`},
		{name: "whole name and component names", groupResources: `{"type":"betaGroups","id":"both","attributes":{"name":"Staff, Family & Friends"}},{"type":"betaGroups","id":"staff","attributes":{"name":"Staff"}},{"type":"betaGroups","id":"friends","attributes":{"name":"Family & Friends"}}`, memberResources: `{"type":"betaGroups","id":"both"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "testers.csv")
			if err := os.WriteFile(path, []byte("email,first_name,last_name,groups\nperson@example.com,,,\"Staff, Family & Friends\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/apps/a1/betaGroups", "limit=200"), asctest.JSON(200, `{"data":[`+tc.groupResources+`]}`))
			reg.Register(asctest.GET("/v1/betaTesters", "filter[email]=person@example.com", "filter[apps]=a1", "fields[betaTesters]=email,firstName,lastName,inviteType,state", "limit=200"), asctest.JSON(200, `{"data":[{"type":"betaTesters","id":"t1","attributes":{"email":"person@example.com"}}]}`))
			reg.Register(asctest.GET("/v1/betaTesters/t1/relationships/betaGroups", "limit=200"), asctest.JSON(200, `{"data":[`+tc.memberResources+`]}`))
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "testflight", "testers", "import", "--app", "a1", "--input", path, "--create-missing-groups", "--dry-run")
			var out asc.BetaTestersImportSummary
			if err != nil || json.Unmarshal([]byte(stdout), &out) != nil || len(out.Groups) != 0 || out.Existed != 1 || out.Updated != 0 || out.Failed != 0 {
				t.Fatalf("import of existing membership: %s, stderr=%s err=%v", stdout, stderr, err)
			}
		})
	}
}
