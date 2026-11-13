package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var getWebAppDistributionFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppDistribution, error) {
	return client.GetAppDistribution(ctx, appID)
}

var setWebAppDistributionFn = func(ctx context.Context, client *webcore.Client, request webcore.AppDistributionSetRequest) (*asc.WebAppDistributionSetResult, error) {
	return client.SetAppDistribution(ctx, request)
}

// WebAppsDistributionCommand returns the web app distribution method command group.
func WebAppsDistributionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps distribution", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "distribution",
		ShortUsage: "asc web apps distribution <subcommand> [flags]",
		ShortHelp:  "Inspect or update the app distribution method via web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Read the app-level distribution method that App Store Connect shows under
Distribution -> App Availability. The public App Store Connect API does not
expose this setting, so it is only reachable through a web session.

The view subcommand is read-only. The set subcommand changes the app-level
public or private distribution method after an explicit confirmation and
verifies the resulting Apple attributes. Unlisted/direct URL distribution is a
separate Apple flow and is not inferred or changed by this command.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppsDistributionViewCommand(),
			WebAppsDistributionSetCommand(),
			WebAppsDistributionUsersCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAppsDistributionSetCommand updates the app-level distribution method.
func WebAppsDistributionSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps distribution set", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	method := fs.String("method", "", "Distribution method: public or private")
	educationDiscount := fs.String("education-discount", "", "Education discount for public distribution: discounted or not-discounted")
	confirm := fs.Bool("confirm", false, "Confirm changing the app distribution method")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc web apps distribution set --app APP_ID --method public|private [--education-discount discounted|not-discounted] --confirm [flags]",
		ShortHelp:  "Set and verify the app distribution method.",
		LongHelp: `Set the app-level distribution method returned by Apple's internal app
resource. public maps to APP_STORE and private maps to CUSTOM. For public
distribution, --education-discount is optional when Apple's current value is
DISCOUNTED or NOT_DISCOUNTED; omission preserves that current value. Private
distribution always sends NOT_APPLICABLE. Existing custom organization and user
rows are preserved and are not changed by this command.

DIRECT_URL/unlisted distribution is a separate read-only flow here and is
never inferred or changed by this command. A successful update is verified by
a follow-up read within the command context. If Apple's PATCH result is
ambiguous or the command context expires, the command prints an uncertain
receipt and returns an error; do not retry until provider state is checked.

Examples:
  asc web apps distribution set --app 6759231657 --method public --education-discount not-discounted --confirm
  asc web apps distribution set --app 6759231657 --method private --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			distributionType, err := webAppDistributionTypeFromMethod(*method)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			educationDiscountType, err := webAppEducationDiscountType(*educationDiscount, distributionType)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web apps distribution set")
			}

			var result *asc.WebAppDistributionSetResult
			err = withWebSpinner("Updating app distribution method", func() error {
				result, err = setWebAppDistributionFn(requestCtx, newWebClientFn(session), webcore.AppDistributionSetRequest{
					AppID:                 resolvedAppID,
					DistributionType:      distributionType,
					EducationDiscountType: educationDiscountType,
				})
				return err
			})

			if result != nil {
				printErr := printWebAppDistributionSet(result, *output.Output, *output.Pretty)
				if printErr != nil {
					if err != nil {
						return errors.Join(withWebAuthHint(err, "web apps distribution set"), printErr)
					}
					return printErr
				}
			}
			if err != nil {
				return withWebAuthHint(err, "web apps distribution set")
			}
			if result == nil {
				return fmt.Errorf("web apps distribution set failed: missing set result")
			}
			return nil
		},
	}
}

func webAppDistributionTypeFromMethod(method string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "public":
		return webcore.AppDistributionTypeAppStore, nil
	case "private":
		return webcore.AppDistributionTypeCustom, nil
	default:
		return "", fmt.Errorf("--method must be public or private")
	}
}

func webAppEducationDiscountType(value, distributionType string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if distributionType == webcore.AppDistributionTypeCustom {
		if value != "" {
			return "", fmt.Errorf("--education-discount cannot be used with --method private")
		}
		return "", nil
	}
	switch value {
	case "":
		return "", nil
	case "discounted":
		return webcore.AppDistributionEducationDiscounted, nil
	case "not-discounted":
		return webcore.AppDistributionEducationNotDiscounted, nil
	default:
		return "", fmt.Errorf("--education-discount must be discounted or not-discounted")
	}
}

// WebAppsDistributionViewCommand returns the distribution method view command.
func WebAppsDistributionViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps distribution view", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc web apps distribution view --app APP_ID [flags]",
		ShortHelp:  "View the app distribution method.",
		LongHelp: `WEB SESSION WORKFLOWS

View the app-level distribution method attributes returned by Apple's internal
app resource. Values are printed exactly as Apple returns them; APP_STORE is
public App Store distribution and CUSTOM is private distribution through Apple
Business Manager or Apple School Manager. Attributes Apple omits are reported as
"unknown" in table output.

Examples:
  asc web apps distribution view --app 6759231657
  asc web apps distribution view --app 6759231657 --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			var result *webcore.AppDistribution
			err = withWebSpinner("Fetching app distribution method", func() error {
				var err error
				result, err = getWebAppDistributionFn(requestCtx, newWebClientFn(session), resolvedAppID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps distribution view")
			}

			return printWebAppDistribution(result, *output.Output, *output.Pretty)
		},
	}
}

func printWebAppDistribution(result *webcore.AppDistribution, output string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		output,
		pretty,
		func() error {
			asc.RenderTable([]string{"field", "value"}, webAppDistributionRows(result))
			return nil
		},
		func() error {
			asc.RenderMarkdown([]string{"field", "value"}, webAppDistributionRows(result))
			return nil
		},
	)
}

func printWebAppDistributionSet(result *asc.WebAppDistributionSetResult, output string, pretty bool) error {
	return shared.PrintOutput(result, output, pretty)
}

func webAppDistributionRows(result *webcore.AppDistribution) [][]string {
	if result == nil {
		return nil
	}
	return [][]string{
		{"app_id", result.AppID},
		{"name", webAppValueOrUnknown(result.Name)},
		{"bundle_id", webAppValueOrUnknown(result.BundleID)},
		{"distribution_type", webAppValueOrUnknown(result.DistributionType)},
		{"education_discount_type", webAppValueOrUnknown(result.EducationDiscountType)},
	}
}
