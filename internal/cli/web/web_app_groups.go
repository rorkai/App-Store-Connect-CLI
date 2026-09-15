package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var listDeveloperAppGroupsFn = func(ctx context.Context, client *webcore.Client, options webcore.DeveloperAppGroupsListOptions) (*webcore.DeveloperAppGroupsListResult, error) {
	return client.ListDeveloperAppGroups(ctx, options)
}

var createDeveloperAppGroupFn = func(ctx context.Context, client *webcore.Client, request webcore.DeveloperAppGroupCreateRequest) (*webcore.DeveloperAppGroup, error) {
	return client.CreateDeveloperAppGroup(ctx, request)
}

var assignDeveloperAppGroupFn = func(ctx context.Context, client *webcore.Client, request webcore.DeveloperAppGroupAssignRequest) (*webcore.DeveloperAppGroupAssignResult, error) {
	return client.AssignDeveloperAppGroup(ctx, request)
}

var unassignDeveloperAppGroupFn = func(ctx context.Context, client *webcore.Client, request webcore.DeveloperAppGroupUnassignRequest) (*asc.WebAppGroupUnassignResult, error) {
	return client.UnassignDeveloperAppGroup(ctx, request)
}

var setDeveloperAppGroupsFn = func(ctx context.Context, client *webcore.Client, request webcore.DeveloperAppGroupSetRequest) (*asc.WebAppGroupSetResult, error) {
	return client.SetDeveloperAppGroups(ctx, request)
}

var deleteDeveloperAppGroupFn = func(ctx context.Context, client *webcore.Client, request webcore.DeveloperAppGroupDeleteRequest) (*asc.WebAppGroupDeleteResult, error) {
	return client.DeleteDeveloperAppGroup(ctx, request)
}

const developerAppGroupProfileWarning = "Warning: this change invalidates existing provisioning profiles that contain the affected App ID. Regenerate affected profiles before the next signed build."

const developerAppGroupProfileHelp = `When this command changes the App ID, it invalidates existing provisioning profiles
that contain that App ID. Regenerate affected profiles before the next signed build.`

// WebAppGroupsCommand returns the Developer Portal App Groups command group.
func WebAppGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-groups", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "app-groups",
		ShortUsage: "asc web app-groups <subcommand> [flags]",
		ShortHelp:  "Manage App Groups via Developer Portal web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

List, register, assign, unassign, reconcile, and delete App Groups through Apple
Developer Portal. These resources are not exposed by the public App Store Connect
API and require a user-owned Apple web session with Account Holder or Admin access.

Examples:
  asc web app-groups list --output table
  asc web app-groups create --name "Example Shared" --identifier "group.com.example.shared" --confirm
  asc web app-groups assign --group "GROUP_RESOURCE_ID" --bundle-id "BUNDLE_RESOURCE_ID" --confirm
  asc web app-groups unassign --group-id "GROUP_RESOURCE_ID" --bundle-id "BUNDLE_RESOURCE_ID" --confirm
  asc web app-groups set --bundle-id "BUNDLE_RESOURCE_ID" --group "GROUP_RESOURCE_ID" --group "OTHER_GROUP_ID" --confirm
  asc web app-groups delete --group-id "GROUP_RESOURCE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppGroupsListCommand(),
			WebAppGroupsCreateCommand(),
			WebAppGroupsAssignCommand(),
			WebAppGroupsUnassignCommand(),
			WebAppGroupsSetCommand(),
			WebAppGroupsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAppGroupsListCommand lists App Groups for the selected Developer Portal team.
func WebAppGroupsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-groups list", flag.ExitOnError)
	paginate := fs.Bool("paginate", false, "Fetch all pages")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web app-groups list [flags]",
		ShortHelp:  "List App Groups via a Developer Portal web session.",
		LongHelp: `List App Groups visible to the selected Apple Developer team.

By default, the command fetches the first page. Pass --paginate to fetch every
page.

The ID column is Apple's opaque App Group resource ID. Pass that value to
"asc web app-groups assign --group", "set --group", "unassign --group-id", or
"delete --group-id".

Example:
  asc web app-groups list --apple-id "user@example.com" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web app-groups list does not accept positional arguments")
			}
			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return developerAppGroupError(err, "web app-groups list")
			}
			result, err := listDeveloperAppGroupsFn(requestCtx, newDeveloperPortalClient(session, portalFlags), webcore.DeveloperAppGroupsListOptions{Paginate: *paginate})
			if err != nil {
				return developerAppGroupError(err, "web app-groups list")
			}
			persistDeveloperPortalSession(session)
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
	portalFlags := bindDeveloperPortalFlags(fs)
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
			if resolvedName == "" {
				return shared.UsageError("--name is required")
			}
			if resolvedIdentifier == "" {
				return shared.UsageError("--identifier is required")
			}
			if err := webcore.ValidateDeveloperAppGroupIdentifier(resolvedIdentifier); err != nil {
				return shared.UsageError("--" + err.Error())
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
				return developerAppGroupError(err, "web app-groups create")
			}
			result, err := createDeveloperAppGroupFn(requestCtx, newDeveloperPortalClient(session, portalFlags), webcore.DeveloperAppGroupCreateRequest{Name: resolvedName, Identifier: resolvedIdentifier})
			// Persist after the create attempt so a later command without
			// --developer-team still targets the team that may have registered
			// the group even when Apple's 2xx body is malformed.
			persistDeveloperPortalSession(session)
			if err != nil {
				return developerAppGroupError(err, "web app-groups create")
			}
			if result == nil {
				return developerAppGroupMissingResultError("web app-groups create", "create")
			}
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
	confirm := fs.Bool("confirm", false, "Confirm assignment; a changed App ID invalidates existing provisioning profiles")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "assign",
		ShortUsage: "asc web app-groups assign --group GROUP_RESOURCE_ID --bundle-id BUNDLE_RESOURCE_ID --confirm [flags]",
		ShortHelp:  "Assign an App Group to a Bundle ID via a web session.",
		LongHelp: `Assign an existing App Group to a Bundle ID while preserving every current
Bundle ID capability and relationship. The operation also enables APP_GROUPS
when needed and is idempotent when the association already exists.

When this command changes the App ID, it invalidates existing provisioning profiles
that contain that App ID. Regenerate affected profiles before the next signed build.

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

			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return developerAppGroupError(err, "web app-groups assign")
			}
			result, err := assignDeveloperAppGroupFn(requestCtx, newDeveloperPortalClient(session, portalFlags), webcore.DeveloperAppGroupAssignRequest{BundleID: resolvedBundleID, GroupID: resolvedGroupID})
			if err != nil {
				return developerAppGroupMutationError(session, err, "web app-groups assign")
			}
			if result == nil {
				return developerAppGroupMissingResultError("web app-groups assign", "assign")
			}
			persistDeveloperPortalSession(session)
			warnDeveloperAppGroupProfileInvalidation(result.Changed)
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

