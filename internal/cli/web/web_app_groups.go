package web

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var listDeveloperAppGroupsFn = func(ctx context.Context, client *webcore.Client) (*webcore.DeveloperAppGroupsListResult, error) {
	return client.ListDeveloperAppGroups(ctx)
}

var createDeveloperAppGroupFn = func(ctx context.Context, client *webcore.Client, request webcore.DeveloperAppGroupCreateRequest) (*webcore.DeveloperAppGroup, error) {
	return client.CreateDeveloperAppGroup(ctx, request)
}

var assignDeveloperAppGroupFn = func(ctx context.Context, client *webcore.Client, request webcore.DeveloperAppGroupAssignRequest) (*webcore.DeveloperAppGroupAssignResult, error) {
	return client.AssignDeveloperAppGroup(ctx, request)
}

// WebAppGroupsCommand returns the Developer Portal App Groups command group.
func WebAppGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-groups", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "app-groups",
		ShortUsage: "asc web app-groups <subcommand> [flags]",
		ShortHelp:  "Manage App Groups via Developer Portal web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

List, register, and assign App Groups through Apple Developer Portal.
These resources are not exposed by the public App Store Connect API and require
a user-owned Apple web session with Account Holder or Admin access.

Examples:
  asc web app-groups list --output table
  asc web app-groups create --name "Example Shared" --identifier "group.com.example.shared" --confirm
  asc web app-groups assign --group "GROUP_RESOURCE_ID" --bundle-id "BUNDLE_RESOURCE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppGroupsListCommand(),
			WebAppGroupsCreateCommand(),
			WebAppGroupsAssignCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAppGroupsListCommand lists App Groups for the selected Developer Portal team.
func WebAppGroupsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-groups list", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web app-groups list [flags]",
		ShortHelp:  "List App Groups via a Developer Portal web session.",
		LongHelp: `List every App Group visible to the selected Apple Developer team.

The ID column is Apple's opaque App Group resource ID. Pass that value to
"asc web app-groups assign --group".

Example:
  asc web app-groups list --apple-id "user@example.com" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web app-groups list does not accept positional arguments")
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return withWebAuthHint(err, "web app-groups list")
			}
			result, err := listDeveloperAppGroupsFn(requestCtx, newWebClientFn(session))
			if err != nil {
				return withWebAuthHint(err, "web app-groups list")
			}
			_ = persistWebSessionFn(session)
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperAppGroupsTable(result) },
				func() error { return renderDeveloperAppGroupsMarkdown(result) },
			)
		},
	}
}

// WebAppGroupsCreateCommand registers an App Group identifier.
func WebAppGroupsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-groups create", flag.ExitOnError)
	name := fs.String("name", "", "Human-readable App Group name")
	identifier := fs.String("identifier", "", "App Group identifier beginning with group.")
	confirm := fs.Bool("confirm", false, "Confirm registering this App Group")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web app-groups create --name NAME --identifier GROUP_ID --confirm [flags]",
		ShortHelp:  "Register an App Group via a Developer Portal web session.",
		LongHelp: `Register a new App Group for the selected Apple Developer team.

Example:
  asc web app-groups create --name "Example Shared" --identifier "group.com.example.shared" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web app-groups create does not accept positional arguments")
			}
			resolvedName := strings.TrimSpace(*name)
			resolvedIdentifier := strings.TrimSpace(*identifier)
			switch {
			case resolvedName == "":
				return shared.UsageError("--name is required")
			case resolvedIdentifier == "":
				return shared.UsageError("--identifier is required")
			case !strings.HasPrefix(resolvedIdentifier, "group."):
				return shared.UsageError("--identifier must start with group.")
			case strings.ContainsAny(resolvedIdentifier, " \t\r\n"):
				return shared.UsageError("--identifier must not contain whitespace")
			case !*confirm:
				return shared.UsageError("--confirm is required")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return withWebAuthHint(err, "web app-groups create")
			}
			result, err := createDeveloperAppGroupFn(requestCtx, newWebClientFn(session), webcore.DeveloperAppGroupCreateRequest{Name: resolvedName, Identifier: resolvedIdentifier})
			if err != nil {
				return withWebAuthHint(err, "web app-groups create")
			}
			if result == nil {
				return fmt.Errorf("web app-groups create failed: missing create result")
			}
			_ = persistWebSessionFn(session)
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperAppGroupTable(result) },
				func() error { return renderDeveloperAppGroupMarkdown(result) },
			)
		},
	}
}

