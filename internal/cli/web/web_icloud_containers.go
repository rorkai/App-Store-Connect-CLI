package web

import (
	"context"
	"flag"
	"fmt"
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

var createDeveloperICloudContainerFn = func(ctx context.Context, client *webcore.Client, request webcore.DeveloperICloudContainerCreateRequest) (*asc.WebICloudContainerCreateResult, error) {
	return client.CreateDeveloperICloudContainer(ctx, request)
}

// newWebICloudContainersCommand returns the Developer Portal iCloud container
// command group.
func newWebICloudContainersCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web icloud-containers", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "icloud-containers",
		ShortUsage: "asc web icloud-containers <subcommand> [flags]",
		ShortHelp:  "List and create iCloud containers via a Developer Portal web session.",
		LongHelp: `List and create iCloud containers for the selected Apple Developer team.

list reads a bounded first page and does not expose --paginate. create
registers a new container. iCloud containers can never be deleted, so create
requires --confirm and cannot be undone.
`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebICloudContainersListCommand(d),
			newWebICloudContainersCreateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newWebICloudContainersListCommand lists iCloud containers for the selected
// Developer Portal team.
func newWebICloudContainersListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web icloud-containers list", flag.ExitOnError)
	hidden := fs.Bool("hidden", false, "List hidden iCloud containers instead of visible containers")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web icloud-containers list [--hidden] [flags]",
		ShortHelp:  "List iCloud containers via a Developer Portal web session.",
		LongHelp: `List iCloud containers visible to the selected Apple Developer team.

Visible containers are returned by default. Pass --hidden to request the hidden
collection. The request asks Apple for up to 1000 resources and does not expose
--paginate; any links or paging metadata Apple returns remain available in JSON.
This command does not rename, delete, or inspect individual containers.

Examples:
  asc web icloud-containers list --output table
  asc web icloud-containers list --hidden --output json
`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateDeveloperPortalFlagsWith(d, portalFlags); err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web icloud-containers list")
			}

			var result *webcore.DeveloperICloudContainersListResult
			err = withWebSpinnerWith(d, "Loading Developer Portal iCloud containers", func() error {
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
			persistDeveloperPortalSessionWith(d, session)

			warnICloudContainerPagingTotal(d, result)
			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperICloudContainersTable(d, result) },
				func() error { return renderDeveloperICloudContainersMarkdown(d, result) },
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

func renderDeveloperICloudContainersTable(d *shared.Deps, result *webcore.DeveloperICloudContainersListResult) error {
	if result == nil {
		asc.RenderTableTo(d.IO.Out, developerICloudContainersHeaders(), nil)
		return nil
	}
	asc.RenderTableTo(d.IO.Out, developerICloudContainersHeaders(), developerICloudContainersRows(result.Data))
	return nil
}

func renderDeveloperICloudContainersMarkdown(d *shared.Deps, result *webcore.DeveloperICloudContainersListResult) error {
	if result == nil {
		asc.RenderMarkdownTo(d.IO.Out, developerICloudContainersHeaders(), nil)
		return nil
	}
	asc.RenderMarkdownTo(d.IO.Out, developerICloudContainersHeaders(), developerICloudContainersRows(result.Data))
	return nil
}

// newWebICloudContainersCreateCommand registers one permanent iCloud container.
func newWebICloudContainersCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web icloud-containers create", flag.ExitOnError)
	identifier := fs.String("identifier", "", "iCloud container identifier; must start with iCloud. (for example iCloud.com.example.app)")
	name := fs.String("name", "", "Container display name")
	confirm := fs.Bool("confirm", false, "Confirm creating a permanent iCloud container")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web icloud-containers create --identifier iCloud.ID --name NAME --confirm [flags]",
		ShortHelp:  "Create a permanent iCloud container via a Developer Portal web session.",
		LongHelp: `Create one iCloud container for the selected Apple Developer team.

iCloud containers can never be deleted. Apple keeps the identifier reserved
for your team permanently, so check the identifier before you run this.

--identifier must start with "iCloud." followed by a reverse-DNS string of
letters, digits, hyphens, and dots, for example iCloud.com.example.app. The
CLI does not add the prefix for you.

The command reads the visible and hidden containers first and refuses an
identifier that already exists. After Apple accepts the create, it reads the
collections again and prints a receipt only when the new container is found.
If the outcome cannot be verified, run asc web icloud-containers list before
retrying; the create request is never retried automatically.

Examples:
  asc web icloud-containers create --identifier "iCloud.com.example.app" --name "Example" --confirm
  asc web icloud-containers create --identifier "iCloud.com.example.app" --name "Example" --confirm --output json
`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedIdentifier := strings.TrimSpace(*identifier)
			resolvedName := strings.TrimSpace(*name)
			if resolvedIdentifier == "" {
				return d.UsageError("--identifier is required")
			}
			if err := webcore.ValidateDeveloperICloudContainerIdentifier(resolvedIdentifier); err != nil {
				return d.UsageError(err.Error())
			}
			if resolvedName == "" {
				return d.UsageError("--name is required")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}
			if err := validateDeveloperPortalFlagsWith(d, portalFlags); err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web icloud-containers create")
			}

			var result *asc.WebICloudContainerCreateResult
			err = withWebSpinnerWith(d, "Creating Developer Portal iCloud container", func() error {
				var createErr error
				result, createErr = createDeveloperICloudContainerFn(requestCtx, newDeveloperPortalClient(session, portalFlags), webcore.DeveloperICloudContainerCreateRequest{
					Identifier: resolvedIdentifier,
					Name:       resolvedName,
				})
				return createErr
			})
			persistDeveloperPortalSessionWith(d, session)
			if err != nil {
				return withWebAuthHint(err, "web icloud-containers create")
			}
			if result == nil {
				return fmt.Errorf("web icloud-containers create failed: missing create result")
			}
			if result.RequestedName != "" {
				fmt.Fprintf(d.IO.ErrOut, "Warning: Apple stored the container name as %q instead of the requested %q\n", result.Name, result.RequestedName)
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// Shared output warns for links.next; this covers totals without a next link.
func warnICloudContainerPagingTotal(d *shared.Deps, result *webcore.DeveloperICloudContainersListResult) {
	if result.GetLinks().Next != "" {
		return
	}
	if total, ok := asc.ParsePagingTotalOK(result.GetMeta()); ok && total > len(result.Data) {
		fmt.Fprintf(d.IO.ErrOut, "Warning: showing %d of %d results; this command reads only the first page\n", len(result.Data), total)
	}
}
