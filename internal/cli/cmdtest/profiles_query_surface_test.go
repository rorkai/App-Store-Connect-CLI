package cmdtest

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestProfilesListSendsQuerySurface(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/profiles" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		values := req.URL.Query()
		checks := map[string]string{
			"filter[name]":         "Development,Store",
			"filter[id]":           "profile-1,profile-2",
			"filter[profileType]":  "IOS_APP_DEVELOPMENT,IOS_APP_STORE",
			"filter[profileState]": "ACTIVE,INVALID",
			"sort":                 "name,-id",
			"fields[profiles]":     "name,expirationDate",
			"fields[bundleIds]":    "identifier",
			"fields[devices]":      "name,udid",
			"fields[certificates]": "displayName,serialNumber",
			"include":              "bundleId,devices,certificates",
			"limit[devices]":       "7",
			"limit[certificates]":  "9",
			"limit":                "5",
		}
		for key, want := range checks {
			if got := values.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	t.Cleanup(server.Close)
	stdout, stderr, runErr := runRootWithDeps(t, stubServerClient(t, server),
		"profiles", "list",
		"--name", "Development, Store",
		"--id", "profile-1, profile-2",
		"--profile-type", "IOS_APP_DEVELOPMENT, IOS_APP_STORE",
		"--profile-state", "ACTIVE, INVALID",
		"--sort", "name,-id",
		"--fields", "name,expirationDate",
		"--bundle-id-fields", "identifier",
		"--device-fields", "name,udid",
		"--certificate-fields", "displayName,serialNumber",
		"--include", "bundleId,devices,certificates",
		"--limit-devices", "7",
		"--limit-certificates", "9",
		"--limit", "5",
		"--output", "json",
	)
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stdout != `{"data":[],"links":{}}`+"\n" {
		t.Fatalf("stdout = %q, want empty profiles response", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestProfilesListQuerySurfacePaginatesFromServerNextURL(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/profiles?cursor=next&limit=200"
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/profiles" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if requestCount == 1 {
			if got := req.URL.Query().Get("filter[name]"); got != "Development" {
				t.Errorf("first filter[name] = %q, want Development", got)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if got := req.URL.Query().Get("include"); got != "devices" {
				t.Errorf("first include = %q, want devices", got)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if got := req.URL.Query().Get("fields[devices]"); got != "name" {
				t.Errorf("first fields[devices] = %q, want name", got)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if got := req.URL.Query().Get("limit[devices]"); got != "3" {
				t.Errorf("first limit[devices] = %q, want 3", got)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":[{"type":"profiles","id":"profile-1"}],"links":{"next":"`+nextURL+`"}}`)
			return
		}
		if requestCount != 2 {
			t.Errorf("unexpected request count: %d", requestCount)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		const wantContinuation = "/v1/profiles?cursor=next&limit=200"
		if got := req.URL.String(); got != wantContinuation {
			t.Errorf("continuation URL = %q, want %q", got, wantContinuation)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"profiles","id":"profile-2"}],"links":{"next":""}}`)
	}))
	t.Cleanup(server.Close)
	stdout, stderr, runErr := runRootWithDeps(t, stubServerClient(t, server),
		"profiles", "list", "--name", "Development", "--include", "devices", "--device-fields", "name", "--limit-devices", "3", "--paginate", "--output", "json",
	)
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, `"id":"profile-1"`) || !strings.Contains(stdout, `"id":"profile-2"`) {
		t.Fatalf("stdout = %q, want both profile pages", stdout)
	}
}

func TestProfilesListSparseFieldsPreserveAttributePresence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/profiles" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if got := req.URL.Query().Get("fields[profiles]"); got != "expirationDate" {
			t.Errorf("fields[profiles] = %q, want expirationDate", got)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"profiles","id":"profile-1","attributes":{"expirationDate":"2026-08-24T00:00:00Z"}}]}`)
	}))
	t.Cleanup(server.Close)
	stdout, stderr, runErr := runRootWithDeps(t, stubServerClient(t, server), "profiles", "list", "--fields", "expirationDate", "--output", "json")
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}

	const want = `{"data":[{"type":"profiles","id":"profile-1","attributes":{"expirationDate":"2026-08-24T00:00:00Z"}}],"links":{}}` + "\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want sparse profile JSON %q", stdout, want)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestProfilesListRelationshipOnlySparseResponsePreservesAttributeAbsence(t *testing.T) {
	for _, relationship := range []string{"devices", "bundleId", "certificates"} {
		t.Run(relationship, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/profiles" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if got := req.URL.Query().Get("fields[profiles]"); got != relationship {
					t.Errorf("fields[profiles] = %q, want %q", got, relationship)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, fmt.Sprintf(`{"data":[{"type":"profiles","id":"profile-1","relationships":{"%s":{"data":[]}}}],"included":[]}`, relationship))
			}))
			t.Cleanup(server.Close)
			stdout, stderr, runErr := runRootWithDeps(t, stubServerClient(t, server), "profiles", "list", "--fields", relationship, "--output", "json")
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}

			want := fmt.Sprintf(`{"data":[{"type":"profiles","id":"profile-1","relationships":{"%s":{"data":[]}}}],"links":{},"included":[]}`+"\n", relationship)
			if stdout != want {
				t.Fatalf("stdout = %q, want relationship-only sparse profile JSON %q", stdout, want)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
		})
	}
}

