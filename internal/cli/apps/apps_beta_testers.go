package apps

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppsRemoveBetaTestersCommand returns the apps remove-beta-testers subcommand.
func AppsRemoveBetaTestersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("remove-beta-testers", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	testers := fs.String("tester", "", "Comma-separated beta tester IDs")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "remove-beta-testers",
		ShortUsage: "asc apps remove-beta-testers --app \"APP_ID\" --tester \"TESTER_ID[,TESTER_ID...]\" --confirm",
		ShortHelp:  "Remove beta testers from an app.",
		LongHelp: `Remove beta testers from an app.

Examples:
  asc apps remove-beta-testers --app "APP_ID" --tester "TESTER_ID" --confirm
  asc apps remove-beta-testers --app "APP_ID" --tester "TESTER_ID1,TESTER_ID2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			testerIDs := shared.SplitCSV(*testers)
			if len(testerIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --tester is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps remove-beta-testers: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.RemoveBetaTestersFromApp(requestCtx, resolvedAppID, testerIDs); err != nil {
				return fmt.Errorf("apps remove-beta-testers: failed to remove testers: %w", err)
			}

			result := &asc.AppBetaTestersUpdateResult{
				AppID:     resolvedAppID,
				TesterIDs: testerIDs,
				Action:    "removed",
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
