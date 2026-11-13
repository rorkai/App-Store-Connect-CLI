package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const versionLocalizationImportMaxFileSize = 1 << 20

// VersionLocalization holds a localization's values keyed by import field name.
type VersionLocalization struct {
	ID     string
	Locale string
	Values map[string]string
}

// VersionLocalizationImportOps performs the API calls for one localization
// resource. Create receives every value from the file; Update receives only
// the values that differ from App Store Connect.
type VersionLocalizationImportOps struct {
	List   func(ctx context.Context, versionID string) ([]VersionLocalization, error)
	Create func(ctx context.Context, versionID, locale string, values map[string]string) (string, error)
	Update func(ctx context.Context, localizationID string, values map[string]string) error
}

// VersionLocalizationImportConfig configures a version-scoped localizations
// import command.
type VersionLocalizationImportConfig struct {
	// CommandPath is the command path after "asc", such as
	// "iap versions localizations import".
	CommandPath         string
	VersionResourceType string
	VersionFlagUsage    string
	ResourceType        string
	// OptionalField is the import field besides "name", such as "description".
	OptionalField string
	ExampleValues string
	NewClient     func() (*asc.Client, error)
	Ops           func(*asc.Client) VersionLocalizationImportOps
}

// NewVersionLocalizationImportCommand builds a command that creates missing
// locales and updates changed ones for a version from one JSON file.
func NewVersionLocalizationImportCommand(config VersionLocalizationImportConfig) *ffcli.Command {
	fs := flag.NewFlagSet(config.CommandPath, flag.ExitOnError)
	versionID := BindResourceIDFlag(fs, "version-id", config.VersionResourceType, config.VersionFlagUsage)
	filePath := fs.String("file", "", "Path to a JSON file mapping locales to fields (required)")
	dryRun := fs.Bool("dry-run", false, "Print the create/update/skip plan without writing")
	confirm := fs.Bool("confirm", false, "Confirm writing localizations (required unless --dry-run)")
	output := BindOutputFlags(fs)

	fields := []string{"name", config.OptionalField}
	shortUsage := fmt.Sprintf(`asc %s --version-id "VERSION_ID" --file "./localizations.json" --confirm`, config.CommandPath)

	return &ffcli.Command{
		Name:       "import",
		ShortUsage: shortUsage,
		ShortHelp:  "Create or update localizations from a JSON file.",
		LongHelp: fmt.Sprintf(`Create or update localizations for a version from one JSON file.

The file maps each locale to the fields to set:
  %s

Fields: %s. Values must be non-empty strings. Omit a field to leave it
unchanged; import never clears a field or deletes a locale. A locale missing
from the version is created and requires "name". An existing locale is updated
only with the fields that differ, and skipped when nothing differs, so
rerunning the same file is safe.

The command reads the version's localizations once and validates the whole
plan before writing. --dry-run prints the plan without writing. With --confirm,
each locale is written independently: a failed locale is reported and the
remaining locales still run. The receipt lists every locale's action and
status, and the command exits non-zero when any locale failed.

Examples:
  asc %s --version-id "VERSION_ID" --file "./localizations.json" --dry-run
  asc %s --version-id "VERSION_ID" --file "./localizations.json" --confirm`,
			config.ExampleValues, strings.Join(fields, ", "), config.CommandPath, config.CommandPath),
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return MissingRequiredUsageError("--version-id")
			}
			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return MissingRequiredUsageError("--file")
			}
			if _, err := ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return UsageError(err.Error())
			}
			if err := RequireConfirmUnlessDryRun(*dryRun, *confirm); err != nil {
				return err
			}

			// Input errors are reported without the full help page, which would
			// bury the one line that names the bad locale or field.
			reportInputError := func(err error) error {
				message := fmt.Sprintf("%s: %v", config.CommandPath, err)
				fmt.Fprintln(os.Stderr, "Error:", message)
				return NewReportedUsageError(UsageErrorInvalidValue, message)
			}
			desired, err := readVersionLocalizationImportFile(pathValue, fields)
			if err != nil {
				return reportInputError(err)
			}

			client, err := config.NewClient()
			if err != nil {
				return fmt.Errorf("%s: %w", config.CommandPath, err)
			}
			ops := config.Ops(client)
			listCtx, cancel := ContextWithTimeout(ctx)
			existing, err := ops.List(listCtx, id)
			cancel()
			if err != nil {
				return fmt.Errorf("%s: list existing localizations: %w", config.CommandPath, err)
			}
			plan, err := planVersionLocalizationImport(desired, existing, fields)
			if err != nil {
				return reportInputError(err)
			}

			result := &asc.LocalizationImportResult{
				Type:      config.ResourceType,
				VersionID: id,
				File:      filepath.Clean(pathValue),
				DryRun:    *dryRun,
				Total:     len(plan),
				Results:   make([]asc.LocalizationImportLocaleResult, 0, len(plan)),
			}
			refused := executeVersionLocalizationImport(ctx, ops, id, plan, result)
			if err := PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if result.Failed > 0 {
				err := fmt.Errorf("%s: %d of %d locales failed", config.CommandPath, result.Failed, result.Total)
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return NewStderrReportedError(NewErrorWithCause(err, refused))
			}
			return nil
		},
	}
}

type versionLocalizationImportStep struct {
	asc.LocalizationImportLocaleResult
	values map[string]string
}

