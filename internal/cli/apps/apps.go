package apps

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func appsListFlags(d *shared.Deps, fs *flag.FlagSet) (output shared.OutputFlags, bundleID *string, name *string, sku *string, versionState *string, reviewSubmissionState *string, sort *string, limit *int, next *string, paginate *bool, appInfoFields *string, iapFields *string, subscriptionGroupFields *string, retentionMessageFields *string) {
	output = d.BindOutputFlags(fs)
	bundleID = fs.String("bundle-id", "", "Filter by bundle ID(s), comma-separated")
	name = fs.String("name", "", "Filter by app name(s), comma-separated")
	sku = fs.String("sku", "", "Filter by SKU(s), comma-separated")
	versionState = fs.String("version-state", "", "Filter by App Store version state(s), comma-separated: "+strings.Join(appVersionStateFilterList(), ", "))
	reviewSubmissionState = fs.String("review-submission-state", "", "Filter by review submission state(s), comma-separated: "+strings.Join(reviewSubmissionStateFilterList(), ", "))
	sort = fs.String("sort", "", "Sort by name, -name, bundleId, -bundleId, sku, or -sku")
	limit = fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next = fs.String("next", "", shared.NextFlagUsage)
	paginate = fs.Bool("paginate", false, shared.PaginateFlagUsage)
	appInfoFields = fs.String("app-info-fields", "", "Sparse fields for included app info records: "+strings.Join(shared.AppInfoSparseFields, ", ")+" (deprecated; removed from API 4.5; prefer asc age-rating view)")
	iapFields = fs.String("iap-fields", "", "Sparse fields for included in-app purchases: "+strings.Join(shared.AppInAppPurchaseSparseFields, ", "))
	subscriptionGroupFields = fs.String("subscription-group-fields", "", "Sparse fields for included subscription groups: "+strings.Join(shared.SubscriptionGroupSparseFields, ", "))
	retentionMessageFields = fs.String("retention-message-fields", "", "Sparse fields for included retention messages: "+strings.Join(shared.RetentionMessageSparseFields, ", "))
	return
}

// NewAppsCommand returns the apps command factory.
func NewAppsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("apps", flag.ExitOnError)

	output, bundleID, name, sku, versionState, reviewSubmissionState, sort, limit, next, paginate, appInfoFields, iapFields, subscriptionGroupFields, retentionMessageFields := appsListFlags(d, fs)
	subcommands := []*ffcli.Command{
		newAppsListCommand(d),
		newAppsPublishedCommand(d),
		newAppsWallCommand(d),
		newAppsPublicCommand(d),
		newAppsRegistryCommand(d),
		newAppsGetCommand(d),
		newAppsRenameCommand(d),
		newAppsInfoCommand(d),
		newAppsCIProductCommand(d),
		newAppsUpdateCommand(d),
		newAppsRemoveBetaTestersCommand(d),
		newAppsSubscriptionGracePeriodCommand(d),
		newAppsSearchKeywordsCommand(d),
		newAppEncryptionDeclarationsCommand(d),
		newAppsContentRightsCommand(d),
	}

	return &ffcli.Command{
		Name:       "apps",
		ShortUsage: "asc apps <subcommand> [flags]",
		ShortHelp:  "List and manage apps in App Store Connect.",
		LongHelp: `List and manage apps in App Store Connect.

Creating an app record runs through a web session, not this command group:
  asc web apps create --name "My App" --bundle-id "com.example.app" --sku "MYAPP123"

Examples:
  asc apps
  asc apps list --bundle-id "com.example.app"
  asc apps published
  asc apps wall
  asc apps wall submit --app "1234567890" --confirm
  asc apps public view --app "1234567890"
  asc apps public search --term "focus" --country us
  asc apps public rank --app "1234567890" --term "focus timer" --country us --platform TV_OS
  asc apps public storefronts list
  asc apps registry pull --path ".asc/app-registry.json"
  asc apps view --id "APP_ID"
  asc apps rename --app "APP_ID" --locale "en-US" --name "New Name"
  asc apps info view --app "APP_ID"
  asc apps info edit --app "APP_ID" --locale "en-US" --whats-new "Bug fixes"
  asc apps ci-product view --id "APP_ID"
  asc apps update --id "APP_ID" --bundle-id "com.example.app"
  asc apps update --id "APP_ID" --primary-locale "en-US"
  asc apps subscription-grace-period view --app "APP_ID"
  asc apps content-rights edit --app "APP_ID" --uses-third-party-content=false
  asc apps --limit 10
  asc apps --sort name
  asc apps --sort sku
  asc apps --version-state IN_REVIEW,WAITING_FOR_REVIEW
  asc apps --review-submission-state IN_REVIEW
  asc apps --app-info-fields state,primaryCategory --iap-fields versions --subscription-group-fields versions
  asc apps --output table
  asc apps --next "<links.next>"
  asc apps --paginate`,
		FlagSet:     fs,
		UsageFunc:   d.VisibleUsageFunc,
		Subcommands: subcommands,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				subcommand := strings.TrimSpace(args[0])
				if subcommand == "create" {
					fmt.Fprintln(d.IO.ErrOut, "Error: `asc apps create` was removed. Use `asc web apps create` instead.")
					return flag.ErrHelp
				}
				fmt.Fprintf(d.IO.ErrOut, "Error: unknown subcommand %q\n", subcommand)
				return flag.ErrHelp
			}
			return appsList(d, ctx, fs, "apps", *output.Output, *output.Pretty, *bundleID, *name, *sku, *versionState, *reviewSubmissionState, *sort, *limit, *next, *paginate, *appInfoFields, *iapFields, *subscriptionGroupFields, *retentionMessageFields)
		},
	}
}

