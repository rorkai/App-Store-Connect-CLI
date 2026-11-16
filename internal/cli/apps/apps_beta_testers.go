package apps

import (
	"context"
	"flag"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppsRemoveBetaTestersCommand returns the apps remove-beta-testers subcommand.
func newAppsRemoveBetaTestersCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("remove-beta-testers", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	testers := shared.BindOnceCSVFlag(fs, "tester", "Comma-separated beta tester IDs")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove-beta-testers",
		ShortUsage: "asc apps remove-beta-testers --app \"APP_ID\" --tester \"TESTER_ID[,TESTER_ID...]\" --confirm",
		ShortHelp:  "Remove beta testers from an app.",
		LongHelp: `Remove beta testers from an app.

Examples:
  asc apps remove-beta-testers --app "APP_ID" --tester "TESTER_ID" --confirm
  asc apps remove-beta-testers --app "APP_ID" --tester "TESTER_ID1,TESTER_ID2" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			testerIDs := shared.SplitCSV(testers.String())
			if len(testerIDs) == 0 {
				return d.RequiredFlag("--tester")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("apps remove-beta-testers: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.RemoveBetaTestersFromApp(requestCtx, resolvedAppID, testerIDs); err != nil {
				return fmt.Errorf("apps remove-beta-testers: failed to remove testers: %w", err)
			}

			result := &asc.AppBetaTestersUpdateResult{
				AppID:     resolvedAppID,
				TesterIDs: testerIDs,
				Action:    "removed",
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
