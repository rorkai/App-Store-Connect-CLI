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

var listDeveloperServiceIDsFn = func(ctx context.Context, client *webcore.Client) (*webcore.DeveloperServiceIDsListResult, error) {
	return client.ListDeveloperServiceIDs(ctx)
}

var getDeveloperServiceIDFn = func(ctx context.Context, client *webcore.Client, serviceID string) (*webcore.DeveloperServiceIDGetResult, error) {
	return client.GetDeveloperServiceID(ctx, serviceID)
}

var createDeveloperServiceIDFn = func(ctx context.Context, client *webcore.Client, request webcore.DeveloperServiceIDCreateRequest) (*asc.WebServiceIDMutationResult, error) {
	return client.CreateDeveloperServiceID(ctx, request)
}

var renameDeveloperServiceIDFn = func(ctx context.Context, client *webcore.Client, request webcore.DeveloperServiceIDRenameRequest) (*asc.WebServiceIDMutationResult, error) {
	return client.RenameDeveloperServiceID(ctx, request)
}

var deleteDeveloperServiceIDFn = func(ctx context.Context, client *webcore.Client, request webcore.DeveloperServiceIDDeleteRequest) (*asc.WebServiceIDMutationResult, error) {
	return client.DeleteDeveloperServiceID(ctx, request)
}

// WebServiceIDsCommand returns the private Developer Portal Services ID
// command group. Services IDs use Apple's private bundleIds endpoint with the
// SERVICES platform and are distinct from public App Store Connect Bundle IDs.
func WebServiceIDsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web service-ids", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "service-ids",
		ShortUsage: "asc web service-ids <subcommand> [flags]",
		ShortHelp:  "Manage Services IDs via Developer Portal web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

List, inspect, register, rename, and delete Services IDs through Apple's
cookie-authenticated Developer Portal. These private resources are represented
by Apple's bundleIds endpoint with platform=SERVICES and are not part of the
public App Store Connect Bundle ID API.

Capability configuration, Website Push IDs, and iCloud containers are separate
workflows. domains set validates a Sign in with Apple domain update and then
stops, because that write request has not been captured.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebServiceIDsListCommand(),
			WebServiceIDsViewCommand(),
			WebServiceIDsCreateCommand(),
			WebServiceIDsRenameCommand(),
			WebServiceIDsDeleteCommand(),
			WebServiceIDsDomainsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebServiceIDsListCommand lists private Services IDs for the selected team.
func WebServiceIDsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web service-ids list", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web service-ids list [flags]",
		ShortHelp:  "List Services IDs via a Developer Portal web session.",
		LongHelp: `List Services IDs visible to the selected Developer Portal team.

The command requests Apple's captured 1000-resource collection with
filter[platform]=SERVICES. JSON output preserves the complete JSON:API envelope
returned by the Developer Portal web-session request.

Example:
  asc web service-ids list --output json
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web service-ids list does not accept positional arguments")
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
				return withWebAuthHint(err, "web service-ids list")
			}
			var result *webcore.DeveloperServiceIDsListResult
			err = withWebSpinner("Loading Developer Portal Services IDs", func() error {
				var listErr error
				result, listErr = listDeveloperServiceIDsFn(requestCtx, newDeveloperPortalClient(session, portalFlags))
				return listErr
			})
			if err != nil {
				return withWebAuthHint(err, "web service-ids list")
			}
			if result == nil {
				return fmt.Errorf("web service-ids list failed: missing list result")
			}
			persistDeveloperPortalSession(session)
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperServiceIDsTable(result) },
				func() error { return renderDeveloperServiceIDsMarkdown(result) },
			)
		},
	}
}

// WebServiceIDsViewCommand reads one private Services ID and its capability
// graph without changing it.
func WebServiceIDsViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web service-ids view", flag.ExitOnError)
	serviceID := fs.String("service-id", "", "Opaque Developer Portal Services ID resource ID")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc web service-ids view --service-id SERVICE_RESOURCE_ID [flags]",
		ShortHelp:  "Inspect one Services ID via a Developer Portal web session.",
		LongHelp: `Inspect one opaque Services ID resource and its included capability