// WebAppGroupsAssignCommand assigns an App Group to a Bundle ID.
func WebAppGroupsAssignCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-groups assign", flag.ExitOnError)
	groupID := fs.String("group", "", "Opaque App Group resource ID from app-groups list")
	bundleID := fs.String("bundle-id", "", "Opaque Developer Portal Bundle ID resource ID")
	confirm := fs.Bool("confirm", false, "Confirm assigning this App Group")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "assign",
		ShortUsage: "asc web app-groups assign --group GROUP_RESOURCE_ID --bundle-id BUNDLE_RESOURCE_ID --confirm [flags]",
		ShortHelp:  "Assign an App Group to a Bundle ID via a web session.",
		LongHelp: `Assign an existing App Group to a Bundle ID while preserving every current
Bundle ID capability and relationship. The operation also enables APP_GROUPS
when needed and is idempotent when the association already exists.

Use opaque resource IDs returned by "asc web app-groups list" and
"asc bundle-ids list".

Example:
  asc web app-groups assign --group "GROUP_RESOURCE_ID" --bundle-id "BUNDLE_RESOURCE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web app-groups assign does not accept positional arguments")
			}
			resolvedGroupID := strings.TrimSpace(*groupID)
			resolvedBundleID := strings.TrimSpace(*bundleID)
			switch {
			case resolvedGroupID == "":
				return shared.UsageError("--group is required")
			case resolvedBundleID == "":
				return shared.UsageError("--bundle-id is required")
			case !*confirm:
				return shared.UsageError("--confirm is required")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return withWebAuthHint(err, "web app-groups assign")
			}
			result, err := assignDeveloperAppGroupFn(requestCtx, newWebClientFn(session), webcore.DeveloperAppGroupAssignRequest{BundleID: resolvedBundleID, GroupID: resolvedGroupID})
			if err != nil {
				return withWebAuthHint(err, "web app-groups assign")
			}
			if result == nil {
				return fmt.Errorf("web app-groups assign failed: missing assign result")
			}
			_ = persistWebSessionFn(session)
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperAppGroupAssignTable(result) },
				func() error { return renderDeveloperAppGroupAssignMarkdown(result) },
			)
		},
	}
}

func developerAppGroupsHeaders() []string {
	return []string{"ID", "Name", "Identifier", "Status"}
}

func developerAppGroupsRows(groups []webcore.DeveloperAppGroup) [][]string {
	rows := make([][]string, 0, len(groups))
	for _, group := range groups {
		rows = append(rows, []string{shared.OrNA(group.ID), shared.OrNA(group.Name), shared.OrNA(group.Identifier), shared.OrNA(group.Status)})
	}
	return rows
}

func renderDeveloperAppGroupsTable(result *webcore.DeveloperAppGroupsListResult) error {
	if result == nil {
		asc.RenderTable(developerAppGroupsHeaders(), nil)
		return nil
	}
	asc.RenderTable(developerAppGroupsHeaders(), developerAppGroupsRows(result.Data))
	return nil
}

func renderDeveloperAppGroupsMarkdown(result *webcore.DeveloperAppGroupsListResult) error {
	if result == nil {
		asc.RenderMarkdown(developerAppGroupsHeaders(), nil)
		return nil
	}
	asc.RenderMarkdown(developerAppGroupsHeaders(), developerAppGroupsRows(result.Data))
	return nil
}

func renderDeveloperAppGroupTable(result *webcore.DeveloperAppGroup) error {
	asc.RenderTable(developerAppGroupsHeaders(), developerAppGroupsRows([]webcore.DeveloperAppGroup{*result}))
	return nil
}

func renderDeveloperAppGroupMarkdown(result *webcore.DeveloperAppGroup) error {
	asc.RenderMarkdown(developerAppGroupsHeaders(), developerAppGroupsRows([]webcore.DeveloperAppGroup{*result}))
	return nil
}

func developerAppGroupAssignRows(result *webcore.DeveloperAppGroupAssignResult) [][]string {
	return [][]string{{result.BundleID, result.GroupID, fmt.Sprintf("%t", result.Changed), result.Status}}
}

func renderDeveloperAppGroupAssignTable(result *webcore.DeveloperAppGroupAssignResult) error {
	asc.RenderTable([]string{"Bundle ID", "Group ID", "Changed", "Status"}, developerAppGroupAssignRows(result))
	return nil
}

func renderDeveloperAppGroupAssignMarkdown(result *webcore.DeveloperAppGroupAssignResult) error {
	asc.RenderMarkdown([]string{"Bundle ID", "Group ID", "Changed", "Status"}, developerAppGroupAssignRows(result))
	return nil
}