// newAppsListCommand returns the apps list subcommand.
func newAppsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("apps list", flag.ExitOnError)

	output, bundleID, name, sku, versionState, reviewSubmissionState, sort, limit, next, paginate, appInfoFields, iapFields, subscriptionGroupFields, retentionMessageFields := appsListFlags(d, fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc apps list [flags]",
		ShortHelp:  "List apps from App Store Connect.",
		LongHelp: `List apps from App Store Connect.

Examples:
  asc apps list
  asc apps list --bundle-id "com.example.app"
  asc apps list --name "My App"
  asc apps list --limit 10
  asc apps list --sort name
  asc apps list --sort sku
  asc apps list --version-state IN_REVIEW,WAITING_FOR_REVIEW
  asc apps list --review-submission-state IN_REVIEW
  asc apps list --app-info-fields state,primaryCategory --iap-fields versions --subscription-group-fields versions
  asc apps list --output table
  asc apps list --next "<links.next>"
  asc apps list --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return appsList(d, ctx, fs, "apps list", *output.Output, *output.Pretty, *bundleID, *name, *sku, *versionState, *reviewSubmissionState, *sort, *limit, *next, *paginate, *appInfoFields, *iapFields, *subscriptionGroupFields, *retentionMessageFields)
		},
	}
}

// newAppsGetCommand returns the apps view subcommand.
func newAppsGetCommand(d *shared.Deps) *ffcli.Command {
	var fieldValues, appInfoFieldValues, iapFieldValues, groupFieldValues, retentionMessageFieldValues, requestedIncludes []string

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "apps view",
		Name:        "view",
		ShortUsage:  "asc apps view --id APP_ID",
		ShortHelp:   "View app details by ID.",
		LongHelp: `View app details by ID.

--id (or --app) also takes the app's bundle ID or exact name.

Examples:
  asc apps view --id "APP_ID"
  asc apps view --app "com.example.app"
  asc apps view --id "APP_ID" --app-info-fields state,primaryCategory --iap-fields versions --subscription-group-fields versions
  asc apps view --id "APP_ID" --include appInfos,appStoreVersions
  asc apps view --id "APP_ID" --output table
  asc apps view --id "APP_ID" --fields "subscriptionStatusUrl,subscriptionStatusUrlVersion,subscriptionStatusUrlForSandbox,subscriptionStatusUrlVersionForSandbox" --output json`,
		IDUsage:            "App Store Connect app ID, bundle ID, or exact app name",
		IDType:             "apps",
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			shared.BindFlagAlias(fs, "app", "id")
			shared.BindFlagAlias(fs, "app-id", "id")
			fields := fs.String("fields", "", "App attribute fields to return, comma-separated: "+strings.Join(appAttributeFields, ", "))
			appInfoFields := fs.String("app-info-fields", "", "Sparse fields for included app info records: "+strings.Join(shared.AppInfoSparseFields, ", ")+" (deprecated; removed from API 4.5; prefer asc age-rating view)")
			iapFields := fs.String("iap-fields", "", "Sparse fields for included in-app purchases: "+strings.Join(shared.AppInAppPurchaseSparseFields, ", "))
			subscriptionGroupFields := fs.String("subscription-group-fields", "", "Sparse fields for included subscription groups: "+strings.Join(shared.SubscriptionGroupSparseFields, ", "))
			retentionMessageFields := fs.String("retention-message-fields", "", "Sparse fields for included retention messages: "+strings.Join(shared.RetentionMessageSparseFields, ", "))
			include := fs.String("include", "", "Include related resources: "+strings.Join(appViewIncludeValues, ", "))
			return func() (err error) {
				if requestedIncludes, err = shared.NormalizeSelection(*include, appViewIncludeValues, "--include"); err != nil {
					return d.UsageError(err.Error())
				}
				if fieldValues, err = normalizeSparseField(fs, *fields, appAttributeFields, "--fields"); err != nil {
					return d.UsageError(err.Error())
				}
				if appInfoFieldValues, err = normalizeSparseField(fs, *appInfoFields, shared.AppInfoSparseFields, "--app-info-fields"); err != nil {
					return d.UsageError(err.Error())
				}
				if iapFieldValues, err = normalizeSparseField(fs, *iapFields, shared.AppInAppPurchaseSparseFields, "--iap-fields"); err != nil {
					return d.UsageError(err.Error())
				}
				if groupFieldValues, err = normalizeSparseField(fs, *subscriptionGroupFields, shared.SubscriptionGroupSparseFields, "--subscription-group-fields"); err != nil {
					return d.UsageError(err.Error())
				}
				if retentionMessageFieldValues, err = normalizeSparseField(fs, *retentionMessageFields, shared.RetentionMessageSparseFields, "--retention-message-fields"); err != nil {
					return d.UsageError(err.Error())
				}

				d.WarnDeprecatedAppInfoFields(appInfoFieldValues, "")
				return nil
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			id, err := shared.ResolveAppIDWithLookup(ctx, client, id)
			if err != nil {
				return nil, err
			}
			includeValues := slices.Clone(requestedIncludes)
			if len(appInfoFieldValues) > 0 {
				includeValues = addInclude(includeValues, "appInfos")
			}
			if len(iapFieldValues) > 0 {
				includeValues = addInclude(includeValues, "inAppPurchases")
			}
			if len(groupFieldValues) > 0 {
				includeValues = addInclude(includeValues, "subscriptionGroups")
			}
			if len(retentionMessageFieldValues) > 0 {
				includeValues = addInclude(includeValues, "retentionMessages")
			}
			fields := fieldValues
			if len(fields) > 0 {
				fields = append(fields, includeValues...)
			}
			opts := []asc.AppOption{
				asc.WithAppFields(fields),
				asc.WithAppAppInfoFields(appInfoFieldValues),
				asc.WithAppInAppPurchaseFields(iapFieldValues),
				asc.WithAppSubscriptionGroupFields(groupFieldValues),
				asc.WithAppRetentionMessageFields(retentionMessageFieldValues),
				asc.WithAppInclude(includeValues),
			}
			resp, err := client.GetAppWithOptions(ctx, id, opts...)
			if err != nil {
				return nil, err
			}
			return asc.RedactAppStoreReviewDetailIncludesInSingleResponse(resp)
		},
	})
}

// newAppsUpdateCommand returns the apps update subcommand.
func newAppsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("apps update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "apps", "App Store Connect app ID")
	bundleID := fs.String("bundle-id", "", "Update bundle ID")
	primaryLocale := fs.String("primary-locale", "", "Update primary locale (e.g., en-US)")
	contentRights := fs.String("content-rights", "", "Content rights declaration: DOES_NOT_USE_THIRD_PARTY_CONTENT or USES_THIRD_PARTY_CONTENT")
	groupPurchaseMode := fs.String("group-purchase-invitation-mode", "", "Group purchase invitations: APPLE_HOSTED or DEVELOPER_HOSTED")
	accessibilityURL := fs.String("accessibility-url", "", "Accessibility URL for the app")
	subscriptionStatusURL := fs.String("subscription-status-url", "", "Production App Store Server Notifications HTTPS URL (sets version V2)")
	sandboxSubscriptionStatusURL := fs.String("sandbox-subscription-status-url", "", "Sandbox App Store Server Notifications HTTPS URL (sets version V2)")
	var streamlinedPurchasing shared.OptionalBool
	fs.Var(&streamlinedPurchasing, "streamlined-purchasing-enabled", "Set streamlined purchasing: true or false")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc apps update --id APP_ID [--bundle-id BUNDLE_ID] [--primary-locale LOCALE] [--content-rights DECLARATION] [--group-purchase-invitation-mode MODE] [--accessibility-url URL] [--subscription-status-url URL] [--sandbox-subscription-status-url URL] [--streamlined-purchasing-enabled true|false]",
		ShortHelp:  "Update app metadata and App Store Server Notifications URLs.",
		LongHelp: `Update app metadata and App Store Server Notifications URLs.

Notification URLs must use HTTPS without credentials or fragments. Each supplied
URL is configured for version V2. Omitted endpoints and other fields stay unchanged.

Examples:
  asc apps update --id "APP_ID" --bundle-id "com.example.app"
  asc apps update --id "APP_ID" --primary-locale "en-US"
  asc apps update --id "APP_ID" --content-rights "DOES_NOT_USE_THIRD_PARTY_CONTENT"
  asc apps update --id "APP_ID" --accessibility-url "https://example.com/accessibility"
  asc apps update --id "APP_ID" --subscription-status-url "https://example.com/notifications"
  asc apps update --id "APP_ID" --sandbox-subscription-status-url "https://example.com/sandbox-notifications"
  asc apps update --id "APP_ID" --streamlined-purchasing-enabled false
  asc apps update --id "APP_ID" --group-purchase-invitation-mode "APPLE_HOSTED"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--id")
			}

			attrs := asc.AppUpdateAttributes{}
			if bundleValue := strings.TrimSpace(*bundleID); bundleValue != "" {
				attrs.BundleID = &bundleValue
			}
			if localeValue := strings.TrimSpace(*primaryLocale); localeValue != "" {
				attrs.PrimaryLocale = &localeValue
			}
			if rightsValue := strings.TrimSpace(*contentRights); rightsValue != "" {
				normalizedRights := asc.ContentRightsDeclaration(strings.ToUpper(rightsValue))
				switch normalizedRights {
				case asc.ContentRightsDeclarationDoesNotUseThirdPartyContent,
					asc.ContentRightsDeclarationUsesThirdPartyContent:
					attrs.ContentRightsDeclaration = &normalizedRights
				default:
					fmt.Fprintf(d.IO.ErrOut, "Error: --content-rights must be %s or %s\n", asc.ContentRightsDeclarationDoesNotUseThirdPartyContent, asc.ContentRightsDeclarationUsesThirdPartyContent)
					return flag.ErrHelp
				}
			}
			if appFlagWasProvided(fs, "group-purchase-invitation-mode") {
				mode := asc.GroupPurchaseInvitationMode(strings.ToUpper(strings.TrimSpace(*groupPurchaseMode)))
				switch mode {
				case asc.GroupPurchaseInvitationModeAppleHosted, asc.GroupPurchaseInvitationModeDeveloperHosted:
					attrs.GroupPurchaseInvitationMode = &mode
				default:
					return d.UsageErrorf("--group-purchase-invitation-mode must be %s or %s", asc.GroupPurchaseInvitationModeAppleHosted, asc.GroupPurchaseInvitationModeDeveloperHosted)
				}
			}
			if appFlagWasProvided(fs, "accessibility-url") {
				value, err := validateAccessibilityURL(*accessibilityURL)
				if err != nil {
					return d.UsageErrorf("--accessibility-url: %v", err)
				}
				attrs.AccessibilityURL = &value
			}
			if appFlagWasProvided(fs, "subscription-status-url") {
				value, err := validateNotificationURL(*subscriptionStatusURL)
				if err != nil {
					return d.UsageErrorf("--subscription-status-url: %v", err)
				}
				version := asc.SubscriptionStatusURLVersionV2
				attrs.SubscriptionStatusURL = &value
				attrs.SubscriptionStatusURLVersion = &version
			}
			if appFlagWasProvided(fs, "sandbox-subscription-status-url") {
				value, err := validateNotificationURL(*sandboxSubscriptionStatusURL)
				if err != nil {
					return d.UsageErrorf("--sandbox-subscription-status-url: %v", err)
				}
				version := asc.SubscriptionStatusURLVersionV2
				attrs.SubscriptionStatusURLForSandbox = &value
				attrs.SubscriptionStatusURLVersionForSandbox = &version
			}
			if streamlinedPurchasing.IsSet() {
				value := streamlinedPurchasing.Value()
				attrs.StreamlinedPurchasingEnabled = &value
			}
			if attrs == (asc.AppUpdateAttributes{}) {
				return d.UpdateFlagRequired(fs, bundleID, primaryLocale, contentRights, groupPurchaseMode, accessibilityURL, subscriptionStatusURL, sandboxSubscriptionStatusURL, &streamlinedPurchasing)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("apps update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			app, err := client.UpdateApp(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("apps update: failed to update: %w", err)
			}

			return d.PrintOutput(app, *output.Output, *output.Pretty)
		},
	}
}

