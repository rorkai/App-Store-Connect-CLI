package testflight

import (
	"context"
	"errors"
	"flag"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/betaapplocalizations"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/prerelease"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type textReplacement struct {
	old string
	new string
}

type rewrittenCommandError struct {
	message string
	err     error
}

func (e rewrittenCommandError) Error() string {
	return e.message
}

func (e rewrittenCommandError) Unwrap() error {
	return e.err
}

func rewriteCommandTree(cmd *ffcli.Command, oldRootPath, newRootPath string, nameRenames map[string]string, textReplacements []textReplacement) *ffcli.Command {
	if cmd == nil {
		return nil
	}

	pathReplacements := collectCommandPathReplacements(cmd, oldRootPath, newRootPath, nameRenames)
	replacements := append(pathReplacements, textReplacements...)
	sortTextReplacements(replacements)

	renameCommandNames(cmd, nameRenames)
	rewriteCommandStrings(cmd, replacements)
	rewriteCommandErrors(cmd, replacements)
	return cmd
}

func collectCommandPathReplacements(cmd *ffcli.Command, oldRootPath, newRootPath string, nameRenames map[string]string) []textReplacement {
	replacements := []textReplacement{}

	var walk func(current *ffcli.Command, oldPath, newPath string)
	walk = func(current *ffcli.Command, oldPath, newPath string) {
		replacements = append(replacements, textReplacement{old: oldPath, new: newPath})
		if strings.HasPrefix(oldPath, "asc testflight ") && strings.HasPrefix(newPath, "asc testflight ") {
			replacements = append(replacements, textReplacement{
				old: strings.TrimPrefix(oldPath, "asc testflight "),
				new: strings.TrimPrefix(newPath, "asc testflight "),
			})
		}

		for _, sub := range current.Subcommands {
			oldChildName := sub.Name
			newChildName := oldChildName
			if renamed, ok := nameRenames[oldChildName]; ok {
				newChildName = renamed
			}
			walk(sub, oldPath+" "+oldChildName, newPath+" "+newChildName)
		}
	}

	walk(cmd, oldRootPath, newRootPath)
	sortTextReplacements(replacements)
	return replacements
}

func sortTextReplacements(replacements []textReplacement) {
	sort.SliceStable(replacements, func(i, j int) bool {
		return len(replacements[i].old) > len(replacements[j].old)
	})
}

func renameCommandNames(cmd *ffcli.Command, nameRenames map[string]string) {
	if cmd == nil {
		return
	}

	if renamed, ok := nameRenames[cmd.Name]; ok {
		cmd.Name = renamed
		if cmd.FlagSet != nil {
			cmd.FlagSet.Init(renamed, cmd.FlagSet.ErrorHandling())
		}
	}
	for _, sub := range cmd.Subcommands {
		renameCommandNames(sub, nameRenames)
	}
}

func rewriteCommandStrings(cmd *ffcli.Command, replacements []textReplacement) {
	if cmd == nil {
		return
	}

	if cmd.ShortUsage != "" {
		cmd.ShortUsage = applyTextReplacements(cmd.ShortUsage, replacements)
	}
	if cmd.ShortHelp != "" {
		cmd.ShortHelp = applyTextReplacements(cmd.ShortHelp, replacements)
	}
	if cmd.LongHelp != "" {
		cmd.LongHelp = applyTextReplacements(cmd.LongHelp, replacements)
	}
	if cmd.FlagSet != nil {
		cmd.FlagSet.VisitAll(func(f *flag.Flag) {
			f.Usage = applyTextReplacements(f.Usage, replacements)
		})
	}
	for _, sub := range cmd.Subcommands {
		rewriteCommandStrings(sub, replacements)
	}
}

func rewriteCommandErrors(cmd *ffcli.Command, replacements []textReplacement) {
	if cmd == nil {
		return
	}

	if cmd.Exec != nil {
		originalExec := cmd.Exec
		cmd.Exec = func(ctx context.Context, args []string) error {
			err := originalExec(ctx, args)
			if err == nil || errors.Is(err, flag.ErrHelp) {
				return err
			}

			rewritten := applyTextReplacements(err.Error(), replacements)
			if rewritten == err.Error() {
				return err
			}
			return rewrittenCommandError{
				message: rewritten,
				err:     err,
			}
		}
	}

	for _, sub := range cmd.Subcommands {
		rewriteCommandErrors(sub, replacements)
	}
}

func applyTextReplacements(input string, replacements []textReplacement) string {
	output := input
	for _, replacement := range replacements {
		output = strings.ReplaceAll(output, replacement.old, replacement.new)
	}
	return output
}

func setUsageFuncRecursively(cmd *ffcli.Command, usageFunc func(*ffcli.Command) string) {
	if cmd == nil {
		return
	}
	cmd.UsageFunc = usageFunc
	for _, sub := range cmd.Subcommands {
		setUsageFuncRecursively(sub, usageFunc)
	}
}

func findSubcommand(cmd *ffcli.Command, name string) *ffcli.Command {
	if cmd == nil {
		return nil
	}
	for _, sub := range cmd.Subcommands {
		if sub.Name == name {
			return sub
		}
	}
	return nil
}

func TestFlightGroupsCommand() *ffcli.Command {
	cmd := rewriteCommandTree(
		BetaGroupsCommand(),
		"asc testflight beta-groups",
		"asc testflight groups",
		map[string]string{
			"beta-groups":               "groups",
			"beta-recruitment-criteria": "recruitment",
			"beta-recruitment-criterion-compatible-build-check": "compatibility",
			"relationships": "links",
			"get":           "view",
			"update":        "edit",
		},
		[]textReplacement{
			{old: "Manage TestFlight beta groups.", new: "Manage TestFlight groups."},
			{old: "Manage TestFlight beta groups", new: "Manage TestFlight groups"},
			{old: "List TestFlight beta groups", new: "List TestFlight groups"},
			{old: "beta testers", new: "testers"},
			{old: "Beta testers", new: "Testers"},
			{old: "beta tester", new: "tester"},
			{old: "Beta tester", new: "Tester"},
			{old: "compatible build status", new: "recruitment compatibility"},
			{old: "Compatible build status", new: "Recruitment compatibility"},
			{old: "beta recruitment criteria", new: "recruitment criteria"},
			{old: "Beta recruitment criteria", new: "Recruitment criteria"},
			{old: "beta recruitment criterion compatible build status", new: "compatible build status"},
			{old: "Beta recruitment criterion compatible build status", new: "Compatible build status"},
			{old: "beta groups", new: "groups"},
			{old: "Beta groups", new: "Groups"},
			{old: "beta group", new: "group"},
			{old: "Beta group", new: "Group"},
			{old: "Get ", new: "View "},
			{old: "get ", new: "view "},
			{old: "Update ", new: "Edit "},
			{old: "update ", new: "edit "},
		},
	)
	setUsageFuncRecursively(cmd, shared.DefaultUsageFunc)
	if compatibilityCmd := findSubcommand(cmd, "compatibility"); compatibilityCmd != nil {
		compatibilityCmd.ShortHelp = "Check recruitment compatibility for a group."
		compatibilityCmd.LongHelp = `Check recruitment compatibility for a group.

Examples:
  asc testflight groups compatibility view --group-id "GROUP_ID"`
		if viewCmd := findSubcommand(compatibilityCmd, "view"); viewCmd != nil {
			viewCmd.ShortHelp = "View recruitment compatibility for a group."
			viewCmd.LongHelp = `View recruitment compatibility for a group.

Examples:
  asc testflight groups compatibility view --group-id "GROUP_ID"`
		}
	}
	return cmd
}

func TestFlightTestersCommand() *ffcli.Command {
	cmd := rewriteCommandTree(
		BetaTestersCommand(),
		"asc testflight beta-testers",
		"asc testflight testers",
		map[string]string{
			"beta-testers":  "testers",
			"beta-groups":   "groups",
			"relationships": "links",
			"get":           "view",
		},
		[]textReplacement{
			{old: "Manage TestFlight beta testers.", new: "Manage TestFlight testers."},
			{old: "Manage TestFlight beta testers", new: "Manage TestFlight testers"},
			{old: "List TestFlight beta testers", new: "List TestFlight testers"},
			{old: "beta groups", new: "groups"},
			{old: "Beta groups", new: "Groups"},
			{old: "beta group", new: "group"},
			{old: "Beta group", new: "Group"},
			{old: "beta testers", new: "testers"},
			{old: "Beta testers", new: "Testers"},
			{old: "beta tester", new: "tester"},
			{old: "Beta tester", new: "Tester"},
			{old: "Get ", new: "View "},
			{old: "get ", new: "view "},
		},
	)
	setUsageFuncRecursively(cmd, shared.DefaultUsageFunc)
	return cmd
}

func TestFlightAgreementsCommand() *ffcli.Command {
	cmd := rewriteCommandTree(
		BetaLicenseAgreementsCommand(),
		"asc testflight beta-license-agreements",
		"asc testflight agreements",
		map[string]string{
			"beta-license-agreements": "agreements",
			"get":                     "view",
			"update":                  "edit",
		},
		[]textReplacement{
			{old: "Manage TestFlight beta license agreements.", new: "Manage TestFlight agreements."},
			{old: "Manage TestFlight beta license agreements", new: "Manage TestFlight agreements"},
			{old: "Fields to include (betaLicenseAgreements), comma-separated", new: "Fields to include for agreements, comma-separated"},
			{old: "beta license agreements", new: "agreements"},
			{old: "Beta license agreements", new: "Agreements"},
			{old: "beta license agreement", new: "agreement"},
			{old: "Beta license agreement", new: "Agreement"},
			{old: "Get ", new: "View "},
			{old: "get ", new: "view "},
			{old: "Update ", new: "Edit "},
			{old: "update ", new: "edit "},
		},
	)
	cmd.UsageFunc = shared.DefaultUsageFunc
	if listCmd := findSubcommand(cmd, "list"); listCmd != nil {
		listCmd.ShortHelp = "List agreements."
	}
	if viewCmd := findSubcommand(cmd, "view"); viewCmd != nil {
		viewCmd.ShortHelp = "View an agreement by ID or app."
		viewCmd.LongHelp = `View an agreement by ID or app.

Examples:
  asc testflight agreements view --id "AGREEMENT_ID"
  asc testflight agreements view --app "APP_ID"`
	}
	if editCmd := findSubcommand(cmd, "edit"); editCmd != nil {
		editCmd.ShortHelp = "Edit an agreement."
		editCmd.LongHelp = `Edit an agreement.

Examples:
  asc testflight agreements edit --id "AGREEMENT_ID" --agreement-text "Updated terms"`
	}
	return cmd
}

func TestFlightNotificationsCommand() *ffcli.Command {
	cmd := rewriteCommandTree(
		BetaNotificationsCommand(),
		"asc testflight beta-notifications",
		"asc testflight notifications",
		map[string]string{
			"beta-notifications": "notifications",
			"create":             "send",
		},
		[]textReplacement{
			{old: "Send TestFlight beta build notifications.", new: "Send TestFlight notifications."},
			{old: "Send TestFlight beta build notifications", new: "Send TestFlight notifications"},
			{old: "beta notification", new: "notification"},
			{old: "Beta notification", new: "Notification"},
			{old: "Create ", new: "Send "},
			{old: "create ", new: "send "},
		},
	)
	cmd.UsageFunc = shared.DefaultUsageFunc
	return cmd
}

func TestFlightConfigCommand() *ffcli.Command {
	cmd := rewriteCommandTree(
		TestFlightSyncCommand(),
		"asc testflight sync",
		"asc testflight config",
		map[string]string{
			"sync": "config",
			"pull": "export",
		},
		[]textReplacement{
			{old: "Sync TestFlight configuration.", new: "Export TestFlight configuration."},
			{old: "Sync TestFlight configuration", new: "Export TestFlight configuration"},
			{old: "beta groups", new: "TestFlight groups"},
			{old: "beta group", new: "TestFlight group"},
			{old: "sync pull", new: "config export"},
			{old: "testflight sync", new: "testflight config"},
		},
	)
	cmd.UsageFunc = shared.DefaultUsageFunc
	return cmd
}

func TestFlightReviewSurfaceCommand() *ffcli.Command {
	cmd := rewriteCommandTree(
		TestFlightReviewCommand(),
		"asc testflight review",
		"asc testflight review",
		map[string]string{
			"get":    "view",
			"update": "edit",
		},
		[]textReplacement{
			{old: "Manage TestFlight beta app review details.", new: "Manage TestFlight review details."},
			{old: "Manage TestFlight beta app review details", new: "Manage TestFlight review details"},
			{old: "beta app review details", new: "review details"},
			{old: "Beta app review details", new: "Review details"},
			{old: "beta app review detail", new: "review detail"},
			{old: "Beta app review detail", new: "Review detail"},
			{old: "beta app review submissions", new: "review submissions"},
			{old: "Beta app review submissions", new: "Review submissions"},
			{old: "beta app review submission", new: "review submission"},
			{old: "Beta app review submission", new: "Review submission"},
			{old: "Get ", new: "View "},
			{old: "get ", new: "view "},
			{old: "Fetch ", new: "View "},
			{old: "fetch ", new: "view "},
			{old: "Update ", new: "Edit "},
			{old: "update ", new: "edit "},
			{old: "Submit a build for beta app review.", new: "Submit a build for TestFlight review."},
			{old: "Submit a build for beta app review", new: "Submit a build for TestFlight review"},
		},
	)
	cmd.ShortHelp = "Manage TestFlight review details."
	cmd.LongHelp = `Manage TestFlight review details and submissions.

Examples:
  asc testflight review view --app "APP_ID"
  asc testflight review edit --id "DETAIL_ID" --contact-email "dev@example.com"
  asc testflight review submit --build-id "BUILD_ID" --confirm
  asc testflight review app view --id "DETAIL_ID"
  asc testflight review submissions list --build-id "BUILD_ID"
  asc testflight review submissions view --id "SUBMISSION_ID"`
	setUsageFuncRecursively(cmd, shared.DefaultUsageFunc)
	return cmd
}

func TestFlightDistributionCommand() *ffcli.Command {
	cmd := rewriteCommandTree(
		TestFlightBetaDetailsCommand(),
		"asc testflight beta-details",
		"asc testflight distribution",
		map[string]string{
			"beta-details": "distribution",
			"get":          "view",
			"update":       "edit",
		},
		[]textReplacement{
			{old: "Manage TestFlight build beta details.", new: "Manage TestFlight distribution settings."},
			{old: "Manage TestFlight build beta details", new: "Manage TestFlight distribution settings"},
			{old: "build beta details", new: "distribution settings"},
			{old: "Build beta details", new: "Distribution settings"},
			{old: "build beta detail", new: "distribution setting"},
			{old: "Build beta detail", new: "Distribution setting"},
			{old: "Get ", new: "View "},
			{old: "get ", new: "view "},
			{old: "Fetch ", new: "View "},
			{old: "fetch ", new: "view "},
			{old: "Update ", new: "Edit "},
			{old: "update ", new: "edit "},
		},
	)
	cmd.ShortHelp = "Manage TestFlight distribution settings."
	cmd.LongHelp = `Manage TestFlight distribution settings.

Examples:
  asc testflight distribution view --build-id "BUILD_ID"
  asc testflight distribution edit --id "DETAIL_ID" --auto-notify
  asc testflight distribution build view --id "DETAIL_ID"`
	setUsageFuncRecursively(cmd, shared.DefaultUsageFunc)
	return cmd
}

func TestFlightMetricsSurfaceCommand() *ffcli.Command {
	cmd := TestFlightMetricsCommand()
	cmd.LongHelp = `Fetch TestFlight metrics.

Examples:
  asc testflight metrics public-link --group "GROUP_ID"
  asc testflight metrics group-testers --group "GROUP_ID"
  asc testflight metrics app-testers --app "APP_ID"`
	cmd.UsageFunc = shared.DefaultUsageFunc
	cmd.Subcommands = []*ffcli.Command{
		TestFlightMetricsPublicLinkCommand(),
		TestFlightMetricsGroupTestersCommand(),
		TestFlightMetricsAppTestersCommand(),
	}
	return cmd
}

func TestFlightAppLocalizationsCommand() *ffcli.Command {
	cmd := rewriteCommandTree(
		betaapplocalizations.BetaAppLocalizationsCommand(),
		"asc beta-app-localizations",
		"asc testflight app-localizations",
		map[string]string{
			"beta-app-localizations": "app-localizations",
		},
		[]textReplacement{
			{old: "beta-app-localizations ", new: "testflight app-localizations "},
			{old: "Manage TestFlight beta app localizations.", new: "Manage TestFlight app localizations."},
			{old: "Manage TestFlight beta app localizations", new: "Manage TestFlight app localizations"},
			{old: "List beta app localizations", new: "List app localizations"},
			{old: "View a beta app localization", new: "View an app localization"},
			{old: "Create a beta app localization", new: "Create an app localization"},
			{old: "Update a beta app localization", new: "Update an app localization"},
			{old: "Delete a beta app localization", new: "Delete an app localization"},
			{old: "View the app for a beta app localization", new: "View the app for an app localization"},
			{old: "beta app localizations", new: "app localizations"},
			{old: "Beta app localizations", new: "App localizations"},
			{old: "beta app localization", new: "app localization"},
			{old: "Beta app localization", new: "App localization"},
		},
	)
	cmd.ShortHelp = "Manage TestFlight app localizations."
	cmd.LongHelp = `Manage TestFlight app localizations.

Examples:
  asc testflight app-localizations list --app "APP_ID"
  asc testflight app-localizations view --id "LOCALIZATION_ID"
  asc testflight app-localizations app view --id "LOCALIZATION_ID"
  asc testflight app-localizations create --app "APP_ID" --locale "en-US" --description "Welcome testers"`
	setUsageFuncRecursively(cmd, shared.DefaultUsageFunc)
	return cmd
}

func TestFlightPreReleaseCommand() *ffcli.Command {
	cmd := rewriteCommandTree(
		prerelease.PreReleaseVersionsCommand(),
		"asc pre-release-versions",
		"asc testflight pre-release",
		map[string]string{
			"pre-release-versions": "pre-release",
			"relationships":        "links",
			"get":                  "view",
		},
		[]textReplacement{
			{old: "pre-release-versions ", new: "testflight pre-release "},
			{old: "pre-release-versions relationships view", new: "testflight pre-release links view"},
			{old: "Manage TestFlight pre-release versions.", new: "Manage pre-release versions."},
			{old: "List TestFlight pre-release versions", new: "List pre-release versions"},
			{old: "View a TestFlight pre-release version", new: "View a pre-release version"},
			{old: "Get the app for a pre-release version", new: "View the app for a pre-release version"},
			{old: "Get relationship linkages for a pre-release version", new: "View relationship linkages for a pre-release version"},
			{old: "Get ", new: "View "},
			{old: "get: ", new: "view: "},
			{old: "get ", new: "view "},
		},
	)
	cmd.ShortHelp = "Manage pre-release versions."
	cmd.LongHelp = `Manage pre-release versions.

Examples:
  asc testflight pre-release list --app "APP_ID"
  asc testflight pre-release view --id "PR_ID"
  asc testflight pre-release app view --id "PR_ID"
  asc testflight pre-release builds list --id "PR_ID"
  asc testflight pre-release links view --id "PR_ID" --type "app"`
	setUsageFuncRecursively(cmd, shared.DefaultUsageFunc)
	return cmd
}
