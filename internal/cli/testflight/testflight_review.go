package testflight

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// TestFlightReviewCommand returns the testflight review command with subcommands.
func TestFlightReviewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review",
		ShortUsage: "asc testflight review <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight beta app review details.",
		LongHelp: `Manage TestFlight beta app review details and submissions.

Examples:
  asc testflight review get --app "APP_ID"
  asc testflight review update --id "DETAIL_ID" --contact-email "dev@example.com"
  asc testflight review submit --build "BUILD_ID" --confirm
  asc testflight review app get --id "DETAIL_ID"
  asc testflight review submissions list --build "BUILD_ID"
  asc testflight review submissions get --id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			TestFlightReviewGetCommand(),
			TestFlightReviewAppCommand(),
			TestFlightReviewUpdateCommand(),
			TestFlightReviewSubmitCommand(),
			TestFlightReviewSubmissionsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// TestFlightReviewGetCommand retrieves beta app review details for an app.
func TestFlightReviewGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := shared.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight review get [flags]",
		ShortHelp:  "Fetch beta app review details for an app.",
		LongHelp: `Fetch beta app review details for an app.

Examples:
  asc testflight review get --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("testflight review get: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("testflight review get: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight review get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaAppReviewDetailsOption{
				asc.WithBetaAppReviewDetailsLimit(*limit),
				asc.WithBetaAppReviewDetailsNextURL(*next),
			}

			details, err := client.GetBetaAppReviewDetails(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("testflight review get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(details, *output.Output, *output.Pretty)
		},
	}
}

// TestFlightReviewUpdateCommand updates beta app review details.
func TestFlightReviewUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "Beta app review detail ID")
	contactFirstName := fs.String("contact-first-name", "", "Contact first name")
	contactLastName := fs.String("contact-last-name", "", "Contact last name")
	contactEmail := fs.String("contact-email", "", "Contact email")
	contactPhone := fs.String("contact-phone", "", "Contact phone")
	demoAccountName := fs.String("demo-account-name", "", "Demo account name")
	demoAccountPassword := fs.String("demo-account-password", "", "Demo account password")
	demoAccountRequired := fs.Bool("demo-account-required", false, "Demo account required")
	notes := fs.String("notes", "", "Review notes")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc testflight review update [flags]",
		ShortHelp:  "Update beta app review details.",
		LongHelp: `Update beta app review details.

Examples:
  asc testflight review update --id "DETAIL_ID" --contact-email "dev@example.com"
  asc testflight review update --id "DETAIL_ID" --notes "Updated review notes"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			detailID := strings.TrimSpace(*id)
			if detailID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
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
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
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

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight review update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			detail, err := client.UpdateBetaAppReviewDetail(requestCtx, detailID, attrs)
			if err != nil {
				return fmt.Errorf("testflight review update: failed to update: %w", err)
			}

			return shared.PrintOutput(detail, *output.Output, *output.Pretty)
		},
	}
}

// TestFlightReviewSubmitCommand submits a build for beta app review.
func TestFlightReviewSubmitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	confirm := fs.Bool("confirm", false, "Confirm submission")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc testflight review submit --build BUILD_ID --confirm",
		ShortHelp:  "Submit a build for beta app review.",
		LongHelp: `Submit a build for beta app review.

Examples:
  asc testflight review submit --build "BUILD_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*buildID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight review submit: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			submission, err := client.CreateBetaAppReviewSubmission(requestCtx, strings.TrimSpace(*buildID))
			if err != nil {
				return fmt.Errorf("testflight review submit: failed to submit: %w", err)
			}

			return shared.PrintOutput(submission, *output.Output, *output.Pretty)
		},
	}
}

