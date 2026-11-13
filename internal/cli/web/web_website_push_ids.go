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

var listDeveloperWebsitePushIDsFn = func(ctx context.Context, client *webcore.Client) (*webcore.DeveloperWebsitePushIDsListResult, error) {
	return client.ListDeveloperWebsitePushIDs(ctx)
}

var getDeveloperWebsitePushIDFn = func(ctx context.Context, client *webcore.Client, websitePushID string) (*webcore.DeveloperWebsitePushIDGetResult, error) {
	return client.GetDeveloperWebsitePushID(ctx, websitePushID)
}

var createDeveloperWebsitePushIDFn = func(ctx context.Context, client *webcore.Client, request webcore.DeveloperWebsitePushIDCreateRequest) (*asc.WebWebsitePushIDMutationResult, error) {
	return client.CreateDeveloperWebsitePushID(ctx, request)
}

var deleteDeveloperWebsitePushIDFn = func(ctx context.Context, client *webcore.Client, request webcore.DeveloperWebsitePushIDDeleteRequest) (*asc.WebWebsitePushIDMutationResult, error) {
	return client.DeleteDeveloperWebsitePushID(ctx, request)
}

// WebWebsitePushIDsCommand returns the Website Push ID command group.
func WebWebsitePushIDsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web website-push-ids", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "website-push-ids",
		ShortUsage: "asc web website-push-ids <subcommand> [flags]",
		ShortHelp:  "Manage Website Push IDs via Developer Portal web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Read and manage Website Push IDs through Apple's Developer Portal web-session
endpoints. The lifecycle commands use Apple's captured modern JSON:API routes;
they fail closed when the capability graph is not empty and do not expose
rename because the current UI has no captured rename control.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebWebsitePushIDsListCommand(),
			WebWebsitePushIDsViewCommand(),
			WebWebsitePushIDsCreateCommand(),
			WebWebsitePushIDsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebWebsitePushIDsViewCommand reads one opaque modern Website Push ID
// resource and its captured capability relationship.
func WebWebsitePushIDsViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web website-push-ids view", flag.ExitOnError)
	websitePushID := fs.String("website-push-id", "", "Opaque Developer Portal Website Push ID resource ID")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc web website-push-ids view --website-push-id WEBSITE_PUSH_RESOURCE_ID [flags]",
		ShortHelp:  "Inspect one Website Push ID via a Developer Portal web session.",
		LongHelp: `Inspect one Website Push ID via a Developer Portal web session.

WEB SESSION WORKFLOWS

Inspect one opaque modern Website Push ID resource returned by Apple's
Developer Portal JSON:API. JSON output preserves Apple's complete response,
including unknown top-level members and capability relationships.

Examples:
  asc web website-push-ids view --website-push-id "WEBSITE_PUSH_RESOURCE_ID" --output table
  asc web website-push-ids view --website-push-id "WEBSITE_PUSH_RESOURCE_ID" --output json

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedWebsitePushID := strings.TrimSpace(*websitePushID)
			if resolvedWebsitePushID == "" {
				return shared.UsageError("--website-push-id is required")
			}
			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web website-push-ids view")
			}

			var result *webcore.DeveloperWebsitePushIDGetResult
			err = withWebSpinner("Loading Developer Portal Website Push ID", func() error {
				var getErr error
				result, getErr = getDeveloperWebsitePushIDFn(requestCtx, newDeveloperPortalClient(session, portalFlags), resolvedWebsitePushID)
				return getErr
			})
			if err != nil {
				return withWebAuthHint(err, "web website-push-ids view")
			}
			if result == nil {
				return fmt.Errorf("web website-push-ids view failed: missing view result")
			}
			persistDeveloperPortalSession(session)

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperWebsitePushIDTable(result) },
				func() error { return renderDeveloperWebsitePushIDMarkdown(result) },
			)
		},
	}
}

// WebWebsitePushIDsCreateCommand registers a Website Push ID through the
// captured modern JSON:API endpoint. Capability configuration is intentionally
// omitted until its writable contract is captured.
func WebWebsitePushIDsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web website-push-ids create", flag.ExitOnError)
	name := fs.String("name", "", "Human-readable Website Push ID name")
	identifier := fs.String("identifier", "", "Website Push ID identifier")
	confirm := fs.Bool("confirm", false, "Confirm registering this Website Push ID")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web website-push-ids create --name NAME --identifier IDENTIFIER --confirm [flags]",
		ShortHelp:  "Register a Website Push ID via a Developer Portal web session.",
		LongHelp: `Register a Website Push ID via a Developer Portal web session.