relationships. The command rejects a Bundle ID whose platform is not SERVICES.
Use --output json to retain the complete capability graph.

Example:
  asc web service-ids view --service-id "SERVICE_RESOURCE_ID" --output json
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web service-ids view does not accept positional arguments")
			}
			resolvedID := strings.TrimSpace(*serviceID)
			if resolvedID == "" {
				return shared.UsageError("--service-id is required")
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
				return withWebAuthHint(err, "web service-ids view")
			}
			var result *webcore.DeveloperServiceIDGetResult
			err = withWebSpinner("Loading Developer Portal Services ID", func() error {
				var getErr error
				result, getErr = getDeveloperServiceIDFn(requestCtx, newDeveloperPortalClient(session, portalFlags), resolvedID)
				return getErr
			})
			if err != nil {
				return withWebAuthHint(err, "web service-ids view")
			}
			if result == nil {
				return fmt.Errorf("web service-ids view failed: missing view result")
			}
			persistDeveloperPortalSession(session)
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperServiceIDTable(result) },
				func() error { return renderDeveloperServiceIDMarkdown(result) },
			)
		},
	}
}

// WebServiceIDsCreateCommand registers a minimal Services ID.
func WebServiceIDsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web service-ids create", flag.ExitOnError)
	identifier := fs.String("identifier", "", "Services ID identifier")
	name := fs.String("name", "", "Human-readable Services ID name")
	confirm := fs.Bool("confirm", false, "Confirm registering this Services ID")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web service-ids create --identifier IDENTIFIER --name NAME --confirm [flags]",
		ShortHelp:  "Register a Services ID via a Developer Portal web session.",
		LongHelp: `Register a minimal Services ID for the selected Developer Portal team.

This command creates the resource with platform=SERVICES and an empty
bundleIdCapabilities relationship. Configure capabilities and Sign in with
Apple settings through a separately captured workflow.

--confirm is required because this command changes the Developer Portal
account.

Example:
  asc web service-ids create --identifier "com.example.service" --name "Example Service" --confirm
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web service-ids create does not accept positional arguments")
			}
			resolvedIdentifier := strings.TrimSpace(*identifier)
			resolvedName := strings.TrimSpace(*name)
			if resolvedIdentifier == "" {
				return shared.UsageError("--identifier is required")
			}
			if resolvedName == "" {
				return shared.UsageError("--name is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
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
				return withWebAuthHint(err, "web service-ids create")
			}
			var result *asc.WebServiceIDMutationResult
			err = withWebSpinner("Registering Developer Portal Services ID", func() error {
				var createErr error
				result, createErr = createDeveloperServiceIDFn(requestCtx, newDeveloperPortalClient(session, portalFlags), webcore.DeveloperServiceIDCreateRequest{
					Identifier: resolvedIdentifier,
					Name:       resolvedName,
				})
				return createErr
			})
			persistDeveloperPortalSession(session)
			if err != nil {
				return withWebAuthHint(err, "web service-ids create")
			}
			if result == nil {
				return fmt.Errorf("web service-ids create failed: missing create result")
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperServiceIDMutationTable(result) },
				func() error { return renderDeveloperServiceIDMutationMarkdown(result) },
			)
		},
	}
}

// WebServiceIDsRenameCommand renames a Services ID while preserving its
// capability graph.
func WebServiceIDsRenameCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web service-ids rename", flag.ExitOnError)
	serviceID := fs.String("service-id", "", "Opaque Developer Portal Services ID resource ID")
	name := fs.String("name", "", "New human-readable Services ID name")
	confirm := fs.Bool("confirm", false, "Confirm renaming this Services ID")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "rename",
		ShortUsage: "asc web service-ids rename --service-id SERVICE_RESOURCE_ID --name NAME --confirm [flags]",
		ShortHelp:  "Rename a Services ID via a Developer Portal web session.",
		LongHelp: `Rename one Services ID after reading and validating its current
platform. The PATCH carries the current capability relationships forward and
changes only the name plus Apple's required private team attribute.

--confirm is required. The command re-reads the resource after PATCH and fails
with an unknown outcome when the write or verification cannot be settled.

