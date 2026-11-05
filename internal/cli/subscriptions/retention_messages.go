package subscriptions

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const retentionMessagesCommandPrefix = "subscriptions retention-messages"

type retentionRelationshipLimit struct{ flag, param string }

type retentionFilter struct {
	flag, param, usage string
	allowed            []string
}

type retentionFetcher func(*asc.Client, context.Context, string, ...asc.RetentionMessagesOption) (any, error)

func retentionFetch[T any](fn func(*asc.Client, context.Context, string, ...asc.RetentionMessagesOption) (*T, error)) retentionFetcher {
	return func(client *asc.Client, ctx context.Context, id string, opts ...asc.RetentionMessagesOption) (any, error) {
		return fn(client, ctx, id, opts...)
	}
}

// retentionRead describes one retention messaging GET command.
type retentionRead struct {
	path, help                       string
	examples                         []string
	ownerFlag, ownerType, ownerUsage string
	primary                          string
	includes                         map[string]string
	limits                           []retentionRelationshipLimit
	filters                          []retentionFilter
	list                             bool
	fetch                            retentionFetcher
	print                            func(any, shared.OutputFlags) error
}

var (
	retentionMessageIncludes = map[string]string{"versions": "retentionMessageVersions", "subscriptions": "subscriptions"}
	retentionMessageLimits   = []retentionRelationshipLimit{{"versions-limit", "limit[versions]"}, {"subscriptions-limit", "limit[subscriptions]"}}
	retentionVersionIncludes = map[string]string{"retentionMessage": "retentionMessages", "localizations": "retentionMessageLocalizations", "placement": "appAssetLibraryPlacements"}
	retentionVersionLimits   = []retentionRelationshipLimit{{"localizations-limit", "limit[localizations]"}}
	retentionLocaleIncludes  = map[string]string{"retentionMessageVersion": "retentionMessageVersions"}
)

