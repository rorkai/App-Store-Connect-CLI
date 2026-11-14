package bundleids

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

// BundleIDsCommand returns the bundle IDs command with subcommands.
func BundleIDsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("bundle-ids", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "bundle-ids",
		ShortUsage: "asc bundle-ids <subcommand> [flags]",
		ShortHelp:  "Manage bundle IDs and capabilities.",
		LongHelp: `Manage bundle IDs and capabilities.

Examples:
  asc bundle-ids list
  asc bundle-ids view --id "BUNDLE_ID"
  asc bundle-ids app view --id "BUNDLE_ID"
  asc bundle-ids profiles list --id "BUNDLE_ID"
  asc bundle-ids create --identifier "com.example.app" --name "Example" --platform IOS
  asc bundle-ids update --id "BUNDLE_ID" --name "New Name"
  asc bundle-ids delete --id "BUNDLE_ID" --confirm
  asc bundle-ids capabilities list --bundle "BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BundleIDsListCommand(),
			BundleIDsGetCommand(),
			BundleIDsAppCommand(),
			BundleIDsProfilesCommand(),
			BundleIDsCreateCommand(),
			BundleIDsUpdateCommand(),
			BundleIDsDeleteCommand(),
			BundleIDsCapabilitiesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BundleIDsListCommand returns the bundle IDs list subcommand.
func BundleIDsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	name := fs.String("name", "", "Filter by name(s), comma-separated")
	platform := fs.String("platform", "", "Filter by platform(s), comma-separated: "+strings.Join(shared.BundleIDPlatformList(), ", "))
	identifier := fs.String("identifier", "", "Filter by identifier(s), comma-separated")
	seedID := fs.String("seed-id", "", "Filter by seed ID(s), comma-separated")
	ids := fs.String("id", "", "Filter by bundle ID(s), comma-separated")
	sort := fs.String("sort", "", "Sort by (comma-separated): "+strings.Join(bundleIDSortValues(), ", "))
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(bundleIDFieldsList(), ", "))
	profileFields := fs.String("profile-fields", "", "Profile fields to include: "+strings.Join(bundleIDProfileFieldsList(), ", "))
	capabilityFields := fs.String("capability-fields", "", "Capability fields to include: "+strings.Join(bundleIDCapabilityFieldsList(), ", "))
	appFields := fs.String("app-fields", "", "App fields to include: "+strings.Join(shared.AppFieldsList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(bundleIDIncludeList(), ", "))
	profilesLimit := fs.Int("profiles-limit", 0, "Maximum included profiles (1-50)")
	capabilitiesLimit := fs.Int("capabilities-limit", 0, "Maximum included capabilities (1-50)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc bundle-ids list [flags]",
		ShortHelp:  "List bundle IDs.",
		LongHelp: `List bundle IDs.

Examples:
  asc bundle-ids list
  asc bundle-ids list --name "Example"
  asc bundle-ids list --identifier "com.example.app"
  asc bundle-ids list --platform IOS
  asc bundle-ids list --include profiles --profile-fields "name,expirationDate"
  asc bundle-ids list --limit 10
  asc bundle-ids list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			profilesLimitSet := bundleIDsListFlagWasSet(fs, "profiles-limit")
			capabilitiesLimitSet := bundleIDsListFlagWasSet(fs, "capabilities-limit")
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("bundle-ids list: %v", err)
			}
			if err := shared.RejectNextFlagConflicts(
				fs,
				*next,
				"bundle-ids list",
				"name", "platform", "identifier", "seed-id", "id", "sort", "fields", "profile-fields", "capability-fields", "app-fields", "include", "profiles-limit", "capabilities-limit", "limit",
			); err != nil {
				return err
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("bundle-ids list: %w", shared.UsageError("--limit must be between 1 and 200"))
			}
			if profilesLimitSet && (*profilesLimit < 1 || *profilesLimit > 50) {
				return fmt.Errorf("bundle-ids list: %w", shared.UsageError("--profiles-limit must be between 1 and 50"))
			}
			if capabilitiesLimitSet && (*capabilitiesLimit < 1 || *capabilitiesLimit > 50) {
				return fmt.Errorf("bundle-ids list: %w", shared.UsageError("--capabilities-limit must be between 1 and 50"))
			}
			sortValue, err := normalizeBundleIDSort(*sort)
			if err != nil {
				return fmt.Errorf("bundle-ids list: %w", shared.UsageError(err.Error()))
			}

			platformValues, err := normalizeBundleIDListPlatforms(shared.SplitCSV(*platform))
			if err != nil {
				return fmt.Errorf("bundle-ids list: %w", shared.UsageError(err.Error()))
			}
			fieldValues, err := shared.NormalizeSelection(*fields, bundleIDFieldsList(), "--fields")
			if err != nil {
				return fmt.Errorf("bundle-ids list: %w", shared.UsageError(err.Error()))
			}
			profileFieldValues, err := shared.NormalizeSelection(*profileFields, bundleIDProfileFieldsList(), "--profile-fields")
			if err != nil {
				return fmt.Errorf("bundle-ids list: %w", shared.UsageError(err.Error()))
			}
			capabilityFieldValues, err := shared.NormalizeSelection(*capabilityFields, bundleIDCapabilityFieldsList(), "--capability-fields")
			if err != nil {
				return fmt.Errorf("bundle-ids list: %w", shared.UsageError(err.Error()))
			}
			appFieldValues, err := shared.NormalizeSelection(*appFields, shared.AppFieldsList(), "--app-fields")
			if err != nil {
				return fmt.Errorf("bundle-ids list: %w", shared.UsageError(err.Error()))
			}
			includeValues, err := shared.NormalizeSelection(*include, bundleIDIncludeList(), "--include")
			if err != nil {
				return fmt.Errorf("bundle-ids list: %w", shared.UsageError(err.Error()))
			}
			if len(profileFieldValues) > 0 && !shared.HasInclude(includeValues, "profiles") {
				return bundleIDsListIncludeRequirementUsageError("--profile-fields", "--profile-fields requires --include profiles")
			}
			if len(capabilityFieldValues) > 0 && !shared.HasInclude(includeValues, "bundleIdCapabilities") {
				return bundleIDsListIncludeRequirementUsageError("--capability-fields", "--capability-fields requires --include bundleIdCapabilities")
			}
			if len(appFieldValues) > 0 && !shared.HasInclude(includeValues, "app") {
				return bundleIDsListIncludeRequirementUsageError("--app-fields", "--app-fields requires --include app")
			}
			if profilesLimitSet && !shared.HasInclude(includeValues, "profiles") {
				return bundleIDsListIncludeRequirementUsageError("--profiles-limit", "--profiles-limit requires --include profiles")
			}
			if capabilitiesLimitSet && !shared.HasInclude(includeValues, "bundleIdCapabilities") {
				return bundleIDsListIncludeRequirementUsageError("--capabilities-limit", "--capabilities-limit requires --include bundleIdCapabilities")
			}

			opts := []asc.BundleIDsOption{
				asc.WithBundleIDsFilterNames(shared.SplitCSV(*name)),
				asc.WithBundleIDsFilterPlatforms(platformValues),
				asc.WithBundleIDsFilterIdentifier(*identifier),
				asc.WithBundleIDsFilterSeedIDs(shared.SplitCSV(*seedID)),
				asc.WithBundleIDsFilterIDs(shared.SplitCSV(*ids)),
				asc.WithBundleIDsSort(sortValue),
				asc.WithBundleIDsFields(fieldValues),
				asc.WithBundleIDsProfilesFields(profileFieldValues),
				asc.WithBundleIDsCapabilitiesFields(capabilityFieldValues),
				asc.WithBundleIDsAppFields(appFieldValues),
				asc.WithBundleIDsInclude(includeValues),
				asc.WithBundleIDsProfilesLimit(*profilesLimit),
				asc.WithBundleIDsCapabilitiesLimit(*capabilitiesLimit),
				asc.Limit[asc.BundleIDsOption](*limit),
				asc.NextURL[asc.BundleIDsOption](*next),
				asc.WithBundleIDsSplitPagination(*paginate),
			}
			if err := asc.ValidateBundleIDsRequest(opts...); err != nil {
				if asc.IsBundleIDsPaginationRequired(err) {
					return bundleIDsListPaginateRequirementUsageError()
				}
				return bundleIDsListRequestValidationUsageError(err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.BundleIDsOption](200))
				firstPage, err := client.GetBundleIDs(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("bundle-ids list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateBundleIDs(requestCtx, firstPage, func(ctx context.Context, nextURL string) (*asc.BundleIDsResponse, error) {
					return client.GetBundleIDs(ctx, asc.NextURL[asc.BundleIDsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("bundle-ids list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBundleIDs(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("bundle-ids list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func bundleIDsListIncludeRequirementUsageError(parameter, message string) error {
	fmt.Fprintln(os.Stderr, "Error: "+message)
	return shared.WithDiagnostic(
		shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message),
		shared.DiagnosticInvalidInput,
		parameter,
	)
}

func bundleIDsListPaginateRequirementUsageError() error {
	const message = "split identifier filter requires --paginate because multiple continuation URLs cannot be represented"
	fmt.Fprintln(os.Stderr, "Error: "+message)
	return shared.WithDiagnostic(
		shared.NewReportedUsageError(shared.UsageErrorMissingRequired, message),
		shared.DiagnosticRequiredInputMissing,
		"--paginate",
	)
}

func bundleIDsListRequestValidationUsageError(err error) error {
	message := strings.TrimPrefix(strings.TrimSpace(err.Error()), "bundleIds: ")
	fmt.Fprintln(os.Stderr, "Error: "+message)
	return shared.WithDiagnostic(
		shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message),
		shared.DiagnosticInvalidInput,
		"",
	)
}

func bundleIDsListFlagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(value *flag.Flag) {
		set = set || value.Name == name
	})
	return set
}

// BundleIDsGetCommand returns the bundle IDs get subcommand.
func BundleIDsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc bundle-ids view --id \"BUNDLE_ID\"",
		ShortHelp:   "View a bundle ID by ID.",
		LongHelp: `View a bundle ID by ID.

Examples:
  asc bundle-ids view --id "BUNDLE_ID"`,
		IDUsage:            "Bundle ID",
		IDType:             "bundleIds",
		RequiredDiagnostic: true,
		ErrorPrefix:        "bundle-ids view",
		FetchErrorPrefix:   "bundle-ids view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBundleID(ctx, id)
		},
	})
}

