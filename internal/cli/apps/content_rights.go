package apps

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

type contentRightsResult struct {
	AppID                    string                        `json:"app_id"`
	ContentRightsDeclaration *asc.ContentRightsDeclaration `json:"content_rights_declaration"`
}

// AppsContentRightsCommand returns the content-rights command group.
func AppsContentRightsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("content-rights", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "content-rights",
		ShortUsage: "asc apps content-rights <subcommand> [flags]",
		ShortHelp:  "Manage an app's content rights declaration.",
		LongHelp: `Manage an app's content rights declaration.

The content rights declaration indicates whether your app uses third-party content.
This is required for App Store submission.

Examples:
  asc apps content-rights view --app "APP_ID"
  asc apps content-rights edit --app "APP_ID" --uses-third-party-content=false`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppsContentRightsViewCommand(),
			AppsContentRightsEditCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppsContentRightsViewCommand returns the content-rights view subcommand.
func AppsContentRightsViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("content-rights view", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc apps content-rights view --app \"APP_ID\"",
		ShortHelp:  "View an app's content rights declaration.",
		LongHelp: `View an app's content rights declaration.

Examples:
  asc apps content-rights view --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps content-rights view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			app, err := client.GetApp(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("apps content-rights view: failed to fetch: %w", err)
			}

			result := buildContentRightsResult(app)
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTable([]string{"field", "value"}, contentRightsRows(result))
					return nil
				},
				func() error {
					asc.RenderMarkdown([]string{"field", "value"}, contentRightsRows(result))
					return nil
				},
			)
		},
	}
}

// AppsContentRightsEditCommand returns the content-rights edit subcommand.
func AppsContentRightsEditCommand() *ffcli.Command {
	fs := flag.NewFlagSet("content-rights edit", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	usesThirdParty := fs.String("uses-third-party-content", "", "Whether app uses third-party content (true/false, yes/no)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "edit",
		ShortUsage: "asc apps content-rights edit --app \"APP_ID\" --uses-third-party-content=false",
		ShortHelp:  "Edit an app's content rights declaration.",
		LongHelp: `Edit an app's content rights declaration.

This declares whether your app uses third-party content, which is required
for App Store submission.

  --uses-third-party-content=false  → DOES_NOT_USE_THIRD_PARTY_CONTENT
  --uses-third-party-content=true   → USES_THIRD_PARTY_CONTENT

Accepts: true, false, yes, no, uses, does-not-use, or the raw API values.

Examples:
  asc apps content-rights edit --app "APP_ID" --uses-third-party-content=false
  asc apps content-rights edit --app "APP_ID" --uses-third-party-content=true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}

			if strings.TrimSpace(*usesThirdParty) == "" {
				fmt.Fprintln(os.Stderr, "Error: --uses-third-party-content is required (true or false)")
				return shared.MissingRequiredUsageError()
			}

			declaration, err := parseContentRightsValue(*usesThirdParty)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps content-rights edit: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.AppUpdateAttributes{
				ContentRightsDeclaration: &declaration,
			}

			app, err := client.UpdateApp(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("apps content-rights edit: failed to update: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Content rights declaration set to %s\n", string(declaration))

			result := buildContentRightsResult(app)
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTable([]string{"field", "value"}, contentRightsRows(result))
					return nil
				},
				func() error {
					asc.RenderMarkdown([]string{"field", "value"}, contentRightsRows(result))
					return nil
				},
			)
		},
	}
}

func buildContentRightsResult(app *asc.AppResponse) contentRightsResult {
	if app == nil {
		return contentRightsResult{}
	}
	return contentRightsResult{
		AppID:                    strings.TrimSpace(app.Data.ID),
		ContentRightsDeclaration: app.Data.Attributes.ContentRightsDeclaration,
	}
}

func contentRightsRows(result contentRightsResult) [][]string {
	declaration := "unset"
	if result.ContentRightsDeclaration != nil && strings.TrimSpace(string(*result.ContentRightsDeclaration)) != "" {
		declaration = string(*result.ContentRightsDeclaration)
	}
	return [][]string{
		{"app_id", result.AppID},
		{"content_rights_declaration", declaration},
	}
}

// parseContentRightsValue converts a user-friendly string to the API enum.
// Accepts: true, false, yes, no, uses, does-not-use, and the raw API values.
func parseContentRightsValue(s string) (asc.ContentRightsDeclaration, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "false", "no", "does-not-use", "does_not_use_third_party_content":
		return asc.ContentRightsDeclarationDoesNotUseThirdPartyContent, nil
	case "true", "yes", "uses", "uses_third_party_content":
		return asc.ContentRightsDeclarationUsesThirdPartyContent, nil
	default:
		return "", fmt.Errorf("invalid value %q: use true/false, yes/no, or DOES_NOT_USE_THIRD_PARTY_CONTENT/USES_THIRD_PARTY_CONTENT", s)
	}
}
