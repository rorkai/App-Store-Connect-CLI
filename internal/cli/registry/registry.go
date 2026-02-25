package registry

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/accessibility"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/account"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/actors"
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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/betaapplocalizations"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/betabuildlocalizations"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/buildbundles"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/buildlocalizations"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/builds"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/bundleids"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/categories"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/certificates"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/completion"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/crashes"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/devices"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/diffcmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/docs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/encryption"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/eula"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/feedback"
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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/offercodes"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/passtypeids"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/performance"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/preorders"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/prerelease"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/pricing"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/productpages"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/profiles"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/promotedpurchases"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/publish"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/releasenotes"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/reviews"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/routingcoverage"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/sandbox"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/screenshots"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/signing"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/status"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/submit"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/subscriptions"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/testflight"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/users"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/versions"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/videopreviews"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/webhooks"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/winbackoffers"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/workflow"
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

// Subcommands returns all root subcommands in display order.
func Subcommands(version string) []*ffcli.Command {
	subs := []*ffcli.Command{
		auth.AuthCommand(),
		auth.AuthDoctorCommand(),
		web.WebCommand(),
		account.AccountCommand(),
		install.InstallSkillsCommand(),
		initcmd.InitCommand(),
		docs.DocsCommand(),
		diffcmd.DiffCommand(),
		status.StatusCommand(),
		insights.InsightsCommand(),
		releasenotes.ReleaseNotesCommand(),
		feedback.FeedbackCommand(),
		crashes.CrashesCommand(),
		reviews.ReviewsCommand(),
		reviews.ReviewCommand(),
		analytics.AnalyticsCommand(),
		performance.PerformanceCommand(),
		finance.FinanceCommand(),
		apps.AppsCommand(),
		appclips.AppClipsCommand(),
		androidiosmapping.AndroidIosMappingCommand(),
		apps.AppSetupCommand(),
		apps.AppTagsCommand(),
		marketplace.MarketplaceCommand(),
		alternativedistribution.Command(),
		webhooks.WebhooksCommand(),
		nominations.NominationsCommand(),
		bundleids.BundleIDsCommand(),
		merchantids.MerchantIDsCommand(),
		certificates.CertificatesCommand(),
		passtypeids.PassTypeIDsCommand(),
		profiles.ProfilesCommand(),
		offercodes.OfferCodesCommand(),
		winbackoffers.WinBackOffersCommand(),
		users.UsersCommand(),
		actors.ActorsCommand(),
		devices.DevicesCommand(),
		testflight.TestFlightCommand(),
		builds.BuildsCommand(),
		buildbundles.BuildBundlesCommand(),
		publish.PublishCommand(),
		workflow.WorkflowCommand(),
		versions.VersionsCommand(),
		productpages.ProductPagesCommand(),
		routingcoverage.RoutingCoverageCommand(),
		apps.AppInfoCommand(),
		apps.AppInfosCommand(),
		eula.EULACommand(),
		agreements.AgreementsCommand(),
		pricing.PricingCommand(),
		preorders.PreOrdersCommand(),
		prerelease.PreReleaseVersionsCommand(),
		localizations.LocalizationsCommand(),
		metadata.MetadataCommand(),
		screenshots.ScreenshotsCommand(),
		videopreviews.VideoPreviewsCommand(),
		backgroundassets.BackgroundAssetsCommand(),
		buildlocalizations.BuildLocalizationsCommand(),
		betaapplocalizations.BetaAppLocalizationsCommand(),
		betabuildlocalizations.BetaBuildLocalizationsCommand(),
		sandbox.SandboxCommand(),
		signing.SigningCommand(),
		notarization.NotarizationCommand(),
		iap.IAPCommand(),
		app_events.Command(),
		subscriptions.SubscriptionsCommand(),
		submit.SubmitCommand(),
		validate.ValidateCommand(),
		xcodecloud.XcodeCloudCommand(),
		categories.CategoriesCommand(),
		agerating.AgeRatingCommand(),
		accessibility.AccessibilityCommand(),
		encryption.EncryptionCommand(),
		promotedpurchases.PromotedPurchasesCommand(),
		migrate.MigrateCommand(),
		notify.NotifyCommand(),
		gamecenter.GameCenterCommand(),
		VersionCommand(version),
	}

	subs = append(subs, completion.CompletionCommand(subs))
	return subs
}