// BundleIDsCreateCommand returns the bundle IDs create subcommand.
func BundleIDsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	identifier := fs.String("identifier", "", "Bundle ID identifier (e.g., com.example.app)")
	name := fs.String("name", "", "Bundle ID name")
	platform := fs.String("platform", "IOS", "Platform: "+strings.Join(shared.BundleIDPlatformList(), ", "))
	seedID := fs.String("seed-id", "", "App ID prefix (seedId); omit for Apple's default")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc bundle-ids create --identifier \"com.example.app\" --name \"Example\" [--platform IOS] [--seed-id SEED_ID]",
		ShortHelp:  "Create a bundle ID.",
		LongHelp: `Create a bundle ID.

Examples:
  asc bundle-ids create --identifier "com.example.app" --name "Example" --platform IOS
  asc bundle-ids create --identifier "com.example.app" --name "Example" --seed-id "ABCDE12345"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			identifierValue := strings.TrimSpace(*identifier)
			if identifierValue == "" {
				return shared.RequiredFlag("--identifier")
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				return shared.RequiredFlag("--name")
			}
			platformValue, err := shared.NormalizeBundleIDPlatform(*platform)
			if err != nil {
				return fmt.Errorf("bundle-ids create: %w", shared.UsageError(err.Error()))
			}
			seedIDValue := strings.TrimSpace(*seedID)
			seedIDSet := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "seed-id" {
					seedIDSet = true
				}
			})
			if seedIDSet && seedIDValue == "" {
				return fmt.Errorf("bundle-ids create: %w", shared.UsageError("--seed-id must not be empty"))
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BundleIDCreateAttributes{
				Name:       nameValue,
				Identifier: identifierValue,
				Platform:   platformValue,
				SeedID:     seedIDValue,
			}
			resp, err := client.CreateBundleID(requestCtx, attrs)
			if err != nil {
				return fmt.Errorf("bundle-ids create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BundleIDsUpdateCommand returns the bundle IDs update subcommand.
func BundleIDsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "bundleIds", "Bundle ID")
	name := fs.String("name", "", "Bundle ID name")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc bundle-ids update --id \"BUNDLE_ID\" --name \"New Name\"",
		ShortHelp:  "Update a bundle ID.",
		LongHelp: `Update a bundle ID.

