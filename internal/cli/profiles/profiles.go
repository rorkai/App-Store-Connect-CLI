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
  asc profiles view --id "PROFILE_ID"
  asc profiles view --id "PROFILE_ID" --include bundleId,certificates,devices
  asc profiles create --name "Profile" --profile-type IOS_APP_DEVELOPMENT --bundle "BUNDLE_ID" --certificate "CERT_ID"
  asc profiles delete --id "PROFILE_ID" --confirm
  asc profiles download --id "PROFILE_ID" --output "./profile.mobileprovision"
  asc profiles inspect --path "./profile.provisionprofile"
  asc profiles links bundle-id --id "PROFILE_ID"
  asc profiles links certificates --id "PROFILE_ID"
  asc profiles links devices --id "PROFILE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			ProfilesListCommand(),
			ProfilesGetCommand(),
			ProfilesRelationshipsCommand(),
			ProfilesCreateCommand(),
			ProfilesDeleteCommand(),
			ProfilesDownloadCommand(),
			ProfilesInspectCommand(),
			ProfilesLocalCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ProfilesListCommand returns the profiles list subcommand.
func ProfilesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	name := fs.String("name", "", "Filter by profile name(s), comma-separated")
	ids := fs.String("id", "", "Filter by profile ID(s), comma-separated")
	profileType := fs.String("profile-type", "", "Filter by profile type(s), comma-separated")
	profileState := fs.String("profile-state", "", "Filter by profile state(s): ACTIVE, INVALID (default: ACTIVE,INVALID)")
	sort := fs.String("sort", "", "Sort by: "+strings.Join(profileSortList(), ", "))
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(profileFieldsList(), ", "))
	bundleIDFields := fs.String("bundle-id-fields", "", "Bundle ID fields to include: "+strings.Join(profileBundleIDFieldsList(), ", "))
	deviceFields := fs.String("device-fields", "", "Device fields to include: "+strings.Join(profileDeviceFieldsList(), ", "))
	certificateFields := fs.String("certificate-fields", "", "Certificate fields to include: "+strings.Join(profileCertificateFieldsList(), ", "))
	include := fs.String("include", "", "Include related resources: "+strings.Join(profileIncludeList(), ", "))
	devicesLimit := fs.Int("limit-devices", 0, "Maximum included devices (1-50)")
	certificatesLimit := fs.Int("limit-certificates", 0, "Maximum included certificates (1-50)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc profiles list [flags]",
		ShortHelp:  "List provisioning profiles.",
		LongHelp: `List provisioning profiles.

Examples:
  asc profiles list
  asc profiles list --name "Profile"
  asc profiles list --id "PROFILE_ID"
  asc profiles list --profile-type IOS_APP_DEVELOPMENT
  asc profiles list --profile-state INVALID
  asc profiles list --include devices --device-fields name,udid --limit-devices 25
  asc profiles list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("profiles list: %v", err)
			}
			if err := shared.RejectNextFlagConflicts(
				fs,
				*next,
				"profiles list",
				"name", "id", "profile-type", "profile-state", "sort", "fields", "bundle-id-fields", "device-fields", "certificate-fields", "include", "limit-devices", "limit-certificates", "limit",
			); err != nil {
				return err
			}
			provided := map[string]bool{}
			fs.Visit(func(parsed *flag.Flag) {
				provided[parsed.Name] = true
			})
			for _, selector := range []struct {
				name  string
				value string
			}{
				{name: "name", value: *name},
				{name: "id", value: *ids},
				{name: "profile-type", value: *profileType},
				{name: "profile-state", value: *profileState},
				{name: "sort", value: *sort},
				{name: "fields", value: *fields},
				{name: "bundle-id-fields", value: *bundleIDFields},
				{name: "device-fields", value: *deviceFields},
				{name: "certificate-fields", value: *certificateFields},
				{name: "include", value: *include},
			} {
				if provided[selector.name] && len(shared.SplitCSV(selector.value)) == 0 {
					return shared.UsageErrorf("profiles list: --%s must not be empty", selector.name)
				}
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("profiles list: --limit must be between 1 and 200")
			}
			if profilesListFlagWasProvided(fs, "limit-devices") && (*devicesLimit < 1 || *devicesLimit > 50) {
				return shared.UsageError("profiles list: --limit-devices must be between 1 and 50")
			}
			if profilesListFlagWasProvided(fs, "limit-certificates") && (*certificatesLimit < 1 || *certificatesLimit > 50) {
				return shared.UsageError("profiles list: --limit-certificates must be between 1 and 50")
			}

			profileSort, err := normalizeProfileSort(*sort)
			if err != nil {
				return shared.UsageError(fmt.Sprintf("profiles list: %v", err))
			}
			profileFields, err := normalizeProfileFields(*fields, "--fields")
			if err != nil {
				return shared.UsageError(fmt.Sprintf("profiles list: %v", err))
			}
			bundleIDFieldsValue, err := normalizeProfileFieldsSelection(*bundleIDFields, profileBundleIDFieldsList(), "--bundle-id-fields")
			if err != nil {
				return shared.UsageError(fmt.Sprintf("profiles list: %v", err))
			}
			deviceFieldsValue, err := normalizeProfileFieldsSelection(*deviceFields, profileDeviceFieldsList(), "--device-fields")
			if err != nil {
				return shared.UsageError(fmt.Sprintf("profiles list: %v", err))
			}
			certificateFieldsValue, err := normalizeProfileFieldsSelection(*certificateFields, profileCertificateFieldsList(), "--certificate-fields")
			if err != nil {
				return shared.UsageError(fmt.Sprintf("profiles list: %v", err))
			}
			includeValues, err := normalizeProfileInclude(*include)
			if err != nil {
				return shared.UsageError(fmt.Sprintf("profiles list: %v", err))
			}
			if len(bundleIDFieldsValue) > 0 && !shared.HasInclude(includeValues, "bundleId") {
				const message = "--bundle-id-fields requires --include bundleId"
				fmt.Fprintln(os.Stderr, "Error: "+message)
				return shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message)
			}
			if len(deviceFieldsValue) > 0 && !shared.HasInclude(includeValues, "devices") {
				const message = "--device-fields requires --include devices"
				fmt.Fprintln(os.Stderr, "Error: "+message)
				return shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message)
			}
			if len(certificateFieldsValue) > 0 && !shared.HasInclude(includeValues, "certificates") {
				const message = "--certificate-fields requires --include certificates"
				fmt.Fprintln(os.Stderr, "Error: "+message)
				return shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message)
			}
			if *devicesLimit != 0 && !shared.HasInclude(includeValues, "devices") {
				const message = "--limit-devices requires --include devices"
				fmt.Fprintln(os.Stderr, "Error: "+message)
				return shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message)
			}
			if *certificatesLimit != 0 && !shared.HasInclude(includeValues, "certificates") {
				const message = "--limit-certificates requires --include certificates"
				fmt.Fprintln(os.Stderr, "Error: "+message)
				return shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message)
			}

			profileTypes := shared.SplitCSVUpper(*profileType)
			profileStates, err := normalizeProfileStates(*profileState)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("profiles list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.ProfilesOption{
				asc.WithProfilesLimit(*limit),
				asc.WithProfilesNextURL(*next),
				asc.WithProfilesStates(profileStates),
			}
			if strings.TrimSpace(*name) != "" {
				opts = append(opts, asc.WithProfilesFilterName(*name))
			}
			if idsValue := shared.SplitCSV(*ids); len(idsValue) > 0 {
				opts = append(opts, asc.WithProfilesFilterIDs(idsValue))
			}
			if len(profileTypes) > 0 {
				opts = append(opts, asc.WithProfilesTypes(profileTypes))
			}
			if profileSort != "" {
				opts = append(opts, asc.WithProfilesSort(profileSort))
			}
			if len(profileFields) > 0 {
				opts = append(opts, asc.WithProfilesFields(profileFields))
			}
			if len(bundleIDFieldsValue) > 0 {
				opts = append(opts, asc.WithProfilesBundleIDFields(bundleIDFieldsValue))
			}
			if len(deviceFieldsValue) > 0 {
				opts = append(opts, asc.WithProfilesDeviceFields(deviceFieldsValue))
			}
			if len(certificateFieldsValue) > 0 {
				opts = append(opts, asc.WithProfilesCertificateFields(certificateFieldsValue))
			}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithProfilesInclude(includeValues))
			}
			if *devicesLimit > 0 {
				opts = append(opts, asc.WithProfilesDevicesLimit(*devicesLimit))
			}
			if *certificatesLimit > 0 {
				opts = append(opts, asc.WithProfilesCertificatesLimit(*certificatesLimit))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithProfilesLimit(200))
				paginated, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetProfiles(ctx, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetProfiles(ctx, asc.WithProfilesNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("profiles list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetProfiles(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("profiles list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func profilesListFlagWasProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(parsed *flag.Flag) {
		if parsed.Name == name {
			provided = true
		}
	})
	return provided
}

// ProfilesGetCommand returns the profiles view subcommand.
func ProfilesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	id := fs.String("id", "", "Profile ID")
	include := fs.String("include", "", "Include related resources: bundleId, certificates, devices")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc profiles view --id \"PROFILE_ID\"",
		ShortHelp:  "View a profile by ID.",
		LongHelp: `View a profile by ID.

Examples:
  asc profiles view --id "PROFILE_ID"
  asc profiles view --id "PROFILE_ID" --include bundleId,certificates,devices`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			includeValues, err := normalizeProfileInclude(*include)
			if err != nil {
				return fmt.Errorf("profiles view: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("profiles view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.ProfilesOption{}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithProfilesInclude(includeValues))
			}

			resp, err := client.GetProfile(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("profiles view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ProfilesCreateCommand returns the profiles create subcommand.
func ProfilesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	name := fs.String("name", "", "Profile name")
	profileType := fs.String("profile-type", "", "Profile type (e.g., IOS_APP_DEVELOPMENT)")
	bundleID := fs.String("bundle", "", "Bundle ID")
	certificates := shared.BindOnceCSVFlag(fs, "certificate", "Certificate ID(s), comma-separated")
	devices := shared.BindOnceCSVFlag(fs, "device", "Device ID(s), comma-separated (optional)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc profiles create --name \"Profile\" --profile-type TYPE --bundle \"BUNDLE_ID\" --certificate \"CERT_ID[,CERT_ID...]\"",
		ShortHelp:  "Create a provisioning profile.",
		LongHelp: `Create a provisioning profile.

Examples:
  asc profiles create --name "Profile" --profile-type IOS_APP_DEVELOPMENT --bundle "BUNDLE_ID" --certificate "CERT_ID"
  asc profiles create --name "Profile" --profile-type IOS_APP_DEVELOPMENT --bundle "BUNDLE_ID" --certificate "CERT_ID" --device "DEVICE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return shared.MissingRequiredUsageError("--name")
			}
			profileTypeValue := strings.ToUpper(strings.TrimSpace(*profileType))
			if profileTypeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --profile-type is required")
				return shared.MissingRequiredUsageError("--profile-type")
			}
			bundleValue := strings.TrimSpace(*bundleID)
			if bundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --bundle is required")
				return shared.MissingRequiredUsageError("--bundle")
			}
			certificateIDs := shared.SplitCSV(certificates.String())
			if len(certificateIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --certificate is required")
				return shared.MissingRequiredUsageError("--certificate")
			}
			deviceIDs := shared.SplitCSV(devices.String())

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("profiles create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.ProfileCreateAttributes{
				Name:        nameValue,
				ProfileType: profileTypeValue,
			}
			resp, err := client.CreateProfile(requestCtx, attrs, bundleValue, certificateIDs, deviceIDs)
			if err != nil {
				return fmt.Errorf("profiles create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ProfilesDeleteCommand returns the profiles delete subcommand.
func ProfilesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Profile ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc profiles delete --id \"PROFILE_ID\" --confirm",
		ShortHelp:  "Delete a provisioning profile.",
		LongHelp: `Delete a provisioning profile.

Examples:
  asc profiles delete --id "PROFILE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("profiles delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteProfile(requestCtx, idValue); err != nil {
				return fmt.Errorf("profiles delete: failed to delete: %w", err)
			}

			result := &asc.ProfileDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// ProfilesDownloadCommand returns the profiles download subcommand.
func ProfilesDownloadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("download", flag.ExitOnError)

	id := fs.String("id", "", "Profile ID")
	outputPath := fs.String("output", "", "Output .mobileprovision or .provisionprofile file path")
	output := shared.BindMetadataOutputFlags(fs)

	return &ffcli.Command{
		Name:       "download",
		ShortUsage: "asc profiles download --id \"PROFILE_ID\" --output ./profile.mobileprovision",
		ShortHelp:  "Download a provisioning profile.",
		LongHelp: `Download a provisioning profile.

Examples:
  asc profiles download --id "PROFILE_ID" --output "./profile.mobileprovision"
  asc profiles download --id "PROFILE_ID" --output "./profile.provisionprofile"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			pathValue := strings.TrimSpace(*outputPath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --output is required")
				return shared.MissingRequiredUsageError("--output")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("profiles download: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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

			return shared.PrintOutput(result, *output.OutputFormat, *output.Pretty)
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
	include := shared.SplitCSV(value)
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

func normalizeProfileSort(value string) (string, error) {
	sortValues := shared.SplitCSV(value)
	if len(sortValues) == 0 {
		return "", nil
	}
	allowed := make(map[string]struct{}, len(profileSortList()))
	for _, item := range profileSortList() {
		allowed[item] = struct{}{}
	}
	for _, item := range sortValues {
		if _, ok := allowed[item]; !ok {
			return "", fmt.Errorf("--sort must be one of: %s", strings.Join(profileSortList(), ", "))
		}
	}
	return strings.Join(sortValues, ","), nil
}

func normalizeProfileFields(value, flagName string) ([]string, error) {
	return normalizeProfileFieldsSelection(value, profileFieldsList(), flagName)
}

func normalizeProfileFieldsSelection(value string, allowed []string, flagName string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		allowedSet[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowedSet[field]; !ok {
			return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(allowed, ", "))
		}
	}
	return fields, nil
}

func profileSortList() []string {
	return []string{"name", "-name", "profileType", "-profileType", "profileState", "-profileState", "id", "-id"}
}

func profileFieldsList() []string {
	return []string{"name", "platform", "profileType", "profileState", "profileContent", "uuid", "createdDate", "expirationDate", "bundleId", "devices", "certificates"}
}

func profileBundleIDFieldsList() []string {
	return []string{"name", "platform", "identifier", "seedId", "profiles", "bundleIdCapabilities", "app"}
}

func profileDeviceFieldsList() []string {
	return []string{"name", "platform", "udid", "deviceClass", "status", "model", "addedDate"}
}

func profileCertificateFieldsList() []string {
	return []string{"name", "certificateType", "displayName", "serialNumber", "platform", "expirationDate", "certificateContent", "activated", "passTypeId"}
}

func normalizeProfileStates(value string) ([]string, error) {
	states := shared.SplitCSVUpper(value)
	if len(states) == 0 {
		return defaultProfileStates(), nil
	}
	allowed := map[string]struct{}{}
	for _, item := range profileStateList() {
		allowed[item] = struct{}{}
	}
	for _, item := range states {
		if _, ok := allowed[item]; !ok {
			return nil, fmt.Errorf("--profile-state must be one of: %s", strings.Join(profileStateList(), ", "))
		}
	}
	return states, nil
}

func defaultProfileStates() []string {
	return []string{string(asc.ProfileStateActive), string(asc.ProfileStateInvalid)}
}

func profileStateList() []string {
	return []string{string(asc.ProfileStateActive), string(asc.ProfileStateInvalid)}
}
