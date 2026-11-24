package bundleids

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/suggest"
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
		UsageFunc: d.DefaultUsageFunc,
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
	shared.BindFlagAlias(fs, "bundle-id", "bundle")
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "bundle-ids capabilities list", 200); err != nil {
				return err
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
		UsageFunc: d.DefaultUsageFunc,
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
					return withCapabilitiesAddConflictHint(fmt.Errorf("bundle-ids capabilities add: failed to create: %w", resolveErr), bundleValue, capabilityValue)
				}
				if !handled {
					return withCapabilitiesAddConflictHint(fmt.Errorf("bundle-ids capabilities add: failed to create: %w", err), bundleValue, capabilityValue)
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
		UsageFunc: d.DefaultUsageFunc,
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
				return d.UpdateFlagRequired(fs, capabilityType, settings)
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
		Deps:       d,
		Name:       "remove",
		ShortUsage: "asc bundle-ids capabilities remove --id \"CAPABILITY_ID\" --confirm",
		ShortHelp:  "Remove a capability from a bundle ID.",
		LongHelp: `Remove a capability from a bundle ID.

Examples:
  asc bundle-ids capabilities remove --id "CAPABILITY_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Capability ID",
		IDType:             "bundleIdCapabilities",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteBundleIDCapability(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
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

// signInWithAppleSettings is the --settings value that enables Sign in with
// Apple as a primary App ID; Apple rejects APPLE_ID_AUTH without a
// configuration ("Please select at least one configuration for Sign In with
// Apple.").
const signInWithAppleSettings = `'[{"key":"APPLE_ID_AUTH_APP_CONSENT","options":[{"key":"PRIMARY_APP_CONSENT","enabled":true}]}]'`

// capabilityTypeAPINames maps entitlement- and portal-style names agents use
// for a capability to the type App Store Connect's API accepts.
var capabilityTypeAPINames = map[string]string{
	"APPLICATION_GROUPS": "APP_GROUPS",
	"SIGN_IN_WITH_APPLE": "APPLE_ID_AUTH",
}

// apiCapabilityTypes is the CapabilityType enum in Apple's OpenAPI document
// (docs/openapi/latest.json): the capabilityType values
// POST /v1/bundleIdCapabilities accepts.
var apiCapabilityTypes = []string{
	"ICLOUD", "IN_APP_PURCHASE", "GAME_CENTER", "PUSH_NOTIFICATIONS", "WALLET",
	"INTER_APP_AUDIO", "MAPS", "ASSOCIATED_DOMAINS", "PERSONAL_VPN", "APP_GROUPS",
	"HEALTHKIT", "HOMEKIT", "WIRELESS_ACCESSORY_CONFIGURATION", "APPLE_PAY",
	"DATA_PROTECTION", "SIRIKIT", "NETWORK_EXTENSIONS", "MULTIPATH", "HOT_SPOT",
	"NFC_TAG_READING", "CLASSKIT", "AUTOFILL_CREDENTIAL_PROVIDER",
	"ACCESS_WIFI_INFORMATION", "NETWORK_CUSTOM_PROTOCOL", "COREMEDIA_HLS_LOW_LATENCY",
	"SYSTEM_EXTENSION_INSTALL", "USER_MANAGEMENT", "APPLE_ID_AUTH",
}

// withCapabilitiesAddConflictHint names the next step for the 409s Apple
// returns on POST /v1/bundleIdCapabilities that are not existence conflicts:
// a capability type outside the API's enum, Sign in with Apple without a
// configuration, and a --bundle that is a bundle identifier rather than the
// bundle ID's resource ID. Every entry of Apple's errors[] is consulted, so a
// cause reported after the first still gets its hint. Every other error is
// returned unchanged.
func withCapabilitiesAddConflictHint(err error, bundle, capability string) error {
	apiErr, ok := errors.AsType[*asc.APIError](err)
	if !ok || apiErr.StatusCode != http.StatusConflict {
		return err
	}
	entries := apiErr.Entries
	if len(entries) == 0 {
		entries = []asc.APIErrorEntry{{Code: apiErr.Code, Detail: apiErr.Detail}}
	}
	for _, entry := range entries {
		if hint := capabilitiesAddConflictEntryHint(entry, bundle, capability); hint != "" {
			return &shared.HintedError{Err: err, Hint: hint}
		}
	}
	return err
}

func capabilitiesAddConflictEntryHint(entry asc.APIErrorEntry, bundle, capability string) string {
	switch {
	case entry.Code == "ENTITY_ERROR.ATTRIBUTE.TYPE" && strings.Contains(entry.Detail, "for the attribute 'capabilityType'"):
		return capabilityTypeRejectedHint(bundle, capability)
	case strings.Contains(entry.Detail, "at least one configuration for Sign In with Apple"):
		return fmt.Sprintf("Sign in with Apple needs a configuration. To enable it as a primary App ID: asc bundle-ids capabilities add --bundle %q --capability APPLE_ID_AUTH --settings %s", bundle, signInWithAppleSettings)
	case entry.Code == "ENTITY_ERROR.RELATIONSHIP.INVALID" && strings.Contains(bundle, "."):
		// Bundle ID resource IDs are alphanumeric; a dot means the caller
		// passed the reverse-DNS identifier.
		return fmt.Sprintf("--bundle takes the bundle ID's resource ID (for example ABCDE12345), not its identifier %q. Look the ID up with asc bundle-ids list --identifier %q, then rerun with --bundle set to it.", bundle, bundle)
	}
	return ""
}

func capabilityTypeRejectedHint(bundle, capability string) string {
	if apiName, ok := capabilityTypeAPINames[capability]; ok {
		hint := fmt.Sprintf("App Store Connect calls this capability %s: asc bundle-ids capabilities add --bundle %q --capability %s", apiName, bundle, apiName)
		if apiName == "APPLE_ID_AUTH" {
			hint += " --settings " + signInWithAppleSettings
		}
		return hint
	}
	if capability == "PRIVATE_CLOUD_COMPUTE" {
		return fmt.Sprintf("PRIVATE_CLOUD_COMPUTE is a Developer Portal-only capability. Enable it through a web session with asc web bundle-ids capabilities enable --bundle-id %q --capability PRIVATE_CLOUD_COMPUTE --confirm", bundle)
	}
	// A near miss of a type the API accepts is a typo, not a portal-only
	// capability: name the type it most likely meant.
	if closest := closestAPICapabilityTypes(capability); len(closest) > 0 {
		command := fmt.Sprintf("asc bundle-ids capabilities add --bundle %q --capability %s", bundle, closest[0])
		if len(closest) == 1 {
			return fmt.Sprintf("%s is not a capability type the App Store Connect API recognizes; the closest one it accepts is %s: %s", capability, closest[0], command)
		}
		return fmt.Sprintf("%s is not a capability type the App Store Connect API recognizes; the closest ones it accepts are %s. Rerun with the one you meant, for example: %s", capability, strings.Join(closest, ", "), command)
	}
	return fmt.Sprintf("The App Store Connect API does not accept %s (Apple lists the capability types it accepts above), and retrying with other flags or --if-exists does not change that. If %s is a capability the Apple Developer portal offers, enable it on this App ID there (Certificates, Identifiers & Profiles > Identifiers), then regenerate the provisioning profiles that use the App ID.", capability, capability)
}

// closestAPICapabilityTypes returns up to three API capability types the
// rejected capability is a likely misspelling of. A name that holds a whole
// API type and more words, such as HEALTHKIT_RECALIBRATE_ESTIMATES, is a
// different capability rather than a misspelling of that type. The suggester
// ranks hyphen-separated names, so underscores are swapped for hyphens going
// in and back coming out.
func closestAPICapabilityTypes(capability string) []string {
	toSuggest := func(value string) string { return strings.ReplaceAll(value, "_", "-") }
	candidates := make([]string, len(apiCapabilityTypes))
	for i, capabilityType := range apiCapabilityTypes {
		candidates[i] = toSuggest(capabilityType)
	}
	matches := suggest.Commands(toSuggest(capability), candidates)
	for i, match := range matches {
		matches[i] = strings.ToUpper(strings.ReplaceAll(match, "-", "_"))
	}
	words := "_" + strings.ToUpper(strings.TrimSpace(capability)) + "_"
	return slices.DeleteFunc(matches, func(apiType string) bool {
		return strings.Contains(words, "_"+apiType+"_")
	})
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
