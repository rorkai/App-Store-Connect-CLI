package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestSandboxUpdateNormalizesTerritory(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v2/sandboxTesters/tester-1").JSONBody(`{"data":{"type":"sandboxTesters","id":"tester-1","attributes":{"territory":"USA"}}}`), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"sandboxTesters",
			"id":"tester-1",
			"attributes":{"territory":"USA"}
		}
	}`))

	stdout, stderr, err := runCommand(t, []string{"sandbox", "update", "--id", "tester-1", "--territory", "US"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"tester-1"`) {
		t.Fatalf("expected sandbox tester output, got %q", stdout)
	}
}
