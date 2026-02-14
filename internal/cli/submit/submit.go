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

func SubmitCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc submit <subcommand> [flags]",
		ShortHelp:  "Submit builds for App Store review.",
		LongHelp:   `Submit builds for App Store review.`,
		UsageFunc:  shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubmitCreateCommand(),
			SubmitStatusCommand(),
			SubmitCancelCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func SubmitCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := fs.String("version", "", "App Store version string")
	versionID := fs.String("version-id", "", "App Store version ID")
	buildID := fs.String("build", "", "Build ID to attach")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	confirm := fs.Bool("confirm", false, "Confirm submission (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc submit create [flags]",
		ShortHelp:  "Submit a build for App Store review.",
		LongHelp: `Submit a build for App Store review.

Examples:
  asc submit create --app "123456789" --version "1.0.0" --build "BUILD_ID" --confirm
  asc submit create --app "123456789" --version-id "VERSION_ID" --build "BUILD_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to submit for review")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*buildID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*version) == "" && strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version or --version-id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*version) != "" && strings.TrimSpace(*versionID) != "" {
				return fmt.Errorf("submit create: --version and --version-id are mutually exclusive")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return fmt.Errorf("submit create: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("submit create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedVersionID := strings.TrimSpace(*versionID)
			if resolvedVersionID == "" {
				resolvedVersionID, err = shared.ResolveAppStoreVersionID(requestCtx, client, resolvedAppID, strings.TrimSpace(*version), normalizedPlatform)
				if err != nil {
					return fmt.Errorf("submit create: %w", err)
				}
			}

			// Attach build to version
			if err := client.AttachBuildToVersion(requestCtx, resolvedVersionID, strings.TrimSpace(*buildID)); err != nil {
				return fmt.Errorf("submit create: failed to attach build: %w", err)
			}

			// Use the new reviewSubmissions API (the old appStoreVersionSubmissions is deprecated)
			// Step 1: Create review submission for the app
			reviewSubmission, err := client.CreateReviewSubmission(requestCtx, resolvedAppID, asc.Platform(normalizedPlatform))
			if err != nil {
				return fmt.Errorf("submit create: failed to create review submission: %w", err)
			}

			// Step 2: Add the app store version as a submission item
			_, err = client.AddReviewSubmissionItem(requestCtx, reviewSubmission.Data.ID, resolvedVersionID)
			if err != nil {
				return fmt.Errorf("submit create: failed to add version to submission: %w", err)
			}

			// Step 3: Submit for review
			submitResp, err := client.SubmitReviewSubmission(requestCtx, reviewSubmission.Data.ID)
			if err != nil {
				return fmt.Errorf("submit create: failed to submit for review: %w", err)
			}

			submittedDate := submitResp.Data.Attributes.SubmittedDate
			var createdDatePtr *string
			if submittedDate != "" {
				createdDatePtr = &submittedDate
			}
			result := &asc.AppStoreVersionSubmissionCreateResult{
				SubmissionID: submitResp.Data.ID,
				VersionID:    resolvedVersionID,
				BuildID:      strings.TrimSpace(*buildID),
				CreatedDate:  createdDatePtr,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func SubmitStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit status", flag.ExitOnError)

	submissionID := fs.String("id", "", "Submission ID")
	versionID := fs.String("version-id", "", "App Store version ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc submit status [flags]",
		ShortHelp:  "Check submission status.",
		LongHelp: `Check submission status.

Examples:
  asc submit status --id "SUBMISSION_ID"
  asc submit status --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*submissionID) == "" && strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --version-id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*submissionID) != "" && strings.TrimSpace(*versionID) != "" {
				return fmt.Errorf("submit status: --id and --version-id are mutually exclusive")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("submit status: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var submissionResp *asc.AppStoreVersionSubmissionResourceResponse
			resolvedVersionID := strings.TrimSpace(*versionID)
			if strings.TrimSpace(*submissionID) != "" {
				submissionResp, err = client.GetAppStoreVersionSubmissionResource(requestCtx, strings.TrimSpace(*submissionID))
				if err != nil && asc.IsNotFound(err) {
					return fmt.Errorf("submit status: no submission found for ID %q", strings.TrimSpace(*submissionID))
				}
			} else {
				submissionResp, err = client.GetAppStoreVersionSubmissionForVersion(requestCtx, resolvedVersionID)
				if err != nil && asc.IsNotFound(err) {
					return fmt.Errorf("submit status: no submission found for version %q", resolvedVersionID)
				}
			}
			if err != nil {
				return fmt.Errorf("submit status: %w", err)
			}

			resolvedSubmissionID := submissionResp.Data.ID
			if submissionResp.Data.Relationships.AppStoreVersion != nil && submissionResp.Data.Relationships.AppStoreVersion.Data.ID != "" {
				resolvedVersionID = submissionResp.Data.Relationships.AppStoreVersion.Data.ID
			}

			result := &asc.AppStoreVersionSubmissionStatusResult{
				ID:          resolvedSubmissionID,
				VersionID:   resolvedVersionID,
				CreatedDate: submissionResp.Data.Attributes.CreatedDate,
			}

			if resolvedVersionID != "" {
				versionResp, err := client.GetAppStoreVersion(requestCtx, resolvedVersionID)
				if err != nil {
					return fmt.Errorf("submit status: %w", err)
				}
				result.VersionString = versionResp.Data.Attributes.VersionString
				result.Platform = string(versionResp.Data.Attributes.Platform)
				result.State = shared.ResolveAppStoreVersionState(versionResp.Data.Attributes)
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func SubmitCancelCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit cancel", flag.ExitOnError)

	submissionID := fs.String("id", "", "Submission ID")
	versionID := fs.String("version-id", "", "App Store version ID")
	confirm := fs.Bool("confirm", false, "Confirm cancellation (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "cancel",
		ShortUsage: "asc submit cancel [flags]",
		ShortHelp:  "Cancel a submission.",
		LongHelp: `Cancel a submission.

Examples:
  asc submit cancel --id "SUBMISSION_ID" --confirm
  asc submit cancel --version-id "VERSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to cancel a submission")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*submissionID) == "" && strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --version-id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*submissionID) != "" && strings.TrimSpace(*versionID) != "" {
				return fmt.Errorf("submit cancel: --id and --version-id are mutually exclusive")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("submit cancel: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

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

				// Resolve via legacy version submission lookup for backward compatibility.
				submissionResp, err := client.GetAppStoreVersionSubmissionForVersion(requestCtx, resolvedVersionID)
				if err != nil {
					if asc.IsNotFound(err) {
						return fmt.Errorf("submit cancel: no legacy submission found for version %q", resolvedVersionID)
					}
					return fmt.Errorf("submit cancel: %w", err)
				}
				resolvedSubmissionID = strings.TrimSpace(submissionResp.Data.ID)
				if resolvedSubmissionID == "" {
					return fmt.Errorf("submit cancel: no legacy submission found for version %q", resolvedVersionID)
				}

				// Prefer the modern reviewSubmissions cancel endpoint when possible.
				_, err = client.CancelReviewSubmission(requestCtx, resolvedSubmissionID)
				if err == nil {
					result := &asc.AppStoreVersionSubmissionCancelResult{
						ID:        resolvedSubmissionID,
						Cancelled: true,
					}
					return shared.PrintOutput(result, *output, *pretty)
				}
				if !asc.IsNotFound(err) {
					return fmt.Errorf("submit cancel: %w", err)
				}

				// Fall back to the legacy delete endpoint for old submission flows.
				if err := client.DeleteAppStoreVersionSubmission(requestCtx, resolvedSubmissionID); err != nil {
					if asc.IsNotFound(err) {
						return fmt.Errorf("submit cancel: no legacy submission found for ID %q", resolvedSubmissionID)
					}
					return fmt.Errorf("submit cancel: %w", err)
				}
			}

			result := &asc.AppStoreVersionSubmissionCancelResult{
				ID:        resolvedSubmissionID,
				Cancelled: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
