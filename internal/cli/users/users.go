package users

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

// UsersCommand returns the users command with subcommands.
func UsersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("users", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "users",
		ShortUsage: "asc users <subcommand> [flags]",
		ShortHelp:  "Manage users and invitations in App Store Connect.",
		LongHelp: `Manage users and invitations in App Store Connect.

Examples:
  asc users list
  asc users view --id "USER_ID"
  asc users view --id "USER_ID" --include visibleApps
  asc users update --id "USER_ID" --roles "ADMIN"
  asc users delete --id "USER_ID" --confirm
  asc users invite --email "user@example.com" --first-name "Jane" --last-name "Doe" --roles "ADMIN" --all-apps
  asc users invites list
  asc users invites visible-apps list --id "INVITE_ID"
  asc users visible-apps list --id "USER_ID"
  asc users visible-apps view --id "USER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			UsersListCommand(),
			UsersGetCommand(),
			UsersUpdateCommand(),
			UsersDeleteCommand(),
			UsersInviteCommand(),
			UsersInvitesCommand(),
			UsersVisibleAppsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// UsersListCommand returns the users list subcommand.
func UsersListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	email := fs.String("email", "", "Filter by email/username")
	role := fs.String("role", "", "Filter by UserRole (comma-separated): "+strings.Join(userRoleList(), ", "))
	visibleApp := fs.String("visible-app", "", "Filter by visible app ID(s), comma-separated")
	sort := fs.String("sort", "", "Sort by one or more comma-separated expressions: username, -username, lastName, or -lastName")
	fields := fs.String("fields", "", "User fields to include: "+strings.Join(usersFieldsList(), ", "))
	appFields := fs.String("app-fields", "", "Fields to include for related apps, comma-separated")
	include := fs.String("include", "", "Include related resources: visibleApps")
	visibleAppsLimit := fs.Int("visible-apps-limit", 0, "Maximum included visible apps (1-50)")
	output := shared.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc users list [flags]",
		ShortHelp:  "List users in App Store Connect.",
		LongHelp: `List users in App Store Connect.

Examples:
  asc users list
  asc users list --email "user@example.com"
  asc users list --role "ADMIN"
  asc users list --role "DEVELOPER,APP_MANAGER"
  asc users list --visible-app "APP_ID"
  asc users list --sort "username,-lastName"
  asc users list --fields "username,lastName" --include visibleApps --app-fields "name,bundleId"
  asc users list --limit 50
  asc users list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("users list: %v", err)
			}
			if err := shared.RejectNextFlagConflicts(
				fs,
				*next,
				"users list",
				"email", "role", "visible-app", "sort", "fields", "app-fields", "include", "visible-apps-limit", "limit",
			); err != nil {
				return err
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("users list: --limit must be between 1 and 200")
			}
			provided := make(map[string]bool)
			fs.Visit(func(f *flag.Flag) {
				provided[f.Name] = true
			})
			if provided["visible-apps-limit"] && (*visibleAppsLimit < 1 || *visibleAppsLimit > 50) {
				return shared.UsageErrorf("users list: --visible-apps-limit must be between 1 and 50")
			}
			visibleAppValues := shared.SplitCSV(*visibleApp)
			if provided["visible-app"] && len(visibleAppValues) == 0 {
				return shared.UsageErrorf("users list: --visible-app must not be empty")
			}
			roleValues, err := normalizeUserRoles(*role, "--role")
			if err != nil {
				return shared.UsageErrorf("users list: %v", err)
			}
			sortValues, err := normalizeUsersSort(*sort, "--sort")
			if err != nil {
				return shared.UsageErrorf("users list: %v", err)
			}
			if provided["sort"] && len(sortValues) == 0 {
				return shared.UsageErrorf("users list: --sort must not be empty")
			}
			fieldsValue, err := normalizeUsersFields(*fields, "--fields")
			if err != nil {
				return shared.UsageErrorf("users list: %v", err)
			}
			if provided["fields"] && len(fieldsValue) == 0 {
				return shared.UsageErrorf("users list: --fields must not be empty")
			}
			appFieldsValue, err := normalizeUsersAppFields(*appFields, "--app-fields")
			if err != nil {
				return shared.UsageErrorf("users list: %v", err)
			}
			if provided["app-fields"] && len(appFieldsValue) == 0 {
				return shared.UsageErrorf("users list: --app-fields must not be empty")
			}
			includeValue, err := normalizeUsersInclude(*include)
			if err != nil {
				return shared.UsageErrorf("users list: %v", err)
			}
			if provided["include"] && len(includeValue) == 0 {
				return shared.UsageErrorf("users list: --include must not be empty")
			}
			if len(appFieldsValue) > 0 && !shared.HasInclude(includeValue, "visibleApps") {
				return shared.UsageErrorf("users list: --app-fields requires --include visibleApps")
			}
			if *visibleAppsLimit > 0 && !shared.HasInclude(includeValue, "visibleApps") {
				return shared.UsageErrorf("users list: --visible-apps-limit requires --include visibleApps")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.UsersOption{
				asc.WithUsersEmail(*email),
				asc.WithUsersRoles(roleValues),
				asc.WithUsersVisibleAppIDs(visibleAppValues),
				asc.WithUsersSort(strings.Join(sortValues, ",")),
				asc.WithUsersFields(fieldsValue),
				asc.WithUsersAppFields(appFieldsValue),
				asc.WithUsersInclude(includeValue),
				asc.WithUsersVisibleAppsLimit(*visibleAppsLimit),
				asc.WithUsersLimit(*limit),
				asc.WithUsersNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithUsersLimit(200))
				firstPage, err := client.GetUsers(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("users list: failed to fetch: %w", err)
				}

				users, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetUsers(ctx, asc.WithUsersNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("users list: %w", err)
				}

				return shared.PrintOutput(users, *output.Output, *output.Pretty)
			}

			users, err := client.GetUsers(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("users list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(users, *output.Output, *output.Pretty)
		},
	}
}

