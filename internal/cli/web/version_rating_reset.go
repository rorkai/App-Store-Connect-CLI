package web

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var (
	getVersionRatingResetFn = func(ctx context.Context, client *webcore.Client, versionID string) (*webcore.RatingResetRequestResponse, error) {
		return client.GetAppStoreVersionRatingResetRequest(ctx, versionID)
	}
	createVersionRatingResetFn = func(ctx context.Context, client *webcore.Client, versionID string) (*webcore.RatingResetRequestResponse, error) {
		return client.CreateAppStoreVersionRatingResetRequest(ctx, versionID)
	}
	deleteVersionRatingResetFn = func(ctx context.Context, client *webcore.Client, requestID string) error {
		return client.DeleteAppStoreVersionRatingResetRequest(ctx, requestID)
	}
)

// VersionRatingResetCommand returns the version overview-rating reset command group.
// It lives in the web command package so it can share the existing web-session
// authentication and provider-selection implementation while remaining mounted
// at "asc versions rating-reset".
func VersionRatingResetCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "rating-reset",
		ShortUsage: "asc versions rating-reset <subcommand> [flags]",
		ShortHelp:  "[experimental] Manage the overview-rating reset for an App Store version.",
		LongHelp: `[experimental] Manage the overview-rating reset for an App Store version.

Apple does not include this resource in its published OpenAPI specification.
These commands use an authenticated Apple Account web session and may stop
working if Apple changes its App Store Connect web service. Run
"asc web auth login" first when no valid cached session is available.

Scheduling a reset affects every country or region for the selected platform.
Written reviews remain visible. After the version is released, you cannot
restore the previous overview rating.

Examples:
  asc versions rating-reset view --version-id "VERSION_ID"
  asc versions rating-reset create --version-id "VERSION_ID" --confirm
  asc versions rating-reset delete --id "RESET_REQUEST_ID" --confirm`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			VersionRatingResetViewCommand(),
			VersionRatingResetCreateCommand(),
			VersionRatingResetDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// VersionRatingResetViewCommand returns the rating-reset view subcommand.
func VersionRatingResetViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("rating-reset view", flag.ExitOnError)
	versionID := fs.String("version-id", "", "App Store version ID (required)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc versions rating-reset view --version-id \"VERSION_ID\" [flags]",
		ShortHelp:  "View the overview-rating reset scheduled for an App Store version.",
		LongHelp: `View the overview-rating reset scheduled for an App Store version.

This command requires a valid Apple Account web session.

Examples:
  asc versions rating-reset view --version-id "VERSION_ID"
  asc versions rating-reset view --version-id "VERSION_ID" --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected arguments: %s", strings.Join(args, " "))
			}

			version := strings.TrimSpace(*versionID)
			if version == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}

			session, err := resolveWebSessionForCommand(ctx, authFlags)
			if err != nil {
				return err
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var response *webcore.RatingResetRequestResponse
			err = withWebSpinner("Loading rating reset request", func() error {
				var requestErr error
				response, requestErr = getVersionRatingResetFn(requestCtx, newWebClientFn(session), version)
				return requestErr
			})
			if err != nil {
				return withWebAuthHint(err, "rating-reset view")
			}
			if response == nil || strings.TrimSpace(response.Data.ID) == "" {
				return fmt.Errorf("rating-reset view failed: rating reset request ID missing from response")
			}

			return shared.PrintOutputWithRenderers(
				response,
				*output.Output,
				*output.Pretty,
				func() error { return renderVersionRatingResetTable(response) },
				func() error { return renderVersionRatingResetMarkdown(response) },
			)
		},
	}
}

// VersionRatingResetCreateCommand returns the rating-reset create subcommand.
func VersionRatingResetCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("rating-reset create", flag.ExitOnError)
	versionID := fs.String("version-id", "", "App Store version ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm scheduling the rating reset (required)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc versions rating-reset create --version-id \"VERSION_ID\" --confirm [flags]",
		ShortHelp:  "Schedule an overview-rating reset when an App Store version is released.",
		LongHelp: `Schedule an overview-rating reset when an App Store version is released.

The reset applies to every country or region for the selected platform. Written
reviews remain visible. You can cancel the request before release, but after the
version is released, the previous overview rating cannot be restored.

This command requires a valid Apple Account web session.

Examples:
  asc versions rating-reset create --version-id "VERSION_ID" --confirm
  asc versions rating-reset create --version-id "VERSION_ID" --confirm --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected arguments: %s", strings.Join(args, " "))
			}

			version := strings.TrimSpace(*versionID)
			if version == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			session, err := resolveWebSessionForCommand(ctx, authFlags)
			if err != nil {
				return err
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var response *webcore.RatingResetRequestResponse
			err = withWebSpinner("Scheduling rating reset", func() error {
				var requestErr error
				response, requestErr = createVersionRatingResetFn(requestCtx, newWebClientFn(session), version)
				return requestErr
			})
			if err != nil {
				return withWebAuthHint(err, "rating-reset create")
			}
			if response == nil || strings.TrimSpace(response.Data.ID) == "" {
				return fmt.Errorf("rating-reset create failed: rating reset request ID missing from response")
			}

			result := &asc.AppStoreVersionRatingResetCreateResult{
				RatingResetRequestID: strings.TrimSpace(response.Data.ID),
				VersionID:            version,
				Scheduled:            true,
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// VersionRatingResetDeleteCommand returns the rating-reset delete subcommand.
func VersionRatingResetDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("rating-reset delete", flag.ExitOnError)
	requestID := fs.String("id", "", "Rating reset request ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm cancellation (required)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc versions rating-reset delete --id \"RESET_REQUEST_ID\" --confirm [flags]",
		ShortHelp:  "Cancel a scheduled overview-rating reset before release.",
		LongHelp: `Cancel a scheduled overview-rating reset before release.

Use the request ID returned by rating-reset view or rating-reset create. This
command requires a valid Apple Account web session.

Examples:
  asc versions rating-reset delete --id "RESET_REQUEST_ID" --confirm
  asc versions rating-reset delete --id "RESET_REQUEST_ID" --confirm --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected arguments: %s", strings.Join(args, " "))
			}

			id := strings.TrimSpace(*requestID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			session, err := resolveWebSessionForCommand(ctx, authFlags)
			if err != nil {
				return err
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			err = withWebSpinner("Cancelling rating reset", func() error {
				return deleteVersionRatingResetFn(requestCtx, newWebClientFn(session), id)
			})
			if err != nil {
				return withWebAuthHint(err, "rating-reset delete")
			}

			result := &asc.AppStoreVersionRatingResetDeleteResult{
				RatingResetRequestID: id,
				Cancelled:            true,
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func versionRatingResetRows(response *webcore.RatingResetRequestResponse) ([]string, [][]string) {
	resetDate := ""
	if response.Data.Attributes.ResetDate != nil {
		resetDate = *response.Data.Attributes.ResetDate
	}
	return []string{"Rating Reset Request ID", "Reset Date"}, [][]string{{response.Data.ID, resetDate}}
}

func renderVersionRatingResetTable(response *webcore.RatingResetRequestResponse) error {
	headers, rows := versionRatingResetRows(response)
	asc.RenderTable(headers, rows)
	return nil
}

func renderVersionRatingResetMarkdown(response *webcore.RatingResetRequestResponse) error {
	headers, rows := versionRatingResetRows(response)
	asc.RenderMarkdown(headers, rows)
	return nil
}
