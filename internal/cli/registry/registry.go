package registry

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/accessibility"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/account"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/actors"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/ads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/agerating"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/agreements"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/alternativedistribution"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/analytics"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/androidiosmapping"
	apicmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/api"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/app_events"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/appclips"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/apps"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/artifacts"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assetlibrary"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/backgroundassets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/buildbundles"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/buildlocalizations"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/builds"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/bundleids"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/capabilities"
	catalogcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/catalog"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/categories"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/certificates"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/completion"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/devices"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/diffcmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/distribute"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/docs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/encryption"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/eula"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/finance"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/gamecenter"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/iap"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/initcmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/insights"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/install"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/localizations"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/marketplace"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/merchantids"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/metadata"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/migrate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/nominations"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/notarization"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/notify"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/optimize"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/passtypeids"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/performance"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/preorders"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/pricing"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/productpages"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/profiles"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/publish"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/release"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/releasenotes"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/reviews"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/routingcoverage"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/sandbox"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/schema"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/screenshots"
	searchcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/search"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/signing"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/snitch"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/status"
	storekitcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/storekit"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/submit"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/subscriptions"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/systemstatus"
	telemetrycmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/telemetry"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/testflight"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/users"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/versions"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/videopreviews"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/webhooks"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/workflow"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/xcode"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/xcodecloud"
)

// VersionCommand returns a version subcommand that prints to d's stdout.
func VersionCommand(d *shared.Deps, version string) *ffcli.Command {
	return &ffcli.Command{
		Name:       "version",
		ShortUsage: "asc version",
		ShortHelp:  "Print version information and exit.",
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			fmt.Fprintln(d.IO.Out, version)
			return nil
		},
	}
}

type factory struct {
	name       string
	shortHelp  string
	newCommand func() *ffcli.Command
}

// Catalog constructs root commands on demand while preserving display order.
type Catalog struct {
	factories   []factory
	rootFlagSet *flag.FlagSet
}

// NewCatalog returns the current root command catalog.
func NewCatalog(version string) *Catalog {
	return NewCatalogWithDeps(version, shared.DefaultDeps())
}