// UsersGetCommand returns the users view subcommand.
func UsersGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "users", "User ID")
	include := fs.String("include", "", "Include related resources: visibleApps")
	includedFields := shared.BindIncludedFieldFlags(fs, map[string]string{"visibleApps": "apps"})
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc users view --id USER_ID",
		ShortHelp:  "View a user by ID.",
		LongHelp: `View a user by ID.

Examples:
  asc users view --id "USER_ID"
  asc users view --id "USER_ID" --include visibleApps`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			includeValues, err := normalizeUsersInclude(*include)
			if err != nil {
				return fmt.Errorf("users view: %w", err)
			}
			included, err := includedFields.Resolve("users view", "", includeValues)
			if err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.UsersOption{asc.WithIncludedFields[asc.UsersOption](included)}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithUsersInclude(includeValues))
			}

			user, err := client.GetUser(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("users view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(user, *output.Output, *output.Pretty)
		},
	}
}

// UsersUpdateCommand returns the users update subcommand.
func UsersUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "users", "User ID")
	roles := shared.BindOnceCSVFlag(fs, "roles", "Comma-separated UserRole values: "+strings.Join(userRoleList(), ", "))
	visibleApps := shared.BindOnceCSVFlag(fs, "visible-app", "Comma-separated app IDs for visible apps")
	confirm := fs.Bool("confirm", false, "Confirm replacing visible apps (required with --visible-app)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc users update --id USER_ID --roles ROLE_ID[,ROLE_ID...] [--visible-app APP_ID[,APP_ID...]] [--confirm]",
		ShortHelp:  "Update a user.",
		LongHelp: `Update a user by ID.

The --visible-app list replaces the user's existing visible-app relationship;
use --confirm when --visible-app is supplied.

Examples:
  asc users update --id "USER_ID" --roles "ADMIN"
  asc users update --id "USER_ID" --roles "ADMIN" --visible-app "APP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			roleValues, err := normalizeUserRoles(roles.String(), "--roles")
			if err != nil {
				return shared.UsageErrorf("users update: %v", err)
			}
			if len(roleValues) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --roles is required")
				return shared.MissingRequiredUsageError("--roles")
			}
			visibleAppIDs := shared.SplitCSV(visibleApps.String())
			confirmSet := false
			fs.Visit(func(parsed *flag.Flag) {
				if parsed.Name == "confirm" {
					confirmSet = true
				}
			})
			if len(visibleAppIDs) == 0 && confirmSet {
				message := "--confirm requires --visible-app"
				fmt.Fprintln(os.Stderr, "Error:", message)
				return shared.WithDiagnostic(
					shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message),
					shared.DiagnosticConflictingInput,
					"--confirm",
				)
			}
			if len(visibleAppIDs) > 0 && !*confirm {
				message := "--confirm is required when --visible-app is set"
				fmt.Fprintln(os.Stderr, "Error:", message)
				return shared.WithDiagnostic(
					shared.NewReportedUsageError(shared.UsageErrorMissingRequired, message),
					shared.DiagnosticRequiredInputMissing,
					"--confirm",
				)
			}
			warnDeprecatedUserRoles(roleValues)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users update: %w", err)
			}
			if len(visibleAppIDs) > 0 && client.IsEnterpriseProgramAPI() {
				return shared.UsageError("users update: --visible-app is not available on the Enterprise Program API")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.UserUpdateAttributes{
				Roles: roleValues,
			}
			if len(visibleAppIDs) > 0 {
				allAppsVisible := false
				attrs.AllAppsVisible = &allAppsVisible
			}

			user, err := client.UpdateUser(requestCtx, idValue, attrs, visibleAppIDs)
			if err != nil {
				return fmt.Errorf("users update: failed to update: %w", err)
			}

			return shared.PrintOutput(user, *output.Output, *output.Pretty)
		},
	}
}

// UsersDeleteCommand returns the users delete subcommand.
func UsersDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "users", "User ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc users delete --id USER_ID --confirm",
		ShortHelp:  "Delete a user.",
		LongHelp: `Delete a user by ID.

Examples:
  asc users delete --id "USER_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteUser(requestCtx, idValue); err != nil {
				return fmt.Errorf("users delete: failed to delete: %w", err)
			}

			result := &asc.UserDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// UsersInviteCommand returns the users invite subcommand.
func UsersInviteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("invite", flag.ExitOnError)

	email := fs.String("email", "", "Email address to invite")
	firstName := fs.String("first-name", "", "First name of the invitee (required)")
	lastName := fs.String("last-name", "", "Last name of the invitee (required)")
	roles := shared.BindOnceCSVFlag(fs, "roles", "Comma-separated UserRole values: "+strings.Join(userRoleList(), ", "))
	allApps := fs.Bool("all-apps", false, "Grant access to all apps")
	visibleApps := shared.BindOnceCSVFlag(fs, "visible-app", "Comma-separated app IDs for visible apps")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "invite",
		ShortUsage: "asc users invite --email EMAIL --first-name NAME --last-name NAME --roles ROLE[,ROLE...] [--all-apps | --visible-app APP_ID[,APP_ID...]]",
		ShortHelp:  "Invite a user.",
		LongHelp: `Invite a new user to App Store Connect.

Examples:
  asc users invite --email "user@example.com" --first-name "Jane" --last-name "Doe" --roles "ADMIN" --all-apps
  asc users invite --email "user@example.com" --first-name "John" --last-name "Smith" --roles "DEVELOPER" --visible-app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			emailValue := strings.TrimSpace(*email)
			if emailValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --email is required")
				return shared.MissingRequiredUsageError("--email")
			}

			firstNameValue := strings.TrimSpace(*firstName)
			if firstNameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --first-name is required")
				return shared.MissingRequiredUsageError("--first-name")
			}

			lastNameValue := strings.TrimSpace(*lastName)
			if lastNameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --last-name is required")
				return shared.MissingRequiredUsageError("--last-name")
			}

			roleValues, err := normalizeUserRoles(roles.String(), "--roles")
			if err != nil {
				return shared.UsageErrorf("users invite: %v", err)
			}
			if len(roleValues) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --roles is required")
				return shared.MissingRequiredUsageError("--roles")
			}
			warnDeprecatedUserRoles(roleValues)

			if *allApps && strings.TrimSpace(visibleApps.String()) != "" {
				fmt.Fprintln(os.Stderr, "Error: --all-apps and --visible-app cannot be used together")
				return flag.ErrHelp
			}

			visibleAppIDs := shared.SplitCSV(visibleApps.String())

			// Whether the flags apply depends on the key, so a missing-credentials
			// error waits until the flags have been checked.
			client, clientErr := shared.GetASCClient()
			enterprise := clientErr == nil && client.IsEnterpriseProgramAPI()
			if enterprise && (*allApps || len(visibleAppIDs) > 0) {
				return shared.UsageError("users invite: --all-apps and --visible-app are not available on the Enterprise Program API")
			}
			if !enterprise && !*allApps && len(visibleAppIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --all-apps or --visible-app is required")
				return shared.MissingRequiredUsageError("")
			}
			if clientErr != nil {
				return fmt.Errorf("users invite: %w", clientErr)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.UserInvitationCreateAttributes{
				Email:     emailValue,
				FirstName: firstNameValue,
				LastName:  lastNameValue,
				Roles:     roleValues,
			}
			if !enterprise {
				allAppsVisible := *allApps
				attrs.AllAppsVisible = &allAppsVisible
			}

			invitation, err := client.CreateUserInvitation(requestCtx, attrs, visibleAppIDs)
			if err != nil {
				return fmt.Errorf("users invite: failed to create: %w", err)
			}

			return shared.PrintOutput(invitation, *output.Output, *output.Pretty)
		},
	}
}

// UsersInvitesCommand returns the users invites command with subcommands.
func UsersInvitesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("invites", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "invites",
		ShortUsage: "asc users invites <subcommand> [flags]",
		ShortHelp:  "Manage user invitations.",
		LongHelp: `Manage user invitations.

