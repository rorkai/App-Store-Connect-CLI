package cmdtest

import (
	"context"
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
}

func TestSubscriptionsHelpShowsCanonicalCommerceSubcommands(t *testing.T) {
	root := RootCommand("1.2.3")

	subscriptionsCmd := findSubcommand(root, "subscriptions")
	if subscriptionsCmd == nil {
		t.Fatal("expected subscriptions command")
		return
	}
	subscriptionsUsage := subscriptionsCmd.UsageFunc(subscriptionsCmd)
	for _, expected := range []string{"pricing", "offers", "review", "promoted-purchases"} {
		if !usageListsSubcommand(subscriptionsUsage, expected) {
			t.Fatalf("expected subscriptions help to list %s, got %q", expected, subscriptionsUsage)
		}
	}
	for _, hidden := range []string{
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
	} {
		if usageListsSubcommand(subscriptionsUsage, hidden) {
			t.Fatalf("expected subscriptions help to hide deprecated flat subcommand %s, got %q", hidden, subscriptionsUsage)
		}
	}

	groupsCmd := findSubcommand(root, "subscriptions", "groups")
	if groupsCmd == nil {
		t.Fatal("expected subscriptions groups command")
		return
	}
	groupsUsage := groupsCmd.UsageFunc(groupsCmd)
	if usageListsSubcommand(groupsUsage, "submit") {
		t.Fatalf("expected subscriptions groups help to hide deprecated submit shim, got %q", groupsUsage)
	}

	pricingCmd := findSubcommand(root, "subscriptions", "pricing")
	if pricingCmd == nil {
		t.Fatal("expected subscriptions pricing command")
		return
	}
	pricingUsage := pricingCmd.UsageFunc(pricingCmd)
	for _, expected := range []string{"summary", "prices", "price-points", "availability"} {
		if !usageListsSubcommand(pricingUsage, expected) {
			t.Fatalf("expected subscriptions pricing help to list %s, got %q", expected, pricingUsage)
		}
	}
	if strings.Contains(pricingUsage, "\nFLAGS\n") {
		t.Fatalf("expected subscriptions pricing group help to avoid parent-level leaf flags, got %q", pricingUsage)
	}

	pricesCmd := findSubcommand(root, "subscriptions", "pricing", "prices")
	if pricesCmd == nil {
		t.Fatal("expected subscriptions pricing prices command")
		return
	}
	pricesUsage := pricesCmd.UsageFunc(pricesCmd)
	if !strings.Contains(pricesUsage, `asc subscriptions pricing prices list --subscription-id "SUB_ID"`) {
		t.Fatalf("expected subscriptions pricing prices help to show canonical subscription selector, got %q", pricesUsage)
	}
	if strings.Contains(pricesUsage, `asc subscriptions pricing prices list --id "SUB_ID"`) {
		t.Fatalf("expected subscriptions pricing prices help to drop legacy --id example, got %q", pricesUsage)
	}
	pricesListCmd := findSubcommand(root, "subscriptions", "pricing", "prices", "list")
	if pricesListCmd == nil {
		t.Fatal("expected subscriptions pricing prices list command")
		return
	}
	pricesListUsage := pricesListCmd.UsageFunc(pricesListCmd)
	if !strings.Contains(pricesListUsage, "--resolved") {
		t.Fatalf("expected subscriptions pricing prices list help to mention --resolved, got %q", pricesListUsage)
	}

	availabilityCmd := findSubcommand(root, "subscriptions", "pricing", "availability")
	if availabilityCmd == nil {
		t.Fatal("expected subscriptions pricing availability command")
		return
	}
	availabilityUsage := availabilityCmd.UsageFunc(availabilityCmd)
	if !strings.Contains(availabilityUsage, `asc subscriptions pricing availability view --availability-id "AVAILABILITY_ID"`) {
		t.Fatalf("expected subscriptions pricing availability help to show canonical availability selector, got %q", availabilityUsage)
	}
	if !strings.Contains(availabilityUsage, `asc subscriptions pricing availability edit --subscription-id "SUB_ID" --territories "US,Canada"`) {
		t.Fatalf("expected subscriptions pricing availability help to show canonical territory flags, got %q", availabilityUsage)
	}
	if strings.Contains(availabilityUsage, `asc subscriptions pricing availability get --availability-id "AVAILABILITY_ID"`) {
		t.Fatalf("expected subscriptions pricing availability help to hide deprecated get alias, got %q", availabilityUsage)
	}
	if strings.Contains(availabilityUsage, `asc subscriptions pricing availability set --subscription-id "SUB_ID" --territories "US,Canada"`) {
		t.Fatalf("expected subscriptions pricing availability help to hide deprecated set alias, got %q", availabilityUsage)
	}

	pricePointsCmd := findSubcommand(root, "subscriptions", "pricing", "price-points")
	if pricePointsCmd == nil {
		t.Fatal("expected subscriptions pricing price-points command")
		return
	}
	pricePointsUsage := pricePointsCmd.UsageFunc(pricePointsCmd)
	if !strings.Contains(pricePointsUsage, `asc subscriptions pricing price-points view --price-point-id "PRICE_POINT_ID"`) {
		t.Fatalf("expected subscriptions pricing price-points help to show canonical price point selector, got %q", pricePointsUsage)
	}
	if !strings.Contains(pricePointsUsage, `asc subscriptions pricing price-points equalizations --price-point-id "PRICE_POINT_ID"`) {
		t.Fatalf("expected subscriptions pricing price-points help to show canonical equalizations selector, got %q", pricePointsUsage)
	}
	if strings.Contains(pricePointsUsage, `asc subscriptions pricing price-points get --id "PRICE_POINT_ID"`) {
		t.Fatalf("expected subscriptions pricing price-points help to drop legacy --id example, got %q", pricePointsUsage)
	}

	offersCmd := findSubcommand(root, "subscriptions", "offers")
	if offersCmd == nil {
		t.Fatal("expected subscriptions offers command")
		return
	}
	offersUsage := offersCmd.UsageFunc(offersCmd)
	for _, expected := range []string{"introductory", "promotional", "offer-codes", "win-back"} {
		if !usageListsSubcommand(offersUsage, expected) {
			t.Fatalf("expected subscriptions offers help to list %s, got %q", expected, offersUsage)
		}
	}

	introductoryViewCmd := findSubcommand(root, "subscriptions", "offers", "introductory", "view")
	if introductoryViewCmd == nil {
		t.Fatal("expected subscriptions offers introductory view command")
		return
	}
	introductoryViewUsage := introductoryViewCmd.UsageFunc(introductoryViewCmd)
	if !strings.Contains(introductoryViewUsage, `asc subscriptions offers introductory view --subscription-id "SUB_ID" --id "OFFER_ID"`) {
		t.Fatalf("expected introductory offer view help to require the parent subscription, got %q", introductoryViewUsage)
	}
	if !strings.Contains(introductoryViewUsage, "--app") {
		t.Fatalf("expected introductory offer view help to document subscription resolution, got %q", introductoryViewUsage)
	}
	if strings.Contains(introductoryViewUsage, `asc subscriptions offers introductory view --id "OFFER_ID"`) {
		t.Fatalf("expected introductory offer view help to drop the unsupported id-only invocation, got %q", introductoryViewUsage)
	}

	offerCodesCmd := findSubcommand(root, "subscriptions", "offers", "offer-codes")
	if offerCodesCmd == nil {
		t.Fatal("expected subscriptions offers offer-codes command")
		return
	}
	offerCodesUsage := offerCodesCmd.UsageFunc(offerCodesCmd)
	if !strings.Contains(offerCodesUsage, "  generate") {
		t.Fatalf("expected subscriptions offers offer-codes help to list generate, got %q", offerCodesUsage)
	}
	if !strings.Contains(offerCodesUsage, "  values") {
		t.Fatalf("expected subscriptions offers offer-codes help to list values, got %q", offerCodesUsage)
	}
	if !strings.Contains(offerCodesUsage, `--prices "US:PRICE_POINT_ID"`) {
		t.Fatalf("expected subscriptions offers offer-codes help to show territory-qualified price examples, got %q", offerCodesUsage)
	}
	if !strings.Contains(offerCodesUsage, `--prices "US"`) {
		t.Fatalf("expected subscriptions offers offer-codes help to show FREE_TRIAL territory examples, got %q", offerCodesUsage)
	}
	if strings.Contains(offerCodesUsage, `--prices "PRICE_ID"`) {
		t.Fatalf("expected subscriptions offers offer-codes help to drop stale price example, got %q", offerCodesUsage)
	}

	promotionalCmd := findSubcommand(root, "subscriptions", "offers", "promotional")
	if promotionalCmd == nil {
		t.Fatal("expected subscriptions offers promotional command")
		return
	}
	promotionalUsage := promotionalCmd.UsageFunc(promotionalCmd)
	if !strings.Contains(promotionalUsage, `--prices "US"`) {
		t.Fatalf("expected promotional offer help to show FREE_TRIAL territory prices, got %q", promotionalUsage)
	}
	if strings.Contains(promotionalUsage, `--prices "PRICE_ID"`) {
		t.Fatalf("expected promotional offer help to drop pre-existing price IDs, got %q", promotionalUsage)
	}
	promotionalCreateCmd := findSubcommand(root, "subscriptions", "offers", "promotional", "create")
	if promotionalCreateCmd == nil {
		t.Fatal("expected subscriptions offers promotional create command")
		return
	}
	promotionalCreateUsage := promotionalCreateCmd.UsageFunc(promotionalCreateCmd)
	if !strings.Contains(promotionalCreateUsage, `--prices "US:PRICE_POINT_ID"`) {
		t.Fatalf("expected promotional offer create help to show paid territory prices, got %q", promotionalCreateUsage)
	}
	if strings.Contains(promotionalCreateUsage, `--prices "PRICE_ID"`) {
		t.Fatalf("expected promotional offer create help to drop pre-existing price IDs, got %q", promotionalCreateUsage)
	}

	reviewCmd := findSubcommand(root, "subscriptions", "review")
	if reviewCmd == nil {
		t.Fatal("expected subscriptions review command")
		return
	}
	reviewUsage := reviewCmd.UsageFunc(reviewCmd)
	for _, expected := range []string{"screenshots", "app-store-screenshot", "submit", "submit-group"} {
		if !usageListsSubcommand(reviewUsage, expected) {
			t.Fatalf("expected subscriptions review help to list %s, got %q", expected, reviewUsage)
		}
	}

	promotedPurchasesCreateCmd := findSubcommand(root, "subscriptions", "promoted-purchases", "create")
	if promotedPurchasesCreateCmd == nil {
		t.Fatal("expected subscriptions promoted-purchases create command")
		return
	}
	promotedPurchasesCreateUsage := promotedPurchasesCreateCmd.UsageFunc(promotedPurchasesCreateCmd)
	if strings.Contains(promotedPurchasesCreateUsage, "--product-type") {
		t.Fatalf("expected canonical promoted-purchases create help to hide --product-type, got %q", promotedPurchasesCreateUsage)
	}

	subscriptionsPromotedPurchasesCmd := findSubcommand(root, "subscriptions", "promoted-purchases")
	if subscriptionsPromotedPurchasesCmd == nil {
		t.Fatal("expected subscriptions promoted-purchases command")
		return
	}
	subscriptionsPromotedPurchasesUsage := subscriptionsPromotedPurchasesCmd.UsageFunc(subscriptionsPromotedPurchasesCmd)
	if strings.Contains(subscriptionsPromotedPurchasesUsage, "subscriptions and in-app purchases") {
		t.Fatalf("expected subscriptions promoted-purchases help to avoid generic mixed-scope wording, got %q", subscriptionsPromotedPurchasesUsage)
	}
	if strings.Contains(subscriptionsPromotedPurchasesUsage, "--product-type SUBSCRIPTION") {
		t.Fatalf("expected subscriptions promoted-purchases help to avoid stale generic create example, got %q", subscriptionsPromotedPurchasesUsage)
	}
	if !strings.Contains(subscriptionsPromotedPurchasesUsage, `asc subscriptions promoted-purchases create --app "APP_ID" --product-id "SUB_ID" --visible-for-all-users true`) {
		t.Fatalf("expected subscriptions promoted-purchases help to show scoped create example, got %q", subscriptionsPromotedPurchasesUsage)
	}

	iapCmd := findSubcommand(root, "iap")
	if iapCmd == nil {
		t.Fatal("expected iap command")
		return
	}
	iapUsage := iapCmd.UsageFunc(iapCmd)
	if !strings.Contains(iapUsage, "  promoted-purchases") {
		t.Fatalf("expected iap help to list promoted-purchases, got %q", iapUsage)
	}
	if usageListsSubcommand(iapUsage, "promoted-purchase") {
		t.Fatalf("expected iap help to hide deprecated singular promoted-purchase shim, got %q", iapUsage)
	}

	iapPromotedPurchasesCmd := findSubcommand(root, "iap", "promoted-purchases")
	if iapPromotedPurchasesCmd == nil {
		t.Fatal("expected iap promoted-purchases command")
		return
	}
	iapPromotedPurchasesUsage := iapPromotedPurchasesCmd.UsageFunc(iapPromotedPurchasesCmd)
	if strings.Contains(iapPromotedPurchasesUsage, "subscriptions and in-app purchases") {
		t.Fatalf("expected iap promoted-purchases help to avoid generic mixed-scope wording, got %q", iapPromotedPurchasesUsage)
	}
	if strings.Contains(iapPromotedPurchasesUsage, "--product-type SUBSCRIPTION") {
		t.Fatalf("expected iap promoted-purchases help to avoid stale subscription create example, got %q", iapPromotedPurchasesUsage)
	}
	if !strings.Contains(iapPromotedPurchasesUsage, `asc iap promoted-purchases create --app "APP_ID" --product-id "IAP_ID" --visible-for-all-users true`) {
		t.Fatalf("expected iap promoted-purchases help to show scoped create example, got %q", iapPromotedPurchasesUsage)
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
	}{
		{
			name:    "subscriptions offers win-back next validation",
			args:    []string{"subscriptions", "offers", "win-back", "list", "--next", "http://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/winBackOffers?cursor=AQ"},
			wantErr: "subscriptions offers win-back list: --next must be an App Store Connect URL",
		},
		{
			name:    "subscriptions promoted-purchases next validation",
			args:    []string{"subscriptions", "promoted-purchases", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps/app-1/promotedPurchases?cursor=AQ"},
			wantErr: "subscriptions promoted-purchases list: --next must be an App Store Connect URL",
		},
		{
			name:    "iap promoted-purchases next validation",
			args:    []string{"iap", "promoted-purchases", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps/app-1/promotedPurchases?cursor=AQ"},
			wantErr: "iap promoted-purchases list: --next must be an App Store Connect URL",
		},
		{
			name:    "subscriptions offers offer-codes values auth error",
			args:    []string{"subscriptions", "offers", "offer-codes", "values", "--batch-id", "batch-1"},
			wantErr: "subscriptions offers offer-codes values:",
		},
		{
			name:    "subscriptions pricing prices next validation",
			args:    []string{"subscriptions", "pricing", "prices", "list", "--next", "http://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/prices?cursor=AQ"},
			wantErr: "subscriptions pricing prices list: --next must be an App Store Connect URL",
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
