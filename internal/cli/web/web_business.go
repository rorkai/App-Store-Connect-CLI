package web

import (
	"context"
	"flag"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// WebBusinessCommand returns App Store Connect Business web-session workflows.
func WebBusinessCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:        "business",
		ShortUsage:  "asc web business <subcommand> [flags]",
		ShortHelp:   "Manage App Store Connect Business agreements.",
		FlagSet:     flag.NewFlagSet("web business", flag.ExitOnError),
		UsageFunc:   shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{WebBusinessAgreementsCommand()},
		Exec:        func(context.Context, []string) error { return flag.ErrHelp },
	}
}

// WebBusinessAgreementsCommand returns Business contract workflows, separate
// from Apple Developer Program licenses under web agreements.
func WebBusinessAgreementsCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:        "agreements",
		ShortUsage:  "asc web business agreements <subcommand> [flags]",
		ShortHelp:   "Inspect and accept App Store Connect Business agreements.",
		FlagSet:     flag.NewFlagSet("web business agreements", flag.ExitOnError),
		UsageFunc:   shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{WebBusinessAgreementsListCommand(), WebBusinessAgreementsAcceptCommand()},
		Exec:        func(context.Context, []string) error { return flag.ErrHelp },
	}
}

// WebBusinessAgreementsAcceptCommand accepts one explicitly selected update.
func WebBusinessAgreementsAcceptCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web business agreements accept", flag.ExitOnError)
	configID := fs.String("contract-config-id", "", "Outstanding configuration ID from availableNewContractConfigId in agreements list")
	confirm := fs.Bool("confirm", false, "Confirm agreeing to the terms and the Business form's Family Sharing consent")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "accept",
		ShortUsage: "asc web business agreements accept --contract-config-id ID --confirm [flags]",
		ShortHelp:  "Accept one updated App Store Connect Business agreement.",
		LongHelp: `Review the updated terms in App Store Connect Business before accepting.
Use availableNewContractConfigId from 'asc web business agreements list'. The
command resolves the unique assigned legal entity and uses the first available
locale, falling back to en-US, as the Business web form does.

--confirm acknowledges both agreeToTerms and agreeToFamilyShare. The request
sends both as true, matching the Business form even when its Family Sharing
checkbox is hidden. Unsupported consent requirements stop before the write.

The command submits once and does not follow redirects or automatically retry
an ambiguous failure. Inspect agreements list before retrying an unknown result.
After the request, a fresh read must show the selected configuration current,
latest, and in effect for the same legal entity, with no replacement pending.
An inconclusive or failed read returns an error without a success receipt.

If Apple requires fresh 2FA, clear only that account's cached session, then
sign in again for the same account and provider and complete 2FA:
  asc web auth logout --apple-id EMAIL
  asc web auth login --apple-id EMAIL --public-provider-id PROVIDER_ID
For non-interactive login, configure --two-factor-code-command or
ASC_WEB_2FA_CODE_COMMAND.

Example:
  asc web business agreements accept --contract-config-id CONFIG_ID --confirm
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*configID)
			if id == "" {
				return shared.UsageError("--contract-config-id is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web business agreements accept")
			}
			result, err := newWebClientFn(session).AcceptBusinessAgreement(requestCtx, id)
			if err != nil {
				return withWebAuthHint(err, "web business agreements accept")
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// WebBusinessAgreementsListCommand lists contracts without losing Apple's envelope.
func WebBusinessAgreementsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web business agreements list", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web business agreements list [flags]",
		ShortHelp:  "List Business agreements and available updated terms.",
		LongHelp: `List the selected App Store Connect provider's Business agreements,
including Paid Applications. JSON preserves Apple's complete response envelope.
An agreement can be active while an updated agreement awaits acceptance.
availableNewContractConfigId identifies that update; configId and configVersion
describe the current agreement.

Example:
  asc web business agreements list --output json
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web business agreements list")
			}
			response, err := newWebClientFn(session).ListBusinessAgreements(requestCtx)
			if err != nil {
				return withWebAuthHint(err, "web business agreements list")
			}
			rows := make([][]string, 0, len(response.Agreements))
			for _, agreement := range response.Agreements {
				var version, outstandingID string
				if agreement.ConfigVersion != nil {
					version = strconv.Itoa(*agreement.ConfigVersion)
				}
				if agreement.AvailableNewContractConfigID != nil {
					outstandingID = *agreement.AvailableNewContractConfigID
				}
				rows = append(rows, []string{agreement.ContractID, agreement.ContractDisplayName, agreement.Status, version, outstandingID})
			}
			return shared.PrintOutputRows(response.Raw, *output.Output, *output.Pretty,
				[]string{"Contract ID", "Agreement", "Status", "Current Version", "New Configuration ID"}, rows)
		},
	}
}
