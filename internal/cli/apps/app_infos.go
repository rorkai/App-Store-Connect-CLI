package apps

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppsInfoListCommand returns the list subcommand for apps info.
func AppsInfoListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps info list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	fields := fs.String("fields", "", "Sparse app info fields: kidsAgeBand")
	ageRatingFields := fs.String("age-rating-fields", "", "Sparse fields for included age rating declaration: socialMedia, socialMediaAgeRestricted")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc apps info list [flags]",
		ShortHelp:  "List all app info records for an app.",
		LongHelp: `List all app info records for an app.

An app can have multiple app info records (one per platform or state). Use this
command to find the specific app info ID when you encounter "multiple app infos
found" errors in other commands.

Examples:
  asc apps info list --app "APP_ID"
  asc apps info list --app "APP_ID" --fields kidsAgeBand
  asc apps info list --app "APP_ID" --age-rating-fields socialMedia,socialMediaAgeRestricted
  asc apps info list --app "APP_ID" --output table
  asc apps info list --app "APP_ID" --output markdown`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if strings.TrimSpace(resolvedAppID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}
			fieldValues, err := normalizeSparseField(*fields, appInfoSparseFields441, "--fields")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			ageRatingFieldValues, err := normalizeSparseField(*ageRatingFields, ageRatingSparseFields441, "--age-rating-fields")
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps info list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			includeValues := []string{}
			if len(ageRatingFieldValues) > 0 {
				includeValues = addInclude(includeValues, "ageRatingDeclaration")
			}
			resp, err := client.GetAppInfos(
				requestCtx, resolvedAppID,
				asc.WithAppInfoFields(fieldValues),
				asc.WithAppInfoAgeRatingDeclarationFields(ageRatingFieldValues),
				asc.WithAppInfoInclude(includeValues),
			)
			if err != nil {
				return fmt.Errorf("apps info list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
