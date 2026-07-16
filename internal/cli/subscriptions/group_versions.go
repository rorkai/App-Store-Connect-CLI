package subscriptions

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

var subscriptionGroupVersionStates = []string{
	"PREPARE_FOR_SUBMISSION", "READY_FOR_REVIEW", "WAITING_FOR_REVIEW", "IN_REVIEW",
	"ACCEPTED", "APPROVED", "REPLACED_WITH_NEW_VERSION", "REJECTED", "DEVELOPER_REJECTED",
}

var (
	subscriptionGroupVersionIncludes           = []string{"subscriptionGroup", "localizations"}
	subscriptionGroupVersionFields             = []string{"version", "state", "subscriptionGroup", "localizations"}
	subscriptionGroupVersionGroupFields        = []string{"referenceName", "subscriptions", "subscriptionGroupLocalizations", "versions"}
	subscriptionGroupVersionLocalizationFields = []string{"name", "customAppName", "locale", "version"}
)

func rejectSubscriptionGroupVersionArgs(args []string) error {
	if len(args) == 0 {
		return nil
	}
	return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
}

func subscriptionGroupAnyFlagSet(fs *flag.FlagSet, names ...string) bool {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}
	found := false
	fs.Visit(func(item *flag.Flag) {
		if _, ok := set[item.Name]; ok {
			found = true
		}
	})
	return found
}

// SubscriptionsGroupsVersionsCommand returns the subscription group versions command group.
func SubscriptionsGroupsVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups versions", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc subscriptions groups versions <subcommand> [flags]",
		ShortHelp:  "Manage subscription group review versions.",
		LongHelp: `Manage subscription group review versions.

Examples:
  asc subscriptions groups versions list --group-id "GROUP_ID"
  asc subscriptions groups versions create --group-id "GROUP_ID"
  asc subscriptions groups versions view --version-id "VERSION_ID"`,
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsGroupsVersionsCreateCommand(),
			SubscriptionsGroupsVersionsListCommand(),
			SubscriptionsGroupsVersionsViewCommand(),
			SubscriptionsGroupsVersionLocalizationsCommand(),
			SubscriptionsGroupsVersionLinksCommand(),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

// SubscriptionsGroupsVersionsCreateCommand creates a discrete version for a group.
func SubscriptionsGroupsVersionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups versions create", flag.ExitOnError)
	groupID := fs.String("group-id", "", "Subscription group ID")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "create", ShortUsage: `asc subscriptions groups versions create --group-id "GROUP_ID"`, ShortHelp: "Create a subscription group version.",
		LongHelp: "Create a subscription group version.\n\nExamples:\n  asc subscriptions groups versions create --group-id \"GROUP_ID\"", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectSubscriptionGroupVersionArgs(args); err != nil {
				return err
			}
			id := strings.TrimSpace(*groupID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return shared.MissingRequiredUsageError()
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups versions create: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.CreateSubscriptionGroupVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions groups versions create: failed to create: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func normalizeSubscriptionGroupVersionStates(value string) ([]string, error) {
	values := shared.SplitCSVUpper(value)
	if len(values) == 0 {
		return nil, nil
	}
	allowed := make(map[string]struct{}, len(subscriptionGroupVersionStates))
	for _, state := range subscriptionGroupVersionStates {
		allowed[state] = struct{}{}
	}
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			return nil, fmt.Errorf("--state must be one of: %s", strings.Join(subscriptionGroupVersionStates, ", "))
		}
	}
	return values, nil
}

