package testflight

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/peterbourgon/ff/v3/ffcli"

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

var hiddenTestFlightCommands sync.Map

func testflightVisibleUsageFunc(c *ffcli.Command) string {
	clone := *c
	if len(c.Subcommands) > 0 {
		visible := make([]*ffcli.Command, 0, len(c.Subcommands))
		for _, sub := range c.Subcommands {
			if !isHiddenTestFlightCommand(sub) {
				visible = append(visible, sub)
			}
		}
		clone.Subcommands = visible
	}
	return shared.DefaultUsageFunc(&clone)
}

func hideTestFlightCommand(cmd *ffcli.Command) *ffcli.Command {
	if cmd != nil {
		hiddenTestFlightCommands.Store(cmd, struct{}{})
	}
	return cmd
}

func isHiddenTestFlightCommand(cmd *ffcli.Command) bool {
	if cmd == nil {
		return false
	}
	_, ok := hiddenTestFlightCommands.Load(cmd)
	return ok
}

func rewriteCommandTree(cmd *ffcli.Command, oldRootPath, newRootPath string, nameRenames map[string]string, textReplacements []textReplacement) *ffcli.Command {
	if cmd == nil {
		return nil
	}

	pathReplacements := collectCommandPathReplacements(cmd, oldRootPath, newRootPath, nameRenames)
	replacements := append(pathReplacements, textReplacements...)

	renameCommandNames(cmd, nameRenames)
	rewriteCommandStrings(cmd, replacements)
	rewriteCommandErrors(cmd, replacements)
	return cmd
}

func rewriteCommandPresentation(cmd *ffcli.Command, oldRootPath, newRootPath string, nameRenames map[string]string) *ffcli.Command {
	if cmd == nil {
		return nil
	}

	pathReplacements := collectCommandPathReplacements(cmd, oldRootPath, newRootPath, nameRenames)
	rewriteCommandStrings(cmd, pathReplacements)
	return cmd
}

