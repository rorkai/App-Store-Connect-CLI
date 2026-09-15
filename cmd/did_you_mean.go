package cmd

import (
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/suggest"
)

// maxUnknownChildSuggestions caps the merged `Try:` block for an unknown
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
// Keys are trimmed lowercase tokens. Values are copy-paste valid invocations of
// real leaf commands, anywhere in the tree, using long-form flags that command
// defines and bare uppercase placeholders (APP_ID, VERSION_ID). A synonym may
// point outside its group when the task really lives elsewhere (`asc agreements
// accept` runs through `asc web agreements`). Tests in did_you_mean_test.go
// resolve every entry against the live command tree and reject a key that
// shadows a real subcommand, so keep the table in sync with the commands.
var unknownChildSynonyms = map[string]map[string][]string{
	"asc age-rating": {
		"get":    {"asc age-rating view --app APP_ID", "asc age-rating audit --app APP_ID"},
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
		"get":    {"asc apps view --id APP_ID", "asc apps info view --app APP_ID"},
		"show":   {"asc apps view --id APP_ID", "asc apps info view --app APP_ID"},
		"find":   {"asc apps list --name NAME", "asc apps list --bundle-id BUNDLE_ID"},
		"search": {"asc apps list --name NAME", "asc apps list --bundle-id BUNDLE_ID"},
	},
	"asc auth": {
		"whoami":    {"asc auth status", "asc auth doctor"},
		"me":        {"asc auth status", "asc auth doctor"},
		"current":   {"asc auth status", "asc auth doctor"},
		"show":      {"asc auth status", "asc auth doctor"},
		"check":     {"asc auth status --validate", "asc auth doctor"},
		"verify":    {"asc auth status --validate", "asc auth doctor"},
		"validate":  {"asc auth status --validate", "asc auth doctor"},
		"test":      {"asc auth status --validate", "asc auth doctor"},
		"add":       {"asc auth login --name NAME --key-id KEY_ID --issuer-id ISSUER_ID --private-key KEY_PATH", "asc auth init"},
		"setup":     {"asc auth login --name NAME --key-id KEY_ID --issuer-id ISSUER_ID --private-key KEY_PATH", "asc auth init"},
		"configure": {"asc auth login --name NAME --key-id KEY_ID --issuer-id ISSUER_ID --private-key KEY_PATH", "asc auth init"},
	},
	"asc builds": {
		"latest":  {"asc builds info --app APP_ID --latest", "asc builds list --app APP_ID --limit 1"},
		"current": {"asc builds info --app APP_ID --latest", "asc builds list --app APP_ID --limit 1"},
		"status":  {"asc builds info --app APP_ID --latest", "asc builds wait --app APP_ID --latest"},
		"show":    {"asc builds info --build-id BUILD_ID", "asc builds list --app APP_ID"},
		"get":     {"asc builds info --build-id BUILD_ID", "asc builds list --app APP_ID"},
	},
	"asc iap": {
		"get":  {"asc iap view --id IAP_ID", "asc iap list --app APP_ID"},
		"show": {"asc iap view --id IAP_ID", "asc iap list --app APP_ID"},
		"add":  {"asc iap create --app APP_ID --product-id PRODUCT_ID --ref-name NAME --type TYPE"},
	},
	"asc localizations": {
		"get":  {"asc localizations list --version VERSION_ID", "asc localizations download --version VERSION_ID --path ./localizations"},
		"show": {"asc localizations list --version VERSION_ID", "asc localizations download --version VERSION_ID --path ./localizations"},
		"view": {"asc localizations list --version VERSION_ID", "asc localizations download --version VERSION_ID --path ./localizations"},
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
		"get":  {"asc subscriptions view --id SUBSCRIPTION_ID", "asc subscriptions list --app APP_ID"},
		"show": {"asc subscriptions view --id SUBSCRIPTION_ID", "asc subscriptions list --app APP_ID"},
		"add": {
			"asc subscriptions create --group-id GROUP_ID --reference-name NAME --product-id PRODUCT_ID --subscription-period PERIOD",
		},
	},
	"asc subscriptions groups": {
		"add":          {"asc subscriptions groups create --app APP_ID --reference-name NAME", "asc subscriptions groups list --app APP_ID"},
		"new":          {"asc subscriptions groups create --app APP_ID --reference-name NAME", "asc subscriptions groups list --app APP_ID"},
		"create-group": {"asc subscriptions groups create --app APP_ID --reference-name NAME", "asc subscriptions groups list --app APP_ID"},
		"get":          {"asc subscriptions groups view --id GROUP_ID", "asc subscriptions groups list --app APP_ID"},
		"show":         {"asc subscriptions groups view --id GROUP_ID", "asc subscriptions groups list --app APP_ID"},
	},
	"asc subscriptions review screenshots": {
		"upload": {"asc subscriptions review screenshots create --subscription-id SUBSCRIPTION_ID --file FILE_PATH"},
		"add":    {"asc subscriptions review screenshots create --subscription-id SUBSCRIPTION_ID --file FILE_PATH"},
		"list":   {"asc subscriptions review screenshots view --screenshot-id SCREENSHOT_ID"},
		"get":    {"asc subscriptions review screenshots view --screenshot-id SCREENSHOT_ID"},
		"show":   {"asc subscriptions review screenshots view --screenshot-id SCREENSHOT_ID"},
	},
	"asc testflight": {
		"builds":     {"asc builds list --app APP_ID", "asc builds upload --app APP_ID --ipa IPA_PATH"},
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
		"latest":  {"asc versions list --app APP_ID --latest", "asc versions view --version-id VERSION_ID"},
		"current": {"asc versions list --app APP_ID --latest", "asc versions view --version-id VERSION_ID"},
		"status":  {"asc versions list --app APP_ID", "asc review status --app APP_ID"},
		"show":    {"asc versions view --version-id VERSION_ID", "asc versions list --app APP_ID"},
		"get":     {"asc versions view --version-id VERSION_ID", "asc versions list --app APP_ID"},
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

// unknownChildSuggestions ranks full invocations for an unknown subcommand of
// command (rendered as commandName): curated synonyms for the exact token
// first, then generic verb aliases the group can satisfy, then nearest-name
// matches among the group's visible subcommands. A suggestion whose command
// path an earlier one already uses is dropped so the block never lists `asc
// review submit` twice. The result holds at most maxUnknownChildSuggestions
// entries and is nil when nothing is confident.
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

	normalized := strings.ToLower(strings.TrimSpace(token))
	curated, isCurated := unknownChildSynonyms[commandName][normalized]
	for _, invocation := range curated {
		add(invocation)
	}
	visible := visibleSubcommandNames(command)
	if !isCurated {
		// A curated entry for this exact token is the better answer, so the
		// generic aliases stay out of its way rather than appending a flagless
		// spelling of the same target.
		for _, name := range genericChildAliases[normalized] {
			if !slices.Contains(visible, name) {
				continue
			}
			add(commandName + " " + name)
		}
	}
	for _, name := range suggest.Commands(token, visible) {
		invocation := commandName + " " + shared.SanitizeTerminal(name)
		if suggestionsCoverCommandPath(suggestions, invocation) {
			continue
		}
		add(invocation)
	}

	if len(suggestions) == 0 {
		return nil
	}
	return suggestions
}

func suggestionsCoverCommandPath(suggestions []string, commandPath string) bool {
	for _, suggestion := range suggestions {
		if suggestion == commandPath || strings.HasPrefix(suggestion, commandPath+" ") {
			return true
		}
	}
	return false
}
