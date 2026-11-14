package cmdtest

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestCategoriesSetFailsWhenAppInfoIsAmbiguous(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"), asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_DISTRIBUTION"}},
				{"type":"appInfos","id":"info-rejected","attributes":{"state":"REJECTED"}}
			]
		}`))

	stdout, stderr, runErr := runCommand(t, []string{"categories", "set", "--app", "app-1", "--primary", "GAMES", "--output", "json"})
	if runErr == nil {
		t.Fatal("expected run error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{
		`2 app infos match app "app-1"; pass --app-info with one of:`,
		`asc apps info list --app "app-1"`,
		"READY_FOR_DISTRIBUTION",
		"REJECTED",
	} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, runErr)
		}
	}
}

func TestCategoriesAndAppSetupInfoSetResolveConfiguredAppID(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		withConfig bool
		wantStderr string
	}{
		{name: "categories set uses config app_id", args: []string{"categories", "set"}, withConfig: true, wantStderr: "Error: categories set: --primary is required"},
		{name: "app-setup categories set uses config app_id", args: []string{"app-setup", "categories", "set"}, withConfig: true, wantStderr: "Error: app-setup categories set: --primary is required"},
		{name: "app-setup info set uses config app_id", args: []string{"app-setup", "info", "set"}, withConfig: true, wantStderr: "Error: provide at least one update flag"},
		{name: "categories set without app is a usage error", args: []string{"categories", "set", "--primary", "GAMES"}, wantStderr: "Error: categories set: --app is required (or set ASC_APP_ID)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			if err := os.Unsetenv("ASC_APP_ID"); err != nil {
				t.Fatalf("unset ASC_APP_ID: %v", err)
			}
			configPath := filepath.Join(t.TempDir(), "config.json")
			if test.withConfig {
				if err := os.WriteFile(configPath, []byte(`{"app_id":"123456789"}`), 0o600); err != nil {
					t.Fatalf("write config: %v", err)
				}
			}
			t.Setenv("ASC_CONFIG_PATH", configPath)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); !isUsageClassError(err) {
					t.Fatalf("error = %v, want a usage error", err)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.HasPrefix(stderr, test.wantStderr+"\n") {
				t.Fatalf("stderr = %q, want prefix %q", stderr, test.wantStderr)
			}
		})
	}
}
