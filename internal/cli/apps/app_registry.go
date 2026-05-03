package apps

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const defaultAppRegistryPath = ".asc/app-registry.json"

type appRegistryFile struct {
	Apps []appRegistryEntry `json:"apps"`
}

type appRegistryEntry struct {
	Key           string   `json:"key"`
	Name          string   `json:"name"`
	ASCAppID      string   `json:"asc_app_id"`
	BundleID      string   `json:"bundle_id"`
	Platform      *string  `json:"platform"`
	PrimaryLocale string   `json:"primary_locale"`
	RepoPath      *string  `json:"repo_path"`
	GA4PropertyID *string  `json:"ga4_property_id"`
	Aliases       []string `json:"aliases"`
}

type appRegistrySyncResult struct {
	Path      string           `json:"path"`
	DryRun    bool             `json:"dryRun"`
	Total     int              `json:"total"`
	Created   int              `json:"created"`
	Updated   int              `json:"updated"`
	Unchanged int              `json:"unchanged"`
	Preserved int              `json:"preserved"`
	Pruned    int              `json:"pruned"`
	Registry  *appRegistryFile `json:"registry,omitempty"`
}

// AppsRegistryCommand returns the local app registry subtree.
func AppsRegistryCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps registry", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "registry",
		ShortUsage: "asc apps registry <subcommand> [flags]",
		ShortHelp:  "Manage a local app registry for agent workflows.",
		LongHelp: `Manage a local app registry for agent workflows.

The registry mirrors App Store Connect app identity fields and preserves
local-only automation fields such as repo paths, analytics IDs, aliases, and
platform hints.

Examples:
  asc apps registry sync
  asc apps registry sync --path ".asc/app-registry.json"
  asc apps registry sync --path "/Users/me/clawd/config/app_registry.json" --dry-run
  asc apps registry sync --prune-missing`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppsRegistrySyncCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppsRegistrySyncCommand returns the app registry sync subcommand.
func AppsRegistrySyncCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps registry sync", flag.ExitOnError)

	path := fs.String("path", defaultAppRegistryPath, "Registry JSON path")
	dryRun := fs.Bool("dry-run", false, "Preview the merged registry without writing it")
	pruneMissing := fs.Bool("prune-missing", false, "Remove local registry entries not returned by App Store Connect")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "sync",
		ShortUsage: "asc apps registry sync [--path PATH] [--dry-run] [--prune-missing] [flags]",
		ShortHelp:  "Sync a local app registry from App Store Connect.",
		LongHelp: `Sync a local app registry from App Store Connect.

The command fetches all apps available to the configured API key, updates ASC
identity fields, and preserves local-only fields by asc_app_id. By default,
entries not returned by App Store Connect are kept to avoid accidental data
loss when using a limited API key. Use --prune-missing to remove them.

Examples:
  asc apps registry sync
  asc apps registry sync --dry-run --output json
  asc apps registry sync --path "/Users/me/clawd/config/app_registry.json"
  asc apps registry sync --path ".asc/app-registry.json" --prune-missing`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: apps registry sync does not accept positional arguments")
				return flag.ErrHelp
			}
			return appsRegistrySync(ctx, appsRegistrySyncOptions{
				Path:         *path,
				DryRun:       *dryRun,
				PruneMissing: *pruneMissing,
				Output:       *output.Output,
				Pretty:       *output.Pretty,
			})
		},
	}
}

type appsRegistrySyncOptions struct {
	Path         string
	DryRun       bool
	PruneMissing bool
	Output       string
	Pretty       bool
}

func appsRegistrySync(ctx context.Context, opts appsRegistrySyncOptions) error {
	path := strings.TrimSpace(opts.Path)
	if path == "" {
		fmt.Fprintln(os.Stderr, "Error: --path is required")
		return flag.ErrHelp
	}

	existing, err := readAppRegistry(path)
	if err != nil {
		return fmt.Errorf("apps registry sync: %w", err)
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("apps registry sync: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	response, err := shared.PaginateWithSpinner(requestCtx,
		func(ctx context.Context) (asc.PaginatedResponse, error) {
			return client.GetApps(ctx, asc.WithAppsLimit(200), asc.WithAppsSort("name"))
		},
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetApps(ctx, asc.WithAppsNextURL(nextURL))
		},
	)
	if err != nil {
		return fmt.Errorf("apps registry sync: failed to fetch apps: %w", err)
	}

	appsResponse, ok := response.(*asc.AppsResponse)
	if !ok {
		return fmt.Errorf("apps registry sync: unexpected apps response type %T", response)
	}

	result, registry, err := mergeAppRegistry(existing, appsResponse.Data, opts.PruneMissing)
	if err != nil {
		return fmt.Errorf("apps registry sync: %w", err)
	}
	result.Path = path
	result.DryRun = opts.DryRun
	if opts.DryRun {
		result.Registry = &registry
	}

	if !opts.DryRun {
		if err := writeAppRegistry(path, registry); err != nil {
			return fmt.Errorf("apps registry sync: failed to write registry: %w", err)
		}
	}

	return printAppRegistrySyncResult(&result, opts.Output, opts.Pretty)
}

