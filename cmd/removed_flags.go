package cmd

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// removedFlagVersion names the release that deleted the 4.x compatibility
// aliases listed in migrate-to-5-0.mdx ("Removed flags").
const removedFlagVersion = "5.0.0"

// removedFlagRule maps one removed 4.x alias, on the commands that accepted
// it, to the 5.0 replacement. Exactly one of replacement or note is set:
// replacement is rendered after "use " and note is rendered as written.
type removedFlagRule struct {
	// flag is the removed spelling without leading dashes.
	flag string
	// commands lists exact command paths (without the leading "asc ") that
	// accepted the alias.
	commands []string
	// requireFlag verifies that the replacement remains registered on commands
	// where the removed alias was bound alongside it.
	requireFlag string
	// replacement is the canonical flag, or the command or environment
	// variable that took over, in backticks.
	replacement string
	// note explains what to do when nothing replaced the flag.
	note string
}

// buildIDAliasCommands are the commands where `--build` was an alias for the
// canonical `--build-id` selector (migrate-to-5-0.mdx, "`--build` where
// `--build-id` is canonical").
var buildIDAliasCommands = []string{
	"builds wait",
	"builds dsyms",
	"builds info",
	"builds expire",
	"builds update",
	"builds add-groups",
	"builds remove-groups",
	"builds individual-testers list",
	"builds individual-testers add",
	"builds individual-testers remove",
	"builds metrics beta-usages",
	"builds links view",
	"builds app view",
	"builds pre-release-version view",
	"builds icons list",
	"builds beta-app-review-submission view",
	"builds build-beta-detail view",
	"builds app-encryption-declaration view",
	"builds test-notes list",
	"builds test-notes view",
	"builds test-notes create",
	"builds test-notes update",
	"builds test-notes delete",
	"testflight testers list",
	"testflight testers add-builds",
	"testflight testers remove-builds",
	"testflight testers export",
	"testflight notifications send",
	"testflight config export",
	"testflight review submit",
	"testflight review submissions list",
	"testflight distribution view",
	"testflight feedback list",
	"testflight crashes list",
	"validate testflight",
	"publish testflight",
	"release stage",
	"review submit",
	"versions attach-build",
	"build-localizations list",
	"build-localizations create",
	"build-bundles list",
	"performance metrics view",
	"performance diagnostics list",
	"performance download",
	"encryption declarations list",
	"encryption declarations assign-builds",
	"apps app-encryption-declarations list",
}

// webTwoFactorCodeAliasCommands is the published 4.11 command inventory that
// bound `--two-factor-code`. Exact paths keep commands added after 5.0.0 from
// receiving a historical removal message for a flag they never accepted.
var webTwoFactorCodeAliasCommands = []string{
	"web auth login",
	"web auth capabilities",
	"web agreements status",
	"web agreements accept",
	"web api-keys create",
	"web sandbox create",
	"web apps create",
	"web apps delete",
	"web apps availability create",
	"web apps compatibility view",
	"web apps compatibility edit",
	"web apps medical-device set",
	"web removed-apps list",
	"web bundle-ids capabilities enable",
	"web bundle-ids capabilities sync-app-clip",
	"web app-groups list",
	"web app-groups create",
	"web app-groups assign",
	"web privacy catalog",
	"web privacy pull",
	"web privacy plan",
	"web privacy apply",
	"web privacy publish",
	"web review list",
	"web review show",
	"web review subscriptions list",
	"web review subscriptions attach",
	"web review subscriptions attach-group",
	"web review subscriptions remove",
	"web review subscriptions remove-group",
	"web review iaps attach",
	"web subscriptions availability remove-from-sale",
	"web subscriptions pricing adjusted-equalizations view",
	"web subscriptions pricing monthly-commitment bootstrap",
	"web analytics overview",
	"web analytics sources",
	"web analytics product-pages",
	"web analytics in-app-events",
	"web analytics app-clips",
	"web analytics campaigns",
	"web analytics sales",
	"web analytics subscriptions",
	"web analytics offers",
	"web analytics benchmarks",
	"web analytics metrics",
	"web analytics retention",
	"web analytics cohorts",
	"web xcode-cloud usage summary",
	"web xcode-cloud usage alert",
	"web xcode-cloud usage months",
	"web xcode-cloud usage days",
	"web xcode-cloud usage workflows",
	"web xcode-cloud products",
	"web xcode-cloud workflows describe",
	"web xcode-cloud workflows create",
	"web xcode-cloud workflows options team-config",
	"web xcode-cloud workflows options build-versions",
	"web xcode-cloud workflows options product-config",
	"web xcode-cloud workflows options schemes",
	"web xcode-cloud workflows options test-destinations",
	"web xcode-cloud workflows options slack-provider",
	"web xcode-cloud workflows options slack-channels",
	"web xcode-cloud workflows edit",
	"web xcode-cloud workflows enable",
	"web xcode-cloud workflows disable",
	"web xcode-cloud env-vars list",
	"web xcode-cloud env-vars set",
	"web xcode-cloud env-vars delete",
	"web xcode-cloud env-vars shared list",
	"web xcode-cloud env-vars shared set",
	"web xcode-cloud env-vars shared delete",
}

