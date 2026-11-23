package testflight

import (
	"context"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newTestFlightReviewCommand returns the testflight review command with subcommands.
func newTestFlightReviewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("review", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review",
		ShortUsage: "asc testflight review <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight beta app review details.",
		LongHelp: `Manage TestFlight beta app review details and submissions.

Examples:
  asc testflight review view --app "APP_ID"
  asc testflight review update --id "DETAIL_ID" --contact-email "dev@example.com"
  asc testflight review submit --build-id "BUILD_ID" --confirm
  asc testflight review app view --id "DETAIL_ID"
  asc testflight review submissions list --build-id "BUILD_ID"
  asc testflight review submissions view --id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newTestFlightReviewGetCommand(d),
			newTestFlightReviewAppCommand(d),
			newTestFlightReviewUpdateCommand(d),
			newTestFlightReviewSubmitCommand(d),
			newTestFlightReviewSubmissionsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newTestFlightReviewGetCommand retrieves beta app review details for an app.
func newTestFlightReviewGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("view", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	includeSensitive := shared.BindIncludeSensitiveFlag(fs)
	output := d.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight review view [flags]",
		ShortHelp:  "View beta app review details for an app.",
		LongHelp: `View beta app review details for an app.

Examples:
  asc testflight review view --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "testflight review view", 200); err != nil {
				return err
			}

			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight review view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BetaAppReviewDetailsOption{
				asc.WithBetaAppReviewDetailsAppID(resolvedAppID),
				asc.Limit[asc.BetaAppReviewDetailsOption](*limit),
				asc.NextURL[asc.BetaAppReviewDetailsOption](*next),
			}

			details, err := client.GetBetaAppReviewDetails(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("testflight review view: failed to fetch: %w", err)
			}

			shared.WarnIncludeSensitive(d.IO.ErrOut, *includeSensitive)
			if !*includeSensitive {
				details = asc.RedactBetaAppReviewDetailsResponse(details)
			}
			return d.PrintOutput(details, *output.Output, *output.Pretty)
		},
	}
}

