package cmdtest

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestBuildIDRequiredErrorsNameCanonicalFlag(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name string
		args []string
	}{
		{name: "validate testflight", args: []string{"validate", "testflight", "--app", "app-1"}},
		{name: "build-bundles list", args: []string{"build-bundles", "list"}},
		{name: "build-localizations create", args: []string{"build-localizations", "create", "--locale", "en-US"}},
		{name: "performance diagnostics list", args: []string{"performance", "diagnostics", "list"}},
		{name: "performance metrics view", args: []string{"performance", "metrics", "view"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runRootWithDeps(t, nil, test.args...)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "Error: --build-id is required") {
				t.Fatalf("expected --build-id required error, got %q", stderr)
			}
		})
	}
}
