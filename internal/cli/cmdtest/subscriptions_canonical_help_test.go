package cmdtest

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestSubscriptionsHelpShowsCanonicalCommerceSubcommands(t *testing.T) {
	root := RootCommand("1.2.3")

	subscriptionsCmd := findSubcommand(root, "subscriptions")
	if subscriptionsCmd == nil {
		t.Fatal("expected subscriptions command")
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
	}
	groupsUsage := groupsCmd.UsageFunc(groupsCmd)
	if usageListsSubcommand(groupsUsage, "submit") {
		t.Fatalf("expected subscriptions groups help to hide deprecated submit shim, got %q", groupsUsage)
	}

	pricingCmd := findSubcommand(root, "subscriptions", "pricing")
	if pricingCmd == nil {
		t.Fatal("expected subscriptions pricing command")
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
	}
	pricesUsage := pricesCmd.UsageFunc(pricesCmd)
	if !strings.Contains(pricesUsage, `asc subscriptions pricing prices list --subscription-id "SUB_ID"`) {
		t.Fatalf("expected subscriptions pricing prices help to show canonical subscription selector, got %q", pricesUsage)
	}
	if strings.Contains(pricesUsage, `asc subscriptions pricing prices list --id "SUB_ID"`) {
		t.Fatalf("expected subscriptions pricing prices help to drop legacy --id example, got %q", pricesUsage)
	}

	availabilityCmd := findSubcommand(root, "subscriptions", "pricing", "availability")
	if availabilityCmd == nil {
		t.Fatal("expected subscriptions pricing availability command")
	}
	availabilityUsage := availabilityCmd.UsageFunc(availabilityCmd)
	if !strings.Contains(availabilityUsage, `asc subscriptions pricing availability view --availability-id "AVAILABILITY_ID"`) {
		t.Fatalf("expected subscriptions pricing availability help to show canonical availability selector, got %q", availabilityUsage)
	}
	if !strings.Contains(availabilityUsage, `asc subscriptions pricing availability edit --subscription-id "SUB_ID" --territories "USA,CAN"`) {
		t.Fatalf("expected subscriptions pricing availability help to show canonical territory flags, got %q", availabilityUsage)
	}
	if strings.Contains(availabilityUsage, `asc subscriptions pricing availability get --availability-id "AVAILABILITY_ID"`) {
		t.Fatalf("expected subscriptions pricing availability help to hide deprecated get alias, got %q", availabilityUsage)
	}
	if strings.Contains(availabilityUsage, `asc subscriptions pricing availability set --subscription-id "SUB_ID" --territories "USA,CAN"`) {
		t.Fatalf("expected subscriptions pricing availability help to hide deprecated set alias, got %q", availabilityUsage)
	}

	pricePointsCmd := findSubcommand(root, "subscriptions", "pricing", "price-points")
	if pricePointsCmd == nil {
		t.Fatal("expected subscriptions pricing price-points command")
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
	}
	offersUsage := offersCmd.UsageFunc(offersCmd)
	for _, expected := range []string{"introductory", "promotional", "offer-codes", "win-back"} {
		if !usageListsSubcommand(offersUsage, expected) {
			t.Fatalf("expected subscriptions offers help to list %s, got %q", expected, offersUsage)
		}
	}

	offerCodesCmd := findSubcommand(root, "subscriptions", "offers", "offer-codes")
	if offerCodesCmd == nil {
		t.Fatal("expected subscriptions offers offer-codes command")
	}
	offerCodesUsage := offerCodesCmd.UsageFunc(offerCodesCmd)
	if !strings.Contains(offerCodesUsage, "  generate") {
		t.Fatalf("expected subscriptions offers offer-codes help to list generate, got %q", offerCodesUsage)
	}
	if !strings.Contains(offerCodesUsage, "  values") {
		t.Fatalf("expected subscriptions offers offer-codes help to list values, got %q", offerCodesUsage)
	}
	if !strings.Contains(offerCodesUsage, `--prices "USA:PRICE_POINT_ID"`) {
		t.Fatalf("expected subscriptions offers offer-codes help to show territory-qualified price examples, got %q", offerCodesUsage)
	}
	if strings.Contains(offerCodesUsage, `--prices "PRICE_ID"`) {
		t.Fatalf("expected subscriptions offers offer-codes help to drop stale price example, got %q", offerCodesUsage)
	}

	reviewCmd := findSubcommand(root, "subscriptions", "review")
	if reviewCmd == nil {
		t.Fatal("expected subscriptions review command")
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
	}
	promotedPurchasesCreateUsage := promotedPurchasesCreateCmd.UsageFunc(promotedPurchasesCreateCmd)
	if strings.Contains(promotedPurchasesCreateUsage, "--product-type") {
		t.Fatalf("expected canonical promoted-purchases create help to hide --product-type, got %q", promotedPurchasesCreateUsage)
	}

	subscriptionsPromotedPurchasesCmd := findSubcommand(root, "subscriptions", "promoted-purchases")
	if subscriptionsPromotedPurchasesCmd == nil {
		t.Fatal("expected subscriptions promoted-purchases command")
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
