package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// WebAppsDistributionUsersCommand returns the app-scoped custom-distribution
// Apple Account recipient command group.
func WebAppsDistributionUsersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps distribution users", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "users",
		ShortUsage: "asc web apps distribution users <subcommand> [flags]",
		ShortHelp:  "Manage Apple Account recipients for custom distribution.",
		LongHelp: `WEB SESSION WORKFLOWS

List and manage the Apple Account recipients attached to one app's custom
distribution configuration. The selected app's complete recipient collection
establishes ownership. Organization recipients and onboarding workflows are
separate and are not changed by these commands.

Create and delete require a validated CUSTOM app and --confirm. They do not
change the app's distribution method and never retry an ambiguous write.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppsDistributionUsersListCommand(),
			WebAppsDistributionUsersCreateCommand(),
			WebAppsDistributionUsersDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAppsDistributionUsersListCommand lists the selected app's complete
// customAppUsers collection when --paginate is requested.
func WebAppsDistributionUsersListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps distribution users list", flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	paginate := fs.Bool("paginate", false, "Fetch every recipient page")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web apps distribution users list --app APP_ID [--paginate] [flags]",
		ShortHelp:  "List Apple Account recipients for an app.",
		LongHelp: `List the Apple Account recipients returned by Apple's app-scoped
customAppUsers collection. JSON output preserves Apple's raw JSON:API envelope,
including unknown top-level and resource members. Use --paginate to follow the
validated collection links and aggregate all pages.

This read is available for any app distribution method. Human output shows the
opaque recipient ID and Apple Account.

Examples:
  asc web apps distribution users list --app 6759231657
  asc web apps distribution users list --app 6759231657 --paginate --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, err := resolveWebDistributionUsersAppID(*appID)
			if err != nil {
				return err
			}
			if err := validateWebDistributionUsersOutput(output); err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web apps distribution users list")
			}
			client := newWebClientFn(session)

			var result *webcore.CustomAppUsersListResult
			err = withWebSpinner("Loading app distribution recipients", func() error {
				result, err = client.ListCustomAppUsersWithPagination(requestCtx, resolvedAppID, *paginate)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps distribution users list")
			}
			if result == nil {
				return fmt.Errorf("web apps distribution users list failed: response did not include a collection")
			}
			return printWebCustomAppUsers(result, *output.Output, *output.Pretty)
		},
	}
}

// WebAppsDistributionUsersCreateCommand creates one app-scoped Apple Account
// recipient after a complete ownership preflight and verifies the read-back.
func WebAppsDistributionUsersCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps distribution users create", flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	recipientAppleID := fs.String("recipient-apple-id", "", "Apple Account to add as a recipient")
	confirm := fs.Bool("confirm", false, "Confirm adding the Apple Account recipient")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web apps distribution users create --app APP_ID --recipient-apple-id APPLE_ACCOUNT --confirm [flags]",
		ShortHelp:  "Add one Apple Account recipient.",
		LongHelp: `Add one Apple Account to the selected app's custom distribution
recipients. The selected app must already use CUSTOM distribution. The ordinary
--apple-id web-session flag selects the authenticated session and is separate
from --recipient-apple-id.

The command reads the complete selected-app collection before writing, skips an
exact existing account, sends one create request, and verifies the new member
with another complete collection read. Ambiguous outcomes remain uncertain and
are never retried automatically.

Example:
  asc web apps distribution users create --app 6759231657 --recipient-apple-id account@example.com --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, err := resolveWebDistributionUsersAppID(*appID)
			if err != nil {
				return err
			}
			appleID := strings.TrimSpace(*recipientAppleID)
			if appleID == "" {
				return shared.RequiredFlag("--recipient-apple-id")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			if err := validateWebDistributionUsersOutput(output); err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web apps distribution users create")
			}
			client := newWebClientFn(session)

			var receipt *asc.WebAppDistributionUserMutationResult
			err = withWebSpinner("Creating app distribution recipient", func() error {
				if err := requireCustomWebAppDistribution(requestCtx, client, resolvedAppID); err != nil {
					return err
				}
				before, err := client.ListCustomAppUsersPaginated(requestCtx, resolvedAppID)
				if err != nil {
					return err
				}
				existing, ok, err := findCustomAppUserByAppleID(before, appleID)
				if err != nil {
					return err
				}
				if ok {
					receipt = webAppDistributionUserReceipt(resolvedAppID, existing.ID, existing.AppleID, false, "unchanged")
					return nil
				}

				created, err := client.CreateCustomAppUser(requestCtx, resolvedAppID, appleID)
				if err != nil {
					if webcore.IsCustomAppUserWriteUncertain(err) {
						recipientID := ""
						if created != nil {
							recipientID = strings.TrimSpace(created.ID)
						}
						receipt = webAppDistributionUserUncertainReceipt(resolvedAppID, recipientID, appleID)
					}
					return err
				}
				if created == nil || strings.TrimSpace(created.ID) == "" {
					receipt = webAppDistributionUserUncertainReceipt(resolvedAppID, "", appleID)
					return fmt.Errorf("custom app user create response did not include a recipient id")
				}
				// The accepted write is still provisional until the selected-app
				// collection confirms the exact returned identity.
				receipt = webAppDistributionUserUncertainReceipt(resolvedAppID, created.ID, appleID)
				after, err := client.ListCustomAppUsersPaginated(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("created recipient %q could not be verified: %w", created.ID, err)
				}
				if !customAppUserCollectionContains(after, created.ID, appleID) {
					return fmt.Errorf("created recipient %q was not present in the selected app collection", created.ID)
				}
				receipt = webAppDistributionUserReceipt(resolvedAppID, created.ID, appleID, true, "created")
				return nil
			})

			return finishWebDistributionUserMutation(receipt, err, "web apps distribution users create", *output.Output, *output.Pretty)
		},
	}
}

// WebAppsDistributionUsersDeleteCommand deletes one selected app recipient
// after a complete ownership preflight and verifies its absence.
func WebAppsDistributionUsersDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps distribution users delete", flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	recipientID := fs.String("id", "", "Opaque custom app recipient resource ID")
	confirm := fs.Bool("confirm", false, "Confirm removing the Apple Account recipient")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc web apps distribution users delete --app APP_ID --id RECIPIENT_ID --confirm [flags]",
		ShortHelp:  "Remove one Apple Account recipient.",
		LongHelp: `Remove one recipient proven to belong to the selected app's
