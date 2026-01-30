package profiles

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ProfilesCommand returns the profiles command with subcommands.
func ProfilesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("profiles", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "profiles",
		ShortUsage: "asc profiles <subcommand> [flags]",
		ShortHelp:  "Manage provisioning profiles.",
		LongHelp: `Manage provisioning profiles.

Examples:
  asc profiles list
  asc profiles list --profile-type IOS_APP_DEVELOPMENT
  asc profiles get --id "PROFILE_ID"
  asc profiles get --id "PROFILE_ID" --include bundleId,certificates,devices
  asc profiles create --name "Profile" --profile-type IOS_APP_DEVELOPMENT --bundle "BUNDLE_ID" --certificate "CERT_ID"
  asc profiles delete --id "PROFILE_ID" --confirm
  asc profiles download --id "PROFILE_ID" --output "./profile.mobileprovision"
  asc profiles relationships bundle-id --id "PROFILE_ID"
  asc profiles relationships certificates --id "PROFILE_ID"
  asc profiles relationships devices --id "PROFILE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ProfilesListCommand(),
			ProfilesGetCommand(),
			ProfilesRelationshipsCommand(),
			ProfilesCreateCommand(),
			ProfilesDeleteCommand(),
			ProfilesDownloadCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ProfilesListCommand returns the profiles list subcommand.
func ProfilesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	profileType := fs.String("profile-type", "", "Filter by profile type(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc profiles list [flags]",
		ShortHelp:  "List provisioning profiles.",
		LongHelp: `List provisioning profiles.

Examples:
  asc profiles list
  asc profiles list --profile-type IOS_APP_DEVELOPMENT
  asc profiles list --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("profiles list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("profiles list: %w", err)
			}

			profileTypes := splitCSVUpper(*profileType)

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("profiles list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.ProfilesOption{
				asc.WithProfilesLimit(*limit),
				asc.WithProfilesNextURL(*next),
			}
			if len(profileTypes) > 0 {
				opts = append(opts, asc.WithProfilesTypes(profileTypes))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithProfilesLimit(200))
				firstPage, err := client.GetProfiles(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("profiles list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetProfiles(ctx, asc.WithProfilesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("profiles list: %w", err)
				}

				return printOutput(paginated, *output, *pretty)
			}

			resp, err := client.GetProfiles(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("profiles list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ProfilesGetCommand returns the profiles get subcommand.
func ProfilesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Profile ID")
	include := fs.String("include", "", "Include related resources: bundleId, certificates, devices")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc profiles get --id \"PROFILE_ID\"",
		ShortHelp:  "Get a profile by ID.",
		LongHelp: `Get a profile by ID.

Examples:
  asc profiles get --id "PROFILE_ID"
  asc profiles get --id "PROFILE_ID" --include bundleId,certificates,devices`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			includeValues, err := normalizeProfileInclude(*include)
			if err != nil {
				return fmt.Errorf("profiles get: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("profiles get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.ProfilesOption{}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithProfilesInclude(includeValues))
			}

			resp, err := client.GetProfile(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("profiles get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ProfilesCreateCommand returns the profiles create subcommand.
func ProfilesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	name := fs.String("name", "", "Profile name")
	profileType := fs.String("profile-type", "", "Profile type (e.g., IOS_APP_DEVELOPMENT)")
	bundleID := fs.String("bundle", "", "Bundle ID")
	certificates := fs.String("certificate", "", "Certificate ID(s), comma-separated")
	devices := fs.String("device", "", "Device ID(s), comma-separated (optional)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc profiles create --name \"Profile\" --profile-type TYPE --bundle \"BUNDLE_ID\" --certificate \"CERT_ID[,CERT_ID...]\"",
		ShortHelp:  "Create a provisioning profile.",
		LongHelp: `Create a provisioning profile.

Examples:
  asc profiles create --name "Profile" --profile-type IOS_APP_DEVELOPMENT --bundle "BUNDLE_ID" --certificate "CERT_ID"
  asc profiles create --name "Profile" --profile-type IOS_APP_DEVELOPMENT --bundle "BUNDLE_ID" --certificate "CERT_ID" --device "DEVICE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}
			profileTypeValue := strings.ToUpper(strings.TrimSpace(*profileType))
			if profileTypeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --profile-type is required")
				return flag.ErrHelp
			}
			bundleValue := strings.TrimSpace(*bundleID)
			if bundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --bundle is required")
				return flag.ErrHelp
			}
			certificateIDs := splitCSV(*certificates)
			if len(certificateIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --certificate is required")
				return flag.ErrHelp
			}
			deviceIDs := splitCSV(*devices)

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("profiles create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.ProfileCreateAttributes{
				Name:        nameValue,
				ProfileType: profileTypeValue,
			}
			resp, err := client.CreateProfile(requestCtx, attrs, bundleValue, certificateIDs, deviceIDs)
			if err != nil {
				return fmt.Errorf("profiles create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ProfilesDeleteCommand returns the profiles delete subcommand.
func ProfilesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Profile ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc profiles delete --id \"PROFILE_ID\" --confirm",
		ShortHelp:  "Delete a provisioning profile.",
		LongHelp: `Delete a provisioning profile.

Examples:
  asc profiles delete --id "PROFILE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("profiles delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteProfile(requestCtx, idValue); err != nil {
				return fmt.Errorf("profiles delete: failed to delete: %w", err)
			}

			result := &asc.ProfileDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return printOutput(result, *output, *pretty)
		},
	}
}

// ProfilesDownloadCommand returns the profiles download subcommand.
func ProfilesDownloadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("download", flag.ExitOnError)

	id := fs.String("id", "", "Profile ID")
	outputPath := fs.String("output", "", "Output .mobileprovision file path")
	output := fs.String("output-format", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "download",
		ShortUsage: "asc profiles download --id \"PROFILE_ID\" --output ./profile.mobileprovision",
		ShortHelp:  "Download a provisioning profile.",
		LongHelp: `Download a provisioning profile.

Examples:
  asc profiles download --id "PROFILE_ID" --output "./profile.mobileprovision"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			pathValue := strings.TrimSpace(*outputPath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --output is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("profiles download: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetProfile(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("profiles download: failed to fetch: %w", err)
			}

			content := strings.TrimSpace(resp.Data.Attributes.ProfileContent)
			if content == "" {
				return fmt.Errorf("profiles download: profile content is empty")
			}

			decoded, err := decodeProfileContent(content)
			if err != nil {
				return fmt.Errorf("profiles download: %w", err)
			}

			if err := shared.WriteProfileFile(pathValue, decoded); err != nil {
				return fmt.Errorf("profiles download: %w", err)
			}

			result := &asc.ProfileDownloadResult{
				ID:         idValue,
				Name:       resp.Data.Attributes.Name,
				OutputPath: pathValue,
			}

			return printOutput(result, *output, *pretty)
		},
	}
}

func decodeProfileContent(content string) ([]byte, error) {
	normalized := strings.Join(strings.Fields(content), "")
	if normalized == "" {
		return nil, fmt.Errorf("profile content is empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func normalizeProfileInclude(value string) ([]string, error) {
	include := splitCSV(value)
	if len(include) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, item := range profileIncludeList() {
		allowed[item] = struct{}{}
	}
	for _, item := range include {
		if _, ok := allowed[item]; !ok {
			return nil, fmt.Errorf("--include must be one of: %s", strings.Join(profileIncludeList(), ", "))
		}
	}
	return include, nil
}

func profileIncludeList() []string {
	return []string{"bundleId", "certificates", "devices"}
}