// NewCatalogWithDeps returns the root command catalog with the commands that
// take dependencies built on d.
func NewCatalogWithDeps(version string, d *shared.Deps) *Catalog {
	catalog := &Catalog{}
	catalog.factories = []factory{
		commandFactory("auth", "Manage authentication for the App Store Connect API.", func() *ffcli.Command { return auth.NewAuthCommand(d) }),
		commandFactory("doctor", "Diagnose authentication configuration issues.", func() *ffcli.Command { return auth.NewAuthDoctorCommand(d) }),
		commandFactory("web", "Apple web-session workflows, including finance report downloads.", func() *ffcli.Command {
			return web.NewWebCommand(d)
		}),
		commandFactory("account", "Inspect account-level health and access signals.", func() *ffcli.Command {
			return account.NewAccountCommand(d)
		}),
		commandFactory("install-skills", "Install the asc skill pack globally for App Store Connect workflows.", func() *ffcli.Command { return install.NewInstallSkillsCommand(d) }),
		commandFactory("init", "Initialize asc helper docs in the current repo.", func() *ffcli.Command {
			return initcmd.NewInitCommand(d)
		}),
		commandFactory("docs", "Access embedded documentation guides and reference helpers.", func() *ffcli.Command {
			return docs.NewCommand(d)
		}),
		commandFactory("diff", "Generate deterministic non-mutating diff plans.", func() *ffcli.Command {
			return diffcmd.NewDiffCommand(d)
		}),
		commandFactory("system-status", "Check Apple Developer service health.", func() *ffcli.Command {
			return systemstatus.NewCommand(d)
		}),
		commandFactory("status", "Show a release pipeline dashboard for an app.", func() *ffcli.Command {
			return status.NewStatusCommand(d)
		}),
		commandFactory("insights", "Generate weekly and daily insights from App Store data sources.", func() *ffcli.Command {
			return insights.NewInsightsCommand(d)
		}),
		commandFactory("release-notes", "Generate and manage App Store release notes.", func() *ffcli.Command {
			return releasenotes.NewCommand(d)
		}),
		commandFactory("reviews", "List and manage App Store customer reviews.", func() *ffcli.Command {
			return reviews.NewReviewsCommand(d)
		}),
		commandFactory("review", "Manage App Store review details, attachments, and submissions.", func() *ffcli.Command {
			return reviews.NewReviewCommand(d)
		}),
		commandFactory("analytics", "Request and download analytics and sales reports.", func() *ffcli.Command {
			return analytics.NewAnalyticsCommand(d)
		}),
		commandFactory("ads", "Manage Apple Ads API resources.", func() *ffcli.Command {
			return ads.NewAdsCommand(d)
		}),
		commandFactory("optimize", "Build cross-API optimization plans.", func() *ffcli.Command { return optimize.NewOptimizeCommand(d) }),
		commandFactory("performance", "Access performance metrics and diagnostic logs.", func() *ffcli.Command {
			return performance.NewPerformanceCommand(d)
		}),
		commandFactory("finance", "Download payments and financial reports.", func() *ffcli.Command {
			return finance.NewFinanceCommand(d)
		}),
		commandFactory("apps", "List and manage apps in App Store Connect.", func() *ffcli.Command {
			return apps.NewAppsCommand(d)
		}),
		commandFactory("app-clips", "Manage App Clip experiences and invocations.", func() *ffcli.Command {
			return appclips.NewAppClipsCommand(d)
		}),
		commandFactory("android-ios-mapping", "Manage Android-to-iOS app mapping details.", func() *ffcli.Command {
			return androidiosmapping.NewAndroidIosMappingCommand(d)
		}),
		commandFactory("app-setup", "Post-create app setup automation.", func() *ffcli.Command {
			return apps.NewAppSetupCommand(d)
		}),
		commandFactory("app-tags", "Inspect Apple-generated App Store discoverability tags.", func() *ffcli.Command {
			return apps.NewAppTagsCommand(d)
		}),
		commandFactory("marketplace", "Manage marketplace resources.", func() *ffcli.Command {
			return marketplace.NewMarketplaceCommand(d)
		}),
		commandFactory("alternative-distribution", "Manage alternative distribution resources.", func() *ffcli.Command {
			return alternativedistribution.NewAlternativeDistributionCommand(d)
		}),
		commandFactory("webhooks", "Manage webhooks in App Store Connect.", func() *ffcli.Command {
			return webhooks.NewWebhooksCommand(d)
		}),
		commandFactory("nominations", "Manage featuring nominations.", func() *ffcli.Command {
			return nominations.NewNominationsCommand(d)
		}),
		commandFactory("bundle-ids", "Manage bundle IDs and capabilities.", func() *ffcli.Command {
			return bundleids.NewBundleIDsCommand(d)
		}),
		commandFactory("merchant-ids", "Manage merchant IDs and certificates.", func() *ffcli.Command { return merchantids.NewMerchantIDsCommand(d) }),
		commandFactory("certificates", "Manage signing certificates.", func() *ffcli.Command { return certificates.NewCertificatesCommand(d) }),
		commandFactory("pass-type-ids", "Manage pass type IDs.", func() *ffcli.Command {
			return passtypeids.NewPassTypeIDsCommand(d)
		}),
		commandFactory("profiles", "Manage provisioning profiles.", func() *ffcli.Command {
			return profiles.NewProfilesCommand(d)
		}),
		commandFactory("users", "Manage users and invitations in App Store Connect.", func() *ffcli.Command {
			return users.NewUsersCommand(d)
		}),
		commandFactory("actors", "Lookup actors (users, API keys) by ID.", func() *ffcli.Command {
			return actors.NewActorsCommand(d)
		}),
		commandFactory("devices", "Manage devices in App Store Connect.", func() *ffcli.Command {
			return devices.NewDevicesCommand(d)
		}),
		commandFactory("testflight", "Manage TestFlight workflows.", func() *ffcli.Command {
			return testflight.NewTestFlightCommand(d)
		}),
		commandFactory("builds", "Manage builds in App Store Connect.", func() *ffcli.Command {
			return builds.NewBuildsCommand(d)
		}),
		commandFactory("build-bundles", "Manage build bundles and App Clip data.", func() *ffcli.Command {
			return buildbundles.NewBuildBundlesCommand(d)
		}),
		commandFactory("publish", "High-level publish workflows for TestFlight and App Store.", func() *ffcli.Command {
			return publish.NewPublishCommand(d)
		}),
		commandFactory("release", "Run high-level App Store release workflows.", func() *ffcli.Command {
			return release.NewReleaseCommand(d)
		}),
		commandFactory("workflow", "Run multi-step automation workflows.", func() *ffcli.Command { return workflow.NewWorkflowCommand(d) }),
		commandFactory("xcode", "Local Xcode build/archive/export and signing-settings helpers.", func() *ffcli.Command {
			return xcode.NewXcodeCommand(d)
		}),
		commandFactory("distribute", "Plan, execute, inspect, and publish iOS distribution artifacts.", func() *ffcli.Command {
			return distribute.NewDistributeCommand(d)
		}),
		commandFactory("ipa-info", "Inspect a local IPA without contacting App Store Connect.", func() *ffcli.Command {
			return artifacts.NewIPAInfoCommand(d)
		}),
		commandFactory("pkg-info", "Inspect a local flat package or product archive without contacting Apple.", func() *ffcli.Command {
			return artifacts.NewPKGInfoCommand(d)
		}),
		commandFactory("versions", "Manage App Store versions.", func() *ffcli.Command {
			return versions.NewVersionsCommand(d)
		}),
		commandFactory("product-pages", "Manage custom product pages and product page experiments.", func() *ffcli.Command {
			return productpages.NewProductPagesCommand(d)
		}),
		commandFactory("routing-coverage", "Manage routing app coverage files.", func() *ffcli.Command {
			return routingcoverage.NewRoutingCoverageCommand(d)
		}),
		commandFactory("eula", "Manage End User License Agreements (EULA).", func() *ffcli.Command {
			return eula.NewEULACommand(d)
		}),
		commandFactory("agreements", "Manage agreements in App Store Connect.", func() *ffcli.Command {
			return agreements.NewAgreementsCommand(d)
		}),
		commandFactory("pricing", "Manage app pricing and availability.", func() *ffcli.Command {
			return pricing.NewPricingCommand(d)
		}),
		commandFactory("pre-orders", "Manage app pre-orders.", func() *ffcli.Command {
			return preorders.NewPreOrdersCommand(d)
		}),
		commandFactory("localizations", "Manage App Store localization metadata.", func() *ffcli.Command {
			return localizations.NewLocalizationsCommand(d)
		}),
		commandFactory("metadata", "Manage app metadata with deterministic workflows and keyword tooling.", func() *ffcli.Command {
			return metadata.NewMetadataCommand(d)
		}),
		commandFactory("screenshots", "Upload and manage App Store screenshots, including local capture, framing, and matrices.", func() *ffcli.Command {
			return screenshots.NewScreenshotsCommand(d)
		}),
		commandFactory("video-previews", "Manage App Store app preview videos.", func() *ffcli.Command {
			return videopreviews.NewVideoPreviewsCommand(d)
		}),
		commandFactory("asset-library", "Inspect Asset Library media and specifications, and upload images and videos.", func() *ffcli.Command {
			return assetlibrary.NewCommand(d)
		}),
		commandFactory("background-assets", "Manage background assets.", func() *ffcli.Command {
			return backgroundassets.NewBackgroundAssetsCommand(d)
		}),
		commandFactory("build-localizations", "Manage build release notes localizations.", func() *ffcli.Command {
			return buildlocalizations.NewBuildLocalizationsCommand(d)
		}),
		commandFactory("sandbox", "Manage sandbox testers in App Store Connect.", func() *ffcli.Command {
			return sandbox.NewSandboxCommand(d)
		}),
		commandFactory("signing", "Manage signing certificates and profiles.", func() *ffcli.Command { return signing.NewSigningCommand(d) }),
		commandFactory("notarization", "Manage macOS notarization submissions.", func() *ffcli.Command {
			return notarization.NewNotarizationCommand(d)
		}),
		commandFactory("catalog", "Plan and reconcile a product catalog from a JSON file.", func() *ffcli.Command { return catalogcmd.NewCommand(d) }),
		commandFactory("iap", "Manage in-app purchases in App Store Connect.", func() *ffcli.Command {
			return iap.NewIAPCommand(d)
		}),
		commandFactory("storekit", "Manage StoreKit server APIs with In-App Purchase API keys.", func() *ffcli.Command {
			return storekitcmd.NewCommand(d)
		}),
		commandFactory("app-events", "Manage App Store in-app events.", func() *ffcli.Command {
			return app_events.NewAppEventsCommand(d)
		}),
		commandFactory("subscriptions", "Manage subscription groups and subscriptions.", func() *ffcli.Command {
			return subscriptions.NewSubscriptionsCommand(d)
		}),
		commandFactory("submit", "Submission lifecycle tools; use `review submit` for an existing build.", func() *ffcli.Command {
			return submit.NewSubmitCommand(d)
		}),
		commandFactory("validate", "Canonical App Store submission readiness report.", func() *ffcli.Command { return validate.NewValidateCommand(d) }),
		commandFactory("xcode-cloud", "Trigger and monitor Xcode Cloud workflows.", func() *ffcli.Command {
			return xcodecloud.NewXcodeCloudCommand(d)
		}),
		commandFactory("categories", "Manage App Store categories.", func() *ffcli.Command {
			return categories.NewCategoriesCommand(d)
		}),
		commandFactory("age-rating", "Manage App Store age rating declarations.", func() *ffcli.Command {
			return agerating.NewAgeRatingCommand(d)
		}),
		commandFactory("accessibility", "Manage accessibility declarations.", func() *ffcli.Command {
			return accessibility.NewAccessibilityCommand(d)
		}),
		commandFactory("encryption", "Manage app encryption declarations and documents.", func() *ffcli.Command {
			return encryption.NewEncryptionCommand(d)
		}),
		commandFactory("migrate", "Migrate metadata from/to fastlane format.", func() *ffcli.Command {
			return migrate.NewMigrateCommand(d)
		}),
		commandFactory("notify", "Send notifications to external services.", func() *ffcli.Command { return notify.NewNotifyCommand(d) }),
		commandFactory("game-center", "Manage Game Center resources in App Store Connect.", func() *ffcli.Command {
			return gamecenter.NewGameCenterCommand(d)
		}),
		commandFactory("capabilities", "Show CLI, API, web-only, and public-API-limited capability coverage.", func() *ffcli.Command {
			return capabilities.NewCommand(d)
		}),
		commandFactory("schema", "Inspect App Store Connect API endpoint schemas at runtime.", func() *ffcli.Command {
			return schema.NewSchemaCommand(d)
		}),
		commandFactory("api", "Send an authenticated raw request to the App Store Connect API.", func() *ffcli.Command {
			return apicmd.NewCommand(d)
		}),
		commandFactory("telemetry", "Manage CLI telemetry settings.", func() *ffcli.Command {
			return telemetrycmd.NewTelemetryCommand(d)
		}),
		commandFactory("commands", "Discover registered command families and their inputs.", func() *ffcli.Command {
			return catalog.newCommandsCommand(d)
		}),
		commandFactory("search", "Search asc commands and examples for agent-oriented command discovery.", func() *ffcli.Command {
			return searchcmd.NewSearchCommand(d, catalog.All)
		}),
		commandFactory("snitch", "Report CLI friction as a GitHub issue.", func() *ffcli.Command {
			return snitch.NewSnitchCommand(d, version)
		}),
		commandFactory("version", "Print version information and exit.", func() *ffcli.Command {
			return VersionCommand(d, version)
		}),
		commandFactory("completion", "Print shell completion scripts.", func() *ffcli.Command {
			return completion.NewCompletionCommand(d, catalog.All, func() *flag.FlagSet {
				return catalog.rootFlagSet
			})
		}),
	}
	return catalog
}

