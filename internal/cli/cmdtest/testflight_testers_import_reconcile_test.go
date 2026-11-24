package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestersImportValidatesAllRowsBeforeRequests(t *testing.T) {
	for _, tail := range []string{"not-an-email,Beta", "person@example.com,Beta"} {
		path := filepath.Join(t.TempDir(), "testers.csv")
		if err := os.WriteFile(path, []byte("email,groups\nperson@example.com,Beta\n"+tail+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, err := runRootWithDeps(t, stubClient(t, asctest.NewRegistry(t)), "testflight", "testers", "import", "--app", "a1", "--input", path, "--create-missing-groups", "--confirm")
		if !errors.Is(err, flag.ErrHelp) || stdout != "" || !strings.Contains(stderr, "CSV row 2") {
			t.Fatalf("stdout=%q stderr=%q err=%v", stdout, stderr, err)
		}
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
			if r.URL.Query().Get("filter[apps]") != "" || r.URL.Query().Get("filter[email]") != lookupEmails || r.URL.Query().Get("fields[betaTesters]") != "email,firstName,lastName,inviteType,state" {
				t.Errorf("lookup must use only input emails across the account: %s", r.URL.RawQuery)
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
	for _, command := range []string{"add", "import"} {
		for _, tc := range []struct {
			name, selector, groups string
			ambiguous              bool
		}{
			{name: "normalized whitespace", selector: "Core  Team", groups: `{"data":[{"type":"betaGroups","id":"g1","attributes":{"name":"Core Team"}}]}`},
			{name: "exact name wins", selector: "Core Team", groups: `{"data":[{"type":"betaGroups","id":"g1","attributes":{"name":"Core Team"}},{"type":"betaGroups","id":"g2","attributes":{"name":"Core  Team"}}]}`},
			{name: "ambiguous normalization", selector: "Core   Team", groups: `{"data":[{"type":"betaGroups","id":"g1","attributes":{"name":"Core Team"}},{"type":"betaGroups","id":"g2","attributes":{"name":"Core  Team"}}]}`, ambiguous: true},
		} {
			t.Run(command+"/"+tc.name, func(t *testing.T) {
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
				t.Setenv("ASC_APP_ID", "")
				reg := asctest.NewRegistry(t)
				reg.Register(asctest.GET("/v1/apps/a1/betaGroups", "limit=200"), asctest.JSON(http.StatusOK, tc.groups))
				if !tc.ambiguous {
					reg.Register(asctest.GET("/v1/betaTesters", "filter[email]=existing@example.com", "fields[betaTesters]=email,firstName,lastName,inviteType,state", "limit=200"),
						asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"t1","attributes":{"email":"existing@example.com"}}]}`))
					reg.Register(asctest.GET("/v1/betaTesters/t1/relationships/betaGroups", "limit=200"),
						asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"g1"}]}`))
				}
				args := []string{"testflight", "testers", command, "--app", "a1", "--create-missing-groups"}
				if command == "add" {
					args = append(args, "--email", "existing@example.com", "--group", tc.selector)
				} else {
					path := filepath.Join(t.TempDir(), "testers.csv")
					if err := os.WriteFile(path, []byte("email,groups\nexisting@example.com,"+tc.selector+"\n"), 0o600); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--input", path, "--confirm")
				}
				stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), args...)
				if tc.ambiguous {
					if err == nil || stdout != "" || !strings.Contains(err.Error(), "2 groups match") {
						t.Fatalf("stdout=%q stderr=%q err=%v", stdout, stderr, err)
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
