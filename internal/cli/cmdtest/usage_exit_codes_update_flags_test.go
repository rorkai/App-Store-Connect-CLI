package cmdtest

import (
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// TestUpdateWithoutUpdateFlagsListsEveryUpdateFlag locks the message an update
// command prints when none of its update flags is given: one line naming every
// flag that counts as an update, the usage page, exit code 2, and the
// required_input_missing diagnostic.
func TestUpdateWithoutUpdateFlagsListsEveryUpdateFlag(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		flags   string
		warning string // a line the command prints before the error
		deps    bool   // the command is built on shared.Deps: run it on test deps
	}{
		{
			name:  "accessibility update",
			args:  []string{"accessibility", "update", "--id", "ID"},
			flags: "--publish, --supports-audio-descriptions, --supports-captions, --supports-dark-interface, --supports-differentiate-without-color-alone, --supports-larger-text, --supports-reduced-motion, --supports-sufficient-contrast, --supports-voice-control, --supports-voiceover",
			deps:  true,
		},
		{
			name:  "age-rating edit",
			args:  []string{"age-rating", "edit", "--id", "ID"},
			flags: "--all-none, --dating-and-relationships, --shared-location, --advertising, --gambling, --health-or-wellness-topics, --loot-box, --messaging-and-chat, --parental-controls, --age-assurance, --social-media, --social-media-age-restricted, --unrestricted-web-access, --user-generated-content, --alcohol-tobacco-drug-use, --contests, --gambling-simulated, --guns-or-other-weapons, --medical-treatment, --profanity-humor, --sexual-content-nudity, --sexual-content-graphic-nudity, --horror-fear, --mature-suggestive, --violence-cartoon, --violence-realistic, --violence-realistic-graphic, --kids-age-band, --age-rating-override, --age-rating-override-v2, --korea-age-rating-override, --grac-rating-classification-number, --clear-grac-rating-classification-number, --developer-age-rating-info-url",
			deps:  true,
		},
		{
			name:  "android-ios-mapping update",
			args:  []string{"android-ios-mapping", "update", "--mapping-id", "ID"},
			flags: "--android-package-name, --fingerprints, --clear-android-package-name, --clear-fingerprints",
			deps:  true,
		},
		{
			name:  "app-clips advanced-experiences update",
			args:  []string{"app-clips", "advanced-experiences", "update", "--experience-id", "ID"},
			flags: "--app-clip-id, --action, --category, --place, --default-language, --is-powered-by, --removed, --header-image-id, --localization-id",
			deps:  true,
		},
		{
			name:  "app-clips default-experiences localizations update",
			args:  []string{"app-clips", "default-experiences", "localizations", "update", "--localization-id", "ID"},
			flags: "--subtitle",
			deps:  true,
		},
		{
			name:  "app-clips default-experiences update",
			args:  []string{"app-clips", "default-experiences", "update", "--experience-id", "ID"},
			flags: "--action, --release-version-id",
			deps:  true,
		},
		{
			name:  "app-clips invocations localizations update",
			args:  []string{"app-clips", "invocations", "localizations", "update", "--localization-id", "ID"},
			flags: "--title",
			deps:  true,
		},
		{
			name:  "app-clips invocations update",
			args:  []string{"app-clips", "invocations", "update", "--invocation-id", "ID"},
			flags: "--url",
			deps:  true,
		},
		{
			name:  "app-clips review-details update",
			args:  []string{"app-clips", "review-details", "update", "--id", "ID"},
			flags: "--url",
			deps:  true,
		},
		{
			name:  "app-events localizations update",
			args:  []string{"app-events", "localizations", "update", "--localization-id", "ID"},
			flags: "--name, --short-description, --long-description",
			deps:  true,
		},
		{
			name:  "app-events update",
			args:  []string{"app-events", "update", "--event-id", "ID"},
			flags: "--name, --event-type, --start, --end, --publish-start, --territories, --deep-link, --purchase-requirement, --primary-locale, --priority, --purpose",
			deps:  true,
		},
		{
			name:  "app-setup info set",
			args:  []string{"app-setup", "info", "set", "--app", "123456789"},
			flags: "--bundle-id, --primary-locale, --name, --subtitle, --privacy-policy-url, --privacy-choices-url, --privacy-policy-text, --content-rights",
			deps:  true,
		},
		{
			name:  "apps info edit",
			args:  []string{"apps", "info", "edit", "--app", "123456789", "--locale", "en-US"},
			flags: "--copy-from-locale, --from-dir, --description, --keywords, --support-url, --marketing-url, --promotional-text, --whats-new",
			deps:  true,
		},
		{
			name:  "apps update",
			args:  []string{"apps", "update", "--id", "ID"},
			flags: "--bundle-id, --primary-locale, --content-rights, --accessibility-url, --subscription-status-url, --sandbox-subscription-status-url, --streamlined-purchasing-enabled",
			deps:  true,
		},
		{
			name:  "build-localizations update",
			args:  []string{"build-localizations", "update", "--id", "ID"},
			flags: "--whats-new",
			deps:  true,
		},
		{
			name:  "builds test-notes update",
			args:  []string{"builds", "test-notes", "update", "--localization-id", "ID"},
			flags: "--whats-new",
			deps:  true,
		},
		{
			name:  "builds update",
			args:  []string{"builds", "update", "--build-id", "ID"},
			flags: "--uses-non-exempt-encryption",
			deps:  true,
		},
		{
			name:  "bundle-ids capabilities update",
			args:  []string{"bundle-ids", "capabilities", "update", "--id", "ID"},
			flags: "--capability, --settings",
			deps:  true,
		},
		{
			name:  "devices update",
			args:  []string{"devices", "update", "--id", "ID"},
			flags: "--name, --status",
			deps:  true,
		},
		{
			name:  "eula update",
			args:  []string{"eula", "update", "--id", "ID"},
			flags: "--agreement-text, --territory",
			deps:  true,
		},
		{
			name:  "game-center achievements localizations update",
			args:  []string{"game-center", "achievements", "localizations", "update", "--id", "ID"},
			flags: "--name, --before-earned-description, --after-earned-description",
			deps:  true,
		},
		{
			name:  "game-center achievements update",
			args:  []string{"game-center", "achievements", "update", "--id", "ID"},
			flags: "--reference-name, --points, --show-before-earned, --repeatable, --archived, --activity-properties",
			deps:  true,
		},
		{
			name:  "game-center achievements v2 localizations update",
			args:  []string{"game-center", "achievements", "v2", "localizations", "update", "--id", "ID"},
			flags: "--name, --before-earned-description, --after-earned-description",
			deps:  true,
		},
		{
			name:  "game-center activities localizations update",
			args:  []string{"game-center", "activities", "localizations", "update", "--id", "ID"},
			flags: "--name, --description",
			deps:  true,
		},
		{
			name:  "game-center activities update",
			args:  []string{"game-center", "activities", "update", "--id", "ID"},
			flags: "--reference-name, --play-style, --min-players, --max-players, --supports-party-code, --archived, --properties",
			deps:  true,
		},
		{
			name:  "game-center challenges localizations update",
			args:  []string{"game-center", "challenges", "localizations", "update", "--id", "ID"},
			flags: "--name, --description",
			deps:  true,
		},
		{
			name:  "game-center challenges update",
			args:  []string{"game-center", "challenges", "update", "--id", "ID"},
			flags: "--reference-name, --repeatable, --archived, --leaderboard-id",
			deps:  true,
		},
		{
			name:  "game-center leaderboard-sets localizations update",
			args:  []string{"game-center", "leaderboard-sets", "localizations", "update", "--id", "ID"},
			flags: "--name",
			deps:  true,
		},
		{
			name:  "game-center leaderboard-sets update",
			args:  []string{"game-center", "leaderboard-sets", "update", "--id", "ID"},
			flags: "--reference-name",
			deps:  true,
		},
		{
			name:  "game-center leaderboard-sets v2 update",
			args:  []string{"game-center", "leaderboard-sets", "v2", "update", "--id", "ID"},
			flags: "--reference-name",
			deps:  true,
		},
		{
			name:  "game-center leaderboards localizations update",
			args:  []string{"game-center", "leaderboards", "localizations", "update", "--id", "ID"},
			flags: "--name, --formatter-override, --formatter-suffix, --formatter-suffix-singular, --description",
			deps:  true,
		},
		{
			name:  "game-center leaderboards update",
			args:  []string{"game-center", "leaderboards", "update", "--id", "ID"},
			flags: "--reference-name, --archived, --formatter, --sort, --submission-type, --score-range-start, --score-range-end, --recurrence-start-date, --recurrence-duration, --recurrence-rule, --visibility, --activity-properties",
			deps:  true,
		},
		{
			name:  "game-center leaderboards v2 localizations update",
			args:  []string{"game-center", "leaderboards", "v2", "localizations", "update", "--id", "ID"},
			flags: "--name, --formatter-override, --formatter-suffix, --formatter-suffix-singular, --description",
			deps:  true,
		},
		{
			name:  "game-center matchmaking queues update",
			args:  []string{"game-center", "matchmaking", "queues", "update", "--id", "ID"},
			flags: "--rule-set-id, --experiment-rule-set-id, --classic-bundle-ids",
			deps:  true,
		},
		{
			name:  "game-center matchmaking rule-sets update",
			args:  []string{"game-center", "matchmaking", "rule-sets", "update", "--id", "ID"},
			flags: "--min-players, --max-players",
			deps:  true,
		},
		{
			name:  "game-center matchmaking rules update",
			args:  []string{"game-center", "matchmaking", "rules", "update", "--id", "ID"},
			flags: "--description, --expression, --weight",
			deps:  true,
		},
		{
			name:  "game-center matchmaking teams update",
			args:  []string{"game-center", "matchmaking", "teams", "update", "--id", "ID"},
			flags: "--min-players, --max-players",
			deps:  true,
		},
		{
			name:  "iap promoted-purchases update",
			args:  []string{"iap", "promoted-purchases", "update", "--promoted-purchase-id", "ID"},
			flags: "--visible-for-all-users, --enabled",
			deps:  true,
		},
		{
			name:  "iap review-screenshots update",
			args:  []string{"iap", "review-screenshots", "update", "--screenshot-id", "ID"},
			flags: "--checksum, --uploaded, --file",
			deps:  true,
		},
		{
			name:  "iap update",
			args:  []string{"iap", "update", "--id", "ID"},
			flags: "--ref-name, --family-sharable, --review-note",
			deps:  true,
		},
		{
			name:  "iap versions localizations update",
			args:  []string{"iap", "versions", "localizations", "update", "--localization-id", "ID"},
			flags: "--name, --clear-name, --description, --clear-description",
			deps:  true,
		},
		{
			name:  "localizations update",
			args:  []string{"localizations", "update", "--locale", "en-US"},
			flags: "--description, --keywords, --whats-new, --promotional-text, --clear-promotional-text, --support-url, --marketing-url",
			deps:  true,
		},
		{
			name:  "localizations update app-info",
			args:  []string{"localizations", "update", "--type", "app-info", "--locale", "en-US"},
			flags: "--name, --subtitle, --clear-subtitle, --privacy-policy-url, --clear-privacy-policy-url, --privacy-choices-url, --privacy-policy-text",
			deps:  true,
		},
		{
			name:  "marketplace search-details update",
			args:  []string{"marketplace", "search-details", "update", "--search-detail-id", "ID"},
			flags: "--catalog-url",
			deps:  true,
		},
		{
			name:    "marketplace webhooks update",
			args:    []string{"marketplace", "webhooks", "update", "--webhook-id", "ID"},
			flags:   "--url, --secret",
			warning: "Warning: marketplace webhooks endpoints are deprecated in App Store Connect API.\n",
			deps:    true,
		},
		{
			name:  "nominations update",
			args:  []string{"nominations", "update", "--id", "ID"},
			flags: "--name, --type, --description, --submitted, --archived, --publish-start-date, --publish-end-date, --device-families, --locales, --supplemental-materials-uris, --has-in-app-events, --launch-in-select-markets-first, --notes, --pre-order-enabled, --app, --in-app-events, --supported-territories",
			deps:  true,
		},
		{
			name:  "pre-orders update",
			args:  []string{"pre-orders", "update", "--territory-availability", "ID"},
			flags: "--release-date, --pre-order-enabled, --available",
			deps:  true,
		},
		{
			name:  "product-pages custom-pages update",
			args:  []string{"product-pages", "custom-pages", "update", "--custom-page-id", "ID"},
			flags: "--name, --visible",
			deps:  true,
		},
		{
			name:  "product-pages experiments treatments update",
			args:  []string{"product-pages", "experiments", "treatments", "update", "--treatment-id", "ID"},
			flags: "--name, --app-icon-name",
			deps:  true,
		},
		{
			name:  "product-pages experiments update",
			args:  []string{"product-pages", "experiments", "update", "--experiment-id", "ID"},
			flags: "--name, --traffic-proportion, --started",
			deps:  true,
		},
		{
			name:  "review details-update",
			args:  []string{"review", "details-update", "--id", "ID"},
			deps:  true,
			flags: "--contact-first-name, --contact-last-name, --contact-email, --contact-phone, --demo-account-name, --demo-account-password, --demo-account-required, --notes",
		},
		{
			name:  "review items update",
			args:  []string{"review", "items", "update", "--id", "ID"},
			deps:  true,
			flags: "--resolved, --removed, --clear-resolved, --clear-removed",
		},
		{
			name:  "review items-update",
			args:  []string{"review", "items-update", "--id", "ID"},
			deps:  true,
			flags: "--resolved, --removed, --clear-resolved, --clear-removed",
		},
		{
			name:  "review submissions-update",
			args:  []string{"review", "submissions-update", "--id", "ID"},
			deps:  true,
			flags: "--platform, --submitted, --canceled, --clear-platform, --clear-submitted, --clear-canceled",
		},
		{
			name:  "sandbox update",
			args:  []string{"sandbox", "update", "--id", "ID"},
			flags: "--territory, --subscription-renewal-rate, --interrupt-purchases",
			deps:  true,
		},
		{
			name:  "subscriptions grace-periods update",
			args:  []string{"subscriptions", "grace-periods", "update", "--id", "ID"},
			flags: "--opt-in, --sandbox-opt-in, --duration, --renewal-type",
			deps:  true,
		},
		{
			name:  "subscriptions groups update",
			args:  []string{"subscriptions", "groups", "update", "--id", "ID"},
			flags: "--reference-name",
			deps:  true,
		},
		{
			name:  "subscriptions groups versions localizations update",
			args:  []string{"subscriptions", "groups", "versions", "localizations", "update", "--id", "ID"},
			flags: "--name, --custom-app-name, --clear-name, --clear-custom-app-name",
			deps:  true,
		},
		{
			name:  "subscriptions offers introductory update",
			args:  []string{"subscriptions", "offers", "introductory", "update", "--id", "ID"},
			flags: "--end-date",
			deps:  true,
		},
		{
			name:  "subscriptions offers retention update",
			args:  []string{"subscriptions", "offers", "retention", "update", "--id", "ID"},
			flags: "--eligibility-paid-months, --eligibility-wait-months",
			deps:  true,
		},
		{
			name:  "subscriptions offers win-back update",
			args:  []string{"subscriptions", "offers", "win-back", "update", "--id", "ID"},
			flags: "--eligibility-paid-months, --eligibility-last-subscribed-min, --eligibility-last-subscribed-max, --eligibility-wait-months, --start-date, --end-date, --priority, --promotion-intent",
			deps:  true,
		},
		{
			name:  "subscriptions promoted-purchases update",
			args:  []string{"subscriptions", "promoted-purchases", "update", "--promoted-purchase-id", "ID"},
			flags: "--visible-for-all-users, --enabled",
			deps:  true,
		},
		{
			name:  "subscriptions retention-messages localizations update",
			args:  []string{"subscriptions", "retention-messages", "localizations", "update", "--id", "ID"},
			flags: "--header, --body, --clear-header, --clear-body",
			deps:  true,
		},
		{
			name:  "subscriptions review screenshots update",
			args:  []string{"subscriptions", "review", "screenshots", "update", "--screenshot-id", "ID"},
			flags: "--checksum, --uploaded",
			deps:  true,
		},
		{
			name:  "subscriptions update",
			args:  []string{"subscriptions", "update", "--id", "ID"},
			flags: "--reference-name, --review-note, --subscription-period, --group-level, --family-sharable, --market-settings, --multi-seat-status",
			deps:  true,
		},
		{
			name:  "subscriptions versions localizations update",
			args:  []string{"subscriptions", "versions", "localizations", "update", "--id", "ID"},
			flags: "--name, --description, --clear-name, --clear-description",
			deps:  true,
		},
		{
			name:  "testflight app-localizations update",
			args:  []string{"testflight", "app-localizations", "update", "--id", "ID"},
			deps:  true,
			flags: "--description, --feedback-email, --marketing-url, --privacy-policy-url, --tv-os-privacy-policy",
		},
		{
			name:  "testflight distribution edit",
			args:  []string{"testflight", "distribution", "edit", "--id", "ID"},
			deps:  true,
			flags: "--auto-notify",
		},
		{
			name:  "testflight groups edit",
			args:  []string{"testflight", "groups", "edit", "--id", "ID"},
			deps:  true,
			flags: "--name, --public-link-enabled, --public-link-limit-enabled, --public-link-limit, --feedback-enabled, --ios-builds-available-for-apple-silicon-mac, --ios-builds-available-for-apple-vision",
		},
		{
			name:  "testflight review edit",
			args:  []string{"testflight", "review", "edit", "--id", "ID"},
			deps:  true,
			flags: "--contact-first-name, --contact-last-name, --contact-email, --contact-phone, --demo-account-name, --demo-account-password, --demo-account-required, --notes",
		},
		{
			name:  "versions update",
			args:  []string{"versions", "update", "--version-id", "ID"},
			flags: "--copyright, --release-type, --review-type, --earliest-release-date, --version, --downloadable",
			deps:  true,
		},
		{
			name:  "web apps compatibility edit",
			args:  []string{"web", "apps", "compatibility", "edit", "--app", "123456789"},
			flags: "--ios-app-on-mac, --ios-app-on-vision-pro",
			deps:  true,
		},
		{
			name:  "webhooks update",
			args:  []string{"webhooks", "update", "--webhook-id", "ID"},
			flags: "--name, --url, --secret, --events, --enabled",
			deps:  true,
		},
		{
			name:  "xcode version edit",
			args:  []string{"xcode", "version", "edit"},
			flags: "--version, --build-number, --next-build-number",
			deps:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr string
			var runErr error
			if test.deps {
				stdout, stderr, runErr = runRootWithDeps(t, nil, test.args...)
			} else {
				stdout, stderr, runErr = runCommand(t, test.args)
			}

			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			wantLine := test.warning + "Error: at least one update flag is required (" + test.flags + ")\n"
			if !strings.HasPrefix(stderr, wantLine) {
				t.Fatalf("stderr = %q, want it to start with %q", stderr, wantLine)
			}
			if got := strings.Count(stderr, "Error: "); got != 1 {
				t.Fatalf("stderr = %q, want exactly one %q line, got %d", stderr, "Error: ", got)
			}
			if !strings.Contains(stderr, "\nUSAGE\n") {
				t.Fatalf("stderr = %q, want the usage page", stderr)
			}
			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok || diagnostic.Code != shared.DiagnosticRequiredInputMissing {
				t.Fatalf("diagnostic = %+v (ok=%t), want code %q", diagnostic, ok, shared.DiagnosticRequiredInputMissing)
			}
		})
	}
}
