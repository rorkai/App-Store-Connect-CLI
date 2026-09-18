package xcodecloud

import (
	"flag"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var ciProductsProductTypeValues = []string{
	"APP",
	"FRAMEWORK",
}

var ciProductsFieldsValues = []string{
	"name",
	"createdDate",
	"productType",
	"app",
	"bundleId",
	"workflows",
	"primaryRepositories",
	"additionalRepositories",
	"buildRuns",
}

var ciProductsAppFieldsValues = []string{
	"accessibilityUrl",
	"name",
	"bundleId",
	"sku",
	"primaryLocale",
	"isOrEverWasMadeForKids",
	"subscriptionStatusUrl",
	"subscriptionStatusUrlVersion",
	"subscriptionStatusUrlForSandbox",
	"subscriptionStatusUrlVersionForSandbox",
	"contentRightsDeclaration",
	"streamlinedPurchasingEnabled",
	"accessibilityDeclarations",
	"appEncryptionDeclarations",
	"appStoreIcon",
	"ciProduct",
	"betaTesters",
	"betaGroups",
	"appStoreVersions",
	"appTags",
	"preReleaseVersions",
	"betaAppLocalizations",
	"builds",
	"betaLicenseAgreement",
	"betaAppReviewDetail",
	"appInfos",
	"appClips",
	"appPricePoints",
	"endUserLicenseAgreement",
	"appPriceSchedule",
	"appAvailabilityV2",
	"inAppPurchases",
	"subscriptionGroups",
	"gameCenterEnabledVersions",
	"perfPowerMetrics",
	"appCustomProductPages",
	"inAppPurchasesV2",
	"promotedPurchases",
	"appEvents",
	"reviewSubmissions",
	"subscriptionGracePeriod",
	"customerReviews",
	"customerReviewSummarizations",
	"gameCenterDetail",
	"appStoreVersionExperimentsV2",
	"alternativeDistributionKey",
	"analyticsReportRequests",
	"marketplaceSearchDetail",
	"buildUploads",
	"backgroundAssets",
	"betaFeedbackScreenshotSubmissions",
	"betaFeedbackCrashSubmissions",
	"searchKeywords",
	"webhooks",
	"androidToIosAppMappingDetails",
}

var ciProductsBundleIDFieldsValues = []string{
	"name",
	"platform",
	"identifier",
	"seedId",
	"profiles",
	"bundleIdCapabilities",
	"app",
}

var ciProductsScmRepositoryFieldsValues = []string{
	"lastAccessedDate",
	"httpCloneUrl",
	"sshCloneUrl",
	"ownerName",
	"repositoryName",
	"scmProvider",
	"defaultBranch",
	"gitReferences",
	"pullRequests",
}

var ciProductsIncludeValues = []string{
	"app",
	"bundleId",
	"primaryRepositories",
}

type ciProductsListFlags struct {
	flagSet                  *flag.FlagSet
	appID                    *string
	productType              *string
	fields                   *string
	appFields                *string
	bundleIDFields           *string
	scmRepositoryFields      *string
	include                  *string
	primaryRepositoriesLimit *int
	limit                    *int
	next                     *string
	paginate                 *bool
	output                   *string
	pretty                   *bool
}

func bindCiProductsListFlags(fs *flag.FlagSet) ciProductsListFlags {
	outputFlags := shared.BindOutputFlags(fs)
	return ciProductsListFlags{
		flagSet:                  fs,
		appID:                    fs.String("app", "", xcodeCloudAppFlagUsage),
		productType:              fs.String("product-type", "", "Filter by product type(s), comma-separated: "+strings.Join(ciProductsProductTypeValues, ", ")),
		fields:                   fs.String("fields", "", "Fields to include for CI products, comma-separated: "+strings.Join(ciProductsFieldsValues, ", ")),
		appFields:                fs.String("app-fields", "", "Fields to include for related apps, comma-separated: "+strings.Join(ciProductsAppFieldsValues, ", ")),
		bundleIDFields:           fs.String("bundle-id-fields", "", "Fields to include for related bundle IDs, comma-separated: "+strings.Join(ciProductsBundleIDFieldsValues, ", ")),
		scmRepositoryFields:      fs.String("scm-repository-fields", "", "Fields to include for related SCM repositories, comma-separated: "+strings.Join(ciProductsScmRepositoryFieldsValues, ", ")),
		include:                  fs.String("include", "", "Include related resources: "+strings.Join(ciProductsIncludeValues, ", ")),
		primaryRepositoriesLimit: fs.Int("primary-repositories-limit", 0, "Maximum included primary repositories (1-50)"),
		limit:                    fs.Int("limit", 0, "Maximum results per page (1-200)"),
		next:                     fs.String("next", "", "Fetch next page using a links.next URL"),
		paginate:                 fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)"),
		output:                   outputFlags.Output,
		pretty:                   outputFlags.Pretty,
	}
}

func normalizeCiProductsProductTypes(fs *flag.FlagSet, value string) ([]string, error) {
	values := shared.SplitCSVUpper(value)
	if ciProductsFlagProvided(fs, "product-type") && len(values) == 0 {
		return nil, fmt.Errorf("--product-type must not be empty")
	}
	for _, item := range values {
		if !containsCiProductsValue(ciProductsProductTypeValues, item) {
			return nil, fmt.Errorf("--product-type must be one of: %s", strings.Join(ciProductsProductTypeValues, ", "))
		}
	}
	return values, nil
}

func normalizeCiProductsSelection(fs *flag.FlagSet, value string, allowed []string, flagName string) ([]string, error) {
	values, err := shared.NormalizeSelection(value, allowed, flagName)
	if err != nil {
		return nil, err
	}
	if ciProductsFlagProvided(fs, strings.TrimPrefix(flagName, "--")) && len(values) == 0 {
		return nil, fmt.Errorf("%s must not be empty", flagName)
	}
	return values, nil
}

func ciProductsFlagProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(f *flag.Flag) {
		provided = provided || f.Name == name
	})
	return provided
}

func addCiProductsValue(values []string, value string) []string {
	if containsCiProductsValue(values, value) {
		return values
	}
	return append(values, value)
}

func containsCiProductsValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
