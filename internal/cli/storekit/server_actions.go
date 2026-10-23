package storekit

import (
	"context"
	"encoding/json"
	"flag"
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
			if err := flags.validate(args, requiredFlag{"--original-transaction-id", *id}, requiredFlag{"--app-account-token", *token}); err != nil {
				return err
			}
			accountToken, err := parseUUIDFlag("--app-account-token", *token)
			if err != nil {
				return err
			}
			_, err = serverRequest(ctx, fs.Name(), flags.common, func(ctx context.Context, c *storekitapi.Client) (json.RawMessage, error) {
				return nil, c.SetAppAccountToken(ctx, *id, accountToken)
			})
			if err != nil {
				return err
			}
			return shared.PrintOutput(&asc.StoreKitAccountTokenResult{OriginalTransactionID: strings.TrimSpace(*id), AppAccountToken: accountToken, Updated: true}, *flags.output.Output, *flags.output.Pretty)
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

func (f serverWriteFlags) validate(args []string, required ...requiredFlag) error {
	if err := rejectUnexpectedArgs(args); err != nil {
		return err
	}
	for _, item := range required {
		if err := requireFlag(item.name, item.value); err != nil {
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
