package accessibility

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

// NewAccessibilityCommand returns the accessibility declarations command built
// on d.
func NewAccessibilityCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("accessibility", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "accessibility",
		ShortUsage: "asc accessibility <subcommand> [flags]",
		ShortHelp:  "Manage accessibility declarations.",
		LongHelp: `Manage accessibility declarations for an app.

Examples:
  asc accessibility list --app "APP_ID"
  asc accessibility view --id "DECLARATION_ID"
  asc accessibility create --app "APP_ID" --device-family IPHONE --supports-voiceover true
  asc accessibility update --id "DECLARATION_ID" --publish true
  asc accessibility delete --id "DECLARATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAccessibilityListCommand(d),
			newAccessibilityGetCommand(d),
			newAccessibilityCreateCommand(d),
			newAccessibilityUpdateCommand(d),
			newAccessibilityDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAccessibilityListCommand returns the accessibility list subcommand.
func newAccessibilityListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	deviceFamily := fs.String("device-family", "", "Filter by device family(s), comma-separated: "+strings.Join(accessibilityDeviceFamilyList(), ", "))
	state := fs.String("state", "", "Filter by state(s), comma-separated: "+strings.Join(accessibilityStateList(), ", "))
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(accessibilityDeclarationFieldList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc accessibility list [flags]",
		ShortHelp:  "List accessibility declarations for an app.",
		LongHelp: `List accessibility declarations for an app.

Examples:
  asc accessibility list --app "APP_ID"
  asc accessibility list --app "APP_ID" --device-family IPHONE
  asc accessibility list --app "APP_ID" --state PUBLISHED --limit 50
  asc accessibility list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("accessibility list: %v", err)
			}
			if err := rejectAccessibilityNextFlagConflicts(d, fs, *next, "app", "device-family", "state", "fields", "limit"); err != nil {
				return err
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return d.UsageError("accessibility list: --limit must be between 1 and 200")
			}

			deviceFamilies, err := normalizeAccessibilityDeviceFamilies(shared.SplitCSVUpper(*deviceFamily))
			if err != nil {
				return d.UsageErrorf("accessibility list: %v", err)
			}

			states, err := normalizeAccessibilityStates(shared.SplitCSVUpper(*state))
			if err != nil {
				return d.UsageErrorf("accessibility list: %v", err)
			}

			fieldsValue, err := normalizeAccessibilityDeclarationFields(*fields)
			if err != nil {
				return d.UsageErrorf("accessibility list: %v", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("accessibility list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AccessibilityDeclarationsOption{
				asc.WithAccessibilityDeclarationsDeviceFamilies(deviceFamilies),
				asc.WithAccessibilityDeclarationsStates(states),
				asc.WithAccessibilityDeclarationsFields(fieldsValue),
				asc.Limit[asc.AccessibilityDeclarationsOption](*limit),
				asc.NextURL[asc.AccessibilityDeclarationsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AccessibilityDeclarationsOption](200))
				firstPage, err := client.GetAccessibilityDeclarations(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("accessibility list: failed to fetch: %w", err)
				}

				pages, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAccessibilityDeclarations(ctx, resolvedAppID, asc.NextURL[asc.AccessibilityDeclarationsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("accessibility list: %w", err)
				}

				return d.PrintOutput(pages, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAccessibilityDeclarations(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("accessibility list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func rejectAccessibilityNextFlagConflicts(d *shared.Deps, fs *flag.FlagSet, next string, names ...string) error {
	if strings.TrimSpace(next) == "" {
		return nil
	}
	provided := make(map[string]struct{}, len(names))
	fs.Visit(func(f *flag.Flag) {
		provided[f.Name] = struct{}{}
	})
	for _, name := range names {
		if _, ok := provided[name]; ok {
			return d.UsageErrorf("accessibility list: --next cannot be combined with --%s", name)
		}
	}
	return nil
}

// newAccessibilityGetCommand returns the accessibility view subcommand.
func newAccessibilityGetCommand(d *shared.Deps) *ffcli.Command {
	var fieldsValue []string

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc accessibility view --id DECLARATION_ID",
		ShortHelp:   "View an accessibility declaration by ID.",
		LongHelp: `View an accessibility declaration by ID.

Examples:
  asc accessibility view --id "DECLARATION_ID"
  asc accessibility view --id "DECLARATION_ID" --fields "deviceFamily,state"`,
		IDUsage:            "Accessibility declaration ID (required)",
		IDType:             "accessibilityDeclarations",
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(accessibilityDeclarationFieldList(), ", "))
			return func() (err error) {
				if fieldsValue, err = normalizeAccessibilityDeclarationFields(*fields); err != nil {
					return d.UsageErrorf("accessibility view: %v", err)
				}
				return nil
			}
		},
		ErrorPrefix:      "accessibility view",
		FetchErrorPrefix: "accessibility view: failed to fetch",
		Deps:             d,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAccessibilityDeclaration(ctx, id, fieldsValue)
		},
	})
}

