package validate

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type validateTestFlightOptions struct {
	AppID   string
	BuildID string
	Strict  bool
	Output  string
	Pretty  bool
}

// ValidateTestFlightCommand returns the asc validate testflight subcommand.
func ValidateTestFlightCommand() *ffcli.Command {
	fs := flag.NewFlagSet("testflight", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	buildID := fs.String("build", "", "Build ID (required)")
	strict := fs.Bool("strict", false, "Treat warnings as errors (exit non-zero)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "testflight",
		ShortUsage: "asc validate testflight --app \"APP_ID\" --build \"BUILD_ID\" [flags]",
		ShortHelp:  "Validate TestFlight build readiness before distribution.",
		LongHelp: `Validate TestFlight readiness for a build.

Checks:
  - Build exists and has finished processing
  - Beta app review details completeness
  - "What to Test" notes present for at least one localization

Examples:
  asc validate testflight --app "APP_ID" --build "BUILD_ID"
  asc validate testflight --app "APP_ID" --build "BUILD_ID" --output table
  asc validate testflight --app "APP_ID" --build "BUILD_ID" --strict`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildValue := strings.TrimSpace(*buildID)
			if buildValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return shared.MissingRequiredUsageError()
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}

			return runValidateTestFlight(ctx, validateTestFlightOptions{
				AppID:   resolvedAppID,
				BuildID: buildValue,
				Strict:  *strict,
				Output:  *output.Output,
				Pretty:  *output.Pretty,
			})
		},
	}
}

func runValidateTestFlight(ctx context.Context, opts validateTestFlightOptions) error {
	client, err := clientFactory()
	if err != nil {
		return fmt.Errorf("validate testflight: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	appResp, err := client.GetApp(requestCtx, opts.AppID)
	if err != nil {
		return fmt.Errorf("validate testflight: failed to fetch app: %w", err)
	}

	var build *validation.Build
	buildResp, err := client.GetBuild(requestCtx, opts.BuildID)
	if err != nil {
		if !asc.IsNotFound(err) {
			return fmt.Errorf("validate testflight: failed to fetch build: %w", err)
		}
	} else {
		attrs := buildResp.Data.Attributes
		build = &validation.Build{
			ID:              buildResp.Data.ID,
			Version:         attrs.Version,
			ProcessingState: attrs.ProcessingState,
			Expired:         attrs.Expired,
		}
	}

	buildAppID := ""
	if build != nil {
		buildAppResp, err := client.GetBuildApp(requestCtx, build.ID)
		if err != nil {
			return fmt.Errorf("validate testflight: failed to fetch build app: %w", err)
		}
		buildAppID = buildAppResp.Data.ID
	}

	var betaReviewDetails *validation.BetaReviewDetails
	betaReviewResp, err := client.GetAppBetaAppReviewDetail(requestCtx, opts.AppID)
	if err != nil {
		if !asc.IsNotFound(err) {
			return fmt.Errorf("validate testflight: failed to fetch beta app review details: %w", err)
		}
	} else {
		attrs := betaReviewResp.Data.Attributes
		betaReviewDetails = &validation.BetaReviewDetails{
			ID:                  betaReviewResp.Data.ID,
			ContactFirstName:    attrs.ContactFirstName,
			ContactLastName:     attrs.ContactLastName,
			ContactEmail:        attrs.ContactEmail,
			ContactPhone:        attrs.ContactPhone,
			DemoAccountName:     attrs.DemoAccountName,
			DemoAccountPassword: attrs.DemoAccountPassword,
			DemoAccountRequired: attrs.DemoAccountRequired,
			Notes:               attrs.Notes,
		}
	}

	var betaBuildLocalizations []validation.BetaBuildLocalization
	if build != nil {
		locsResp, err := client.GetBetaBuildLocalizations(requestCtx, build.ID, asc.WithBetaBuildLocalizationsLimit(200))
		if err != nil {
			return fmt.Errorf("validate testflight: failed to fetch beta build localizations: %w", err)
		}
		betaBuildLocalizations = make([]validation.BetaBuildLocalization, 0, len(locsResp.Data))
		for _, loc := range locsResp.Data {
			attrs := loc.Attributes
			betaBuildLocalizations = append(betaBuildLocalizations, validation.BetaBuildLocalization{
				ID:       loc.ID,
				Locale:   attrs.Locale,
				WhatsNew: attrs.WhatsNew,
			})
		}
	}

	report := validation.ValidateTestFlight(validation.TestFlightInput{
		AppID:                  opts.AppID,
		AppPrimaryLocale:       appResp.Data.Attributes.PrimaryLocale,
		BuildID:                opts.BuildID,
		Build:                  build,
		BuildAppID:             buildAppID,
		BetaReviewDetails:      betaReviewDetails,
		BetaBuildLocalizations: betaBuildLocalizations,
	}, opts.Strict)

	if err := shared.PrintOutput(&report, opts.Output, opts.Pretty); err != nil {
		return err
	}

	if report.Summary.Blocking > 0 {
		return shared.NewReportedError(fmt.Errorf("validate testflight: found %d blocking issue(s)", report.Summary.Blocking))
	}

	return nil
}
