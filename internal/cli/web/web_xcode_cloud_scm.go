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

func webXcodeCloudScmCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web xcode-cloud scm", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "scm",
		ShortUsage: "asc web xcode-cloud scm <subcommand> [flags]",
		ShortHelp:  "Inspect Xcode Cloud SCM connections.",
		LongHelp: `WEB SESSION WORKFLOWS

Inspect SCM provider and connection metadata available to the authenticated
Xcode Cloud web session. These commands only read provider state.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			webXcodeCloudScmProvidersCommand(d),
			webXcodeCloudScmConnectionStatusCommand(d),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func webXcodeCloudScmProvidersCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web xcode-cloud scm providers", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "providers",
		ShortUsage: "asc web xcode-cloud scm providers <subcommand> [flags]",
		ShortHelp:  "Inspect Xcode Cloud SCM providers.",
		LongHelp: `WEB SESSION WORKFLOWS

Inspect SCM providers available to the authenticated Xcode Cloud web session.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			webXcodeCloudScmProvidersListCommand(d),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func webXcodeCloudScmProvidersListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web xcode-cloud scm providers list", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web xcode-cloud scm providers list [flags]",
		ShortHelp:  "List Xcode Cloud SCM providers.",
		LongHelp: `WEB SESSION WORKFLOWS

List SCM providers available to the authenticated Xcode Cloud web session.
The response is a plain Apple JSON array and does not support pagination.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			teamID, err := webXcodeCloudScmTeamID(session, "providers list")
			if err != nil {
				return err
			}

			result, err := withWebSpinnerValueWith(d, "Loading Xcode Cloud SCM providers", func() ([]webcore.CIScmProvider, error) {
				return newCIClientFn(session).GetCIScmProviders(requestCtx, teamID)
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud scm providers list")
			}
			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderCIScmProvidersTable(d, result) },
				func() error { return renderCIScmProvidersMarkdown(d, result) },
			)
		},
	}
}

func webXcodeCloudScmConnectionStatusCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web xcode-cloud scm connection-status", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)
	scmProviderID := fs.String("scm-provider-id", "", "Private Xcode Cloud SCM provider ID (required)")

	return &ffcli.Command{
		Name:       "connection-status",
		ShortUsage: "asc web xcode-cloud scm connection-status --scm-provider-id ID [flags]",
		ShortHelp:  "Show an Xcode Cloud SCM connection status.",
		LongHelp: `WEB SESSION WORKFLOWS

Show the connection status for one SCM provider returned by
"scm providers list". This command only reads the selected provider state.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			providerID := strings.TrimSpace(*scmProviderID)
			if providerID == "" {
				return d.RequiredFlag("--scm-provider-id")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			teamID, err := webXcodeCloudScmTeamID(session, "connection-status")
			if err != nil {
				return err
			}

			result, err := withWebSpinnerValueWith(d, "Loading Xcode Cloud SCM connection status", func() (*webcore.CIScmConnectionStatus, error) {
				return newCIClientFn(session).GetCIScmConnectionStatus(requestCtx, teamID, providerID)
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud scm connection-status")
			}
			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderCIScmConnectionStatusTable(d, result, providerID) },
				func() error { return renderCIScmConnectionStatusMarkdown(d, result, providerID) },
			)
		},
	}
}

func webXcodeCloudScmTeamID(session *webcore.AuthSession, operation string) (string, error) {
	if session == nil {
		return "", fmt.Errorf("xcode-cloud scm %s failed: web session is unavailable", operation)
	}
	teamID := strings.TrimSpace(session.PublicProviderID)
	if teamID == "" {
		return "", fmt.Errorf("xcode-cloud scm %s failed: session has no public provider ID", operation)
	}
	return teamID, nil
}

func renderCIScmProvidersTable(d *shared.Deps, result []webcore.CIScmProvider) error {
	asc.RenderTableTo(d.IO.Out, ciScmProviderHeaders(), ciScmProviderRows(result))
	return nil
}

func renderCIScmProvidersMarkdown(d *shared.Deps, result []webcore.CIScmProvider) error {
	asc.RenderMarkdownTo(d.IO.Out, ciScmProviderHeaders(), ciScmProviderRows(result))
	return nil
}

func ciScmProviderHeaders() []string {
	return []string{"ID", "Provider", "Name", "Registered", "Connected"}
}

func ciScmProviderRows(result []webcore.CIScmProvider) [][]string {
	rows := make([][]string, 0, len(result))
	for _, provider := range result {
		rows = append(rows, []string{
			valueOrNA(provider.ID),
			valueOrNA(provider.Provider),
			valueOrNA(provider.ProviderDisplayName),
			formatCIScmBool(provider.IsRegistered),
			formatCIScmBool(provider.IsUserConnected),
		})
	}
	return rows
}

func renderCIScmConnectionStatusTable(d *shared.Deps, result *webcore.CIScmConnectionStatus, providerID string) error {
	asc.RenderTableTo(d.IO.Out, ciScmConnectionStatusHeaders(), ciScmConnectionStatusRows(result, providerID))
	return nil
}

func renderCIScmConnectionStatusMarkdown(d *shared.Deps, result *webcore.CIScmConnectionStatus, providerID string) error {
	asc.RenderMarkdownTo(d.IO.Out, ciScmConnectionStatusHeaders(), ciScmConnectionStatusRows(result, providerID))
	return nil
}

func ciScmConnectionStatusHeaders() []string {
	return []string{"SCM Provider ID", "Status"}
}

func ciScmConnectionStatusRows(result *webcore.CIScmConnectionStatus, providerID string) [][]string {
	if result == nil {
		return [][]string{{valueOrNA(providerID), "unknown"}}
	}
	return [][]string{{
		valueOrNA(providerID),
		valueOrNA(result.Status),
	}}
}

func formatCIScmBool(value *bool) string {
	if value == nil {
		return "unknown"
	}
	return strconv.FormatBool(*value)
}
