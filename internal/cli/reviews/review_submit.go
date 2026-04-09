package reviews

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

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
	Messages         []string                         `json:"messages,omitempty"`
}

// ReviewSubmitCommand returns the high-level review submit command.
func ReviewSubmitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := fs.String("version", "", "App Store version string")
	versionID := fs.String("version-id", "", "App Store version ID")
	buildID := fs.String("build", "", "Build ID to attach")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	confirm := fs.Bool("confirm", false, "Confirm submission (required unless --dry-run)")
	dryRun := fs.Bool("dry-run", false, "Preview the review submission flow without mutating")
	output := shared.BindOutputFlags(fs)

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

Examples:
  asc review submit --app "123456789" --version "1.2.3" --build "BUILD_ID" --confirm
  asc review submit --app "123456789" --version-id "VERSION_ID" --build "BUILD_ID" --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
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
				return shared.UsageError("--version and --version-id are mutually exclusive")
			}
			if !*confirm && !*dryRun {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required unless --dry-run is set")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			requestedPlatform := ""
			if strings.TrimSpace(*version) != "" || visited["platform"] {
				normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
				if err != nil {
					return shared.UsageError(err.Error())
				}
				requestedPlatform = normalizedPlatform
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review submit: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedVersionID := strings.TrimSpace(*versionID)
			effectivePlatform := requestedPlatform
			versionString := strings.TrimSpace(*version)

			if resolvedVersionID != "" {
				versionData, err := shared.ResolveOwnedAppStoreVersionByID(requestCtx, client, resolvedAppID, resolvedVersionID, requestedPlatform)
				if err != nil {
					return fmt.Errorf("review submit: fetch app store version %q: %w", resolvedVersionID, err)
				}

				effectivePlatform, err = shared.NormalizeAppStoreVersionPlatform(string(versionData.Attributes.Platform))
				if err != nil {
					return fmt.Errorf("review submit: version %q returned unsupported platform %q", resolvedVersionID, string(versionData.Attributes.Platform))
				}
				versionString = strings.TrimSpace(versionData.Attributes.VersionString)
			} else {
				resolvedVersionID, err = shared.ResolveAppStoreVersionID(requestCtx, client, resolvedAppID, versionString, effectivePlatform)
				if err != nil {
					return fmt.Errorf("review submit: %w", err)
				}
			}

			existingSubmissionID, err := submitcli.LookupExistingSubmissionForVersion(requestCtx, client, resolvedVersionID, 0)
			if err != nil {
				return fmt.Errorf("review submit: failed to lookup existing submission: %w", err)
			}
			if existingSubmissionID != "" {
				result := reviewSubmitResult{
					AppID:            resolvedAppID,
					Version:          versionString,
					VersionID:        resolvedVersionID,
					BuildID:          strings.TrimSpace(*buildID),
					Platform:         effectivePlatform,
					DryRun:           *dryRun,
					SubmissionID:     existingSubmissionID,
					AlreadySubmitted: true,
				}
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			if err := submitcli.SubmissionLocalizationPreflight(requestCtx, client, resolvedAppID, resolvedVersionID, effectivePlatform, "asc review submit"); err != nil {
				return fmt.Errorf("review submit: %w", err)
			}
			submitcli.SubmissionSubscriptionPreflight(requestCtx, client, resolvedAppID, "asc review submit")

			submitResult, err := submitcli.SubmitResolvedVersion(requestCtx, client, submitcli.SubmitResolvedVersionOptions{
				AppID:                    resolvedAppID,
				VersionID:                resolvedVersionID,
				BuildID:                  strings.TrimSpace(*buildID),
				Platform:                 effectivePlatform,
				EnsureBuildAttached:      true,
				LookupExistingSubmission: true,
				DryRun:                   *dryRun,
				Emit: func(message string) {
					fmt.Fprintln(os.Stderr, message)
				},
			})
			if err != nil {
				return fmt.Errorf("review submit: %w", err)
			}

			result := reviewSubmitResult{
				AppID:            resolvedAppID,
				Version:          versionString,
				VersionID:        resolvedVersionID,
				BuildID:          strings.TrimSpace(*buildID),
				Platform:         effectivePlatform,
				DryRun:           *dryRun,
				SubmissionID:     submitResult.SubmissionID,
				SubmittedDate:    submitResult.SubmittedDate,
				AlreadySubmitted: submitResult.AlreadySubmitted,
				WouldSubmit:      submitResult.WouldSubmit,
				BuildAttachment:  submitResult.BuildAttachment,
				Messages:         submitResult.Messages,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
