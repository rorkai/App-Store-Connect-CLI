package apps

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppsInfoListCommand returns the list subcommand for apps info.
func newAppsInfoListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("apps info list", flag.ExitOnError)

	appID := shared.BindResourceIDFlag(fs, "app", "apps", "App Store Connect app ID (or ASC_APP_ID env)")
	fields := fs.String("fields", "", "Sparse app info fields: "+strings.Join(shared.AppInfoSparseFields, ", ")+" (deprecated; removed from API 4.5; prefer asc age-rating view)")
	ageRatingFields := fs.String("age-rating-fields", "", "Sparse fields for included age rating declaration: "+strings.Join(shared.AgeRatingDeclarationSparseFields, ", "))
	include := fs.String("include", "", "Include related resources: "+strings.Join(appInfoIncludeList(), ", "))
	output := d.BindOutputFlags(fs)

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
  asc apps info list --app "APP_ID" --fields state,primaryCategory
  asc apps info list --app "APP_ID" --age-rating-fields socialMedia,socialMediaAgeRestricted
  asc apps info list --app "APP_ID" --include "appInfoLocalizations,primaryCategory"
  asc apps info list --app "APP_ID" --output table
  asc apps info list --app "APP_ID" --output markdown`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appSelector := d.ResolveAppSelector(*appID)
			resolvedAppID := appSelector.Value
			if strings.TrimSpace(resolvedAppID) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}
			fieldValues, err := normalizeSparseField(fs, *fields, shared.AppInfoSparseFields, "--fields")
			if err != nil {
				return d.UsageError(err.Error())
			}
			ageRatingFieldValues, err := normalizeSparseField(fs, *ageRatingFields, shared.AgeRatingDeclarationSparseFields, "--age-rating-fields")
			if err != nil {
				return d.UsageError(err.Error())
			}
			includeValues, err := normalizeAppInfoInclude(*include)
			if err != nil {
				return d.UsageError(err.Error())
			}

			d.WarnDeprecatedAppInfoFields(fieldValues, "")

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("apps info list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resolvedAppID, err = shared.ResolveScopedAppID(requestCtx, client, appSelector)
			if err != nil {
				return fmt.Errorf("apps info list: resolve app: %w", err)
			}
			requestCtx = asc.WithNoRedirectReads(requestCtx)

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

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
