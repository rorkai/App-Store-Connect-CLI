package offercodes

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

var offerCodeDurationValues = []string{
	string(asc.SubscriptionOfferDurationThreeDays),
	string(asc.SubscriptionOfferDurationOneWeek),
	string(asc.SubscriptionOfferDurationTwoWeeks),
	string(asc.SubscriptionOfferDurationOneMonth),
	string(asc.SubscriptionOfferDurationTwoMonths),
	string(asc.SubscriptionOfferDurationThreeMonths),
	string(asc.SubscriptionOfferDurationSixMonths),
	string(asc.SubscriptionOfferDurationOneYear),
}

var offerCodeDurationMap = map[string]asc.SubscriptionOfferDuration{
	string(asc.SubscriptionOfferDurationThreeDays):   asc.SubscriptionOfferDurationThreeDays,
	string(asc.SubscriptionOfferDurationOneWeek):     asc.SubscriptionOfferDurationOneWeek,
	string(asc.SubscriptionOfferDurationTwoWeeks):    asc.SubscriptionOfferDurationTwoWeeks,
	string(asc.SubscriptionOfferDurationOneMonth):    asc.SubscriptionOfferDurationOneMonth,
	string(asc.SubscriptionOfferDurationTwoMonths):   asc.SubscriptionOfferDurationTwoMonths,
	string(asc.SubscriptionOfferDurationThreeMonths): asc.SubscriptionOfferDurationThreeMonths,
	string(asc.SubscriptionOfferDurationSixMonths):   asc.SubscriptionOfferDurationSixMonths,
	string(asc.SubscriptionOfferDurationOneYear):     asc.SubscriptionOfferDurationOneYear,
}

var offerCodeModeValues = []string{
	string(asc.SubscriptionOfferModePayAsYouGo),
	string(asc.SubscriptionOfferModePayUpFront),
	string(asc.SubscriptionOfferModeFreeTrial),
}

var offerCodeModeMap = map[string]asc.SubscriptionOfferMode{
	string(asc.SubscriptionOfferModePayAsYouGo): asc.SubscriptionOfferModePayAsYouGo,
	string(asc.SubscriptionOfferModePayUpFront): asc.SubscriptionOfferModePayUpFront,
	string(asc.SubscriptionOfferModeFreeTrial):  asc.SubscriptionOfferModeFreeTrial,
}

var offerCodeEligibilityValues = []string{
	string(asc.SubscriptionOfferEligibilityStackWithIntroOffers),
	string(asc.SubscriptionOfferEligibilityReplaceIntroOffers),
}

var offerCodeEligibilityMap = map[string]asc.SubscriptionOfferEligibility{
	string(asc.SubscriptionOfferEligibilityStackWithIntroOffers): asc.SubscriptionOfferEligibilityStackWithIntroOffers,
	string(asc.SubscriptionOfferEligibilityReplaceIntroOffers):   asc.SubscriptionOfferEligibilityReplaceIntroOffers,
}

var offerCodeCustomerEligibilityValues = []string{
	string(asc.SubscriptionCustomerEligibilityNew),
	string(asc.SubscriptionCustomerEligibilityExisting),
	string(asc.SubscriptionCustomerEligibilityExpired),
}

var offerCodeCustomerEligibilityMap = map[string]asc.SubscriptionCustomerEligibility{
	string(asc.SubscriptionCustomerEligibilityNew):      asc.SubscriptionCustomerEligibilityNew,
	string(asc.SubscriptionCustomerEligibilityExisting): asc.SubscriptionCustomerEligibilityExisting,
	string(asc.SubscriptionCustomerEligibilityExpired):  asc.SubscriptionCustomerEligibilityExpired,
}

