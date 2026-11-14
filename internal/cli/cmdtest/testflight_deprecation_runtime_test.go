package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestFlightGroupsDeleteUsesCanonicalSuccessMessage(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.DELETE("/v1/betaGroups/group-1"),
		asctest.JSON(http.StatusNoContent, ""))

	_, stderr, err := runCommand(t, []string{"testflight", "groups", "delete", "--id", "group-1", "--confirm"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stderr, "Successfully deleted group group-1") {
		t.Fatalf("expected canonical delete message, got %q", stderr)
	}
	if strings.Contains(stderr, "beta group") {
		t.Fatalf("expected canonical message without beta terminology, got %q", stderr)
	}
}
