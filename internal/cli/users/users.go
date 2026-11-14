package users

import (
	"context"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewUsersCommand returns the users command with subcommands.
func NewUsersCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("users", flag.ExitOnError)

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
			newUsersListCommand(d),
			newUsersGetCommand(d),
			newUsersUpdateCommand(d),
			newUsersDeleteCommand(d),
			newUsersInviteCommand(d),
			newUsersInvitesCommand(d),
			newUsersVisibleAppsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newUsersListCommand returns the users list subcommand.
func newUsersListCommand(d *shared.Deps) *ffcli.Command {
	var (
		email                                     *string
		visibleAppsLimit                          *int
		visibleAppValues, roleValues, sortValues  []string
		fieldsValue, appFieldsValue, includeValue []string
	)

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:        d,
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc users list [flags]",
		ShortHelp:   "List users in App Store Connect.",
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
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextBeforeLimit:      true,
		NextConflicts:        []string{"email", "role", "visible-app", "sort", "fields", "app-fields", "include", "visible-apps-limit", "limit"},
		Flags: func(fs *flag.FlagSet) func() error {
			email = fs.String("email", "", "Filter by email/username")
			role := fs.String("role", "", "Filter by UserRole (comma-separated): "+strings.Join(userRoleList(), ", "))
			visibleApp := fs.String("visible-app", "", "Filter by visible app ID(s), comma-separated")
			sort := fs.String("sort", "", "Sort by one or more comma-separated expressions: username, -username, lastName, or -lastName")
			fields := fs.String("fields", "", "User fields to include: "+strings.Join(usersFieldsList(), ", "))
			appFields := fs.String("app-fields", "", "Fields to include for related apps, comma-separated")
			include := fs.String("include", "", "Include related resources: visibleApps")
			visibleAppsLimit = fs.Int("visible-apps-limit", 0, "Maximum included visible apps (1-50)")

			return func() error {
				provided := make(map[string]bool)
				fs.Visit(func(f *flag.Flag) {
					provided[f.Name] = true
				})
				if provided["visible-apps-limit"] && (*visibleAppsLimit < 1 || *visibleAppsLimit > 50) {
					return d.UsageErrorf("users list: --visible-apps-limit must be between 1 and 50")
				}
				visibleAppValues = shared.SplitCSV(*visibleApp)
				if provided["visible-app"] && len(visibleAppValues) == 0 {
					return d.UsageErrorf("users list: --visible-app must not be empty")
				}
				var err error
				if roleValues, err = normalizeUserRoles(*role, "--role"); err != nil {
					return d.UsageErrorf("users list: %v", err)
				}
				if sortValues, err = normalizeUsersSort(*sort, "--sort"); err != nil {
					return d.UsageErrorf("users list: %v", err)
				}
				if provided["sort"] && len(sortValues) == 0 {
					return d.UsageErrorf("users list: --sort must not be empty")
				}
				if fieldsValue, err = normalizeUsersFields(*fields, "--fields"); err != nil {
					return d.UsageErrorf("users list: %v", err)
				}
				if provided["fields"] && len(fieldsValue) == 0 {
					return d.UsageErrorf("users list: --fields must not be empty")
				}
				if appFieldsValue, err = normalizeUsersAppFields(*appFields, "--app-fields"); err != nil {
					return d.UsageErrorf("users list: %v", err)
				}
				if provided["app-fields"] && len(appFieldsValue) == 0 {
					return d.UsageErrorf("users list: --app-fields must not be empty")
				}
				if includeValue, err = normalizeUsersInclude(*include); err != nil {
					return d.UsageErrorf("users list: %v", err)
				}
				if provided["include"] && len(includeValue) == 0 {
					return d.UsageErrorf("users list: --include must not be empty")
				}
				if len(appFieldsValue) > 0 && !shared.HasInclude(includeValue, "visibleApps") {
					return d.UsageErrorf("users list: --app-fields requires --include visibleApps")
				}
				if *visibleAppsLimit > 0 && !shared.HasInclude(includeValue, "visibleApps") {
					return d.UsageErrorf("users list: --visible-apps-limit requires --include visibleApps")
				}
				return nil
			}
		},
		ErrorPrefix:      "users list",
		FetchErrorPrefix: "users list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetUsers(ctx,
				asc.WithUsersEmail(*email),
				asc.WithUsersRoles(roleValues),
				asc.WithUsersVisibleAppIDs(visibleAppValues),
				asc.WithUsersSort(strings.Join(sortValues, ",")),
				asc.WithUsersFields(fieldsValue),
				asc.WithUsersAppFields(appFieldsValue),
				asc.WithUsersInclude(includeValue),
				asc.WithUsersVisibleAppsLimit(*visibleAppsLimit),
				asc.Limit[asc.UsersOption](limit),
				asc.NextURL[asc.UsersOption](next),
			)
		},
	})
}

