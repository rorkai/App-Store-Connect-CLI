package cmdtest

import (
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// TestAccountAndDistributionInputValidationReturnsUsageExitCode locks the
// usage-error contract for pre-request flag validation across the account,
// provisioning, distribution and Xcode Cloud command groups. Every case below
// rejects a flag value before any App Store Connect client is constructed, so
// each must print "Error: <message>" to stderr and exit with the documented
// usage code 2 rather than the generic runtime failure code 1.
//
// This extends the analytics coverage in analytics_usage_exit_codes_test.go and
// the app/build coverage in usage_exit_codes_command_groups_test.go to the
// remaining groups tracked by issue #518.
func TestAccountAndDistributionInputValidationReturnsUsageExitCode(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
		deps    bool // the command is built on shared.Deps: run it on test deps
	}{
		{
			name:    "actors list limit",
			args:    []string{"actors", "list", "--limit", "201"},
			wantErr: "actors list: --limit must be between 1 and 200",
			deps:    true,
		},
		{
			name:    "actors list next",
			args:    []string{"actors", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "actors list: --next must be an App Store Connect URL",
			deps:    true,
		},
		{
			name:    "actors list invalid fields",
			args:    []string{"actors", "list", "--id", "ACTOR_ID", "--fields", "bad"},
			wantErr: "actors list: --fields must be one of: actorType, userFirstName, userLastName, userEmail, apiKeyId",
			deps:    true,
		},
		{
			name:    "actors view invalid fields",
			args:    []string{"actors", "view", "--id", "ACTOR_ID", "--fields", "bad"},
			wantErr: "actors view: --fields must be one of: actorType, userFirstName, userLastName, userEmail, apiKeyId",
			deps:    true,
		},
		{
			name:    "agreements territories list limit",
			args:    []string{"agreements", "territories", "list", "--id", "X", "--limit", "201"},
			wantErr: "agreements territories list: --limit must be between 1 and 200",
			deps:    true,
		},
		{
			name:    "agreements territories list next",
			args:    []string{"agreements", "territories", "list", "--id", "X", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "agreements territories list: --next must be an App Store Connect URL",
			deps:    true,
		},
		{
			name:    "agreements territories list next id extraction",
			args:    []string{"agreements", "territories", "list", "--next", "https://api.appstoreconnect.apple.com/v1/endUserLicenseAgreements//territories?cursor=AQ"},
			wantErr: "agreements territories list: invalid --next URL",
			deps:    true,
		},
		{
			name:    "alternative-distribution domains list limit",
			args:    []string{"alternative-distribution", "domains", "list", "--limit", "201"},
			wantErr: "alternative-distribution domains list: --limit must be between 1 and 200",
		},
		{
			name:    "alternative-distribution domains list next",
			args:    []string{"alternative-distribution", "domains", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "alternative-distribution domains list: --next must be an App Store Connect URL",
		},
		{
			name:    "alternative-distribution keys list limit",
			args:    []string{"alternative-distribution", "keys", "list", "--limit", "201"},
			wantErr: "alternative-distribution keys list: --limit must be between 1 and 200",
		},
		{
			name:    "alternative-distribution keys list next",
			args:    []string{"alternative-distribution", "keys", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "alternative-distribution keys list: --next must be an App Store Connect URL",
		},
		{
			name:    "alternative-distribution keys create both key sources",
			args:    []string{"alternative-distribution", "keys", "create", "--app", "APP_ID", "--public-key", "KEY", "--public-key-path", "key.pem"},
			wantErr: "alternative-distribution keys create: only one of --public-key or --public-key-path is allowed",
		},
		{
			name:    "alternative-distribution packages versions deltas limit",
			args:    []string{"alternative-distribution", "packages", "versions", "deltas", "--version-id", "V", "--limit", "201"},
			wantErr: "alternative-distribution packages versions deltas: --limit must be between 1 and 200",
		},
		{
			name:    "alternative-distribution packages versions deltas next",
			args:    []string{"alternative-distribution", "packages", "versions", "deltas", "--version-id", "V", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "alternative-distribution packages versions deltas: --next must be an App Store Connect URL",
		},
		{
			name:    "alternative-distribution packages versions list limit",
			args:    []string{"alternative-distribution", "packages", "versions", "list", "--package-id", "P", "--limit", "201"},
			wantErr: "alternative-distribution packages versions list: --limit must be between 1 and 200",
		},
		{
			name:    "alternative-distribution packages versions list next",
			args:    []string{"alternative-distribution", "packages", "versions", "list", "--package-id", "P", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "alternative-distribution packages versions list: --next must be an App Store Connect URL",
		},
		{
			name:    "alternative-distribution packages versions variants limit",
			args:    []string{"alternative-distribution", "packages", "versions", "variants", "--version-id", "V", "--limit", "201"},
			wantErr: "alternative-distribution packages versions variants: --limit must be between 1 and 200",
		},
		{
			name:    "alternative-distribution packages versions variants next",
			args:    []string{"alternative-distribution", "packages", "versions", "variants", "--version-id", "V", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "alternative-distribution packages versions variants: --next must be an App Store Connect URL",
		},
		{
			name:    "android-ios-mapping list limit",
			args:    []string{"android-ios-mapping", "list", "--app", "123456789", "--limit", "201"},
			wantErr: "android-ios-mapping list: --limit must be between 1 and 200",
		},
		{
			name:    "android-ios-mapping list next",
			args:    []string{"android-ios-mapping", "list", "--app", "123456789", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "android-ios-mapping list: --next must be an App Store Connect URL",
		},
		{
			name:    "android-ios-mapping list invalid fields",
			args:    []string{"android-ios-mapping", "list", "--app", "APP_ID", "--fields", "bad"},
			wantErr: "android-ios-mapping list: --fields must be one of: packageName, appSigningKeyPublicCertificateSha256Fingerprints",
		},
		{
			name:    "android-ios-mapping view invalid fields",
			args:    []string{"android-ios-mapping", "view", "--mapping-id", "MAP_ID", "--fields", "bad"},
			wantErr: "android-ios-mapping view: --fields must be one of: packageName, appSigningKeyPublicCertificateSha256Fingerprints",
		},
		{
			name:    "android-ios-mapping update conflicting package name flags",
			args:    []string{"android-ios-mapping", "update", "--mapping-id", "MAP_ID", "--android-package-name", "com.example", "--clear-android-package-name"},
			wantErr: "android-ios-mapping update: --android-package-name cannot be used with --clear-android-package-name",
		},
		{
			name:    "background-assets list limit",
			args:    []string{"background-assets", "list", "--app", "123456789", "--limit", "201"},
			wantErr: "background-assets list: --limit must be between 1 and 200",
		},
		{
			name:    "background-assets list next",
			args:    []string{"background-assets", "list", "--app", "123456789", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "background-assets list: --next must be an App Store Connect URL",
		},
		{
			name:    "background-assets upload-files list limit",
			args:    []string{"background-assets", "upload-files", "list", "--version-id", "V", "--limit", "201"},
			wantErr: "background-assets upload-files list: --limit must be between 1 and 200",
		},
		{
			name:    "background-assets upload-files list next",
			args:    []string{"background-assets", "upload-files", "list", "--version-id", "V", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "background-assets upload-files list: --next must be an App Store Connect URL",
		},
		{
			name:    "background-assets upload-files create invalid asset type",
			args:    []string{"background-assets", "upload-files", "create", "--version-id", "VERSION_ID", "--file", "asset.bin", "--asset-type", "bad"},
			wantErr: "background-assets upload-files create: --asset-type must be one of: ASSET, MANIFEST",
		},
		{
			name:    "background-assets upload-files update invalid uploaded",
			args:    []string{"background-assets", "upload-files", "update", "--upload-file-id", "FILE_ID", "--uploaded", "bad"},
			wantErr: "background-assets upload-files update: --uploaded must be true or false",
		},
		{
			name:    "background-assets versions list limit",
			args:    []string{"background-assets", "versions", "list", "--background-asset-id", "B", "--limit", "201"},
			wantErr: "background-assets versions list: --limit must be between 1 and 200",
		},
		{
			name:    "background-assets versions list next",
			args:    []string{"background-assets", "versions", "list", "--background-asset-id", "B", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "background-assets versions list: --next must be an App Store Connect URL",
		},
		{
			name:    "bundle-ids capabilities list next",
			args:    []string{"bundle-ids", "capabilities", "list", "--bundle", "B", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "bundle-ids capabilities list: --next must be an App Store Connect URL",
		},
		{
			name:    "bundle-ids list next",
			args:    []string{"bundle-ids", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "bundle-ids list: --next must be an App Store Connect URL",
		},
		{
			name:    "bundle-ids profiles list limit",
			args:    []string{"bundle-ids", "profiles", "list", "--id", "X", "--limit", "201"},
			wantErr: "bundle-ids profiles list: --limit must be between 1 and 200",
		},
		{
			name:    "bundle-ids profiles list next",
			args:    []string{"bundle-ids", "profiles", "list", "--id", "X", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "bundle-ids profiles list: --next must be an App Store Connect URL",
		},
		{
			name:    "categories subcategories next",
			args:    []string{"categories", "subcategories", "--category-id", "C", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "categories subcategories: --next must be an App Store Connect URL",
		},
		{
			name:    "certificates list limit",
			args:    []string{"certificates", "list", "--limit", "201"},
			wantErr: "certificates list: --limit must be between 1 and 200",
		},
		{
			name:    "certificates view invalid include",
			args:    []string{"certificates", "view", "--id", "CERT_ID", "--include", "bad"},
			wantErr: "certificates view: --include must be one of: passTypeId",
		},
		{
			name:    "certificates update invalid activated",
			args:    []string{"certificates", "update", "--id", "CERT_ID", "--activated", "bad"},
			wantErr: "certificates update: --activated must be true or false",
		},
		{
			name:    "devices view invalid fields",
			args:    []string{"devices", "view", "--id", "DEVICE_ID", "--fields", "bad"},
			wantErr: "devices view: --fields must be one of: addedDate, deviceClass, model, name, platform, status, udid",
		},
		{
			name:    "devices update invalid status",
			args:    []string{"devices", "update", "--id", "DEVICE_ID", "--status", "bad"},
			wantErr: "devices update: --status must be one of: ENABLED, DISABLED",
		},
		{
			name:    "iap promoted-purchases list next",
			args:    []string{"iap", "promoted-purchases", "list", "--app", "123456789", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "iap promoted-purchases list: --next must be an App Store Connect URL",
		},
		{
			name:    "merchant-ids certificates list limit",
			args:    []string{"merchant-ids", "certificates", "list", "--merchant-id", "M", "--limit", "201"},
			wantErr: "merchant-ids certificates list: --limit must be between 1 and 200",
		},
		{
			name:    "merchant-ids certificates list next",
			args:    []string{"merchant-ids", "certificates", "list", "--merchant-id", "M", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "merchant-ids certificates list: --next must be an App Store Connect URL",
		},
		{
			name:    "merchant-ids certificates list next without merchant id",
			args:    []string{"merchant-ids", "certificates", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "merchant-ids certificates list: --next must be an App Store Connect URL",
		},
		{
			name:    "merchant-ids certificates view limit",
			args:    []string{"merchant-ids", "certificates", "view", "--merchant-id", "M", "--limit", "201"},
			wantErr: "merchant-ids certificates view: --limit must be between 1 and 200",
		},
		{
			name:    "merchant-ids certificates view next",
			args:    []string{"merchant-ids", "certificates", "view", "--merchant-id", "M", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "merchant-ids certificates view: --next must be an App Store Connect URL",
		},
		{
			name:    "merchant-ids certificates view next without merchant id",
			args:    []string{"merchant-ids", "certificates", "view", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "merchant-ids certificates view: --next must be an App Store Connect URL",
		},
		{
			name:    "merchant-ids list certificates-limit",
			args:    []string{"merchant-ids", "list", "--certificates-limit", "51"},
			wantErr: "merchant-ids list: --certificates-limit must be between 1 and 50",
		},
		{
			name:    "merchant-ids list limit",
			args:    []string{"merchant-ids", "list", "--limit", "201"},
			wantErr: "merchant-ids list: --limit must be between 1 and 200",
		},
		{
			name:    "merchant-ids list next",
			args:    []string{"merchant-ids", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "merchant-ids list: --next must be an App Store Connect URL",
		},
		{
			name:    "nominations list in-app-events-limit",
			args:    []string{"nominations", "list", "--in-app-events-limit", "51"},
			wantErr: "nominations list: --in-app-events-limit must be between 1 and 50",
			deps:    true,
		},
		{
			name:    "nominations list limit",
			args:    []string{"nominations", "list", "--limit", "201"},
			wantErr: "nominations list: --limit must be between 1 and 200",
			deps:    true,
		},
		{
			name:    "nominations list next",
			args:    []string{"nominations", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "nominations list: --next must be an App Store Connect URL",
			deps:    true,
		},
		{
			name:    "nominations list related-apps-limit",
			args:    []string{"nominations", "list", "--related-apps-limit", "51"},
			wantErr: "nominations list: --related-apps-limit must be between 1 and 50",
			deps:    true,
		},
		{
			name:    "nominations list supported-territories-limit",
			args:    []string{"nominations", "list", "--supported-territories-limit", "201"},
			wantErr: "nominations list: --supported-territories-limit must be between 1 and 200",
			deps:    true,
		},
		{
			name:    "nominations view in-app-events-limit",
			args:    []string{"nominations", "view", "--id", "N", "--in-app-events-limit", "51"},
			wantErr: "nominations view: --in-app-events-limit must be between 1 and 50",
			deps:    true,
		},
		{
			name:    "nominations view related-apps-limit",
			args:    []string{"nominations", "view", "--id", "N", "--related-apps-limit", "51"},
			wantErr: "nominations view: --related-apps-limit must be between 1 and 50",
			deps:    true,
		},
		{
			name:    "nominations view supported-territories-limit",
			args:    []string{"nominations", "view", "--id", "N", "--supported-territories-limit", "201"},
			wantErr: "nominations view: --supported-territories-limit must be between 1 and 200",
			deps:    true,
		},
		{
			name:    "marketplace search-details view invalid fields",
			args:    []string{"marketplace", "search-details", "view", "--app", "APP_ID", "--fields", "bad"},
			wantErr: "marketplace search-details view: --fields must be one of: catalogUrl",
			deps:    true,
		},
		{
			name:    "nominations list invalid status",
			args:    []string{"nominations", "list", "--status", "bad"},
			wantErr: "nominations list: --status must be one of: DRAFT, SUBMITTED, ARCHIVED",
			deps:    true,
		},
		{
			name:    "nominations view invalid fields",
			args:    []string{"nominations", "view", "--id", "NOMINATION_ID", "--fields", "bad"},
			wantErr: "nominations view: --fields must be one of: name, type, description, createdDate, lastModifiedDate, submittedDate, state, publishStartDate, publishEndDate, deviceFamilies, locales, supplementalMaterialsUris, hasInAppEvents, launchInSelectMarketsFirst, notes, preOrderEnabled, relatedApps, createdByActor, lastModifiedByActor, submittedByActor, inAppEvents, supportedTerritories",
			deps:    true,
		},
		{
			name:    "pass-type-ids certificates list limit",
			args:    []string{"pass-type-ids", "certificates", "list", "--pass-type-id", "P", "--limit", "201"},
			wantErr: "pass-type-ids certificates list: --limit must be between 1 and 200",
		},
		{
			name:    "pass-type-ids certificates view limit",
			args:    []string{"pass-type-ids", "certificates", "view", "--pass-type-id", "P", "--limit", "201"},
			wantErr: "pass-type-ids certificates view: --limit must be between 1 and 200",
		},
		{
			name:    "pass-type-ids list limit",
			args:    []string{"pass-type-ids", "list", "--limit", "201"},
			wantErr: "pass-type-ids list: --limit must be between 1 and 200",
		},
		{
			name:    "pass-type-ids list limit-certificates",
			args:    []string{"pass-type-ids", "list", "--limit-certificates", "51"},
			wantErr: "pass-type-ids list: --limit-certificates must be between 1 and 50",
		},
		{
			name:    "pass-type-ids list next",
			args:    []string{"pass-type-ids", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "pass-type-ids list: --next must be an App Store Connect URL",
		},
		{
			name:    "pass-type-ids view limit-certificates",
			args:    []string{"pass-type-ids", "view", "--pass-type-id", "P", "--limit-certificates", "51"},
			wantErr: "pass-type-ids view: --limit-certificates must be between 1 and 50",
		},
		{
			name:    "profiles links certificates limit",
			args:    []string{"profiles", "links", "certificates", "--id", "X", "--limit", "201"},
			wantErr: "profiles links certificates: --limit must be between 1 and 200",
		},
		{
			name:    "profiles links certificates next",
			args:    []string{"profiles", "links", "certificates", "--id", "X", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "profiles links certificates: --next must be an App Store Connect URL",
		},
		{
			name:    "profiles links certificates next without id",
			args:    []string{"profiles", "links", "certificates", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "profiles links certificates: --next must be an App Store Connect URL",
		},
		{
			name:    "profiles links devices limit",
			args:    []string{"profiles", "links", "devices", "--id", "X", "--limit", "201"},
			wantErr: "profiles links devices: --limit must be between 1 and 200",
		},
		{
			name:    "profiles links devices next",
			args:    []string{"profiles", "links", "devices", "--id", "X", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "profiles links devices: --next must be an App Store Connect URL",
		},
		{
			name:    "profiles links devices next without id",
			args:    []string{"profiles", "links", "devices", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "profiles links devices: --next must be an App Store Connect URL",
		},
		{
			name:    "profiles links certificates next id extraction",
			args:    []string{"profiles", "links", "certificates", "--next", "https://api.appstoreconnect.apple.com/v1/profiles//relationships/certificates?cursor=AQ"},
			wantErr: "profiles links certificates: invalid --next URL",
		},
		{
			name:    "profiles links devices next id extraction",
			args:    []string{"profiles", "links", "devices", "--next", "https://api.appstoreconnect.apple.com/v1/profiles//relationships/devices?cursor=AQ"},
			wantErr: "profiles links devices: invalid --next URL",
		},
		{
			name:    "profiles list next",
			args:    []string{"profiles", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "profiles list: --next must be an App Store Connect URL",
		},
		{
			name:    "profiles view invalid include",
			args:    []string{"profiles", "view", "--id", "PROFILE_ID", "--include", "bad"},
			wantErr: "profiles view: --include must be one of: bundleId, certificates, devices",
		},
		{
			name:    "sandbox list limit",
			args:    []string{"sandbox", "list", "--limit", "201"},
			wantErr: "sandbox list: --limit must be between 1 and 200",
		},
		{
			name:    "sandbox list next",
			args:    []string{"sandbox", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "sandbox list: --next must be an App Store Connect URL",
		},
		{
			name:    "sandbox list invalid email",
			args:    []string{"sandbox", "list", "--email", "bad"},
			wantErr: "sandbox list: --email must be a valid email address",
		},
		{
			name:    "sandbox view invalid email",
			args:    []string{"sandbox", "view", "--email", "bad"},
			wantErr: "sandbox view: --email must be a valid email address",
		},
		{
			name:    "sandbox update invalid renewal rate",
			args:    []string{"sandbox", "update", "--id", "TESTER_ID", "--subscription-renewal-rate", "bad"},
			wantErr: "sandbox update: --subscription-renewal-rate must be one of: MONTHLY_RENEWAL_EVERY_FIFTEEN_MINUTES, MONTHLY_RENEWAL_EVERY_FIVE_MINUTES, MONTHLY_RENEWAL_EVERY_ONE_HOUR, MONTHLY_RENEWAL_EVERY_THIRTY_MINUTES, MONTHLY_RENEWAL_EVERY_THREE_MINUTES",
		},
		{
			name:    "sandbox clear-history invalid email",
			args:    []string{"sandbox", "clear-history", "--email", "bad", "--confirm"},
			wantErr: "sandbox clear-history: --email must be a valid email address",
		},
		{
			name:    "testflight crashes list limit",
			args:    []string{"testflight", "crashes", "list", "--limit", "201"},
			wantErr: "testflight crashes list: --limit must be between 1 and 200",
		},
		{
			name:    "testflight crashes list next",
			args:    []string{"testflight", "crashes", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "testflight crashes list: --next must be an App Store Connect URL",
		},
		{
			name:    "testflight crashes list sort",
			args:    []string{"testflight", "crashes", "list", "--sort", "bogus"},
			wantErr: "testflight crashes list: --sort must be one of: createdDate, -createdDate",
		},
		{
			name:    "testflight feedback list limit",
			args:    []string{"testflight", "feedback", "list", "--limit", "201"},
			wantErr: "testflight feedback list: --limit must be between 1 and 200",
		},
		{
			name:    "testflight feedback list next",
			args:    []string{"testflight", "feedback", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "testflight feedback list: --next must be an App Store Connect URL",
		},
		{
			name:    "testflight feedback list sort",
			args:    []string{"testflight", "feedback", "list", "--sort", "bogus"},
			wantErr: "testflight feedback list: --sort must be one of: createdDate, -createdDate",
		},
		{
			name:    "users invites list limit",
			args:    []string{"users", "invites", "list", "--limit", "201"},
			wantErr: "users invites list: --limit must be between 1 and 200",
		},
		{
			name:    "users invites list next",
			args:    []string{"users", "invites", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "users invites list: --next must be an App Store Connect URL",
		},
		{
			name:    "users invites visible-apps list limit",
			args:    []string{"users", "invites", "visible-apps", "list", "--id", "X", "--limit", "201"},
			wantErr: "users invites visible-apps list: --limit must be between 1 and 200",
		},
		{
			name:    "users invites visible-apps list next",
			args:    []string{"users", "invites", "visible-apps", "list", "--id", "X", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "users invites visible-apps list: --next must be an App Store Connect URL",
		},
		{
			name:    "users invites visible-apps list next id extraction",
			args:    []string{"users", "invites", "visible-apps", "list", "--next", "https://api.appstoreconnect.apple.com/v1/userInvitations//visibleApps?cursor=AQ"},
			wantErr: "users invites visible-apps list: invalid --next URL",
		},
		{
			name:    "users visible-apps list next id extraction",
			args:    []string{"users", "visible-apps", "list", "--next", "https://api.appstoreconnect.apple.com/v1/users//visibleApps?cursor=AQ"},
			wantErr: "users visible-apps list: invalid --next URL",
		},
		{
			name:    "users visible-apps view next id extraction",
			args:    []string{"users", "visible-apps", "view", "--next", "https://api.appstoreconnect.apple.com/v1/users//visibleApps?cursor=AQ"},
			wantErr: "users visible-apps view: invalid --next URL",
		},
		{
			name:    "users list limit",
			args:    []string{"users", "list", "--limit", "201"},
			wantErr: "users list: --limit must be between 1 and 200",
		},
		{
			name:    "users list next",
			args:    []string{"users", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "users list: --next must be an App Store Connect URL",
		},
		{
			name:    "users visible-apps list limit",
			args:    []string{"users", "visible-apps", "list", "--id", "X", "--limit", "201"},
			wantErr: "users visible-apps list: --limit must be between 1 and 200",
		},
		{
			name:    "users visible-apps list next",
			args:    []string{"users", "visible-apps", "list", "--id", "X", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "users visible-apps list: --next must be an App Store Connect URL",
		},
		{
			name:    "users visible-apps view limit",
			args:    []string{"users", "visible-apps", "view", "--id", "X", "--limit", "201"},
			wantErr: "users visible-apps view: --limit must be between 1 and 200",
		},
		{
			name:    "users visible-apps view next",
			args:    []string{"users", "visible-apps", "view", "--id", "X", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "users visible-apps view: --next must be an App Store Connect URL",
		},
		{
			name:    "users view invalid include",
			args:    []string{"users", "view", "--id", "USER_ID", "--include", "bad"},
			wantErr: "users view: --include must be one of: visibleApps",
		},
		{
			name:    "xcode-cloud artifacts list limit",
			args:    []string{"xcode-cloud", "artifacts", "list", "--action-id", "A", "--limit", "201"},
			wantErr: "xcode-cloud artifacts list: --limit must be between 1 and 200",
		},
		{
			name:    "xcode-cloud artifacts list next",
			args:    []string{"xcode-cloud", "artifacts", "list", "--action-id", "A", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "xcode-cloud artifacts list: --next must be an App Store Connect URL",
		},
		{
			name:    "xcode-cloud products limit",
			args:    []string{"xcode-cloud", "products", "--limit", "201"},
			wantErr: "xcode-cloud products: --limit must be between 1 and 200",
		},
		{
			name:    "xcode-cloud products next",
			args:    []string{"xcode-cloud", "products", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "xcode-cloud products: --next must be an App Store Connect URL",
		},
		{
			name:    "xcode-cloud workflows limit",
			args:    []string{"xcode-cloud", "workflows", "--limit", "201"},
			wantErr: "xcode-cloud workflows: --limit must be between 1 and 200",
		},
		{
			name:    "xcode-cloud workflows next",
			args:    []string{"xcode-cloud", "workflows", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "xcode-cloud workflows: --next must be an App Store Connect URL",
		},
		{
			name:    "bundle-ids app view without id",
			args:    []string{"bundle-ids", "app", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "background-assets app-store-releases view without id",
			args:    []string{"background-assets", "app-store-releases", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "background-assets external-beta-releases view without id",
			args:    []string{"background-assets", "external-beta-releases", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "background-assets internal-beta-releases view without id",
			args:    []string{"background-assets", "internal-beta-releases", "view"},
			wantErr: "--id is required",
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

// TestMarketplaceWebhooksInputValidationReturnsUsageExitCode covers the same
// contract for `asc marketplace webhooks list`, which prints a deprecation
// warning ahead of the diagnostic and therefore cannot use
// assertUsageErrorStderr's "first line" assertion.
func TestMarketplaceWebhooksInputValidationReturnsUsageExitCode(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "limit above maximum",
			args:    []string{"marketplace", "webhooks", "list", "--limit", "201"},
			wantErr: "marketplace webhooks list: --limit must be between 1 and 200",
		},
		{
			name:    "invalid next",
			args:    []string{"marketplace", "webhooks", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "marketplace webhooks list: --next must be an App Store Connect URL",
		},
		{
			name:    "invalid fields",
			args:    []string{"marketplace", "webhooks", "list", "--fields", "bad"},
			wantErr: "marketplace webhooks list: --fields must be one of: endpointUrl",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runRootWithDeps(t, nil, test.args...)

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			wantLine := "Error: " + test.wantErr + "\n"
			if !strings.Contains(stderr, wantLine) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, wantLine)
			}
			if got := strings.Count(stderr, "Error: "); got != 1 {
				t.Fatalf("stderr = %q, want exactly one %q diagnostic, got %d", stderr, "Error: ", got)
			}
		})
	}
}