func appsList(d *shared.Deps, ctx context.Context, fs *flag.FlagSet, command string, output string, pretty bool, bundleID string, name string, sku string, versionState string, reviewSubmissionState string, sort string, limit int, next string, paginate bool, appInfoFields string, iapFields string, subscriptionGroupFields string, retentionMessageFields string) error {
	if err := d.ValidatePaginationFlags(
		ctx, fs, command, 200,
		"bundle-id", "name", "sku", "version-state", "review-submission-state", "sort", "limit",
		"app-info-fields", "iap-fields", "subscription-group-fields", "retention-message-fields",
	); err != nil {
		return err
	}
	next = strings.TrimSpace(next)
	if err := shared.ValidateSort(sort, "name", "-name", "bundleId", "-bundleId", "sku", "-sku"); err != nil {
		return d.UsageErrorf("apps: %v", err)
	}
	if next != "" {
		if parsed, err := url.Parse(next); err != nil || parsed.Path != "/v1/apps" {
			return d.UsageError("apps: --next must point to /v1/apps")
		}
	}
	versionStateValues := shared.SplitCSVUpper(versionState)
	if appFlagWasProvided(fs, "version-state") {
		if strings.TrimSpace(versionState) == "" {
			return d.UsageError("--version-state must not be empty")
		}
		if csvContainsEmptyValue(versionState) {
			return d.UsageError("--version-state must not contain empty values")
		}
	}
	versionStateValues, err := normalizeAppVersionStateFilters(versionStateValues)
	if err != nil {
		return d.UsageError(err.Error())
	}
	reviewSubmissionStateValues := shared.SplitCSVUpper(reviewSubmissionState)
	if appFlagWasProvided(fs, "review-submission-state") {
		if strings.TrimSpace(reviewSubmissionState) == "" {
			return d.UsageError("--review-submission-state must not be empty")
		}
		if csvContainsEmptyValue(reviewSubmissionState) {
			return d.UsageError("--review-submission-state must not contain empty values")
		}
	}
	reviewSubmissionStateValues, err = normalizeReviewSubmissionStateFilters(reviewSubmissionStateValues)
	if err != nil {
		return d.UsageError(err.Error())
	}
	appInfoFieldValues, err := normalizeSparseField(fs, appInfoFields, shared.AppInfoSparseFields, "--app-info-fields")
	if err != nil {
		return d.UsageError(err.Error())
	}
	iapFieldValues, err := normalizeSparseField(fs, iapFields, shared.AppInAppPurchaseSparseFields, "--iap-fields")
	if err != nil {
		return d.UsageError(err.Error())
	}
	groupFieldValues, err := normalizeSparseField(fs, subscriptionGroupFields, shared.SubscriptionGroupSparseFields, "--subscription-group-fields")
	if err != nil {
		return d.UsageError(err.Error())
	}
	retentionMessageFieldValues, err := normalizeSparseField(fs, retentionMessageFields, shared.RetentionMessageSparseFields, "--retention-message-fields")
	if err != nil {
		return d.UsageError(err.Error())
	}

	d.WarnDeprecatedAppInfoFields(appInfoFieldValues, next)

	client, err := d.Client()
	if err != nil {
		return fmt.Errorf("apps: %w", err)
	}

	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()

	opts := []asc.AppsOption{
		asc.WithAppsBundleIDs(shared.SplitCSV(bundleID)),
		asc.WithAppsNames(shared.SplitCSV(name)),
		asc.WithAppsSKUs(shared.SplitCSV(sku)),
		asc.WithAppsVersionStates(versionStateValues),
		asc.WithAppsReviewSubmissionStates(reviewSubmissionStateValues),
		asc.Limit[asc.AppsOption](limit),
		asc.NextURL[asc.AppsOption](next),
		asc.WithAppsAppInfoFields(appInfoFieldValues),
		asc.WithAppsInAppPurchaseFields(iapFieldValues),
		asc.WithAppsSubscriptionGroupFields(groupFieldValues),
		asc.WithAppsRetentionMessageFields(retentionMessageFieldValues),
	}
	includeValues := []string{}
	if len(appInfoFieldValues) > 0 {
		includeValues = addInclude(includeValues, "appInfos")
	}
	if len(iapFieldValues) > 0 {
		includeValues = addInclude(includeValues, "inAppPurchases")
	}
	if len(groupFieldValues) > 0 {
		includeValues = addInclude(includeValues, "subscriptionGroups")
	}
	if len(retentionMessageFieldValues) > 0 {
		includeValues = addInclude(includeValues, "retentionMessages")
	}
	opts = append(opts, asc.WithAppsInclude(includeValues))
	if strings.TrimSpace(sort) != "" {
		opts = append(opts, asc.WithAppsSort(sort))
	}

	if paginate {
		paginateOpts := append(opts, asc.Limit[asc.AppsOption](200))
		apps, err := d.PaginateWithSpinner(
			requestCtx,
			func(ctx context.Context) (asc.PaginatedResponse, error) {
				return client.GetApps(ctx, paginateOpts...)
			},
			func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetApps(ctx, asc.NextURL[asc.AppsOption](nextURL))
			},
		)
		if err != nil {
			return fmt.Errorf("apps: %w", err)
		}

		return d.PrintOutput(apps, output, pretty)
	}

	apps, err := client.GetApps(requestCtx, opts...)
	if err != nil {
		return fmt.Errorf("apps: failed to fetch: %w", err)
	}

	return d.PrintOutput(apps, output, pretty)
}