// newUsersGetCommand returns the users view subcommand.
func newUsersGetCommand(d *shared.Deps) *ffcli.Command {
	var (
		includeValues []string
		included      asc.IncludedFields
	)

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc users view --id USER_ID",
		ShortHelp:   "View a user by ID.",
		LongHelp: `View a user by ID.

Examples:
  asc users view --id "USER_ID"
  asc users view --id "USER_ID" --include visibleApps`,
		IDUsage:            "User ID",
		IDType:             "users",
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			include := fs.String("include", "", "Include related resources: visibleApps")
			// Apple rejects these spec fields[apps] values with 400 on users view.
			includedFields := shared.BindIncludedFieldFlagsExcept(fs, map[string]string{"visibleApps": "apps"}, map[string][]string{"apps": {
				"appStoreIcon", "performanceOverviews", "customerReviewSummarizations", "analyticsReportRequests",
				"betaFeedbackScreenshotSubmissions", "betaFeedbackCrashSubmissions", "webhooks", "androidToIosAppMappingDetails",
			}})
			return func() (err error) {
				if includeValues, err = normalizeUsersInclude(*include); err != nil {
					return d.UsageErrorf("users view: %v", err)
				}
				included, err = includedFields.ResolveWith(d, "users view", "", includeValues)
				return err
			}
		},
		ErrorPrefix:      "users view",
		FetchErrorPrefix: "users view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			opts := []asc.UsersOption{asc.WithIncludedFields[asc.UsersOption](included)}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithUsersInclude(includeValues))
			}
			return client.GetUser(ctx, id, opts...)
		},
	})
}