func collectCommandPathReplacements(cmd *ffcli.Command, oldRootPath, newRootPath string, nameRenames map[string]string) []textReplacement {
	replacements := []textReplacement{}

	var walk func(current *ffcli.Command, oldPath, newPath string)
	walk = func(current *ffcli.Command, oldPath, newPath string) {
		replacements = append(replacements,
			textReplacement{old: oldPath, new: newPath},
			textReplacement{old: strings.TrimPrefix(oldPath, "asc testflight "), new: strings.TrimPrefix(newPath, "asc testflight ")},
		)

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

	sort.SliceStable(replacements, func(i, j int) bool {
		return len(replacements[i].old) > len(replacements[j].old)
	})

	return replacements
}

func renameCommandNames(cmd *ffcli.Command, nameRenames map[string]string) {
	if cmd == nil {
		return
	}

	if renamed, ok := nameRenames[cmd.Name]; ok {
		cmd.Name = renamed
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

func deprecatedAliasCommand(cmd *ffcli.Command, shortUsage, shortHelp, longHelp string) *ffcli.Command {
	if cmd == nil {
		return nil
	}
	cmd.ShortUsage = shortUsage
	cmd.ShortHelp = shortHelp
	cmd.LongHelp = longHelp
	cmd.UsageFunc = shared.DeprecatedUsageFunc
	return hideTestFlightCommand(cmd)
}

func RemovedTestFlightAppsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps", flag.ExitOnError)

	return hideTestFlightCommand(&ffcli.Command{
		Name:       "apps",
		ShortUsage: "asc apps <subcommand> [flags]",
		ShortHelp:  "REMOVED: use `asc apps`.",
		LongHelp:   "Use `asc apps list` for collection lookup and `asc apps get --id APP_ID` for a single app.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			suggestion := "asc apps"
			if len(args) > 0 {
				switch strings.TrimSpace(args[0]) {
				case "list":
					suggestion = "asc apps list"
				case "get", "view":
					suggestion = "asc apps get --id APP_ID"
				}
			}

			fmt.Fprintf(os.Stderr, "Error: `asc testflight apps` was removed. Use `%s` instead.\n", suggestion)
			return flag.ErrHelp
		},
	})
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
			"get":    "view",
			"update": "edit",
		},
		[]textReplacement{
			{old: "Manage TestFlight beta groups.", new: "Manage TestFlight groups."},
			{old: "Manage TestFlight beta groups", new: "Manage TestFlight groups"},
			{old: "List TestFlight beta groups", new: "List TestFlight groups"},
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
	setUsageFuncRecursively(cmd, testflightVisibleUsageFunc)
	if relationshipsCmd := findSubcommand(cmd, "relationships"); relationshipsCmd != nil {
		hideTestFlightCommand(relationshipsCmd)
	}
	if compatibilityCmd := findSubcommand(cmd, "compatibility"); compatibilityCmd != nil {
		compatibilityCmd.ShortHelp = "Check recruitment compatibility for a group."
		compatibilityCmd.LongHelp = `Check recruitment compatibility for a group.

Examples:
  asc testflight groups compatibility view --group-id "GROUP_ID"`
	}
	return cmd
}

func TestFlightTestersCommand() *ffcli.Command {
	cmd := rewriteCommandTree(
		BetaTestersCommand(),
		"asc testflight beta-testers",
		"asc testflight testers",
		map[string]string{
			"beta-testers": "testers",
			"beta-groups":  "groups",
			"get":          "view",
		},
		[]textReplacement{
			{old: "Manage TestFlight beta testers.", new: "Manage TestFlight testers."},
			{old: "Manage TestFlight beta testers", new: "Manage TestFlight testers"},
			{old: "List TestFlight beta testers", new: "List TestFlight testers"},
			{old: "beta testers", new: "testers"},
			{old: "Beta testers", new: "Testers"},
			{old: "beta tester", new: "tester"},
			{old: "Beta tester", new: "Tester"},
			{old: "Get ", new: "View "},
			{old: "get ", new: "view "},
		},
	)
	setUsageFuncRecursively(cmd, testflightVisibleUsageFunc)
	if relationshipsCmd := findSubcommand(cmd, "relationships"); relationshipsCmd != nil {
		hideTestFlightCommand(relationshipsCmd)
	}
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
	cmd.UsageFunc = testflightVisibleUsageFunc
	if len(cmd.Subcommands) >= 3 {
		cmd.Subcommands[0].ShortHelp = "List agreements."
		cmd.Subcommands[1].ShortHelp = "View an agreement by ID or app."
		cmd.Subcommands[1].LongHelp = `View an agreement by ID or app.

Examples:
  asc testflight agreements view --id "AGREEMENT_ID"
  asc testflight agreements view --app "APP_ID"`
		cmd.Subcommands[2].ShortHelp = "Edit an agreement."
		cmd.Subcommands[2].LongHelp = `Edit an agreement.

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
	cmd.UsageFunc = testflightVisibleUsageFunc
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
			{old: "sync pull", new: "config export"},
			{old: "testflight sync", new: "testflight config"},
		},
	)
	cmd.UsageFunc = testflightVisibleUsageFunc
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
  asc testflight review submit --build "BUILD_ID" --confirm
  asc testflight review app view --id "DETAIL_ID"
  asc testflight review submissions list --build "BUILD_ID"
  asc testflight review submissions view --id "SUBMISSION_ID"`
	setUsageFuncRecursively(cmd, testflightVisibleUsageFunc)

	cmd.Subcommands = append(cmd.Subcommands,
		deprecatedAliasCommand(
			TestFlightReviewGetCommand(),
			"asc testflight review view [flags]",
			"DEPRECATED: use `asc testflight review view`.",
			"DEPRECATED: use `asc testflight review view --app APP_ID`.",
		),
		deprecatedAliasCommand(
			TestFlightReviewUpdateCommand(),
			"asc testflight review edit [flags]",
			"DEPRECATED: use `asc testflight review edit`.",
			"DEPRECATED: use `asc testflight review edit --id DETAIL_ID ...`.",
		),
	)

	if appCmd := findSubcommand(cmd, "app"); appCmd != nil {
		appCmd.Subcommands = append(appCmd.Subcommands,
			deprecatedAliasCommand(
				TestFlightReviewAppGetCommand(),
				"asc testflight review app view --id \"DETAIL_ID\"",
				"DEPRECATED: use `asc testflight review app view`.",
				"DEPRECATED: use `asc testflight review app view --id DETAIL_ID`.",
			),
		)
	}

	if submissionsCmd := findSubcommand(cmd, "submissions"); submissionsCmd != nil {
		submissionsCmd.Subcommands = append(submissionsCmd.Subcommands,
			deprecatedAliasCommand(
				TestFlightReviewSubmissionsGetCommand(),
				"asc testflight review submissions view --id \"SUBMISSION_ID\"",
				"DEPRECATED: use `asc testflight review submissions view`.",
				"DEPRECATED: use `asc testflight review submissions view --id SUBMISSION_ID`.",
			),
		)
	}

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
  asc testflight distribution view --build "BUILD_ID"
  asc testflight distribution edit --id "DETAIL_ID" --auto-notify
  asc testflight distribution build view --id "DETAIL_ID"`
	setUsageFuncRecursively(cmd, testflightVisibleUsageFunc)
	return cmd
}

func DeprecatedBetaDetailsAliasCommand() *ffcli.Command {
	cmd := deprecatedAliasCommand(
		TestFlightBetaDetailsCommand(),
		"asc testflight distribution <subcommand> [flags]",
		"DEPRECATED: use `asc testflight distribution`.",
		"DEPRECATED: use `asc testflight distribution ...`.",
	)
	setUsageFuncRecursively(cmd, shared.DeprecatedUsageFunc)

	if viewCmd := findSubcommand(cmd, "get"); viewCmd != nil {
		viewCmd.ShortUsage = "asc testflight distribution view [flags]"
		viewCmd.ShortHelp = "DEPRECATED: use `asc testflight distribution view`."
		viewCmd.LongHelp = "DEPRECATED: use `asc testflight distribution view --build BUILD_ID`."
	}
	if editCmd := findSubcommand(cmd, "update"); editCmd != nil {
		editCmd.ShortUsage = "asc testflight distribution edit [flags]"
		editCmd.ShortHelp = "DEPRECATED: use `asc testflight distribution edit`."
		editCmd.LongHelp = "DEPRECATED: use `asc testflight distribution edit --id DETAIL_ID ...`."
	}
	if buildCmd := findSubcommand(cmd, "build"); buildCmd != nil {
		buildCmd.ShortUsage = "asc testflight distribution build <subcommand> [flags]"
		buildCmd.ShortHelp = "DEPRECATED: use `asc testflight distribution build view`."
		buildCmd.LongHelp = "DEPRECATED: use `asc testflight distribution build view --id DETAIL_ID`."
		if getCmd := findSubcommand(buildCmd, "get"); getCmd != nil {
			getCmd.ShortUsage = "asc testflight distribution build view --id \"DETAIL_ID\""
			getCmd.ShortHelp = "DEPRECATED: use `asc testflight distribution build view`."
			getCmd.LongHelp = "DEPRECATED: use `asc testflight distribution build view --id DETAIL_ID`."
		}
	}
	return hideTestFlightCommand(cmd)
}

func TestFlightMetricsSurfaceCommand() *ffcli.Command {
	cmd := TestFlightMetricsCommand()
	cmd.LongHelp = `Fetch TestFlight metrics.

Examples:
  asc testflight metrics public-link --group "GROUP_ID"
  asc testflight metrics group-testers --group "GROUP_ID"
  asc testflight metrics app-testers --app "APP_ID"`
	cmd.UsageFunc = testflightVisibleUsageFunc
	cmd.Subcommands = []*ffcli.Command{
		TestFlightMetricsPublicLinkCommand(),
		TestFlightMetricsGroupTestersCommand(),
		TestFlightMetricsAppTestersCommand(),
		DeprecatedMetricsTestersAliasCommand(),
		DeprecatedMetricsBetaTesterUsagesAliasCommand(),
	}
	return cmd
}

func DeprecatedBetaGroupsAliasCommand() *ffcli.Command {
	cmd := deprecatedAliasCommand(
		rewriteCommandPresentation(
			BetaGroupsCommand(),
			"asc testflight beta-groups",
			"asc testflight groups",
			map[string]string{
				"beta-groups":               "groups",
				"beta-recruitment-criteria": "recruitment",
				"beta-recruitment-criterion-compatible-build-check": "compatibility",
				"get":    "view",
				"update": "edit",
			},
		),
		"asc testflight groups <subcommand> [flags]",
		"DEPRECATED: use `asc testflight groups`.",
		"DEPRECATED: use `asc testflight groups ...`.",
	)
	setUsageFuncRecursively(cmd, shared.DeprecatedUsageFunc)
	return cmd
}

func DeprecatedBetaTestersAliasCommand() *ffcli.Command {
	cmd := deprecatedAliasCommand(
		rewriteCommandPresentation(
			BetaTestersCommand(),
			"asc testflight beta-testers",
			"asc testflight testers",
			map[string]string{
				"beta-testers": "testers",
				"beta-groups":  "groups",
				"get":          "view",
			},
		),
		"asc testflight testers <subcommand> [flags]",
		"DEPRECATED: use `asc testflight testers`.",
		"DEPRECATED: use `asc testflight testers ...`.",
	)
	setUsageFuncRecursively(cmd, shared.DeprecatedUsageFunc)
	return cmd
}

func DeprecatedBetaLicenseAgreementsAliasCommand() *ffcli.Command {
	cmd := deprecatedAliasCommand(
		rewriteCommandPresentation(
			BetaLicenseAgreementsCommand(),
			"asc testflight beta-license-agreements",
			"asc testflight agreements",
			map[string]string{
				"beta-license-agreements": "agreements",
				"get":                     "view",
				"update":                  "edit",
			},
		),
		"asc testflight agreements <subcommand> [flags]",
		"DEPRECATED: use `asc testflight agreements`.",
		"DEPRECATED: use `asc testflight agreements ...`.",
	)
	setUsageFuncRecursively(cmd, shared.DeprecatedUsageFunc)
	return cmd
}

func DeprecatedBetaNotificationsAliasCommand() *ffcli.Command {
	cmd := deprecatedAliasCommand(
		rewriteCommandPresentation(
			BetaNotificationsCommand(),
			"asc testflight beta-notifications",
			"asc testflight notifications",
			map[string]string{
				"beta-notifications": "notifications",
				"create":             "send",
			},
		),
		"asc testflight notifications send --build \"BUILD_ID\"",
		"DEPRECATED: use `asc testflight notifications send`.",
		"DEPRECATED: use `asc testflight notifications send --build BUILD_ID`.",
	)
	setUsageFuncRecursively(cmd, shared.DeprecatedUsageFunc)
	return cmd
}

func DeprecatedTestFlightSyncAliasCommand() *ffcli.Command {
	cmd := deprecatedAliasCommand(
		rewriteCommandPresentation(
			TestFlightSyncCommand(),
			"asc testflight sync",
			"asc testflight config",
			map[string]string{
				"sync": "config",
				"pull": "export",
			},
		),
		"asc testflight config export [flags]",
		"DEPRECATED: use `asc testflight config export`.",
		"DEPRECATED: use `asc testflight config export --app APP_ID --output ./testflight.yaml`.",
	)
	setUsageFuncRecursively(cmd, shared.DeprecatedUsageFunc)
	return cmd
}
