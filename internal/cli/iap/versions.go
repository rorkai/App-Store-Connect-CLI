package iap

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var iapVersionStates = []string{
	"PREPARE_FOR_SUBMISSION", "READY_FOR_REVIEW", "WAITING_FOR_REVIEW", "IN_REVIEW",
	"ACCEPTED", "APPROVED", "REPLACED_WITH_NEW_VERSION", "REJECTED", "DEVELOPER_REJECTED",
}

var iapVersionIncludes = []string{"inAppPurchase", "image", "images", "localizations"}

var (
	iapVersionFields             = []string{"version", "state", "inAppPurchase", "image", "images", "localizations"}
	iapVersionIAPFields          = []string{"name", "productId", "inAppPurchaseType", "state", "reviewNote", "familySharable", "contentHosting", "inAppPurchaseLocalizations", "pricePoints", "content", "appStoreReviewScreenshot", "promotedPurchase", "iapPriceSchedule", "inAppPurchaseAvailability", "images", "offerCodes", "versions"}
	iapVersionImageFields        = []string{"fileSize", "fileName", "assetToken", "imageAsset", "uploadOperations", "assetDeliveryState"}
	iapVersionLocalizationFields = []string{"name", "locale", "description", "version"}
)

type iapVersionFieldFlagValues struct {
	version      *string
	iap          *string
	image        *string
	localization *string
}

type iapVersionFieldSelections struct {
	version      []string
	iap          []string
	image        []string
	localization []string
}

func bindIAPVersionFieldFlags(fs *flag.FlagSet) iapVersionFieldFlagValues {
	return iapVersionFieldFlagValues{
		version:      fs.String("version-fields", "", "fields[inAppPurchaseVersions] (comma-separated)"),
		iap:          fs.String("iap-fields", "", "fields[inAppPurchases] (comma-separated)"),
		image:        fs.String("image-fields", "", "fields[inAppPurchaseImages] (comma-separated)"),
		localization: fs.String("localization-fields", "", "fields[inAppPurchaseLocalizations] (comma-separated)"),
	}
}

func normalizeIAPVersionFieldFlags(flags iapVersionFieldFlagValues) (iapVersionFieldSelections, error) {
	var selections iapVersionFieldSelections
	checks := []struct {
		value   string
		allowed []string
		name    string
		target  *[]string
	}{
		{*flags.version, iapVersionFields, "--version-fields", &selections.version},
		{*flags.iap, iapVersionIAPFields, "--iap-fields", &selections.iap},
		{*flags.image, iapVersionImageFields, "--image-fields", &selections.image},
		{*flags.localization, iapVersionLocalizationFields, "--localization-fields", &selections.localization},
	}
	for _, check := range checks {
		values, err := shared.NormalizeSelection(check.value, check.allowed, check.name)
		if err != nil {
			return iapVersionFieldSelections{}, err
		}
		*check.target = values
	}
	return selections, nil
}

// newIAPVersionsCommand returns the IAP versions command group.
func newIAPVersionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc iap versions <subcommand> [flags]",
		ShortHelp:  "Manage versioned in-app purchase review content.",
		LongHelp: `Manage versioned in-app purchase review content.

Examples:
  asc iap versions list --iap-id "IAP_ID"
  asc iap versions create --iap-id "IAP_ID"
  asc iap versions view --version-id "VERSION_ID"
  asc iap versions submit --version-id "VERSION_ID" --submission "SUBMISSION_ID" --confirm`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newIAPVersionsCreateCommand(d), newIAPVersionsListCommand(d), newIAPVersionsViewCommand(d),
			newIAPVersionImageCommand(d), newIAPVersionImagesCommand(d), newIAPVersionLocalizationsCommand(d),
			newIAPVersionLinksCommand(d), newIAPVersionSubmitCommand(d),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func newIAPVersionsCreateCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		FlagSetName: "versions create",
		Name:        "create",
		ShortUsage:  `asc iap versions create --iap-id "IAP_ID"`,
		ShortHelp:   "Create an in-app purchase version.",
		LongHelp:    "Create an in-app purchase version.\n\nExamples:\n  asc iap versions create --iap-id \"IAP_ID\"",
		IDFlag:      "iap-id",
		IDUsage:     "In-app purchase ID",
		IDType:      "inAppPurchases",
		Deps:        d,
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateInAppPurchaseVersion(ctx, id)
		},
	})
}

