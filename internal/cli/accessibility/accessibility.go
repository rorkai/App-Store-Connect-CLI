package accessibility

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

// AccessibilityCommand returns the accessibility declarations command.
func AccessibilityCommand() *ffcli.Command {
	fs := flag.NewFlagSet("accessibility", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "accessibility",
		ShortUsage: "asc accessibility <subcommand> [flags]",
		ShortHelp:  "Manage accessibility declarations.",
		LongHelp: `Manage accessibility declarations for an app.

Examples:
  asc accessibility list --app "APP_ID"
  asc accessibility get --id "DECLARATION_ID"
  asc accessibility create --app "APP_ID" --device-family IPHONE --supports-voiceover true
  asc accessibility update --id "DECLARATION_ID" --publish true
  asc accessibility delete --id "DECLARATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AccessibilityListCommand(),
			AccessibilityGetCommand(),
			AccessibilityCreateCommand(),
			AccessibilityUpdateCommand(),
			AccessibilityDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AccessibilityListCommand returns the accessibility list subcommand.
func AccessibilityListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	deviceFamily := fs.String("device-family", "", "Filter by device family(s), comma-separated: "+strings.Join(accessibilityDeviceFamilyList(), ", "))
	state := fs.String("state", "", "Filter by state(s), comma-separated: "+strings.Join(accessibilityStateList(), ", "))
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(accessibilityDeclarationFieldList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("accessibility list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("accessibility list: %w", err)
			}

			deviceFamilies, err := normalizeAccessibilityDeviceFamilies(shared.SplitCSVUpper(*deviceFamily))
			if err != nil {
				return fmt.Errorf("accessibility list: %w", err)
			}

			states, err := normalizeAccessibilityStates(shared.SplitCSVUpper(*state))
			if err != nil {
				return fmt.Errorf("accessibility list: %w", err)
			}

			fieldsValue, err := normalizeAccessibilityDeclarationFields(*fields)
			if err != nil {
				return fmt.Errorf("accessibility list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("accessibility list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AccessibilityDeclarationsOption{
				asc.WithAccessibilityDeclarationsDeviceFamilies(deviceFamilies),
				asc.WithAccessibilityDeclarationsStates(states),
				asc.WithAccessibilityDeclarationsFields(fieldsValue),
				asc.WithAccessibilityDeclarationsLimit(*limit),
				asc.WithAccessibilityDeclarationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAccessibilityDeclarationsLimit(200))
				firstPage, err := client.GetAccessibilityDeclarations(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("accessibility list: failed to fetch: %w", err)
				}

				pages, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAccessibilityDeclarations(ctx, resolvedAppID, asc.WithAccessibilityDeclarationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("accessibility list: %w", err)
				}

				return shared.PrintOutput(pages, *output, *pretty)
			}

			resp, err := client.GetAccessibilityDeclarations(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("accessibility list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AccessibilityGetCommand returns the accessibility get subcommand.
func AccessibilityGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Accessibility declaration ID (required)")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(accessibilityDeclarationFieldList(), ", "))
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc accessibility get --id DECLARATION_ID",
		ShortHelp:  "Get an accessibility declaration by ID.",
		LongHelp: `Get an accessibility declaration by ID.

Examples:
  asc accessibility get --id "DECLARATION_ID"
  asc accessibility get --id "DECLARATION_ID" --fields "deviceFamily,state"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			fieldsValue, err := normalizeAccessibilityDeclarationFields(*fields)
			if err != nil {
				return fmt.Errorf("accessibility get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("accessibility get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAccessibilityDeclaration(requestCtx, idValue, fieldsValue)
			if err != nil {
				return fmt.Errorf("accessibility get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AccessibilityCreateCommand returns the accessibility create subcommand.
func AccessibilityCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

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
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			deviceFamilyValue := strings.TrimSpace(*deviceFamily)
			if deviceFamilyValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --device-family is required")
				return flag.ErrHelp
			}

			normalizedDeviceFamily, err := normalizeAccessibilityDeviceFamily(deviceFamilyValue)
			if err != nil {
				return fmt.Errorf("accessibility create: %w", err)
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
				return fmt.Errorf("accessibility create: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("accessibility create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAccessibilityDeclaration(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("accessibility create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AccessibilityUpdateCommand returns the accessibility update subcommand.
func AccessibilityUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "Accessibility declaration ID (required)")
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
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
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
				return fmt.Errorf("accessibility update: %w", err)
			}

			if !asc.HasAccessibilityDeclarationUpdates(attrs) {
				return fmt.Errorf("accessibility update: at least one update flag is required")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("accessibility update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAccessibilityDeclaration(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("accessibility update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AccessibilityDeleteCommand returns the accessibility delete subcommand.
func AccessibilityDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Accessibility declaration ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc accessibility delete --id DECLARATION_ID --confirm",
		ShortHelp:  "Delete an accessibility declaration.",
		LongHelp: `Delete an accessibility declaration.

Examples:
  asc accessibility delete --id "DECLARATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
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

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("accessibility delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAccessibilityDeclaration(requestCtx, idValue); err != nil {
				return fmt.Errorf("accessibility delete: failed to delete: %w", err)
			}

			result := &asc.AccessibilityDeclarationDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
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
	for _, item := range accessibilityDeviceFamilyList() {
		if normalized == item {
			return normalized, nil
		}
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