func readAppRegistry(path string) (appRegistryFile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return appRegistryFile{}, nil
	}
	if err != nil {
		return appRegistryFile{}, err
	}

	var registry appRegistryFile
	if err := json.Unmarshal(data, &registry); err != nil {
		return appRegistryFile{}, fmt.Errorf("invalid registry JSON %q: %w", path, err)
	}
	normalizeRegistryEntries(registry.Apps)
	return registry, nil
}

func writeAppRegistry(path string, registry appRegistryFile) error {
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	hadExisting := false
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to overwrite symlink %q", path)
		}
		if info.IsDir() {
			return fmt.Errorf("registry path %q is a directory", path)
		}
		hadExisting = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), ".app-registry-*.json")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Chmod(0o600); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		if !hadExisting {
			return err
		}

		backupFile, backupErr := os.CreateTemp(filepath.Dir(path), ".app-registry-backup-*.json")
		if backupErr != nil {
			return err
		}
		backupPath := backupFile.Name()
		if closeErr := backupFile.Close(); closeErr != nil {
			return closeErr
		}
		if removeErr := os.Remove(backupPath); removeErr != nil {
			return removeErr
		}

		if moveErr := os.Rename(path, backupPath); moveErr != nil {
			return moveErr
		}
		if moveErr := os.Rename(tempPath, path); moveErr != nil {
			_ = os.Rename(backupPath, path)
			return moveErr
		}
		_ = os.Remove(backupPath)
	}
	success = true
	return nil
}

func mergeAppRegistry(existing appRegistryFile, resources []asc.Resource[asc.AppAttributes], pruneMissing bool) (appRegistrySyncResult, appRegistryFile, error) {
	normalizeRegistryEntries(existing.Apps)
	if err := validateUniqueRegistryASCAppIDs(existing.Apps); err != nil {
		return appRegistrySyncResult{}, appRegistryFile{}, err
	}
	if err := validateUniqueASCResources(resources); err != nil {
		return appRegistrySyncResult{}, appRegistryFile{}, err
	}

	existingByID := make(map[string]appRegistryEntry, len(existing.Apps))
	for _, app := range existing.Apps {
		if strings.TrimSpace(app.ASCAppID) == "" {
			continue
		}
		existingByID[app.ASCAppID] = app
	}

	sort.Slice(resources, func(i, j int) bool {
		left := resources[i]
		right := resources[j]
		leftName := strings.ToLower(strings.TrimSpace(left.Attributes.Name))
		rightName := strings.ToLower(strings.TrimSpace(right.Attributes.Name))
		if leftName != rightName {
			return leftName < rightName
		}
		return left.ID < right.ID
	})

	usedKeys := make(map[string]struct{}, len(existing.Apps)+len(resources))
	for _, app := range existing.Apps {
		if key := strings.TrimSpace(app.Key); key != "" {
			usedKeys[key] = struct{}{}
		}
	}

	seenASCIDs := make(map[string]struct{}, len(resources))
	merged := make([]appRegistryEntry, 0, len(existing.Apps)+len(resources))
	result := appRegistrySyncResult{}

	for _, resource := range resources {
		appID := strings.TrimSpace(resource.ID)
		if appID == "" {
			continue
		}
		seenASCIDs[appID] = struct{}{}

		existingApp, found := existingByID[appID]
		before := existingApp
		if !found {
			existingApp = appRegistryEntry{
				Key:           uniqueAppRegistryKey(slugifyAppRegistryKey(resource.Attributes.Name, appID), usedKeys),
				ASCAppID:      appID,
				Platform:      nil,
				RepoPath:      nil,
				GA4PropertyID: nil,
				Aliases:       []string{},
			}
			result.Created++
		} else if strings.TrimSpace(existingApp.Key) == "" {
			existingApp.Key = uniqueAppRegistryKey(slugifyAppRegistryKey(resource.Attributes.Name, appID), usedKeys)
		}

		mergedApp := existingApp
		mergedApp.Name = strings.TrimSpace(resource.Attributes.Name)
		mergedApp.ASCAppID = appID
		mergedApp.BundleID = strings.TrimSpace(resource.Attributes.BundleID)
		mergedApp.PrimaryLocale = strings.TrimSpace(resource.Attributes.PrimaryLocale)
		normalizeRegistryEntry(&mergedApp)

		if found {
			if reflect.DeepEqual(before, mergedApp) {
				result.Unchanged++
			} else {
				result.Updated++
			}
		}
		merged = append(merged, mergedApp)
		usedKeys[mergedApp.Key] = struct{}{}
	}

	for _, app := range existing.Apps {
		if _, seen := seenASCIDs[app.ASCAppID]; seen {
			continue
		}
		if pruneMissing {
			result.Pruned++
			continue
		}
		merged = append(merged, app)
		result.Preserved++
	}

	sortAppRegistryEntries(merged)
	result.Total = len(merged)
	return result, appRegistryFile{Apps: merged}, nil
}