// newTestFlightReviewUpdateCommand updates beta app review details.
func newTestFlightReviewUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "betaAppReviewDetails", "Beta app review detail ID")
	contactFirstName := fs.String("contact-first-name", "", "Contact first name")
	contactLastName := fs.String("contact-last-name", "", "Contact last name")
	contactEmail := fs.String("contact-email", "", "Contact email")
	contactPhone := fs.String("contact-phone", "", "Contact phone")
	demoAccountName := fs.String("demo-account-name", "", "Demo account name")
	demoAccountPassword := fs.String("demo-account-password", "", "Demo account password")
	demoAccountRequired := fs.Bool("demo-account-required", false, "Demo account required")
	notes := fs.String("notes", "", "Review notes")
	includeSensitive := shared.BindIncludeSensitiveFlag(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc testflight review update [flags]",
		ShortHelp:  "Update beta app review details.",
		LongHelp: `Update beta app review details.

Examples:
  asc testflight review update --id "DETAIL_ID" --contact-email "dev@example.com"
  asc testflight review update --id "DETAIL_ID" --notes "Updated review notes"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			detailID := strings.TrimSpace(*id)
			if detailID == "" {
				return d.RequiredFlag("--id")
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			hasUpdates := visited["contact-first-name"] ||
				visited["contact-last-name"] ||
				visited["contact-email"] ||
				visited["contact-phone"] ||
				visited["demo-account-name"] ||
				visited["demo-account-password"] ||
				visited["demo-account-required"] ||
				visited["notes"]
			if !hasUpdates {
				return d.UpdateFlagRequired(fs, contactFirstName, contactLastName, contactEmail, contactPhone, demoAccountName, demoAccountPassword, demoAccountRequired, notes)
			}

			attrs := asc.BetaAppReviewDetailUpdateAttributes{}
			if visited["contact-first-name"] {
				value := strings.TrimSpace(*contactFirstName)
				attrs.ContactFirstName = &value
			}
			if visited["contact-last-name"] {
				value := strings.TrimSpace(*contactLastName)
				attrs.ContactLastName = &value
			}
			if visited["contact-email"] {
				value := strings.TrimSpace(*contactEmail)
				attrs.ContactEmail = &value
			}
			if visited["contact-phone"] {
				value := strings.TrimSpace(*contactPhone)
				attrs.ContactPhone = &value
			}
			if visited["demo-account-name"] {
				value := strings.TrimSpace(*demoAccountName)
				attrs.DemoAccountName = &value
			}
			if visited["demo-account-password"] {
				value := strings.TrimSpace(*demoAccountPassword)
				attrs.DemoAccountPassword = &value
			}
			if visited["demo-account-required"] {
				value := *demoAccountRequired
				attrs.DemoAccountRequired = &value
			}
			if visited["notes"] {
				value := strings.TrimSpace(*notes)
				attrs.Notes = &value
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight review update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			detail, err := client.UpdateBetaAppReviewDetail(requestCtx, detailID, attrs)
			if err != nil {
				return fmt.Errorf("testflight review update: failed to update: %w", err)
			}

			shared.WarnIncludeSensitive(d.IO.ErrOut, *includeSensitive)
			if !*includeSensitive {
				detail = asc.RedactBetaAppReviewDetailResponse(detail)
			}
			return d.PrintOutput(detail, *output.Output, *output.Pretty)
		},
	}
}

// newTestFlightReviewSubmitCommand submits a build for beta app review.
func newTestFlightReviewSubmitCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("submit", flag.ExitOnError)

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID")
	shared.BindFlagAlias(fs, "build", "build-id")
	confirm := fs.Bool("confirm", false, "Confirm submission")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc testflight review submit --build-id BUILD_ID --confirm",
		ShortHelp:  "Submit a build for beta app review.",
		LongHelp: `Submit a build for beta app review.

Examples:
  asc testflight review submit --build-id "BUILD_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*buildID) == "" {
				return d.RequiredFlag("--build-id")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight review submit: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			submission, err := client.CreateBetaAppReviewSubmission(requestCtx, strings.TrimSpace(*buildID))
			if err != nil {
				err = shared.ExplainBetaReviewSubmissionError(requestCtx, client, strings.TrimSpace(*buildID), err)
				return fmt.Errorf("testflight review submit: failed to submit: %w", err)
			}

			return d.PrintOutput(submission, *output.Output, *output.Pretty)
		},
	}
}

// newTestFlightReviewAppCommand returns the review app command group.
func newTestFlightReviewAppCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc testflight review app <subcommand> [flags]",
		ShortHelp:  "View the app for a beta app review detail.",
		LongHelp: `View the app for a beta app review detail.

Examples:
  asc testflight review app view --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newTestFlightReviewAppGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newTestFlightReviewAppGetCommand retrieves the app for a beta app review detail.
func newTestFlightReviewAppGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "app view",
		Name:        "view",
		ShortUsage:  "asc testflight review app view --id \"DETAIL_ID\"",
		ShortHelp:   "View the app for a beta app review detail.",
		LongHelp: `View the app for a beta app review detail.

Examples:
  asc testflight review app view --id "DETAIL_ID"`,
		IDUsage:            "Beta app review detail ID",
		IDType:             "betaAppReviewDetails",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBetaAppReviewDetailApp(ctx, id)
		},
	})
}

// newTestFlightReviewSubmissionsCommand returns the review submissions command group.
func newTestFlightReviewSubmissionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("submissions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "submissions",
		ShortUsage: "asc testflight review submissions <subcommand> [flags]",
		ShortHelp:  "View beta app review submissions.",
		LongHelp: `View beta app review submissions.

Examples:
  asc testflight review submissions view --id "SUBMISSION_ID"
  asc testflight review submissions build --id "SUBMISSION_ID"
  asc testflight review submissions list --build-id "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newTestFlightReviewSubmissionsListCommand(d),
			newTestFlightReviewSubmissionsGetCommand(d),
			newTestFlightReviewSubmissionsBuildCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newTestFlightReviewSubmissionsListCommand lists beta app review submissions.
func newTestFlightReviewSubmissionsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("submissions list", flag.ExitOnError)

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID to filter")
	shared.BindFlagAlias(fs, "build", "build-id")
	appFlag := fs.String("app", "", fmt.Sprintf("App Store Connect app ID, bundle ID, or exact name: without --build-id, list the submissions of its %d most recently uploaded builds; with --build-id, require the build to be this app's", reviewSubmissionsAppBuildsLimit))
	shared.BindFlagAlias(fs, "app-id", "app")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight review submissions list (--build-id \"BUILD_ID\" | --app \"APP_ID\") [flags]",
		ShortHelp:  "List beta app review submissions.",
		LongHelp: `List beta app review submissions.

Apple lists beta app review submissions by build. --app lists them for the
app's most recently uploaded builds, newest first; next to --build-id it
checks that the build belongs to the app.

Examples:
  asc testflight review submissions list --build-id "BUILD_ID"
  asc testflight review submissions list --app "APP_ID"
  asc testflight review submissions list --build-id "BUILD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "testflight review submissions list", 200); err != nil {
				return err
			}
			buildValue := strings.TrimSpace(*buildID)
			appValue := strings.TrimSpace(*appFlag)
			if buildValue == "" && appValue == "" {
				return d.RequiredFlag("--build-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight review submissions list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			buildIDs := []string{buildValue}
			if appValue != "" {
				buildIDs, err = reviewSubmissionsAppBuildIDs(requestCtx, client, appValue, buildValue)
				if err != nil {
					return fmt.Errorf("testflight review submissions list: %w", err)
				}
				if len(buildIDs) == 0 {
					return d.PrintOutput(&asc.BetaAppReviewSubmissionsResponse{Data: []asc.Resource[asc.BetaAppReviewSubmissionAttributes]{}}, *output.Output, *output.Pretty)
				}
			}

			opts := []asc.BetaAppReviewSubmissionsOption{
				asc.Limit[asc.BetaAppReviewSubmissionsOption](*limit),
				asc.NextURL[asc.BetaAppReviewSubmissionsOption](*next),
			}

			opts = append(opts, asc.WithBetaAppReviewSubmissionsBuildIDs(buildIDs))

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.BetaAppReviewSubmissionsOption](200))
				resp, err := d.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetBetaAppReviewSubmissions(ctx, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetBetaAppReviewSubmissions(ctx, asc.NextURL[asc.BetaAppReviewSubmissionsOption](nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("testflight review submissions list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBetaAppReviewSubmissions(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("testflight review submissions list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// reviewSubmissionsAppBuildsLimit is how many of an app's most recent builds
// --app reads submissions for: one page of builds, whose IDs Apple accepts in
// one filter[build] value. At 200 IDs the URL is about 7.9 KB; Apple answers
// HTTP 500 once it passes about 8.3 KB (docs/API_NOTES.md), so raising this
// needs the filter split across requests.
const reviewSubmissionsAppBuildsLimit = 200

// reviewSubmissionsAppBuildIDs returns the build IDs whose submissions --app
// selects: buildID when it belongs to app, otherwise the app's most recently
// uploaded builds.
func reviewSubmissionsAppBuildIDs(ctx context.Context, client *asc.Client, app, buildID string) ([]string, error) {
	appID, err := shared.ResolveAppIDWithLookup(ctx, client, app)
	if err != nil {
		return nil, err
	}
	if buildID != "" {
		linkage, err := client.GetBuildAppRelationship(ctx, buildID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve app for build %q: %w", buildID, err)
		}
		if owner := strings.TrimSpace(linkage.Data.ID); owner != appID {
			mismatch := fmt.Errorf("build %q belongs to app %q, not %q", buildID, owner, appID)
			return nil, shared.WithDiagnostic(shared.NewValidationError(mismatch), shared.DiagnosticInvalidInput, "--build-id")
		}
		return []string{buildID}, nil
	}
	builds, err := client.GetBuilds(ctx, appID,
		asc.WithBuildsSort("-uploadedDate"),
		asc.Limit[asc.BuildsOption](reviewSubmissionsAppBuildsLimit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list builds for app %q: %w", appID, err)
	}
	ids := make([]string, 0, len(builds.Data))
	for _, build := range builds.Data {
		ids = append(ids, build.ID)
	}
	return ids, nil
}

// newTestFlightReviewSubmissionsGetCommand retrieves a beta app review submission by ID.
func newTestFlightReviewSubmissionsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "submissions view",
		Name:        "view",
		ShortUsage:  "asc testflight review submissions view --id \"SUBMISSION_ID\"",
		ShortHelp:   "View a beta app review submission by ID.",
		LongHelp: `View a beta app review submission by ID.

Examples:
  asc testflight review submissions view --id "SUBMISSION_ID"`,
		IDUsage:            "Beta app review submission ID",
		IDType:             "betaAppReviewSubmissions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBetaAppReviewSubmission(ctx, id)
		},
	})
}

// newTestFlightReviewSubmissionsBuildCommand retrieves the build for a beta app review submission.
func newTestFlightReviewSubmissionsBuildCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("submissions build", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "betaAppReviewSubmissions", "Beta app review submission ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "build",
		ShortUsage: "asc testflight review submissions build --id \"SUBMISSION_ID\"",
		ShortHelp:  "Get the build for a beta app review submission.",
		LongHelp: `Get the build for a beta app review submission.

Examples:
  asc testflight review submissions build --id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight review submissions build: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetBetaAppReviewSubmissionBuild(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("testflight review submissions build: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newTestFlightBetaDetailsCommand returns the testflight beta-details command with subcommands.
func newTestFlightBetaDetailsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("beta-details", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-details",
		ShortUsage: "asc testflight beta-details <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight build beta details.",
		LongHelp: `Manage TestFlight build beta details.

Examples:
  asc testflight beta-details view --build-id "BUILD_ID"
  asc testflight beta-details update --id "DETAIL_ID" --auto-notify`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newTestFlightBetaDetailsGetCommand(d),
			newTestFlightBetaDetailsBuildCommand(d),
			newTestFlightBetaDetailsUpdateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newTestFlightBetaDetailsGetCommand retrieves build beta details for a build.
func newTestFlightBetaDetailsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("view", flag.ExitOnError)

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID")
	shared.BindFlagAlias(fs, "build", "build-id")
	output := d.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight beta-details view [flags]",
		ShortHelp:  "View build beta details for a build.",
		LongHelp: `View build beta details for a build.

To find the build ID, list the app's builds and use the returned id field:
  asc builds list --app "APP_ID" --paginate --output json

Examples:
  asc testflight beta-details view --build-id "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "testflight beta-details view", 200); err != nil {
				return err
			}

			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--build-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight beta-details view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BuildBetaDetailsOption{
				asc.WithBuildBetaDetailsBuildIDs([]string{trimmedBuildID}),
				asc.Limit[asc.BuildBetaDetailsOption](*limit),
				asc.NextURL[asc.BuildBetaDetailsOption](*next),
			}

			details, err := client.GetBuildBetaDetails(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("testflight beta-details view: failed to fetch: %w", err)
			}

			return d.PrintOutput(details, *output.Output, *output.Pretty)
		},
	}
}

// newTestFlightBetaDetailsBuildCommand returns the beta-details build command group.
func newTestFlightBetaDetailsBuildCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("build", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "build",
		ShortUsage: "asc testflight beta-details build <subcommand> [flags]",
		ShortHelp:  "View the build for a build beta detail.",
		LongHelp: `View the build for a build beta detail.

Examples:
  asc testflight beta-details build view --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newTestFlightBetaDetailsBuildGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newTestFlightBetaDetailsBuildGetCommand retrieves the build for a build beta detail.
func newTestFlightBetaDetailsBuildGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "build view",
		Name:        "view",
		ShortUsage:  "asc testflight beta-details build view --id \"DETAIL_ID\"",
		ShortHelp:   "View the build for a build beta detail.",
		LongHelp: `View the build for a build beta detail.

Examples:
  asc testflight beta-details build view --id "DETAIL_ID"`,
		IDUsage:            "Build beta detail ID",
		IDType:             "buildBetaDetails",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBuildBetaDetailBuild(ctx, id)
		},
	})
}

