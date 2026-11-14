package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAppsRemoveBetaTestersValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "apps remove-beta-testers missing app",
			args:    []string{"apps", "remove-beta-testers", "--tester", "TESTER_ID", "--confirm"},
			wantErr: "--app is required",
		},
		{
			name:    "apps remove-beta-testers missing tester",
			args:    []string{"apps", "remove-beta-testers", "--app", "APP_ID", "--confirm"},
			wantErr: "--tester is required",
		},
		{
			name:    "apps remove-beta-testers missing confirm",
			args:    []string{"apps", "remove-beta-testers", "--app", "APP_ID", "--tester", "TESTER_ID"},
			wantErr: "--confirm is required",
		},
	}

	runValidationTests(t, tests)
}

func TestAppsRemoveBetaTestersOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.DELETE("/v1/apps/app-1/relationships/betaTesters").JSONBody(`{"data":[{"type":"betaTesters","id":"tester-1"},{"type":"betaTesters","id":"tester-2"}]}`),
		asctest.JSON(http.StatusNoContent, ""))

	stdout, stderr, err := runCommand(t, []string{"apps", "remove-beta-testers", "--app", "app-1", "--tester", "tester-1,tester-2", "--confirm"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"appId":"app-1"`) {
		t.Fatalf("expected app id in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"testerIds"`) {
		t.Fatalf("expected tester ids in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"action":"removed"`) {
		t.Fatalf("expected action removed in output, got %q", stdout)
	}
}
