package storekit

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	storekitapi "github.com/rudrankriyam/App-Store-Connect-CLI/internal/storekit"
)

const unverifiedJWSHelp = `Apple signs transactions, renewal info, and notification payloads as JWS.
JSON output prints Apple's response unmodified. --decode adds a sibling
"<field>Decoded" object for each signed field; table and markdown output show
decoded fields. Decoding does NOT verify Apple's signature or certificate
chain, so never grant entitlements from decoded output alone.`

var historyProductTypes = []string{"AUTO_RENEWABLE", "NON_RENEWABLE", "CONSUMABLE", "NON_CONSUMABLE"}

var historyOwnershipTypes = []string{"FAMILY_SHARED", "PURCHASED", "ASSIGNED"}

func transactionsCommand(d *shared.Deps) *ffcli.Command {
	return groupCommand(d, "transactions",
		"Read and manage In-App Purchase transactions with the App Store Server API.",
		`Read and manage In-App Purchase transactions with the App Store Server API.

Examples:
  asc storekit transactions view --transaction-id 2000000000000001 --environment sandbox
  asc storekit transactions history --transaction-id 2000000000000001 --environment production --paginate
  asc storekit transactions history --transaction-id 2000000000000001 --product-type AUTO_RENEWABLE --sort DESCENDING --environment production
  asc storekit transactions history --transaction-id 2000000000000001 --ownership-type ASSIGNED --environment production
  asc storekit transactions set-account-token --original-transaction-id 2000000000000001 --app-account-token 11111111-2222-4333-8444-555555555555 --environment sandbox --confirm
  asc storekit transactions send-consumption --transaction-id 2000000000000001 --customer-consented true --delivery-status DELIVERED --sample-content-provided false --environment production --confirm
  asc storekit transactions finish --transaction-id 2000000000000001 --environment production --confirm`,
		transactionsViewCommand(d), transactionsHistoryCommand(d), setAccountTokenCommand(d), sendConsumptionCommand(d), finishTransactionCommand(d))
}

func subscriptionsCommand(d *shared.Deps) *ffcli.Command {
	return groupCommand(d, "subscriptions",
		"Read subscription status and extend renewal dates with the App Store Server API.",
		`Read auto-renewable subscription statuses and extend renewal dates with the
App Store Server API.

Examples:
  asc storekit subscriptions status --transaction-id 2000000000000001 --environment production
  asc storekit subscriptions status --transaction-id 2000000000000001 --status 1,4 --environment sandbox
  asc storekit subscriptions extend --original-transaction-id 2000000000000001 --days 7 --reason 3 --request-id outage-2026-10-01 --environment production --confirm
  asc storekit subscriptions extension-status --product-id com.example.monthly --request-id 11111111-2222-4333-8444-555555555555 --environment production`,
		subscriptionsStatusCommand(d), extendSubscriptionCommand(d), extendAllSubscriptionsCommand(d), extensionStatusCommand(d))
}

func refundsCommand(d *shared.Deps) *ffcli.Command {
	return groupCommand(d, "refunds",
		"Read a customer's refunded In-App Purchases with the App Store Server API.",
		`Read a customer's refunded In-App Purchases with the App Store Server API.

Examples:
  asc storekit refunds history --transaction-id 2000000000000001 --environment production --paginate`,
		refundsHistoryCommand(d))
}

func ordersCommand(d *shared.Deps) *ffcli.Command {
	return groupCommand(d, "orders",
		"Look up a customer's In-App Purchases by order ID.",
		`Look up a customer's In-App Purchases by the order ID from their App Store receipt.

Apple doesn't offer order lookup in the sandbox environment.

Examples:
  asc storekit orders lookup --order-id MQXYZ12345 --environment production`,
		ordersLookupCommand(d))
}

func notificationsCommand(d *shared.Deps) *ffcli.Command {
	return groupCommand(d, "notifications",
		"Read App Store Server Notifications history and send test notifications.",
		`Read App Store Server Notifications history and send test notifications.

Examples:
  asc storekit notifications history --start 2026-09-01 --end 2026-09-08 --environment production
  asc storekit notifications history --start 2026-09-01 --end 2026-09-08 --type SUBSCRIBED --only-failures --environment production --paginate
  asc storekit notifications test --environment sandbox --confirm
  asc storekit notifications test-status --token TOKEN --environment sandbox`,
		notificationsHistoryCommand(d), notificationsTestCommand(d), notificationsTestStatusCommand(d))
}