Examples:
  asc users invites list
  asc users invites view --id "INVITE_ID"
  asc users invites revoke --id "INVITE_ID" --confirm
  asc users invites visible-apps list --id "INVITE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			UsersInvitesListCommand(),
			UsersInvitesGetCommand(),
			UsersInvitesRevokeCommand(),
			UsersInvitesVisibleAppsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// UsersInvitesListCommand returns the users invites list subcommand.
func UsersInvitesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	output := shared.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc users invites list [flags]",
		ShortHelp:  "List user invitations.",
		LongHelp: `List user invitations.

Examples:
  asc users invites list
  asc users invites list --limit 50
  asc users invites list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("users invites list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("users invites list: %v", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users invites list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.UserInvitationsOption{
				asc.WithUserInvitationsLimit(*limit),
				asc.WithUserInvitationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithUserInvitationsLimit(200))
				firstPage, err := client.GetUserInvitations(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("users invites list: failed to fetch: %w", err)
				}

				invites, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetUserInvitations(ctx, asc.WithUserInvitationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("users invites list: %w", err)
				}

				return shared.PrintOutput(invites, *output.Output, *output.Pretty)
			}

			invites, err := client.GetUserInvitations(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("users invites list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(invites, *output.Output, *output.Pretty)
		},
	}
}

// UsersInvitesGetCommand returns the users invites view subcommand.
func UsersInvitesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "userInvitations", "Invitation ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc users invites view --id INVITE_ID",
		ShortHelp:  "View a user invitation by ID.",
		LongHelp: `View a user invitation by ID.

Examples:
  asc users invites view --id "INVITE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users invites view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			invite, err := client.GetUserInvitation(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("users invites view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(invite, *output.Output, *output.Pretty)
		},
	}
}

// UsersInvitesRevokeCommand returns the users invites revoke subcommand.
func UsersInvitesRevokeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("revoke", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "userInvitations", "Invitation ID")
	confirm := fs.Bool("confirm", false, "Confirm revocation")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "revoke",
		ShortUsage: "asc users invites revoke --id INVITE_ID --confirm",
		ShortHelp:  "Revoke a user invitation.",
		LongHelp: `Revoke a user invitation by ID.

Examples:
  asc users invites revoke --id "INVITE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users invites revoke: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteUserInvitation(requestCtx, idValue); err != nil {
				return fmt.Errorf("users invites revoke: failed to revoke: %w", err)
			}

			result := &asc.UserInvitationRevokeResult{
				ID:      idValue,
				Revoked: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func normalizeUsersInclude(value string) ([]string, error) {
	include := shared.SplitCSV(value)
	if len(include) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, item := range usersIncludeList() {
		allowed[item] = struct{}{}
	}
	for _, item := range include {
		if _, ok := allowed[item]; !ok {
			return nil, fmt.Errorf("--include must be one of: %s", strings.Join(usersIncludeList(), ", "))
		}
	}
	return include, nil
}

func usersIncludeList() []string {
	return []string{"visibleApps"}
}