func normalizeIAPVersionStates(value string) ([]string, error) {
	values := shared.SplitCSVUpper(value)
	if len(values) == 0 {
		return nil, nil
	}
	allowed := make(map[string]struct{}, len(iapVersionStates))
	for _, state := range iapVersionStates {
		allowed[state] = struct{}{}
	}
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			return nil, fmt.Errorf("--state must be one of: %s", strings.Join(iapVersionStates, ", "))
		}
	}
	return values, nil
}

func bindIAPVersionQueryFlags(fs *flag.FlagSet) (state, include *string, limit, imagesLimit, localizationsLimit *int, next *string, paginate *bool) {
	state = fs.String("state", "", "Filter by state (comma-separated)")
	include = fs.String("include", "", "Include relationships: inAppPurchase,image,images,localizations")
	limit = fs.Int("limit", 0, shared.LimitFlagUsage(200))
	imagesLimit = fs.Int("images-limit", 0, "Maximum included images (1-50)")
	localizationsLimit = fs.Int("localizations-limit", 0, "Maximum included localizations (1-50)")
	next = fs.String("next", "", shared.NextFlagUsage)
	paginate = fs.Bool("paginate", false, shared.PaginateFlagUsage)
	return
}

func iapVersionQueryOptions(stateValue, includeValue string, limit, imagesLimit, localizationsLimit int, next string, fieldFlags iapVersionFieldFlagValues) ([]asc.IAPVersionsOption, error) {
	if imagesLimit != 0 && (imagesLimit < 1 || imagesLimit > 50) {
		return nil, fmt.Errorf("--images-limit must be between 1 and 50")
	}
	if localizationsLimit != 0 && (localizationsLimit < 1 || localizationsLimit > 50) {
		return nil, fmt.Errorf("--localizations-limit must be between 1 and 50")
	}
	states, err := normalizeIAPVersionStates(stateValue)
	if err != nil {
		return nil, err
	}
	include, err := shared.NormalizeSelection(includeValue, iapVersionIncludes, "--include")
	if err != nil {
		return nil, err
	}
	fields, err := normalizeIAPVersionFieldFlags(fieldFlags)
	if err != nil {
		return nil, err
	}
	return []asc.IAPVersionsOption{
		asc.WithIAPVersionsStates(states),
		asc.WithIAPVersionsInclude(include),
		asc.Limit[asc.IAPVersionsOption](limit),
		asc.WithIAPVersionsImagesLimit(imagesLimit),
		asc.WithIAPVersionsLocalizationsLimit(localizationsLimit),
		asc.WithIAPVersionsNextURL(next),
		asc.WithIAPVersionsFields(fields.version),
		asc.WithIAPVersionsIAPFields(fields.iap),
		asc.WithIAPVersionsImageFields(fields.image),
		asc.WithIAPVersionsLocalizationFields(fields.localization),
	}, nil
}

func newIAPVersionsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions list", flag.ExitOnError)
	iapID := shared.BindResourceIDFlag(fs, "iap-id", "inAppPurchases", "In-app purchase ID")
	state, include, limit, imagesLimit, localizationsLimit, next, paginate := bindIAPVersionQueryFlags(fs)
	fieldFlags := bindIAPVersionFieldFlags(fs)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortUsage: `asc iap versions list --iap-id "IAP_ID" [flags]`, ShortHelp: "List versions for an in-app purchase.",
		LongHelp: `List versions for an in-app purchase.

To find the in-app purchase ID, list the app's in-app purchases and use the
returned id field:
  asc iap list --app "APP_ID" --paginate --output json

Examples:
  asc iap versions list --iap-id "IAP_ID"
  asc iap versions list --iap-id "IAP_ID" --state PREPARE_FOR_SUBMISSION --paginate`, FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "iap versions list", 200, "state", "include", "limit", "images-limit", "localizations-limit", "version-fields", "iap-fields", "image-fields", "localization-fields"); err != nil {
				return err
			}
			id := strings.TrimSpace(*iapID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--iap-id")
			}
			firstPageLimit := *limit
			if *paginate && firstPageLimit == 0 && strings.TrimSpace(*next) == "" {
				firstPageLimit = 200
			}
			opts, err := iapVersionQueryOptions(*state, *include, firstPageLimit, *imagesLimit, *localizationsLimit, *next, fieldFlags)
			if err != nil {
				return d.UsageError("iap versions list: " + err.Error())
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("iap versions list: %w", err)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			resp, err := client.GetInAppPurchaseVersions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap versions list: failed to fetch: %w", err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchaseVersions(ctx, id, asc.WithIAPVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap versions list: %w", err)
				}
				return d.PrintOutput(aggregated, *output.Output, *output.Pretty)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func newIAPVersionsViewCommand(d *shared.Deps) *ffcli.Command {
	var opts []asc.IAPVersionGetOption

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName:        "versions view",
		Name:               "view",
		ShortUsage:         `asc iap versions view --version-id "VERSION_ID" [flags]`,
		ShortHelp:          "View an in-app purchase version.",
		LongHelp:           "View an in-app purchase version.\n\nExamples:\n  asc iap versions view --version-id \"VERSION_ID\" --include localizations,images",
		IDFlag:             "version-id",
		IDUsage:            "In-app purchase version ID",
		IDType:             "inAppPurchaseVersions",
		RequiredDiagnostic: true,
		Deps:               d,
		Flags: func(fs *flag.FlagSet) func() error {
			shared.BindFlagAlias(fs, "id", "version-id")
			include := fs.String("include", "", "Include relationships: inAppPurchase,image,images,localizations")
			imagesLimit := fs.Int("images-limit", 0, "Maximum included images (1-50)")
			localizationsLimit := fs.Int("localizations-limit", 0, "Maximum included localizations (1-50)")
			fieldFlags := bindIAPVersionFieldFlags(fs)
			return func() error {
				if *imagesLimit != 0 && (*imagesLimit < 1 || *imagesLimit > 50) {
					return d.UsageError("iap versions view: --images-limit must be between 1 and 50")
				}
				if *localizationsLimit != 0 && (*localizationsLimit < 1 || *localizationsLimit > 50) {
					return d.UsageError("iap versions view: --localizations-limit must be between 1 and 50")
				}
				includes, err := shared.NormalizeSelection(*include, iapVersionIncludes, "--include")
				if err != nil {
					return d.UsageError("iap versions view: " + err.Error())
				}
				fields, err := normalizeIAPVersionFieldFlags(fieldFlags)
				if err != nil {
					return d.UsageError("iap versions view: " + err.Error())
				}
				opts = []asc.IAPVersionGetOption{
					asc.WithIAPVersionGetInclude(includes),
					asc.WithIAPVersionGetFields(fields.version),
					asc.WithIAPVersionGetIAPFields(fields.iap),
					asc.WithIAPVersionGetImageFields(fields.image),
					asc.WithIAPVersionGetLocalizationFields(fields.localization),
					asc.WithIAPVersionGetImagesLimit(*imagesLimit),
					asc.WithIAPVersionGetLocalizationsLimit(*localizationsLimit),
				}
				return nil
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetInAppPurchaseVersion(ctx, id, opts...)
		},
	})
}

func newIAPVersionImageCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions image", flag.ExitOnError)
	versionID := shared.BindResourceIDFlag(fs, "version-id", "inAppPurchaseVersions", "In-app purchase version ID")
	imageFields := fs.String("image-fields", "", "fields[inAppPurchaseImages] (comma-separated)")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "image", ShortUsage: `asc iap versions image --version-id "VERSION_ID"`, ShortHelp: "View the primary image for an IAP version.", LongHelp: "View the primary image for an IAP version.", FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				return d.RequiredFlag("--version-id")
			}
			fields, err := shared.NormalizeSelection(*imageFields, iapVersionImageFields, "--image-fields")
			if err != nil {
				return d.UsageError("iap versions image: " + err.Error())
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("iap versions image: %w", err)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			resp, err := client.GetInAppPurchaseVersionImage(requestCtx, id, asc.WithIAPVersionImageFields(fields))
			if err != nil {
				return fmt.Errorf("iap versions image: failed to fetch: %w", err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func newIAPVersionSubmitCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions submit", flag.ExitOnError)
	versionID := shared.BindResourceIDFlag(fs, "version-id", "inAppPurchaseVersions", "In-app purchase version ID")
	submissionID := shared.BindResourceIDFlag(fs, "submission", "reviewSubmissions", "Review submission ID")
	confirm := fs.Bool("confirm", false, "Confirm adding the version to review")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "submit", ShortUsage: `asc iap versions submit --version-id "VERSION_ID" --submission "SUBMISSION_ID" --confirm`, ShortHelp: "Add an IAP version to a review submission.",
		LongHelp: "Add an IAP version to a review submission. This version-scoped workflow replaces the deprecated product-scoped submission resource.\n\nExamples:\n  asc iap versions submit --version-id \"VERSION_ID\" --submission \"SUBMISSION_ID\" --confirm", FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			vid := strings.TrimSpace(*versionID)
			if vid == "" {
				return d.RequiredFlag("--version-id")
			}
			sid := strings.TrimSpace(*submissionID)
			if sid == "" {
				return d.RequiredFlag("--submission")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("iap versions submit: %w", err)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			resp, err := client.CreateReviewSubmissionItem(requestCtx, sid, asc.ReviewSubmissionItemTypeInAppPurchaseVersion, vid)
			if err != nil {
				return fmt.Errorf("iap versions submit: failed to add review item: %w", err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func newIAPVersionLinksCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions links", flag.ExitOnError)
	return &ffcli.Command{
		Name: "links", ShortUsage: "asc iap versions links <subcommand> [flags]", ShortHelp: "View raw IAP version relationship linkages.", LongHelp: "View raw IAP version relationship linkages.", FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{iapVersionLinkagesCommand(d, "versions", true), iapVersionImageLinkageCommand(d), iapVersionLinkagesCommand(d, "images", false), iapVersionLinkagesCommand(d, "localizations", false)}, Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func iapVersionImageLinkageCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions links image", flag.ExitOnError)
	versionID := shared.BindResourceIDFlag(fs, "version-id", "inAppPurchaseVersions", "In-app purchase version ID")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "image", ShortUsage: `asc iap versions links image --version-id "VERSION_ID"`, ShortHelp: "View the primary image linkage.", LongHelp: "View the primary image linkage.", FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				return d.RequiredFlag("--version-id")
			}
			client, err := d.Client()
			if err != nil {
				return err
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			resp, err := client.GetInAppPurchaseVersionImageRelationship(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap versions links image: failed to fetch: %w", err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func iapVersionLinkagesCommand(d *shared.Deps, name string, parentIAP bool) *ffcli.Command {
	fs := d.NewFlagSet("versions links "+name, flag.ExitOnError)
	idFlag := "version-id"
	idType := "inAppPurchaseVersions"
	idHelp := "In-app purchase version ID"
	if parentIAP {
		idFlag = "iap-id"
		idType = "inAppPurchases"
		idHelp = "In-app purchase ID"
	}
	id := shared.BindResourceIDFlag(fs, idFlag, idType, idHelp)
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: name, ShortUsage: fmt.Sprintf("asc iap versions links %s --%s \"ID\"", name, idFlag), ShortHelp: "View relationship linkages.", LongHelp: "View relationship linkages.", FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "iap versions links "+name, 200, "limit"); err != nil {
				return err
			}
			value := strings.TrimSpace(*id)
			if value == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --%s is required\n", idFlag)
				return shared.MissingRequiredUsageError("--" + idFlag)
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("iap versions links %s: %w", name, err)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			fetchPage := func(pageCtx context.Context, opts ...asc.LinkagesOption) (*asc.LinkagesResponse, error) {
				if parentIAP {
					return client.GetInAppPurchaseVersionsRelationships(pageCtx, value, opts...)
				}
				if name == "images" {
					return client.GetInAppPurchaseVersionImagesRelationships(pageCtx, value, opts...)
				}
				return client.GetInAppPurchaseVersionLocalizationsRelationships(pageCtx, value, opts...)
			}
			firstPageLimit := *limit
			if *paginate && firstPageLimit == 0 {
				firstPageLimit = 200
			}
			resp, err := fetchPage(requestCtx, asc.Limit[asc.LinkagesOption](firstPageLimit), asc.NextURL[asc.LinkagesOption](*next))
			if err != nil {
				return fmt.Errorf("iap versions links %s: failed to fetch: %w", name, err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return fetchPage(ctx, asc.NextURL[asc.LinkagesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap versions links %s: %w", name, err)
				}
				return d.PrintOutput(aggregated, *output.Output, *output.Pretty)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