Examples:
  asc bundle-ids update --id "BUNDLE_ID" --name "New Name"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return shared.RequiredFlag("--id")
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				return shared.RequiredFlag("--name")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BundleIDUpdateAttributes{Name: nameValue}
			resp, err := client.UpdateBundleID(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("bundle-ids update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BundleIDsDeleteCommand returns the bundle IDs delete subcommand.
func BundleIDsDeleteCommand() *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc bundle-ids delete --id \"BUNDLE_ID\" --confirm",
		ShortHelp:   "Delete a bundle ID.",
		LongHelp: `Delete a bundle ID.

Examples:
  asc bundle-ids delete --id "BUNDLE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Bundle ID",
		IDType:             "bundleIds",
		RequiredDiagnostic: true,
		ErrorPrefix:        "bundle-ids delete",
		DeleteErrorPrefix:  "bundle-ids delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteBundleID(ctx, id)
		},
		Result: func(id string) any {
			return &asc.BundleIDDeleteResult{ID: id, Deleted: true}
		},
	})
}

func normalizeBundleIDListPlatforms(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		platform, err := shared.NormalizeBundleIDPlatform(value)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, string(platform))
	}
	return normalized, nil
}

func normalizeBundleIDSort(value string) (string, error) {
	values := shared.SplitCSV(value)
	for _, item := range values {
		if err := shared.ValidateSort(item, bundleIDSortValues()...); err != nil {
			return "", err
		}
	}
	return strings.Join(values, ","), nil
}

func bundleIDSortValues() []string {
	return []string{
		"name", "-name",
		"platform", "-platform",
		"identifier", "-identifier",
		"seedId", "-seedId",
		"id", "-id",
	}
}

func bundleIDFieldsList() []string {
	return []string{"name", "platform", "identifier", "seedId", "profiles", "bundleIdCapabilities", "app"}
}

func bundleIDProfileFieldsList() []string {
	return []string{
		"name", "platform", "profileType", "profileState", "profileContent", "uuid",
		"createdDate", "expirationDate", "bundleId", "devices", "certificates",
	}
}

func bundleIDCapabilityFieldsList() []string {
	return []string{"capabilityType", "settings"}
}

func bundleIDIncludeList() []string {
	return []string{"profiles", "bundleIdCapabilities", "app"}
}