// OfferCodesGetCommand returns the offer codes get subcommand.
func OfferCodesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Subscription offer code ID (required)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc offer-codes get --offer-code-id ID",
		ShortHelp:  "Get a subscription offer code by ID.",
		LongHelp: `Get a subscription offer code by ID.

Examples:
  asc offer-codes get --offer-code-id "OFFER_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*offerCodeID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("offer-codes get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionOfferCode(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("offer-codes get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// OfferCodesCreateCommand returns the offer codes create subcommand.
func OfferCodesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID (required)")
	name := fs.String("name", "", "Offer code name (required)")
	customerEligibilities := fs.String("customer-eligibilities", "", "Customer eligibilities: "+strings.Join(offerCodeCustomerEligibilityValues, ", "))
	offerEligibility := fs.String("offer-eligibility", "", "Offer eligibility: "+strings.Join(offerCodeEligibilityValues, ", "))
	duration := fs.String("duration", "", "Offer duration: "+strings.Join(offerCodeDurationValues, ", "))
	offerMode := fs.String("offer-mode", "", "Offer mode: "+strings.Join(offerCodeModeValues, ", "))
	var numberOfPeriods optionalInt
	fs.Var(&numberOfPeriods, "number-of-periods", "Number of periods (required)")
	autoRenewEnabled := fs.String("auto-renew-enabled", "", "Auto-renew enabled (true/false)")
	priceIDs := fs.String("price-id", "", "Offer code price ID(s), comma-separated (required)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc offer-codes create [flags]",
		ShortHelp:  "Create a subscription offer code.",
		LongHelp: `Create a subscription offer code.

