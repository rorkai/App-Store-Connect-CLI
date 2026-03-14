package testflight

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// TestFlightCommand returns the testflight command with subcommands.
func TestFlightCommand() *ffcli.Command {
	fs := flag.NewFlagSet("testflight", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "testflight",
		ShortUsage: "asc testflight <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight workflows.",
		LongHelp: `Manage TestFlight workflows.

Examples:
  asc testflight groups list --app "APP_ID"
  asc testflight testers list --app "APP_ID"
  asc testflight feedback list --app "APP_ID"
  asc testflight crashes view --submission-id "SUBMISSION_ID"
  asc testflight crashes log --submission-id "SUBMISSION_ID"
  asc testflight review view --app "APP_ID"
  asc testflight distribution view --build "BUILD_ID"
  asc testflight metrics group-testers --group "GROUP_ID"
  asc testflight metrics app-testers --app "APP_ID"
  asc testflight agreements view --app "APP_ID"
  asc testflight notifications send --build "BUILD_ID"
  asc testflight config export --app "APP_ID" --output "./testflight.yaml"
  asc testflight app-localizations list --app "APP_ID"
  asc testflight pre-release list --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: testflightVisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			RemovedTestFlightAppsCommand(),
			TestFlightGroupsCommand(),
			TestFlightTestersCommand(),
			TestFlightFeedbackCommand(),
			TestFlightCrashesCommand(),
			TestFlightAgreementsCommand(),
			TestFlightNotificationsCommand(),
			TestFlightReviewSurfaceCommand(),
			TestFlightDistributionCommand(),
			TestFlightRecruitmentCommand(),
			TestFlightMetricsSurfaceCommand(),
			TestFlightConfigCommand(),
			TestFlightAppLocalizationsCommand(),
			TestFlightPreReleaseCommand(),
			DeprecatedBetaGroupsAliasCommand(),
			DeprecatedBetaTestersAliasCommand(),
			DeprecatedBetaFeedbackAliasCommand(),
			DeprecatedBetaCrashLogsAliasCommand(),
			DeprecatedBetaDetailsAliasCommand(),
			DeprecatedBetaLicenseAgreementsAliasCommand(),
			DeprecatedBetaNotificationsAliasCommand(),
			DeprecatedTestFlightSyncAliasCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
