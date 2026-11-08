package cmdtest

import (
	"path/filepath"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// TestCommandGroupInputValidationReturnsUsageExitCode locks the usage-error
// contract for pre-request flag validation across the app, build, content, and
// review command groups. Every case below rejects a flag value before any
// App Store Connect client is constructed, so each must print
// "Error: <message>" to stderr and exit with the documented usage code 2
// rather than the generic runtime failure code 1.
//
// The sibling analytics coverage in analytics_usage_exit_codes_test.go pins the
// same contract for `asc analytics`; this file extends it to the remaining
// groups tracked by issue #518.
func TestCommandGroupInputValidationReturnsUsageExitCode(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "accessibility list limit",
			args:    []string{"accessibility", "list", "--limit", "201"},
			wantErr: "accessibility list: --limit must be between 1 and 200",
		},
		{
			name:    "accessibility list next",
			args:    []string{"accessibility", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "accessibility list: --next must be an App Store Connect URL",
		},
		{
			name:    "app-clips advanced-experiences list limit",
			args:    []string{"app-clips", "advanced-experiences", "list", "--limit", "201"},
			wantErr: "app-clips advanced-experiences list: --limit must be between 1 and 200",
		},
		{
			name:    "app-clips advanced-experiences list next",
			args:    []string{"app-clips", "advanced-experiences", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-clips advanced-experiences list: --next must be an App Store Connect URL",
		},
		{
			name:    "app-clips advanced-experiences-links limit",
			args:    []string{"app-clips", "advanced-experiences-links", "--limit", "201"},
			wantErr: "app-clips advanced-experiences-links: --limit must be between 1 and 200",
		},
		{
			name:    "app-clips advanced-experiences-links next",
			args:    []string{"app-clips", "advanced-experiences-links", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-clips advanced-experiences-links: --next must be an App Store Connect URL",
		},
		{
			name:    "app-clips default-experiences list limit",
			args:    []string{"app-clips", "default-experiences", "list", "--limit", "201"},
			wantErr: "app-clips default-experiences list: --limit must be between 1 and 200",
		},
		{
			name:    "app-clips default-experiences list next",
			args:    []string{"app-clips", "default-experiences", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-clips default-experiences list: --next must be an App Store Connect URL",
		},
		{
			name:    "app-clips default-experiences localizations list limit",
			args:    []string{"app-clips", "default-experiences", "localizations", "list", "--limit", "201"},
			wantErr: "app-clips default-experiences localizations list: --limit must be between 1 and 200",
		},
		{
			name:    "app-clips default-experiences localizations list next",
			args:    []string{"app-clips", "default-experiences", "localizations", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-clips default-experiences localizations list: --next must be an App Store Connect URL",
		},
		{
			name:    "app-clips default-experiences-links limit",
			args:    []string{"app-clips", "default-experiences-links", "--limit", "201"},
			wantErr: "app-clips default-experiences-links: --limit must be between 1 and 200",
		},
		{
			name:    "app-clips default-experiences-links next",
			args:    []string{"app-clips", "default-experiences-links", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-clips default-experiences-links: --next must be an App Store Connect URL",
		},
		{
			name:    "app-clips invocations list limit",
			args:    []string{"app-clips", "invocations", "list", "--limit", "201"},
			wantErr: "app-clips invocations list: --limit must be between 1 and 200",
		},
		{
			name:    "app-clips invocations list next",
			args:    []string{"app-clips", "invocations", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-clips invocations list: --next must be an App Store Connect URL",
		},
		{
			name:    "app-clips invocations localizations list limit",
			args:    []string{"app-clips", "invocations", "localizations", "list", "--limit", "201"},
			wantErr: "app-clips invocations localizations list: --limit must be between 1 and 200",
		},
		{
			name:    "app-clips list limit",
			args:    []string{"app-clips", "list", "--limit", "201"},
			wantErr: "app-clips list: --limit must be between 1 and 200",
		},
		{
			name:    "app-clips list next",
			args:    []string{"app-clips", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-clips list: --next must be an App Store Connect URL",
		},
		{
			name:    "app-events links limit",
			args:    []string{"app-events", "links", "--limit", "201"},
			wantErr: "app-events links: --limit must be between 1 and 200",
		},
		{
			name:    "app-events links next",
			args:    []string{"app-events", "links", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-events links: --next must be an App Store Connect URL",
		},
		{
			name:    "app-events list next",
			args:    []string{"app-events", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-events list: --next must be an App Store Connect URL",
		},
		{
			name:    "app-events localizations list limit",
			args:    []string{"app-events", "localizations", "list", "--event-id", "E", "--limit", "201"},
			wantErr: "app-events localizations list: --limit must be between 1 and 200",
		},
		{
			name:    "app-events localizations list next",
			args:    []string{"app-events", "localizations", "list", "--event-id", "E", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-events localizations list: --next must be an App Store Connect URL",
		},
		{
			name:    "app-events localizations screenshots list limit",
			args:    []string{"app-events", "localizations", "screenshots", "list", "--limit", "201"},
			wantErr: "app-events localizations screenshots list: --limit must be between 1 and 200",
		},
		{
			name:    "app-events localizations screenshots list next",
			args:    []string{"app-events", "localizations", "screenshots", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-events localizations screenshots list: --next must be an App Store Connect URL",
		},
		{
			name:    "app-events localizations screenshots-links limit",
			args:    []string{"app-events", "localizations", "screenshots-links", "--limit", "201"},
			wantErr: "app-events localizations screenshots-links: --limit must be between 1 and 200",
		},
		{
			name:    "app-events localizations screenshots-links next",
			args:    []string{"app-events", "localizations", "screenshots-links", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-events localizations screenshots-links: --next must be an App Store Connect URL",
		},
		{
			name:    "app-events localizations video-clips list limit",
			args:    []string{"app-events", "localizations", "video-clips", "list", "--limit", "201"},
			wantErr: "app-events localizations video-clips list: --limit must be between 1 and 200",
		},
		{
			name:    "app-events localizations video-clips list next",
			args:    []string{"app-events", "localizations", "video-clips", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-events localizations video-clips list: --next must be an App Store Connect URL",
		},
		{
			name:    "app-events localizations video-clips-links limit",
			args:    []string{"app-events", "localizations", "video-clips-links", "--limit", "201"},
			wantErr: "app-events localizations video-clips-links: --limit must be between 1 and 200",
		},
		{
			name:    "app-events localizations video-clips-links next",
			args:    []string{"app-events", "localizations", "video-clips-links", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-events localizations video-clips-links: --next must be an App Store Connect URL",
		},
		{
			name:    "app-events screenshots links limit",
			args:    []string{"app-events", "screenshots", "links", "--limit", "201"},
			wantErr: "app-events screenshots links: --limit must be between 1 and 200",
		},
		{
			name:    "app-events screenshots links next",
			args:    []string{"app-events", "screenshots", "links", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-events screenshots links: --next must be an App Store Connect URL",
		},
		{
			name:    "app-events screenshots list limit",
			args:    []string{"app-events", "screenshots", "list", "--limit", "201"},
			wantErr: "app-events screenshots list: --limit must be between 1 and 200",
		},
		{
			name:    "app-events screenshots list next",
			args:    []string{"app-events", "screenshots", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-events screenshots list: --next must be an App Store Connect URL",
		},
		{
			name:    "app-events video-clips links limit",
			args:    []string{"app-events", "video-clips", "links", "--limit", "201"},
			wantErr: "app-events video-clips links: --limit must be between 1 and 200",
		},
		{
			name:    "app-events video-clips links next",
			args:    []string{"app-events", "video-clips", "links", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-events video-clips links: --next must be an App Store Connect URL",
		},
		{
			name:    "app-events video-clips list limit",
			args:    []string{"app-events", "video-clips", "list", "--limit", "201"},
			wantErr: "app-events video-clips list: --limit must be between 1 and 200",
		},
		{
			name:    "app-events video-clips list next",
			args:    []string{"app-events", "video-clips", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-events video-clips list: --next must be an App Store Connect URL",
		},
		{
			name:    "app-tags links limit",
			args:    []string{"app-tags", "links", "--limit", "201"},
			wantErr: "app-tags links: --limit must be between 1 and 200",
		},
		{
			name:    "app-tags links next",
			args:    []string{"app-tags", "links", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-tags links: --next must be an App Store Connect URL",
		},
		{
			name:    "app-tags territories limit",
			args:    []string{"app-tags", "territories", "--id", "X", "--limit", "201"},
			wantErr: "app-tags territories: --limit must be between 1 and 200",
		},
		{
			name:    "app-tags territories next",
			args:    []string{"app-tags", "territories", "--id", "X", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-tags territories: --next must be an App Store Connect URL",
		},
		{
			name:    "app-tags territories-links limit",
			args:    []string{"app-tags", "territories-links", "--id", "X", "--limit", "201"},
			wantErr: "app-tags territories-links: --limit must be between 1 and 200",
		},
		{
			name:    "app-tags territories-links next",
			args:    []string{"app-tags", "territories-links", "--id", "X", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "app-tags territories-links: --next must be an App Store Connect URL",
		},
		{
			name:    "app-tags view territory-limit",
			args:    []string{"app-tags", "view", "--app", "123456789", "--id", "X", "--territory-limit", "51"},
			wantErr: "app-tags view: --territory-limit must be between 1 and 50",
		},
		{
			name:    "apps app-encryption-declarations list build-limit",
			args:    []string{"apps", "app-encryption-declarations", "list", "--build-limit", "51"},
			wantErr: "apps app-encryption-declarations list: --build-limit must be between 1 and 50",
		},
		{
			name:    "apps app-encryption-declarations list limit",
			args:    []string{"apps", "app-encryption-declarations", "list", "--limit", "201"},
			wantErr: "apps app-encryption-declarations list: --limit must be between 1 and 200",
		},
		{
			name:    "apps app-encryption-declarations list next",
			args:    []string{"apps", "app-encryption-declarations", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "apps app-encryption-declarations list: --next must be an App Store Connect URL",
		},
		{
			name:    "apps info territory-age-ratings list limit",
			args:    []string{"apps", "info", "territory-age-ratings", "list", "--limit", "201"},
			wantErr: "apps info territory-age-ratings list: --limit must be between 1 and 200",
		},
		{
			name:    "apps info territory-age-ratings list next",
			args:    []string{"apps", "info", "territory-age-ratings", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "apps info territory-age-ratings list: --next must be an App Store Connect URL",
		},
		{
			name:    "apps search-keywords list limit",
			args:    []string{"apps", "search-keywords", "list", "--limit", "201"},
			wantErr: "apps search-keywords list: --limit must be between 1 and 200",
		},
		{
			name:    "apps search-keywords list next",
			args:    []string{"apps", "search-keywords", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "apps search-keywords list: --next must be an App Store Connect URL",
		},
		{
			name:    "build-bundles app-clip invocations list limit",
			args:    []string{"build-bundles", "app-clip", "invocations", "list", "--limit", "201"},
			wantErr: "build-bundles app-clip invocations list: --limit must be between 1 and 200",
		},
		{
			name:    "build-bundles app-clip invocations list next",
			args:    []string{"build-bundles", "app-clip", "invocations", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "build-bundles app-clip invocations list: --next must be an App Store Connect URL",
		},
		{
			name:    "build-bundles file-sizes list limit",
			args:    []string{"build-bundles", "file-sizes", "list", "--limit", "201"},
			wantErr: "build-bundles file-sizes list: --limit must be between 1 and 200",
		},
		{
			name:    "build-bundles file-sizes list next",
			args:    []string{"build-bundles", "file-sizes", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "build-bundles file-sizes list: --next must be an App Store Connect URL",
		},
		{
			name:    "build-bundles list limit",
			args:    []string{"build-bundles", "list", "--limit", "201"},
			wantErr: "build-bundles list: --limit must be between 1 and 50",
		},
		{
			name:    "build-localizations list limit",
			args:    []string{"build-localizations", "list", "--limit", "201"},
			wantErr: "build-localizations list: --limit must be between 1 and 200",
		},
		{
			name:    "build-localizations list next",
			args:    []string{"build-localizations", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "build-localizations list: --next must be an App Store Connect URL",
		},
		{
			name:    "builds icons list limit",
			args:    []string{"builds", "icons", "list", "--limit", "201"},
			wantErr: "builds icons list: --limit must be between 1 and 200",
		},
		{
			name:    "builds icons list next",
			args:    []string{"builds", "icons", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "builds icons list: --next must be an App Store Connect URL",
		},
		{
			name:    "builds individual-testers list limit",
			args:    []string{"builds", "individual-testers", "list", "--limit", "201"},
			wantErr: "builds individual-testers list: --limit must be between 1 and 200",
		},
		{
			name:    "builds individual-testers list next",
			args:    []string{"builds", "individual-testers", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "builds individual-testers list: --next must be an App Store Connect URL",
		},
		{
			name:    "builds links view limit",
			args:    []string{"builds", "links", "view", "--limit", "201"},
			wantErr: "builds links view: --limit must be between 1 and 200",
		},
		{
			name:    "builds links view next",
			args:    []string{"builds", "links", "view", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "builds links view: --next must be an App Store Connect URL",
		},
		{
			name:    "builds metrics beta-usages next",
			args:    []string{"builds", "metrics", "beta-usages", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "builds metrics beta-usages: --next must be an App Store Connect URL",
		},
		{
			name:    "builds test-notes list limit",
			args:    []string{"builds", "test-notes", "list", "--limit", "201"},
			wantErr: "builds test-notes list: --limit must be between 1 and 200",
		},
		{
			name:    "builds test-notes list next",
			args:    []string{"builds", "test-notes", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "builds test-notes list: --next must be an App Store Connect URL",
		},
		{
			name:    "builds uploads files list limit",
			args:    []string{"builds", "uploads", "files", "list", "--limit", "201"},
			wantErr: "builds uploads files list: --limit must be between 1 and 200",
		},
		{
			name:    "builds uploads files list next",
			args:    []string{"builds", "uploads", "files", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "builds uploads files list: --next must be an App Store Connect URL",
		},
		{
			name:    "builds uploads list next",
			args:    []string{"builds", "uploads", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "builds uploads list: --next must be an App Store Connect URL",
		},
		{
			name:    "builds uploads list owner with next",
			args:    []string{"builds", "uploads", "list", "--app", "APP_A", "--next", "https://api.appstoreconnect.apple.com/v1/apps/APP_B/buildUploads?cursor=next"},
			wantErr: "builds uploads list: --next cannot be combined with --app",
		},
		{
			name:    "localizations download limit",
			args:    []string{"localizations", "download", "--limit", "201"},
			wantErr: "localizations download: --limit must be between 1 and 200",
		},
		{
			name:    "localizations download next",
			args:    []string{"localizations", "download", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "localizations download: --next must be an App Store Connect URL",
		},
		{
			name:    "localizations update app info rejects version",
			args:    []string{"localizations", "update", "--type", "app-info", "--app", "app-a", "--version", "version-b", "--locale", "en-US", "--subtitle", "New"},
			wantErr: "--version requires --type version",
		},
		{
			name:    "localizations update version rejects app info",
			args:    []string{"localizations", "update", "--version", "version-a", "--app-info", "info-b", "--locale", "en-US", "--description", "New"},
			wantErr: "--app-info requires --type app-info",
		},
		{
			name:    "localizations list limit",
			args:    []string{"localizations", "list", "--limit", "201"},
			wantErr: "localizations list: --limit must be between 1 and 200",
		},
		{
			name:    "localizations list next",
			args:    []string{"localizations", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "localizations list: --next must be an App Store Connect URL",
		},
		{
			name:    "performance diagnostics rejects unrelated cursor",
			args:    []string{"performance", "diagnostics", "list", "--build-id", "build-a", "--next", "https://api.appstoreconnect.apple.com/v1/apps/6759231657/appInfos?limit=1"},
			wantErr: "performance diagnostics list: --next must point to a build's diagnostic signatures",
		},
		{
			name:    "performance diagnostics rejects a cursor for another build",
			args:    []string{"performance", "diagnostics", "list", "--build-id", "build-a", "--next", "https://api.appstoreconnect.apple.com/v1/builds/build-b/diagnosticSignatures?cursor=AQ"},
			wantErr: "performance diagnostics list: --build-id must match the build in --next",
		},
		{
			name:    "performance diagnostics list limit",
			args:    []string{"performance", "diagnostics", "list", "--build-id", "B", "--limit", "201"},
			wantErr: "performance diagnostics list: --limit must be between 1 and 200",
		},
		{
			name:    "performance diagnostics view limit",
			args:    []string{"performance", "diagnostics", "view", "--id", "I", "--limit", "201"},
			wantErr: "performance diagnostics view: --limit must be between 1 and 200",
		},
		{
			name:    "product-pages custom-pages list limit",
			args:    []string{"product-pages", "custom-pages", "list", "--limit", "201"},
			wantErr: "custom-pages list: --limit must be between 1 and 200",
		},
		{
			name:    "product-pages custom-pages list next",
			args:    []string{"product-pages", "custom-pages", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "custom-pages list: --next must be an App Store Connect URL",
		},
		{
			name:    "product-pages custom-pages localizations list limit",
			args:    []string{"product-pages", "custom-pages", "localizations", "list", "--limit", "201"},
			wantErr: "custom-pages localizations list: --limit must be between 1 and 200",
		},
		{
			name:    "product-pages custom-pages localizations list next",
			args:    []string{"product-pages", "custom-pages", "localizations", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "custom-pages localizations list: --next must be an App Store Connect URL",
		},
		{
			name:    "product-pages custom-pages localizations preview-sets list limit",
			args:    []string{"product-pages", "custom-pages", "localizations", "preview-sets", "list", "--localization-id", "L", "--limit", "201"},
			wantErr: "custom-pages localizations preview-sets list: --limit must be between 1 and 200",
		},
		{
			name:    "product-pages custom-pages localizations preview-sets list next",
			args:    []string{"product-pages", "custom-pages", "localizations", "preview-sets", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "custom-pages localizations preview-sets list: --next must be an App Store Connect URL",
		},
		{
			name:    "product-pages custom-pages localizations screenshot-sets list limit",
			args:    []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list", "--localization-id", "L", "--limit", "201"},
			wantErr: "custom-pages localizations screenshot-sets list: --limit must be between 1 and 200",
		},
		{
			name:    "product-pages custom-pages localizations screenshot-sets list next",
			args:    []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "custom-pages localizations screenshot-sets list: --next must be an App Store Connect URL",
		},
		{
			name:    "product-pages custom-pages versions list limit",
			args:    []string{"product-pages", "custom-pages", "versions", "list", "--limit", "201"},
			wantErr: "custom-pages versions list: --limit must be between 1 and 200",
		},
		{
			name:    "product-pages custom-pages versions list next",
			args:    []string{"product-pages", "custom-pages", "versions", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "custom-pages versions list: --next must be an App Store Connect URL",
		},
		{
			name:    "product-pages experiments list limit",
			args:    []string{"product-pages", "experiments", "list", "--limit", "201"},
			wantErr: "experiments list: --limit must be between 1 and 200",
		},
		{
			name:    "product-pages experiments list next",
			args:    []string{"product-pages", "experiments", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "experiments list: --next must be an App Store Connect URL",
		},
		{
			name:    "product-pages experiments treatments list limit",
			args:    []string{"product-pages", "experiments", "treatments", "list", "--limit", "201"},
			wantErr: "experiments treatments list: --limit must be between 1 and 200",
		},
		{
			name:    "product-pages experiments treatments list next",
			args:    []string{"product-pages", "experiments", "treatments", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "experiments treatments list: --next must be an App Store Connect URL",
		},
		{
			name:    "product-pages experiments treatments localizations list limit",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "list", "--limit", "201"},
			wantErr: "experiments treatments localizations list: --limit must be between 1 and 200",
		},
		{
			name:    "product-pages experiments treatments localizations list next",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "experiments treatments localizations list: --next must be an App Store Connect URL",
		},
		{
			name:    "product-pages experiments treatments localizations preview-sets list limit",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "list", "--localization-id", "L", "--limit", "201"},
			wantErr: "experiments treatments localizations preview-sets list: --limit must be between 1 and 200",
		},
		{
			name:    "product-pages experiments treatments localizations preview-sets list next",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "experiments treatments localizations preview-sets list: --next must be an App Store Connect URL",
		},
		{
			name:    "product-pages experiments treatments localizations screenshot-sets list limit",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list", "--localization-id", "L", "--limit", "201"},
			wantErr: "experiments treatments localizations screenshot-sets list: --limit must be between 1 and 200",
		},
		{
			name:    "product-pages experiments treatments localizations screenshot-sets list next",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "experiments treatments localizations screenshot-sets list: --next must be an App Store Connect URL",
		},
		{
			name:    "review attachments-list limit",
			args:    []string{"review", "attachments-list", "--limit", "201"},
			wantErr: "review attachments-list: --limit must be between 1 and 200",
		},
		{
			name:    "review attachments-list next",
			args:    []string{"review", "attachments-list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "review attachments-list: --next must be an App Store Connect URL",
		},
		{
			name:    "review submissions-items-ids next",
			args:    []string{"review", "submissions-items-ids", "--id", "X", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "review submissions-items-ids: --next must be an App Store Connect URL",
		},
		{
			name:    "review items list next",
			args:    []string{"review", "items", "list", "--submission", "SUBMISSION_ID", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "review items list: --next must be an App Store Connect URL",
		},
		{
			name:    "review items-list next",
			args:    []string{"review", "items-list", "--submission", "SUBMISSION_ID", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "review items-list: --next must be an App Store Connect URL",
		},
		{
			name:    "reviews limit",
			args:    []string{"reviews", "--app", "123456789", "--limit", "201"},
			wantErr: "reviews: --limit must be between 1 and 200",
		},
		{
			name:    "reviews next",
			args:    []string{"reviews", "--app", "123456789", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "reviews: --next must be an App Store Connect URL",
		},
		{
			name:    "reviews list limit",
			args:    []string{"reviews", "list", "--app", "123456789", "--limit", "201"},
			wantErr: "reviews: --limit must be between 1 and 200",
		},
		{
			name:    "reviews list next",
			args:    []string{"reviews", "list", "--app", "123456789", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "reviews: --next must be an App Store Connect URL",
		},
		{
			name:    "reviews summarizations limit",
			args:    []string{"reviews", "summarizations", "--app", "123456789", "--limit", "201"},
			wantErr: "reviews summarizations: --limit must be between 1 and 200",
		},
		{
			name:    "reviews summarizations next",
			args:    []string{"reviews", "summarizations", "--app", "123456789", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "reviews summarizations: --next must be an App Store Connect URL",
		},
		{
			name:    "versions customer-reviews list limit",
			args:    []string{"versions", "customer-reviews", "list", "--limit", "201"},
			wantErr: "versions customer-reviews list: --limit must be between 1 and 200",
		},
		{
			name:    "versions customer-reviews list next",
			args:    []string{"versions", "customer-reviews", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "versions customer-reviews list: --next must be an App Store Connect URL",
		},
		{
			name:    "versions experiments-v2 list limit",
			args:    []string{"versions", "experiments-v2", "list", "--limit", "201"},
			wantErr: "versions experiments-v2 list: --limit must be between 1 and 200",
		},
		{
			name:    "versions experiments-v2 list next",
			args:    []string{"versions", "experiments-v2", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "versions experiments-v2 list: --next must be an App Store Connect URL",
		},
		{
			name:    "versions links limit",
			args:    []string{"versions", "links", "--version-id", "V", "--limit", "201"},
			wantErr: "versions links: --limit must be between 1 and 200",
		},
		{
			name:    "versions links next",
			args:    []string{"versions", "links", "--version-id", "V", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "versions links: --next must be an App Store Connect URL",
		},
		{
			name:    "versions list limit",
			args:    []string{"versions", "list", "--limit", "201"},
			wantErr: "versions list: --limit must be between 1 and 200",
		},
		{
			name:    "versions list next",
			args:    []string{"versions", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "versions list: --next must be an App Store Connect URL",
		},
		{
			name:    "versions list owner with next",
			args:    []string{"versions", "list", "--app", "APP_A", "--next", "https://api.appstoreconnect.apple.com/v1/apps/APP_B/appStoreVersions?cursor=next"},
			wantErr: "versions list: --next cannot be combined with --app",
		},
		{
			name:    "web apps compatibility view without app",
			args:    []string{"web", "apps", "compatibility", "view"},
			wantErr: "--app is required (or set ASC_APP_ID)",
		},
		{
			name:    "web apps tax-category view without app",
			args:    []string{"web", "apps", "tax-category", "view"},
			wantErr: "--app is required (or set ASC_APP_ID)",
		},
		{
			name:    "web service-ids view without service-id",
			args:    []string{"web", "service-ids", "view"},
			wantErr: "--service-id is required",
		},
		{
			name:    "web sign-in-keys view without key-id",
			args:    []string{"web", "sign-in-keys", "view"},
			wantErr: "--key-id is required",
		},
		{
			name:    "web subscriptions pricing adjusted-equalizations view without price-point-id",
			args:    []string{"web", "subscriptions", "pricing", "adjusted-equalizations", "view"},
			wantErr: "--price-point-id is required",
		},
		{
			name:    "web xcode-cloud settings version-aliases view without product-id",
			args:    []string{"web", "xcode-cloud", "settings", "version-aliases", "view"},
			wantErr: "--product-id is required",
		},
		{
			name:    "reviews view without id",
			args:    []string{"reviews", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "apps ci-product view without id",
			args:    []string{"apps", "ci-product", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "apps content-rights view without app",
			args:    []string{"apps", "content-rights", "view"},
			wantErr: "--app is required (or set ASC_APP_ID)",
		},
		{
			name:    "app-clips default-experiences view without experience-id",
			args:    []string{"app-clips", "default-experiences", "view"},
			wantErr: "--experience-id is required",
		},
		{
			name:    "app-clips default-experiences localizations view without localization-id",
			args:    []string{"app-clips", "default-experiences", "localizations", "view"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "app-clips advanced-experiences view without experience-id",
			args:    []string{"app-clips", "advanced-experiences", "view"},
			wantErr: "--experience-id is required",
		},
		{
			name:    "app-clips advanced-experiences images view without id",
			args:    []string{"app-clips", "advanced-experiences", "images", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "app-clips header-images view without id",
			args:    []string{"app-clips", "header-images", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "app-clips invocations view without invocation-id",
			args:    []string{"app-clips", "invocations", "view"},
			wantErr: "--invocation-id is required",
		},
		{
			name:    "app-clips review-details view without id",
			args:    []string{"app-clips", "review-details", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "product-pages custom-pages view without custom-page-id",
			args:    []string{"product-pages", "custom-pages", "view"},
			wantErr: "--custom-page-id is required",
		},
		{
			name:    "product-pages custom-pages versions view without custom-page-version-id",
			args:    []string{"product-pages", "custom-pages", "versions", "view"},
			wantErr: "--custom-page-version-id is required",
		},
		{
			name:    "product-pages custom-pages localizations view without localization-id",
			args:    []string{"product-pages", "custom-pages", "localizations", "view"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "product-pages experiments view without experiment-id",
			args:    []string{"product-pages", "experiments", "view"},
			wantErr: "--experiment-id is required",
		},
		{
			name:    "product-pages experiments treatments view without treatment-id",
			args:    []string{"product-pages", "experiments", "treatments", "view"},
			wantErr: "--treatment-id is required",
		},
		{
			name:    "product-pages experiments treatments localizations view without localization-id",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "view"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "build-localizations view without id",
			args:    []string{"build-localizations", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "app-events view without event-id",
			args:    []string{"app-events", "view"},
			wantErr: "--event-id is required",
		},
		{
			name:    "app-events localizations view without localization-id",
			args:    []string{"app-events", "localizations", "view"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "app-events screenshots view without screenshot-id",
			args:    []string{"app-events", "screenshots", "view"},
			wantErr: "--screenshot-id is required",
		},
		{
			name:    "app-events video-clips view without clip-id",
			args:    []string{"app-events", "video-clips", "view"},
			wantErr: "--clip-id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runCommand(t, test.args)

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			assertUsageErrorStderr(t, stderr, test.wantErr)
		})
	}
}
