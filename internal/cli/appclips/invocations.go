package appclips

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppClipInvocationsCommand returns the invocations command group.
func newAppClipInvocationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("invocations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "invocations",
		ShortUsage: "asc app-clips invocations <subcommand> [flags]",
		ShortHelp:  "Manage beta App Clip invocations.",
		LongHelp: `Manage beta App Clip invocations.

Examples:
  asc app-clips invocations list --build-bundle-id "BUILD_BUNDLE_ID"
  asc app-clips invocations create --build-bundle-id "BUILD_BUNDLE_ID" --url "https://example.com/clip" --locale "en-US" --title "Try it"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppClipInvocationsListCommand(d),
			newAppClipInvocationsGetCommand(d),
			newAppClipInvocationsCreateCommand(d),
			newAppClipInvocationsUpdateCommand(d),
			newAppClipInvocationsDeleteCommand(d),
			newAppClipInvocationLocalizationsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppClipInvocationsListCommand lists beta App Clip invocations for a build bundle.
func newAppClipInvocationsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	buildBundleID := fs.String("build-bundle-id", "", "Build bundle ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-clips invocations list --build-bundle-id \"BUILD_BUNDLE_ID\" [flags]",
		ShortHelp:  "List beta App Clip invocations for a build bundle.",
		LongHelp: `List beta App Clip invocations for a build bundle.

Examples:
  asc app-clips invocations list --build-bundle-id "BUILD_BUNDLE_ID"
  asc app-clips invocations list --build-bundle-id "BUILD_BUNDLE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "app-clips invocations list", 200); err != nil {
				return err
			}

			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--build-bundle-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips invocations list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BetaAppClipInvocationsOption{
				asc.Limit[asc.BetaAppClipInvocationsOption](*limit),
				asc.NextURL[asc.BetaAppClipInvocationsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.BetaAppClipInvocationsOption](200))
				firstPage, err := client.GetBuildBundleBetaAppClipInvocations(requestCtx, buildBundleValue, paginateOpts...)
				if err != nil {
					if asc.IsNotFound(err) {
						fmt.Fprintln(d.IO.ErrOut, "No invocations found.")
						empty := &asc.BetaAppClipInvocationsResponse{Data: []asc.Resource[asc.BetaAppClipInvocationAttributes]{}}
						return d.PrintOutput(empty, *output.Output, *output.Pretty)
					}
					return fmt.Errorf("app-clips invocations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBuildBundleBetaAppClipInvocations(ctx, buildBundleValue, asc.NextURL[asc.BetaAppClipInvocationsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips invocations list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBuildBundleBetaAppClipInvocations(requestCtx, buildBundleValue, opts...)
			if err != nil {
				if asc.IsNotFound(err) {
					fmt.Fprintln(d.IO.ErrOut, "No invocations found.")
					empty := &asc.BetaAppClipInvocationsResponse{Data: []asc.Resource[asc.BetaAppClipInvocationAttributes]{}}
					return d.PrintOutput(empty, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips invocations list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipInvocationsGetCommand gets a beta App Clip invocation by ID.
func newAppClipInvocationsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc app-clips invocations view --invocation-id \"INVOCATION_ID\"",
		ShortHelp:  "View a beta App Clip invocation by ID.",
		LongHelp: `View a beta App Clip invocation by ID.

Examples:
  asc app-clips invocations view --invocation-id "INVOCATION_ID"`,
		IDFlag:             "invocation-id",
		IDUsage:            "Invocation ID",
		IDType:             "betaAppClipInvocations",
		RequiredDiagnostic: true,
		Deps:               d,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBetaAppClipInvocation(ctx, id)
		},
	})
}

// newAppClipInvocationsCreateCommand creates a beta App Clip invocation.
func newAppClipInvocationsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	buildBundleID := fs.String("build-bundle-id", "", "Build bundle ID")
	url := fs.String("url", "", "Invocation URL")
	localizationIDs := shared.BindOnceCSVFlag(fs, "localization-id", "Existing localization ID(s), comma-separated")
	locale := fs.String("locale", "", "Inline localization locale (use with --title)")
	title := fs.String("title", "", "Inline localization title (use with --locale)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-clips invocations create --build-bundle-id \"BUILD_BUNDLE_ID\" --url \"https://example.com/clip\" (--localization-id \"LOC_ID\" | --locale \"en-US\" --title \"Try it\") [flags]",
		ShortHelp:  "Create a beta App Clip invocation.",
		LongHelp: `Create a beta App Clip invocation.

Provide either pre-existing localization IDs with ` + "`--localization-id`" + ` or an inline localization with ` + "`--locale`" + ` and ` + "`--title`" + `.

Examples:
  asc app-clips invocations create --build-bundle-id "BUILD_BUNDLE_ID" --url "https://example.com/clip" --locale "en-US" --title "Try it"
  asc app-clips invocations create --build-bundle-id "BUILD_BUNDLE_ID" --url "https://example.com/clip" --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" {
				return d.RequiredFlag("--build-bundle-id")
			}

			urlValue := strings.TrimSpace(*url)
			if urlValue == "" {
				return d.RequiredFlag("--url")
			}

			localizationValues := shared.SplitCSV(localizationIDs.String())
			localeValue := strings.TrimSpace(*locale)
			titleValue := strings.TrimSpace(*title)
			if titleValue != "" && localeValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --locale is required when --title is set")
				return shared.MissingRequiredUsageError("--locale")
			}
			if localeValue != "" && titleValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --title is required when --locale is set")
				return shared.MissingRequiredUsageError("--title")
			}

			inlineLocalizations := make([]asc.BetaAppClipInvocationLocalizationCreateAttributes, 0, 1)
			if localeValue != "" && titleValue != "" {
				inlineLocalizations = append(inlineLocalizations, asc.BetaAppClipInvocationLocalizationCreateAttributes{
					Locale: localeValue,
					Title:  titleValue,
				})
			}
			if len(localizationValues) == 0 && len(inlineLocalizations) == 0 {
				fmt.Fprintln(d.IO.ErrOut, "Error: provide --localization-id or both --locale and --title")
				return flag.ErrHelp
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips invocations create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.BetaAppClipInvocationCreateAttributes{URL: urlValue}
			resp, err := client.CreateBetaAppClipInvocation(requestCtx, buildBundleValue, attrs, localizationValues, inlineLocalizations)
			if err != nil {
				return fmt.Errorf("app-clips invocations create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipInvocationsUpdateCommand updates a beta App Clip invocation.
func newAppClipInvocationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.BetaAppClipInvocationUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc app-clips invocations update --invocation-id \"INVOCATION_ID\" [flags]",
		ShortHelp:  "Update a beta App Clip invocation.",
		LongHelp: `Update a beta App Clip invocation.

Examples:
  asc app-clips invocations update --invocation-id "INVOCATION_ID" --url "https://example.com/clip"`,
		IDFlag:  "invocation-id",
		IDUsage: "Invocation ID",
		IDType:  "betaAppClipInvocations",
		Fields: []shared.AttributeFlag{
			shared.ExplicitStringFlag(&attrs.URL, "url", "Invocation URL"),
		},
		Deps: d,
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateBetaAppClipInvocation(ctx, id, &attrs)
		},
	})
}

// newAppClipInvocationsDeleteCommand deletes a beta App Clip invocation.
func newAppClipInvocationsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc app-clips invocations delete --invocation-id \"INVOCATION_ID\" --confirm",
		ShortHelp:  "Delete a beta App Clip invocation.",
		LongHelp: `Delete a beta App Clip invocation.

Examples:
  asc app-clips invocations delete --invocation-id "INVOCATION_ID" --confirm`,
		IDFlag:             "invocation-id",
		IDUsage:            "Invocation ID",
		IDType:             "betaAppClipInvocations",
		RequiredDiagnostic: true,
		Deps:               d,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteBetaAppClipInvocation(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}