// newTestFlightBetaDetailsUpdateCommand updates build beta details by ID.
func newTestFlightBetaDetailsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "buildBetaDetails", "Build beta detail ID")
	autoNotify := fs.Bool("auto-notify", false, "Enable auto-notify for external testers")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc testflight beta-details update [flags]",
		ShortHelp:  "Update build beta details.",
		LongHelp: `Update build beta details.

Examples:
  asc testflight beta-details update --id "DETAIL_ID" --auto-notify

External distribution is managed through group assignment:
  asc builds add-groups --build-id "BUILD_ID" --group "GROUP_ID" --submit --confirm
  asc builds remove-groups --build-id "BUILD_ID" --group "GROUP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			detailID := strings.TrimSpace(*id)
			if detailID == "" {
				return d.RequiredFlag("--id")
			}

			hasUpdates := visited["auto-notify"]
			if !hasUpdates {
				return d.UpdateFlagRequired(fs, autoNotify)
			}

			attrs := asc.BuildBetaDetailUpdateAttributes{}
			if visited["auto-notify"] {
				value := *autoNotify
				attrs.AutoNotifyEnabled = &value
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight beta-details update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			detail, err := client.UpdateBuildBetaDetail(requestCtx, detailID, attrs)
			if err != nil {
				return fmt.Errorf("testflight beta-details update: failed to update: %w", err)
			}

			return d.PrintOutput(detail, *output.Output, *output.Pretty)
		},
	}
}

// newTestFlightRecruitmentCommand returns the testflight recruitment command with subcommands.
func newTestFlightRecruitmentCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("recruitment", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "recruitment",
		ShortUsage: "asc testflight recruitment <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight recruitment criteria.",
		LongHelp: `Manage TestFlight recruitment criteria.

