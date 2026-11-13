package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/subscriptions"
)

func TestSubscriptionsAvailabilityHelpMatchesRegisteredPath(t *testing.T) {
	root := RootCommand("1.2.3")
	source := subscriptions.SubscriptionsAvailabilityCommand()

	tests := []struct {
		name          string
		sourceCommand *ffcli.Command
		rootPath      []string
		usagePrefix   string
	}{
		{
			name:          "group",
			sourceCommand: source,
			rootPath:      []string{"subscriptions", "pricing", "availability"},
			usagePrefix:   "asc subscriptions pricing availability ",
		},
		{
			name:          "view",
			sourceCommand: findSubcommand(source, "view"),
			rootPath:      []string{"subscriptions", "pricing", "availability", "view"},
			usagePrefix:   "asc subscriptions pricing availability view ",
		},
		{
			name:          "available-territories",
			sourceCommand: findSubcommand(source, "available-territories"),
			rootPath:      []string{"subscriptions", "pricing", "availability", "available-territories"},
			usagePrefix:   "asc subscriptions pricing availability available-territories ",
		},
		{
			name:          "edit",
			sourceCommand: findSubcommand(source, "edit"),
			rootPath:      []string{"subscriptions", "pricing", "availability", "edit"},
			usagePrefix:   "asc subscriptions pricing availability edit ",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.sourceCommand == nil {
				t.Fatal("expected source command")
			}
			if registered := findSubcommand(root, test.rootPath...); registered == nil {
				t.Fatalf("expected registered command %q", strings.Join(test.rootPath, " "))
			}
			if !strings.HasPrefix(test.sourceCommand.ShortUsage, test.usagePrefix) {
				t.Fatalf("expected source usage to match registered path %q, got %q", strings.Join(test.rootPath, " "), test.sourceCommand.ShortUsage)
			}
			if strings.Contains(test.sourceCommand.LongHelp, "asc subscriptions availability") {
				t.Fatalf("expected examples to use registered pricing path, got %q", test.sourceCommand.LongHelp)
			}
		})
	}

	availableTerritories := findSubcommand(source, "available-territories")
	if !strings.Contains(availableTerritories.LongHelp, "Use --next instead of either selector") {
		t.Fatalf("expected available-territories help to explain cursor continuation, got %q", availableTerritories.LongHelp)
	}
}

func TestSubscriptionsHelpShowsCanonicalCommerceSubcommands(t *testing.T) {
	root := RootCommand("1.2.3")

	tests := []struct {
		path   []string
		shown  []string
		hidden []string
	}{
		{
			path:  []string{"subscriptions"},
			shown: []string{"setup", "pricing", "offers", "review", "promoted-purchases"},
			hidden: []string{
				"prices",
				"availability",
				"price-points",
				"introductory-offers",
				"promotional-offers",
				"offer-codes",
				"win-back-offers",
				"review-screenshots",
				"app-store-review-screenshot",
				"submit",
				"promoted-purchase",
			},
		},
		{path: []string{"subscriptions", "groups"}, hidden: []string{"submit"}},
		{path: []string{"subscriptions", "pricing"}, shown: []string{"summary", "prices", "price-points", "availability"}},
		{path: []string{"subscriptions", "offers"}, shown: []string{"introductory", "promotional", "offer-codes", "win-back"}},
		{path: []string{"subscriptions", "offers", "offer-codes"}, shown: []string{"generate", "values"}},
		{path: []string{"subscriptions", "review"}, shown: []string{"screenshots", "app-store-screenshot"}, hidden: []string{"submit", "submit-group"}},
		{path: []string{"iap"}, shown: []string{"promoted-purchases"}, hidden: []string{"promoted-purchase"}},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.path, " "), func(t *testing.T) {
			command := findSubcommand(root, test.path...)
			if command == nil {
				t.Fatalf("expected %s command", strings.Join(test.path, " "))
			}
			usage := command.UsageFunc(command)
			for _, name := range test.shown {
				if !usageListsSubcommand(usage, name) {
					t.Fatalf("expected help to list %s, got %q", name, usage)
				}
			}
			for _, name := range test.hidden {
				if usageListsSubcommand(usage, name) {
					t.Fatalf("expected help to hide %s, got %q", name, usage)
				}
			}
		})
	}

	pricingCmd := findSubcommand(root, "subscriptions", "pricing")
	if pricingUsage := pricingCmd.UsageFunc(pricingCmd); strings.Contains(pricingUsage, "\nFLAGS\n") {
		t.Fatalf("expected subscriptions pricing group help to avoid parent-level leaf flags, got %q", pricingUsage)
	}
	createCmd := findSubcommand(root, "subscriptions", "promoted-purchases", "create")
	if createUsage := createCmd.UsageFunc(createCmd); strings.Contains(createUsage, "--product-type") {
		t.Fatalf("expected canonical promoted-purchases create help to hide --product-type, got %q", createUsage)
	}
}

func TestRemovedLegacyCommerceRootCommandsAreNotRegistered(t *testing.T) {
	root := RootCommand("1.2.3")

	for _, name := range []string{"offer-codes", "win-back-offers", "promoted-purchases"} {
		if cmd := findSubcommand(root, name); cmd != nil {
			t.Fatalf("expected removed root command %s to be absent", name)
		}
	}
}

func TestCanonicalWrapperErrorsUseCanonicalPaths(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
		// usageError marks a pre-request flag check converted to a usage
		// error (#518). Such a check reports itself while the command is
		// still running, so the canonical path must already be in the
		// printed diagnostic rather than only in the returned error.
		usageError bool
	}{
		{
			name:       "subscriptions offers win-back next validation",
			args:       []string{"subscriptions", "offers", "win-back", "list", "--next", "http://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/winBackOffers?cursor=AQ"},
			wantErr:    "subscriptions offers win-back list: --next must be an App Store Connect URL",
			usageError: true,
		},
		{
			name:       "subscriptions promoted-purchases next validation",
			args:       []string{"subscriptions", "promoted-purchases", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps/app-1/promotedPurchases?cursor=AQ"},
			wantErr:    "subscriptions promoted-purchases list: --next must be an App Store Connect URL",
			usageError: true,
		},
		{
			name:       "iap promoted-purchases next validation",
			args:       []string{"iap", "promoted-purchases", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps/app-1/promotedPurchases?cursor=AQ"},
			wantErr:    "iap promoted-purchases list: --next must be an App Store Connect URL",
			usageError: true,
		},
		{
			name:    "subscriptions offers offer-codes values auth error",
			args:    []string{"subscriptions", "offers", "offer-codes", "values", "--batch-id", "batch-1"},
			wantErr: "subscriptions offers offer-codes values:",
		},
		{
			name:       "subscriptions pricing prices next validation",
			args:       []string{"subscriptions", "pricing", "prices", "list", "--next", "http://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/prices?cursor=AQ"},
			wantErr:    "subscriptions pricing prices list: --next must be an App Store Connect URL",
			usageError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("expected error %q, got %v", test.wantErr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if test.usageError {
				if !errors.Is(runErr, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", runErr)
				}
				if !strings.Contains(stderr, "Error: "+test.wantErr) {
					t.Fatalf("expected stderr to contain %q, got %q", "Error: "+test.wantErr, stderr)
				}
				return
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func usageListsSubcommand(usage string, name string) bool {
	for _, line := range strings.Split(usage, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == name {
			return true
		}
	}
	return false
}
