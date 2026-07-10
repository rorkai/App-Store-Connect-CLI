package registry

import (
	"context"
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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/app_events"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/appclips"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/apps"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/backgroundassets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/buildbundles"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/buildlocalizations"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/builds"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/bundleids"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/capabilities"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/categories"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/certificates"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/completion"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/devices"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/diffcmd"
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

// VersionCommand returns a version subcommand.
func VersionCommand(version string) *ffcli.Command {
	return &ffcli.Command{
		Name:       "version",
		ShortUsage: "asc version",
		ShortHelp:  "Print version information and exit.",
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			fmt.Println(version)
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
	factories []factory
}

// NewCatalog returns the current root command catalog.
func NewCatalog(version string) *Catalog {
	catalog := &Catalog{}
	catalog.factories = []factory{
		commandFactory("auth", "Manage authentication for the App Store Connect API.", auth.AuthCommand),
		commandFactory("doctor", "Diagnose authentication configuration issues.", auth.AuthDoctorCommand),
		commandFactory("web", "Apple web-session workflows.", web.WebCommand),
		commandFactory("account", "Inspect account-level health and access signals.", account.AccountCommand),
		commandFactory("install-skills", "Install the asc skill pack globally for App Store Connect workflows.", install.InstallSkillsCommand),
		commandFactory("init", "Initialize asc helper docs in the current repo.", initcmd.InitCommand),
		commandFactory("docs", "Access embedded documentation guides and reference helpers.", docs.DocsCommand),
		commandFactory("diff", "Generate deterministic non-mutating diff plans.", diffcmd.DiffCommand),
		commandFactory("status", "Show a release pipeline dashboard for an app.", status.StatusCommand),
		commandFactory("insights", "Generate weekly and daily insights from App Store data sources.", insights.InsightsCommand),
		commandFactory("release-notes", "Generate and manage App Store release notes.", releasenotes.ReleaseNotesCommand),
		commandFactory("reviews", "List and manage App Store customer reviews.", reviews.ReviewsCommand),
		commandFactory("review", "Manage App Store review details, attachments, and submissions.", reviews.ReviewCommand),
		commandFactory("analytics", "Request and download analytics and sales reports.", analytics.AnalyticsCommand),
		commandFactory("ads", "Manage Apple Ads Campaign Management API resources.", ads.AdsCommand),
		commandFactory("performance", "Access performance metrics and diagnostic logs.", performance.PerformanceCommand),
		commandFactory("finance", "Download payments and financial reports.", finance.FinanceCommand),
		commandFactory("apps", "List and manage apps in App Store Connect.", apps.AppsCommand),
		commandFactory("app-clips", "Manage App Clip experiences and invocations.", appclips.AppClipsCommand),
		commandFactory("android-ios-mapping", "Manage Android-to-iOS app mapping details.", androidiosmapping.AndroidIosMappingCommand),
		commandFactory("app-setup", "Post-create app setup automation.", apps.AppSetupCommand),
		commandFactory("app-tags", "Inspect Apple-generated App Store discoverability tags.", apps.AppTagsCommand),
		commandFactory("marketplace", "Manage marketplace resources.", marketplace.MarketplaceCommand),
		commandFactory("alternative-distribution", "Manage alternative distribution resources.", alternativedistribution.Command),
		commandFactory("webhooks", "Manage webhooks in App Store Connect.", webhooks.WebhooksCommand),
		commandFactory("nominations", "Manage featuring nominations.", nominations.NominationsCommand),
		commandFactory("bundle-ids", "Manage bundle IDs and capabilities.", bundleids.BundleIDsCommand),
		commandFactory("merchant-ids", "Manage merchant IDs and certificates.", merchantids.MerchantIDsCommand),
		commandFactory("certificates", "Manage signing certificates.", certificates.CertificatesCommand),
		commandFactory("pass-type-ids", "Manage pass type IDs.", passtypeids.PassTypeIDsCommand),
		commandFactory("profiles", "Manage provisioning profiles.", profiles.ProfilesCommand),
		commandFactory("users", "Manage users and invitations in App Store Connect.", users.UsersCommand),
		commandFactory("actors", "Lookup actors (users, API keys) by ID.", actors.ActorsCommand),
		commandFactory("devices", "Manage devices in App Store Connect.", devices.DevicesCommand),
		commandFactory("testflight", "Manage TestFlight workflows.", testflight.TestFlightCommand),
		commandFactory("builds", "Manage builds in App Store Connect.", builds.BuildsCommand),
		commandFactory("build-bundles", "Manage build bundles and App Clip data.", buildbundles.BuildBundlesCommand),
		commandFactory("publish", "High-level publish workflows for TestFlight and App Store.", publish.PublishCommand),
		commandFactory("release", "Run high-level App Store release workflows.", release.ReleaseCommand),
		commandFactory("workflow", "Run multi-step automation workflows.", workflow.WorkflowCommand),
		commandFactory("xcode", "Local Xcode archive/export helpers (macOS only).", xcode.XcodeCommand),
		commandFactory("versions", "Manage App Store versions.", versions.VersionsCommand),
		commandFactory("product-pages", "Manage custom product pages and product page experiments.", productpages.ProductPagesCommand),
		commandFactory("routing-coverage", "Manage routing app coverage files.", routingcoverage.RoutingCoverageCommand),
		commandFactory("eula", "Manage End User License Agreements (EULA).", eula.EULACommand),
		commandFactory("agreements", "Manage agreements in App Store Connect.", agreements.AgreementsCommand),
		commandFactory("pricing", "Manage app pricing and availability.", pricing.PricingCommand),
		commandFactory("pre-orders", "Manage app pre-orders.", preorders.PreOrdersCommand),
		commandFactory("localizations", "Manage App Store localization metadata.", localizations.LocalizationsCommand),
		commandFactory("metadata", "Manage app metadata with deterministic workflows and keyword tooling.", metadata.MetadataCommand),
		commandFactory("screenshots", "Upload and manage App Store screenshots; local capture/frame workflow is [experimental].", screenshots.ScreenshotsCommand),
		commandFactory("video-previews", "Manage App Store app preview videos.", videopreviews.VideoPreviewsCommand),
		commandFactory("background-assets", "Manage background assets.", backgroundassets.BackgroundAssetsCommand),
		commandFactory("build-localizations", "Manage build release notes localizations.", buildlocalizations.BuildLocalizationsCommand),
		commandFactory("sandbox", "Manage sandbox testers in App Store Connect.", sandbox.SandboxCommand),
		commandFactory("signing", "Manage signing certificates and profiles.", signing.SigningCommand),
		commandFactory("notarization", "Manage macOS notarization submissions.", notarization.NotarizationCommand),
		commandFactory("iap", "Manage in-app purchases in App Store Connect.", iap.IAPCommand),
		commandFactory("storekit", "Manage StoreKit server APIs with In-App Purchase API keys.", storekitcmd.Command),
		commandFactory("app-events", "Manage App Store in-app events.", app_events.Command),
		commandFactory("subscriptions", "Manage subscription groups and subscriptions.", subscriptions.SubscriptionsCommand),
		commandFactory("submit", "Submission lifecycle tools; use `publish appstore --submit` to ship.", submit.SubmitCommand),
		commandFactory("validate", "Canonical App Store submission readiness report.", validate.ValidateCommand),
		commandFactory("xcode-cloud", "Trigger and monitor Xcode Cloud workflows.", xcodecloud.XcodeCloudCommand),
		commandFactory("categories", "Manage App Store categories.", categories.CategoriesCommand),
		commandFactory("age-rating", "Manage App Store age rating declarations.", agerating.AgeRatingCommand),
		commandFactory("accessibility", "Manage accessibility declarations.", accessibility.AccessibilityCommand),
		commandFactory("encryption", "Manage app encryption declarations and documents.", encryption.EncryptionCommand),
		commandFactory("migrate", "Migrate metadata from/to fastlane format.", migrate.MigrateCommand),
		commandFactory("notify", "Send notifications to external services.", notify.NotifyCommand),
		commandFactory("game-center", "Manage Game Center resources in App Store Connect.", gamecenter.GameCenterCommand),
		commandFactory("capabilities", "Show CLI, API, web-only, and public-API-limited capability coverage.", capabilities.Command),
		commandFactory("schema", "Inspect App Store Connect API endpoint schemas at runtime.", schema.SchemaCommand),
		commandFactory("telemetry", "Manage CLI telemetry settings.", telemetrycmd.TelemetryCommand),
		commandFactory("search", "Search asc commands and examples for agent-oriented command discovery.", func() *ffcli.Command {
			return searchcmd.SearchCommand(catalog.All)
		}),
		commandFactory("snitch", "Report CLI friction as a GitHub issue.", func() *ffcli.Command {
			return snitch.SnitchCommand(version)
		}),
		commandFactory("version", "Print version information and exit.", func() *ffcli.Command {
			return VersionCommand(version)
		}),
		commandFactory("completion", "Print shell completion scripts.", func() *ffcli.Command {
			return completion.CompletionCommand(catalog.MetadataCommands())
		}),
	}
	return catalog
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
