package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestSandboxViewByEmailStopsOnRepeatedNextURL(t *testing.T) {
	setupAuth(t)
	const repeatedNext = "https://api.appstoreconnect.apple.com/v2/sandboxTesters?cursor=repeat"
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/sandboxTesters", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":"`+repeatedNext+`"}}`))
	reg.Register(asctest.GET("/v2/sandboxTesters", "cursor=repeat"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":"`+repeatedNext+`"}}`))

	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"sandbox", "view", "--email", "tester@example.com"}, "1.2.3"); code == 0 {
			t.Fatal("expected repeated pagination failure")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if requests := len(reg.Requests()); requests != 2 {
		t.Fatalf("expected repeated next URL to stop before third request, got %d", requests)
	}
	if !strings.Contains(stderr, "detected repeated pagination URL") {
		t.Fatalf("expected repeated pagination error, got %q", stderr)
	}
}

func TestSandboxUsageErrors(t *testing.T) {
	runUsageExitCases(t, []usageExitCase{
		{
			name:    "ViewMissingIDAndEmail",
			args:    []string{"sandbox", "view"},
			wantErr: "--id or --email is required",
		},
		{
			name:    "UpdateMissingIDAndEmail",
			args:    []string{"sandbox", "update", "--territory", "USA"},
			wantErr: "--id or --email is required",
		},
		{
			name:    "UpdateMissingUpdateFields",
			args:    []string{"sandbox", "update", "--id", "tester-1"},
			wantErr: "--territory, --interrupt-purchases, or --subscription-renewal-rate is required",
		},
		{
			name:    "ClearHistoryMissingConfirm",
			args:    []string{"sandbox", "clear-history", "--id", "tester-1"},
			wantErr: "--confirm is required",
		},
		{
			name:    "ClearHistoryMissingIDAndEmail",
			args:    []string{"sandbox", "clear-history", "--confirm"},
			wantErr: "--id or --email is required",
		},
	})
}