// SetCompletionRootFlagSet supplies the flags accepted by the root command.
func (c *Catalog) SetCompletionRootFlagSet(fs *flag.FlagSet) {
	c.rootFlagSet = fs
}

func commandFactory(name, shortHelp string, newCommand func() *ffcli.Command) factory {
	return factory{name: name, shortHelp: shortHelp, newCommand: newCommand}
}

// MetadataCommands returns lightweight root entries without invoking factories.
func (c *Catalog) MetadataCommands() []*ffcli.Command {
	commands := make([]*ffcli.Command, 0, len(c.factories))
	for _, factory := range c.factories {
		commands = append(commands, &ffcli.Command{
			Name:      factory.name,
			ShortHelp: factory.shortHelp,
			UsageFunc: shared.DefaultUsageFunc,
		})
	}
	return commands
}

// CommandsFor returns root metadata with only the requested command materialized.
func (c *Catalog) CommandsFor(name string) []*ffcli.Command {
	commands := c.MetadataCommands()
	for i, factory := range c.factories {
		if strings.EqualFold(factory.name, strings.TrimSpace(name)) {
			commands[i] = materialize(factory)
			break
		}
	}
	return commands
}

// All materializes every command for full-tree callers such as search and docs.
func (c *Catalog) All() []*ffcli.Command {
	commands := make([]*ffcli.Command, 0, len(c.factories))
	for _, factory := range c.factories {
		commands = append(commands, materialize(factory))
	}
	return commands
}

func materialize(factory factory) *ffcli.Command {
	if factory.newCommand == nil {
		return nil
	}
	return factory.newCommand()
}

// Subcommands returns all root subcommands in display order.
func Subcommands(version string) []*ffcli.Command {
	return NewCatalog(version).All()
}