Examples:
  asc testflight recruitment options
  asc testflight recruitment set --group "GROUP_ID" --os-version-filter "IPHONE=26,IPAD=26"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newTestFlightRecruitmentOptionsCommand(d),
			newTestFlightRecruitmentSetCommand(d),
			newTestFlightRecruitmentDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newTestFlightRecruitmentDeleteCommand deletes recruitment criteria by ID.
func newTestFlightRecruitmentDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:       d,
		Name:       "delete",
		ShortUsage: "asc testflight recruitment delete --id \"CRITERIA_ID\" --confirm",
		ShortHelp:  "Delete recruitment criteria.",
		LongHelp: `Delete recruitment criteria.

Examples:
  asc testflight recruitment delete --id "CRITERIA_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Recruitment criteria ID",
		IDType:             "betaRecruitmentCriteria",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteBetaRecruitmentCriteria(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

// newTestFlightRecruitmentOptionsCommand lists recruitment criteria options.
func newTestFlightRecruitmentOptionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("options", flag.ExitOnError)

	output := d.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	fields := fs.String("fields", "deviceFamilyOsVersions", "Fields to include: deviceFamilyOsVersions")

	return &ffcli.Command{
		Name:       "options",
		ShortUsage: "asc testflight recruitment options [flags]",
		ShortHelp:  "List recruitment criteria options.",
		LongHelp: `List recruitment criteria options.

Examples:
  asc testflight recruitment options`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "testflight recruitment options", 200, "fields"); err != nil {
				return err
			}

			fieldsValue, err := normalizeBetaRecruitmentCriterionOptionsFields(*fields)
			if err != nil {
				return usageErrorFromValidation(ctx, d, "testflight recruitment options: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight recruitment options: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BetaRecruitmentCriterionOptionsOption{
				asc.WithBetaRecruitmentCriterionOptionsFields(fieldsValue),
				asc.Limit[asc.BetaRecruitmentCriterionOptionsOption](*limit),
				asc.NextURL[asc.BetaRecruitmentCriterionOptionsOption](*next),
			}

			options, err := client.GetBetaRecruitmentCriterionOptions(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("testflight recruitment options: failed to fetch: %w", err)
			}

			return d.PrintOutput(options, *output.Output, *output.Pretty)
		},
	}
}

// newTestFlightRecruitmentSetCommand creates recruitment criteria for a group.
func newTestFlightRecruitmentSetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("set", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group", "betaGroups", "Beta group ID")
	filters := fs.String("os-version-filter", "", "Device family OS filters (e.g., IPHONE=26,IPAD=26)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc testflight recruitment set --group GROUP_ID --os-version-filter FILTERS",
		ShortHelp:  "Set recruitment criteria for a group.",
		LongHelp: `Set recruitment criteria for a group.

