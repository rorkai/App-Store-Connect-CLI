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

var listDeveloperBundleIDCapabilitiesFn = func(ctx context.Context, client *webcore.Client, bundleID string) (*webcore.DeveloperBundleIDCapabilitiesListResult, error) {
	return client.ListDeveloperBundleIDCapabilities(ctx, bundleID)
}

func newWebBundleIDCapabilitiesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web bundle-ids capabilities list", flag.ExitOnError)
	bundleID := fs.String("bundle-id", "", "Opaque Developer Portal Bundle ID resource ID")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortUsage: "asc web bundle-ids capabilities list --bundle-id BUNDLE_RESOURCE_ID [flags]",
		ShortHelp: "List the Developer Portal capability catalog.",
		LongHelp: `List the Developer Portal capability catalog.

WEB SESSION WORKFLOWS

Read capability IDs, names, settings, and editability from Apple's team catalog.
The Bundle ID supplies portal context. The catalog includes requestable entries,
so an entry does not prove approval or that the capability is enabled on the App ID.
Use "asc web bundle-ids view" to inspect the App ID's current capabilities.
JSON preserves Apple's complete response envelope.

Examples:
  asc web bundle-ids capabilities list --bundle-id "BUNDLE_RESOURCE_ID" --output table
  asc web bundle-ids capabilities list --bundle-id "BUNDLE_RESOURCE_ID" --output json
`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedBundleID := strings.TrimSpace(*bundleID)
			if resolvedBundleID == "" {
				return d.UsageError("--bundle-id is required")
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
				return withWebAuthHint(err, "web bundle-ids capabilities list")
			}
			var result *webcore.DeveloperBundleIDCapabilitiesListResult
			err = withWebSpinnerWith(d, "Loading Developer Portal capabilities", func() error {
				var listErr error
				result, listErr = listDeveloperBundleIDCapabilitiesFn(requestCtx, newDeveloperPortalClient(session, portalFlags), resolvedBundleID)
				return listErr
			})
			if err != nil {
				return withWebAuthHint(err, "web bundle-ids capabilities list")
			}
			if result == nil {
				return fmt.Errorf("web bundle-ids capabilities list failed: missing result")
			}
			persistDeveloperPortalSessionWith(d, session)
			headers := []string{"ID", "Name", "Editable"}
			rows := make([][]string, 0, len(result.Data))
			for _, capability := range result.Data {
				editable := ""
				if capability.Attributes.Editable != nil {
					editable = strconv.FormatBool(*capability.Attributes.Editable)
				}
				rows = append(rows, []string{
					shared.OrNA(capability.ID), shared.OrNA(capability.Attributes.Name), shared.OrNA(editable),
				})
			}
			return d.PrintOutputWithRenderers(result, *output.Output, *output.Pretty,
				func() error { asc.RenderTableTo(d.IO.Out, headers, rows); return nil },
				func() error { asc.RenderMarkdownTo(d.IO.Out, headers, rows); return nil })
		},
	}
}