func subscriptionGroupVersionOptions(stateValue, includeValue, fieldsValue, groupFieldsValue, localizationFieldsValue string, limit, localizationsLimit int, next string) ([]asc.SubscriptionGroupVersionsOption, error) {
	if limit != 0 && (limit < 1 || limit > 200) {
		return nil, fmt.Errorf("--limit must be between 1 and 200")
	}
	if localizationsLimit != 0 && (localizationsLimit < 1 || localizationsLimit > 50) {
		return nil, fmt.Errorf("--localizations-limit must be between 1 and 50")
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return nil, err
	}
	states, err := normalizeSubscriptionGroupVersionStates(stateValue)
	if err != nil {
		return nil, err
	}
	include, err := shared.NormalizeSelection(includeValue, subscriptionGroupVersionIncludes, "--include")
	if err != nil {
		return nil, err
	}
	fields, err := shared.NormalizeSelection(fieldsValue, subscriptionGroupVersionFields, "--fields")
	if err != nil {
		return nil, err
	}
	groupFields, err := shared.NormalizeSelection(groupFieldsValue, subscriptionGroupVersionGroupFields, "--group-fields")
	if err != nil {
		return nil, err
	}
	localizationFields, err := shared.NormalizeSelection(localizationFieldsValue, subscriptionGroupVersionLocalizationFields, "--localization-fields")
	if err != nil {
		return nil, err
	}
	return []asc.SubscriptionGroupVersionsOption{
		asc.WithSubscriptionGroupVersionsStates(states),
		asc.WithSubscriptionGroupVersionsInclude(include),
		asc.WithSubscriptionGroupVersionsFields(fields),
		asc.WithSubscriptionGroupVersionsGroupFields(groupFields),
		asc.WithSubscriptionGroupVersionsLocalizationFields(localizationFields),
		asc.WithSubscriptionGroupVersionsLimit(limit),
		asc.WithSubscriptionGroupVersionsLocalizationsLimit(localizationsLimit),
		asc.WithSubscriptionGroupVersionsNextURL(next),
	}, nil
}

func bindSubscriptionGroupVersionFlags(fs *flag.FlagSet) (state, include, fields, groupFields, localizationFields *string, limit, localizationsLimit *int, next *string, paginate *bool) {
	state = fs.String("state", "", "Filter by state (comma-separated)")
	include = fs.String("include", "", "Include relationships: subscriptionGroup,localizations")
	fields = fs.String("fields", "", "Version fields: version,state,subscriptionGroup,localizations")
	groupFields = fs.String("group-fields", "", "Included group fields (comma-separated)")
	localizationFields = fs.String("localization-fields", "", "Included localization fields (comma-separated)")
	limit = fs.Int("limit", 0, "Maximum results per page (1-200)")
	localizationsLimit = fs.Int("localizations-limit", 0, "Maximum included localizations (1-50)")
	next = fs.String("next", "", "Fetch next page using a links.next URL")
	paginate = fs.Bool("paginate", false, "Automatically fetch all pages")
	return
}

// SubscriptionsGroupsVersionsListCommand lists versions for a group.
func SubscriptionsGroupsVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups versions list", flag.ExitOnError)
	groupID := fs.String("group-id", "", "Subscription group ID")
	state, include, fields, groupFields, localizationFields, limit, localizationsLimit, next, paginate := bindSubscriptionGroupVersionFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortUsage: `asc subscriptions groups versions list --group-id "GROUP_ID" [flags]`, ShortHelp: "List versions for a subscription group.",
		LongHelp: "List versions for a subscription group.\n\nExamples:\n  asc subscriptions groups versions list --group-id \"GROUP_ID\" --state PREPARE_FOR_SUBMISSION --paginate", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectSubscriptionGroupVersionArgs(args); err != nil {
				return err
			}
			id := strings.TrimSpace(*groupID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*next) != "" && subscriptionGroupAnyFlagSet(fs, "state", "include", "fields", "group-fields", "localization-fields", "limit", "localizations-limit") {
				return shared.UsageError("subscriptions groups versions list: --next cannot be combined with query flags")
			}
			opts, err := subscriptionGroupVersionOptions(*state, *include, *fields, *groupFields, *localizationFields, *limit, *localizationsLimit, *next)
			if err != nil {
				return shared.UsageError("subscriptions groups versions list: " + err.Error())
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups versions list: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionGroupVersions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions groups versions list: failed to fetch: %w", err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionGroupVersions(ctx, id, asc.WithSubscriptionGroupVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions groups versions list: %w", err)
				}
				return shared.PrintOutput(aggregated, *output.Output, *output.Pretty)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsGroupsVersionsViewCommand retrieves a group version.
func SubscriptionsGroupsVersionsViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups versions view", flag.ExitOnError)
	versionID := fs.String("version-id", "", "Subscription group version ID")
	include := fs.String("include", "", "Include relationships: subscriptionGroup,localizations")
	fields := fs.String("fields", "", "Version fields: version,state,subscriptionGroup,localizations")
	groupFields := fs.String("group-fields", "", "Included group fields (comma-separated)")
	localizationFields := fs.String("localization-fields", "", "Included localization fields (comma-separated)")
	localizationsLimit := fs.Int("localizations-limit", 0, "Maximum included localizations (1-50)")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "view", ShortUsage: `asc subscriptions groups versions view --version-id "VERSION_ID" [flags]`, ShortHelp: "View a subscription group version.",
		LongHelp: "View a subscription group version.\n\nExamples:\n  asc subscriptions groups versions view --version-id \"VERSION_ID\" --include localizations", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectSubscriptionGroupVersionArgs(args); err != nil {
				return err
			}
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError()
			}
			opts, err := subscriptionGroupVersionOptions("", *include, *fields, *groupFields, *localizationFields, 0, *localizationsLimit, "")
			if err != nil {
				return shared.UsageError("subscriptions groups versions view: " + err.Error())
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups versions view: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionGroupVersion(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions groups versions view: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsGroupsVersionLinksCommand groups raw relationship linkage reads.
func SubscriptionsGroupsVersionLinksCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups versions links", flag.ExitOnError)
	return &ffcli.Command{
		Name: "links", ShortUsage: "asc subscriptions groups versions links <subcommand> [flags]", ShortHelp: "View raw subscription group version linkages.", LongHelp: "View raw subscription group version linkages.",
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			subscriptionsGroupsVersionLinkagesCommand("versions", true),
			subscriptionsGroupsVersionLinkagesCommand("localizations", false),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func subscriptionsGroupsVersionLinkagesCommand(name string, groupOwned bool) *ffcli.Command {
	fs := flag.NewFlagSet("groups versions links "+name, flag.ExitOnError)
	groupID := fs.String("group-id", "", "Subscription group ID")
	versionID := fs.String("version-id", "", "Subscription group version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: name, ShortUsage: "asc subscriptions groups versions links " + name + " [flags]", ShortHelp: "View " + name + " relationship linkages.", LongHelp: "View " + name + " relationship linkages.",
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectSubscriptionGroupVersionArgs(args); err != nil {
				return err
			}
			id := strings.TrimSpace(*versionID)
			requiredFlag := "--version-id"
			if groupOwned {
				id = strings.TrimSpace(*groupID)
				requiredFlag = "--group-id"
			}
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: %s is required\n", requiredFlag)
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*next) != "" && subscriptionGroupAnyFlagSet(fs, "limit") {
				return shared.UsageError("subscriptions groups versions links " + name + ": --next cannot be combined with --limit")
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("subscriptions groups versions links " + name + ": --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageError("subscriptions groups versions links " + name + ": " + err.Error())
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups versions links %s: %w", name, err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			fetch := func(ctx context.Context, nextURL string) (*asc.LinkagesResponse, error) {
				opts := []asc.LinkagesOption{asc.WithLinkagesLimit(*limit), asc.WithLinkagesNextURL(nextURL)}
				if groupOwned {
					return client.GetSubscriptionGroupVersionsRelationships(ctx, id, opts...)
				}
				return client.GetSubscriptionGroupVersionLocalizationsRelationships(ctx, id, opts...)
			}
			resp, err := fetch(requestCtx, *next)
			if err != nil {
				return fmt.Errorf("subscriptions groups versions links %s: failed to fetch: %w", name, err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return fetch(ctx, nextURL)
				})
				if err != nil {
					return fmt.Errorf("subscriptions groups versions links %s: %w", name, err)
				}
				return shared.PrintOutput(aggregated, *output.Output, *output.Pretty)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
