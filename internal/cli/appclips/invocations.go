package appclips

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

var appClipsClientFactory = shared.GetASCClient

// AppClipInvocationsCommand returns the invocations command group.
func AppClipInvocationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("invocations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "invocations",
		ShortUsage: "asc app-clips invocations <subcommand> [flags]",
		ShortHelp:  "Manage beta App Clip invocations.",
		LongHelp: `Manage beta App Clip invocations.

Examples:
  asc app-clips invocations list --build-bundle-id "BUILD_BUNDLE_ID"
  asc app-clips invocations create --build-bundle-id "BUILD_BUNDLE_ID" --url "https://example.com/clip" --locale "en-US" --title "Try it"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipInvocationsListCommand(),
			AppClipInvocationsGetCommand(),
			AppClipInvocationsCreateCommand(),
			AppClipInvocationsUpdateCommand(),
			AppClipInvocationsDeleteCommand(),
			AppClipInvocationLocalizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipInvocationsListCommand lists beta App Clip invocations for a build bundle.
func AppClipInvocationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	buildBundleID := fs.String("build-bundle-id", "", "Build bundle ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-clips invocations list --build-bundle-id \"BUILD_BUNDLE_ID\" [flags]",
		ShortHelp:  "List beta App Clip invocations for a build bundle.",
		LongHelp: `List beta App Clip invocations for a build bundle.

Examples:
  asc app-clips invocations list --build-bundle-id "BUILD_BUNDLE_ID"
  asc app-clips invocations list --build-bundle-id "BUILD_BUNDLE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.ValidatePaginationFlags(ctx, fs, "app-clips invocations list", 200); err != nil {
				return err
			}

			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--build-bundle-id")
			}

			client, err := appClipsClientFactory()
			if err != nil {
				return fmt.Errorf("app-clips invocations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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
						fmt.Fprintln(os.Stderr, "No invocations found.")
						empty := &asc.BetaAppClipInvocationsResponse{Data: []asc.Resource[asc.BetaAppClipInvocationAttributes]{}}
						return shared.PrintOutput(empty, *output.Output, *output.Pretty)
					}
					return fmt.Errorf("app-clips invocations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBuildBundleBetaAppClipInvocations(ctx, buildBundleValue, asc.NextURL[asc.BetaAppClipInvocationsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips invocations list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBuildBundleBetaAppClipInvocations(requestCtx, buildBundleValue, opts...)
			if err != nil {
				if asc.IsNotFound(err) {
					fmt.Fprintln(os.Stderr, "No invocations found.")
					empty := &asc.BetaAppClipInvocationsResponse{Data: []asc.Resource[asc.BetaAppClipInvocationAttributes]{}}
					return shared.PrintOutput(empty, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips invocations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipInvocationsGetCommand gets a beta App Clip invocation by ID.
func AppClipInvocationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	invocationID := shared.BindResourceIDFlag(fs, "invocation-id", "betaAppClipInvocations", "Invocation ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc app-clips invocations view --invocation-id \"INVOCATION_ID\"",
		ShortHelp:  "View a beta App Clip invocation by ID.",
		LongHelp: `View a beta App Clip invocation by ID.

Examples:
  asc app-clips invocations view --invocation-id "INVOCATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			invocationValue := strings.TrimSpace(*invocationID)
			if invocationValue == "" {
				return shared.RequiredFlag("--invocation-id")
			}

			client, err := appClipsClientFactory()
			if err != nil {
				return fmt.Errorf("app-clips invocations view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaAppClipInvocation(requestCtx, invocationValue)
			if err != nil {
				return fmt.Errorf("app-clips invocations view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipInvocationsCreateCommand creates a beta App Clip invocation.
func AppClipInvocationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	buildBundleID := fs.String("build-bundle-id", "", "Build bundle ID")
	url := fs.String("url", "", "Invocation URL")
	localizationIDs := shared.BindOnceCSVFlag(fs, "localization-id", "Existing localization ID(s), comma-separated")
	locale := fs.String("locale", "", "Inline localization locale (use with --title)")
	title := fs.String("title", "", "Inline localization title (use with --locale)")
	output := shared.BindOutputFlags(fs)

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
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" {
				return shared.RequiredFlag("--build-bundle-id")
			}

			urlValue := strings.TrimSpace(*url)
			if urlValue == "" {
				return shared.RequiredFlag("--url")
			}

			localizationValues := shared.SplitCSV(localizationIDs.String())
			localeValue := strings.TrimSpace(*locale)
			titleValue := strings.TrimSpace(*title)
			if titleValue != "" && localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required when --title is set")
				return shared.MissingRequiredUsageError("--locale")
			}
			if localeValue != "" && titleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --title is required when --locale is set")
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
				fmt.Fprintln(os.Stderr, "Error: provide --localization-id or both --locale and --title")
				return flag.ErrHelp
			}

			client, err := appClipsClientFactory()
			if err != nil {
				return fmt.Errorf("app-clips invocations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BetaAppClipInvocationCreateAttributes{URL: urlValue}
			resp, err := client.CreateBetaAppClipInvocation(requestCtx, buildBundleValue, attrs, localizationValues, inlineLocalizations)
			if err != nil {
				return fmt.Errorf("app-clips invocations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipInvocationsUpdateCommand updates a beta App Clip invocation.
func AppClipInvocationsUpdateCommand() *ffcli.Command {
	d := shared.DefaultDeps()
	d.Client = func() (*asc.Client, error) { return appClipsClientFactory() }
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

// AppClipInvocationsDeleteCommand deletes a beta App Clip invocation.
func AppClipInvocationsDeleteCommand() *ffcli.Command {
	d := shared.DefaultDeps()
	d.Client = func() (*asc.Client, error) { return appClipsClientFactory() }

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
