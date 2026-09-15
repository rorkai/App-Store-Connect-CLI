package bundleids

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BundleIDsCapabilitiesCommand returns the bundle IDs capabilities command group.
func BundleIDsCapabilitiesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("capabilities", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "capabilities",
		ShortUsage: "asc bundle-ids capabilities <subcommand> [flags]",
		ShortHelp:  "Manage bundle ID capabilities.",
		LongHelp: `Manage bundle ID capabilities.

Examples:
  asc bundle-ids capabilities list --bundle "BUNDLE_ID"
  asc bundle-ids capabilities add --bundle "BUNDLE_ID" --capability ICLOUD
  asc bundle-ids capabilities update --id "CAPABILITY_ID" --settings '[{"key":"ICLOUD_VERSION","options":[{"key":"XCODE_6","enabled":true}]}]'
  asc bundle-ids capabilities remove --id "CAPABILITY_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BundleIDsCapabilitiesListCommand(),
			BundleIDsCapabilitiesAddCommand(),
			BundleIDsCapabilitiesUpdateCommand(),
			BundleIDsCapabilitiesRemoveCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BundleIDsCapabilitiesListCommand returns the bundle IDs capabilities list subcommand.
func BundleIDsCapabilitiesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	bundleID := shared.BindResourceIDFlag(fs, "bundle", "bundleIds", "Bundle ID")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc bundle-ids capabilities list --bundle \"BUNDLE_ID\" [flags]",
		ShortHelp:  "List bundle ID capabilities.",
		LongHelp: `List bundle ID capabilities.

Examples:
  asc bundle-ids capabilities list --bundle "BUNDLE_ID"
  asc bundle-ids capabilities list --bundle "BUNDLE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("bundle-ids capabilities list: %v", err)
			}
			bundleValue := strings.TrimSpace(*bundleID)
			if bundleValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --bundle is required")
				return shared.MissingRequiredUsageError("--bundle")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BundleIDCapabilitiesOption{
				asc.WithBundleIDCapabilitiesNextURL(*next),
			}

			if *paginate {
				firstPage, err := client.GetBundleIDCapabilities(requestCtx, bundleValue, opts...)
				if err != nil {
					return fmt.Errorf("bundle-ids capabilities list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBundleIDCapabilities(ctx, bundleValue, asc.WithBundleIDCapabilitiesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("bundle-ids capabilities list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBundleIDCapabilities(requestCtx, bundleValue, opts...)
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BundleIDsCapabilitiesAddCommand returns the bundle IDs capabilities add subcommand.
func BundleIDsCapabilitiesAddCommand() *ffcli.Command {
	fs := flag.NewFlagSet("add", flag.ExitOnError)

	bundleID := shared.BindResourceIDFlag(fs, "bundle", "bundleIds", "Bundle ID")
	capability := fs.String("capability", "", "Capability type (e.g., ICLOUD, IN_APP_PURCHASE)")
	settings := fs.String("settings", "", "Capability settings as a structure-validated JSON array (optional)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add",
		ShortUsage: "asc bundle-ids capabilities add --bundle \"BUNDLE_ID\" --capability CAPABILITY_TYPE [flags]",
		ShortHelp:  "Add a capability to a bundle ID.",
		LongHelp: `Add a capability to a bundle ID.

Settings require exact JSON field names and value types. Setting and option key
strings are sent unchanged so values newer than Apple's published schema work.

Examples:
  asc bundle-ids capabilities add --bundle "BUNDLE_ID" --capability ICLOUD
  asc bundle-ids capabilities add --bundle "BUNDLE_ID" --capability ICLOUD --settings '[{"key":"ICLOUD_VERSION","options":[{"key":"XCODE_6","enabled":true}]}]'`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.RejectPositionalArgs(args); err != nil {
				return err
			}
			bundleValue := strings.TrimSpace(*bundleID)
			if bundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --bundle is required")
				return shared.MissingRequiredUsageError("--bundle")
			}
			capabilityValue := strings.ToUpper(strings.TrimSpace(*capability))
			if capabilityValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --capability is required")
				return shared.MissingRequiredUsageError("--capability")
			}

			settingsValue, err := parseCapabilitySettings(*settings)
			if err != nil {
				return shared.UsageErrorf("bundle-ids capabilities add: %v", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities add: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BundleIDCapabilityCreateAttributes{
				CapabilityType: capabilityValue,
				Settings:       settingsValue,
			}
			resp, err := client.CreateBundleIDCapability(requestCtx, bundleValue, attrs)
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities add: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BundleIDsCapabilitiesUpdateCommand returns the bundle IDs capabilities update subcommand.
func BundleIDsCapabilitiesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "Capability ID")
	capabilityType := fs.String("capability", "", "Capability type (e.g., ICLOUD, IN_APP_PURCHASE)")
	settings := fs.String("settings", "", "Capability settings as a structure-validated JSON array")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc bundle-ids capabilities update --id \"CAPABILITY_ID\" [flags]",
		ShortHelp:  "Update a bundle ID capability.",
		LongHelp: `Update a bundle ID capability.

Settings require exact JSON field names and value types. Setting and option key
strings are sent unchanged so values newer than Apple's published schema work.

Examples:
  asc bundle-ids capabilities update --id "CAPABILITY_ID" --settings '[{"key":"ICLOUD_VERSION","options":[{"key":"XCODE_6","enabled":true}]}]'
  asc bundle-ids capabilities update --id "CAPABILITY_ID" --capability PUSH_NOTIFICATIONS
  asc bundle-ids capabilities update --id "CAPABILITY_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.RejectPositionalArgs(args); err != nil {
				return err
			}
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			capabilityValue := strings.ToUpper(strings.TrimSpace(*capabilityType))
			settingsValue, err := parseCapabilitySettings(*settings)
			if err != nil {
				return shared.UsageErrorf("bundle-ids capabilities update: %v", err)
			}

			// Treat empty settings arrays as no-op updates.
			if capabilityValue == "" && len(settingsValue) == 0 {
				fmt.Fprintln(os.Stderr, "Error: at least one update field is required (--capability or --settings)")
				return shared.MissingRequiredUsageError("")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BundleIDCapabilityUpdateAttributes{
				CapabilityType: capabilityValue,
				Settings:       settingsValue,
			}
			resp, err := client.UpdateBundleIDCapability(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BundleIDsCapabilitiesRemoveCommand returns the bundle IDs capabilities remove subcommand.
func BundleIDsCapabilitiesRemoveCommand() *ffcli.Command {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)

	id := fs.String("id", "", "Capability ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove",
		ShortUsage: "asc bundle-ids capabilities remove --id \"CAPABILITY_ID\" --confirm",
		ShortHelp:  "Remove a capability from a bundle ID.",
		LongHelp: `Remove a capability from a bundle ID.

Examples:
  asc bundle-ids capabilities remove --id "CAPABILITY_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.RejectPositionalArgs(args); err != nil {
				return err
			}
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
				return fmt.Errorf("bundle-ids capabilities remove: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBundleIDCapability(requestCtx, idValue); err != nil {
				return fmt.Errorf("bundle-ids capabilities remove: failed to delete: %w", err)
			}

			result := &asc.BundleIDCapabilityDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func parseCapabilitySettings(value string) ([]asc.CapabilitySetting, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	var settings []asc.CapabilitySetting
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&settings); err != nil {
		return nil, fmt.Errorf("--settings must be valid JSON array: %w", err)
	}
	if settings == nil {
		return nil, fmt.Errorf("--settings must be a JSON array, got null")
	}
	var rawSettings any
	if err := json.Unmarshal([]byte(trimmed), &rawSettings); err != nil {
		return nil, fmt.Errorf("--settings must be valid JSON array: %w", err)
	}
	if err := rejectCapabilitySettingsNulls(rawSettings, "settings"); err != nil {
		return nil, fmt.Errorf("--settings: %w", err)
	}
	if err := validateCapabilitySettingsJSON(rawSettings); err != nil {
		return nil, fmt.Errorf("--settings: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("--settings must contain one JSON array")
	}
	if err := validateCapabilitySettings(settings); err != nil {
		return nil, fmt.Errorf("--settings: %w", err)
	}
	return settings, nil
}

var capabilitySettingFields = []string{
	"allowedInstances",
	"description",
	"enabledByDefault",
	"key",
	"minInstances",
	"name",
	"options",
	"visible",
}

var capabilityOptionFields = []string{
	"description",
	"enabled",
	"enabledByDefault",
	"key",
	"name",
	"supportsWildcard",
}

func validateCapabilitySettingsJSON(value any) error {
	settings, ok := value.([]any)
	if !ok {
		return nil
	}
	for settingIndex, item := range settings {
		setting, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for field := range setting {
			if !slices.Contains(capabilitySettingFields, field) {
				return fmt.Errorf("unknown field %q at setting index %d", field, settingIndex)
			}
		}
		settingLocation := fmt.Sprintf("setting index %d", settingIndex)
		for _, field := range []string{"allowedInstances", "description", "name"} {
			if err := validateCapabilityOptionalString(setting, field, settingLocation); err != nil {
				return err
			}
		}
		options, ok := setting["options"].([]any)
		if !ok {
			continue
		}
		for optionIndex, item := range options {
			option, ok := item.(map[string]any)
			if !ok {
				continue
			}
			for field := range option {
				if !slices.Contains(capabilityOptionFields, field) {
					return fmt.Errorf("unknown field %q at setting index %d, option index %d", field, settingIndex, optionIndex)
				}
			}
			optionLocation := fmt.Sprintf("setting index %d, option index %d", settingIndex, optionIndex)
			for _, field := range []string{"description", "name"} {
				if err := validateCapabilityOptionalString(option, field, optionLocation); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateCapabilityOptionalString(object map[string]any, field, location string) error {
	raw, present := object[field]
	if !present {
		return nil
	}
	value, ok := raw.(string)
	if !ok {
		return nil
	}
	if value == "" {
		return fmt.Errorf("%s at %s must not be empty", field, location)
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s at %s must not be blank", field, location)
	}
	return nil
}

func rejectCapabilitySettingsNulls(value any, path string) error {
	switch typed := value.(type) {
	case nil:
		return fmt.Errorf("%s must not be null", path)
	case []any:
		for index, item := range typed {
			if err := rejectCapabilitySettingsNulls(item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	case map[string]any:
		for key, item := range typed {
			if err := rejectCapabilitySettingsNulls(item, path+"."+key); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateCapabilitySettings(settings []asc.CapabilitySetting) error {
	for settingIndex, setting := range settings {
		if strings.TrimSpace(setting.Key) == "" {
			return fmt.Errorf("capability setting key at index %d must not be empty", settingIndex)
		}
		for optionIndex, option := range setting.Options {
			if strings.TrimSpace(option.Key) == "" {
				return fmt.Errorf("capability option key at setting index %d, option index %d must not be empty", settingIndex, optionIndex)
			}
		}
	}
	return nil
}