Example:
  asc web service-ids rename --service-id "SERVICE_RESOURCE_ID" --name "New Name" --confirm
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web service-ids rename does not accept positional arguments")
			}
			resolvedID := strings.TrimSpace(*serviceID)
			resolvedName := strings.TrimSpace(*name)
			if resolvedID == "" {
				return shared.UsageError("--service-id is required")
			}
			if resolvedName == "" {
				return shared.UsageError("--name is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
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
				return withWebAuthHint(err, "web service-ids rename")
			}
			var result *asc.WebServiceIDMutationResult
			err = withWebSpinner("Renaming Developer Portal Services ID", func() error {
				var renameErr error
				result, renameErr = renameDeveloperServiceIDFn(requestCtx, newDeveloperPortalClient(session, portalFlags), webcore.DeveloperServiceIDRenameRequest{
					ServiceID: resolvedID,
					Name:      resolvedName,
				})
				return renameErr
			})
			persistDeveloperPortalSession(session)
			if err != nil {
				return withWebAuthHint(err, "web service-ids rename")
			}
			if result == nil {
				return fmt.Errorf("web service-ids rename failed: missing rename result")
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperServiceIDMutationTable(result) },
				func() error { return renderDeveloperServiceIDMutationMarkdown(result) },
			)
		},
	}
}

// WebServiceIDsDeleteCommand deletes a Services ID after a platform-checked
// preflight and post-delete detail read.
func WebServiceIDsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web service-ids delete", flag.ExitOnError)
	serviceID := fs.String("service-id", "", "Opaque Developer Portal Services ID resource ID")
	confirm := fs.Bool("confirm", false, "Confirm deleting this Services ID")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc web service-ids delete --service-id SERVICE_RESOURCE_ID --confirm [flags]",
		ShortHelp:  "Delete a Services ID via a Developer Portal web session.",
		LongHelp: `Delete one Services ID after proving that its resource platform is
SERVICES. A post-delete detail read must return 404 before the command reports
success. A 5xx, transport failure, or failed verification is an unknown outcome;
inspect the resource before retrying.

--confirm is required.

Example:
  asc web service-ids delete --service-id "SERVICE_RESOURCE_ID" --confirm
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web service-ids delete does not accept positional arguments")
			}
			resolvedID := strings.TrimSpace(*serviceID)
			if resolvedID == "" {
				return shared.UsageError("--service-id is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
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
				return withWebAuthHint(err, "web service-ids delete")
			}
			var result *asc.WebServiceIDMutationResult
			err = withWebSpinner("Deleting Developer Portal Services ID", func() error {
				var deleteErr error
				result, deleteErr = deleteDeveloperServiceIDFn(requestCtx, newDeveloperPortalClient(session, portalFlags), webcore.DeveloperServiceIDDeleteRequest{ServiceID: resolvedID})
				return deleteErr
			})
			persistDeveloperPortalSession(session)
			if err != nil {
				return withWebAuthHint(err, "web service-ids delete")
			}
			if result == nil {
				return fmt.Errorf("web service-ids delete failed: missing delete result")
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperServiceIDMutationTable(result) },
				func() error { return renderDeveloperServiceIDMutationMarkdown(result) },
			)
		},
	}
}

func renderDeveloperServiceIDsTable(result *webcore.DeveloperServiceIDsListResult) error {
	headers := developerBundleIDHeaders()
	headers[0] = "Service ID"
	if result == nil {
		asc.RenderTable(headers, nil)
		return nil
	}
	asc.RenderTable(headers, developerBundleIDRows(result.Data))
	return nil
}

func renderDeveloperServiceIDsMarkdown(result *webcore.DeveloperServiceIDsListResult) error {
	headers := developerBundleIDHeaders()
	headers[0] = "Service ID"
	if result == nil {
		asc.RenderMarkdown(headers, nil)
		return nil
	}
	asc.RenderMarkdown(headers, developerBundleIDRows(result.Data))
	return nil
}

func renderDeveloperServiceIDTable(result *webcore.DeveloperServiceIDGetResult) error {
	headers := developerBundleIDDetailHeaders()
	headers[0] = "Service ID"
	if result == nil {
		asc.RenderTable(headers, nil)
		return nil
	}
	if len(result.Included) > 0 {
		fmt.Fprintln(os.Stderr, "Warning: table or Markdown output omits included Services ID resources; use --output json to inspect the complete capability graph.")
	}
	asc.RenderTable(headers, developerBundleIDDetailRows(result.Data))
	return nil
}

func renderDeveloperServiceIDMarkdown(result *webcore.DeveloperServiceIDGetResult) error {
	headers := developerBundleIDDetailHeaders()
	headers[0] = "Service ID"
	if result == nil {
		asc.RenderMarkdown(headers, nil)
		return nil
	}
	if len(result.Included) > 0 {
		fmt.Fprintln(os.Stderr, "Warning: table or Markdown output omits included Services ID resources; use --output json to inspect the complete capability graph.")
	}
	asc.RenderMarkdown(headers, developerBundleIDDetailRows(result.Data))
	return nil
}

func renderDeveloperServiceIDMutationTable(result *asc.WebServiceIDMutationResult) error {
	asc.RenderTable(webServiceIDMutationHeaders(), webServiceIDMutationRows(result))
	return nil
}

func renderDeveloperServiceIDMutationMarkdown(result *asc.WebServiceIDMutationResult) error {
	asc.RenderMarkdown(webServiceIDMutationHeaders(), webServiceIDMutationRows(result))
	return nil
}

// WebServiceIDsDomainsCommand groups Sign in with Apple domain configuration.
func WebServiceIDsDomainsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web service-ids domains", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "domains",
		ShortUsage: "asc web service-ids domains <subcommand> [flags]",
		ShortHelp:  "Sign in with Apple domain configuration for a Services ID.",
		LongHelp: `Sign in with Apple domain configuration for a Services ID.