func retentionReadCommand(r retentionRead) *ffcli.Command {
	command := retentionMessagesCommandPrefix + " " + r.path
	fs := flag.NewFlagSet(r.path, flag.ExitOnError)
	ownerID := shared.BindResourceIDFlag(fs, r.ownerFlag, r.ownerType, r.ownerUsage)
	var fields, include, next *string
	if r.primary != "" {
		fields = fs.String("fields", "", "Fields for "+r.primary+": "+strings.Join(shared.IncludedResourceFields(r.primary), ", "))
	}
	includeNames := slices.Sorted(maps.Keys(r.includes))
	var included *shared.IncludedFieldFlags
	if len(includeNames) > 0 {
		include = fs.String("include", "", "Include relationships: "+strings.Join(includeNames, ", "))
		included = shared.BindIncludedFieldFlags(fs, r.includes)
	}
	limits := make([]*int, len(r.limits))
	for i, limit := range r.limits {
		limits[i] = fs.Int(limit.flag, 0, "Maximum included "+strings.TrimSuffix(limit.flag, "-limit")+" (1-50)")
	}
	filters := make([]*string, len(r.filters))
	for i, filter := range r.filters {
		filters[i] = fs.String(filter.flag, "", filter.usage)
	}
	var limit *int
	var paginate *bool
	if r.list {
		limit = fs.Int("limit", 0, "Maximum results per page (1-200)")
		next = fs.String("next", "", "Fetch next page using a links.next URL")
		paginate = fs.Bool("paginate", false, "Automatically fetch all pages")
	}
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       r.path[strings.LastIndex(r.path, " ")+1:],
		ShortUsage: "asc " + command + " --" + r.ownerFlag + " ID [flags]",
		ShortHelp:  r.help,
		LongHelp:   r.help + "\n\nExamples:\n  " + strings.Join(r.examples, "\n  "),
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.RejectPositionalArgs(args); err != nil {
				return err
			}
			id, nextURL := strings.TrimSpace(*ownerID), ""
			if next != nil {
				nextURL = strings.TrimSpace(*next)
			}
			if err := shared.ValidateNextURL(nextURL); err != nil {
				return shared.UsageErrorf("%s: %v", command, err)
			}
			conflicts := []string{r.ownerFlag, "fields", "include", "limit"}
			for _, l := range r.limits {
				conflicts = append(conflicts, l.flag)
			}
			for _, f := range r.filters {
				conflicts = append(conflicts, f.flag)
			}
			if err := shared.RejectNextFlagConflicts(fs, nextURL, command, conflicts...); err != nil {
				return err
			}
			if r.ownerType == "apps" && nextURL == "" {
				id = shared.ResolveAppID(id)
			}
			if id == "" && nextURL == "" {
				fmt.Fprintf(os.Stderr, "Error: --%s is required\n", r.ownerFlag)
				return shared.MissingRequiredUsageError("--" + r.ownerFlag)
			}
			opts := []asc.RetentionMessagesOption{asc.WithRetentionMessagesNextURL(nextURL)}
			if fields != nil {
				values, err := normalizeSelectionFlag(fs, *fields, "--fields", shared.IncludedResourceFields(r.primary))
				if err != nil {
					return err
				}
				opts = append(opts, asc.WithRetentionMessagesQuery("fields["+r.primary+"]", values...))
			}
			if include != nil {
				includes, err := normalizeSelectionFlag(fs, *include, "--include", includeNames)
				if err != nil {
					return err
				}
				resolved, err := included.Resolve(command, nextURL, includes)
				if err != nil {
					return err
				}
				opts = append(opts, asc.WithRetentionMessagesQuery("include", includes...))
				for resourceType, values := range resolved {
					opts = append(opts, asc.WithRetentionMessagesQuery("fields["+resourceType+"]", values...))
				}
			}
			for i, l := range r.limits {
				if value := *limits[i]; value != 0 {
					if value < 1 || value > 50 {
						return shared.UsageErrorf("%s: --%s must be between 1 and 50", command, l.flag)
					}
					opts = append(opts, asc.WithRetentionMessagesQuery(l.param, strconv.Itoa(value)))
				}
			}
			for i, f := range r.filters {
				values := shared.SplitCSV(*filters[i])
				if f.allowed != nil {
					var err error
					if values, err = normalizeSelectionFlag(fs, *filters[i], "--"+f.flag, f.allowed); err != nil {
						return err
					}
				}
				opts = append(opts, asc.WithRetentionMessagesQuery(f.param, values...))
			}
			if limit != nil {
				if *limit != 0 && (*limit < 1 || *limit > 200) {
					return shared.UsageErrorf("%s: --limit must be between 1 and 200", command)
				}
				pageLimit := *limit
				if *paginate && pageLimit == 0 {
					pageLimit = 200
				}
				if pageLimit > 0 {
					opts = append(opts, asc.WithRetentionMessagesQuery("limit", strconv.Itoa(pageLimit)))
				}
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := r.fetch(client, requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("%s: failed to fetch: %w", command, err)
			}
			if paginate != nil && *paginate {
				resp, err = asc.PaginateAll(requestCtx, resp.(asc.PaginatedResponse), func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					page, err := r.fetch(client, ctx, id, asc.WithRetentionMessagesNextURL(nextURL))
					if err != nil {
						return nil, err
					}
					return page.(asc.PaginatedResponse), nil
				})
				if err != nil {
					return fmt.Errorf("%s: %w", command, err)
				}
			}
			if r.print != nil {
				return r.print(resp, output)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func retentionGroupCommand(path, help string, subcommands ...*ffcli.Command) *ffcli.Command {
	return &ffcli.Command{
		Name:        path[strings.LastIndex(path, " ")+1:],
		ShortUsage:  "asc " + strings.TrimSpace(retentionMessagesCommandPrefix+" "+path) + " <subcommand> [flags]",
		ShortHelp:   help,
		LongHelp:    help,
		FlagSet:     flag.NewFlagSet(path, flag.ExitOnError),
		UsageFunc:   shared.DefaultUsageFunc,
		Subcommands: subcommands,
		Exec:        func(context.Context, []string) error { return flag.ErrHelp },
	}
}

// retentionWriteCommand builds a mutation command; run validates flags and returns
// the call to make once an authenticated client exists.
func retentionWriteCommand(path, usage, help string, examples []string, bind func(*flag.FlagSet) func() (func(context.Context, *asc.Client) (any, error), error)) *ffcli.Command {
	command := retentionMessagesCommandPrefix + " " + path
	fs := flag.NewFlagSet(path, flag.ExitOnError)
	run := bind(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       path[strings.LastIndex(path, " ")+1:],
		ShortUsage: "asc " + command + " " + usage,
		ShortHelp:  strings.SplitN(help, "\n", 2)[0],
		LongHelp:   help + "\n\nExamples:\n  " + strings.Join(examples, "\n  "),
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.RejectPositionalArgs(args); err != nil {
				return err
			}
			call, err := run()
			if err != nil {
				return err
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			result, err := call(requestCtx, client)
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func requiredRetentionValue(value *string, name string) (string, error) {
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		fmt.Fprintf(os.Stderr, "Error: --%s is required\n", name)
		return "", shared.MissingRequiredUsageError("--" + name)
	}
	return trimmed, nil
}

// optionalRetentionText returns nil when the flag was not provided and enforces
// Apple's documented character limit when it was.
func optionalRetentionText(fs *flag.FlagSet, value *string, name string, maxLength int) (*string, error) {
	if !flagWasProvided(fs, name) {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, shared.UsageErrorf("--%s must not be empty", name)
	}
	if utf8.RuneCountInString(trimmed) > maxLength {
		return nil, shared.UsageErrorf("--%s must be at most %d characters", name, maxLength)
	}
	return &trimmed, nil
}

// SubscriptionsRetentionMessagesCommand returns the retention messages command group.
func SubscriptionsRetentionMessagesCommand() *ffcli.Command {
	group := retentionGroupCommand("", "Manage retention messages shown when people cancel a subscription.",
		retentionReadCommand(retentionRead{
			path: "list", help: "List retention messages for an app.", list: true,
			examples:  []string{`asc subscriptions retention-messages list --app "APP_ID"`, `asc subscriptions retention-messages list --app "APP_ID" --include versions --paginate`},
			ownerFlag: "app", ownerType: "apps", ownerUsage: "App Store Connect app ID (or ASC_APP_ID env)",
			primary: "retentionMessages", includes: retentionMessageIncludes, limits: retentionMessageLimits,
			fetch: retentionFetch((*asc.Client).GetAppRetentionMessages),
		}),
		retentionReadCommand(retentionRead{
			path: "view", help: "View a retention message.",
			examples:  []string{`asc subscriptions retention-messages view --id "MESSAGE_ID" --include versions`},
			ownerFlag: "id", ownerType: "retentionMessages", ownerUsage: "Retention message ID",
			primary: "retentionMessages", includes: retentionMessageIncludes, limits: retentionMessageLimits,
			fetch: retentionFetch((*asc.Client).GetRetentionMessage),
		}),
		retentionMessagesCreateCommand(),
		retentionMessagesDeleteCommand(),
		retentionReadCommand(retentionRead{
			path: "assigned", help: "View the retention message assigned to a subscription.",
			examples:  []string{`asc subscriptions retention-messages assigned --subscription-id "SUB_ID"`},
			ownerFlag: "subscription-id", ownerType: "subscriptions", ownerUsage: "Subscription ID",
			primary: "retentionMessages", includes: retentionMessageIncludes, limits: retentionMessageLimits,
			fetch: retentionFetch((*asc.Client).GetSubscriptionRetentionMessage),
			print: func(resp any, output shared.OutputFlags) error {
				raw := *resp.(*json.RawMessage)
				headers, rows, err := asc.RetentionMessageResourceRows(raw)
				if err != nil {
					return err
				}
				return shared.PrintOutputRows(raw, *output.Output, *output.Pretty, headers, rows)
			},
		}),
		retentionMessagesAssignCommand(),
		retentionGroupCommand("subscriptions", "List subscriptions that use a retention message.",
			retentionReadCommand(retentionRead{
				path: "subscriptions list", help: "List subscriptions assigned to a retention message.", list: true,
				examples:  []string{`asc subscriptions retention-messages subscriptions list --message-id "MESSAGE_ID" --state APPROVED`},
				ownerFlag: "message-id", ownerType: "retentionMessages", ownerUsage: "Retention message ID",
				primary: "subscriptions",
				filters: []retentionFilter{
					{"product-id", "filter[productId]", "Filter by product IDs (comma-separated)", nil},
					{"name", "filter[name]", "Filter by subscription names (comma-separated)", nil},
					{"state", "filter[state]", "Filter by states (comma-separated)", subscriptionStates},
					{"sort", "sort", "Sort order: name, -name", []string{"name", "-name"}},
				},
				fetch: retentionFetch((*asc.Client).GetRetentionMessageSubscriptions),
			}),
		),
		retentionMessageVersionsCommand(),
		retentionMessageLocalizationsCommand(),
		retentionMessageLinksCommand(),
	)
	group.Name = "retention-messages"
	group.LongHelp = `Manage retention messages shown when people cancel a subscription.

A message holds its content in versions. Create a version, add localized text,
optionally place an asset library image, assign the message to a subscription,
then submit the version for review:
  asc review items add --submission "SUBMISSION_ID" --item-type retentionMessageVersions --item-id "VERSION_ID"
Apple only accepts that submission when the app already has an approved App
Store version, or when the submission also includes an App Store version.

An app can have at most 10 retention messages, and a subscription has at most
one. To choose messages in real time from your server, use
asc storekit retention-messaging.

Examples:
  asc subscriptions retention-messages create --app "APP_ID" --reference-name "winter-stay"
  asc subscriptions retention-messages versions create --message-id "MESSAGE_ID"
  asc subscriptions retention-messages localizations create --version-id "VERSION_ID" --locale en-US --header "Stay with us" --body "Keep your plan."
  asc subscriptions retention-messages assign --subscription-id "SUB_ID" --message-id "MESSAGE_ID"`
	return group
}

var subscriptionStates = []string{"MISSING_METADATA", "READY_TO_SUBMIT", "WAITING_FOR_REVIEW", "IN_REVIEW", "DEVELOPER_ACTION_NEEDED", "PENDING_BINARY_APPROVAL", "APPROVED", "DEVELOPER_REMOVED_FROM_SALE", "REMOVED_FROM_SALE", "REJECTED"}

func retentionMessagesCreateCommand() *ffcli.Command {
	return retentionWriteCommand("create", `--app "APP_ID" --reference-name NAME`,
		"Create a retention message for an app.\n\nThe reference name (up to 64 characters) isn't shown to people and can't be changed later.",
		[]string{`asc subscriptions retention-messages create --app "APP_ID" --reference-name "winter-stay"`},
		func(fs *flag.FlagSet) func() (func(context.Context, *asc.Client) (any, error), error) {
			appID := shared.BindResourceIDFlag(fs, "app", "apps", "App Store Connect app ID (or ASC_APP_ID env)")
			referenceName := fs.String("reference-name", "", "Reference name (up to 64 characters)")
			return func() (func(context.Context, *asc.Client) (any, error), error) {
				*appID = shared.ResolveAppID(*appID)
				app, err := requiredRetentionValue(appID, "app")
				if err != nil {
					return nil, err
				}
				if _, err := requiredRetentionValue(referenceName, "reference-name"); err != nil {
					return nil, err
				}
				name, err := optionalRetentionText(fs, referenceName, "reference-name", 64)
				if err != nil {
					return nil, err
				}
				return func(ctx context.Context, client *asc.Client) (any, error) {
					return client.CreateRetentionMessage(ctx, app, *name)
				}, nil
			}
		})
}

func retentionDeleteCommand(path, ownerType, label string, del func(*asc.Client, context.Context, string) error) *ffcli.Command {
	return retentionWriteCommand(path, `--id "ID" --confirm`, "Delete a "+label+".",
		[]string{"asc " + retentionMessagesCommandPrefix + " " + path + ` --id "ID" --confirm`},
		func(fs *flag.FlagSet) func() (func(context.Context, *asc.Client) (any, error), error) {
			id := shared.BindResourceIDFlag(fs, "id", ownerType, strings.ToUpper(label[:1])+label[1:]+" ID")
			confirm := fs.Bool("confirm", false, "Confirm deletion")
			return func() (func(context.Context, *asc.Client) (any, error), error) {
				value, err := requiredRetentionValue(id, "id")
				if err != nil {
					return nil, err
				}
				if !*confirm {
					fmt.Fprintln(os.Stderr, "Error: --confirm is required")
					return nil, shared.MissingRequiredUsageError("--confirm")
				}
				return func(ctx context.Context, client *asc.Client) (any, error) {
					if err := del(client, ctx, value); err != nil {
						return nil, err
					}
					return &asc.AssetDeleteResult{ID: value, Deleted: true}, nil
				}, nil
			}
		})
}

func retentionMessagesDeleteCommand() *ffcli.Command {
	return retentionDeleteCommand("delete", "retentionMessages", "retention message", (*asc.Client).DeleteRetentionMessage)
}

func retentionMessagesAssignCommand() *ffcli.Command {
	return retentionWriteCommand("assign", `--subscription-id "SUB_ID" --message-id "MESSAGE_ID"`,
		"Assign a retention message to a subscription.\n\nA subscription has at most one retention message; assigning one replaces the current message.\nApple has no unassign request; deleting the message clears the assignment.\nAssign a message before presenting retention offers with it.",
		[]string{`asc subscriptions retention-messages assign --subscription-id "SUB_ID" --message-id "MESSAGE_ID"`},
		func(fs *flag.FlagSet) func() (func(context.Context, *asc.Client) (any, error), error) {
			subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID")
			messageID := shared.BindResourceIDFlag(fs, "message-id", "retentionMessages", "Retention message ID")
			return func() (func(context.Context, *asc.Client) (any, error), error) {
				subscription, err := requiredRetentionValue(subscriptionID, "subscription-id")
				if err != nil {
					return nil, err
				}
				message, err := requiredRetentionValue(messageID, "message-id")
				if err != nil {
					return nil, err
				}
				return func(ctx context.Context, client *asc.Client) (any, error) {
					if err := client.SetSubscriptionRetentionMessage(ctx, subscription, message); err != nil {
						return nil, err
					}
					return &asc.RetentionMessageAssignResult{SubscriptionID: subscription, RetentionMessageID: message, Assigned: true}, nil
				}, nil
			}
		})
}

func retentionMessageVersionsCommand() *ffcli.Command {
	return retentionGroupCommand("versions", "Manage retention message versions.",
		retentionReadCommand(retentionRead{
			path: "versions list", help: "List versions of a retention message.", list: true,
			examples:  []string{`asc subscriptions retention-messages versions list --message-id "MESSAGE_ID" --include localizations`},
			ownerFlag: "message-id", ownerType: "retentionMessages", ownerUsage: "Retention message ID",
			primary: "retentionMessageVersions", includes: retentionVersionIncludes, limits: retentionVersionLimits,
			fetch: retentionFetch((*asc.Client).GetRetentionMessageVersions),
		}),
		retentionReadCommand(retentionRead{
			path: "versions view", help: "View a retention message version and its review state.",
			examples:  []string{`asc subscriptions retention-messages versions view --id "VERSION_ID" --include localizations,placement`},
			ownerFlag: "id", ownerType: "retentionMessageVersions", ownerUsage: "Retention message version ID",
			primary: "retentionMessageVersions", includes: retentionVersionIncludes, limits: retentionVersionLimits,
			fetch: retentionFetch((*asc.Client).GetRetentionMessageVersion),
		}),
		retentionWriteCommand("versions create", `--message-id "MESSAGE_ID" [--placement-id "PLACEMENT_ID"]`,
			"Create a version of a retention message.\n\nA version starts in PREPARE_FOR_SUBMISSION and can't be modified or deleted after creation.\nTo change approved content, create a new version. Omit --placement-id for a text-only\nmessage, or add an image later with versions placement create.",
			[]string{`asc subscriptions retention-messages versions create --message-id "MESSAGE_ID"`, `asc subscriptions retention-messages versions create --message-id "MESSAGE_ID" --placement-id "PLACEMENT_ID"`},
			func(fs *flag.FlagSet) func() (func(context.Context, *asc.Client) (any, error), error) {
				messageID := shared.BindResourceIDFlag(fs, "message-id", "retentionMessages", "Retention message ID")
				placementID := shared.BindResourceIDFlag(fs, "placement-id", "appAssetLibraryPlacements", "Existing asset library placement ID (optional)")
				return func() (func(context.Context, *asc.Client) (any, error), error) {
					message, err := requiredRetentionValue(messageID, "message-id")
					if err != nil {
						return nil, err
					}
					return func(ctx context.Context, client *asc.Client) (any, error) {
						return client.CreateRetentionMessageVersion(ctx, message, *placementID)
					}, nil
				}
			}),
		retentionGroupCommand("versions placement", "View or create the image placement of a retention message version.",
			retentionReadCommand(retentionRead{
				path: "versions placement view", help: "View the asset library placement of a retention message version.",
				examples:  []string{`asc subscriptions retention-messages versions placement view --version-id "VERSION_ID" --include image`},
				ownerFlag: "version-id", ownerType: "retentionMessageVersions", ownerUsage: "Retention message version ID",
				primary:  "appAssetLibraryPlacements",
				includes: map[string]string{"image": "appAssetLibraryImages", "video": "appAssetLibraryVideos", "retentionMessageVersion": "retentionMessageVersions"},
				fetch:    retentionFetch((*asc.Client).GetRetentionMessageVersionPlacement),
				print: func(resp any, output shared.OutputFlags) error {
					raw := *resp.(*json.RawMessage)
					headers, rows, err := asc.CreativePlacementResourceRows(raw)
					if err != nil {
						return err
					}
					return shared.PrintOutputRows(raw, *output.Output, *output.Pretty, headers, rows)
				},
			}),
			retentionWriteCommand("versions placement create", `--version-id "VERSION_ID" --image-id "IMAGE_ID"`,
				"Place an asset library image on a retention message version.\n\nRetention messages accept images only, with placement type RETENTION_MESSAGE_ASSET\nin the DEFAULT_PROFILE group, at most one image per version. Apple's current spec\nis 3840x2160 (PNG or JPEG) or 5244x2950 (PNG), no alpha channel; check it with\n  asc asset-library specs --placement-type RETENTION_MESSAGE_ASSET --feature RETENTION_MESSAGING\nUpload the image with \"asc asset-library images upload\" first; Apple can take\nlonger than the default upload wait to process it, so raise ASC_UPLOAD_TIMEOUT\nor check the image state with \"asc asset-library images view\" before placing it.\nThis does not submit the version for review.",
				[]string{`asc subscriptions retention-messages versions placement create --version-id "VERSION_ID" --image-id "IMAGE_ID"`},
				func(fs *flag.FlagSet) func() (func(context.Context, *asc.Client) (any, error), error) {
					versionID := shared.BindResourceIDFlag(fs, "version-id", "retentionMessageVersions", "Retention message version ID")
					imageID := shared.BindResourceIDFlag(fs, "image-id", "appAssetLibraryImages", "Uploaded asset library image ID")
					return func() (func(context.Context, *asc.Client) (any, error), error) {
						version, err := requiredRetentionValue(versionID, "version-id")
						if err != nil {
							return nil, err
						}
						image, err := requiredRetentionValue(imageID, "image-id")
						if err != nil {
							return nil, err
						}
						return func(ctx context.Context, client *asc.Client) (any, error) {
							const placementType, placementGroup = "RETENTION_MESSAGE_ASSET", "DEFAULT_PROFILE"
							id, state, err := shared.CreateLibraryPlacement(ctx, client, "retentionMessageVersions", version, "image", image, placementType, placementGroup)
							if err != nil {
								return nil, err
							}
							return &asc.RetentionMessagePlacementCreateResult{ID: id, Created: true, RetentionMessageVersionID: version, ImageID: image, PlacementType: placementType, PlacementGroup: placementGroup, State: state}, nil
						}, nil
					}
				}),
		),
	)
}

func retentionMessageLocalizationsCommand() *ffcli.Command {
	return retentionGroupCommand("localizations", "Manage localized retention message text.",
		retentionReadCommand(retentionRead{
			path: "localizations list", help: "List localizations of a retention message version.", list: true,
			examples:  []string{`asc subscriptions retention-messages localizations list --version-id "VERSION_ID"`},
			ownerFlag: "version-id", ownerType: "retentionMessageVersions", ownerUsage: "Retention message version ID",
			primary: "retentionMessageLocalizations", includes: retentionLocaleIncludes,
			fetch: retentionFetch((*asc.Client).GetRetentionMessageVersionLocalizations),
		}),
		retentionReadCommand(retentionRead{
			path: "localizations view", help: "View a retention message localization.",
			examples:  []string{`asc subscriptions retention-messages localizations view --id "LOCALIZATION_ID"`},
			ownerFlag: "id", ownerType: "retentionMessageLocalizations", ownerUsage: "Retention message localization ID",
			primary: "retentionMessageLocalizations", includes: retentionLocaleIncludes,
			fetch: retentionFetch((*asc.Client).GetRetentionMessageLocalization),
		}),
		retentionWriteCommand("localizations create", `--version-id "VERSION_ID" --locale LOCALE [--header TEXT] [--body TEXT]`,
			"Create a localization for a retention message version.\n\nThe header can contain up to 66 characters and the body up to 144. Apple rejects\nnew localizations on an APPROVED version.",
			[]string{`asc subscriptions retention-messages localizations create --version-id "VERSION_ID" --locale en-US --header "Your cortado is waiting" --body "Stay subscribed and keep your daily cortado coming."`},
			func(fs *flag.FlagSet) func() (func(context.Context, *asc.Client) (any, error), error) {
				versionID := shared.BindResourceIDFlag(fs, "version-id", "retentionMessageVersions", "Retention message version ID")
				locale := fs.String("locale", "", "Locale (for example, en-US)")
				header := fs.String("header", "", "Header text (up to 66 characters)")
				body := fs.String("body", "", "Body text (up to 144 characters)")
				return func() (func(context.Context, *asc.Client) (any, error), error) {
					version, err := requiredRetentionValue(versionID, "version-id")
					if err != nil {
						return nil, err
					}
					localeValue, err := requiredRetentionValue(locale, "locale")
					if err != nil {
						return nil, err
					}
					attrs := asc.RetentionMessageLocalizationCreateAttributes{Locale: localeValue}
					if attrs.Header, err = optionalRetentionText(fs, header, "header", 66); err != nil {
						return nil, err
					}
					if attrs.Body, err = optionalRetentionText(fs, body, "body", 144); err != nil {
						return nil, err
					}
					return func(ctx context.Context, client *asc.Client) (any, error) {
						return client.CreateRetentionMessageLocalization(ctx, version, attrs)
					}, nil
				}
			}),
		retentionWriteCommand("localizations update", `--id "LOCALIZATION_ID" [--header TEXT | --clear-header] [--body TEXT | --clear-body]`,
			"Update the header or body of a retention message localization.\n\nThe locale can't be changed; delete the localization and create a new one instead.",
			[]string{`asc subscriptions retention-messages localizations update --id "LOCALIZATION_ID" --body "Stay subscribed for one more month."`, `asc subscriptions retention-messages localizations update --id "LOCALIZATION_ID" --clear-header`},
			func(fs *flag.FlagSet) func() (func(context.Context, *asc.Client) (any, error), error) {
				id := shared.BindResourceIDFlag(fs, "id", "retentionMessageLocalizations", "Retention message localization ID")
				header := fs.String("header", "", "Header text (up to 66 characters)")
				body := fs.String("body", "", "Body text (up to 144 characters)")
				clearHeader := fs.Bool("clear-header", false, "Set the header to null")
				clearBody := fs.Bool("clear-body", false, "Set the body to null")
				nullable := func(value *string, name string, maxLength int, clear bool) (*asc.NullableString, error) {
					text, err := optionalRetentionText(fs, value, name, maxLength)
					switch {
					case err != nil:
						return nil, err
					case text != nil && clear:
						return nil, shared.UsageErrorf("--%s cannot be used with --clear-%s", name, name)
					case clear:
						return &asc.NullableString{}, nil
					case text != nil:
						return &asc.NullableString{Value: text}, nil
					}
					return nil, nil
				}
				return func() (func(context.Context, *asc.Client) (any, error), error) {
					value, err := requiredRetentionValue(id, "id")
					if err != nil {
						return nil, err
					}
					var attrs asc.RetentionMessageLocalizationUpdateAttributes
					if attrs.Header, err = nullable(header, "header", 66, *clearHeader); err != nil {
						return nil, err
					}
					if attrs.Body, err = nullable(body, "body", 144, *clearBody); err != nil {
						return nil, err
					}
					if attrs.Header == nil && attrs.Body == nil {
						fmt.Fprintln(os.Stderr, "Error: --header, --body, --clear-header, or --clear-body is required")
						return nil, shared.MissingRequiredUsageError("--header", "--body")
					}
					return func(ctx context.Context, client *asc.Client) (any, error) {
						return client.UpdateRetentionMessageLocalization(ctx, value, attrs)
					}, nil
				}
			}),
		retentionDeleteCommand("localizations delete", "retentionMessageLocalizations", "retention message localization", (*asc.Client).DeleteRetentionMessageLocalization),
	)
}

func retentionMessageLinksCommand() *ffcli.Command {
	link := func(name, help, ownerFlag, ownerType, ownerUsage string, list bool, fetch retentionFetcher) *ffcli.Command {
		return retentionReadCommand(retentionRead{
			path: "links " + name, help: help, list: list,
			examples:  []string{"asc " + retentionMessagesCommandPrefix + " links " + name + " --" + ownerFlag + ` "ID"`},
			ownerFlag: ownerFlag, ownerType: ownerType, ownerUsage: ownerUsage, fetch: fetch,
		})
	}
	return retentionGroupCommand("links", "View raw retention messaging relationship linkages.",
		link("messages", "View retention message linkages for an app.", "app", "apps", "App Store Connect app ID (or ASC_APP_ID env)", true, retentionFetch((*asc.Client).GetAppRetentionMessagesRelationships)),
		link("versions", "View version linkages for a retention message.", "message-id", "retentionMessages", "Retention message ID", true, retentionFetch((*asc.Client).GetRetentionMessageVersionsRelationships)),
		link("subscriptions", "View subscription linkages for a retention message.", "message-id", "retentionMessages", "Retention message ID", true, retentionFetch((*asc.Client).GetRetentionMessageSubscriptionsRelationships)),
		link("localizations", "View localization linkages for a retention message version.", "version-id", "retentionMessageVersions", "Retention message version ID", true, retentionFetch((*asc.Client).GetRetentionMessageVersionLocalizationsRelationships)),
		link("placement", "View the placement linkage for a retention message version.", "version-id", "retentionMessageVersions", "Retention message version ID", false, retentionFetch((*asc.Client).GetRetentionMessageVersionPlacementRelationship)),
		link("assigned", "View the retention message linkage for a subscription.", "subscription-id", "subscriptions", "Subscription ID", false, retentionFetch((*asc.Client).GetSubscriptionRetentionMessageRelationship)),
	)
}
