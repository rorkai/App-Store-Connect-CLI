package cmdtest

import (
	"errors"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSubscriptionVersionLocalizationDeleteRequiresConfirmBeforeAuthOrHTTP(t *testing.T) {
	clearSubscriptionVersionAuth(t)

	clientFactoryCalled := false
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientFactoryCalled = true
		return nil, errors.New("client factory must not run before confirmation")
	})
	t.Cleanup(restore)

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"subscriptions", "versions", "localizations", "delete",
			"--id", "loc-1",
		}, "1.2.3")
		if code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.HasPrefix(stderr, "Error: --confirm is required\n") {
		t.Fatalf("stderr = %q, want missing-confirm diagnostic first", stderr)
	}
	if strings.Count(stderr, "Error: --confirm is required\n") != 1 {
		t.Fatalf("stderr = %q, want one missing-confirm diagnostic", stderr)
	}
	const usage = "USAGE\n  asc subscriptions versions localizations delete --id \"LOCALIZATION_ID\" --confirm\n"
	if strings.Count(stderr, "USAGE\n") != 1 {
		t.Fatalf("stderr = %q, want one usage block", stderr)
	}
	if !strings.Contains(stderr, usage) {
		t.Fatalf("stderr = %q, want localization-delete usage", stderr)
	}
	if clientFactoryCalled {
		t.Fatal("authenticated client factory ran before confirmation")
	}
}
