package cmd

import (
	"regexp"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/suggest"
)

// maxUnknownChildSuggestions caps the selected `Try:` tier for an unknown
// subcommand. Three lines answer the caller without turning the error into a
// second help page.
const maxUnknownChildSuggestions = 3

// unknownChildSynonyms maps a fully qualified command group to the verbs
// callers guess there that the group never had, and the full invocations each
// guess most likely meant. Telemetry records that roughly one in seven 5.x
// installs hits an unknown child under these groups but, by design, never the
// guessed token, so this table is seeded from realistic guesses rather than
// observed ones and ranks ahead of the nearest-name suggester.
//
// The table also maps the paths agents guess from older or sibling command
// shapes, such as spellings borrowed from a sibling group (`asc subscriptions
// review-screenshots`, after `asc iap review-screenshots`). A guess that means
// exactly one command is a command alias in command_aliases.go instead. The "asc" key covers root-level guesses, where the
// nearest-name suggester otherwise names an unrelated command (`asc app-info`
// is not `asc ipa-info`).
//
// Keys are trimmed lowercase tokens. Values are copy-paste valid invocations of
// real leaf commands, anywhere in the tree, using long-form flags that command
// defines and bare uppercase placeholders (APP_ID, VERSION_ID). A synonym may
// point outside its group when the task really lives elsewhere (`asc agreements
// accept` runs through `asc web agreements`). Tests in did_you_mean_test.go
// resolve every entry against the live command tree and reject a key that
// shadows a real subcommand, so keep the table in sync with the commands.
var unknownChildSynonyms = map[string]map[string][]string{
	"asc": {
		"app-availability": {
			"asc pricing availability view --app APP_ID",
			"asc pricing availability edit --app APP_ID --territory TERRITORIES --available true",
		},
		"privacy": {"asc web privacy pull --app APP_ID"},
		"beta":    {"asc testflight groups list --app APP_ID", "asc testflight testers list --app APP_ID"},
	},
	"asc age-rating": {
		"show":   {"asc age-rating view --app APP_ID", "asc age-rating audit --app APP_ID"},
		"set":    {"asc age-rating edit --app APP_ID --kids-age-band KIDS_AGE_BAND", "asc age-rating view --app APP_ID"},
		"update": {"asc age-rating edit --app APP_ID --kids-age-band KIDS_AGE_BAND", "asc age-rating view --app APP_ID"},
	},
	"asc agreements": {
		"accept": {"asc web agreements accept --agreement-id AGREEMENT_ID --confirm", "asc web agreements status"},
		"status": {"asc web agreements status", "asc agreements territories list --id EULA_ID"},
		"list":   {"asc web agreements status", "asc agreements territories list --id EULA_ID"},
		"show":   {"asc web agreements status", "asc agreements territories list --id EULA_ID"},
		"view":   {"asc web agreements status", "asc agreements territories list --id EULA_ID"},
		"get":    {"asc web agreements status", "asc agreements territories list --id EULA_ID"},
	},
	"asc apps": {
		"show":   {"asc apps view --id APP_ID", "asc apps info view --app APP_ID"},
		"find":   {"asc apps list --name NAME", "asc apps list --bundle-id BUNDLE_ID"},
		"search": {"asc apps list --name NAME", "asc apps list --bundle-id BUNDLE_ID"},
	},
	"asc auth": {
		"whoami":    {"asc auth status", "asc auth doctor"},
		"me":        {"asc auth status", "asc auth doctor"},
		"current":   {"asc auth status", "asc auth doctor"},
		"show":      {"asc auth status", "asc auth doctor"},
		"list":      {"asc auth status", "asc auth switch --name PROFILE"},
		"check":     {"asc auth status --validate", "asc auth doctor"},
		"verify":    {"asc auth status --validate", "asc auth doctor"},
		"validate":  {"asc auth status --validate", "asc auth doctor"},
		"test":      {"asc auth status --validate", "asc auth doctor"},
		"add":       {"asc auth login --name NAME --key-id KEY_ID --issuer-id ISSUER_ID --private-key KEY_PATH", "asc auth init"},
		"setup":     {"asc auth login --name NAME --key-id KEY_ID --issuer-id ISSUER_ID --private-key KEY_PATH", "asc auth init"},
		"configure": {"asc auth login --name NAME --key-id KEY_ID --issuer-id ISSUER_ID --private-key KEY_PATH", "asc auth init"},
	},
	"asc builds": {
		"latest":              {"asc builds info --app APP_ID --latest", "asc builds list --app APP_ID --limit 1"},
		"current":             {"asc builds info --app APP_ID --latest", "asc builds list --app APP_ID --limit 1"},
		"status":              {"asc builds info --app APP_ID --latest", "asc builds wait --app APP_ID --latest"},
		"show":                {"asc builds info --build-id BUILD_ID", "asc builds list --app APP_ID"},
		"bundles":             {"asc build-bundles list --build-id BUILD_ID", "asc builds info --build-id BUILD_ID"},
		"build-bundles":       {"asc build-bundles list --build-id BUILD_ID", "asc builds info --build-id BUILD_ID"},
		"localizations":       {"asc build-localizations list --build-id BUILD_ID", "asc builds test-notes list --build-id BUILD_ID"},
		"build-localizations": {"asc build-localizations list --build-id BUILD_ID", "asc builds test-notes list --build-id BUILD_ID"},
		"notes":               {"asc builds test-notes list --build-id BUILD_ID", "asc builds test-notes view --build-id BUILD_ID --locale en-US"},
		"release-notes":       {"asc build-localizations list --build-id BUILD_ID", "asc builds test-notes list --build-id BUILD_ID"},
		"whats-new":           {"asc build-localizations list --build-id BUILD_ID", "asc builds test-notes list --build-id BUILD_ID"},
		"what-to-test":        {"asc builds test-notes list --build-id BUILD_ID", "asc builds test-notes view --build-id BUILD_ID --locale en-US"},
		"distribute":          {"asc publish testflight --app APP_ID --ipa PATH --group GROUP_ID", "asc builds add-groups --build-id BUILD_ID --group GROUP_ID"},
		"publish":             {"asc publish testflight --app APP_ID --ipa PATH --group GROUP_ID", "asc publish appstore --app APP_ID --ipa PATH --version VERSION"},
		"submit":              {"asc review submit --app APP_ID --version VERSION --build-id BUILD_ID --confirm", "asc publish appstore --app APP_ID --ipa PATH --version VERSION --submit --confirm"},
		"attach":              {"asc versions attach-build --version-id VERSION_ID --build-id BUILD_ID"},
		"link":                {"asc versions attach-build --version-id VERSION_ID --build-id BUILD_ID"},
		"delete":              {"asc builds expire --build-id BUILD_ID --confirm", "asc builds expire-all --app APP_ID --older-than 90d --confirm"},
		"remove":              {"asc builds expire --build-id BUILD_ID --confirm", "asc builds remove-groups --build-id BUILD_ID --group GROUP_ID --confirm"},
		"rm":                  {"asc builds expire --build-id BUILD_ID --confirm", "asc builds remove-groups --build-id BUILD_ID --group GROUP_ID --confirm"},
		"testers":             {"asc builds individual-testers list --build-id BUILD_ID", "asc builds individual-testers add --build-id BUILD_ID --tester TESTER_ID"},
	},
	"asc iap": {
		"show": {"asc iap view --id IAP_ID", "asc iap list --app APP_ID"},
		"add":  {"asc iap create --app APP_ID --product-id PRODUCT_ID --ref-name NAME --type TYPE"},
		"submit": {
			"asc review submissions-create --app APP_ID --platform IOS",
			"asc iap versions submit --version-id IAP_VERSION_ID --submission SUBMISSION_ID --confirm",
			"asc review submissions-submit --id SUBMISSION_ID --confirm",
		},
	},
	"asc localizations": {
		"show": {"asc localizations view --id LOCALIZATION_ID", "asc localizations list --version VERSION_ID"},
		"add":  {"asc localizations create --version VERSION_ID --locale LOCALE"},
	},
	"asc metadata": {
		"get":      {"asc metadata pull --app APP_ID --version VERSION --dir ./metadata", "asc apps info view --app APP_ID"},
		"fetch":    {"asc metadata pull --app APP_ID --version VERSION --dir ./metadata", "asc apps info view --app APP_ID"},
		"download": {"asc metadata pull --app APP_ID --version VERSION --dir ./metadata", "asc apps info view --app APP_ID"},
		"show":     {"asc metadata pull --app APP_ID --version VERSION --dir ./metadata", "asc apps info view --app APP_ID"},
		"view":     {"asc metadata pull --app APP_ID --version VERSION --dir ./metadata", "asc apps info view --app APP_ID"},
		"set":      {"asc metadata push --app APP_ID --version VERSION --dir ./metadata --confirm", "asc metadata validate --dir ./metadata"},
		"update":   {"asc metadata push --app APP_ID --version VERSION --dir ./metadata --confirm", "asc metadata validate --dir ./metadata"},
		"upload":   {"asc metadata push --app APP_ID --version VERSION --dir ./metadata --confirm", "asc metadata validate --dir ./metadata"},
	},
	"asc pricing": {
		"view": {"asc pricing current --app APP_ID", "asc pricing availability view --app APP_ID"},
		"set": {
			"asc pricing schedule create --app APP_ID --price-point PRICE_POINT_ID --base-territory TERRITORY",
			"asc pricing schedule create --app APP_ID --free --base-territory TERRITORY",
		},
	},
	"asc pricing availability": {
		"territories": {
			"asc pricing availability view --app APP_ID",
			"asc pricing availability territory-availabilities --availability AVAILABILITY_ID",
			"asc pricing territories list",
		},
	},
	"asc review": {
		"submit-for-review": {"asc review submit --app APP_ID --version VERSION --build-id BUILD_ID --confirm"},
		"state":             {"asc review status --app APP_ID", "asc review doctor --app APP_ID"},
		"check":             {"asc review status --app APP_ID", "asc review doctor --app APP_ID"},
		"list":              {"asc review history --app APP_ID", "asc review submissions list --app APP_ID"},
	},
	"asc screenshots": {
		"add":  {"asc screenshots upload --app APP_ID --version VERSION --path DIR --device-type IPHONE_65", "asc screenshots plan --app APP_ID --version VERSION"},
		"push": {"asc screenshots upload --app APP_ID --version VERSION --path DIR --device-type IPHONE_65", "asc screenshots plan --app APP_ID --version VERSION"},
		"put":  {"asc screenshots upload --app APP_ID --version VERSION --path DIR --device-type IPHONE_65", "asc screenshots plan --app APP_ID --version VERSION"},
		"get":  {"asc screenshots list --app APP_ID --version VERSION --locale LOCALE", "asc screenshots download --id SCREENSHOT_ID --output ./screenshot.png"},
		"show": {"asc screenshots list --app APP_ID --version VERSION --locale LOCALE", "asc screenshots download --id SCREENSHOT_ID --output ./screenshot.png"},
		"view": {"asc screenshots list --app APP_ID --version VERSION --locale LOCALE", "asc screenshots download --id SCREENSHOT_ID --output ./screenshot.png"},
	},
	"asc subscriptions": {
		"show": {"asc subscriptions view --id SUBSCRIPTION_ID", "asc subscriptions list --app APP_ID"},
		"add": {
			"asc subscriptions create --group-id GROUP_ID --reference-name NAME --product-id PRODUCT_ID --subscription-period PERIOD",
		},
		"review-screenshots": {
			"asc subscriptions review screenshots create --subscription-id SUBSCRIPTION_ID --file FILE_PATH",
			"asc subscriptions review screenshots view --screenshot-id SCREENSHOT_ID",
		},
	},
	"asc subscriptions groups": {
		"localizations": {
			"asc subscriptions groups versions list --group-id GROUP_ID",
			"asc subscriptions groups versions localizations list --version-id VERSION_ID",
		},
		"add":          {"asc subscriptions groups create --app APP_ID --reference-name NAME", "asc subscriptions groups list --app APP_ID"},
		"new":          {"asc subscriptions groups create --app APP_ID --reference-name NAME", "asc subscriptions groups list --app APP_ID"},
		"create-group": {"asc subscriptions groups create --app APP_ID --reference-name NAME", "asc subscriptions groups list --app APP_ID"},
		"show":         {"asc subscriptions groups view --id GROUP_ID", "asc subscriptions groups list --app APP_ID"},
	},
	"asc subscriptions review": {
		"submit": {
			"asc review submissions-create --app APP_ID --platform IOS",
			"asc review items add --submission SUBMISSION_ID --item-type subscriptionVersions --item-id SUBSCRIPTION_VERSION_ID",
			"asc review submissions-submit --id SUBMISSION_ID --confirm",
		},
		"submit-group": {
			"asc review submissions-create --app APP_ID --platform IOS",
			"asc review items add --submission SUBMISSION_ID --item-type subscriptionGroupVersions --item-id GROUP_VERSION_ID",
			"asc review submissions-submit --id SUBMISSION_ID --confirm",
		},
	},
	"asc subscriptions review screenshots": {
		"upload": {"asc subscriptions review screenshots create --subscription-id SUBSCRIPTION_ID --file FILE_PATH"},
		"add":    {"asc subscriptions review screenshots create --subscription-id SUBSCRIPTION_ID --file FILE_PATH"},
		"list":   {"asc subscriptions review screenshots view --screenshot-id SCREENSHOT_ID"},
		"show":   {"asc subscriptions review screenshots view --screenshot-id SCREENSHOT_ID"},
	},
	"asc testflight": {
		"upload":     {"asc publish testflight --app APP_ID --ipa IPA_PATH --group GROUP_NAME", "asc builds upload --app APP_ID --ipa IPA_PATH"},
		"publish":    {"asc publish testflight --app APP_ID --ipa IPA_PATH --group GROUP_NAME", "asc builds upload --app APP_ID --ipa IPA_PATH"},
		"distribute": {"asc publish testflight --app APP_ID --ipa IPA_PATH --group GROUP_NAME", "asc builds upload --app APP_ID --ipa IPA_PATH"},
		"submit":     {"asc publish testflight --app APP_ID --ipa IPA_PATH --group GROUP_NAME", "asc builds upload --app APP_ID --ipa IPA_PATH"},
		"invite": {
			"asc testflight testers add --app APP_ID --email EMAIL --group GROUP_NAME",
			"asc testflight groups add-testers --group GROUP_ID --email EMAIL",
		},
	},
	"asc testflight groups": {
		"add":          {"asc testflight groups create --app APP_ID --name NAME", "asc testflight groups add-testers --group GROUP_ID --email EMAIL"},
		"create-group": {"asc testflight groups create --app APP_ID --name NAME"},
		"new":          {"asc testflight groups create --app APP_ID --name NAME"},
		"invite": {
			"asc testflight groups add-testers --group GROUP_ID --email EMAIL",
			"asc testflight testers add --app APP_ID --email EMAIL --group GROUP_NAME",
		},
	},
	"asc versions": {
		"latest":    {"asc versions list --app APP_ID --latest", "asc versions view --version-id VERSION_ID"},
		"current":   {"asc versions list --app APP_ID --latest", "asc versions view --version-id VERSION_ID"},
		"status":    {"asc versions list --app APP_ID", "asc review status --app APP_ID"},
		"show":      {"asc versions view --version-id VERSION_ID", "asc versions list --app APP_ID"},
		"get-build": {"asc versions view --version-id VERSION_ID --include-build", "asc builds info --build-id BUILD_ID"},
		"build":     {"asc versions view --version-id VERSION_ID --include-build", "asc builds info --build-id BUILD_ID"},
	},
}