The set command validates its flags and then stops. No accepted domain write
request has been captured, so it does not call Apple.
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebServiceIDsDomainsSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebServiceIDsDomainsSetCommand refuses a domain update until the request is captured.
func WebServiceIDsDomainsSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web service-ids domains set", flag.ExitOnError)
	serviceID := fs.String("service-id", "", "Services ID resource ID")
	domain := fs.String("domain", "", "Domain to associate, comma-separated")
	returnURL := fs.String("return-url", "", "Return URL, comma-separated")
	confirm := fs.Bool("confirm", false, "Confirm the domain update")
	_ = bindWebSessionFlags(fs)
	_ = bindDeveloperPortalFlags(fs)
	_ = shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc web service-ids domains set --service-id ID --domain DOMAIN --return-url URL --confirm",
		ShortHelp:  "Refuse a Sign in with Apple domain update until a write request is captured.",
		LongHelp: `Validate a Sign in with Apple domain update and stop.

No accepted domain write request has been captured. The command checks
--service-id, --domain, --return-url, and --confirm, then returns an error. It
does not open a session and does not call Apple.

Examples:
  asc web service-ids domains set --service-id "SERVICE_ID" --domain "example.com" --return-url "https://example.com/callback" --confirm
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web service-ids domains set does not accept positional arguments")
			}
			if strings.TrimSpace(*serviceID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --service-id is required")
				return shared.MissingRequiredUsageError("--service-id")
			}
			if strings.TrimSpace(*domain) == "" {
				fmt.Fprintln(os.Stderr, "Error: --domain is required")
				return shared.MissingRequiredUsageError("--domain")
			}
			if strings.TrimSpace(*returnURL) == "" {
				fmt.Fprintln(os.Stderr, "Error: --return-url is required")
				return shared.MissingRequiredUsageError("--return-url")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}
			fmt.Fprintln(os.Stderr, "Error: web service-ids domains set is not available: no accepted write request has been captured")
			return errors.New("web service-ids domains set is not available: no accepted write request has been captured")
		},
	}
}

func webServiceIDMutationHeaders() []string {
	return []string{"Operation", "Service ID", "Identifier", "Name", "Changed", "Verified", "Status"}
}

func webServiceIDMutationRows(result *asc.WebServiceIDMutationResult) [][]string {
	if result == nil {
		return nil
	}
	return [][]string{{
		result.Operation,
		result.ServiceID,
		result.Identifier,
		result.Name,
		fmt.Sprintf("%t", result.Changed),
		fmt.Sprintf("%t", result.Verified),
		result.Status,
	}}
}