// readVersionLocalizationImportFile reads the file through a root anchored at
// its own directory and returns the locales in file order.
func readVersionLocalizationImportFile(path string, fields []string) ([]VersionLocalization, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve --file %q: %w", path, err)
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	data, err := root.ReadFileLimited(filepath.Base(absolute), versionLocalizationImportMaxFileSize)
	if err != nil {
		return nil, fmt.Errorf("read --file %q: %w", path, err)
	}
	return parseVersionLocalizationImport(data, fields)
}

func parseVersionLocalizationImport(data []byte, fields []string) ([]VersionLocalization, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, errors.New("invalid import file: expected a JSON object mapping locales to fields")
	}

	entries := make([]VersionLocalization, 0)
	seen := make(map[string]string)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("invalid import file: %w", err)
		}
		key, _ := token.(string)
		var raw map[string]json.RawMessage
		if err := decoder.Decode(&raw); err != nil || raw == nil {
			return nil, fmt.Errorf("locale %q: expected an object of fields", key)
		}
		locale, err := CanonicalizeAppStoreLocalizationLocale(key)
		if err != nil {
			return nil, err
		}
		if previous, ok := seen[strings.ToLower(locale)]; ok {
			return nil, fmt.Errorf("locale %q duplicates %q", key, previous)
		}
		seen[strings.ToLower(locale)] = key

		values := make(map[string]string, len(raw))
		for _, name := range slices.Sorted(maps.Keys(raw)) {
			if !slices.Contains(fields, name) {
				return nil, fmt.Errorf("locale %q: unknown field %q (allowed: %s)", key, name, strings.Join(fields, ", "))
			}
			var value string
			if err := json.Unmarshal(raw[name], &value); err != nil {
				return nil, fmt.Errorf("locale %q: field %q must be a string", key, name)
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return nil, fmt.Errorf("locale %q: field %q must not be empty; omit it to leave it unchanged", key, name)
			}
			values[name] = value
		}
		if len(values) == 0 {
			return nil, fmt.Errorf("locale %q: set at least one of: %s", key, strings.Join(fields, ", "))
		}
		entries = append(entries, VersionLocalization{Locale: locale, Values: values})
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("invalid import file: %w", err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("invalid import file: expected a single JSON object")
	}
	if len(entries) == 0 {
		return nil, errors.New("the import file must contain at least one locale")
	}
	return entries, nil
}

// planVersionLocalizationImport decides create, update, or skip for every
// locale and rejects the run before any write when a step cannot succeed.
func planVersionLocalizationImport(desired, existing []VersionLocalization, fields []string) ([]versionLocalizationImportStep, error) {
	byLocale := make(map[string]VersionLocalization, len(existing))
	for _, item := range existing {
		byLocale[strings.ToLower(NormalizeLocaleCode(item.Locale))] = item
	}

	plan := make([]versionLocalizationImportStep, 0, len(desired))
	for _, entry := range desired {
		current, ok := byLocale[strings.ToLower(entry.Locale)]
		if !ok {
			if entry.Values["name"] == "" {
				return nil, fmt.Errorf("locale %q does not exist on the version yet; creating it requires \"name\"", entry.Locale)
			}
			plan = append(plan, versionLocalizationImportStep{asc.LocalizationImportLocaleResult{Locale: entry.Locale, Action: "create", Fields: orderedFieldNames(fields, entry.Values)}, entry.Values})
			continue
		}
		changed := make(map[string]string)
		for _, field := range fields {
			if value, set := entry.Values[field]; set && value != current.Values[field] {
				changed[field] = value
			}
		}
		action := "update"
		if len(changed) == 0 {
			action = "skip"
		}
		plan = append(plan, versionLocalizationImportStep{asc.LocalizationImportLocaleResult{Locale: entry.Locale, Action: action, Fields: orderedFieldNames(fields, changed), LocalizationID: current.ID}, changed})
	}
	return plan, nil
}

// executeVersionLocalizationImport writes each planned locale independently so
// one rejected locale does not block the others; the receipt reports each one.
func executeVersionLocalizationImport(ctx context.Context, ops VersionLocalizationImportOps, versionID string, plan []versionLocalizationImportStep, result *asc.LocalizationImportResult) error {
	var refused error
	for _, step := range plan {
		entry := step.LocalizationImportLocaleResult
		switch {
		case entry.Action == "skip":
			entry.Status = "skipped"
			result.Skipped++
		case result.DryRun:
			entry.Status = "planned"
			result.Planned++
		default:
			requestCtx, cancel := ContextWithTimeout(ctx)
			var err error
			if entry.Action == "create" {
				entry.LocalizationID, err = ops.Create(requestCtx, versionID, entry.Locale, step.values)
			} else {
				err = ops.Update(requestCtx, entry.LocalizationID, step.values)
			}
			cancel()
			if err != nil {
				entry.Status = "failed"
				entry.Error = err.Error()
				refused = KeepReadOnlyRefusal(refused, err)
				result.Failed++
			} else {
				entry.Status = "succeeded"
				result.Succeeded++
			}
		}
		result.Results = append(result.Results, entry)
	}
	return refused
}

func orderedFieldNames(fields []string, values map[string]string) []string {
	names := make([]string, 0, len(values))
	for _, field := range fields {
		if _, ok := values[field]; ok {
			names = append(names, field)
		}
	}
	return names
}

// VersionLocalizationValue returns a pointer to the named value for an
// optional nullable attribute, or nil when the field is not being written.
func VersionLocalizationValue(values map[string]string, field string) *asc.NullableString {
	value, ok := values[field]
	if !ok {
		return nil
	}
	return &asc.NullableString{Value: &value}
}