func TestProfilesListSparseFieldsPreserveIncludedDevices(t *testing.T) {
	const includedDevices = `[{"type":"devices","id":"device-1","attributes":{"name":"iPhone","udid":"00000000-0000-0000-0000-000000000001"}}]`
	tests := []struct {
		name string
		body string
	}{
		{
			name: "relationships and included",
			body: `{"data":[{"type":"profiles","id":"profile-1","attributes":{"name":"Development"},"relationships":{"devices":{"data":[{"type":"devices","id":"device-1"}]}}}],"links":{},"included":` + includedDevices + `}`,
		},
		{
			name: "included without relationships",
			body: `{"data":[{"type":"profiles","id":"profile-1","attributes":{"name":"Development"}}],"links":{},"included":` + includedDevices + `}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var query url.Values
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/profiles" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				query = req.URL.Query()
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, test.body)
			}))
			t.Cleanup(server.Close)
			stdout, stderr, runErr := runRootWithDeps(t, stubServerClient(t, server),
				"profiles", "list",
				"--fields", "name",
				"--include", "devices",
				"--device-fields", "name,udid",
				"--output", "json",
			)
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}

			wantQuery := map[string]string{
				"fields[profiles]":     "name",
				"fields[devices]":      "name,udid",
				"include":              "devices",
				"filter[profileState]": "ACTIVE,INVALID",
			}
			for key, want := range wantQuery {
				if got := query.Get(key); got != want {
					t.Errorf("%s = %q, want %q", key, got, want)
				}
			}
			if len(query) != len(wantQuery) {
				t.Errorf("query = %s, want exactly %d parameters", query.Encode(), len(wantQuery))
			}
			if stdout != test.body+"\n" {
				t.Fatalf("stdout = %q, want byte-identical envelope %q", stdout, test.body+"\n")
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
		})
	}
}

func TestProfilesListRejectsInvalidQueryValuesBeforeAuth(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    string
		concise bool
	}{
		{name: "sort", args: []string{"profiles", "list", "--sort", "createdDate"}, want: "--sort must be one of:"},
		{name: "empty name", args: []string{"profiles", "list", "--name", ","}, want: "--name must not be empty"},
		{name: "empty id", args: []string{"profiles", "list", "--id", " \t"}, want: "--id must not be empty"},
		{name: "empty profile type", args: []string{"profiles", "list", "--profile-type", ","}, want: "--profile-type must not be empty"},
		{name: "empty profile state", args: []string{"profiles", "list", "--profile-state", ""}, want: "--profile-state must not be empty"},
		{name: "empty sort", args: []string{"profiles", "list", "--sort", ","}, want: "--sort must not be empty"},
		{name: "empty profile fields", args: []string{"profiles", "list", "--fields", ","}, want: "--fields must not be empty"},
		{name: "empty bundle fields", args: []string{"profiles", "list", "--bundle-id-fields", " \t"}, want: "--bundle-id-fields must not be empty"},
		{name: "empty device fields", args: []string{"profiles", "list", "--device-fields", ""}, want: "--device-fields must not be empty"},
		{name: "empty certificate fields", args: []string{"profiles", "list", "--certificate-fields", ","}, want: "--certificate-fields must not be empty"},
		{name: "empty include", args: []string{"profiles", "list", "--include", ","}, want: "--include must not be empty"},
		{name: "profile fields", args: []string{"profiles", "list", "--fields", "notAField"}, want: "--fields must be one of:"},
		{name: "bundle fields", args: []string{"profiles", "list", "--bundle-id-fields", "uuid"}, want: "--bundle-id-fields must be one of:"},
		{name: "device fields", args: []string{"profiles", "list", "--device-fields", "uuid"}, want: "--device-fields must be one of:"},
		{name: "certificate fields", args: []string{"profiles", "list", "--certificate-fields", "uuid"}, want: "--certificate-fields must be one of:"},
		{name: "include", args: []string{"profiles", "list", "--include", "app"}, want: "--include must be one of:"},
		{name: "device limit", args: []string{"profiles", "list", "--limit-devices", "51"}, want: "--limit-devices must be between 1 and 50"},
		{name: "explicit zero device limit", args: []string{"profiles", "list", "--limit-devices", "0"}, want: "--limit-devices must be between 1 and 50"},
		{name: "certificate limit", args: []string{"profiles", "list", "--limit-certificates", "51"}, want: "--limit-certificates must be between 1 and 50"},
		{name: "explicit zero certificate limit", args: []string{"profiles", "list", "--limit-certificates", "0"}, want: "--limit-certificates must be between 1 and 50"},
		{name: "bundle fields dependency", args: []string{"profiles", "list", "--bundle-id-fields", "identifier"}, want: "--bundle-id-fields requires --include bundleId", concise: true},
		{name: "device fields dependency", args: []string{"profiles", "list", "--device-fields", "name"}, want: "--device-fields requires --include devices", concise: true},
		{name: "certificate fields dependency", args: []string{"profiles", "list", "--certificate-fields", "name"}, want: "--certificate-fields requires --include certificates", concise: true},
		{name: "device limit dependency", args: []string{"profiles", "list", "--limit-devices", "7"}, want: "--limit-devices requires --include devices", concise: true},
		{name: "certificate limit dependency", args: []string{"profiles", "list", "--limit-certificates", "7"}, want: "--limit-certificates requires --include certificates", concise: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			client := func() (*asc.Client, error) {
				called = true
				return nil, errors.New("client factory must not run during validation")
			}

			stdout, stderr, err := runRootWithDeps(t, client, test.args...)
			if err == nil {
				t.Fatal("run error = nil, want validation error")
			}
			if test.concise {
				if errors.Is(err, flag.ErrHelp) || !shared.IsReportedUsageError(err) {
					t.Fatalf("expected concise reported usage error without flag.ErrHelp, got %v", err)
				}
				if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitUsage)
				}
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if test.concise {
				want := "Error: " + test.want + "\n"
				if stderr != want {
					t.Fatalf("stderr = %q, want exact concise diagnostic %q", stderr, want)
				}
			} else if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
			if called {
				t.Fatal("client factory ran before validation")
			}
		})
	}
}

func TestProfilesListRejectsNextQueryConflictsBeforeAuth(t *testing.T) {
	const next = "https://api.appstoreconnect.apple.com/v1/profiles?cursor=abc"
	tests := []struct {
		name  string
		flag  string
		value string
	}{
		{name: "name", flag: "--name", value: "Development"},
		{name: "id", flag: "--id", value: "profile-1"},
		{name: "profile type", flag: "--profile-type", value: "IOS_APP_STORE"},
		{name: "profile state", flag: "--profile-state", value: "ACTIVE"},
		{name: "sort", flag: "--sort", value: "name"},
		{name: "fields", flag: "--fields", value: "name"},
		{name: "bundle fields", flag: "--bundle-id-fields", value: "identifier"},
		{name: "device fields", flag: "--device-fields", value: "name"},
		{name: "certificate fields", flag: "--certificate-fields", value: "name"},
		{name: "include", flag: "--include", value: "devices"},
		{name: "device limit", flag: "--limit-devices", value: "7"},
		{name: "certificate limit", flag: "--limit-certificates", value: "7"},
		{name: "limit", flag: "--limit", value: "7"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			client := func() (*asc.Client, error) {
				called = true
				return nil, errors.New("client factory must not run during validation")
			}

			stdout, stderr, err := runRootWithDeps(t, client, "profiles", "list", "--next", next, test.flag, test.value)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("run error = %v, want flag.ErrHelp", err)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			want := "profiles list: --next cannot be combined with " + test.flag
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if called {
				t.Fatal("client factory ran before --next conflict validation")
			}
		})
	}
}

func installProfilesQueryTestClient(t *testing.T, server *httptest.Server) {
	t.Helper()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create profiles query test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))
}
