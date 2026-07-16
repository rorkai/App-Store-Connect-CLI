package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
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

// IAPVersionsCommand returns the IAP versions command group.
func IAPVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions", flag.ExitOnError)
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
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPVersionsCreateCommand(), IAPVersionsListCommand(), IAPVersionsViewCommand(),
			IAPVersionImageCommand(), IAPVersionImagesCommand(), IAPVersionLocalizationsCommand(),
			IAPVersionLinksCommand(), IAPVersionSubmitCommand(),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func IAPVersionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions create", flag.ExitOnError)
	iapID := fs.String("iap-id", "", "In-app purchase ID")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "create", ShortUsage: `asc iap versions create --iap-id "IAP_ID"`, ShortHelp: "Create an in-app purchase version.",
		LongHelp: "Create an in-app purchase version.\n\nExamples:\n  asc iap versions create --iap-id \"IAP_ID\"", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, _ []string) error {
			id := strings.TrimSpace(*iapID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return shared.MissingRequiredUsageError()
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap versions create: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.CreateInAppPurchaseVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap versions create: failed to create: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
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
	limit = fs.Int("limit", 0, "Maximum results per page (1-200)")
	imagesLimit = fs.Int("images-limit", 0, "Maximum included images (1-50)")
	localizationsLimit = fs.Int("localizations-limit", 0, "Maximum included localizations (1-50)")
	next = fs.String("next", "", "Fetch next page using a links.next URL")
	paginate = fs.Bool("paginate", false, "Automatically fetch all pages")
	return
}

func iapVersionQueryOptions(stateValue, includeValue string, limit, imagesLimit, localizationsLimit int, next string) ([]asc.IAPVersionsOption, error) {
	if limit != 0 && (limit < 1 || limit > 200) {
		return nil, fmt.Errorf("--limit must be between 1 and 200")
	}
	if imagesLimit != 0 && (imagesLimit < 1 || imagesLimit > 50) {
		return nil, fmt.Errorf("--images-limit must be between 1 and 50")
	}
	if localizationsLimit != 0 && (localizationsLimit < 1 || localizationsLimit > 50) {
		return nil, fmt.Errorf("--localizations-limit must be between 1 and 50")
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return nil, err
	}
	states, err := normalizeIAPVersionStates(stateValue)
	if err != nil {
		return nil, err
	}
	include, err := shared.NormalizeSelection(includeValue, iapVersionIncludes, "--include")
	if err != nil {
		return nil, err
	}
	return []asc.IAPVersionsOption{asc.WithIAPVersionsStates(states), asc.WithIAPVersionsInclude(include), asc.WithIAPVersionsLimit(limit), asc.WithIAPVersionsImagesLimit(imagesLimit), asc.WithIAPVersionsLocalizationsLimit(localizationsLimit), asc.WithIAPVersionsNextURL(next)}, nil
}

func IAPVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions list", flag.ExitOnError)
	iapID := fs.String("iap-id", "", "In-app purchase ID")
	state, include, limit, imagesLimit, localizationsLimit, next, paginate := bindIAPVersionQueryFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortUsage: `asc iap versions list --iap-id "IAP_ID" [flags]`, ShortHelp: "List versions for an in-app purchase.",
		LongHelp: "List versions for an in-app purchase.\n\nExamples:\n  asc iap versions list --iap-id \"IAP_ID\"\n  asc iap versions list --iap-id \"IAP_ID\" --state PREPARE_FOR_SUBMISSION --paginate", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, _ []string) error {
			id := strings.TrimSpace(*iapID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return shared.MissingRequiredUsageError()
			}
			opts, err := iapVersionQueryOptions(*state, *include, *limit, *imagesLimit, *localizationsLimit, *next)
			if err != nil {
				return shared.UsageError("iap versions list: " + err.Error())
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap versions list: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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
				return shared.PrintOutput(aggregated, *output.Output, *output.Pretty)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func IAPVersionsViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions view", flag.ExitOnError)
	versionID := fs.String("version-id", "", "In-app purchase version ID")
	include := fs.String("include", "", "Include relationships: inAppPurchase,image,images,localizations")
	imagesLimit := fs.Int("images-limit", 0, "Maximum included images (1-50)")
	localizationsLimit := fs.Int("localizations-limit", 0, "Maximum included localizations (1-50)")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "view", ShortUsage: `asc iap versions view --version-id "VERSION_ID" [flags]`, ShortHelp: "View an in-app purchase version.",
		LongHelp: "View an in-app purchase version.\n\nExamples:\n  asc iap versions view --version-id \"VERSION_ID\" --include localizations,images", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, _ []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError()
			}
			opts, err := iapVersionQueryOptions("", *include, 0, *imagesLimit, *localizationsLimit, "")
			if err != nil {
				return shared.UsageError("iap versions view: " + err.Error())
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap versions view: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetInAppPurchaseVersion(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap versions view: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func IAPVersionImageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions image", flag.ExitOnError)
	versionID := fs.String("version-id", "", "In-app purchase version ID")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "image", ShortUsage: `asc iap versions image --version-id "VERSION_ID"`, ShortHelp: "View the primary image for an IAP version.", LongHelp: "View the primary image for an IAP version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, _ []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError()
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap versions image: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetInAppPurchaseVersionImage(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap versions image: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func IAPVersionSubmitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions submit", flag.ExitOnError)
	versionID := fs.String("version-id", "", "In-app purchase version ID")
	submissionID := fs.String("submission", "", "Review submission ID")
	confirm := fs.Bool("confirm", false, "Confirm adding the version to review")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "submit", ShortUsage: `asc iap versions submit --version-id "VERSION_ID" --submission "SUBMISSION_ID" --confirm`, ShortHelp: "Add an IAP version to a review submission.",
		LongHelp: "Add an IAP version to a review submission. The legacy `asc iap submit --iap-id` command remains unchanged.\n\nExamples:\n  asc iap versions submit --version-id \"VERSION_ID\" --submission \"SUBMISSION_ID\" --confirm", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, _ []string) error {
			vid := strings.TrimSpace(*versionID)
			if vid == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError()
			}
			sid := strings.TrimSpace(*submissionID)
			if sid == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap versions submit: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.CreateReviewSubmissionItem(requestCtx, sid, asc.ReviewSubmissionItemTypeInAppPurchaseVersion, vid)
			if err != nil {
				return fmt.Errorf("iap versions submit: failed to add review item: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func IAPVersionLinksCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions links", flag.ExitOnError)
	return &ffcli.Command{
		Name: "links", ShortUsage: "asc iap versions links <subcommand> [flags]", ShortHelp: "View raw IAP version relationship linkages.", LongHelp: "View raw IAP version relationship linkages.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{iapVersionLinkagesCommand("versions", true), iapVersionImageLinkageCommand(), iapVersionLinkagesCommand("images", false), iapVersionLinkagesCommand("localizations", false)}, Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func iapVersionImageLinkageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions links image", flag.ExitOnError)
	versionID := fs.String("version-id", "", "In-app purchase version ID")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "image", ShortUsage: `asc iap versions links image --version-id "VERSION_ID"`, ShortHelp: "View the primary image linkage.", LongHelp: "View the primary image linkage.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, _ []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError()
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return err
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetInAppPurchaseVersionImageRelationship(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap versions links image: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func iapVersionLinkagesCommand(name string, parentIAP bool) *ffcli.Command {
	fs := flag.NewFlagSet("versions links "+name, flag.ExitOnError)
	idFlag := "version-id"
	idHelp := "In-app purchase version ID"
	if parentIAP {
		idFlag = "iap-id"
		idHelp = "In-app purchase ID"
	}
	id := fs.String(idFlag, "", idHelp)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: name, ShortUsage: fmt.Sprintf("asc iap versions links %s --%s \"ID\"", name, idFlag), ShortHelp: "View relationship linkages.", LongHelp: "View relationship linkages.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, _ []string) error {
			value := strings.TrimSpace(*id)
			if value == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --%s is required\n", idFlag)
				return shared.MissingRequiredUsageError()
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError(fmt.Sprintf("iap versions links %s: --limit must be between 1 and 200", name))
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageError(fmt.Sprintf("iap versions links %s: %v", name, err))
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap versions links %s: %w", name, err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			opts := []asc.LinkagesOption{asc.WithLinkagesLimit(*limit), asc.WithLinkagesNextURL(*next)}
			var resp *asc.LinkagesResponse
			if parentIAP {
				resp, err = client.GetInAppPurchaseVersionsRelationships(requestCtx, value, opts...)
			} else if name == "images" {
				resp, err = client.GetInAppPurchaseVersionImagesRelationships(requestCtx, value, opts...)
			} else {
				resp, err = client.GetInAppPurchaseVersionLocalizationsRelationships(requestCtx, value, opts...)
			}
			if err != nil {
				return fmt.Errorf("iap versions links %s: failed to fetch: %w", name, err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