// removedFlagRules mirrors the published command paths in the "Removed flags"
// tables in migrate-to-5-0.mdx.
var removedFlagRules = []removedFlagRule{
	{
		flag:        "build",
		commands:    buildIDAliasCommands,
		replacement: "`--build-id`",
	},
	{
		flag: "id",
		commands: []string{
			"builds app view",
			"builds pre-release-version view",
			"builds icons list",
			"builds beta-app-review-submission view",
			"builds build-beta-detail view",
			"builds app-encryption-declaration view",
		},
		replacement: "`--build-id`",
	},
	{
		flag:        "id",
		commands:    []string{"builds test-notes view", "builds test-notes update", "builds test-notes delete"},
		replacement: "`--localization-id`",
	},
	{flag: "newest", commands: []string{"builds wait"}, replacement: "`--latest`"},
	{flag: "app-id", commands: []string{"builds list"}, replacement: "`--app`"},
	{flag: "group-id", commands: []string{"testflight groups view"}, replacement: "`--id`"},
	{
		flag:        "external-testing",
		commands:    []string{"testflight distribution edit"},
		replacement: "`asc builds add-groups --submit --confirm` or `asc builds remove-groups --confirm`",
	},
	{flag: "id", commands: []string{"versions view", "versions update"}, replacement: "`--version-id`"},
	{flag: "app", commands: []string{"apps view"}, replacement: "`--id`"},
	{flag: "id", commands: []string{"apps public view", "apps public prices", "apps public descriptions"}, replacement: "`--app`"},
	{flag: "app-info", commands: []string{"apps info view"}, replacement: "`--info-id`"},
	{
		flag: "id",
		commands: []string{
			"apps info relationships primary-category",
			"apps info relationships primary-subcategory-one",
			"apps info relationships primary-subcategory-two",
			"apps info relationships secondary-category",
			"apps info relationships secondary-subcategory-one",
			"apps info relationships secondary-subcategory-two",
			"apps info territory-age-ratings list",
		},
		replacement: "`--info-id`",
	},
	{flag: "bundle-id", commands: []string{"bundle-ids capabilities list"}, replacement: "`--bundle`"},
	{flag: "version-id", commands: []string{"localizations list", "localizations update"}, replacement: "`--version`"},
	{flag: "localization-id", commands: []string{"screenshots list"}, replacement: "`--version-localization`"},
	{
		flag:     "available-in-new-territories",
		commands: []string{"pre-orders enable"},
		note:     "pre-orders are enabled by patching territory availabilities",
	},
	{flag: "version-limit", commands: []string{"subscriptions list", "subscriptions view"}, replacement: "`--versions-limit`"},
	{flag: "subscription-id", commands: []string{"subscriptions view"}, replacement: "`--id`"},
	{flag: "image-limit", commands: []string{"subscriptions versions list", "subscriptions versions view"}, replacement: "`--images-limit`"},
	{flag: "localization-limit", commands: []string{"subscriptions versions list", "subscriptions versions view"}, replacement: "`--localizations-limit`"},
	{flag: "id", commands: []string{"subscriptions review screenshots delete"}, replacement: "`--screenshot-id`"},
	{flag: "limit", commands: []string{"game-center details list"}, note: "each app has a single Game Center detail"},
	{flag: "next", commands: []string{"game-center details list"}, note: "each app has a single Game Center detail"},
	{flag: "paginate", commands: []string{"game-center details list"}, note: "each app has a single Game Center detail"},
	{
		flag:     "challenge-enabled",
		commands: []string{"game-center details create", "game-center details update"},
		note:     "App Store Connect no longer accepts `challengeEnabled`",
	},
	{
		flag:        "two-factor-code",
		commands:    webTwoFactorCodeAliasCommands,
		requireFlag: "two-factor-code-command",
		replacement: "`--two-factor-code-command` or `ASC_WEB_2FA_CODE_COMMAND`",
	},
	{flag: "id", commands: []string{"xcode-cloud status"}, replacement: "`--run-id`"},
	{
		flag:        "password",
		commands:    []string{"signing sync push", "signing sync pull"},
		replacement: "`--password-file` or `ASC_SIGNING_SYNC_PASSWORD`",
	},
	{
		flag: "keyword",
		commands: []string{
			"ads v5 campaign-negative-keywords view",
			"ads v5 ad-group-negative-keywords view",
		},
		replacement: "`--negative-keyword`",
	},
}

