package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestSandboxListNormalizesTerritoryFilter(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/sandboxTesters"), asctest.JSON(http.StatusOK, `{
		"data":[
			{"type":"sandboxTesters","id":"tester-usa","attributes":{"email":"usa@example.com","territory":"USA"}},
			{"type":"sandboxTesters","id":"tester-fra","attributes":{"email":"fra@example.com","territory":"FRA"}}
		]
	}`))

	stdout, stderr, err := runCommand(t, []string{"sandbox", "list", "--territory", "United States"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	assertEmptyStderr(t, stderr)
	if !strings.Contains(stdout, `"id":"tester-usa"`) {
		t.Fatalf("expected USA tester in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"id":"tester-fra"`) {
		t.Fatalf("did not expect FRA tester in output, got %q", stdout)
	}
}

func TestSandboxListAllowsEmptyTerritoryFilter(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/sandboxTesters"), asctest.JSON(http.StatusOK, `{
		"data":[
			{"type":"sandboxTesters","id":"tester-usa","attributes":{"territory":"USA"}},
			{"type":"sandboxTesters","id":"tester-fra","attributes":{"territory":"FRA"}}
		]
	}`))

	stdout, stderr, err := runCommand(t, []string{"sandbox", "list", "--territory="})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	assertEmptyStderr(t, stderr)
	if !strings.Contains(stdout, `"id":"tester-usa"`) || !strings.Contains(stdout, `"id":"tester-fra"`) {
		t.Fatalf("expected unfiltered tester output, got %q", stdout)
	}
}

func TestSandboxListKeepsFiltersAcrossPagination(t *testing.T) {
	tests := []struct {
		name        string
		filterArgs  []string
		initialNext string
		firstData   string
		secondData  string
		wantID      string
		droppedID   string
	}{
		{
			name:       "territory",
			filterArgs: []string{"--territory", "United States"},
			firstData:  `{"type":"sandboxTesters","id":"tester-usa","attributes":{"territory":"USA"}}`,
			secondData: `{"type":"sandboxTesters","id":"tester-fra","attributes":{"territory":"FRA"}}`,
			wantID:     "tester-usa",
			droppedID:  "tester-fra",
		},
		{
			name:       "email",
			filterArgs: []string{"--email", "usa@example.com"},
			firstData:  `{"type":"sandboxTesters","id":"tester-usa","attributes":{"email":"usa@example.com"}}`,
			secondData: `{"type":"sandboxTesters","id":"tester-other","attributes":{"email":"other@example.com"}}`,
			wantID:     "tester-usa",
			droppedID:  "tester-other",
		},
		{
			name:        "territory from next URL",
			filterArgs:  []string{"--territory", "USA"},
			initialNext: "https://api.appstoreconnect.apple.com/v2/sandboxTesters?cursor=page-1",
			firstData:   `{"type":"sandboxTesters","id":"tester-usa","attributes":{"territory":"USA"}}`,
			secondData:  `{"type":"sandboxTesters","id":"tester-fra","attributes":{"territory":"FRA"}}`,
			wantID:      "tester-usa",
			droppedID:   "tester-fra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, requests := runSandboxListFilterPagination(t, tt.filterArgs, tt.initialNext, tt.firstData, tt.secondData)

			assertEmptyStderr(t, stderr)
			if len(requests) != 2 {
				t.Fatalf("expected two paginated requests, got %d (%v)", len(requests), requests)
			}
			if tt.initialNext != "" && requests[0] != tt.initialNext {
				t.Fatalf("expected initial request to use --next URL %q, got %q", tt.initialNext, requests[0])
			}
			if !strings.Contains(stdout, `"id":"`+tt.wantID+`"`) {
				t.Fatalf("expected %s in output, got %q", tt.wantID, stdout)
			}
			if strings.Contains(stdout, `"id":"`+tt.droppedID+`"`) {
				t.Fatalf("did not expect %s in output, got %q", tt.droppedID, stdout)
			}
		})
	}
}

func runSandboxListFilterPagination(t *testing.T, filterArgs []string, initialNext, firstData, secondData string) (string, string, []string) {
	t.Helper()
	setupAuth(t)

	secondURL := "https://api.appstoreconnect.apple.com/v2/sandboxTesters?cursor=page-2"
	firstQuery := "limit=200"
	if initialNext != "" {
		firstQuery = "cursor=page-1"
	}
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/sandboxTesters", firstQuery), asctest.JSON(http.StatusOK, `{"data":[`+firstData+`],"links":{"next":"`+secondURL+`"}}`))
	reg.Register(asctest.GET("/v2/sandboxTesters", "cursor=page-2"), asctest.JSON(http.StatusOK, `{"data":[`+secondData+`]}`))

	args := append([]string{"sandbox", "list"}, filterArgs...)
	args = append(args, "--paginate", "--output", "json")
	if initialNext != "" {
		args = append(args, "--next", initialNext)
	}

	stdout, stderr, err := runCommand(t, args)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	requests := []string{}
	for _, req := range reg.Requests() {
		requests = append(requests, req.URL.String())
	}
	return stdout, stderr, requests
}