Examples:
  asc testflight recruitment set --group "GROUP_ID" --os-version-filter "IPHONE=26,IPAD=26"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedGroupID := strings.TrimSpace(*groupID)
			if trimmedGroupID == "" {
				return d.RequiredFlag("--group")
			}

			filterValues, err := parseDeviceFamilyOsVersionFilters(*filters)
			if err != nil {
				return usageErrorFromValidation(ctx, d, "testflight recruitment set: %v", err)
			}
			if len(filterValues) == 0 {
				return d.RequiredFlag("--os-version-filter")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight recruitment set: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			existing, err := client.GetBetaGroupBetaRecruitmentCriteria(requestCtx, trimmedGroupID)
			if err == nil {
				criteriaID := strings.TrimSpace(existing.Data.ID)
				if validationErr := validateBetaRecruitmentCriteriaID(criteriaID); validationErr != nil {
					return validationErr
				}
				criteria, err := client.UpdateBetaRecruitmentCriteria(requestCtx, criteriaID, filterValues)
				if err != nil {
					return fmt.Errorf("testflight recruitment set: failed to update: %w", err)
				}
				return d.PrintOutput(criteria, *output.Output, *output.Pretty)
			} else if !asc.IsNotFound(err) {
				return fmt.Errorf("testflight recruitment set: failed to fetch existing criteria: %w", err)
			}

			criteria, createErr := client.CreateBetaRecruitmentCriteria(requestCtx, trimmedGroupID, filterValues)
			if createErr != nil {
				return fmt.Errorf("testflight recruitment set: failed to set: %w", createErr)
			}

			return d.PrintOutput(criteria, *output.Output, *output.Pretty)
		},
	}
}

func normalizeBetaRecruitmentCriterionOptionsFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{
		"deviceFamilyOsVersions": {},
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, shared.WithDiagnostic(
				shared.NewValidationError(fmt.Errorf("--fields must be one of: deviceFamilyOsVersions")),
				shared.DiagnosticInvalidInput,
				"--fields",
			)
		}
	}
	return fields, nil
}

func validateBetaRecruitmentCriteriaID(value string) error {
	if strings.TrimSpace(value) == "" {
		return shared.WithDiagnostic(
			shared.NewValidationError(fmt.Errorf("testflight recruitment set: criteria id is empty")),
			shared.DiagnosticStateNotReady,
			"--group",
		)
	}
	return nil
}

func parseDeviceFamilyOsVersionFilters(value string) ([]asc.DeviceFamilyOsVersionFilter, error) {
	entries := shared.SplitCSV(value)
	if len(entries) == 0 {
		return nil, nil
	}

	filters := make([]asc.DeviceFamilyOsVersionFilter, 0, len(entries))
	for _, entry := range entries {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			return nil, shared.WithDiagnostic(
				shared.NewValidationError(fmt.Errorf("--os-version-filter must use DEVICE_FAMILY=MIN_OS (e.g., IPHONE=26)")),
				shared.DiagnosticInvalidInput,
				"--os-version-filter",
			)
		}
		familyValue := strings.TrimSpace(parts[0])
		versionValue := strings.TrimSpace(parts[1])
		if familyValue == "" || versionValue == "" {
			return nil, shared.WithDiagnostic(
				shared.NewValidationError(fmt.Errorf("--os-version-filter must use DEVICE_FAMILY=MIN_OS (e.g., IPHONE=26)")),
				shared.DiagnosticInvalidInput,
				"--os-version-filter",
			)
		}

		family, err := normalizeBetaRecruitmentDeviceFamily(familyValue)
		if err != nil {
			return nil, err
		}

		minVersion := versionValue
		maxVersion := ""
		if strings.Contains(versionValue, "..") {
			rangeParts := strings.SplitN(versionValue, "..", 2)
			minVersion = strings.TrimSpace(rangeParts[0])
			maxVersion = strings.TrimSpace(rangeParts[1])
			if minVersion == "" || maxVersion == "" {
				return nil, shared.WithDiagnostic(
					shared.NewValidationError(fmt.Errorf("--os-version-filter must use DEVICE_FAMILY=MIN_OS[..MAX_OS]")),
					shared.DiagnosticInvalidInput,
					"--os-version-filter",
				)
			}
		}

		filters = append(filters, asc.DeviceFamilyOsVersionFilter{
			DeviceFamily:       family,
			MinimumOsInclusive: minVersion,
			MaximumOsInclusive: maxVersion,
		})
	}

	return filters, nil
}