// TestFlightReviewAppCommand returns the review app command group.
func TestFlightReviewAppCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc testflight review app <subcommand> [flags]",
		ShortHelp:  "View the app for a beta app review detail.",
		LongHelp: `View the app for a beta app review detail.

Examples:
  asc testflight review app get --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			TestFlightReviewAppGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// TestFlightReviewAppGetCommand retrieves the app for a beta app review detail.
func TestFlightReviewAppGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app get", flag.ExitOnError)

	id := fs.String("id", "", "Beta app review detail ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight review app get --id \"DETAIL_ID\"",
		ShortHelp:  "Get the app for a beta app review detail.",
		LongHelp: `Get the app for a beta app review detail.

Examples:
  asc testflight review app get --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight review app get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaAppReviewDetailApp(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("testflight review app get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// TestFlightReviewSubmissionsCommand returns the review submissions command group.
func TestFlightReviewSubmissionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "submissions",
		ShortUsage: "asc testflight review submissions <subcommand> [flags]",
		ShortHelp:  "View beta app review submissions.",
		LongHelp: `View beta app review submissions.

Examples:
  asc testflight review submissions get --id "SUBMISSION_ID"
  asc testflight review submissions build --id "SUBMISSION_ID"
  asc testflight review submissions list --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			TestFlightReviewSubmissionsListCommand(),
			TestFlightReviewSubmissionsGetCommand(),
			TestFlightReviewSubmissionsBuildCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// TestFlightReviewSubmissionsListCommand lists beta app review submissions.
func TestFlightReviewSubmissionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions list", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID to filter")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight review submissions list --build \"BUILD_ID\" [flags]",
		ShortHelp:  "List beta app review submissions.",
		LongHelp: `List beta app review submissions.

Examples:
  asc testflight review submissions list --build "BUILD_ID"
  asc testflight review submissions list --build "BUILD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildValue := strings.TrimSpace(*buildID)
			if buildValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("testflight review submissions list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("testflight review submissions list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight review submissions list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaAppReviewSubmissionsOption{
				asc.WithBetaAppReviewSubmissionsLimit(*limit),
				asc.WithBetaAppReviewSubmissionsNextURL(*next),
			}

			opts = append(opts, asc.WithBetaAppReviewSubmissionsBuildIDs([]string{buildValue}))

			if *paginate {
				paginateOpts := append(opts, asc.WithBetaAppReviewSubmissionsLimit(200))
				firstPage, err := client.GetBetaAppReviewSubmissions(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("testflight review submissions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBetaAppReviewSubmissions(ctx, asc.WithBetaAppReviewSubmissionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("testflight review submissions list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBetaAppReviewSubmissions(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("testflight review submissions list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// TestFlightReviewSubmissionsGetCommand retrieves a beta app review submission by ID.
func TestFlightReviewSubmissionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions get", flag.ExitOnError)

	id := fs.String("id", "", "Beta app review submission ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight review submissions get --id \"SUBMISSION_ID\"",
		ShortHelp:  "Get a beta app review submission by ID.",
		LongHelp: `Get a beta app review submission by ID.

Examples:
  asc testflight review submissions get --id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight review submissions get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaAppReviewSubmission(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("testflight review submissions get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// TestFlightReviewSubmissionsBuildCommand retrieves the build for a beta app review submission.
func TestFlightReviewSubmissionsBuildCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions build", flag.ExitOnError)

	id := fs.String("id", "", "Beta app review submission ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "build",
		ShortUsage: "asc testflight review submissions build --id \"SUBMISSION_ID\"",
		ShortHelp:  "Get the build for a beta app review submission.",
		LongHelp: `Get the build for a beta app review submission.

Examples:
  asc testflight review submissions build --id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight review submissions build: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaAppReviewSubmissionBuild(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("testflight review submissions build: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// TestFlightBetaDetailsCommand returns the testflight beta-details command with subcommands.
func TestFlightBetaDetailsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-details", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-details",
		ShortUsage: "asc testflight beta-details <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight build beta details.",
		LongHelp: `Manage TestFlight build beta details.

Examples:
  asc testflight beta-details get --build "BUILD_ID"
  asc testflight beta-details update --id "DETAIL_ID" --auto-notify`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			TestFlightBetaDetailsGetCommand(),
			TestFlightBetaDetailsBuildCommand(),
			TestFlightBetaDetailsUpdateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// TestFlightBetaDetailsGetCommand retrieves build beta details for a build.
func TestFlightBetaDetailsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	output := shared.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight beta-details get [flags]",
		ShortHelp:  "Fetch build beta details for a build.",
		LongHelp: `Fetch build beta details for a build.

Examples:
  asc testflight beta-details get --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("testflight beta-details get: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("testflight beta-details get: %w", err)
			}

			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-details get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BuildBetaDetailsOption{
				asc.WithBuildBetaDetailsBuildIDs([]string{trimmedBuildID}),
				asc.WithBuildBetaDetailsLimit(*limit),
				asc.WithBuildBetaDetailsNextURL(*next),
			}

			details, err := client.GetBuildBetaDetails(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("testflight beta-details get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(details, *output.Output, *output.Pretty)
		},
	}
}

// TestFlightBetaDetailsBuildCommand returns the beta-details build command group.
func TestFlightBetaDetailsBuildCommand() *ffcli.Command {
	fs := flag.NewFlagSet("build", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "build",
		ShortUsage: "asc testflight beta-details build <subcommand> [flags]",
		ShortHelp:  "View the build for a build beta detail.",
		LongHelp: `View the build for a build beta detail.

Examples:
  asc testflight beta-details build get --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			TestFlightBetaDetailsBuildGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// TestFlightBetaDetailsBuildGetCommand retrieves the build for a build beta detail.
func TestFlightBetaDetailsBuildGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("build get", flag.ExitOnError)

	id := fs.String("id", "", "Build beta detail ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight beta-details build get --id \"DETAIL_ID\"",
		ShortHelp:  "Get the build for a build beta detail.",
		LongHelp: `Get the build for a build beta detail.

Examples:
  asc testflight beta-details build get --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-details build get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBuildBetaDetailBuild(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("testflight beta-details build get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// TestFlightBetaDetailsUpdateCommand updates build beta details by ID.
func TestFlightBetaDetailsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	const (
		externalStateEnabled  = "READY_FOR_TESTING"
		externalStateDisabled = "NOT_READY_FOR_TESTING"
	)

	id := fs.String("id", "", "Build beta detail ID")
	autoNotify := fs.Bool("auto-notify", false, "Enable auto-notify for external testers")
	externalTesting := fs.Bool("external-testing", false, "Enable external testing (maps to externalBuildState)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc testflight beta-details update [flags]",
		ShortHelp:  "Update build beta details.",
		LongHelp: `Update build beta details.

Examples:
  asc testflight beta-details update --id "DETAIL_ID" --auto-notify
  asc testflight beta-details update --id "DETAIL_ID" --external-testing true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			detailID := strings.TrimSpace(*id)
			if detailID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			hasUpdates := visited["auto-notify"] || visited["external-testing"]
			if !hasUpdates {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			attrs := asc.BuildBetaDetailUpdateAttributes{}
			if visited["auto-notify"] {
				value := *autoNotify
				attrs.AutoNotifyEnabled = &value
			}
			if visited["external-testing"] {
				state := externalStateDisabled
				if *externalTesting {
					state = externalStateEnabled
				}
				attrs.ExternalBuildState = &state
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-details update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			detail, err := client.UpdateBuildBetaDetail(requestCtx, detailID, attrs)
			if err != nil {
				return fmt.Errorf("testflight beta-details update: failed to update: %w", err)
			}

			return shared.PrintOutput(detail, *output.Output, *output.Pretty)
		},
	}
}

// TestFlightRecruitmentCommand returns the testflight recruitment command with subcommands.
func TestFlightRecruitmentCommand() *ffcli.Command {
	fs := flag.NewFlagSet("recruitment", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "recruitment",
		ShortUsage: "asc testflight recruitment <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight recruitment criteria.",
		LongHelp: `Manage TestFlight recruitment criteria.

Examples:
  asc testflight recruitment options
  asc testflight recruitment set --group "GROUP_ID" --os-version-filter "IPHONE=26,IPAD=26"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			TestFlightRecruitmentOptionsCommand(),
			TestFlightRecruitmentSetCommand(),
			TestFlightRecruitmentDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// TestFlightRecruitmentDeleteCommand deletes beta recruitment criteria by ID.
func TestFlightRecruitmentDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Beta recruitment criteria ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc testflight recruitment delete --id \"CRITERIA_ID\" --confirm",
		ShortHelp:  "Delete beta recruitment criteria.",
		LongHelp: `Delete beta recruitment criteria.

Examples:
  asc testflight recruitment delete --id "CRITERIA_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			criteriaID := strings.TrimSpace(*id)
			if criteriaID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight recruitment delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBetaRecruitmentCriteria(requestCtx, criteriaID); err != nil {
				return fmt.Errorf("testflight recruitment delete: failed to delete: %w", err)
			}

			result := &asc.BetaRecruitmentCriteriaDeleteResult{
				ID:      criteriaID,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// TestFlightRecruitmentOptionsCommand lists recruitment criteria options.
func TestFlightRecruitmentOptionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("options", flag.ExitOnError)

	output := shared.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	fields := fs.String("fields", "deviceFamilyOsVersions", "Fields to include: deviceFamilyOsVersions")

	return &ffcli.Command{
		Name:       "options",
		ShortUsage: "asc testflight recruitment options [flags]",
		ShortHelp:  "List beta recruitment criteria options.",
		LongHelp: `List beta recruitment criteria options.

Examples:
  asc testflight recruitment options`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("testflight recruitment options: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("testflight recruitment options: %w", err)
			}

			fieldsValue, err := normalizeBetaRecruitmentCriterionOptionsFields(*fields)
			if err != nil {
				return fmt.Errorf("testflight recruitment options: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight recruitment options: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaRecruitmentCriterionOptionsOption{
				asc.WithBetaRecruitmentCriterionOptionsFields(fieldsValue),
				asc.WithBetaRecruitmentCriterionOptionsLimit(*limit),
				asc.WithBetaRecruitmentCriterionOptionsNextURL(*next),
			}

			options, err := client.GetBetaRecruitmentCriterionOptions(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("testflight recruitment options: failed to fetch: %w", err)
			}

			return shared.PrintOutput(options, *output.Output, *output.Pretty)
		},
	}
}

// TestFlightRecruitmentSetCommand creates beta recruitment criteria for a group.
func TestFlightRecruitmentSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("set", flag.ExitOnError)

	groupID := fs.String("group", "", "Beta group ID")
	filters := fs.String("os-version-filter", "", "Device family OS filters (e.g., IPHONE=26,IPAD=26)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc testflight recruitment set --group GROUP_ID --os-version-filter FILTERS",
		ShortHelp:  "Set beta recruitment criteria for a group.",
		LongHelp: `Set beta recruitment criteria for a group.

Examples:
  asc testflight recruitment set --group "GROUP_ID" --os-version-filter "IPHONE=26,IPAD=26"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedGroupID := strings.TrimSpace(*groupID)
			if trimmedGroupID == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return flag.ErrHelp
			}

			filterValues, err := parseDeviceFamilyOsVersionFilters(*filters)
			if err != nil {
				return fmt.Errorf("testflight recruitment set: %w", err)
			}
			if len(filterValues) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --os-version-filter is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight recruitment set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			existing, err := client.GetBetaGroupBetaRecruitmentCriteria(requestCtx, trimmedGroupID)
			if err == nil {
				criteriaID := strings.TrimSpace(existing.Data.ID)
				if criteriaID == "" {
					return fmt.Errorf("testflight recruitment set: criteria id is empty")
				}
				criteria, err := client.UpdateBetaRecruitmentCriteria(requestCtx, criteriaID, filterValues)
				if err != nil {
					return fmt.Errorf("testflight recruitment set: failed to update: %w", err)
				}
				return shared.PrintOutput(criteria, *output.Output, *output.Pretty)
			} else if !asc.IsNotFound(err) {
				return fmt.Errorf("testflight recruitment set: failed to fetch existing criteria: %w", err)
			}

			criteria, createErr := client.CreateBetaRecruitmentCriteria(requestCtx, trimmedGroupID, filterValues)
			if createErr != nil {
				return fmt.Errorf("testflight recruitment set: failed to set: %w", createErr)
			}

			return shared.PrintOutput(criteria, *output.Output, *output.Pretty)
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
			return nil, fmt.Errorf("--fields must be one of: deviceFamilyOsVersions")
		}
	}
	return fields, nil
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
			return nil, fmt.Errorf("--os-version-filter must use DEVICE_FAMILY=MIN_OS (e.g., IPHONE=26)")
		}
		familyValue := strings.TrimSpace(parts[0])
		versionValue := strings.TrimSpace(parts[1])
		if familyValue == "" || versionValue == "" {
			return nil, fmt.Errorf("--os-version-filter must use DEVICE_FAMILY=MIN_OS (e.g., IPHONE=26)")
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
				return nil, fmt.Errorf("--os-version-filter must use DEVICE_FAMILY=MIN_OS[..MAX_OS]")
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
	return "", fmt.Errorf("--os-version-filter device family must be one of: %s", strings.Join(betaRecruitmentDeviceFamilyList(), ", "))
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

// TestFlightMetricsCommand returns the testflight metrics command with subcommands.
func TestFlightMetricsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metrics", flag.ExitOnError)

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
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			TestFlightMetricsPublicLinkCommand(),
			TestFlightMetricsTestersCommand(),
			TestFlightMetricsBetaTesterUsagesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// TestFlightMetricsPublicLinkCommand fetches public link usage metrics.
func TestFlightMetricsPublicLinkCommand() *ffcli.Command {
	fs := flag.NewFlagSet("public-link", flag.ExitOnError)

	groupID := fs.String("group", "", "Beta group ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "public-link",
		ShortUsage: "asc testflight metrics public-link --group GROUP_ID",
		ShortHelp:  "Fetch TestFlight public link usage metrics.",
		LongHelp: `Fetch TestFlight public link usage metrics.

Examples:
  asc testflight metrics public-link --group "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedGroupID := strings.TrimSpace(*groupID)
			if trimmedGroupID == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight metrics public-link: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			metrics, err := client.GetBetaGroupPublicLinkUsages(requestCtx, trimmedGroupID)
			if err != nil {
				return fmt.Errorf("testflight metrics public-link: failed to fetch: %w", err)
			}

			return shared.PrintOutput(metrics, *output.Output, *output.Pretty)
		},
	}
}

// TestFlightMetricsTestersCommand fetches beta tester usage metrics.
func TestFlightMetricsTestersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("testers", flag.ExitOnError)

	groupID := fs.String("group", "", "Beta group ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "testers",
		ShortUsage: "asc testflight metrics testers --group GROUP_ID",
		ShortHelp:  "Fetch TestFlight beta tester usage metrics.",
		LongHelp: `Fetch TestFlight beta tester usage metrics.

Examples:
  asc testflight metrics testers --group "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedGroupID := strings.TrimSpace(*groupID)
			if trimmedGroupID == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight metrics testers: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			metrics, err := client.GetBetaGroupTesterUsages(requestCtx, trimmedGroupID)
			if err != nil {
				return fmt.Errorf("testflight metrics testers: failed to fetch: %w", err)
			}

			return shared.PrintOutput(metrics, *output.Output, *output.Pretty)
		},
	}
}
