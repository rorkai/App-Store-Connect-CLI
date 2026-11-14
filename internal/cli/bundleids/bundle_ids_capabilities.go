package bundleids

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newBundleIDsCapabilitiesCommand returns the bundle IDs capabilities command group.
func newBundleIDsCapabilitiesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("capabilities", flag.ExitOnError)

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
			newBundleIDsCapabilitiesListCommand(d),
			newBundleIDsCapabilitiesAddCommand(d),
			newBundleIDsCapabilitiesUpdateCommand(d),
			newBundleIDsCapabilitiesRemoveCommand(d),
			newBundleIDsCapabilitiesReconcileCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBundleIDsCapabilitiesListCommand returns the bundle IDs capabilities list subcommand.
func newBundleIDsCapabilitiesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	bundleID := shared.BindResourceIDFlag(fs, "bundle", "bundleIds", "Bundle ID")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

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
				return d.UsageErrorf("bundle-ids capabilities list: %v", err)
			}
			bundleValue := strings.TrimSpace(*bundleID)
			if bundleValue == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--bundle")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BundleIDCapabilitiesOption{
				asc.NextURL[asc.BundleIDCapabilitiesOption](*next),
			}

			if *paginate {
				firstPage, err := client.GetBundleIDCapabilities(requestCtx, bundleValue, opts...)
				if err != nil {
					return fmt.Errorf("bundle-ids capabilities list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBundleIDCapabilities(ctx, bundleValue, asc.NextURL[asc.BundleIDCapabilitiesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("bundle-ids capabilities list: %w", err)
				}

				return d.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBundleIDCapabilities(requestCtx, bundleValue, opts...)
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBundleIDsCapabilitiesAddCommand returns the bundle IDs capabilities add subcommand.
func newBundleIDsCapabilitiesAddCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("add", flag.ExitOnError)

	bundleID := shared.BindResourceIDFlag(fs, "bundle", "bundleIds", "Bundle ID")
	capability := fs.String("capability", "", "Capability type (e.g., ICLOUD, IN_APP_PURCHASE)")
	settings := fs.String("settings", "", "Capability settings as a structure-validated JSON array (optional)")
	ifExists := shared.BindIfExistsFlag(fs, shared.IfExistsSkip, shared.IfExistsUpdate)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add",
		ShortUsage: "asc bundle-ids capabilities add --bundle \"BUNDLE_ID\" --capability CAPABILITY_TYPE [flags]",
		ShortHelp:  "Add a capability to a bundle ID.",
		LongHelp: `Add a capability to a bundle ID.

Settings require exact JSON field names and value types. Setting and option key
strings are sent unchanged so values newer than Apple's published schema work.

Examples:
  asc bundle-ids capabilities add --bundle "BUNDLE_ID" --capability ICLOUD
  asc bundle-ids capabilities add --bundle "BUNDLE_ID" --capability ICLOUD --settings '[{"key":"ICLOUD_VERSION","options":[{"key":"XCODE_6","enabled":true}]}]'
  asc bundle-ids capabilities add --bundle "BUNDLE_ID" --capability ICLOUD --if-exists skip

--if-exists controls what happens when App Store Connect answers 409 because
the capability is already enabled on the bundle ID. Apple usually accepts a
repeated add of an API-creatable capability as success and returns the
existing capability, so the flag engages only when Apple reports the duplicate
as a 409. fail (default) returns the error. skip reads the existing capability
back, prints it, and exits 0 without changing it. update applies --settings to
the existing capability with PATCH /v1/bundleIdCapabilities/{id}; with no
--settings there is nothing to apply, so update behaves like skip. Any other
409 keeps failing, including ENTITY_ERROR.ATTRIBUTE.TYPE for a capability type
the API cannot create, even when that capability is already enabled.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			bundleValue := strings.TrimSpace(*bundleID)
			if bundleValue == "" {
				return d.RequiredFlag("--bundle")
			}
			capabilityValue := strings.ToUpper(strings.TrimSpace(*capability))
			if capabilityValue == "" {
				return d.RequiredFlag("--capability")
			}

			settingsValue, err := parseCapabilitySettings(*settings)
			if err != nil {
				return d.UsageErrorf("bundle-ids capabilities add: %v", err)
			}

			ifExistsMode, err := d.ParseIfExistsMode(*ifExists, shared.IfExistsSkip, shared.IfExistsUpdate)
			if err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities add: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.BundleIDCapabilityCreateAttributes{
				CapabilityType: capabilityValue,
				Settings:       settingsValue,
			}
			resp, err := client.CreateBundleIDCapability(requestCtx, bundleValue, attrs)
			if err != nil {
				existing, handled, resolveErr := shared.ResolveIfExistsConflict(ifExistsMode, err, capabilitiesAddExistsCodes, func() (*asc.BundleIDCapabilityResponse, bool, error) {
					return findExistingBundleIDCapability(requestCtx, client, bundleValue, capabilityValue)
				})
				if resolveErr != nil {
					return fmt.Errorf("bundle-ids capabilities add: failed to create: %w", resolveErr)
				}
				if !handled {
					return fmt.Errorf("bundle-ids capabilities add: failed to create: %w", err)
				}
				resp = existing
				outcome := "left unchanged"
				if ifExistsMode == shared.IfExistsUpdate && len(settingsValue) > 0 {
					updated, updateErr := client.UpdateBundleIDCapability(requestCtx, existing.Data.ID, asc.BundleIDCapabilityUpdateAttributes{
						Settings: settingsValue,
					})
					if updateErr != nil {
						return fmt.Errorf("bundle-ids capabilities add: update existing capability %s: %w", existing.Data.ID, updateErr)
					}
					resp = updated
					outcome = "updated it in place"
				}
				fmt.Fprintf(d.IO.ErrOut, "bundle-ids capabilities add: capability %s already enabled on bundle ID %s as %s; %s (--if-exists %s)\n",
					capabilityValue, bundleValue, existing.Data.ID, outcome, ifExistsMode)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBundleIDsCapabilitiesUpdateCommand returns the bundle IDs capabilities update subcommand.
func newBundleIDsCapabilitiesUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "bundleIdCapabilities", "Capability ID")
	capabilityType := fs.String("capability", "", "Capability type (e.g., ICLOUD, IN_APP_PURCHASE)")
	settings := fs.String("settings", "", "Capability settings as a structure-validated JSON array")
	output := d.BindOutputFlags(fs)

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
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--id")
			}

			capabilityValue := strings.ToUpper(strings.TrimSpace(*capabilityType))
			settingsValue, err := parseCapabilitySettings(*settings)
			if err != nil {
				return d.UsageErrorf("bundle-ids capabilities update: %v", err)
			}

			// Treat empty settings arrays as no-op updates.
			if capabilityValue == "" && len(settingsValue) == 0 {
				fmt.Fprintln(d.IO.ErrOut, "Error: at least one update field is required (--capability or --settings)")
				return shared.MissingRequiredUsageError("")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.BundleIDCapabilityUpdateAttributes{
				CapabilityType: capabilityValue,
				Settings:       settingsValue,
			}
			resp, err := client.UpdateBundleIDCapability(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBundleIDsCapabilitiesRemoveCommand returns the bundle IDs capabilities remove subcommand.
func newBundleIDsCapabilitiesRemoveCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "remove",
		Name:        "remove",
		ShortUsage:  "asc bundle-ids capabilities remove --id \"CAPABILITY_ID\" --confirm",
		ShortHelp:   "Remove a capability from a bundle ID.",
		LongHelp: `Remove a capability from a bundle ID.

Examples:
  asc bundle-ids capabilities remove --id "CAPABILITY_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Capability ID",
		IDType:             "bundleIdCapabilities",
		RequiredDiagnostic: true,
		ErrorPrefix:        "bundle-ids capabilities remove",
		DeleteErrorPrefix:  "bundle-ids capabilities remove: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteBundleIDCapability(ctx, id)
		},
		Result: func(id string) any {
			return &asc.BundleIDCapabilityDeleteResult{ID: id, Deleted: true}
		},
	})
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