// genericChildAliases maps the verbs callers try in any group to the child
// names asc uses for the same task. An alias only fires when the group really
// has that visible child, so it needs no per-group curation and can never name
// a command that does not exist, which is what lets it cover the long tail of
// groups the curated table will never reach. It ranks below that table, whose
// entries also know which flags the target needs, and above the nearest-name
// matcher, which cannot bridge `get` to `view` on its own.
var genericChildAliases = map[string][]string{
	"get":     {"view", "list"},
	"show":    {"view", "list"},
	"fetch":   {"view", "list"},
	"read":    {"view", "list"},
	"ls":      {"list"},
	"set":     {"edit", "update"},
	"update":  {"edit"},
	"modify":  {"edit", "update"},
	"change":  {"edit", "update"},
	"add":     {"create", "register"},
	"new":     {"create"},
	"make":    {"create"},
	"remove":  {"delete", "revoke"},
	"rm":      {"delete", "revoke"},
	"del":     {"delete"},
	"destroy": {"delete"},
}

// appIDOperandLeaf names, per command group, the leaf whose --app takes the App
// Store Connect app ID callers give the group as a bare operand (`asc versions
// 1234567890`). Groups whose operand could also be a build or version number
// are left out.
var appIDOperandLeaf = map[string]string{
	"asc apps info": "view",
	"asc versions":  "list",
}

