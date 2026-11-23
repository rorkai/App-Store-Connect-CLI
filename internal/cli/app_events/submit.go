package app_events

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppEventsSubmitCommand returns the app events submit subcommand.
func newAppEventsSubmitCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("submit", flag.ExitOnError)

	eventID := shared.BindResourceIDFlag(fs, "event-id", "appEvents", "App event ID")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	confirm := fs.Bool("confirm", false, "Confirm submission (required)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc app-events submit [flags]",
		ShortHelp:  "Submit an in-app event for review.",
		LongHelp: `Submit an in-app event for review.

Examples:
  asc app-events submit --event-id "EVENT_ID" --app "APP_ID" --confirm
  asc app-events submit --event-id "EVENT_ID" --app "APP_ID" --platform IOS --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*eventID)
			if id == "" {
				return d.RequiredFlag("--event-id")
			}

			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return d.UsageErrorf("app-events submit: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-events submit: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			reviewSubmission, err := client.CreateReviewSubmission(requestCtx, resolvedAppID, asc.Platform(normalizedPlatform))
			if err != nil {
				var partialErr *asc.ReviewSubmissionCreatePartialError
				if errors.As(err, &partialErr) && partialErr.Response != nil &&
					partialErr.Response.Data.Type == asc.ResourceTypeReviewSubmissions {
					submissionID := strings.TrimSpace(partialErr.Response.Data.ID)
					if submissionID != "" {
						// The submission is an empty unsubmitted draft
						// (READY_FOR_REVIEW), which App Store Connect refuses to
						// cancel, so name the commands that finish it instead.
						return fmt.Errorf(
							"app-events submit: review submission %q may have been created, but no event was added or submitted; inspect it with `asc review submissions-get --id %s`. App Store Connect does not cancel an unsubmitted draft (READY_FOR_REVIEW): add the event with `asc review items add --submission %s --item-type appEvents --item-id %s` and send it with `asc review submissions-submit --id %s --confirm`: %w",
							submissionID,
							submissionID,
							submissionID,
							id,
							submissionID,
							err,
						)
					}
				}
				return fmt.Errorf("app-events submit: failed to create review submission: %w", err)
			}

			itemResp, err := client.CreateReviewSubmissionItem(requestCtx, reviewSubmission.Data.ID, asc.ReviewSubmissionItemTypeAppEvent, id)
			if err != nil {
				return fmt.Errorf("app-events submit: failed to add event to submission: %w", err)
			}

			submitResp, err := client.SubmitReviewSubmission(requestCtx, reviewSubmission.Data.ID)
			if err != nil {
				return fmt.Errorf("app-events submit: failed to submit for review: %w", err)
			}

			submittedDate := submitResp.Data.Attributes.SubmittedDate
			var submittedDatePtr *string
			if submittedDate != "" {
				submittedDatePtr = &submittedDate
			}

			result := &asc.AppEventSubmissionResult{
				SubmissionID:  submitResp.Data.ID,
				ItemID:        itemResp.Data.ID,
				EventID:       id,
				AppID:         resolvedAppID,
				Platform:      normalizedPlatform,
				SubmittedDate: submittedDatePtr,
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