func csvContainsEmptyValue(value string) bool {
	for _, element := range strings.Split(value, ",") {
		if strings.TrimSpace(element) == "" {
			return true
		}
	}
	return false
}

func validateNotificationURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || strings.Contains(value, "#") {
		return "", fmt.Errorf("must be an absolute HTTPS URL without credentials or fragments")
	}
	return value, nil
}

func validateAccessibilityURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.User != nil {
		return "", fmt.Errorf("must be an absolute HTTP or HTTPS URL without credentials")
	}
	return value, nil
}

// These attributes are supported by GET /v1/apps/{id} and decoded by AppAttributes.
var appAttributeFields = []string{
	"name", "bundleId", "sku", "primaryLocale", "contentRightsDeclaration", "groupPurchaseInvitationMode", "accessibilityUrl",
	"subscriptionStatusUrl", "subscriptionStatusUrlVersion", "subscriptionStatusUrlForSandbox", "subscriptionStatusUrlVersionForSandbox",
}

// appViewIncludeValues are the include values GET /v1/apps/{id} accepts. The
// spec also lists appEncryptionDeclarations, but Apple rejects it with 400 "is
// not a valid relationship name".
var appViewIncludeValues = []string{
	"appStoreIcon", "ciProduct", "betaGroups", "appStoreVersions", "preReleaseVersions",
	"betaAppLocalizations", "builds", "betaLicenseAgreement", "betaAppReviewDetail", "appInfos", "appClips",
	"endUserLicenseAgreement", "inAppPurchases", "subscriptionGroups", "retentionMessages", "gameCenterEnabledVersions",
	"appCustomProductPages", "inAppPurchasesV2", "promotedPurchases", "appEvents", "reviewSubmissions",
	"subscriptionGracePeriod", "gameCenterDetail", "appStoreVersionExperimentsV2", "androidToIosAppMappingDetails",
}