// lookupRemovedFlag returns the rule for a removed alias on the named command.
// commandPath is the space-joined path without the leading "asc ", flagName is
// the spelling without leading dashes, and flags is the command's flag set
// (nil when unknown).
func lookupRemovedFlag(commandPath, flagName string, flags *flag.FlagSet) (removedFlagRule, bool) {
	for _, rule := range removedFlagRules {
		if rule.flag != flagName || !rule.matchesCommand(commandPath, flags) {
			continue
		}
		return rule, true
	}
	return removedFlagRule{}, false
}

func (rule removedFlagRule) matchesCommand(commandPath string, flags *flag.FlagSet) bool {
	if rule.requireFlag != "" && (flags == nil || flags.Lookup(rule.requireFlag) == nil) {
		return false
	}
	for _, command := range rule.commands {
		if command == commandPath {
			return true
		}
	}
	return false
}

// guidance renders the clause that follows "was removed in 5.0.0; ".
func (rule removedFlagRule) guidance() string {
	if rule.replacement != "" {
		return "use " + rule.replacement
	}
	return rule.note
}

// printRemovedFlagHint writes the removal diagnostic for a removed alias and
// reports whether the unknown flag matched one. The flag name comes from the
// rule table rather than the invocation, so a value attached with `=` is never
// echoed.
func printRemovedFlagHint(w io.Writer, commandName, flagName string, flags *flag.FlagSet) bool {
	name, ok := flagLookupName(flagName)
	if !ok {
		return false
	}
	rule, ok := lookupRemovedFlag(strings.TrimPrefix(commandName, "asc "), name, flags)
	if !ok {
		return false
	}
	fmt.Fprintf(
		w,
		"Error: `--%s` was removed in %s; %s (see migrate-to-5-0)\n",
		rule.flag,
		removedFlagVersion,
		rule.guidance(),
	)
	printFlagSuggestions(w, removedFlagGuidanceSuggestions(rule, flags, flagName))
	fmt.Fprintf(w, "For help:\n  %s --help\n", commandName)
	return true
}