custom distribution collection. The selected app must already use CUSTOM
distribution. The command reads the complete collection before writing, sends
one delete request, and verifies the recipient is absent afterward. A confirmed
absent ID is a verified unchanged result; ambiguous outcomes remain uncertain
and are never retried automatically.

Example:
  asc web apps distribution users delete --app 6759231657 --id RECIPIENT_ID --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, err := resolveWebDistributionUsersAppID(*appID)
			if err != nil {
				return err
			}
			requestedID := strings.TrimSpace(*recipientID)
			if requestedID == "" {
				return shared.UsageError("--id is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			if err := validateWebDistributionUsersOutput(output); err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web apps distribution users delete")
			}
			client := newWebClientFn(session)

			var receipt *asc.WebAppDistributionUserMutationResult
			err = withWebSpinner("Deleting app distribution recipient", func() error {
				if err := requireCustomWebAppDistribution(requestCtx, client, resolvedAppID); err != nil {
					return err
				}
				before, err := client.ListCustomAppUsersPaginated(requestCtx, resolvedAppID)
				if err != nil {
					return err
				}
				target, ok := findCustomAppUserByID(before, requestedID)
				if !ok {
					receipt = webAppDistributionUserReceipt(resolvedAppID, requestedID, "", false, "unchanged")
					return nil
				}

				err = client.DeleteCustomAppUser(requestCtx, resolvedAppID, requestedID)
				if err != nil {
					if webcore.IsCustomAppUserWriteUncertain(err) {
						receipt = webAppDistributionUserUncertainReceipt(resolvedAppID, requestedID, target.AppleID)
					}
					return err
				}
				receipt = webAppDistributionUserUncertainReceipt(resolvedAppID, requestedID, target.AppleID)
				after, err := client.ListCustomAppUsersPaginated(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("deleted recipient %q could not be verified: %w", requestedID, err)
				}
				if customAppUserCollectionContainsID(after, requestedID) {
					return fmt.Errorf("deleted recipient %q is still present in the selected app collection", requestedID)
				}
				receipt = webAppDistributionUserReceipt(resolvedAppID, requestedID, target.AppleID, true, "deleted")
				return nil
			})

			return finishWebDistributionUserMutation(receipt, err, "web apps distribution users delete", *output.Output, *output.Pretty)
		},
	}
}

func resolveWebDistributionUsersAppID(value string) (string, error) {
	resolved := strings.TrimSpace(shared.ResolveAppID(value))
	if resolved == "" {
		fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
		return "", shared.MissingRequiredUsageError("--app")
	}
	return resolved, nil
}

func validateWebDistributionUsersOutput(output shared.OutputFlags) error {
	if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
		return shared.UsageError(err.Error())
	}
	return nil
}