The captured request creates the resource with an explicitly empty
websitepushIdCapabilities relationship. This command refuses to write when
Apple reports a non-empty or unreadable capability catalog. A successful
create is verified by re-reading the returned resource; an uncertain write is
reported without an automatic retry.

Examples:
  asc web website-push-ids create --name "Example Website" --identifier "web.example.com" --confirm

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			request := webcore.DeveloperWebsitePushIDCreateRequest{Name: *name, Identifier: *identifier}
			if err := request.Validate(); err != nil {
				return shared.UsageError(websitePushIDCreateUsageError(err))
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web website-push-ids create")
			}
			var result *asc.WebWebsitePushIDMutationResult
			err = withWebSpinner("Registering Developer Portal Website Push ID", func() error {
				var createErr error
				result, createErr = createDeveloperWebsitePushIDFn(requestCtx, newDeveloperPortalClient(session, portalFlags), request)
				return createErr
			})
			if err != nil {
				return developerWebsitePushIDMutationError(session, err, "web website-push-ids create")
			}
			if result == nil {
				return fmt.Errorf("web website-push-ids create failed: missing create result")
			}
			persistDeveloperPortalSession(session)
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// WebWebsitePushIDsDeleteCommand deletes a Website Push ID after proving that
// it is deletable and has no attached capability references.
func WebWebsitePushIDsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web website-push-ids delete", flag.ExitOnError)
	websitePushID := fs.String("website-push-id", "", "Opaque Developer Portal Website Push ID resource ID")
	confirm := fs.Bool("confirm", false, "Confirm deleting this Website Push ID")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc web website-push-ids delete --website-push-id WEBSITE_PUSH_RESOURCE_ID --confirm [flags]",
		ShortHelp:  "Delete a Website Push ID via a Developer Portal web session.",
		LongHelp: `Delete a Website Push ID via a Developer Portal web session.

The command requires Apple's detail response to report canDelete=true and an
explicitly empty websitepushIdCapabilities relationship. The captured delete
is a POST with X-HTTP-Method-Override: DELETE and a JSON body containing only
the selected teamId. A successful 204 is verified by a canonical detail read
and the legacy Website Push ID list. An uncertain write is reported without an
automatic retry.

Example:
  asc web website-push-ids delete --website-push-id "WEBSITE_PUSH_RESOURCE_ID" --confirm

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			request := webcore.DeveloperWebsitePushIDDeleteRequest{WebsitePushID: *websitePushID}
			if err := request.Validate(); err != nil {
				return shared.UsageError("--website-push-id is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web website-push-ids delete")
			}
			var result *asc.WebWebsitePushIDMutationResult
			err = withWebSpinner("Deleting Developer Portal Website Push ID", func() error {
				var deleteErr error
				result, deleteErr = deleteDeveloperWebsitePushIDFn(requestCtx, newDeveloperPortalClient(session, portalFlags), request)
				return deleteErr
			})
			if err != nil {
				return developerWebsitePushIDMutationError(session, err, "web website-push-ids delete")
			}
			if result == nil {
				return fmt.Errorf("web website-push-ids delete failed: missing delete result")
			}
			persistDeveloperPortalSession(session)
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func developerWebsitePushIDMutationError(session *webcore.AuthSession, err error, command string) error {
	var unverified *webcore.DeveloperWebsitePushIDUnverifiedError
	if errors.As(err, &unverified) {
		// Keep subsequent inspection on the team whose write is uncertain.
		persistDeveloperPortalSession(session)
	}
	return withWebAuthHint(err, command)
}

func websitePushIDCreateUsageError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if strings.HasPrefix(message, "name ") {
		return "--" + message
	}
	if strings.HasPrefix(message, "identifier ") {
		return "--" + message
	}
	return message
}

// WebWebsitePushIDsListCommand lists Website Push IDs visible to the selected
// Developer Portal team. Apple’s legacy endpoint has no captured continuation
// contract, so this command requests its fixed first page only.
func WebWebsitePushIDsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web website-push-ids list", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web website-push-ids list [flags]",
		ShortHelp:  "List Website Push IDs via a Developer Portal web session.",
		LongHelp: `List Website Push IDs via a Developer Portal web session.

WEB SESSION WORKFLOWS

List the Website Push IDs visible to the selected Apple Developer team. The
command returns Apple's complete legacy response envelope in JSON. Formatted
output shows only a small scalar projection of each entry.

