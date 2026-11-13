package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	sandboxcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/sandbox"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var createWebSandboxTesterFn = func(ctx context.Context, client *webcore.Client, attrs webcore.SandboxAccountCreateAttributes) error {
	return client.CreateSandboxAccount(ctx, attrs)
}

var listWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client) (*webcore.SandboxAccountListResponse, error) {
	return client.ListSandboxAccounts(ctx)
}

var deleteWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client, ids []string) error {
	return client.DeleteSandboxAccounts(ctx, ids)
}

type webSandboxCreateResult struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Territory string `json:"territory"`
	Submitted bool   `json:"submitted"`
}

// WebSandboxCommand returns the detached web sandbox command group.
func WebSandboxCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web sandbox", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "sandbox",
		ShortUsage: "asc web sandbox <subcommand> [flags]",
		ShortHelp:  "Manage sandbox testers via web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Create and delete sandbox testers using App Store Connect's web session
endpoints. The public App Store Connect API does not expose these account
management operations.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebSandboxCreateCommand(),
			WebSandboxDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebSandboxDeleteCommand deletes sandbox testers through Apple's private web
// session endpoint. It preflights every requested ID against the private list,
// refuses family members or incomplete/ambiguous snapshots, and verifies the
// deletion with one fresh list request.
func WebSandboxDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web sandbox delete", flag.ExitOnError)

	ids := fs.String("id", "", "Sandbox tester account ID(s), comma-separated")
	confirm := fs.Bool("confirm", false, "Confirm deleting the selected sandbox testers")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc web sandbox delete --id ID[,ID...] --confirm [flags]",
		ShortHelp:  "Delete sandbox testers via web API.",
		LongHelp: `WEB SESSION WORKFLOWS

Delete sandbox testers through App Store Connect's web-session API. The
command first reads Apple's private account list, refuses any requested account
that is marked as a family member, sends one delete request for the validated
IDs, and reads the list again to confirm that every ID is absent.

The command fails before mutation when an ID is missing or duplicated in the
list, when Apple omits the family-member flag, or when the 50-account response
is incomplete. A transport error during deletion or verification is reported
as an unknown outcome; do not retry until a fresh list confirms the state.

Required:
  --id, --confirm

Examples:
  asc web sandbox delete --id "SANDBOX_ACCOUNT_ID" --confirm
  asc web sandbox delete --id "ACCOUNT_ID_1,ACCOUNT_ID_2" --confirm --output json

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			requestedIDs := shared.SplitUniqueCSV(*ids)
			if len(requestedIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			preflight, err := withWebSpinnerValue("Checking sandbox testers before deletion", func() (*webcore.SandboxAccountListResponse, error) {
				return listWebSandboxAccountsFn(requestCtx, client)
			})
			if err != nil {
				return withWebAuthHint(err, "web sandbox delete")
			}
			if err := validateSandboxDeletePreflight(preflight, requestedIDs); err != nil {
				return err
			}

			if err := withWebSpinner("Deleting sandbox testers via Apple web API", func() error {
				return deleteWebSandboxAccountsFn(requestCtx, client, requestedIDs)
			}); err != nil {
				return sandboxDeleteMutationError(err)
			}

			verified, err := withWebSpinnerValue("Verifying sandbox tester deletion", func() (*webcore.SandboxAccountListResponse, error) {
				return listWebSandboxAccountsFn(requestCtx, client)
			})
			if err != nil {
				return sandboxDeleteUnknownOutcome("verification", err)
			}
			if err := validateSandboxDeletePostcondition(verified, requestedIDs); err != nil {
				return err
			}

			result := &asc.WebSandboxDeleteResult{IDs: requestedIDs, Deleted: true}
			if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return sandboxDeleteVerifiedOutputError(requestedIDs, err)
			}
			return nil
		},
	}
}

func validateSandboxDeletePreflight(snapshot *webcore.SandboxAccountListResponse, requestedIDs []string) error {
	if err := validateSandboxAccountListSnapshot(snapshot); err != nil {
		return fmt.Errorf("web sandbox delete failed: %w", err)
	}

	requested := make(map[string]struct{}, len(requestedIDs))
	for _, id := range requestedIDs {
		requested[id] = struct{}{}
	}
	matches := make(map[string][]webcore.SandboxAccount, len(requestedIDs))
	for _, account := range snapshot.Accounts {
		id := strings.TrimSpace(account.ID)
		if _, ok := requested[id]; ok {
			matches[id] = append(matches[id], account)
		}
	}

	for _, id := range requestedIDs {
		accounts := matches[id]
		switch len(accounts) {
		case 0:
			return fmt.Errorf("web sandbox delete failed: sandbox account %q was not found in Apple's private account list", id)
		case 1:
			account := accounts[0]
			if account.IsInFamily == nil {
				return fmt.Errorf("web sandbox delete failed: Apple omitted isInFamily for sandbox account %q; refusing deletion", id)
			}
			if *account.IsInFamily {
				return fmt.Errorf("web sandbox delete failed: sandbox account %q is a family member; refusing deletion", id)
			}
		default:
			return fmt.Errorf("web sandbox delete failed: sandbox account %q appeared %d times in Apple's private account list; refusing ambiguous deletion", id, len(accounts))
		}
	}
	return nil
}

func validateSandboxDeletePostcondition(snapshot *webcore.SandboxAccountListResponse, requestedIDs []string) error {
	if err := validateSandboxAccountListSnapshot(snapshot); err != nil {
		return sandboxDeleteUnknownOutcome("verification", err)
	}
	requested := make(map[string]struct{}, len(requestedIDs))
	for _, id := range requestedIDs {
		requested[id] = struct{}{}
	}
	for _, account := range snapshot.Accounts {
		id := strings.TrimSpace(account.ID)
		if _, ok := requested[id]; ok {
			return sandboxDeleteUnknownOutcome("verification", fmt.Errorf("server still returned sandbox account %q after deletion", id))
		}
	}
	return nil
}

func validateSandboxAccountListSnapshot(snapshot *webcore.SandboxAccountListResponse) error {
	if snapshot == nil {
		return fmt.Errorf("no sandbox account list returned")
	}
	if snapshot.TotalAccounts < 0 || snapshot.TotalInactiveAccounts < 0 {
		return fmt.Errorf("invalid sandbox account totals returned")
	}
	if snapshot.TotalAccounts < len(snapshot.Accounts) {
		return fmt.Errorf("sandbox account list reported %d accounts but only %d total accounts", len(snapshot.Accounts), snapshot.TotalAccounts)
	}
	if snapshot.TotalAccounts > len(snapshot.Accounts) {
		return fmt.Errorf("sandbox account list returned only %d of %d accounts; refusing to assume a requested account is absent", len(snapshot.Accounts), snapshot.TotalAccounts)
	}
	return nil
}

func sandboxDeleteMutationError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *webcore.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Status == http.StatusRequestTimeout || apiErr.Status >= 500 && apiErr.Status < 600 {
			return sandboxDeleteUnknownOutcome("delete request", err)
		}
		return withWebAuthHint(err, "web sandbox delete")
	}
	return sandboxDeleteUnknownOutcome("delete request", err)
}

func sandboxDeleteUnknownOutcome(stage string, err error) error {
	if err == nil {
		return fmt.Errorf("web sandbox delete failed: outcome unknown after %s", stage)
	}
	return fmt.Errorf("web sandbox delete failed: outcome unknown after %s; Apple may have processed the request, so do not retry until a fresh sandbox account list confirms state: %w", stage, err)
}

func sandboxDeleteVerifiedOutputError(ids []string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("web sandbox delete completed and verified for sandbox account IDs %q, but output failed; do not retry the deletion: %w", strings.Join(ids, ", "), err)
}

// WebSandboxCreateCommand creates a sandbox tester via App Store Connect's
// web session endpoints.
func WebSandboxCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web sandbox create", flag.ExitOnError)

	firstName := fs.String("first-name", "", "Sandbox tester first name")
	lastName := fs.String("last-name", "", "Sandbox tester last name")
	email := fs.String("email", "", "Sandbox tester email address")
	password := fs.String("password", "", "Sandbox tester password")
	territory := fs.String("territory", "", "Sandbox tester territory/storefront code (e.g., USA)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web sandbox create --first-name NAME --last-name NAME --email EMAIL --password PASS --territory USA [flags]",
		ShortHelp:  "Create a sandbox tester via web API.",
		LongHelp: `WEB SESSION WORKFLOWS

