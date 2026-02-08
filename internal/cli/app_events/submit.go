package app_events

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

// AppEventsSubmitCommand returns the app events submit subcommand.
func AppEventsSubmitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)

	eventID := fs.String("event-id", "", "App event ID")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	confirm := fs.Bool("confirm", false, "Confirm submission (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc app-events submit [flags]",
		ShortHelp:  "Submit an in-app event for review.",
		LongHelp: `Submit an in-app event for review.

Examples:
  asc app-events submit --event-id "EVENT_ID" --app "APP_ID" --confirm
  asc app-events submit --event-id "EVENT_ID" --app "APP_ID" --platform IOS --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to submit for review")
				return flag.ErrHelp
			}

			id := strings.TrimSpace(*eventID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id is required")
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return fmt.Errorf("app-events submit: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-events submit: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			reviewSubmission, err := client.CreateReviewSubmission(requestCtx, resolvedAppID, asc.Platform(normalizedPlatform))
			if err != nil {
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

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
