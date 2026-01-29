package registry

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/accessibility"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/actors"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/agerating"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/analytics"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/androidiosmapping"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/apps"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/buildbundles"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/buildlocalizations"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/builds"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/bundleids"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/categories"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/certificates"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/crashes"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/devices"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/encryption"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/eula"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/feedback"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/finance"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/gamecenter"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/iap"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/install"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/localizations"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/merchantids"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/migrate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/marketplace"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/nominations"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/offercodes"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/passtypeids"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/performance"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/preorders"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/prerelease"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/pricing"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/profiles"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/promotedpurchases"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/publish"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/reviews"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/routingcoverage"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/sandbox"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/signing"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/submit"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/subscriptions"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/testflight"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/users"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/versions"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/winbackoffers"
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
	return []*ffcli.Command{
		auth.AuthCommand(),
		install.InstallCommand(),
		feedback.FeedbackCommand(),
		crashes.CrashesCommand(),
		reviews.ReviewsCommand(),
		reviews.ReviewCommand(),
		analytics.AnalyticsCommand(),
		performance.PerformanceCommand(),
		finance.FinanceCommand(),
		apps.AppsCommand(),
		androidiosmapping.AndroidIosMappingCommand(),
		apps.AppSetupCommand(),
		apps.AppTagsCommand(),
		marketplace.MarketplaceCommand(),
		nominations.NominationsCommand(),
		bundleids.BundleIDsCommand(),
		merchantids.MerchantIDsCommand(),
		passtypeids.PassTypeIDsCommand(),
		certificates.CertificatesCommand(),
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
		versions.VersionsCommand(),
		routingcoverage.RoutingCoverageCommand(),
		apps.AppInfoCommand(),
		eula.EULACommand(),
		pricing.PricingCommand(),
		preorders.PreOrdersCommand(),
		prerelease.PreReleaseVersionsCommand(),
		localizations.LocalizationsCommand(),
		assets.AssetsCommand(),
		buildlocalizations.BuildLocalizationsCommand(),
		testflight.BetaGroupsCommand(),
		testflight.BetaTestersCommand(),
		sandbox.SandboxCommand(),
		signing.SigningCommand(),
		iap.IAPCommand(),
		subscriptions.SubscriptionsCommand(),
		submit.SubmitCommand(),
		xcodecloud.XcodeCloudCommand(),
		categories.CategoriesCommand(),
		agerating.AgeRatingCommand(),
		accessibility.AccessibilityCommand(),
		encryption.EncryptionCommand(),
		promotedpurchases.PromotedPurchasesCommand(),
		migrate.MigrateCommand(),
		gamecenter.GameCenterCommand(),
		VersionCommand(version),
	}
}