Create a sandbox tester through App Store Connect's web API.
The current web flow validates the name/email first, validates the password,
then submits the create request with a 3-letter storefront code such as USA.
Apple may still require email verification before the tester is usable.

Required:
  --first-name, --last-name, --email, --password, --territory

Examples:
  asc web sandbox create --first-name "Jane" --last-name "Tester" --email "jane+sandbox@example.com" --password "Passwordtest1" --territory "USA"
  asc web sandbox create --first-name "Monthly" --last-name "Probe" --email "billing+monthly@example.com" --password "Passwordtest1" --territory "USA" --apple-id "user@example.com"

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			firstNameValue, err := normalizeWebSandboxName("--first-name", *firstName)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			lastNameValue, err := normalizeWebSandboxName("--last-name", *lastName)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			emailValue, err := normalizeWebSandboxEmail(*email)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			passwordValue, err := normalizeWebSandboxPassword(*password)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			territoryValue, err := sandboxcmd.NormalizeSandboxTerritoryCode(*territory)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			client := newWebClientFn(session)
			createAttrs := webcore.SandboxAccountCreateAttributes{
				FirstName:       firstNameValue,
				LastName:        lastNameValue,
				AccountName:     emailValue,
				AccountPassword: passwordValue,
				StoreFront:      territoryValue,
			}

			err = withWebSpinner("Creating sandbox tester via Apple web API", func() error {
				return createWebSandboxTesterFn(requestCtx, client, createAttrs)
			})
			if err != nil {
				return withWebAuthHint(err, "web sandbox create")
			}

			result := &webSandboxCreateResult{
				FirstName: firstNameValue,
				LastName:  lastNameValue,
				Email:     emailValue,
				Territory: territoryValue,
				Submitted: true,
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderWebSandboxCreateTable(result) },
				func() error { return renderWebSandboxCreateMarkdown(result) },
			)
		},
	}
}

