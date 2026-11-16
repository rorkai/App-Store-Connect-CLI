package web

import (
	"context"
	"flag"
	"fmt"
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

// NewVersionRatingResetCommand returns the version overview-rating reset command group.
// It lives in the web command package so it can share the existing web-session
// authentication and provider-selection implementation while remaining mounted
// at "asc versions rating-reset".
func NewVersionRatingResetCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "rating-reset",
		ShortUsage: "asc versions rating-reset <subcommand> [flags]",
		ShortHelp:  "Manage the overview-rating reset for an App Store version.",
		LongHelp: `Manage the overview-rating reset for an App Store version.

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
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newVersionRatingResetViewCommand(d),
			newVersionRatingResetCreateCommand(d),
			newVersionRatingResetDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newVersionRatingResetViewCommand returns the rating-reset view subcommand.
func newVersionRatingResetViewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("rating-reset view", flag.ExitOnError)
	versionID := fs.String("version-id", "", "App Store version ID (required)")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			version := strings.TrimSpace(*versionID)
			if version == "" {
				return d.RequiredFlag("--version-id")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			var response *webcore.RatingResetRequestResponse
			err = withWebSpinnerWith(d, "Loading rating reset request", func() error {
				var requestErr error
				response, requestErr = getVersionRatingResetFn(requestCtx, newWebClientFn(session), version)
				return requestErr
			})
			if err != nil {
				return withWebAuthHint(err, "rating-reset view")
			}
			if response == nil {
				return fmt.Errorf("rating-reset view failed: response missing")
			}
			if response.Data != nil && strings.TrimSpace(response.Data.ID) == "" {
				return fmt.Errorf("rating-reset view failed: rating reset request ID missing from response")
			}

			return d.PrintOutputWithRenderers(
				response,
				*output.Output,
				*output.Pretty,
				func() error { return renderVersionRatingResetTable(d, response) },
				func() error { return renderVersionRatingResetMarkdown(d, response) },
			)
		},
	}
}

// newVersionRatingResetCreateCommand returns the rating-reset create subcommand.
func newVersionRatingResetCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("rating-reset create", flag.ExitOnError)
	versionID := fs.String("version-id", "", "App Store version ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm scheduling the rating reset (required)")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			version := strings.TrimSpace(*versionID)
			if version == "" {
				return d.RequiredFlag("--version-id")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			var response *webcore.RatingResetRequestResponse
			err = withWebSpinnerWith(d, "Scheduling rating reset", func() error {
				var requestErr error
				response, requestErr = createVersionRatingResetFn(requestCtx, newWebClientFn(session), version)
				return requestErr
			})
			if err != nil {
				return withWebAuthHint(err, "rating-reset create")
			}
			if response == nil || response.Data == nil || strings.TrimSpace(response.Data.ID) == "" {
				return fmt.Errorf("rating-reset create failed: rating reset request ID missing from response")
			}

			result := &asc.AppStoreVersionRatingResetCreateResult{
				RatingResetRequestID: strings.TrimSpace(response.Data.ID),
				VersionID:            version,
				Scheduled:            true,
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newVersionRatingResetDeleteCommand returns the rating-reset delete subcommand.
func newVersionRatingResetDeleteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("rating-reset delete", flag.ExitOnError)
	requestID := fs.String("id", "", "Rating reset request ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm cancellation (required)")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*requestID)
			if id == "" {
				return d.RequiredFlag("--id")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			err = withWebSpinnerWith(d, "Cancelling rating reset", func() error {
				return deleteVersionRatingResetFn(requestCtx, newWebClientFn(session), id)
			})
			if err != nil {
				return withWebAuthHint(err, "rating-reset delete")
			}

			result := &asc.AppStoreVersionRatingResetDeleteResult{
				RatingResetRequestID: id,
				Cancelled:            true,
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func versionRatingResetRows(response *webcore.RatingResetRequestResponse) ([]string, [][]string) {
	headers := []string{"Rating Reset Request ID", "Reset Date", "Scheduled"}
	if response.Data == nil {
		return headers, [][]string{{"", "", "false"}}
	}
	resetDate := ""
	if response.Data.Attributes.ResetDate != nil {
		resetDate = *response.Data.Attributes.ResetDate
	}
	return headers, [][]string{{response.Data.ID, resetDate, "true"}}
}

func renderVersionRatingResetTable(d *shared.Deps, response *webcore.RatingResetRequestResponse) error {
	headers, rows := versionRatingResetRows(response)
	asc.RenderTableTo(d.IO.Out, headers, rows)
	return nil
}

func renderVersionRatingResetMarkdown(d *shared.Deps, response *webcore.RatingResetRequestResponse) error {
	headers, rows := versionRatingResetRows(response)
	asc.RenderMarkdownTo(d.IO.Out, headers, rows)
	return nil
}