// newUsersUpdateCommand returns the users update subcommand.
func newUsersUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "users", "User ID")
	roles := shared.BindOnceCSVFlag(fs, "roles", "Comma-separated UserRole values: "+strings.Join(userRoleList(), ", "))
	visibleApps := shared.BindOnceCSVFlag(fs, "visible-app", "Comma-separated app IDs for visible apps")
	var provisioningAllowed shared.OptionalBool
	fs.Var(&provisioningAllowed, "provisioning-allowed", "Grant or remove provisioning access: true or false (omit to keep current access while the new roles allow it)")
	confirm := fs.Bool("confirm", false, "Confirm replacing visible apps (required with --visible-app)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc users update --id USER_ID --roles ROLE_ID[,ROLE_ID...] [--visible-app APP_ID[,APP_ID...]] [--confirm] [--provisioning-allowed true|false]",
		ShortHelp:  "Update a user.",
		LongHelp: `Update a user by ID.

The --visible-app list replaces the user's existing visible-app relationship;
use --confirm when --visible-app is supplied.

Examples:
  asc users update --id "USER_ID" --roles "ADMIN"
  asc users update --id "USER_ID" --roles "ADMIN" --visible-app "APP_ID" --confirm
  asc users update --id "USER_ID" --roles "DEVELOPER" --provisioning-allowed false`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--id")
			}

			roleValues, err := normalizeUserRoles(roles.String(), "--roles")
			if err != nil {
				return d.UsageErrorf("users update: %v", err)
			}
			if len(roleValues) == 0 {
				return d.RequiredFlag("--roles")
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
				fmt.Fprintln(d.IO.ErrOut, "Error:", message)
				return shared.WithDiagnostic(
					shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message),
					shared.DiagnosticConflictingInput,
					"--confirm",
				)
			}
			if len(visibleAppIDs) > 0 && !*confirm {
				message := "--confirm is required when --visible-app is set"
				fmt.Fprintln(d.IO.ErrOut, "Error:", message)
				return shared.WithDiagnostic(
					shared.NewReportedUsageError(shared.UsageErrorMissingRequired, message),
					shared.DiagnosticRequiredInputMissing,
					"--confirm",
				)
			}
			warnDeprecatedUserRoles(d, roleValues)

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("users update: %w", err)
			}
			if len(visibleAppIDs) > 0 && client.IsEnterpriseProgramAPI() {
				return d.UsageError("users update: --visible-app is not available on the Enterprise Program API")
			}
			if provisioningAllowed.IsSet() && client.IsEnterpriseProgramAPI() {
				return d.UsageError("users update: --provisioning-allowed is not available on the Enterprise Program API")
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.UserUpdateAttributes{
				Roles: roleValues,
			}
			if len(visibleAppIDs) > 0 {
				allAppsVisible := false
				attrs.AllAppsVisible = &allAppsVisible
			}
			if provisioningAllowed.IsSet() {
				allowed := provisioningAllowed.Value()
				attrs.ProvisioningAllowed = &allowed
			} else if !client.IsEnterpriseProgramAPI() && slices.ContainsFunc(roleValues, rolePermitsProvisioning) {
				// Omitting the attribute has reset provisioning access, so keep
				// access the user has when the new roles can still hold it.
				current, err := client.GetUser(requestCtx, idValue)
				if err != nil {
					return fmt.Errorf("users update: failed to read current provisioning access: %w", err)
				}
				if current.Data.Attributes.ProvisioningAllowed {
					allowed := true
					attrs.ProvisioningAllowed = &allowed
				}
			}

			user, err := client.UpdateUser(requestCtx, idValue, attrs, visibleAppIDs)
			if err != nil {
				return fmt.Errorf("users update: failed to update: %w", err)
			}

			return d.PrintOutput(user, *output.Output, *output.Pretty)
		},
	}
}

// newUsersDeleteCommand returns the users delete subcommand.
func newUsersDeleteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("delete", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "users", "User ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--confirm")
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("users delete: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.DeleteUser(requestCtx, idValue); err != nil {
				return fmt.Errorf("users delete: failed to delete: %w", err)
			}

			result := &asc.UserDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newUsersInviteCommand returns the users invite subcommand.
func newUsersInviteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("invite", flag.ExitOnError)

	email := fs.String("email", "", "Email address to invite")
	firstName := fs.String("first-name", "", "First name of the invitee (required)")
	lastName := fs.String("last-name", "", "Last name of the invitee (required)")
	roles := shared.BindOnceCSVFlag(fs, "roles", "Comma-separated UserRole values: "+strings.Join(userRoleList(), ", "))
	allApps := fs.Bool("all-apps", false, "Grant access to all apps")
	visibleApps := shared.BindOnceCSVFlag(fs, "visible-app", "Comma-separated app IDs for visible apps")
	var provisioningAllowed shared.OptionalBool
	fs.Var(&provisioningAllowed, "provisioning-allowed", "Grant provisioning access: true or false (omit for Apple's default)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "invite",
		ShortUsage: "asc users invite --email EMAIL --first-name NAME --last-name NAME --roles ROLE[,ROLE...] [--all-apps | --visible-app APP_ID[,APP_ID...]] [--provisioning-allowed true|false]",
		ShortHelp:  "Invite a user.",
		LongHelp: `Invite a new user to App Store Connect.

Examples:
  asc users invite --email "user@example.com" --first-name "Jane" --last-name "Doe" --roles "ADMIN" --all-apps
  asc users invite --email "user@example.com" --first-name "John" --last-name "Smith" --roles "DEVELOPER" --visible-app "APP_ID"
  asc users invite --email "user@example.com" --first-name "Jane" --last-name "Doe" --roles "DEVELOPER" --all-apps --provisioning-allowed true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			emailValue := strings.TrimSpace(*email)
			if emailValue == "" {
				return d.RequiredFlag("--email")
			}

			firstNameValue := strings.TrimSpace(*firstName)
			if firstNameValue == "" {
				return d.RequiredFlag("--first-name")
			}

			lastNameValue := strings.TrimSpace(*lastName)
			if lastNameValue == "" {
				return d.RequiredFlag("--last-name")
			}

			roleValues, err := normalizeUserRoles(roles.String(), "--roles")
			if err != nil {
				return d.UsageErrorf("users invite: %v", err)
			}
			if len(roleValues) == 0 {
				return d.RequiredFlag("--roles")
			}
			warnDeprecatedUserRoles(d, roleValues)

			if *allApps && strings.TrimSpace(visibleApps.String()) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --all-apps and --visible-app cannot be used together")
				return flag.ErrHelp
			}

			visibleAppIDs := shared.SplitCSV(visibleApps.String())

			// Whether the flags apply depends on the key, so a missing-credentials
			// error waits until the flags have been checked.
			client, clientErr := d.Client()
			enterprise := clientErr == nil && client.IsEnterpriseProgramAPI()
			if enterprise && (*allApps || len(visibleAppIDs) > 0) {
				return d.UsageError("users invite: --all-apps and --visible-app are not available on the Enterprise Program API")
			}
			if enterprise && provisioningAllowed.IsSet() {
				return d.UsageError("users invite: --provisioning-allowed is not available on the Enterprise Program API")
			}
			if !enterprise && !*allApps && len(visibleAppIDs) == 0 {
				fmt.Fprintln(d.IO.ErrOut, "Error: --all-apps or --visible-app is required")
				return shared.MissingRequiredUsageError("")
			}
			if clientErr != nil {
				return fmt.Errorf("users invite: %w", clientErr)
			}

			requestCtx, cancel := d.Timeout(ctx)
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
			if provisioningAllowed.IsSet() {
				allowed := provisioningAllowed.Value()
				attrs.ProvisioningAllowed = &allowed
			}

			invitation, err := client.CreateUserInvitation(requestCtx, attrs, visibleAppIDs)
			if err != nil {
				return fmt.Errorf("users invite: failed to create: %w", err)
			}

			return d.PrintOutput(invitation, *output.Output, *output.Pretty)
		},
	}
}

// newUsersInvitesCommand returns the users invites command with subcommands.
func newUsersInvitesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("invites", flag.ExitOnError)

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
			newUsersInvitesListCommand(d),
			newUsersInvitesGetCommand(d),
			newUsersInvitesRevokeCommand(d),
			newUsersInvitesVisibleAppsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newUsersInvitesListCommand returns the users invites list subcommand.
func newUsersInvitesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:        d,
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc users invites list [flags]",
		ShortHelp:   "List user invitations.",
		LongHelp: `List user invitations.

Examples:
  asc users invites list
  asc users invites list --limit 50
  asc users invites list --paginate`,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "users invites list",
		FetchErrorPrefix:     "users invites list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetUserInvitations(ctx,
				asc.Limit[asc.UserInvitationsOption](limit),
				asc.NextURL[asc.UserInvitationsOption](next),
			)
		},
	})
}

// newUsersInvitesGetCommand returns the users invites view subcommand.
func newUsersInvitesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc users invites view --id INVITE_ID",
		ShortHelp:   "View a user invitation by ID.",
		LongHelp: `View a user invitation by ID.

Examples:
  asc users invites view --id "INVITE_ID"`,
		IDUsage:            "Invitation ID",
		IDType:             "userInvitations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "users invites view",
		FetchErrorPrefix:   "users invites view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetUserInvitation(ctx, id)
		},
	})
}

// newUsersInvitesRevokeCommand returns the users invites revoke subcommand.
func newUsersInvitesRevokeCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("revoke", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "userInvitations", "Invitation ID")
	confirm := fs.Bool("confirm", false, "Confirm revocation")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--confirm")
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("users invites revoke: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.DeleteUserInvitation(requestCtx, idValue); err != nil {
				return fmt.Errorf("users invites revoke: failed to revoke: %w", err)
			}

			result := &asc.UserInvitationRevokeResult{
				ID:      idValue,
				Revoked: true,
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
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