// newAccessibilityCreateCommand returns the accessibility create subcommand.
func newAccessibilityCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	deviceFamily := fs.String("device-family", "", "Device family: "+strings.Join(accessibilityDeviceFamilyList(), ", "))
	supportsAudioDescriptions := fs.String("supports-audio-descriptions", "", "Supports audio descriptions (true/false)")
	supportsCaptions := fs.String("supports-captions", "", "Supports captions (true/false)")
	supportsDarkInterface := fs.String("supports-dark-interface", "", "Supports dark interface (true/false)")
	supportsDifferentiateWithoutColorAlone := fs.String("supports-differentiate-without-color-alone", "", "Supports differentiate without color alone (true/false)")
	supportsLargerText := fs.String("supports-larger-text", "", "Supports larger text (true/false)")
	supportsReducedMotion := fs.String("supports-reduced-motion", "", "Supports reduced motion (true/false)")
	supportsSufficientContrast := fs.String("supports-sufficient-contrast", "", "Supports sufficient contrast (true/false)")
	supportsVoiceControl := fs.String("supports-voice-control", "", "Supports voice control (true/false)")
	supportsVoiceover := fs.String("supports-voiceover", "", "Supports voiceover (true/false)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc accessibility create --app APP_ID --device-family DEVICE_FAMILY [flags]",
		ShortHelp:  "Create an accessibility declaration.",
		LongHelp: `Create an accessibility declaration.

Examples:
  asc accessibility create --app "APP_ID" --device-family IPHONE --supports-voiceover true
  asc accessibility create --app "APP_ID" --device-family IPAD --supports-captions true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			deviceFamilyValue := strings.TrimSpace(*deviceFamily)
			if deviceFamilyValue == "" {
				return d.RequiredFlag("--device-family")
			}

			normalizedDeviceFamily, err := normalizeAccessibilityDeviceFamily(deviceFamilyValue)
			if err != nil {
				return d.UsageErrorf("accessibility create: %v", err)
			}

			attrs, err := buildAccessibilityDeclarationCreateAttributes(normalizedDeviceFamily, map[string]string{
				"supports-audio-descriptions":                *supportsAudioDescriptions,
				"supports-captions":                          *supportsCaptions,
				"supports-dark-interface":                    *supportsDarkInterface,
				"supports-differentiate-without-color-alone": *supportsDifferentiateWithoutColorAlone,
				"supports-larger-text":                       *supportsLargerText,
				"supports-reduced-motion":                    *supportsReducedMotion,
				"supports-sufficient-contrast":               *supportsSufficientContrast,
				"supports-voice-control":                     *supportsVoiceControl,
				"supports-voiceover":                         *supportsVoiceover,
			})
			if err != nil {
				return d.UsageErrorf("accessibility create: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("accessibility create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateAccessibilityDeclaration(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("accessibility create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAccessibilityUpdateCommand returns the accessibility update subcommand.
func newAccessibilityUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "accessibilityDeclarations", "Accessibility declaration ID (required)")
	publish := fs.String("publish", "", "Publish declaration (true/false)")
	supportsAudioDescriptions := fs.String("supports-audio-descriptions", "", "Supports audio descriptions (true/false)")
	supportsCaptions := fs.String("supports-captions", "", "Supports captions (true/false)")
	supportsDarkInterface := fs.String("supports-dark-interface", "", "Supports dark interface (true/false)")
	supportsDifferentiateWithoutColorAlone := fs.String("supports-differentiate-without-color-alone", "", "Supports differentiate without color alone (true/false)")
	supportsLargerText := fs.String("supports-larger-text", "", "Supports larger text (true/false)")
	supportsReducedMotion := fs.String("supports-reduced-motion", "", "Supports reduced motion (true/false)")
	supportsSufficientContrast := fs.String("supports-sufficient-contrast", "", "Supports sufficient contrast (true/false)")
	supportsVoiceControl := fs.String("supports-voice-control", "", "Supports voice control (true/false)")
	supportsVoiceover := fs.String("supports-voiceover", "", "Supports voiceover (true/false)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc accessibility update --id DECLARATION_ID [flags]",
		ShortHelp:  "Update an accessibility declaration.",
		LongHelp: `Update an accessibility declaration.

Examples:
  asc accessibility update --id "DECLARATION_ID" --supports-voiceover true
  asc accessibility update --id "DECLARATION_ID" --publish true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--id")
			}

			attrs, err := buildAccessibilityDeclarationUpdateAttributes(map[string]string{
				"publish":                                    *publish,
				"supports-audio-descriptions":                *supportsAudioDescriptions,
				"supports-captions":                          *supportsCaptions,
				"supports-dark-interface":                    *supportsDarkInterface,
				"supports-differentiate-without-color-alone": *supportsDifferentiateWithoutColorAlone,
				"supports-larger-text":                       *supportsLargerText,
				"supports-reduced-motion":                    *supportsReducedMotion,
				"supports-sufficient-contrast":               *supportsSufficientContrast,
				"supports-voice-control":                     *supportsVoiceControl,
				"supports-voiceover":                         *supportsVoiceover,
			})
			if err != nil {
				return d.UsageErrorf("accessibility update: %v", err)
			}

			if !asc.HasAccessibilityDeclarationUpdates(attrs) {
				return d.UsageError("accessibility update: at least one update flag is required")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("accessibility update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateAccessibilityDeclaration(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("accessibility update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAccessibilityDeleteCommand returns the accessibility delete subcommand.
func newAccessibilityDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc accessibility delete --id DECLARATION_ID --confirm",
		ShortHelp:   "Delete an accessibility declaration.",
		LongHelp: `Delete an accessibility declaration.

Examples:
  asc accessibility delete --id "DECLARATION_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Accessibility declaration ID (required)",
		IDType:             "accessibilityDeclarations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "accessibility delete",
		DeleteErrorPrefix:  "accessibility delete: failed to delete",
		Deps:               d,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAccessibilityDeclaration(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AccessibilityDeclarationDeleteResult{ID: id, Deleted: true}
		},
	})
}

func buildAccessibilityDeclarationCreateAttributes(deviceFamily string, values map[string]string) (asc.AccessibilityDeclarationCreateAttributes, error) {
	attrs := asc.AccessibilityDeclarationCreateAttributes{
		DeviceFamily: asc.DeviceFamily(deviceFamily),
	}

	supportsAudioDescriptions, err := shared.ParseOptionalBoolFlag("--supports-audio-descriptions", values["supports-audio-descriptions"])
	if err != nil {
		return attrs, err
	}
	supportsCaptions, err := shared.ParseOptionalBoolFlag("--supports-captions", values["supports-captions"])
	if err != nil {
		return attrs, err
	}
	supportsDarkInterface, err := shared.ParseOptionalBoolFlag("--supports-dark-interface", values["supports-dark-interface"])
	if err != nil {
		return attrs, err
	}
	supportsDifferentiateWithoutColorAlone, err := shared.ParseOptionalBoolFlag("--supports-differentiate-without-color-alone", values["supports-differentiate-without-color-alone"])
	if err != nil {
		return attrs, err
	}
	supportsLargerText, err := shared.ParseOptionalBoolFlag("--supports-larger-text", values["supports-larger-text"])
	if err != nil {
		return attrs, err
	}
	supportsReducedMotion, err := shared.ParseOptionalBoolFlag("--supports-reduced-motion", values["supports-reduced-motion"])
	if err != nil {
		return attrs, err
	}
	supportsSufficientContrast, err := shared.ParseOptionalBoolFlag("--supports-sufficient-contrast", values["supports-sufficient-contrast"])
	if err != nil {
		return attrs, err
	}
	supportsVoiceControl, err := shared.ParseOptionalBoolFlag("--supports-voice-control", values["supports-voice-control"])
	if err != nil {
		return attrs, err
	}
	supportsVoiceover, err := shared.ParseOptionalBoolFlag("--supports-voiceover", values["supports-voiceover"])
	if err != nil {
		return attrs, err
	}

	attrs.SupportsAudioDescriptions = supportsAudioDescriptions
	attrs.SupportsCaptions = supportsCaptions
	attrs.SupportsDarkInterface = supportsDarkInterface
	attrs.SupportsDifferentiateWithoutColorAlone = supportsDifferentiateWithoutColorAlone
	attrs.SupportsLargerText = supportsLargerText
	attrs.SupportsReducedMotion = supportsReducedMotion
	attrs.SupportsSufficientContrast = supportsSufficientContrast
	attrs.SupportsVoiceControl = supportsVoiceControl
	attrs.SupportsVoiceover = supportsVoiceover

	return attrs, nil
}

func buildAccessibilityDeclarationUpdateAttributes(values map[string]string) (asc.AccessibilityDeclarationUpdateAttributes, error) {
	var attrs asc.AccessibilityDeclarationUpdateAttributes

	publish, err := shared.ParseOptionalBoolFlag("--publish", values["publish"])
	if err != nil {
		return attrs, err
	}
	supportsAudioDescriptions, err := shared.ParseOptionalBoolFlag("--supports-audio-descriptions", values["supports-audio-descriptions"])
	if err != nil {
		return attrs, err
	}
	supportsCaptions, err := shared.ParseOptionalBoolFlag("--supports-captions", values["supports-captions"])
	if err != nil {
		return attrs, err
	}
	supportsDarkInterface, err := shared.ParseOptionalBoolFlag("--supports-dark-interface", values["supports-dark-interface"])
	if err != nil {
		return attrs, err
	}
	supportsDifferentiateWithoutColorAlone, err := shared.ParseOptionalBoolFlag("--supports-differentiate-without-color-alone", values["supports-differentiate-without-color-alone"])
	if err != nil {
		return attrs, err
	}
	supportsLargerText, err := shared.ParseOptionalBoolFlag("--supports-larger-text", values["supports-larger-text"])
	if err != nil {
		return attrs, err
	}
	supportsReducedMotion, err := shared.ParseOptionalBoolFlag("--supports-reduced-motion", values["supports-reduced-motion"])
	if err != nil {
		return attrs, err
	}
	supportsSufficientContrast, err := shared.ParseOptionalBoolFlag("--supports-sufficient-contrast", values["supports-sufficient-contrast"])
	if err != nil {
		return attrs, err
	}
	supportsVoiceControl, err := shared.ParseOptionalBoolFlag("--supports-voice-control", values["supports-voice-control"])
	if err != nil {
		return attrs, err
	}
	supportsVoiceover, err := shared.ParseOptionalBoolFlag("--supports-voiceover", values["supports-voiceover"])
	if err != nil {
		return attrs, err
	}

	attrs.Publish = publish
	attrs.SupportsAudioDescriptions = supportsAudioDescriptions
	attrs.SupportsCaptions = supportsCaptions
	attrs.SupportsDarkInterface = supportsDarkInterface
	attrs.SupportsDifferentiateWithoutColorAlone = supportsDifferentiateWithoutColorAlone
	attrs.SupportsLargerText = supportsLargerText
	attrs.SupportsReducedMotion = supportsReducedMotion
	attrs.SupportsSufficientContrast = supportsSufficientContrast
	attrs.SupportsVoiceControl = supportsVoiceControl
	attrs.SupportsVoiceover = supportsVoiceover

	return attrs, nil
}

func normalizeAccessibilityDeviceFamily(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if slices.Contains(accessibilityDeviceFamilyList(), normalized) {
		return normalized, nil
	}
	return "", fmt.Errorf("--device-family must be one of: %s", strings.Join(accessibilityDeviceFamilyList(), ", "))
}

func normalizeAccessibilityDeviceFamilies(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	for _, value := range values {
		if _, ok := accessibilityDeviceFamilies[value]; !ok {
			return nil, fmt.Errorf("--device-family must be one of: %s", strings.Join(accessibilityDeviceFamilyList(), ", "))
		}
	}
	return values, nil
}

func normalizeAccessibilityStates(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	for _, value := range values {
		if _, ok := accessibilityStates[value]; !ok {
			return nil, fmt.Errorf("--state must be one of: %s", strings.Join(accessibilityStateList(), ", "))
		}
	}
	return values, nil
}

func normalizeAccessibilityDeclarationFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, field := range accessibilityDeclarationFieldList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(accessibilityDeclarationFieldList(), ", "))
		}
	}

	return fields, nil
}

var accessibilityDeviceFamilies = map[string]struct{}{
	"IPHONE":      {},
	"IPAD":        {},
	"APPLE_TV":    {},
	"APPLE_WATCH": {},
	"MAC":         {},
	"VISION":      {},
}

var accessibilityStates = map[string]struct{}{
	"DRAFT":     {},
	"PUBLISHED": {},
	"REPLACED":  {},
}

func accessibilityDeviceFamilyList() []string {
	return []string{"IPHONE", "IPAD", "APPLE_TV", "APPLE_WATCH", "MAC", "VISION"}
}

func accessibilityStateList() []string {
	return []string{"DRAFT", "PUBLISHED", "REPLACED"}
}

func accessibilityDeclarationFieldList() []string {
	return []string{
		"deviceFamily",
		"state",
		"supportsAudioDescriptions",
		"supportsCaptions",
		"supportsDarkInterface",
		"supportsDifferentiateWithoutColorAlone",
		"supportsLargerText",
		"supportsReducedMotion",
		"supportsSufficientContrast",
		"supportsVoiceControl",
		"supportsVoiceover",
	}
}
