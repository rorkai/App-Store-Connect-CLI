package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebBundleIDCapabilitiesListRequiresBundleIDBeforeAuthentication(t *testing.T) {
	d, out, diagnostics := newWebTestDeps()
	cmd := newWebBundleIDCapabilitiesListCommand(d)
	stdout, stderr := runWithWebTestDeps(out, diagnostics, func() {
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
	if stdout != "" || !strings.Contains(stderr, "--bundle-id is required") {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestWebBundleIDCapabilitiesListOutputsCatalog(t *testing.T) {
	defer stubWebBundleIDReadDependencies(t)()
	original := listDeveloperBundleIDCapabilitiesFn
	t.Cleanup(func() { listDeveloperBundleIDCapabilitiesFn = original })
	const envelope = `{"data":[{"type":"capabilities","id":"CARPLAY_PLAYABLE_CONTENT","attributes":{"name":"CarPlay Audio App","editable":true}}],"future":[]}`
	listDeveloperBundleIDCapabilitiesFn = func(_ context.Context, _ *webcore.Client, bundleID string) (*webcore.DeveloperBundleIDCapabilitiesListResult, error) {
		if bundleID != "bundle-1" {
			t.Fatalf("bundle context=%q", bundleID)
		}
		var result webcore.DeveloperBundleIDCapabilitiesListResult
		if err := json.Unmarshal([]byte(envelope), &result); err != nil {
			t.Fatal(err)
		}
		result.Raw = json.RawMessage(envelope)
		return &result, nil
	}
	for _, format := range []string{"json", "table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			d, out, diagnostics := newWebTestDeps()
			cmd := newWebBundleIDCapabilitiesListCommand(d)
			if err := cmd.FlagSet.Parse([]string{"--bundle-id", " bundle-1 ", "--output", format}); err != nil {
				t.Fatal(err)
			}
			stdout, stderr := runWithWebTestDeps(out, diagnostics, func() {
				if err := cmd.Exec(context.Background(), nil); err != nil {
					t.Fatal(err)
				}
			})
			if stderr != "" {
				t.Fatalf("unexpected diagnostics %q", stderr)
			}
			if format == "json" {
				if strings.TrimSpace(stdout) != envelope {
					t.Fatalf("Apple envelope changed: %s", stdout)
				}
			} else {
				for _, value := range []string{"ID", "Name", "Editable", "CARPLAY_PLAYABLE_CONTENT", "CarPlay Audio App", "true"} {
					if !strings.Contains(stdout, value) {
						t.Fatalf("missing %q in %s", value, stdout)
					}
				}
			}
		})
	}
}
