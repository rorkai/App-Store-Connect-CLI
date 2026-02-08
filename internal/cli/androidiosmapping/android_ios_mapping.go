package androidiosmapping

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

// AndroidIosMappingCommand returns the android-to-iOS mapping command group.
func AndroidIosMappingCommand() *ffcli.Command {
	fs := flag.NewFlagSet("android-ios-mapping", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "android-ios-mapping",
		ShortUsage: "asc android-ios-mapping <subcommand> [flags]",
		ShortHelp:  "Manage Android-to-iOS app mapping details.",
		LongHelp: `Manage Android-to-iOS app mapping details.

Examples:
  asc android-ios-mapping list --app "APP_ID"
  asc android-ios-mapping get --mapping-id "MAPPING_ID"
  asc android-ios-mapping create --app "APP_ID" --android-package-name "com.example.android" --fingerprints "SHA1,SHA2"
  asc android-ios-mapping update --mapping-id "MAPPING_ID" --android-package-name "com.example.android.new"
  asc android-ios-mapping delete --mapping-id "MAPPING_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AndroidIosMappingListCommand(),
			AndroidIosMappingGetCommand(),
			AndroidIosMappingCreateCommand(),
			AndroidIosMappingUpdateCommand(),
			AndroidIosMappingDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AndroidIosMappingListCommand returns the mapping list subcommand.
func AndroidIosMappingListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	fields := fs.String("fields", "", "Fields to return (comma-separated: "+strings.Join(androidIosMappingFieldsList(), ", ")+")")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc android-ios-mapping list --app \"APP_ID\"",
		ShortHelp:  "List Android-to-iOS app mappings for an app.",
		LongHelp: `List Android-to-iOS app mappings for an app.

Examples:
  asc android-ios-mapping list --app "APP_ID"
  asc android-ios-mapping list --app "APP_ID" --limit 10`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("android-ios-mapping list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("android-ios-mapping list: %w", err)
			}
			fieldValues, err := normalizeAndroidIosMappingFields(*fields)
			if err != nil {
				return fmt.Errorf("android-ios-mapping list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("android-ios-mapping list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AndroidToIosAppMappingDetailsOption{
				asc.WithAndroidToIosAppMappingDetailsFields(fieldValues),
				asc.WithAndroidToIosAppMappingDetailsLimit(*limit),
				asc.WithAndroidToIosAppMappingDetailsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAndroidToIosAppMappingDetailsLimit(200))
				firstPage, err := client.GetAndroidToIosAppMappingDetails(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("android-ios-mapping list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAndroidToIosAppMappingDetails(ctx, resolvedAppID, asc.WithAndroidToIosAppMappingDetailsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("android-ios-mapping list: %w", err)
				}

				return shared.PrintOutput(paginated, *output, *pretty)
			}

			resp, err := client.GetAndroidToIosAppMappingDetails(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("android-ios-mapping list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AndroidIosMappingGetCommand returns the mapping get subcommand.
func AndroidIosMappingGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("mapping-id", "", "Mapping ID")
	fields := fs.String("fields", "", "Fields to return (comma-separated: "+strings.Join(androidIosMappingFieldsList(), ", ")+")")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc android-ios-mapping get --mapping-id \"MAPPING_ID\"",
		ShortHelp:  "Get an Android-to-iOS app mapping by ID.",
		LongHelp: `Get an Android-to-iOS app mapping by ID.

Examples:
  asc android-ios-mapping get --mapping-id "MAPPING_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*id) == "" {
				fmt.Fprintln(os.Stderr, "Error: --mapping-id is required")
				return flag.ErrHelp
			}
			fieldValues, err := normalizeAndroidIosMappingFields(*fields)
			if err != nil {
				return fmt.Errorf("android-ios-mapping get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("android-ios-mapping get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAndroidToIosAppMappingDetail(requestCtx, strings.TrimSpace(*id),
				asc.WithAndroidToIosAppMappingDetailsFields(fieldValues),
			)
			if err != nil {
				return fmt.Errorf("android-ios-mapping get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AndroidIosMappingCreateCommand returns the mapping create subcommand.
func AndroidIosMappingCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	packageName := fs.String("android-package-name", "", "Android package name (e.g., com.example.android)")
	fingerprints := fs.String("fingerprints", "", "Signing key fingerprints (comma-separated)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc android-ios-mapping create --app \"APP_ID\" --android-package-name \"com.example.android\" --fingerprints \"SHA1,SHA2\"",
		ShortHelp:  "Create an Android-to-iOS app mapping.",
		LongHelp: `Create an Android-to-iOS app mapping.

Examples:
  asc android-ios-mapping create --app "APP_ID" --android-package-name "com.example.android" --fingerprints "SHA1,SHA2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			packageValue := strings.TrimSpace(*packageName)
			if packageValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --android-package-name is required")
				return flag.ErrHelp
			}
			fingerprintValues := shared.SplitCSV(*fingerprints)
			if len(fingerprintValues) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --fingerprints is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("android-ios-mapping create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAndroidToIosAppMappingDetail(requestCtx, resolvedAppID, asc.AndroidToIosAppMappingDetailCreateAttributes{
				PackageName: packageValue,
				AppSigningKeyPublicCertificateSha256Fingerprints: fingerprintValues,
			})
			if err != nil {
				return fmt.Errorf("android-ios-mapping create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AndroidIosMappingUpdateCommand returns the mapping update subcommand.
func AndroidIosMappingUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("mapping-id", "", "Mapping ID")
	packageName := fs.String("android-package-name", "", "Android package name (e.g., com.example.android)")
	fingerprints := fs.String("fingerprints", "", "Signing key fingerprints (comma-separated)")
	clearPackageName := fs.Bool("clear-android-package-name", false, "Clear the Android package name")
	clearFingerprints := fs.Bool("clear-fingerprints", false, "Clear signing key fingerprints")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc android-ios-mapping update --mapping-id \"MAPPING_ID\" [flags]",
		ShortHelp:  "Update an Android-to-iOS app mapping.",
		LongHelp: `Update an Android-to-iOS app mapping.

Examples:
  asc android-ios-mapping update --mapping-id "MAPPING_ID" --android-package-name "com.example.android.new"
  asc android-ios-mapping update --mapping-id "MAPPING_ID" --fingerprints "SHA1,SHA2"
  asc android-ios-mapping update --mapping-id "MAPPING_ID" --clear-android-package-name
  asc android-ios-mapping update --mapping-id "MAPPING_ID" --clear-fingerprints`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*id)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --mapping-id is required")
				return flag.ErrHelp
			}

			seen := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				seen[f.Name] = true
			})
			if !seen["android-package-name"] && !seen["fingerprints"] && !*clearPackageName && !*clearFingerprints {
				return fmt.Errorf("android-ios-mapping update: at least one update flag is required")
			}
			if seen["android-package-name"] && *clearPackageName {
				return fmt.Errorf("android-ios-mapping update: --android-package-name cannot be used with --clear-android-package-name")
			}
			if seen["fingerprints"] && *clearFingerprints {
				return fmt.Errorf("android-ios-mapping update: --fingerprints cannot be used with --clear-fingerprints")
			}

			var attrs asc.AndroidToIosAppMappingDetailUpdateAttributes
			if seen["android-package-name"] {
				packageValue := strings.TrimSpace(*packageName)
				if packageValue == "" {
					return fmt.Errorf("android-ios-mapping update: --android-package-name cannot be empty")
				}
				attrs.PackageName = &asc.NullableString{Value: &packageValue}
			}
			if *clearPackageName {
				attrs.PackageName = &asc.NullableString{}
			}
			if seen["fingerprints"] {
				fingerprintValues := shared.SplitCSV(*fingerprints)
				if len(fingerprintValues) == 0 {
					return fmt.Errorf("android-ios-mapping update: --fingerprints must include at least one value")
				}
				attrs.AppSigningKeyPublicCertificateSha256Fingerprints = &asc.NullableStringSlice{Value: fingerprintValues}
			}
			if *clearFingerprints {
				attrs.AppSigningKeyPublicCertificateSha256Fingerprints = &asc.NullableStringSlice{}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("android-ios-mapping update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAndroidToIosAppMappingDetail(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("android-ios-mapping update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AndroidIosMappingDeleteCommand returns the mapping delete subcommand.
func AndroidIosMappingDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("mapping-id", "", "Mapping ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc android-ios-mapping delete --mapping-id \"MAPPING_ID\" --confirm",
		ShortHelp:  "Delete an Android-to-iOS app mapping.",
		LongHelp: `Delete an Android-to-iOS app mapping.

Examples:
  asc android-ios-mapping delete --mapping-id "MAPPING_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*id)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --mapping-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("android-ios-mapping delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAndroidToIosAppMappingDetail(requestCtx, trimmedID); err != nil {
				return fmt.Errorf("android-ios-mapping delete: %w", err)
			}

			result := &asc.AndroidToIosAppMappingDeleteResult{
				ID:      trimmedID,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func androidIosMappingFieldsList() []string {
	return []string{
		"packageName",
		"appSigningKeyPublicCertificateSha256Fingerprints",
	}
}

func normalizeAndroidIosMappingFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, field := range androidIosMappingFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(androidIosMappingFieldsList(), ", "))
		}
	}

	return fields, nil
}
