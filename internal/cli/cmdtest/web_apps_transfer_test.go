package cmdtest

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestWebAppsTransferStatusRequiresApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	root := RootCommand("test")
	if findSubcommand(root, "web", "apps", "transfer", "status") == nil {
		t.Fatal("transfer status is not registered")
	}
	stdout, stderr, runErr := runRootWithDeps(t, nil, "web", "apps", "transfer", "status")
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(stderr, "--app is required") || stdout != "" {
		t.Fatalf("err=%v stdout=%q stderr=%q", runErr, stdout, stderr)
	}
}