// WebAppGroupsUnassignCommand removes an App Group from a Bundle ID.
func WebAppGroupsUnassignCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-groups unassign", flag.ExitOnError)
	groupID := fs.String("group-id", "", "Opaque App Group resource ID from app-groups list")
	bundleID := fs.String("bundle-id", "", "Opaque Developer Portal Bundle ID resource ID")
	confirm := fs.Bool("confirm", false, "Confirm removal; a changed App ID invalidates existing provisioning profiles")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "unassign",
		ShortUsage: "asc web app-groups unassign --group-id GROUP_RESOURCE_ID --bundle-id BUNDLE_RESOURCE_ID --confirm [flags]",
		ShortHelp:  "Remove an App Group from a Bundle ID via a web session.",
		LongHelp: `Remove one App Group from a Bundle ID while preserving every other Bundle ID
capability and relationship. Removing the last group disables the APP_GROUPS
capability. The operation is idempotent when the group is not assigned and
verifies the result by re-reading the Bundle ID.

` + developerAppGroupProfileHelp + `

Use opaque resource IDs returned by "asc web app-groups list" and
"asc bundle-ids list".

Example:
  asc web app-groups unassign --group-id "GROUP_RESOURCE_ID" --bundle-id "BUNDLE_RESOURCE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web app-groups unassign does not accept positional arguments")
			}
			resolvedGroupID := strings.TrimSpace(*groupID)
			resolvedBundleID := strings.TrimSpace(*bundleID)
			switch {
			case resolvedGroupID == "":
				return shared.UsageError("--group-id is required")
			case resolvedBundleID == "":
				return shared.UsageError("--bundle-id is required")
			case !*confirm:
				return shared.UsageError("--confirm is required")
			}

			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return developerAppGroupError(err, "web app-groups unassign")
			}
			result, err := unassignDeveloperAppGroupFn(requestCtx, newDeveloperPortalClient(session, portalFlags), webcore.DeveloperAppGroupUnassignRequest{BundleID: resolvedBundleID, GroupID: resolvedGroupID})
			if err != nil {
				return developerAppGroupMutationError(session, err, "web app-groups unassign")
			}
			if result == nil {
				return developerAppGroupMissingResultError("web app-groups unassign", "unassign")
			}
			persistDeveloperPortalSession(session)
			warnDeveloperAppGroupProfileInvalidation(result.Changed)
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// WebAppGroupsSetCommand converges a Bundle ID on a complete App Group set.
func WebAppGroupsSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-groups set", flag.ExitOnError)
	var groupIDs shared.MultiStringFlag
	fs.Var(&groupIDs, "group", "Opaque App Group resource ID to keep assigned (repeatable)")
	bundleID := fs.String("bundle-id", "", "Opaque Developer Portal Bundle ID resource ID")
	confirm := fs.Bool("confirm", false, "Confirm reconciliation; a changed App ID invalidates existing provisioning profiles")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc web app-groups set --bundle-id BUNDLE_RESOURCE_ID --group GROUP_RESOURCE_ID [--group GROUP_RESOURCE_ID ...] --confirm [flags]",
		ShortHelp:  "Set the complete App Group set of a Bundle ID via a web session.",
		LongHelp: `Replace a Bundle ID's App Group assignments with exactly the groups passed via
--group. Groups missing from the desired set are removed, new groups are added,
and every other Bundle ID capability and relationship is preserved. The receipt
lists the added and removed groups. When the current set already matches, no
write is sent and the receipt reports changed=false. The result is verified by
re-reading the Bundle ID.

` + developerAppGroupProfileHelp + `

Use opaque resource IDs returned by "asc web app-groups list" and
"asc bundle-ids list".