func groupsCommand(d *shared.Deps) *ffcli.Command {
	return groupCommand(d, "groups",
		"Read multiseat customer groups and their members.",
		`Read multiseat customer groups and their members with the App Store Server API.

Production serves groups with a groupType of ORGANIZATION; sandbox also
serves Apple's placeholder groups. Group membership is separate from
entitlement: read transactions to decide what a customer can access.

Examples:
  asc storekit groups list --transaction-id 2000000000000001 --environment production
  asc storekit groups members --group-id 900000000000000000 --environment sandbox --paginate`,
		groupsListCommand(d), groupsMembersCommand(d))
}

func transactionsViewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit transactions view", flag.ExitOnError)
	transactionID := fs.String("transaction-id", "", "Transaction ID or original transaction ID")
	flags := bindServerReadFlags(d, fs, true)
	return documentedLeafCommand("view", "asc storekit transactions view --transaction-id ID --environment ENV [flags]",
		"Get one transaction (Get Transaction Info).",
		"Get one transaction with Apple's Get Transaction Info endpoint.\n\n"+unverifiedJWSHelp, fs,
		func(ctx context.Context, args []string) error {
			if err := validateServerRead(d, flags, requiredFlag{"--transaction-id", *transactionID}); err != nil {
				return err
			}
			raw, err := serverRequest(ctx, d, fs.Name(), flags.common, func(ctx context.Context, client *storekitapi.Client) (json.RawMessage, error) {
				return client.GetTransactionInfo(ctx, *transactionID)
			})
			if err != nil {
				return err
			}
			return printServerResponse(d, raw, flags, transactionInfoTable)
		})
}

func transactionsHistoryCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit transactions history", flag.ExitOnError)
	transactionID := fs.String("transaction-id", "", "Any transaction, original transaction, or app transaction ID for the customer")
	revision := fs.String("revision", "", "Revision token from a previous response, to continue from that page")
	start := fs.String("start", "", "Only purchases on or after this date (YYYY-MM-DD or RFC3339)")
	end := fs.String("end", "", "Only purchases before this date (YYYY-MM-DD or RFC3339)")
	productIDs := fs.String("product-id", "", "Product ID filter (comma-separated)")
	productTypes := fs.String("product-type", "", "Product type filter (comma-separated): "+strings.Join(historyProductTypes, ", "))
	ownershipType := fs.String("ownership-type", "", "In-app ownership filter (one value): "+strings.Join(historyOwnershipTypes, ", "))
	sort := fs.String("sort", "", "Sort by modified date: ASCENDING (Apple's default) or DESCENDING")
	var revoked shared.OptionalBool
	fs.Var(&revoked, "revoked", "true for only revoked transactions, false for only non-revoked")
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	flags := bindServerReadFlags(d, fs, true)
	return documentedLeafCommand("history", "asc storekit transactions history --transaction-id ID --environment ENV [flags]",
		"Get a customer's transaction history (Get Transaction History v2).",
		"Get a customer's In-App Purchase history with Apple's Get Transaction History v2\nendpoint. Apple returns up to 20 transactions per page; use --paginate for all\npages or --revision to continue from a stored revision with the same filters.\n\n"+unverifiedJWSHelp, fs,
		func(ctx context.Context, args []string) error {
			if err := validateServerRead(d, flags, requiredFlag{"--transaction-id", *transactionID}); err != nil {
				return err
			}
			query, err := transactionHistoryQuery(d, *start, *end, *productIDs, *productTypes, *ownershipType, *sort, revoked)
			if err != nil {
				return err
			}
			raw, err := serverRequest(ctx, d, fs.Name(), flags.common, func(ctx context.Context, client *storekitapi.Client) (json.RawMessage, error) {
				return fetchServerPages(func(token string) (json.RawMessage, error) {
					return client.GetTransactionHistory(ctx, *transactionID, query, token)
				}, *revision, *paginate, storekitapi.SignedTransactionPages)
			})
			if err != nil {
				return err
			}
			return printServerPage(d, raw, flags, signedTransactionsTable, *paginate, storekitapi.SignedTransactionPages, "--revision")
		})
}

func subscriptionsStatusCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit subscriptions status", flag.ExitOnError)
	transactionID := fs.String("transaction-id", "", "Any transaction, original transaction, or app transaction ID for the customer")
	status := fs.String("status", "", "Status filter (comma-separated): 1 active, 2 expired, 3 billing retry, 4 grace period, 5 revoked")
	flags := bindServerReadFlags(d, fs, true)
	return documentedLeafCommand("status", "asc storekit subscriptions status --transaction-id ID --environment ENV [flags]",
		"Get all subscription statuses for a customer (Get All Subscription Statuses).",
		"Get the status of every auto-renewable subscription a customer has in your app\nwith Apple's Get All Subscription Statuses endpoint.\n\n"+unverifiedJWSHelp, fs,
		func(ctx context.Context, args []string) error {
			if err := validateServerRead(d, flags, requiredFlag{"--transaction-id", *transactionID}); err != nil {
				return err
			}
			statuses, err := parseSubscriptionStatuses(d, *status)
			if err != nil {
				return err
			}
			raw, err := serverRequest(ctx, d, fs.Name(), flags.common, func(ctx context.Context, client *storekitapi.Client) (json.RawMessage, error) {
				return client.GetAllSubscriptionStatuses(ctx, *transactionID, statuses)
			})
			if err != nil {
				return err
			}
			return printServerResponse(d, raw, flags, subscriptionStatusTable)
		})
}

func refundsHistoryCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit refunds history", flag.ExitOnError)
	transactionID := fs.String("transaction-id", "", "Any transaction, original transaction, or app transaction ID for the customer")
	revision := fs.String("revision", "", "Revision token from a previous response, to continue from that page")
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	flags := bindServerReadFlags(d, fs, true)
	return documentedLeafCommand("history", "asc storekit refunds history --transaction-id ID --environment ENV [flags]",
		"Get a customer's refunded purchases (Get Refund History v2).",
		"Get a customer's App Store-approved refunds with Apple's Get Refund History v2\nendpoint. Apple returns up to 20 transactions per page; use --paginate for all\npages or --revision to continue from a stored revision.\n\n"+unverifiedJWSHelp, fs,
		func(ctx context.Context, args []string) error {
			if err := validateServerRead(d, flags, requiredFlag{"--transaction-id", *transactionID}); err != nil {
				return err
			}
			raw, err := serverRequest(ctx, d, fs.Name(), flags.common, func(ctx context.Context, client *storekitapi.Client) (json.RawMessage, error) {
				return fetchServerPages(func(token string) (json.RawMessage, error) {
					return client.GetRefundHistory(ctx, *transactionID, token)
				}, *revision, *paginate, storekitapi.SignedTransactionPages)
			})
			if err != nil {
				return err
			}
			return printServerPage(d, raw, flags, signedTransactionsTable, *paginate, storekitapi.SignedTransactionPages, "--revision")
		})
}

func ordersLookupCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit orders lookup", flag.ExitOnError)
	orderID := fs.String("order-id", "", "Order ID from the customer's App Store receipt")
	flags := bindServerReadFlags(d, fs, true)
	return documentedLeafCommand("lookup", "asc storekit orders lookup --order-id ID --environment production [flags]",
		"Look up In-App Purchases by order ID (Look Up Order ID).",
		"Look up the In-App Purchases in a customer's order with Apple's Look Up Order ID\nendpoint. A status of 0 means the order ID is valid; 1 means it's invalid or\nhas no In-App Purchases for your app. Apple doesn't offer this endpoint in\nsandbox, so --environment must be production.\n\n"+unverifiedJWSHelp, fs,
		func(ctx context.Context, args []string) error {
			if err := validateServerRead(d, flags, requiredFlag{"--order-id", *orderID}); err != nil {
				return err
			}
			if err := requireEnvironment(d, flags.common, storekitapi.Production,
				"orders lookup requires --environment production (Apple doesn't offer Look Up Order ID in sandbox)"); err != nil {
				return err
			}
			raw, err := serverRequest(ctx, d, fs.Name(), flags.common, func(ctx context.Context, client *storekitapi.Client) (json.RawMessage, error) {
				return client.LookUpOrderID(ctx, *orderID)
			})
			if err != nil {
				return err
			}
			return printServerResponse(d, raw, flags, orderLookupTable)
		})
}

func notificationsHistoryCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit notifications history", flag.ExitOnError)
	start := fs.String("start", "", "Start of the timespan (YYYY-MM-DD or RFC3339); Apple keeps 180 days in production, 30 in sandbox")
	end := fs.String("end", "", "End of the timespan (YYYY-MM-DD or RFC3339)")
	notificationType := fs.String("type", "", "Only this notificationType, such as SUBSCRIBED or REFUND")
	subtype := fs.String("subtype", "", "Only this notification subtype (requires --type)")
	transactionID := fs.String("transaction-id", "", "Only notifications for this customer's transaction (cannot combine with --type)")
	onlyFailures := fs.Bool("only-failures", false, "Only notifications that haven't reached your server")
	paginationToken := fs.String("pagination-token", "", "Pagination token from a previous response, to continue from that page")
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	flags := bindServerReadFlags(d, fs, true)
	return documentedLeafCommand("history", "asc storekit notifications history --start DATE --end DATE --environment ENV [flags]",
		"Get App Store Server Notifications history (Get Notification History).",
		"Get the version 2 notifications the App Store attempted to send to your server\nwith Apple's Get Notification History endpoint. Apple transports this read as a\nPOST; it changes nothing and runs in read-only mode. Apple returns up to 20\nrecords per page; use --paginate for all pages.\n\n"+unverifiedJWSHelp, fs,
		func(ctx context.Context, args []string) error {
			if err := validateServerRead(d, flags, requiredFlag{"--start", *start}, requiredFlag{"--end", *end}); err != nil {
				return err
			}
			request, err := notificationHistoryRequest(d, *start, *end, *notificationType, *subtype, *transactionID, *onlyFailures)
			if err != nil {
				return err
			}
			raw, err := serverRequest(ctx, d, fs.Name(), flags.common, func(ctx context.Context, client *storekitapi.Client) (json.RawMessage, error) {
				return fetchServerPages(func(token string) (json.RawMessage, error) {
					return client.GetNotificationHistory(ctx, request, token)
				}, *paginationToken, *paginate, storekitapi.NotificationHistoryPages)
			})
			if err != nil {
				return err
			}
			return printServerPage(d, raw, flags, notificationHistoryTable, *paginate, storekitapi.NotificationHistoryPages, "--pagination-token")
		})
}

func notificationsTestCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit notifications test", flag.ExitOnError)
	confirm := fs.Bool("confirm", false, "Confirm sending a TEST notification to your configured server URL")
	flags := bindServerReadFlags(d, fs, false)
	return documentedLeafCommand("test", "asc storekit notifications test --environment ENV --confirm [flags]",
		"Ask the App Store to send a TEST notification (Request a Test Notification).",
		`Ask the App Store to send one TEST notification to the App Store Server
Notifications URL configured for the chosen environment, with Apple's Request a
Test Notification endpoint. Pass the returned testNotificationToken to
"asc storekit notifications test-status" to see whether your server received it.

Examples:
  asc storekit notifications test --environment sandbox --confirm`, fs,
		func(ctx context.Context, args []string) error {
			if err := validateServerRead(d, flags); err != nil {
				return err
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}
			raw, err := serverRequest(ctx, d, fs.Name(), flags.common, func(ctx context.Context, client *storekitapi.Client) (json.RawMessage, error) {
				return client.RequestTestNotification(ctx)
			})
			if err != nil {
				return err
			}
			return printServerResponse(d, raw, flags, testNotificationTable)
		})
}

func notificationsTestStatusCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit notifications test-status", flag.ExitOnError)
	token := fs.String("token", "", "testNotificationToken from asc storekit notifications test")
	flags := bindServerReadFlags(d, fs, true)
	return documentedLeafCommand("test-status", "asc storekit notifications test-status --token TOKEN --environment ENV [flags]",
		"Check a TEST notification's delivery (Get Test Notification Status).",
		"Check whether the App Store delivered a TEST notification to your server with\nApple's Get Test Notification Status endpoint. Apple returns 404 until the\nstatus is available.\n\n"+unverifiedJWSHelp, fs,
		func(ctx context.Context, args []string) error {
			if err := validateServerRead(d, flags, requiredFlag{"--token", *token}); err != nil {
				return err
			}
			raw, err := serverRequest(ctx, d, fs.Name(), flags.common, func(ctx context.Context, client *storekitapi.Client) (json.RawMessage, error) {
				return client.GetTestNotificationStatus(ctx, *token)
			})
			if err != nil {
				return err
			}
			return printServerResponse(d, raw, flags, testNotificationStatusTable)
		})
}

func groupsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit groups list", flag.ExitOnError)
	transactionID := fs.String("transaction-id", "", "Any transaction, original transaction, or app transaction ID for the customer")
	flags := bindServerReadFlags(d, fs, false)
	return documentedLeafCommand("list", "asc storekit groups list --transaction-id ID --environment ENV [flags]",
		"List the groups a customer belongs to (Get Customer Groups).",
		`List the multiseat groups a customer belongs to, and their role for each
product, with Apple's Get Customer Groups endpoint. In sandbox, a customer
without a CONSUMER group gets Apple's placeholder ORGANIZATION group
900000000000000000.

Examples:
  asc storekit groups list --transaction-id 2000000000000001 --environment production`, fs,
		func(ctx context.Context, args []string) error {
			if err := validateServerRead(d, flags, requiredFlag{"--transaction-id", *transactionID}); err != nil {
				return err
			}
			raw, err := serverRequest(ctx, d, fs.Name(), flags.common, func(ctx context.Context, client *storekitapi.Client) (json.RawMessage, error) {
				return client.GetCustomerGroups(ctx, *transactionID)
			})
			if err != nil {
				return err
			}
			return printServerResponse(d, raw, flags, customerGroupsTable)
		})
}

func groupsMembersCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("storekit groups members", flag.ExitOnError)
	groupID := fs.String("group-id", "", "Group ID from asc storekit groups list")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(100))
	paginationToken := fs.String("pagination-token", "", "Pagination token from a previous response, to continue from that page")
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	flags := bindServerReadFlags(d, fs, false)
	return documentedLeafCommand("members", "asc storekit groups members --group-id ID --environment ENV [flags]",
		"List the members of a group (Get Group Members).",
		`List the customers in a multiseat group, by app transaction ID, with Apple's
Get Group Members endpoint.

Examples:
  asc storekit groups members --group-id 900000000000000000 --environment sandbox --paginate`, fs,
		func(ctx context.Context, args []string) error {
			if flagWasSet(fs, "limit") && (*limit < 1 || *limit > 100) {
				return d.UsageError("storekit groups members: --limit must be between 1 and 100")
			}
			if err := validateServerRead(d, flags, requiredFlag{"--group-id", *groupID}); err != nil {
				return err
			}
			raw, err := serverRequest(ctx, d, fs.Name(), flags.common, func(ctx context.Context, client *storekitapi.Client) (json.RawMessage, error) {
				return fetchServerPages(func(token string) (json.RawMessage, error) {
					return client.GetGroupMembers(ctx, *groupID, *limit, token)
				}, *paginationToken, *paginate, storekitapi.GroupMemberPages)
			})
			if err != nil {
				return err
			}
			return printServerPage(d, raw, flags, groupMembersTable, *paginate, storekitapi.GroupMemberPages, "--pagination-token")
		})
}

func groupCommand(d *shared.Deps, name, short, long string, subcommands ...*ffcli.Command) *ffcli.Command {
	return &ffcli.Command{
		Name:        name,
		ShortUsage:  "asc storekit " + name + " <subcommand> [flags]",
		ShortHelp:   short,
		LongHelp:    long,
		FlagSet:     d.NewFlagSet("storekit "+name, flag.ExitOnError),
		Subcommands: subcommands,
		UsageFunc:   shared.DefaultUsageFunc,
		Exec:        func(ctx context.Context, args []string) error { return flag.ErrHelp },
	}
}

func documentedLeafCommand(name, usage, short, long string, fs *flag.FlagSet, exec func(context.Context, []string) error) *ffcli.Command {
	command := leafCommand(name, usage, short, fs, exec)
	command.LongHelp = long
	return command
}

type requiredFlag struct {
	name  string
	value string
}

func validateServerRead(d *shared.Deps, flags serverReadFlags, required ...requiredFlag) error {
	for _, item := range required {
		if err := requireFlag(d, item.name, item.value); err != nil {
			return err
		}
	}
	if flags.decode != nil && *flags.decode && shared.NormalizeOutputFormat(*flags.output.Output) != "json" {
		return d.UsageError("--decode requires --output json")
	}
	return nil
}

// requireEnvironment rejects any environment other than want, for endpoints
// Apple offers in only one environment.
func requireEnvironment(d *shared.Deps, common commonFlags, want storekitapi.Environment, message string) error {
	environment, err := resolveEnvironment(common.Environment)
	if err != nil {
		return d.UsageError(err.Error())
	}
	if environment != want {
		return d.UsageError(message)
	}
	return nil
}

func serverRequest(ctx context.Context, d *shared.Deps, command string, common commonFlags, call func(context.Context, *storekitapi.Client) (json.RawMessage, error)) (json.RawMessage, error) {
	client, _, err := resolveClient(ctx, d, common)
	if err != nil {
		return nil, usageOrWrap(d, command, err)
	}
	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()
	raw, err := call(requestCtx, client)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", command, err)
	}
	return raw, nil
}

