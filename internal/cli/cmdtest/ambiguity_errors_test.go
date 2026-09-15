package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// These tests pin the ambiguity error contract end to end: when a selector
// matches several resources the command exits with its usual code and stderr
// lists every candidate with its ID and the flag that accepts one of them.

func assertStderrLines(t *testing.T, stderr string, wantLines ...string) {
	t.Helper()
	for _, want := range wantLines {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr)
		}
	}
}

func TestAmbiguousAppNameListsCandidatesAndAppFlag(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if req.URL.Query().Get("filter[bundleId]") != "" {
			return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[
			{"type":"apps","id":"app-2","attributes":{"name":"Ambiguous App","bundleId":"com.example.two"}},
			{"type":"apps","id":"app-1","attributes":{"name":"Ambiguous App","bundleId":"com.example.one"}}
		]}`), nil
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"builds", "list", "--app", "Ambiguous App"}, "1.2.3"); code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertStderrLines(
		t, stderr,
		`2 apps match "Ambiguous App"; pass --app with one of:`,
		"\n  app-1  Ambiguous App  com.example.one\n  app-2  Ambiguous App  com.example.two\n",
	)
}

func TestAmbiguousBetaGroupNameListsCandidatesAndGroupFlag(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[
			{"type":"betaGroups","id":"group-internal","attributes":{"name":"Beta","isInternalGroup":true}},
			{"type":"betaGroups","id":"group-external","attributes":{"name":"Beta","isInternalGroup":false}}
		],"links":{}}`), nil
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"testflight", "testers", "add", "--app", "app-1", "--email", "tester@example.com", "--group", "Beta"}, "1.2.3"); code == cmd.ExitSuccess {
			t.Fatal("expected ambiguity failure")
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertStderrLines(
		t, stderr,
		`2 groups match "Beta"; pass --group with one of:`,
		"\n  group-internal  Beta  internal\n  group-external  Beta  external\n",
	)
}

func TestAmbiguousSandboxTesterEmailListsCandidatesAndIDFlag(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v2/sandboxTesters" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[
			{"type":"sandboxTesters","id":"tester-1","attributes":{"email":"dup@example.com","firstName":"Jane","lastName":"Doe"}},
			{"type":"sandboxTesters","id":"tester-2","attributes":{"email":"dup@example.com","firstName":"John","lastName":"Doe"}}
		],"links":{}}`), nil
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"sandbox", "view", "--email", "dup@example.com"}, "1.2.3"); code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertStderrLines(
		t, stderr,
		`2 sandbox testers match email "dup@example.com"; pass --id with one of:`,
		"\n  tester-1  dup@example.com  Jane Doe\n  tester-2  dup@example.com  John Doe\n",
	)
}
