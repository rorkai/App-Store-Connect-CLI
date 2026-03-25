package web

import (
	"context"
	"flag"
	"fmt"
	"net/mail"
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
		ShortHelp:  "[experimental] Create sandbox testers via web sessions.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Create sandbox testers using App Store Connect's private web session endpoints.
This command is intentionally detached from the official App Store Connect API
because Apple does not expose sandbox tester creation there.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebSandboxCreateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebSandboxCreateCommand creates a sandbox tester via App Store Connect's
// private web session endpoints.
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
		ShortHelp:  "[experimental] Create a sandbox tester via web API.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Create a sandbox tester through App Store Connect's private web API.
The current web flow validates the name/email first, validates the password,
then submits the create request with a 3-letter storefront code such as USA.
Apple may still require email verification before the tester is usable.

Required:
  --first-name, --last-name, --email, --password, --territory

Examples:
  asc web sandbox create --first-name "Jane" --last-name "Tester" --email "jane+sandbox@example.com" --password "Passwordtest1" --territory "USA"
  asc web sandbox create --first-name "Monthly" --last-name "Probe" --email "billing+monthly@example.com" --password "Passwordtest1" --territory "USA" --apple-id "user@example.com"

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web sandbox create does not accept positional arguments")
			}

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

			session, err := resolveWebSessionForCommand(ctx, authFlags)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

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