func validateUniqueRegistryASCAppIDs(apps []appRegistryEntry) error {
	seen := make(map[string]struct{}, len(apps))
	for _, app := range apps {
		appID := strings.TrimSpace(app.ASCAppID)
		if appID == "" {
			continue
		}
		if _, ok := seen[appID]; ok {
			return fmt.Errorf("registry contains duplicate asc_app_id %q", appID)
		}
		seen[appID] = struct{}{}
	}
	return nil
}

func validateUniqueASCResources(resources []asc.Resource[asc.AppAttributes]) error {
	seen := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		appID := strings.TrimSpace(resource.ID)
		if appID == "" {
			continue
		}
		if _, ok := seen[appID]; ok {
			return fmt.Errorf("App Store Connect returned duplicate app id %q", appID)
		}
		seen[appID] = struct{}{}
	}
	return nil
}

func normalizeRegistryEntries(entries []appRegistryEntry) {
	for i := range entries {
		normalizeRegistryEntry(&entries[i])
	}
}

func normalizeRegistryEntry(entry *appRegistryEntry) {
	entry.Key = strings.TrimSpace(entry.Key)
	entry.Name = strings.TrimSpace(entry.Name)
	entry.ASCAppID = strings.TrimSpace(entry.ASCAppID)
	entry.BundleID = strings.TrimSpace(entry.BundleID)
	entry.PrimaryLocale = strings.TrimSpace(entry.PrimaryLocale)
	entry.Platform = trimOptionalString(entry.Platform)
	entry.RepoPath = trimOptionalString(entry.RepoPath)
	entry.GA4PropertyID = trimOptionalString(entry.GA4PropertyID)
	if entry.Aliases == nil {
		entry.Aliases = []string{}
	}
	for j := range entry.Aliases {
		entry.Aliases[j] = strings.TrimSpace(entry.Aliases[j])
	}
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func sortAppRegistryEntries(entries []appRegistryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		leftName := strings.ToLower(strings.TrimSpace(left.Name))
		rightName := strings.ToLower(strings.TrimSpace(right.Name))
		if leftName != rightName {
			return leftName < rightName
		}
		if left.ASCAppID != right.ASCAppID {
			return left.ASCAppID < right.ASCAppID
		}
		return left.Key < right.Key
	})
}

func slugifyAppRegistryKey(name string, appID string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	key := strings.Trim(builder.String(), "-")
	if key == "" {
		key = "app-" + strings.TrimSpace(appID)
	}
	return key
}

func uniqueAppRegistryKey(base string, used map[string]struct{}) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "app"
	}
	if _, exists := used[base]; !exists {
		used[base] = struct{}{}
		return base
	}
	for i := 2; ; i++ {
		candidate := base + "-" + strconv.Itoa(i)
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
	}
}

func printAppRegistrySyncResult(result *appRegistrySyncResult, format string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		format,
		pretty,
		func() error { return renderAppRegistrySyncResult(result, false) },
		func() error { return renderAppRegistrySyncResult(result, true) },
	)
}

func renderAppRegistrySyncResult(result *appRegistrySyncResult, markdown bool) error {
	if result == nil {
		return fmt.Errorf("registry sync result is nil")
	}

	headers := []string{"Path", "Dry Run", "Total", "Created", "Updated", "Unchanged", "Preserved", "Pruned"}
	rows := [][]string{{
		result.Path,
		strconv.FormatBool(result.DryRun),
		strconv.Itoa(result.Total),
		strconv.Itoa(result.Created),
		strconv.Itoa(result.Updated),
		strconv.Itoa(result.Unchanged),
		strconv.Itoa(result.Preserved),
		strconv.Itoa(result.Pruned),
	}}

	if markdown {
		asc.RenderMarkdown(headers, rows)
		return nil
	}
	asc.RenderTable(headers, rows)
	return nil
}
