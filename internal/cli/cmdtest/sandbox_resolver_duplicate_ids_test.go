package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestSandboxViewByEmailDeduplicatesRepeatedResourceIDsAcrossPages(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v2/sandboxTesters?cursor=next"
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v2/sandboxTesters", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[
		{"type":"sandboxTesters","id":"tester-1","attributes":{"email":"same@example.com","firstName":"Jane"}}
	],"links":{"next":"`+nextURL+`"}}`))
	reg.Register(asctest.GET("/v2/sandboxTesters", "cursor=next"), asctest.JSON(http.StatusOK, `{"data":[
		{"type":"sandboxTesters","id":"tester-1","attributes":{"email":"same@example.com","firstName":"Jane"}}
	],"links":{}}`))

	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "sandbox", "view", "--email", "same@example.com", "--output", "json")
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
	}
	if requests := len(reg.Requests()); requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if !strings.Contains(stdout, `"id":"tester-1"`) {
		t.Fatalf("expected selected tester in stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestSandboxViewByEmailKeepsDistinctIDsAmbiguousAcrossPages(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v2/sandboxTesters?cursor=next"
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v2/sandboxTesters", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[
		{"type":"sandboxTesters","id":" tester-2 ","attributes":{"email":"ambiguous@example.com"}}
	],"links":{"next":"`+nextURL+`"}}`))
	reg.Register(asctest.GET("/v2/sandboxTesters", "cursor=next"), asctest.JSON(http.StatusOK, `{"data":[
		{"type":"sandboxTesters","id":" tester-1 ","attributes":{"email":"ambiguous@example.com"}}
	],"links":{}}`))

	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "sandbox", "view", "--email", "ambiguous@example.com")
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{"tester-1", "tester-2"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing candidate %q: %s", want, stderr)
		}
	}
	if first, second := strings.Index(stderr, "tester-1"), strings.Index(stderr, "tester-2"); first < 0 || second < 0 || first > second {
		t.Fatalf("stderr candidates are not sorted by normalized ID: %s", stderr)
	}
}

func TestSandboxViewByEmailRejectsBlankResourceID(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v2/sandboxTesters", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[
		{"type":"sandboxTesters","id":"   ","attributes":{"email":"malformed@example.com"}}
	],"links":{}}`))

	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "sandbox", "view", "--email", "malformed@example.com")
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "empty ID") {
		t.Fatalf("stderr missing empty ID diagnostic: %s", stderr)
	}
}

func TestSandboxByEmailNormalizesID(t *testing.T) {
	tests := []struct {
		name          string
		email         string
		args          []string
		mutation      func(reg *asctest.Registry)
		wantStdout    string
		notWantStdout string
	}{
		{
			name:          "view",
			email:         "view@example.com",
			args:          []string{"sandbox", "view", "--email", "view@example.com", "--output", "json"},
			wantStdout:    `"id":"tester-42"`,
			notWantStdout: `"id":" tester-42 "`,
		},
		{
			name:  "update",
			email: "update@example.com",
			args:  []string{"sandbox", "update", "--email", "update@example.com", "--territory", "USA", "--output", "json"},
			mutation: func(reg *asctest.Registry) {
				reg.Register(asctest.PATCH("/v2/sandboxTesters/tester-42").JSONBody(`{"data":{"type":"sandboxTesters","id":"tester-42","attributes":{"territory":"USA"}}}`),
					asctest.JSON(http.StatusOK, `{"data":{"type":"sandboxTesters","id":"tester-42","attributes":{"email":"update@example.com","territory":"USA"}}}`))
			},
			wantStdout: `"id":"tester-42"`,
		},
		{
			name:  "clear-history",
			email: "clear@example.com",
			args:  []string{"sandbox", "clear-history", "--email", "clear@example.com", "--confirm", "--output", "json"},
			mutation: func(reg *asctest.Registry) {
				reg.Register(asctest.POST("/v2/sandboxTestersClearPurchaseHistoryRequest").JSONBody(`{"data":{"type":"sandboxTestersClearPurchaseHistoryRequest","relationships":{"sandboxTesters":{"data":[{"type":"sandboxTesters","id":"tester-42"}]}}}}`),
					asctest.JSON(http.StatusCreated, `{"data":{"type":"sandboxTestersClearPurchaseHistoryRequest","id":"request-1"}}`))
			},
			wantStdout: `"testerId":"tester-42"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v2/sandboxTesters", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"sandboxTesters","id":" tester-42 ","attributes":{"email":"`+test.email+`"}}],"links":{}}`))
			if test.mutation != nil {
				test.mutation(reg)
			}

			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), test.args...)
			if code != cmd.ExitSuccess {
				t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("expected stdout to contain %q, got %q", test.wantStdout, stdout)
			}
			if test.notWantStdout != "" && strings.Contains(stdout, test.notWantStdout) {
				t.Fatalf("expected stdout not to contain %q, got %q", test.notWantStdout, stdout)
			}
		})
	}
}
