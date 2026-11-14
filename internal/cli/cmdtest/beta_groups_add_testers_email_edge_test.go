package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBetaGroupsAddTestersMergesTesterAndEmailWithoutDuplicates(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaGroups/group-1/app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`))
	reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "filter[email]=tester@example.com", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-1"}]}`))
	reg.Register(asctest.POST("/v1/betaGroups/group-1/relationships/betaTesters").JSONBody(`{"data":[{"type":"betaTesters","id":"tester-1"}]}`),
		asctest.JSON(http.StatusNoContent, ""))

	stdout, stderr, err := runCommand(t, []string{
		"testflight", "groups", "add-testers",
		"--group", "group-1",
		"--tester", "tester-1",
		"--email", "tester@example.com",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stdout, `"action":"added"`) || !strings.Contains(stdout, `"testerIds":["tester-1"]`) {
		t.Fatalf("expected an added receipt for the deduped tester, got %q", stdout)
	}
	if !strings.Contains(stderr, "Successfully added 1 tester(s) to group group-1") {
		t.Fatalf("expected deduped success message, got %q", stderr)
	}
}

func TestBetaGroupsAddTestersEmailPartialLookupFailureDoesNotMutate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaGroups/group-1/app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`))
	reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "filter[email]=valid@example.com", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-valid"}]}`))
	reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "filter[email]=missing@example.com", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))

	stdout, _, runErr := runCommand(t, []string{
		"testflight", "groups", "add-testers",
		"--group", "group-1",
		"--email", "valid@example.com,missing@example.com",
	})

	if runErr == nil {
		t.Fatal("expected lookup failure, got nil")
	}
	if !strings.Contains(runErr.Error(), `tester email "missing@example.com" not found for app "app-1"`) {
		t.Fatalf("expected missing email error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on failure, got %q", stdout)
	}
}

func TestBetaGroupsAddTestersAmbiguousEmailListsIDsWithoutPersonalData(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaGroups/group-1/app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`))
	reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "filter[email]=dup@example.com", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[
			{"type":"betaTesters","id":"tester-1","attributes":{"email":"dup@example.com","firstName":"Private","lastName":"One"}},
			{"type":"betaTesters","id":"tester-2","attributes":{"email":"dup@example.com","firstName":"Private","lastName":"Two"}}
		]}`))

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"testflight", "groups", "add-testers",
			"--group", "group-1",
			"--email", "dup@example.com",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{`Error: groups add-testers: 2 testers match email "dup@example.com"; pass --tester with one of:`, "tester-1", "tester-2"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q: %s", want, stderr)
		}
	}
	for _, privateValue := range []string{"Private One", "Private Two"} {
		if strings.Contains(stderr, privateValue) {
			t.Fatalf("stderr must not expose tester name %q: %s", privateValue, stderr)
		}
	}
	if strings.Count(stderr, "dup@example.com") != 1 {
		t.Fatalf("selected email should appear only in the ambiguity header: %s", stderr)
	}
	if requests := len(reg.Requests()); requests != 2 {
		t.Fatalf("requests = %d, want 2 (no POST)", requests)
	}
}

func TestBetaGroupsAddTestersSingleEmailSampleFailsClosed(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaGroups/group-1/app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`))
	reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "filter[email]=sample@example.com", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-1","attributes":{"email":"sample@example.com"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/betaTesters?cursor=next"}}`))

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"testflight", "groups", "add-testers",
			"--group", "group-1",
			"--email", "sample@example.com",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{"sample matches", "tester-1"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q: %s", want, stderr)
		}
	}
	if requests := len(reg.Requests()); requests != 2 {
		t.Fatalf("requests = %d, want 2 (no POST)", requests)
	}
}