func requireCustomWebAppDistribution(ctx context.Context, client *webcore.Client, appID string) error {
	distribution, err := getWebAppDistributionFn(ctx, client, appID)
	if err != nil {
		return err
	}
	if distribution == nil {
		return fmt.Errorf("cannot safely manage recipients: missing app distribution response")
	}
	if strings.TrimSpace(distribution.AppID) != appID {
		return fmt.Errorf("cannot safely manage recipients: app distribution response identified app %q, want %q", distribution.AppID, appID)
	}
	if strings.ToUpper(strings.TrimSpace(distribution.DistributionType)) != webcore.AppDistributionTypeCustom {
		return fmt.Errorf("app %q must use CUSTOM distribution before managing recipients; use asc web apps distribution set --app %s --method private --confirm", appID, appID)
	}
	return nil
}

func findCustomAppUserByAppleID(result *webcore.CustomAppUsersListResult, appleID string) (webcore.CustomAppUser, bool, error) {
	if result == nil {
		return webcore.CustomAppUser{}, false, nil
	}
	var match webcore.CustomAppUser
	for _, user := range result.Data {
		if user.AppleID == appleID {
			if match.ID != "" && match.ID != user.ID {
				return webcore.CustomAppUser{}, false, fmt.Errorf("selected app collection contains multiple recipients for the exact Apple Account; refusing to create a duplicate")
			}
			match = user
		}
	}
	if match.ID == "" {
		return webcore.CustomAppUser{}, false, nil
	}
	return match, true, nil
}

func findCustomAppUserByID(result *webcore.CustomAppUsersListResult, recipientID string) (webcore.CustomAppUser, bool) {
	if result == nil {
		return webcore.CustomAppUser{}, false
	}
	for _, user := range result.Data {
		if user.ID == recipientID {
			return user, true
		}
	}
	return webcore.CustomAppUser{}, false
}

func customAppUserCollectionContains(result *webcore.CustomAppUsersListResult, recipientID, appleID string) bool {
	if result == nil {
		return false
	}
	for _, user := range result.Data {
		if user.ID == recipientID && user.AppleID == appleID {
			return true
		}
	}
	return false
}

func customAppUserCollectionContainsID(result *webcore.CustomAppUsersListResult, recipientID string) bool {
	if result == nil {
		return false
	}
	for _, user := range result.Data {
		if user.ID == recipientID {
			return true
		}
	}
	return false
}

func webAppDistributionUserReceipt(appID, recipientID, appleID string, changed bool, status string) *asc.WebAppDistributionUserMutationResult {
	return &asc.WebAppDistributionUserMutationResult{
		AppID:       appID,
		RecipientID: recipientID,
		AppleID:     appleID,
		Changed:     &changed,
		Verified:    true,
		Status:      status,
	}
}

func webAppDistributionUserUncertainReceipt(appID, recipientID, appleID string) *asc.WebAppDistributionUserMutationResult {
	return &asc.WebAppDistributionUserMutationResult{
		AppID:       appID,
		RecipientID: recipientID,
		AppleID:     appleID,
		Changed:     nil,
		Verified:    false,
		Status:      "uncertain",
	}
}

func finishWebDistributionUserMutation(receipt *asc.WebAppDistributionUserMutationResult, operationErr error, operation, output string, pretty bool) error {
	if receipt != nil {
		printErr := shared.PrintOutput(receipt, output, pretty)
		if printErr != nil {
			if operationErr != nil {
				return errors.Join(withWebAuthHint(operationErr, operation), printErr)
			}
			return printErr
		}
	}
	if operationErr != nil {
		return withWebAuthHint(operationErr, operation)
	}
	if receipt == nil {
		return fmt.Errorf("%s failed: missing mutation receipt", operation)
	}
	return nil
}

func printWebCustomAppUsers(result *webcore.CustomAppUsersListResult, output string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		output,
		pretty,
		func() error {
			asc.RenderTable(webCustomAppUsersHeaders(), webCustomAppUsersRows(result))
			return nil
		},
		func() error {
			asc.RenderMarkdown(webCustomAppUsersHeaders(), webCustomAppUsersRows(result))
			return nil
		},
	)
}

func webCustomAppUsersHeaders() []string {
	return []string{"ID", "Apple Account"}
}

func webCustomAppUsersRows(result *webcore.CustomAppUsersListResult) [][]string {
	if result == nil {
		return nil
	}
	rows := make([][]string, 0, len(result.Data))
	for _, user := range result.Data {
		rows = append(rows, []string{user.ID, user.AppleID})
	}
	return rows
}
