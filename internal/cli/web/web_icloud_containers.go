package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var listDeveloperICloudContainersFn = func(ctx context.Context, client *webcore.Client, hidden bool) (*webcore.DeveloperICloudContainersListResult, error) {
	return client.ListDeveloperICloudContainers(ctx, hidden)
}

// WebICloudContainersCommand returns the read-only Developer Portal iCloud
// container command group.
func WebICloudContainersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web icloud-containers", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "icloud-containers",
		ShortUsage: "asc web icloud-containers <subcommand> [flags]",
		ShortHelp:  "Read iCloud containers via a Developer Portal web session.",
		LongHelp: `Read iCloud containers through the selected Apple Developer team.

List reads a bounded first page and does not expose --paginate. create
validates its flags and then stops. No iCloud container write request has been
captured, so create does not call Apple.
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebICloudContainersListCommand(),
			WebICloudContainersCreateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebICloudContainersListCommand lists iCloud containers for the selected
// Developer Portal team.
func WebICloudContainersListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web icloud-containers list", flag.ExitOnError)
	hidden := fs.Bool("hidden", false, "List hidden iCloud containers instead of visible containers")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web icloud-containers list [--hidden] [flags]",
		ShortHelp:  "List iCloud containers via a Developer Portal web session.",
		LongHelp: `List iCloud containers visible to the selected Apple Developer team.

Visible containers are returned by default. Pass --hidden to request the hidden
collection. The request asks Apple for up to 1000 resources and does not expose
--paginate; any links or paging metadata Apple returns remain available in JSON.
This command does not create, rename, delete, or inspect individual containers.

Examples:
  asc web icloud-containers list --output table
  asc web icloud-containers list --hidden --output json
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web icloud-containers list does not accept positional arguments")
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
				return withWebAuthHint(err, "web icloud-containers list")
			}

			var result *webcore.DeveloperICloudContainersListResult
			err = withWebSpinner("Loading Developer Portal iCloud containers", func() error {
				var listErr error
				result, listErr = listDeveloperICloudContainersFn(requestCtx, newDeveloperPortalClient(session, portalFlags), *hidden)
				return listErr
			})
			if err != nil {
				return withWebAuthHint(err, "web icloud-containers list")
			}
			if result == nil {
				return fmt.Errorf("web icloud-containers list failed: missing list result")
			}
			persistDeveloperPortalSession(session)

			warnICloudContainerPagingTotal(result)
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperICloudContainersTable(result) },
				func() error { return renderDeveloperICloudContainersMarkdown(result) },
			)
		},
	}
}

func developerICloudContainersHeaders() []string {
	return []string{"ID", "Name", "Identifier", "Prefix", "Hidden", "Can Edit", "Can Delete", "Response ID"}
}

func developerICloudContainersRows(containers []webcore.DeveloperICloudContainer) [][]string {
	rows := make([][]string, 0, len(containers))
	for _, container := range containers {
		attributes := container.Attributes
		rows = append(rows, []string{
			shared.OrNA(container.ID),
			shared.OrNA(attributes.Name),
			shared.OrNA(attributes.Identifier),
			shared.OrNA(attributes.Prefix),
			strconv.FormatBool(attributes.Hidden),
			strconv.FormatBool(attributes.CanEdit),
			strconv.FormatBool(attributes.CanDelete),
			shared.OrNA(attributes.ResponseID),
		})
	}
	return rows
}

func renderDeveloperICloudContainersTable(result *webcore.DeveloperICloudContainersListResult) error {
	if result == nil {
		asc.RenderTable(developerICloudContainersHeaders(), nil)
		return nil
	}
	asc.RenderTable(developerICloudContainersHeaders(), developerICloudContainersRows(result.Data))
	return nil
}

func renderDeveloperICloudContainersMarkdown(result *webcore.DeveloperICloudContainersListResult) error {
	if result == nil {
		asc.RenderMarkdown(developerICloudContainersHeaders(), nil)
		return nil
	}
	asc.RenderMarkdown(developerICloudContainersHeaders(), developerICloudContainersRows(result.Data))
	return nil
}

// Shared output warns for links.next; this covers totals without a next link.
// WebICloudContainersCreateCommand validates a create request and refuses it.
// The Developer Portal list response does not establish a create body, so this
// command does not send one.
func WebICloudContainersCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web icloud-containers create", flag.ExitOnError)
	identifier := fs.String("identifier", "", "iCloud container identifier (for example iCloud.com.example.app)")
	name := fs.String("name", "", "Container display name")
	confirm := fs.Bool("confirm", false, "Confirm creation")
	_ = bindWebSessionFlags(fs)
	_ = bindDeveloperPortalFlags(fs)
	_ = shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web icloud-containers create --identifier ID --name NAME --confirm",
		ShortHelp:  "Refuse iCloud container creation until a write request is captured.",
		LongHelp: `Validate an iCloud container create request and stop.

No accepted create request has been captured. The command checks --identifier,
--name, and --confirm, then returns an error. It does not open a session and
does not call Apple.

Examples:
  asc web icloud-containers create --identifier "iCloud.com.example.app" --name "Example" --confirm
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web icloud-containers create does not accept positional arguments")
			}
			if strings.TrimSpace(*identifier) == "" {
				fmt.Fprintln(os.Stderr, "Error: --identifier is required")
				return shared.MissingRequiredUsageError("--identifier")
			}
			if strings.TrimSpace(*name) == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return shared.MissingRequiredUsageError("--name")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}
			fmt.Fprintln(os.Stderr, "Error: web icloud-containers create is not available: no accepted write request has been captured")
			return errors.New("web icloud-containers create is not available: no accepted write request has been captured")
		},
	}
}

func warnICloudContainerPagingTotal(result *webcore.DeveloperICloudContainersListResult) {
	if result.GetLinks().Next != "" {
		return
	}
	if total, ok := asc.ParsePagingTotalOK(result.GetMeta()); ok && total > len(result.Data) {
		fmt.Fprintf(os.Stderr, "Warning: showing %d of %d results; this command reads only the first page\n", len(result.Data), total)
	}
}