Examples:
  asc offer-codes create --subscription-id "SUB_ID" --name "SPRING" --customer-eligibilities NEW --offer-eligibility STACK_WITH_INTRO_OFFERS --duration ONE_MONTH --offer-mode PAY_AS_YOU_GO --number-of-periods 1 --price-id "PRICE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			subscription := strings.TrimSpace(*subscriptionID)
			if subscription == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}

			trimmedName := strings.TrimSpace(*name)
			if trimmedName == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			if strings.TrimSpace(*customerEligibilities) == "" {
				fmt.Fprintln(os.Stderr, "Error: --customer-eligibilities is required")
				return flag.ErrHelp
			}
			customerEligibilityValues, err := normalizeOfferCodeCustomerEligibilities(*customerEligibilities)
			if err != nil {
				return fmt.Errorf("offer-codes create: %w", err)
			}

			if strings.TrimSpace(*offerEligibility) == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-eligibility is required")
				return flag.ErrHelp
			}
			offerEligibilityValue, err := normalizeOfferCodeEligibility(*offerEligibility)
			if err != nil {
				return fmt.Errorf("offer-codes create: %w", err)
			}

			if strings.TrimSpace(*duration) == "" {
				fmt.Fprintln(os.Stderr, "Error: --duration is required")
				return flag.ErrHelp
			}
			durationValue, err := normalizeOfferCodeDuration(*duration)
			if err != nil {
				return fmt.Errorf("offer-codes create: %w", err)
			}

			if strings.TrimSpace(*offerMode) == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-mode is required")
				return flag.ErrHelp
			}
			offerModeValue, err := normalizeOfferCodeMode(*offerMode)
			if err != nil {
				return fmt.Errorf("offer-codes create: %w", err)
			}

			if !numberOfPeriods.set {
				fmt.Fprintln(os.Stderr, "Error: --number-of-periods is required")
				return flag.ErrHelp
			}
			if numberOfPeriods.value <= 0 {
				return fmt.Errorf("offer-codes create: --number-of-periods must be greater than 0")
			}

			prices := parseCommaSeparatedIDs(*priceIDs)
			if len(prices) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --price-id is required")
				return flag.ErrHelp
			}

			autoRenewEnabledValue, err := parseOptionalBoolFlag("--auto-renew-enabled", *autoRenewEnabled)
			if err != nil {
				return fmt.Errorf("offer-codes create: %w", err)
			}

			priceData := make([]asc.ResourceData, 0, len(prices))
			for _, priceID := range prices {
				priceData = append(priceData, asc.ResourceData{
					Type: asc.ResourceTypeSubscriptionOfferCodePrices,
					ID:   priceID,
				})
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("offer-codes create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			req := asc.SubscriptionOfferCodeCreateRequest{
				Data: asc.SubscriptionOfferCodeCreateData{
					Type: asc.ResourceTypeSubscriptionOfferCodes,
					Attributes: asc.SubscriptionOfferCodeCreateAttributes{
						Name:                  trimmedName,
						CustomerEligibilities: customerEligibilityValues,
						OfferEligibility:      offerEligibilityValue,
						Duration:              durationValue,
						OfferMode:             offerModeValue,
						NumberOfPeriods:       numberOfPeriods.value,
						AutoRenewEnabled:      autoRenewEnabledValue,
					},
					Relationships: asc.SubscriptionOfferCodeCreateRelationships{
						Subscription: asc.Relationship{
							Data: asc.ResourceData{
								Type: asc.ResourceTypeSubscriptions,
								ID:   subscription,
							},
						},
						Prices: asc.RelationshipList{Data: priceData},
					},
				},
			}

			resp, err := client.CreateSubscriptionOfferCode(requestCtx, req)
			if err != nil {
				return fmt.Errorf("offer-codes create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// OfferCodesUpdateCommand returns the offer codes update subcommand.
func OfferCodesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Subscription offer code ID (required)")
	active := fs.String("active", "", "Set active (true/false)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc offer-codes update [flags]",
		ShortHelp:  "Update a subscription offer code.",
		LongHelp: `Update a subscription offer code.

Examples:
  asc offer-codes update --offer-code-id "OFFER_CODE_ID" --active true`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*offerCodeID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code-id is required")
				return flag.ErrHelp
			}

			activeValue, err := parseOptionalBoolFlag("--active", *active)
			if err != nil {
				return fmt.Errorf("offer-codes update: %w", err)
			}
			if activeValue == nil {
				fmt.Fprintln(os.Stderr, "Error: --active is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("offer-codes update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateSubscriptionOfferCode(requestCtx, trimmedID, asc.SubscriptionOfferCodeUpdateAttributes{Active: activeValue})
			if err != nil {
				return fmt.Errorf("offer-codes update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

type optionalInt struct {
	set   bool
	value int
}

func (i *optionalInt) Set(value string) error {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("must be an integer")
	}
	i.value = parsed
	i.set = true
	return nil
}

func (i *optionalInt) String() string {
	if !i.set {
		return ""
	}
	return strconv.Itoa(i.value)
}

func normalizeOfferCodeDuration(value string) (asc.SubscriptionOfferDuration, error) {
	normalized := normalizeEnumValue(value)
	if normalized == "" {
		return "", nil
	}
	if duration, ok := offerCodeDurationMap[normalized]; ok {
		return duration, nil
	}
	return "", fmt.Errorf("--duration must be one of: %s", strings.Join(offerCodeDurationValues, ", "))
}

func normalizeOfferCodeMode(value string) (asc.SubscriptionOfferMode, error) {
	normalized := normalizeEnumValue(value)
	if normalized == "" {
		return "", nil
	}
	if mode, ok := offerCodeModeMap[normalized]; ok {
		return mode, nil
	}
	return "", fmt.Errorf("--offer-mode must be one of: %s", strings.Join(offerCodeModeValues, ", "))
}

func normalizeOfferCodeEligibility(value string) (asc.SubscriptionOfferEligibility, error) {
	normalized := normalizeEnumValue(value)
	if normalized == "" {
		return "", nil
	}
	if eligibility, ok := offerCodeEligibilityMap[normalized]; ok {
		return eligibility, nil
	}
	return "", fmt.Errorf("--offer-eligibility must be one of: %s", strings.Join(offerCodeEligibilityValues, ", "))
}

func normalizeOfferCodeCustomerEligibilities(value string) ([]asc.SubscriptionCustomerEligibility, error) {
	values := splitCSV(value)
	if len(values) == 0 {
		return nil, nil
	}

	eligibilities := make([]asc.SubscriptionCustomerEligibility, 0, len(values))
	for _, item := range values {
		normalized := normalizeEnumValue(item)
		if eligibility, ok := offerCodeCustomerEligibilityMap[normalized]; ok {
			eligibilities = append(eligibilities, eligibility)
			continue
		}
		return nil, fmt.Errorf("--customer-eligibilities must be one of: %s", strings.Join(offerCodeCustomerEligibilityValues, ", "))
	}

	return eligibilities, nil
}

func normalizeEnumValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	normalized := strings.ToUpper(trimmed)
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}
