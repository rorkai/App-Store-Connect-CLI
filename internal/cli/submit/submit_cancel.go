package submit

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

func SubmitCancelCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit cancel", flag.ExitOnError)

	submissionID := fs.String("id", "", "Submission ID")
	versionID := fs.String("version-id", "", "App Store version ID")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID); used with --version-id for modern API lookup")
	confirm := fs.Bool("confirm", false, "Confirm cancellation (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "cancel",
		ShortUsage: "asc submit cancel [flags]",
		ShortHelp:  "Cancel a submission.",
		LongHelp: `Cancel a submission.

Cancels an active review submission. Use --id for a known submission ID,
or --version-id to find and cancel the submission for a specific version.
When using --version-id, provide --app for reliable lookup via the modern
reviewSubmissions API; without --app, falls back to the legacy endpoint.

Examples:
  asc submit cancel --id "SUBMISSION_ID" --confirm
  asc submit cancel --version-id "VERSION_ID" --confirm
  asc submit cancel --version-id "VERSION_ID" --app "APP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to cancel a submission")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*submissionID) == "" && strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --version-id is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*submissionID) != "" && strings.TrimSpace(*versionID) != "" {
				return shared.UsageError("--id and --version-id are mutually exclusive")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("submit cancel: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer func() {
				if cancel != nil {
					cancel()
				}
			}()

			refreshRequestCtx := func() {
				if cancel != nil {
					cancel()
				}
				requestCtx, cancel = shared.ContextWithTimeout(ctx)
			}

			resolvedSubmissionID := strings.TrimSpace(*submissionID)
			if resolvedSubmissionID != "" {
				_, err := client.CancelReviewSubmission(requestCtx, resolvedSubmissionID)
				if err != nil {
					if asc.IsNotFound(err) {
						return fmt.Errorf("submit cancel: no review submission found for ID %q", resolvedSubmissionID)
					}
					return fmt.Errorf("submit cancel: %w", err)
				}
			} else {
				resolvedVersionID := strings.TrimSpace(*versionID)
				explicitAppID := strings.TrimSpace(*appID)

				// Prefer the app relationship on the version itself so a stale
				// ASC_APP_ID/config value does not misdirect the modern lookup.
				resolvedAppID := ""
				versionResp, vErr := client.GetAppStoreVersion(requestCtx, resolvedVersionID, asc.WithAppStoreVersionInclude([]string{"app"}))
				if vErr == nil {
					if aid, aidErr := resolveAppIDFromVersionResponse(versionResp); aidErr == nil {
						if explicitAppID != "" && explicitAppID != aid {
							return fmt.Errorf("submit cancel: version %q belongs to app %q, not %q", resolvedVersionID, aid, explicitAppID)
						}
						resolvedAppID = aid
					}
				}
				if resolvedAppID == "" {
					if explicitAppID != "" {
						resolvedAppID = explicitAppID
					} else {
						resolvedAppID = shared.ResolveAppID(*appID)
					}
				}

				if resolvedAppID != "" {
					submission, findErr := findReviewSubmissionForVersion(requestCtx, client, resolvedAppID, resolvedVersionID)
					if findErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: modern review submission lookup failed: %v (falling back to legacy)\n", findErr)
					} else if submission != nil {
						if submission.Attributes.SubmissionState == asc.ReviewSubmissionStateCanceling {
							resolvedSubmissionID = submission.ID
							result := &asc.AppStoreVersionSubmissionCancelResult{
								ID:        resolvedSubmissionID,
								Cancelled: true,
							}
							return shared.PrintOutput(result, *output.Output, *output.Pretty)
						}
						if !isPotentiallyCancellableReviewSubmissionState(submission.Attributes.SubmissionState) {
							submission = nil
						}
					}
					if submission != nil {
						_, cancelErr := client.CancelReviewSubmission(requestCtx, submission.ID)
						if cancelErr != nil {
							if isExpectedNonCancellableReviewSubmissionError(cancelErr) {
								if refreshedCanceling, refreshErr := reviewSubmissionIsState(requestCtx, client, submission.ID, asc.ReviewSubmissionStateCanceling); refreshErr == nil && refreshedCanceling {
									resolvedSubmissionID = submission.ID
									result := &asc.AppStoreVersionSubmissionCancelResult{
										ID:        resolvedSubmissionID,
										Cancelled: true,
									}
									return shared.PrintOutput(result, *output.Output, *output.Pretty)
								}
								return fmt.Errorf("submit cancel: submission %s is no longer cancellable: %w", submission.ID, cancelErr)
							} else {
								return fmt.Errorf("submit cancel: failed to cancel submission %s: %w", submission.ID, cancelErr)
							}
						}
						if submission != nil {
							resolvedSubmissionID = submission.ID
							result := &asc.AppStoreVersionSubmissionCancelResult{
								ID:        resolvedSubmissionID,
								Cancelled: true,
							}
							return shared.PrintOutput(result, *output.Output, *output.Pretty)
						}
					}
				}

				// Fall back to legacy version submission lookup.
				if requestCtx.Err() != nil {
					refreshRequestCtx()
				}
				submissionResp, err := client.GetAppStoreVersionSubmissionForVersion(requestCtx, resolvedVersionID)
				if err != nil {
					if asc.IsNotFound(err) {
						return fmt.Errorf("submit cancel: no active submission found for version %q (tried modern and legacy APIs)", resolvedVersionID)
					}
					return fmt.Errorf("submit cancel: %w", err)
				}
				resolvedSubmissionID = strings.TrimSpace(submissionResp.Data.ID)
				if resolvedSubmissionID == "" {
					return fmt.Errorf("submit cancel: no submission found for version %q", resolvedVersionID)
				}

				// Try modern cancel, then legacy delete.
				_, err = client.CancelReviewSubmission(requestCtx, resolvedSubmissionID)
				if err == nil {
					result := &asc.AppStoreVersionSubmissionCancelResult{
						ID:        resolvedSubmissionID,
						Cancelled: true,
					}
					return shared.PrintOutput(result, *output.Output, *output.Pretty)
				}
				if !asc.IsNotFound(err) {
					return fmt.Errorf("submit cancel: %w", err)
				}

				if err := client.DeleteAppStoreVersionSubmission(requestCtx, resolvedSubmissionID); err != nil {
					if asc.IsNotFound(err) {
						return fmt.Errorf("submit cancel: no submission found for ID %q", resolvedSubmissionID)
					}
					return fmt.Errorf("submit cancel: %w", err)
				}
			}

			result := &asc.AppStoreVersionSubmissionCancelResult{
				ID:        resolvedSubmissionID,
				Cancelled: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
