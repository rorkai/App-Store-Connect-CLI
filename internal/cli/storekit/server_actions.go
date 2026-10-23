package storekit

import (
	"context"
	"encoding/json"
	"flag"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	storekitapi "github.com/rudrankriyam/App-Store-Connect-CLI/internal/storekit"
)

// AppTransactionsCommand reads purchases of the app itself, separate from IAPs.
func AppTransactionsCommand() *ffcli.Command {
	return groupCommand("app-transactions", "Read signed app-purchase information.",
		`Read a customer's purchase of the app itself with the App Store Server API.

Examples:
  asc storekit app-transactions view --transaction-id 2000000000000001 --environment sandbox`, appTransactionViewCommand())
}

func appTransactionViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit app-transactions view", flag.ExitOnError)
	id := fs.String("transaction-id", "", "Any transaction, original transaction, or app transaction ID for the customer")
	flags := bindServerReadFlags(fs, true)
	return documentedLeafCommand("view", "asc storekit app-transactions view --transaction-id ID --environment ENV [flags]", "Get a customer's app transaction information.",
		"Get signed app-purchase information, including the original app version and purchase date.\n\n"+unverifiedJWSHelp+`

Examples:
  asc storekit app-transactions view --transaction-id 2000000000000001 --environment sandbox
  asc storekit app-transactions view --transaction-id 2000000000000001 --environment production --decode --output json`, fs, func(ctx context.Context, args []string) error {
			if err := validateServerRead(args, flags, requiredFlag{"--transaction-id", *id}); err != nil {
				return err
			}
			raw, err := serverRequest(ctx, fs.Name(), flags.common, func(ctx context.Context, c *storekitapi.Client) (json.RawMessage, error) {
				return c.GetAppTransactionInfo(ctx, *id)
			})
			if err != nil {
				return err
			}
			return printServerResponse(raw, flags, appTransactionInfoTable)
		})
}

func setAccountTokenCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit transactions set-account-token", flag.ExitOnError)
	id := fs.String("original-transaction-id", "", "Original transaction ID whose account association is replaced")
	token := fs.String("app-account-token", "", "Customer account UUID to associate with the purchase")
	flags := bindServerWriteFlags(fs)
	return documentedLeafCommand("set-account-token", "asc storekit transactions set-account-token --original-transaction-id ID --app-account-token UUID --environment ENV --confirm [flags]", "Set or replace a purchase's app account token.",
		`Set an account association for any purchase type. For auto-renewable subscriptions,
Apple updates the current and future renewals, leaving past transactions unchanged.
Family Shared transactions are unsupported. Requires --confirm.

Examples:
  asc storekit transactions set-account-token --original-transaction-id 2000000000000001 --app-account-token 11111111-2222-4333-8444-555555555555 --environment sandbox --confirm`, fs, func(ctx context.Context, args []string) error {
			var accountToken string
			if err := flags.validate(args, func() (err error) {
				accountToken, err = parseUUIDFlag("--app-account-token", *token)
				return err
			}, requiredFlag{"--original-transaction-id", *id}, requiredFlag{"--app-account-token", *token}); err != nil {
				return err
			}
			_, err := serverRequest(ctx, fs.Name(), flags.common, func(ctx context.Context, c *storekitapi.Client) (json.RawMessage, error) {
				return nil, c.SetAppAccountToken(ctx, *id, accountToken)
			})
			if err != nil {
				return err
			}
			return shared.PrintOutput(&asc.StoreKitAccountTokenResult{OriginalTransactionID: strings.TrimSpace(*id), AppAccountToken: accountToken, Updated: true}, *flags.output.Output, *flags.output.Pretty)
		})
}

func finishTransactionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit transactions finish", flag.ExitOnError)
	id := fs.String("transaction-id", "", "Transaction ID to mark as finished after delivering its content")
	flags := bindServerWriteFlags(fs)
	return documentedLeafCommand("finish", "asc storekit transactions finish --transaction-id ID --environment ENV --confirm [flags]", "Mark a transaction as finished after content delivery.",
		`Notify Apple only after your server has delivered the purchased content.
If the app already calls StoreKit Transaction.finish(), this server call is
unnecessary. Requires --confirm. The CLI sends once without automatic retries.

Examples:
  asc storekit transactions finish --transaction-id 2000000000000001 --environment production --confirm`, fs, func(ctx context.Context, args []string) error {
			if err := flags.validate(args, nil, requiredFlag{"--transaction-id", *id}); err != nil {
				return err
			}
			_, err := serverRequest(ctx, fs.Name(), flags.common, func(ctx context.Context, c *storekitapi.Client) (json.RawMessage, error) {
				return nil, c.FinishTransaction(ctx, *id)
			})
			if err != nil {
				return err
			}
			return shared.PrintOutput(&asc.StoreKitFinishResult{TransactionID: strings.TrimSpace(*id), Finished: true}, *flags.output.Output, *flags.output.Pretty)
		})
}

func sendConsumptionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("storekit transactions send-consumption", flag.ExitOnError)
	id := fs.String("transaction-id", "", "Transaction ID from a CONSUMPTION_REQUEST notification")
	var consent, sample shared.OptionalBool
	fs.Var(&consent, "customer-consented", "Required: explicitly true when the customer consented to share consumption data")
	fs.Var(&sample, "sample-content-provided", "Required: true or false, whether a free sample, trial, or functionality information was provided")
	delivery := fs.String("delivery-status", "", "Required: DELIVERED, UNDELIVERED_QUALITY_ISSUE, UNDELIVERED_WRONG_ITEM, UNDELIVERED_SERVER_OUTAGE, UNDELIVERED_OTHER")
	percentage := fs.Int("consumption-percentage", 0, "Consumed portion in integer milliunits (0-100000); omit for auto-renewable subscriptions")
	refund := fs.String("refund-preference", "", "Optional: DECLINE, GRANT_FULL, GRANT_PRORATED")
	flags := bindServerWriteFlags(fs)
	return documentedLeafCommand("send-consumption", "asc storekit transactions send-consumption --transaction-id ID --customer-consented true --delivery-status STATUS --sample-content-provided BOOL --environment ENV --confirm [flags]", "Send v2 consumption information for a refund request.",
		`Respond to a CONSUMPTION_REQUEST notification within 12 hours. Send data only
with the customer's consent. Apple makes the refund decision; acceptance of this
request does not grant or deny a refund.

Omit --consumption-percentage for auto-renewable subscriptions: Apple calculates
it. For other product types, 50000 means 50% consumed. GRANT_PRORATED requires
a percentage strictly between 0 and 100000 for those types. Undelivered purchases
require --consumption-percentage 0 for other product types; omit it for
auto-renewable subscriptions. Requires --confirm.

Examples:
  asc storekit transactions send-consumption --transaction-id 2000000000000001 --customer-consented true --delivery-status DELIVERED --sample-content-provided false --consumption-percentage 50000 --environment production --confirm`, fs, func(ctx context.Context, args []string) error {
			var request storekitapi.ConsumptionRequest
			if err := flags.validate(args, func() error {
				if !consent.IsSet() || !consent.Value() {
					return shared.UsageError("--customer-consented must be explicitly true")
				}
				if !sample.IsSet() {
					return shared.UsageError("--sample-content-provided is required")
				}
				request = storekitapi.ConsumptionRequest{CustomerConsented: true, SampleContentProvided: sample.Value(), DeliveryStatus: storekitapi.DeliveryStatus(strings.ToUpper(strings.TrimSpace(*delivery))), RefundPreference: storekitapi.RefundPreference(strings.ToUpper(strings.TrimSpace(*refund)))}
				deliveryValues := []string{"DELIVERED", "UNDELIVERED_QUALITY_ISSUE", "UNDELIVERED_WRONG_ITEM", "UNDELIVERED_SERVER_OUTAGE", "UNDELIVERED_OTHER"}
				if !slices.Contains(deliveryValues, string(request.DeliveryStatus)) {
					return shared.UsageError("--delivery-status must be one of: " + strings.Join(deliveryValues, ", "))
				}
				if flagWasSet(fs, "refund-preference") && !slices.Contains([]string{"DECLINE", "GRANT_FULL", "GRANT_PRORATED"}, string(request.RefundPreference)) {
					return shared.UsageError("--refund-preference must be one of: DECLINE, GRANT_FULL, GRANT_PRORATED")
				}
				if flagWasSet(fs, "consumption-percentage") {
					if *percentage < 0 || *percentage > 100000 {
						return shared.UsageError("--consumption-percentage must be between 0 and 100000")
					}
					request.ConsumptionPercentage = percentage
				}
				if request.DeliveryStatus != "DELIVERED" && request.ConsumptionPercentage != nil && *request.ConsumptionPercentage != 0 {
					return shared.UsageError("--consumption-percentage must be 0 when delivery status is not DELIVERED")
				}
				if request.RefundPreference == "GRANT_PRORATED" && request.ConsumptionPercentage != nil && (*percentage == 0 || *percentage == 100000) {
					return shared.UsageError("--consumption-percentage must be between 1 and 99999 for GRANT_PRORATED")
				}
				return nil
			}, requiredFlag{"--transaction-id", *id}, requiredFlag{"--delivery-status", *delivery}); err != nil {
				return err
			}
			_, err := serverRequest(ctx, fs.Name(), flags.common, func(ctx context.Context, c *storekitapi.Client) (json.RawMessage, error) {
				return nil, c.SendConsumptionInformation(ctx, *id, request)
			})
			if err != nil {
				return err
			}
			return shared.PrintOutput(&asc.StoreKitConsumptionResult{TransactionID: strings.TrimSpace(*id), Submitted: true}, *flags.output.Output, *flags.output.Pretty)
		})
}

