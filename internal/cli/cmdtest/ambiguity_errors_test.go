package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=Ambiguous App", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps", "filter[name]=Ambiguous App", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[
			{"type":"apps","id":"app-2","attributes":{"name":"Ambiguous App","bundleId":"com.example.two"}},
			{"type":"apps","id":"app-1","attributes":{"name":"Ambiguous App","bundleId":"com.example.one"}}
		]}`))

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
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[
			{"type":"betaGroups","id":"group-internal","attributes":{"name":"Beta","isInternalGroup":true}},
			{"type":"betaGroups","id":"group-external","attributes":{"name":"Beta","isInternalGroup":false}}
		],"links":{}}`))

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
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v2/sandboxTesters", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[
			{"type":"sandboxTesters","id":"tester-1","attributes":{"email":"dup@example.com","firstName":"Jane","lastName":"Doe"}},
			{"type":"sandboxTesters","id":"tester-2","attributes":{"email":"dup@example.com","firstName":"John","lastName":"Doe"}}
		],"links":{}}`))

	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "sandbox", "view", "--email", "dup@example.com")
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertStderrLines(
		t, stderr,
		`2 sandbox testers match email "dup@example.com"; pass --id with one of:`,
		"\n  tester-1\n  tester-2\n",
	)
	for _, privateValue := range []string{"Jane Doe", "John Doe"} {
		if strings.Contains(stderr, privateValue) {
			t.Fatalf("stderr must not expose tester name %q: %s", privateValue, stderr)
		}
	}
}

func TestBuildsTestNotesUpdateByBuildLocaleAmbiguousListsCandidates(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.GET("/v1/betaBuildLocalizations", "filter[build]=build-1", "filter[locale]=en-US", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[
			{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US"}},
			{"type":"betaBuildLocalizations","id":"loc-2","attributes":{"locale":"en-US"}}
		]}`))

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"builds", "test-notes", "update",
			"--build-id", "build-1",
			"--locale", "en-US",
			"--whats-new", "Updated notes",
		}, "1.2.3")
		if code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertStderrLines(
		t, stderr,
		`2 build localizations match build "build-1" and locale "en-US"; pass --localization-id with one of:`,
		"\n  loc-1  en-US\n  loc-2  en-US\n",
	)
	if requests := len(reg.Requests()); requests != 2 {
		t.Fatalf("requests = %d, want 2 (no PATCH)", requests)
	}
}