func normalizeWebSandboxName(flagName, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", flagName)
	}
	return trimmed, nil
}

func normalizeWebSandboxEmail(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("--email is required")
	}
	parsedAddress, err := mail.ParseAddress(trimmed)
	if err != nil || parsedAddress == nil || strings.TrimSpace(parsedAddress.Address) != trimmed {
		return "", fmt.Errorf("--email must be a valid email address")
	}
	return trimmed, nil
}

func normalizeWebSandboxPassword(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("--password is required")
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	for _, r := range trimmed {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}

	if utf8.RuneCountInString(trimmed) < 8 || !hasUpper || !hasLower || !hasDigit {
		return "", fmt.Errorf("--password must be at least 8 characters and include uppercase, lowercase, and numeric characters")
	}
	return trimmed, nil
}

func renderWebSandboxCreateTable(result *webSandboxCreateResult) error {
	webRows := [][]string{{
		result.FirstName,
		result.LastName,
		result.Email,
		result.Territory,
		fmt.Sprintf("%t", result.Submitted),
	}}
	webHeaders := []string{"First Name", "Last Name", "Email", "Territory", "Submitted"}
	asc.RenderTable(webHeaders, webRows)
	return nil
}

func renderWebSandboxCreateMarkdown(result *webSandboxCreateResult) error {
	webRows := [][]string{{
		result.FirstName,
		result.LastName,
		result.Email,
		result.Territory,
		fmt.Sprintf("%t", result.Submitted),
	}}
	webHeaders := []string{"First Name", "Last Name", "Email", "Territory", "Submitted"}
	asc.RenderMarkdown(webHeaders, webRows)
	return nil
}
