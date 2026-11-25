package web

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newWebBusinessDSACommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:        "dsa",
		ShortUsage:  "asc web business dsa <subcommand> [flags]",
		ShortHelp:   "Inspect Digital Services Act trader status.",
		FlagSet:     d.NewFlagSet("web business dsa", flag.ExitOnError),
		UsageFunc:   d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{newWebBusinessDSAStatusCommand(d)},
		Exec:        func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func newWebBusinessDSAStatusCommand(d *shared.Deps) *ffcli.Command {
	const command = "web business dsa status"
	fs := d.NewFlagSet(command, flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc web business dsa status [flags]",
		ShortHelp:  "Read the selected account's DSA trader and verification status.",
		LongHelp: `Read the selected web session account's Digital Services Act status.
The summary contains traderDeclaration and each seller's verificationStatus
and isEuCovered. Status strings are reported as Apple returns them.

Example:
  asc web business dsa status --output json
`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return d.UsageError("unexpected positional arguments")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			accountID, client, requestCtx, cancel, err := resolveWebComplianceClient(ctx, d, authFlags, command)
			defer cancel()
			if err != nil {
				return err
			}
			result, err := client.GetDSAStatus(requestCtx, accountID)
			if err != nil {
				return withWebAuthHint(err, command)
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
