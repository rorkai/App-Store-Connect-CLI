package cmdtest

import (
	"context"
	"testing"

	webcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebReviewSandboxEntitlementsValidatesBeforeSession(t *testing.T) {
	setCmdtestHome(t)
	t.Cleanup(webcmd.SetResolveWebSession(func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("unexpected session resolution")
		return nil, "", nil
	}))
	setArgs := []string{"--version-id", "version-1", "--entitlement", "com.apple.security.network.server", "--justification", "Text", "--confirm"}
	for _, tc := range []struct {
		args    []string
		message string
	}{
		{[]string{"list"}, "--version-id is required"},
		{[]string{"set", "--entitlement", "com.apple.security.network.server", "--justification", "Text", "--confirm"}, "--version-id is required"},
		{[]string{"set", "--version-id", "version-1", "--justification", "Text", "--confirm"}, "--entitlement is required"},
		{[]string{"set", "--version-id", "version-1", "--entitlement", "com.apple.security.network.server", "--justification", "  ", "--confirm"}, "--justification is required"},
		{[]string{"set", "--version-id", "version-1", "--entitlement", "com.apple.security.network.server", "--justification", "Text"}, "--confirm is required"},
		{append(append([]string{"set"}, setArgs...), "--output", "yaml"), `(got "yaml")`},
		{[]string{"delete", "--version-id", "version-1", "--entitlement", "com.apple.security.network.server"}, "--confirm is required"},
	} {
		assertUsageExitWithDeps(t, nil, append([]string{"web", "review", "sandbox-entitlements"}, tc.args...), tc.message)
	}
}