Example:
  asc web app-groups set --bundle-id "BUNDLE_RESOURCE_ID" --group "GROUP_RESOURCE_ID" --group "OTHER_GROUP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web app-groups set does not accept positional arguments")
			}
			resolvedBundleID := strings.TrimSpace(*bundleID)
			switch {
			case resolvedBundleID == "":
				return shared.UsageError("--bundle-id is required")
			case len(groupIDs) == 0:
				return shared.UsageError("at least one --group is required")
			case !*confirm:
				return shared.UsageError("--confirm is required")
			}

			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return developerAppGroupError(err, "web app-groups set")
			}
			result, err := setDeveloperAppGroupsFn(requestCtx, newDeveloperPortalClient(session, portalFlags), webcore.DeveloperAppGroupSetRequest{BundleID: resolvedBundleID, GroupIDs: []string(groupIDs)})
			if err != nil {
				return developerAppGroupMutationError(session, err, "web app-groups set")
			}
			if result == nil {
				return developerAppGroupMissingResultError("web app-groups set", "set")
			}
			persistDeveloperPortalSession(session)
			warnDeveloperAppGroupProfileInvalidation(result.Changed)
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// WebAppGroupsDeleteCommand deletes an App Group registration.
func WebAppGroupsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-groups delete", flag.ExitOnError)
	groupID := fs.String("group-id", "", "Opaque App Group resource ID from app-groups list")
	confirm := fs.Bool("confirm", false, "Confirm deletion; deleting a group invalidates existing provisioning profiles for App IDs that referenced it")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc web app-groups delete --group-id GROUP_RESOURCE_ID --confirm [flags]",
		ShortHelp:  "Delete an App Group via a Developer Portal web session.",
		LongHelp: `Delete an App Group registration from the selected Apple Developer team.

The command fails closed when the group is still assigned to any Bundle ID: no
delete is sent, the exit code is non-zero, and the assigned Bundle IDs are named
on stderr. Remove the assignments first with "asc web app-groups unassign" or
"asc web app-groups set". A successful delete is verified by re-reading the
team's App Group list.

Deleting a group invalidates existing provisioning profiles for App IDs that
referenced it. Regenerate affected profiles before the next signed build.

Example:
  asc web app-groups delete --group-id "GROUP_RESOURCE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web app-groups delete does not accept positional arguments")
			}
			resolvedGroupID := strings.TrimSpace(*groupID)
			switch {
			case resolvedGroupID == "":
				return shared.UsageError("--group-id is required")
			case !*confirm:
				return shared.UsageError("--confirm is required")
			}

			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return developerAppGroupError(err, "web app-groups delete")
			}
			result, err := deleteDeveloperAppGroupFn(requestCtx, newDeveloperPortalClient(session, portalFlags), webcore.DeveloperAppGroupDeleteRequest{GroupID: resolvedGroupID})
			if err != nil {
				return developerAppGroupMutationError(session, err, "web app-groups delete")
			}
			if result == nil {
				return developerAppGroupMissingResultError("web app-groups delete", "delete")
			}
			persistDeveloperPortalSession(session)
			warnDeveloperAppGroupProfileInvalidation(result.Deleted)
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// developerAppGroupMutationError keeps the auth hint behavior of every other
// web command, converts a user-fixable App Group argument into the usage-error
// contract, and additionally warns when the portal accepted a write that could
// not be verified, because the App ID may already have changed. Every returned
// failure carries a diagnostic code so no App Group failure is reported as an
// unclassified internal error.
func developerAppGroupMutationError(session *webcore.AuthSession, err error, command string) error {
	var identifier *webcore.DeveloperAppGroupIdentifierError
	if errors.As(err, &identifier) {
		parameter := developerAppGroupIDParameter(command)
		return shared.WithDiagnostic(
			shared.UsageErrorf("%s is invalid: %s", parameter, identifier.Error()),
			shared.DiagnosticInvalidInput,
			parameter,
		)
	}
	var unverified *webcore.DeveloperAppGroupUnverifiedError
	if errors.As(err, &unverified) {
		// Persist before returning so a later command without --developer-team
		// still targets the team that accepted the write.
		persistDeveloperPortalSession(session)
		_, _ = fmt.Fprintln(os.Stderr, "Warning: the Developer Portal accepted the change but it could not be verified; assume it was applied.")
		warnDeveloperAppGroupProfileInvalidation(true)
	}
	return developerAppGroupError(err, command)
}

// developerAppGroupError renders an App Group failure the way every other web
// command does and attaches the structured reason behind it. A failure that
// already classifies itself keeps its own code and parameter, so an operator
// mistake is never relabeled as an internal defect.
func developerAppGroupError(err error, command string) error {
	if err == nil {
		return nil
	}
	rendered := withWebAuthHint(err, command)
	if diagnostic, ok := shared.DiagnosticFromError(err); ok {
		return shared.WithDiagnostic(rendered, diagnostic.Code, diagnostic.Parameter)
	}
	return shared.WithDiagnostic(rendered, developerAppGroupDiagnosticCode(err), "")
}

// developerAppGroupMissingResultError classifies a 2xx portal response that
// produced no receipt. It is a CLI-side invariant failure, not an operator
// mistake, so it keeps the internal_error code while still reporting one.
func developerAppGroupMissingResultError(command, receipt string) error {
	return shared.WithDiagnostic(
		fmt.Errorf("%s failed: missing %s result", command, receipt),
		shared.DiagnosticInternalError,
		"",
	)
}

// developerAppGroupIDParameter names the flag that carries the App Group
// resource ID for a command, so a rejected value points at the flag the
// operator typed.
func developerAppGroupIDParameter(command string) string {
	switch {
	case strings.HasSuffix(command, "unassign"), strings.HasSuffix(command, "delete"):
		return "--group-id"
	default:
		return "--group"
	}
}

// developerAppGroupDiagnosticCode maps an App Group failure onto the bounded
// diagnostic taxonomy. Unrecognized failures still report internal_error
// explicitly instead of leaving the telemetry dimension empty.
func developerAppGroupDiagnosticCode(err error) shared.DiagnosticCode {
	var unverified *webcore.DeveloperAppGroupUnverifiedError
	var inUse *webcore.DeveloperAppGroupInUseError
	var unreadable *webcore.DeveloperAppGroupUnreadableResponseError
	var notFound *webcore.DeveloperAppGroupNotFoundError
	var resultErr *webcore.DeveloperPortalResultError
	var apiErr *webcore.APIError
	var urlErr *url.Error
	// A failure the CLI already reports as a usage or validation problem keeps
	// that meaning: it describes what the operator typed or the state they
	// asked about, not a defect.
	switch shared.ClassifyUsageError(err) {
	case shared.UsageErrorMissingRequired:
		return shared.DiagnosticRequiredInputMissing
	case shared.UsageErrorInvalidValue, shared.UsageErrorOther:
		return shared.DiagnosticInvalidInput
	}
	if shared.IsValidationError(err) {
		return shared.DiagnosticStateNotReady
	}
	switch {
	case errors.As(err, &notFound):
		return shared.DiagnosticResourceNotFound
	case errors.Is(err, webcore.ErrDeveloperPortalTeamNotSelected), errors.Is(err, errNoCachedWebSession):
		return shared.DiagnosticAuthenticationRejected
	case errors.As(err, &unverified):
		return shared.DiagnosticStateNotReady
	case errors.As(err, &inUse):
		return shared.DiagnosticResourceConflict
	case errors.As(err, &unreadable):
		return shared.DiagnosticDependencyFailed
	case errors.As(err, &resultErr):
		return shared.DiagnosticRequestFailed
	case errors.As(err, &apiErr):
		switch apiErr.Status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return shared.DiagnosticAuthenticationRejected
		case http.StatusNotFound:
			return shared.DiagnosticResourceNotFound
		case http.StatusConflict:
			return shared.DiagnosticResourceConflict
		default:
			return shared.DiagnosticRequestFailed
		}
	case errors.Is(err, webcore.ErrInvalidAppleAccountCredentials), errors.Is(err, shared.ErrMissingAuth):
		return shared.DiagnosticAuthenticationRejected
	case errors.As(err, &urlErr),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, context.Canceled),
		errors.Is(err, io.ErrUnexpectedEOF),
		errors.Is(err, io.EOF):
		return shared.DiagnosticRequestFailed
	default:
		return shared.DiagnosticInternalError
	}
}

func warnDeveloperAppGroupProfileInvalidation(changed bool) {
	if !changed {
		return
	}
	_, _ = fmt.Fprintln(os.Stderr, developerAppGroupProfileWarning)
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