type serverWriteFlags struct {
	common  commonFlags
	output  shared.OutputFlags
	confirm *bool
}

func bindServerWriteFlags(fs *flag.FlagSet) serverWriteFlags {
	return serverWriteFlags{common: bindCommonFlags(fs), output: shared.BindOutputFlags(fs), confirm: fs.Bool("confirm", false, "Confirm this mutation (required)")}
}

func (f serverWriteFlags) validate(args []string, check func() error, required ...requiredFlag) error {
	if err := rejectUnexpectedArgs(args); err != nil {
		return err
	}
	for _, item := range required {
		if err := requireFlag(item.name, item.value); err != nil {
			return err
		}
	}
	if check != nil {
		if err := check(); err != nil {
			return err
		}
	}
	if !*f.confirm {
		return shared.UsageError("--confirm is required")
	}
	return nil
}

func appTransactionInfoTable(raw json.RawMessage) ([]string, [][]string, error) {
	var response struct {
		SignedAppTransactionInfo string `json:"signedAppTransactionInfo"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, nil, err
	}
	var claims struct {
		AppTransactionID           string `json:"appTransactionId"`
		BundleID                   string `json:"bundleId"`
		OriginalApplicationVersion string `json:"originalApplicationVersion"`
		OriginalPurchaseDate       int64  `json:"originalPurchaseDate"`
		ReceiptType                string `json:"receiptType"`
	}
	if err := decodeClaims(response.SignedAppTransactionInfo, &claims); err != nil {
		return nil, nil, err
	}
	return []string{"App Transaction ID", "Bundle ID", "Original App Version", "Original Purchase Date", "Receipt Type"}, [][]string{{claims.AppTransactionID, claims.BundleID, claims.OriginalApplicationVersion, formatMillis(claims.OriginalPurchaseDate), claims.ReceiptType}}, nil
}
