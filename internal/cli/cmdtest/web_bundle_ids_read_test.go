package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"

	webcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebBundleIDsViewMissingIDFailsBeforeSession(t *testing.T) {
	restoreSession := webcmd.SetResolveWebSession(func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("web session must not resolve when --bundle-id is missing")
		return nil, "", nil
	})
	defer restoreSession()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "bundle-ids", "view"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --bundle-id is required") {
		t.Fatalf("expected missing id diagnostic, got %q", stderr)
	}
}

func TestWebBundleIDsListRootOutputsJSON(t *testing.T) {
	restoreSession := webcmd.SetResolveWebSession(func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	defer restoreSession()
	restoreList := webcmd.SetListDeveloperBundleIDs(func(context.Context, *webcore.Client) (*webcore.DeveloperBundleIDsListResult, error) {
		return &webcore.DeveloperBundleIDsListResult{
			Data: []webcore.DeveloperBundleID{{
				ID:   "bundle-1",
				Type: "bundleIds",
				Attributes: map[string]any{
					"name":       "Example App",
					"identifier": "com.example.app",
					"platform":   "IOS",
				},
			}},
		}, nil
	})
	defer restoreList()
	restorePersist := webcmd.SetPersistWebSession(func(*webcore.AuthSession) error { return nil })
	defer restorePersist()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "bundle-ids", "list", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("unexpected run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result webcore.DeveloperBundleIDsListResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode JSON output %q: %v", stdout, err)
	}
	if len(result.Data) != 1 || result.Data[0].ID != "bundle-1" || result.Data[0].Attributes["identifier"] != "com.example.app" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
