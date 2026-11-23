package reviews

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/builds"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	submitcli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/submit"
)

type reviewSubmitResult struct {
	AppID            string                           `json:"appId"`
	Version          string                           `json:"version,omitempty"`
	VersionID        string                           `json:"versionId"`
	BuildID          string                           `json:"buildId"`
	Platform         string                           `json:"platform"`
	DryRun           bool                             `json:"dryRun,omitempty"`
	SubmissionID     string                           `json:"submissionId,omitempty"`
	SubmittedDate    string                           `json:"submittedDate,omitempty"`
	AlreadySubmitted bool                             `json:"alreadySubmitted,omitempty"`
	WouldSubmit      bool                             `json:"wouldSubmit,omitempty"`
	BuildAttachment  *submitcli.BuildAttachmentResult `json:"buildAttachment,omitempty"`
	// ReviewItems lists every item submitted (or, with --dry-run, to be
	// submitted) to App Review, the version first.
	ReviewItems []asc.ReviewSubmissionReceiptItem `json:"reviewItems,omitempty"`
	Messages    []string                          `json:"messages,omitempty"`
}

func newReviewSubmitCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("submit", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := fs.String("version", "", "App Store version string")
	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	build := builds.BindVersionBuildSelectorWith(d, fs)
	confirm := fs.Bool("confirm", false, "Confirm submission (required unless --dry-run)")
	dryRun := fs.Bool("dry-run", false, "Preview the review submission flow without mutating")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc review submit [flags]",
		ShortHelp:  "Attach a build and submit an already-prepared App Store version for review.",
		LongHelp: `Attach a build and submit an already-prepared App Store version for review.

This is the easier modern wrapper around:
  - asc versions attach-build
  - asc review submissions-create
  - asc review items-add
  - asc review submissions-submit

Select the build with --build-id, or with --build-number or --latest, which
only match builds of the version's app, version string, and platform. A
selected build must be VALID; --wait waits for processing first.

When the version is already in a draft review submission that also holds
subscription or subscription group versions (for example staged with
` + "`asc web review subscriptions attach --review-items`" + `), submitting the draft
sends those items to App Review too. The command names each one on stderr
before submitting (and with --dry-run), with how to remove it first, and the
JSON receipt lists every submitted item in reviewItems, with named false for
the items the command line did not name.

Examples:
  asc review submit --app "123456789" --version "1.2.3" --build-id "BUILD_ID" --confirm
  asc review submit --app "123456789" --version "1.2.3" --build-number "45" --confirm
  asc review submit --app "123456789" --version "1.2.3" --build-number "46" --wait --confirm
  asc review submit --app "123456789" --version-id "VERSION_ID" --build-id "BUILD_ID" --dry-run`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			if err := build.Validate(); err != nil {
				return err
			}
			if strings.TrimSpace(*version) == "" && strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --version or --version-id is required")
				return shared.MissingRequiredUsageError("")
			}
			if strings.TrimSpace(*version) != "" && strings.TrimSpace(*versionID) != "" {
				return shared.WithDiagnostic(d.UsageError("--version and --version-id are mutually exclusive"), shared.DiagnosticConflictingInput, "")
			}
			if err := d.RequireConfirmUnlessDryRun(*dryRun, *confirm); err != nil {
				return err
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			requestedPlatform := ""
			if strings.TrimSpace(*version) != "" || visited["platform"] {
				normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
				if err != nil {
					return shared.WithDiagnostic(d.UsageError(err.Error()), shared.DiagnosticInvalidInput, "--platform")
				}
				requestedPlatform = normalizedPlatform
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("review submit: %w", err)
			}

			lookupCtx, lookupCancel := d.Timeout(ctx)
			defer lookupCancel()

			resolvedVersionID := strings.TrimSpace(*versionID)
			effectivePlatform := requestedPlatform
			versionString := strings.TrimSpace(*version)

			if resolvedVersionID != "" {
				versionData, err := shared.ResolveOwnedAppStoreVersionByID(lookupCtx, client, resolvedAppID, resolvedVersionID, requestedPlatform)
				if err != nil {
					return fmt.Errorf("review submit: fetch app store version %q: %w", resolvedVersionID, err)
				}

				effectivePlatform, err = shared.NormalizeAppStoreVersionPlatform(string(versionData.Attributes.Platform))
				if err != nil {
					return shared.WithDiagnostic(
						fmt.Errorf("review submit: version %q returned unsupported platform %q", resolvedVersionID, string(versionData.Attributes.Platform)),
						shared.DiagnosticDependencyFailed,
						"",
					)
				}
				versionString = strings.TrimSpace(versionData.Attributes.VersionString)
			} else {
				resolvedVersionID, err = shared.ResolveAppStoreVersionID(lookupCtx, client, resolvedAppID, versionString, effectivePlatform)
				if err != nil {
					return fmt.Errorf("review submit: %w", err)
				}
			}
			lookupCancel()

			// The build is resolved before the request budget starts, so
			// --wait does not use it up.
			buildID, err := build.Resolve(ctx, client, builds.VersionBuildScope{
				AppID:    resolvedAppID,
				Version:  versionString,
				Platform: effectivePlatform,
			})
			if err != nil {
				return fmt.Errorf("review submit: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			existingSubmissionID, err := submitcli.LookupExistingSubmissionForVersion(requestCtx, client, resolvedVersionID, 0)
			if err != nil {
				return fmt.Errorf("review submit: failed to lookup existing submission: %w", err)
			}
			if existingSubmissionID != "" {
				result := reviewSubmitResult{
					AppID:            resolvedAppID,
					Version:          versionString,
					VersionID:        resolvedVersionID,
					BuildID:          buildID,
					Platform:         effectivePlatform,
					DryRun:           *dryRun,
					SubmissionID:     existingSubmissionID,
					AlreadySubmitted: true,
				}
				return printReviewSubmitResult(d, result, *output.Output, *output.Pretty)
			}

			if err := submitcli.SubmissionLocalizationPreflightWith(requestCtx, d, client, resolvedAppID, resolvedVersionID, effectivePlatform, "asc review submit"); err != nil {
				return fmt.Errorf("review submit: %w", err)
			}
			if err := submitcli.SubmissionAgeRatingPreflightWith(requestCtx, d, client, resolvedAppID, resolvedVersionID, 0, "asc review submit"); err != nil {
				return fmt.Errorf("review submit: %w", err)
			}
			submitcli.SubmissionSubscriptionPreflightWith(requestCtx, d, client, resolvedAppID, "asc review submit")

			submitResult, err := submitcli.SubmitResolvedVersionWith(requestCtx, d, client, submitcli.SubmitResolvedVersionOptions{
				AppID:                    resolvedAppID,
				VersionID:                resolvedVersionID,
				BuildID:                  buildID,
				Platform:                 effectivePlatform,
				EnsureBuildAttached:      true,
				LookupExistingSubmission: false,
				DryRun:                   *dryRun,
				Emit: func(message string) {
					fmt.Fprintln(d.IO.ErrOut, message)
				},
			})
			if err != nil {
				return fmt.Errorf("review submit: %w", err)
			}

			result := reviewSubmitResult{
				AppID:            resolvedAppID,
				Version:          versionString,
				VersionID:        resolvedVersionID,
				BuildID:          buildID,
				Platform:         effectivePlatform,
				DryRun:           *dryRun,
				SubmissionID:     submitResult.SubmissionID,
				SubmittedDate:    submitResult.SubmittedDate,
				AlreadySubmitted: submitResult.AlreadySubmitted,
				WouldSubmit:      submitResult.WouldSubmit,
				BuildAttachment:  submitResult.BuildAttachment,
				ReviewItems:      submitResult.ReviewItems,
				Messages:         submitResult.Messages,
			}

			return printReviewSubmitResult(d, result, *output.Output, *output.Pretty)
		},
	}
}

func printReviewSubmitResult(d *shared.Deps, result reviewSubmitResult, format string, pretty bool) error {
	headers := []string{"App ID", "Version ID", "Build ID", "Platform", "Submission ID", "Already Submitted", "Dry Run"}
	rows := [][]string{{
		result.AppID,
		result.VersionID,
		result.BuildID,
		result.Platform,
		result.SubmissionID,
		strconv.FormatBool(result.AlreadySubmitted),
		strconv.FormatBool(result.DryRun),
	}}
	return d.PrintOutputRows(result, format, pretty, headers, rows)
}
