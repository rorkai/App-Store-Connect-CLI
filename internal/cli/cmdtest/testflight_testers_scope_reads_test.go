package cmdtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTesterReconciliationReadsStayInScope(t *testing.T) {
	for _, command := range []string{"add", "import"} {
		for _, tc := range []struct {
			name, path, next string
			allowed          bool
		}{
			{name: "tester redirect", path: "/v1/betaTesters"},
			{name: "group discovery redirect", path: "/v1/apps/app-1/betaGroups"},
			{name: "group membership redirect", path: "/v1/betaTesters/t1/relationships/betaGroups"},
			{name: "build ownership redirect", path: "/v1/builds/b1/relationships/app"},
			{name: "build membership redirect", path: "/v1/builds/b1/relationships/individualTesters"},
			{name: "group discovery continuation", path: "/v1/apps/app-1/betaGroups", next: "/fixture/scope-sentinel"},
			{name: "group membership continuation", path: "/v1/betaTesters/t1/relationships/betaGroups", next: "/fixture/scope-sentinel"},
			{name: "build membership continuation", path: "/v1/builds/b1/relationships/individualTesters", next: "/fixture/scope-sentinel"},
			{name: "changed tester filter", path: "/v1/betaTesters", next: "/v1/betaTesters?filter%5Bapps%5D=other"},
			{name: "unscoped tester page without cursor", path: "/v1/betaTesters", next: "/v1/betaTesters?limit=200"},
			{name: "tester cursor adds unrelated expansion", path: "/v1/betaTesters", next: "/v1/betaTesters?cursor=two&include=apps"},
			{name: "tester cursor reorders sparse fields", path: "/v1/betaTesters", next: "/v1/betaTesters?cursor=two&fields%5BbetaTesters%5D=state,inviteType,lastName,firstName,email", allowed: true},
			{name: "opaque tester cursor", path: "/v1/betaTesters", next: "/v1/betaTesters?cursor=two", allowed: true},
		} {
			if command == "import" && strings.HasPrefix(tc.path, "/v1/builds/") {
				continue
			}
			t.Run(command+"/"+tc.name, func(t *testing.T) {
				t.Setenv("ASC_APP_ID", "")
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
				writes, escaped := 0, 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.Method != http.MethodGet {
						writes++
						w.WriteHeader(http.StatusNoContent)
						return
					}
					if r.URL.Path == tc.path && r.URL.Query().Get("cursor") == "" && tc.next == "" {
						http.Redirect(w, r, "/fixture/scope-sentinel", http.StatusTemporaryRedirect)
						return
					}
					if tc.name == "unscoped tester page without cursor" && r.URL.Path == "/v1/betaTesters" && r.URL.RawQuery == "limit=200" {
						escaped++
						_, _ = w.Write([]byte(`{"data":[{"type":"betaTesters","id":"foreign","attributes":{"email":"tester@example.com"}}]}`))
						return
					}
					if tc.allowed && r.URL.Path == "/v1/betaTesters" && r.URL.Query().Get("cursor") == "two" {
						q := r.URL.Query()
						fields := q.Get("fields[betaTesters]")
						validFields := fields == "email,firstName,lastName,inviteType,state" || fields == "state,inviteType,lastName,firstName,email"
						if q.Get("filter[apps]") != "app-1" || q.Get("filter[email]") != "tester@example.com" || !validFields {
							t.Errorf("tester page omitted request scope: %v", q)
						}
					}
					var body string
					switch r.URL.Path {
					case "/v1/apps/app-1/betaGroups":
						body = `{"data":[{"type":"betaGroups","id":"g1","attributes":{"name":"Beta"}}]}`
					case "/v1/betaTesters", "/fixture/scope-sentinel":
						if r.URL.Path == "/fixture/scope-sentinel" {
							escaped++
						}
						body = `{"data":[{"type":"betaTesters","id":"t1","attributes":{"email":"tester@example.com"}}]}`
					case "/v1/betaTesters/foreign/relationships/betaGroups":
						body = `{"data":[]}`
					case "/v1/betaTesters/t1/relationships/betaGroups":
						body = `{"data":[{"type":"betaGroups","id":"g1"}]}`
					case "/v1/builds/b1/relationships/app":
						body = `{"data":{"type":"apps","id":"app-1"}}`
					case "/v1/builds/b1/relationships/individualTesters":
						body = `{"data":[{"type":"betaTesters","id":"t1"}]}`
					default:
						t.Errorf("unexpected request %s %s", r.Method, r.URL)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if r.URL.Path == tc.path && r.URL.Query().Get("cursor") == "" && tc.next != "" {
						var document map[string]any
						if err := json.Unmarshal([]byte(body), &document); err != nil {
							t.Error(err)
							return
						}
						if tc.name == "unscoped tester page without cursor" {
							document["data"] = []any{}
						}
						document["links"] = map[string]string{"next": tc.next}
						if err := json.NewEncoder(w).Encode(document); err != nil {
							t.Error(err)
						}
						return
					}
					_, _ = w.Write([]byte(body))
				}))
				defer server.Close()
				args := []string{"testflight", "testers", command, "--app", "app-1"}
				if command == "add" {
					args = append(args, "--email", "tester@example.com", "--group", "Beta", "--build-id", "b1")
				} else {
					path := filepath.Join(t.TempDir(), "testers.csv")
					if err := os.WriteFile(path, []byte("email,groups\ntester@example.com,Beta\n"), 0o600); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--input", path, "--confirm")
				}
				stdout, stderr, err := runRootWithDeps(t, stubServerClient(t, server), args...)
				if (err == nil) != tc.allowed || escaped != 0 || writes != 0 {
					t.Fatalf("scope result: allowed=%t escaped=%d writes=%d stdout=%s stderr=%s err=%v", tc.allowed, escaped, writes, stdout, stderr, err)
				}
			})
		}
	}
}