// capabilitiesAddExistsCodes lists the Apple 409 codes accepted as "this
// capability is already enabled on the bundle ID" on
// POST /v1/bundleIdCapabilities. ENTITY_ERROR.ATTRIBUTE.TYPE (a capability type
// Apple does not accept for this bundle ID) is also a 409 and is not on the
// list, so it keeps failing; the read-back is what finally proves existence.
var capabilitiesAddExistsCodes = []string{
	"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE",
	"ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS",
}

// findExistingBundleIDCapability reads back the capability a 409 conflict
// referred to, keyed by capability type. It reports found=false when the bundle
// ID has no such capability so the caller can surface the original conflict.
func findExistingBundleIDCapability(ctx context.Context, client *asc.Client, bundleID, capabilityType string) (*asc.BundleIDCapabilityResponse, bool, error) {
	firstPage, err := client.GetBundleIDCapabilities(ctx, bundleID)
	if err != nil {
		return nil, false, err
	}
	allPages, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetBundleIDCapabilities(ctx, bundleID, asc.NextURL[asc.BundleIDCapabilitiesOption](nextURL))
	})
	if err != nil {
		return nil, false, err
	}
	capabilities, ok := allPages.(*asc.BundleIDCapabilitiesResponse)
	if !ok {
		return nil, false, fmt.Errorf("unexpected bundle ID capabilities response type: %T", allPages)
	}
	for _, candidate := range capabilities.Data {
		if strings.EqualFold(strings.TrimSpace(candidate.Attributes.CapabilityType), capabilityType) {
			// Apple exposes no GET /v1/bundleIdCapabilities/{id} (only POST,
			// PATCH and DELETE), so the collection item is the only
			// representation available and the single-resource envelope has to
			// be built from it. Nothing is invented: the resource object is
			// Apple's, verbatim.
			return &asc.BundleIDCapabilityResponse{Data: candidate}, true, nil
		}
	}
	return nil, false, nil
}