func fetchServerPages(fetch func(token string) (json.RawMessage, error), startToken string, paginate bool, cursor storekitapi.PageCursor) (json.RawMessage, error) {
	first, err := fetch(startToken)
	if err != nil || !paginate {
		return first, err
	}
	return storekitapi.CollectPages(first, cursor, fetch)
}

func transactionHistoryQuery(d *shared.Deps, start, end, productIDs, productTypes, ownershipType, sort string, revoked shared.OptionalBool) (storekitapi.TransactionHistoryQuery, error) {
	var query storekitapi.TransactionHistoryQuery
	var err error
	if query.StartDate, err = parseOptionalTimeFlag(d, "--start", start); err != nil {
		return query, err
	}
	if query.EndDate, err = parseOptionalTimeFlag(d, "--end", end); err != nil {
		return query, err
	}
	if !query.StartDate.IsZero() && !query.EndDate.IsZero() && !query.StartDate.Before(query.EndDate) {
		return query, d.UsageError("--start must be before --end")
	}
	query.ProductIDs = shared.SplitUniqueCSV(productIDs)
	query.ProductTypes = shared.SplitCSVUpper(productTypes)
	for _, productType := range query.ProductTypes {
		if !slices.Contains(historyProductTypes, productType) {
			return query, d.UsageErrorf("--product-type must be one of: %s", strings.Join(historyProductTypes, ", "))
		}
	}
	query.OwnershipType = strings.ToUpper(strings.TrimSpace(ownershipType))
	if query.OwnershipType != "" && !slices.Contains(historyOwnershipTypes, query.OwnershipType) {
		return query, d.UsageErrorf("--ownership-type must be one of: %s", strings.Join(historyOwnershipTypes, ", "))
	}
	query.Sort = strings.ToUpper(strings.TrimSpace(sort))
	if query.Sort != "" && query.Sort != "ASCENDING" && query.Sort != "DESCENDING" {
		return query, d.UsageError("--sort must be one of: ASCENDING, DESCENDING")
	}
	if revoked.IsSet() {
		value := revoked.Value()
		query.Revoked = &value
	}
	return query, nil
}

func notificationHistoryRequest(d *shared.Deps, start, end, notificationType, subtype, transactionID string, onlyFailures bool) (storekitapi.NotificationHistoryRequest, error) {
	startDate, err := parseOptionalTimeFlag(d, "--start", start)
	if err != nil {
		return storekitapi.NotificationHistoryRequest{}, err
	}
	endDate, err := parseOptionalTimeFlag(d, "--end", end)
	if err != nil {
		return storekitapi.NotificationHistoryRequest{}, err
	}
	if !startDate.Before(endDate) {
		return storekitapi.NotificationHistoryRequest{}, d.UsageError("--start must be before --end")
	}
	request := storekitapi.NotificationHistoryRequest{
		StartDate:           startDate.UnixMilli(),
		EndDate:             endDate.UnixMilli(),
		NotificationType:    strings.ToUpper(strings.TrimSpace(notificationType)),
		NotificationSubtype: strings.ToUpper(strings.TrimSpace(subtype)),
		TransactionID:       strings.TrimSpace(transactionID),
		OnlyFailures:        onlyFailures,
	}
	if request.NotificationSubtype != "" && request.NotificationType == "" {
		return request, d.UsageError("--subtype requires --type")
	}
	if request.NotificationType != "" && request.TransactionID != "" {
		return request, d.UsageError("--type and --transaction-id are mutually exclusive")
	}
	return request, nil
}

func parseSubscriptionStatuses(d *shared.Deps, value string) ([]int, error) {
	var statuses []int
	for _, item := range shared.SplitUniqueCSV(value) {
		status, err := strconv.Atoi(item)
		if err != nil || status < 1 || status > 5 {
			return nil, d.UsageError("--status values must be integers from 1 to 5")
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// parseOptionalTimeFlag accepts YYYY-MM-DD (UTC midnight) or RFC3339.
func parseOptionalTimeFlag(d *shared.Deps, name, value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed, nil
	}
	if parsed, ok := shared.ParseRFC3339Date(value); ok {
		return parsed, nil
	}
	return time.Time{}, d.UsageErrorf("%s must be YYYY-MM-DD or RFC3339", name)
}
