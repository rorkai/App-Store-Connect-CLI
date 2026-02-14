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
		ShortHelp:  "Manage App Store Connect users and invitations.",
		LongHelp: `Manage App Store Connect users and invitations.

Examples:
  asc users list
  asc users get --id "USER_ID"
  asc users get --id "USER_ID" --include visibleApps
  asc users update --id "USER_ID" --roles "ADMIN"
  asc users delete --id "USER_ID" --confirm
  asc users invite --email "user@example.com" --roles "ADMIN" --all-apps
  asc users invites list
  asc users invites visible-apps list --id "INVITE_ID"
  asc users visible-apps list --id "USER_ID"
  asc users visible-apps get --id "USER_ID"`,
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
	role := fs.String("role", "", "Filter by role (comma-separated): ADMIN, DEVELOPER, APP_MANAGER, etc.")
	output := shared.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc users list [flags]",
		ShortHelp:  "List App Store Connect users.",
		LongHelp: `List App Store Connect users.

Examples:
  asc users list
  asc users list --email "user@example.com"
  asc users list --role "ADMIN"
  asc users list --role "DEVELOPER,APP_MANAGER"
  asc users list --limit 50
  asc users list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("users list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("users list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.UsersOption{
				asc.WithUsersEmail(*email),
				asc.WithUsersRoles(shared.SplitCSV(*role)),
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

// UsersGetCommand returns the users get subcommand.
func UsersGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "User ID")
	include := fs.String("include", "", "Include related resources: visibleApps")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc users get --id USER_ID",
		ShortHelp:  "Get a user by ID.",
		LongHelp: `Get a user by ID.

Examples:
  asc users get --id "USER_ID"
  asc users get --id "USER_ID" --include visibleApps`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			includeValues, err := normalizeUsersInclude(*include)
			if err != nil {
				return fmt.Errorf("users get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.UsersOption{}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithUsersInclude(includeValues))
			}

			user, err := client.GetUser(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("users get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(user, *output.Output, *output.Pretty)
		},
	}
}

// UsersUpdateCommand returns the users update subcommand.
func UsersUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "User ID")
	roles := fs.String("roles", "", "Comma-separated role IDs")
	visibleApps := fs.String("visible-app", "", "Comma-separated app IDs for visible apps")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc users update --id USER_ID --roles ROLE_ID[,ROLE_ID...] [--visible-app APP_ID[,APP_ID...]]",
		ShortHelp:  "Update a user.",
		LongHelp: `Update a user by ID.

Examples:
  asc users update --id "USER_ID" --roles "ADMIN"
  asc users update --id "USER_ID" --roles "ADMIN" --visible-app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			roleValues := shared.SplitCSV(*roles)
			if len(roleValues) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --roles is required")
				return flag.ErrHelp
			}

			visibleAppIDs := shared.SplitCSV(*visibleApps)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users update: %w", err)
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

			user, err := client.UpdateUser(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("users update: failed to update: %w", err)
			}

			if len(visibleAppIDs) > 0 {
				if err := client.SetUserVisibleApps(requestCtx, idValue, visibleAppIDs); err != nil {
					return fmt.Errorf("users update: roles updated but failed to set visible apps: %w", err)
				}
			}

			return shared.PrintOutput(user, *output.Output, *output.Pretty)
		},
	}
}

// UsersDeleteCommand returns the users delete subcommand.
func UsersDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "User ID")
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
				return flag.ErrHelp
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
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
	roles := fs.String("roles", "", "Comma-separated role IDs")
	allApps := fs.Bool("all-apps", false, "Grant access to all apps")
	visibleApps := fs.String("visible-app", "", "Comma-separated app IDs for visible apps")
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
				return flag.ErrHelp
			}

			firstNameValue := strings.TrimSpace(*firstName)
			if firstNameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --first-name is required")
				return flag.ErrHelp
			}

			lastNameValue := strings.TrimSpace(*lastName)
			if lastNameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --last-name is required")
				return flag.ErrHelp
			}

			roleValues := shared.SplitCSV(*roles)
			if len(roleValues) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --roles is required")
				return flag.ErrHelp
			}

			if *allApps && strings.TrimSpace(*visibleApps) != "" {
				fmt.Fprintln(os.Stderr, "Error: --all-apps and --visible-app cannot be used together")
				return flag.ErrHelp
			}

			visibleAppIDs := shared.SplitCSV(*visibleApps)

			if !*allApps && len(visibleAppIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --all-apps or --visible-app is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users invite: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.UserInvitationCreateAttributes{
				Email:     emailValue,
				FirstName: firstNameValue,
				LastName:  lastNameValue,
				Roles:     roleValues,
			}
			if *allApps {
				allAppsVisible := true
				attrs.AllAppsVisible = &allAppsVisible
			} else {
				allAppsVisible := false
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
  asc users invites get --id "INVITE_ID"
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
				return fmt.Errorf("users invites list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("users invites list: %w", err)
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

// UsersInvitesGetCommand returns the users invites get subcommand.
func UsersInvitesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Invitation ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc users invites get --id INVITE_ID",
		ShortHelp:  "Get a user invitation by ID.",
		LongHelp: `Get a user invitation by ID.

Examples:
  asc users invites get --id "INVITE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users invites get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			invite, err := client.GetUserInvitation(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("users invites get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(invite, *output.Output, *output.Pretty)
		},
	}
}

// UsersInvitesRevokeCommand returns the users invites revoke subcommand.
func UsersInvitesRevokeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("revoke", flag.ExitOnError)

	id := fs.String("id", "", "Invitation ID")
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
				return flag.ErrHelp
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
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