Examples:
  asc web website-push-ids list --output table
  asc web website-push-ids list --developer-team "TEAM_ID" --output json

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web website-push-ids list")
			}

			var result *webcore.DeveloperWebsitePushIDsListResult
			err = withWebSpinner("Loading Developer Portal Website Push IDs", func() error {
				var listErr error
				result, listErr = listDeveloperWebsitePushIDsFn(requestCtx, newDeveloperPortalClient(session, portalFlags))
				return listErr
			})
			if err != nil {
				return withWebAuthHint(err, "web website-push-ids list")
			}
			if result == nil {
				return fmt.Errorf("web website-push-ids list failed: missing list result")
			}
			persistDeveloperPortalSession(session)

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperWebsitePushIDsTable(result) },
				func() error { return renderDeveloperWebsitePushIDsMarkdown(result) },
			)
		},
	}
}

func developerWebsitePushIDsHeaders() []string {
	return []string{"Website Push ID", "Name", "Identifier"}
}

func developerWebsitePushIDsRows(result *webcore.DeveloperWebsitePushIDsListResult) [][]string {
	if result == nil {
		return nil
	}
	rows := make([][]string, 0, len(result.WebsitePushIDList))
	for _, entry := range result.WebsitePushIDList {
		rows = append(rows, []string{
			shared.OrNA(developerWebsitePushIDValue(entry, "websitePushId", "id")),
			shared.OrNA(developerWebsitePushIDValue(entry, "name")),
			shared.OrNA(developerWebsitePushIDValue(entry, "identifier")),
		})
	}
	return rows
}

func developerWebsitePushIDValue(entry webcore.DeveloperWebsitePushID, keys ...string) string {
	for _, key := range keys {
		value, ok := entry[key]
		if !ok || value == nil {
			continue
		}
		if text, ok := value.(string); ok {
			return strings.TrimSpace(text)
		}
		return fmt.Sprint(value)
	}
	return ""
}

func renderDeveloperWebsitePushIDsTable(result *webcore.DeveloperWebsitePushIDsListResult) error {
	var rows [][]string
	if result != nil {
		rows = developerWebsitePushIDsRows(result)
	}
	asc.RenderTable(developerWebsitePushIDsHeaders(), rows)
	return nil
}

func renderDeveloperWebsitePushIDsMarkdown(result *webcore.DeveloperWebsitePushIDsListResult) error {
	var rows [][]string
	if result != nil {
		rows = developerWebsitePushIDsRows(result)
	}
	asc.RenderMarkdown(developerWebsitePushIDsHeaders(), rows)
	return nil
}

func developerWebsitePushIDDetailHeaders() []string {
	return []string{"Website Push ID", "Name", "Identifier", "Can Delete", "Can Edit"}
}

func developerWebsitePushIDDetailRows(resource webcore.DeveloperWebsitePushIDResource) [][]string {
	return [][]string{{
		shared.OrNA(resource.ID),
		shared.OrNA(websitePushIDResourceStringAttribute(resource.Attributes, "name")),
		shared.OrNA(websitePushIDResourceStringAttribute(resource.Attributes, "identifier")),
		shared.OrNA(websitePushIDResourceBoolAttribute(resource.Attributes, "canDelete")),
		shared.OrNA(websitePushIDResourceBoolAttribute(resource.Attributes, "canEdit")),
	}}
}

func renderDeveloperWebsitePushIDTable(result *webcore.DeveloperWebsitePushIDGetResult) error {
	if result == nil {
		asc.RenderTable(developerWebsitePushIDDetailHeaders(), nil)
		return nil
	}
	warnDeveloperWebsitePushIDIncludedOutput(result)
	asc.RenderTable(developerWebsitePushIDDetailHeaders(), developerWebsitePushIDDetailRows(result.Data))
	return nil
}

func renderDeveloperWebsitePushIDMarkdown(result *webcore.DeveloperWebsitePushIDGetResult) error {
	if result == nil {
		asc.RenderMarkdown(developerWebsitePushIDDetailHeaders(), nil)
		return nil
	}
	warnDeveloperWebsitePushIDIncludedOutput(result)
	asc.RenderMarkdown(developerWebsitePushIDDetailHeaders(), developerWebsitePushIDDetailRows(result.Data))
	return nil
}

func warnDeveloperWebsitePushIDIncludedOutput(result *webcore.DeveloperWebsitePushIDGetResult) {
	if result == nil || len(result.Included) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, "Warning: table or Markdown output omits included Website Push ID resources; use --output json to inspect the complete capability graph.")
}

func websitePushIDResourceStringAttribute(attributes map[string]any, key string) string {
	value, ok := attributes[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func websitePushIDResourceBoolAttribute(attributes map[string]any, key string) string {
	value, ok := attributes[key]
	if !ok || value == nil {
		return ""
	}
	boolean, ok := value.(bool)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%t", boolean)
}