// appIDOperandPattern matches an App Store Connect app ID. App IDs have at
// least nine digits, so a short number, which could be a version or build
// number, never matches.
var appIDOperandPattern = regexp.MustCompile(`^[1-9][0-9]{8,19}$`)

// unknownChildSuggestions returns the strongest non-empty suggestion tier for
// an unknown subcommand of command (rendered as commandName): curated synonyms
// for the exact token, then generic verb aliases the group can satisfy, then
// nearest-name matches among the group's visible subcommands. Tiers are not
// mixed: weaker guesses do not pad an already useful answer. Generic and fuzzy
// matches point to command help because the matcher cannot know which required
// flags the caller intended; curated entries carry complete invocations.
func unknownChildSuggestions(command *ffcli.Command, commandName, token string) []string {
	suggestions := make([]string, 0, maxUnknownChildSuggestions)
	seen := make(map[string]struct{}, maxUnknownChildSuggestions)
	add := func(invocation string) {
		if _, ok := seen[invocation]; ok || len(suggestions) >= maxUnknownChildSuggestions {
			return
		}
		seen[invocation] = struct{}{}
		suggestions = append(suggestions, invocation)
	}

	if leaf, ok := appIDOperandLeaf[commandName]; ok && appIDOperandPattern.MatchString(token) {
		return []string{commandName + " " + leaf + " --app " + token}
	}

	normalized := strings.ToLower(strings.TrimSpace(token))
	curated, isCurated := unknownChildSynonyms[commandName][normalized]
	if isCurated {
		for _, invocation := range curated {
			add(invocation)
		}
		return suggestions
	}

	visible := visibleSubcommandNames(command)
	for _, name := range genericChildAliases[normalized] {
		if !slices.Contains(visible, name) {
			continue
		}
		add(commandName + " " + name + " --help")
	}
	if len(suggestions) > 0 {
		return suggestions
	}

	for _, name := range suggest.Commands(token, visible) {
		add(commandName + " " + shared.SanitizeTerminal(name) + " --help")
	}

	if len(suggestions) == 0 {
		return nil
	}
	return suggestions
}
