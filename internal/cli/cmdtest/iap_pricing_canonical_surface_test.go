package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func findCommandByPath(t *testing.T, path ...string) *ffcli.Command {
	t.Helper()

	current := RootCommand("1.2.3")
	for _, name := range path {
		found := false
		for _, sub := range current.Subcommands {
			if sub != nil && sub.Name == name {
				current = sub
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected command path %q to exist", strings.Join(path, " "))
		}
	}
	return current
}

func usageForCommand(t *testing.T, path ...string) string {
	t.Helper()

	cmd := findCommandByPath(t, path...)
	if cmd.UsageFunc == nil {
		t.Fatalf("expected usage func for %q", strings.Join(path, " "))
	}
	return cmd.UsageFunc(cmd)
}

func runRootCommand(t *testing.T, args []string) (string, string, error) {
	t.Helper()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	return stdout, stderr, runErr
}

func TestIAPHelpShowsCanonicalPricingRoot(t *testing.T) {
	stdout, stderr, runErr := runRootCommand(t, []string{"iap"})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "\n  pricing") {
		t.Fatalf("expected iap help to list pricing root, got %q", stderr)
	}
	for _, hidden := range []string{
		"\n  prices",
		"\n  price-points",
		"\n  price-schedules",
		"\n  availability",
		"\n  availabilities",
	} {
		if strings.Contains(stderr, hidden) {
			t.Fatalf("expected iap help to hide deprecated pricing alias %q, got %q", strings.TrimSpace(hidden), stderr)
		}
	}
}

func TestIAPPricingHelpShowsCanonicalSubcommands(t *testing.T) {
	stdout, stderr, runErr := runRootCommand(t, []string{"iap", "pricing"})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}

	for _, want := range []string{"summary", "price-points", "schedules", "availability", "availabilities"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected pricing help to contain %q, got %q", want, stderr)
		}
	}
	for _, hidden := range []string{"prices", "price-schedules"} {
		if strings.Contains(stderr, "\n  "+hidden+"\t") {
			t.Fatalf("expected pricing help to avoid legacy alias %q, got %q", hidden, stderr)
		}
	}
}

func TestIAPPricingMovedSurfacesUseCanonicalHelp(t *testing.T) {
	tests := []struct {
		name         string
		path         []string
		wantUsage    string
		wantContains []string
		wantNotShown []string
	}{
		{
			name:         "summary",
			path:         []string{"iap", "pricing", "summary"},
			wantUsage:    "asc iap pricing summary [flags]",
			wantContains: []string{"Show consolidated in-app purchase pricing summary."},
			wantNotShown: []string{"asc iap prices [flags]"},
		},
		{
			name:         "price points",
			path:         []string{"iap", "pricing", "price-points"},
			wantUsage:    "asc iap pricing price-points <subcommand> [flags]",
			wantContains: []string{"list", "equalizations"},
			wantNotShown: []string{"asc iap price-points <subcommand> [flags]"},
		},
		{
			name:         "schedules",
			path:         []string{"iap", "pricing", "schedules"},
			wantUsage:    "asc iap pricing schedules <subcommand> [flags]",
			wantContains: []string{"view", "base-territory", "create", "manual-prices", "automatic-prices"},
			wantNotShown: []string{"asc iap price-schedules <subcommand> [flags]"},
		},
		{
			name:         "availability",
			path:         []string{"iap", "pricing", "availability"},
			wantUsage:    "asc iap pricing availability <subcommand> [flags]",
			wantContains: []string{"view", "set"},
			wantNotShown: []string{"asc iap availability <subcommand> [flags]"},
		},
		{
			name:         "availabilities",
			path:         []string{"iap", "pricing", "availabilities"},
			wantUsage:    "asc iap pricing availabilities <subcommand> [flags]",
			wantContains: []string{"view", "available-territories"},
			wantNotShown: []string{"asc iap availabilities <subcommand> [flags]"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usage := usageForCommand(t, test.path...)
			if !strings.Contains(usage, test.wantUsage) {
				t.Fatalf("expected usage to contain %q, got %q", test.wantUsage, usage)
			}
			for _, want := range test.wantContains {
				if !strings.Contains(usage, want) {
					t.Fatalf("expected usage to contain %q, got %q", want, usage)
				}
			}
			for _, hidden := range test.wantNotShown {
				if strings.Contains(usage, hidden) {
					t.Fatalf("expected usage to hide %q, got %q", hidden, usage)
				}
			}
		})
	}
}

func TestRemovedIAPPricingAliasPathsFallBackToCanonicalIAPHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "prices alias",
			args: []string{"iap", "prices"},
		},
		{
			name: "price points alias",
			args: []string{"iap", "price-points", "list"},
		},
		{
			name: "schedules alias",
			args: []string{"iap", "price-schedules", "get"},
		},
		{
			name: "availability alias",
			args: []string{"iap", "availability", "get"},
		},
		{
			name: "availabilities alias",
			args: []string{"iap", "availabilities", "get"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runRootCommand(t, test.args)

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "asc iap <subcommand> [flags]") {
				t.Fatalf("expected removed alias path to fall back to iap help, got %q", stderr)
			}
			if !strings.Contains(stderr, "asc iap pricing summary --app \"APP_ID\"") {
				t.Fatalf("expected removed alias path to point at canonical pricing help, got %q", stderr)
			}
			for _, hidden := range []string{"\n  prices", "\n  price-points", "\n  price-schedules", "\n  availability", "\n  availabilities"} {
				if strings.Contains(stderr, hidden) {
					t.Fatalf("expected removed alias help to keep %q hidden, got %q", strings.TrimSpace(hidden), stderr)
				}
			}
		})
	}
}

func TestCanonicalIAPPricingValidationPaths(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "summary missing selector",
			args:    []string{"iap", "pricing", "summary"},
			wantErr: "--app or --iap-id is required",
		},
		{
			name:    "price points missing iap id",
			args:    []string{"iap", "pricing", "price-points", "list"},
			wantErr: "--iap-id is required",
		},
		{
			name:    "schedules missing selector",
			args:    []string{"iap", "pricing", "schedules", "view"},
			wantErr: "--iap-id or --schedule-id is required",
		},
		{
			name:    "availability missing iap id",
			args:    []string{"iap", "pricing", "availability", "view"},
			wantErr: "--iap-id is required",
		},
		{
			name:    "availabilities missing id",
			args:    []string{"iap", "pricing", "availabilities", "view"},
			wantErr: "--id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runRootCommand(t, test.args)

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