func normalizeBetaRecruitmentDeviceFamily(value string) (asc.DeviceFamily, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if slices.Contains(betaRecruitmentDeviceFamilyList(), normalized) {
		return asc.DeviceFamily(normalized), nil
	}
	return "", shared.WithDiagnostic(
		shared.NewValidationError(fmt.Errorf("--os-version-filter device family must be one of: %s", strings.Join(betaRecruitmentDeviceFamilyList(), ", "))),
		shared.DiagnosticInvalidInput,
		"--os-version-filter",
	)
}

func betaRecruitmentDeviceFamilyList() []string {
	return []string{
		string(asc.DeviceFamilyIPhone),
		string(asc.DeviceFamilyIPad),
		string(asc.DeviceFamilyMac),
		string(asc.DeviceFamilyVision),
		string(asc.DeviceFamilyAppleTV),
		string(asc.DeviceFamilyAppleWatch),
	}
}

// newTestFlightMetricsCommand returns the testflight metrics command with subcommands.
func newTestFlightMetricsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("metrics", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "metrics",
		ShortUsage: "asc testflight metrics <subcommand> [flags]",
		ShortHelp:  "Fetch TestFlight metrics.",
		LongHelp: `Fetch TestFlight metrics.

Examples:
  asc testflight metrics public-link --group "GROUP_ID"
  asc testflight metrics testers --group "GROUP_ID"
  asc testflight metrics beta-tester-usages --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newTestFlightMetricsPublicLinkCommand(d),
			newTestFlightMetricsTestersCommand(d),
			newTestFlightMetricsBetaTesterUsagesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newTestFlightMetricsPublicLinkCommand fetches public link usage metrics.
func newTestFlightMetricsPublicLinkCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("public-link", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group", "betaGroups", "Beta group ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "public-link",
		ShortUsage: "asc testflight metrics public-link --group GROUP_ID",
		ShortHelp:  "Fetch TestFlight public link usage metrics.",
		LongHelp: `Fetch TestFlight public link usage metrics.

Examples:
  asc testflight metrics public-link --group "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedGroupID := strings.TrimSpace(*groupID)
			if trimmedGroupID == "" {
				return d.RequiredFlag("--group")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight metrics public-link: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			metrics, err := client.GetBetaGroupPublicLinkUsages(requestCtx, trimmedGroupID)
			if err != nil {
				return fmt.Errorf("testflight metrics public-link: failed to fetch: %w", err)
			}

			return d.PrintOutput(metrics, *output.Output, *output.Pretty)
		},
	}
}

// newTestFlightMetricsTestersCommand fetches beta tester usage metrics.
func newTestFlightMetricsTestersCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("testers", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group", "betaGroups", "Beta group ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "testers",
		ShortUsage: "asc testflight metrics testers --group GROUP_ID",
		ShortHelp:  "Fetch TestFlight beta tester usage metrics.",
		LongHelp: `Fetch TestFlight beta tester usage metrics.

Examples:
  asc testflight metrics testers --group "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedGroupID := strings.TrimSpace(*groupID)
			if trimmedGroupID == "" {
				return d.RequiredFlag("--group")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight metrics testers: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			metrics, err := client.GetBetaGroupTesterUsages(requestCtx, trimmedGroupID)
			if err != nil {
				return fmt.Errorf("testflight metrics testers: failed to fetch: %w", err)
			}

			return d.PrintOutput(metrics, *output.Output, *output.Pretty)
		},
	}
}
