package profiles

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// profileUUIDValidationRegex ensures the UUID from a provisioning profile is safe to use
// as a filename component (prevents absolute paths / path traversal).
var profileUUIDValidationRegex = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func isValidProfileUUID(uuid string) bool {
	return profileUUIDValidationRegex.MatchString(uuid)
}

type localProfile struct {
	UUID      string    `json:"uuid"`
	Name      string    `json:"name,omitempty"`
	TeamID    string    `json:"teamId,omitempty"`
	BundleID  string    `json:"bundleId,omitempty"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	Path      string    `json:"path"`
	Expired   bool      `json:"expired"`
}

type localInstallResult struct {
	Source        string       `json:"source"`
	InstalledPath string       `json:"installedPath"`
	Action        string       `json:"action"`
	Profile       localProfile `json:"profile"`
}

type localCleanItem struct {
	UUID   string `json:"uuid,omitempty"`
	Name   string `json:"name,omitempty"`
	Path   string `json:"path"`
	Reason string `json:"reason,omitempty"`
}

type localSkippedItem struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type localListResult struct {
	InstallDir string `json:"installDir"`

	Total   int `json:"total"`
	Listed  int `json:"listed"`
	Skipped int `json:"skipped"`

	Items        []localProfile     `json:"items,omitempty"`
	SkippedItems []localSkippedItem `json:"skippedItems,omitempty"`
}

type localCleanResult struct {
	InstallDir string `json:"installDir"`
	Expired    bool   `json:"expired"`
	DryRun     bool   `json:"dryRun"`

	Planned int `json:"planned"`
	Deleted int `json:"deleted"`
	Skipped int `json:"skipped"`

	Items        []localCleanItem `json:"items,omitempty"`
	DeletedItems []localCleanItem `json:"deletedItems,omitempty"`
	SkippedItems []localCleanItem `json:"skippedItems,omitempty"`
}

// ProfilesLocalCommand returns the profiles local command group.
func ProfilesLocalCommand() *ffcli.Command {
	fs := flag.NewFlagSet("local", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "local",
		ShortUsage: "asc profiles local <subcommand> [flags]",
		ShortHelp:  "Manage locally installed provisioning profiles.",
		LongHelp: `Manage locally installed provisioning profiles.

These commands operate on local disk state (Xcode's Provisioning Profiles directory),
not on App Store Connect API profile resources.

Examples:
  asc profiles local install --path "./profile.mobileprovision"
  asc profiles local list
  asc profiles local clean --expired --dry-run
  asc profiles local clean --expired --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ProfilesLocalInstallCommand(),
			ProfilesLocalListCommand(),
			ProfilesLocalCleanCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ProfilesLocalInstallCommand returns the profiles local install subcommand.
func ProfilesLocalInstallCommand() *ffcli.Command {
	fs := flag.NewFlagSet("install", flag.ExitOnError)

	sourcePath := fs.String("path", "", "Path to a .mobileprovision file to install")
	profileID := fs.String("id", "", "Profile ID to download and install")
	installDir := fs.String("install-dir", "", "Install directory (defaults to Xcode's Provisioning Profiles dir on macOS)")
	force := fs.Bool("force", false, "Overwrite an existing installed profile with the same UUID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "install",
		ShortUsage: "asc profiles local install (--path \"./profile.mobileprovision\" | --id \"PROFILE_ID\") [flags]",
		ShortHelp:  "Install a provisioning profile locally.",
		LongHelp: `Install a provisioning profile locally.

By default, this installs into:
  ~/Library/MobileDevice/Provisioning Profiles

Examples:
  asc profiles local install --path "./profile.mobileprovision"
  asc profiles local install --id "PROFILE_ID"
  asc profiles local install --path "./profile.mobileprovision" --force`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pathValue := strings.TrimSpace(*sourcePath)
			idValue := strings.TrimSpace(*profileID)
			if pathValue == "" && idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --path or --id is required")
				return flag.ErrHelp
			}
			if pathValue != "" && idValue != "" {
				return shared.UsageError("--path and --id are mutually exclusive")
			}

			resolvedInstallDir, err := resolveProfilesInstallDir(*installDir)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			var (
				content []byte
				source  string
			)
			if pathValue != "" {
				file, err := shared.OpenExistingNoFollow(pathValue)
				if err != nil {
					return fmt.Errorf("profiles local install: open input: %w", err)
				}
				defer func() { _ = file.Close() }()

				data, err := io.ReadAll(file)
				if err != nil {
					return fmt.Errorf("profiles local install: read input: %w", err)
				}
				content = data
				source = pathValue
			} else {
				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("profiles local install: %w", err)
				}

				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				resp, err := client.GetProfile(requestCtx, idValue)
				if err != nil {
					return fmt.Errorf("profiles local install: failed to fetch: %w", err)
				}
				raw := strings.TrimSpace(resp.Data.Attributes.ProfileContent)
				if raw == "" {
					return fmt.Errorf("profiles local install: profile content is empty")
				}
				decoded, err := decodeProfileContent(raw)
				if err != nil {
					return fmt.Errorf("profiles local install: %w", err)
				}
				content = decoded
				source = idValue
			}

			parsed, err := parseMobileProvision(content)
			if err != nil {
				return fmt.Errorf("profiles local install: %w", err)
			}
			uuid := strings.TrimSpace(parsed.UUID)
			if uuid == "" {
				return fmt.Errorf("profiles local install: profile UUID is missing")
			}
			if !isValidProfileUUID(uuid) {
				return fmt.Errorf("profiles local install: invalid profile UUID %q", uuid)
			}

			destPath := filepath.Join(resolvedInstallDir, uuid+".mobileprovision")
			action := "installed"
			hadExisting := false
			if _, err := os.Lstat(destPath); err == nil {
				hadExisting = true
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("profiles local install: stat output path: %w", err)
			}

			if err := writeProfileFile(destPath, content, *force); err != nil {
				if errors.Is(err, os.ErrExist) {
					return fmt.Errorf("profiles local install: output file already exists: %w", err)
				}
				return fmt.Errorf("profiles local install: %w", err)
			}
			if *force && hadExisting {
				action = "replaced"
			}

			item := localProfile{
				UUID:      uuid,
				Name:      strings.TrimSpace(parsed.Name),
				TeamID:    strings.TrimSpace(parsed.TeamID()),
				BundleID:  strings.TrimSpace(parsed.BundleID()),
				ExpiresAt: parsed.ExpirationDate,
				CreatedAt: parsed.CreationDate,
				Path:      destPath,
				Expired:   isExpired(parsed.ExpirationDate, time.Now()),
			}

			result := &localInstallResult{
				Source:        source,
				InstalledPath: destPath,
				Action:        action,
				Profile:       item,
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderLocalInstallResult(result, false) },
				func() error { return renderLocalInstallResult(result, true) },
			)
		},
	}
}

// ProfilesLocalListCommand returns the profiles local list subcommand.
func ProfilesLocalListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	installDir := fs.String("install-dir", "", "Install directory (defaults to Xcode's Provisioning Profiles dir on macOS)")
	bundleID := fs.String("bundle-id", "", "Filter by bundle ID")
	teamID := fs.String("team-id", "", "Filter by team ID")
	expiredOnly := fs.Bool("expired", false, "Show only expired profiles")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc profiles local list [flags]",
		ShortHelp:  "List locally installed provisioning profiles.",
		LongHelp: `List locally installed provisioning profiles.

Examples:
  asc profiles local list
  asc profiles local list --expired
  asc profiles local list --bundle-id "com.example.app" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedInstallDir, err := resolveProfilesInstallDir(*installDir)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			scanned, skipped, err := scanLocalProfiles(resolvedInstallDir, time.Now())
			if err != nil {
				return fmt.Errorf("profiles local list: %w", err)
			}

			filtered := make([]localProfile, 0, len(scanned))
			for _, item := range scanned {
				if *expiredOnly && !item.Expired {
					continue
				}
				if b := strings.TrimSpace(*bundleID); b != "" && !strings.EqualFold(strings.TrimSpace(item.BundleID), b) {
					continue
				}
				if t := strings.TrimSpace(*teamID); t != "" && !strings.EqualFold(strings.TrimSpace(item.TeamID), t) {
					continue
				}
				filtered = append(filtered, item)
			}

			// Stable output order: expire date then UUID.
			sort.Slice(filtered, func(i, j int) bool {
				if !filtered[i].ExpiresAt.Equal(filtered[j].ExpiresAt) {
					return filtered[i].ExpiresAt.Before(filtered[j].ExpiresAt)
				}
				return filtered[i].UUID < filtered[j].UUID
			})

			sort.Slice(skipped, func(i, j int) bool {
				return skipped[i].Path < skipped[j].Path
			})

			result := &localListResult{
				InstallDir:   resolvedInstallDir,
				Total:        len(scanned) + len(skipped),
				Listed:       len(filtered),
				Skipped:      len(skipped),
				Items:        filtered,
				SkippedItems: skipped,
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderLocalListResult(result, false) },
				func() error { return renderLocalListResult(result, true) },
			)
		},
	}
}

// ProfilesLocalCleanCommand returns the profiles local clean subcommand.
func ProfilesLocalCleanCommand() *ffcli.Command {
	fs := flag.NewFlagSet("clean", flag.ExitOnError)

	installDir := fs.String("install-dir", "", "Install directory (defaults to Xcode's Provisioning Profiles dir on macOS)")
	expiredOnly := fs.Bool("expired", false, "Delete expired profiles")
	dryRun := fs.Bool("dry-run", false, "Show deletion plan without deleting")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "clean",
		ShortUsage: "asc profiles local clean --expired --dry-run | asc profiles local clean --expired --confirm",
		ShortHelp:  "Clean up locally installed provisioning profiles.",
		LongHelp: `Clean up locally installed provisioning profiles.

Examples:
  asc profiles local clean --expired --dry-run
  asc profiles local clean --expired --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*dryRun && !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}
			if !*expiredOnly {
				// At least one clean mode is required; currently only --expired exists.
				return shared.UsageError("at least one clean mode is required (e.g. --expired)")
			}

			resolvedInstallDir, err := resolveProfilesInstallDir(*installDir)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			now := time.Now()
			items, skipped, err := scanLocalProfiles(resolvedInstallDir, now)
			if err != nil {
				return fmt.Errorf("profiles local clean: %w", err)
			}

			toDelete := make([]localProfile, 0, len(items))
			for _, item := range items {
				if *expiredOnly && !item.Expired {
					continue
				}
				if !*expiredOnly {
					// Phase 1: no other clean modes yet.
					continue
				}
				toDelete = append(toDelete, item)
			}

			result := &localCleanResult{
				InstallDir: resolvedInstallDir,
				Expired:    *expiredOnly,
				DryRun:     *dryRun,
				Planned:    len(toDelete),
			}
			for _, item := range skipped {
				result.Skipped++
				result.SkippedItems = append(result.SkippedItems, localCleanItem{
					Path:   item.Path,
					Reason: item.Reason,
				})
			}
			for _, item := range toDelete {
				result.Items = append(result.Items, localCleanItem{
					UUID: item.UUID,
					Name: item.Name,
					Path: item.Path,
				})
			}

			if *dryRun {
				return shared.PrintOutputWithRenderers(
					result,
					*output.Output,
					*output.Pretty,
					func() error { return renderLocalCleanResult(result, false) },
					func() error { return renderLocalCleanResult(result, true) },
				)
			}

			for _, item := range toDelete {
				if err := deleteLocalProfileFile(item.Path); err != nil {
					result.Skipped++
					result.SkippedItems = append(result.SkippedItems, localCleanItem{UUID: item.UUID, Name: item.Name, Path: item.Path, Reason: err.Error()})
					continue
				}
				result.Deleted++
				result.DeletedItems = append(result.DeletedItems, localCleanItem{UUID: item.UUID, Name: item.Name, Path: item.Path})
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderLocalCleanResult(result, false) },
				func() error { return renderLocalCleanResult(result, true) },
			)
		},
	}
}

func resolveProfilesInstallDir(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" {
		return filepath.Clean(trimmed), nil
	}
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("--install-dir is required on non-macOS platforms")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "MobileDevice", "Provisioning Profiles"), nil
}

func scanLocalProfiles(installDir string, now time.Time) ([]localProfile, []localSkippedItem, error) {
	entries, err := os.ReadDir(installDir)
	if err != nil {
		// If directory doesn't exist, treat as empty.
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	items := make([]localProfile, 0, len(entries))
	skipped := make([]localSkippedItem, 0, 4)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".mobileprovision") {
			continue
		}

		fullPath := filepath.Join(installDir, entry.Name())

		// Treat the install directory as untrusted disk state. Refuse non-regular files
		// (FIFOs/devices/sockets) to avoid blocking reads, and refuse symlinks.
		info, err := os.Lstat(fullPath)
		if err != nil {
			skipped = append(skipped, localSkippedItem{
				Path:   fullPath,
				Reason: fmt.Sprintf("stat: %v", err),
			})
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			skipped = append(skipped, localSkippedItem{
				Path:   fullPath,
				Reason: "refusing to follow symlink",
			})
			continue
		}
		if !info.Mode().IsRegular() {
			skipped = append(skipped, localSkippedItem{
				Path:   fullPath,
				Reason: "refusing to read non-regular file",
			})
			continue
		}

		file, err := shared.OpenExistingNoFollow(fullPath)
		if err != nil {
			skipped = append(skipped, localSkippedItem{
				Path:   fullPath,
				Reason: fmt.Sprintf("open: %v", err),
			})
			continue
		}

		// Re-check after open to close the TOCTOU gap between the Lstat pre-check
		// and the actual open (the file may have been replaced).
		openedInfo, statErr := file.Stat()
		if statErr != nil {
			_ = file.Close()
			skipped = append(skipped, localSkippedItem{
				Path:   fullPath,
				Reason: fmt.Sprintf("stat (opened): %v", statErr),
			})
			continue
		}
		if !openedInfo.Mode().IsRegular() {
			_ = file.Close()
			skipped = append(skipped, localSkippedItem{
				Path:   fullPath,
				Reason: "refusing to read non-regular file",
			})
			continue
		}

		data, readErr := io.ReadAll(file)
		_ = file.Close()
		if readErr != nil {
			skipped = append(skipped, localSkippedItem{
				Path:   fullPath,
				Reason: fmt.Sprintf("read: %v", readErr),
			})
			continue
		}

		parsed, err := parseMobileProvision(data)
		if err != nil {
			skipped = append(skipped, localSkippedItem{
				Path:   fullPath,
				Reason: fmt.Sprintf("parse: %v", err),
			})
			continue
		}

		uuid := strings.TrimSpace(parsed.UUID)
		if uuid == "" {
			skipped = append(skipped, localSkippedItem{
				Path:   fullPath,
				Reason: "profile UUID is missing",
			})
			continue
		}

		items = append(items, localProfile{
			UUID:      uuid,
			Name:      strings.TrimSpace(parsed.Name),
			TeamID:    strings.TrimSpace(parsed.TeamID()),
			BundleID:  strings.TrimSpace(parsed.BundleID()),
			ExpiresAt: parsed.ExpirationDate,
			CreatedAt: parsed.CreationDate,
			Path:      fullPath,
			Expired:   isExpired(parsed.ExpirationDate, now),
		})
	}

	return items, skipped, nil
}

func isExpired(expiresAt time.Time, now time.Time) bool {
	if expiresAt.IsZero() {
		return false
	}
	// Expiration is inclusive: if now == expiresAt, treat as expired.
	return !now.Before(expiresAt)
}

func deleteLocalProfileFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to delete symlink %q", path)
	}
	if info.IsDir() {
		return fmt.Errorf("refusing to delete directory %q", path)
	}
	return os.Remove(path)
}

func renderLocalInstallResult(result *localInstallResult, markdown bool) error {
	if result == nil {
		return fmt.Errorf("result is nil")
	}

	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}

	render(
		[]string{"UUID", "Name", "Team ID", "Bundle ID", "Expired", "Installed Path", "Action"},
		[][]string{{
			result.Profile.UUID,
			result.Profile.Name,
			result.Profile.TeamID,
			result.Profile.BundleID,
			fmt.Sprintf("%t", result.Profile.Expired),
			result.InstalledPath,
			result.Action,
		}},
	)
	return nil
}

func renderLocalListResult(result *localListResult, markdown bool) error {
	if result == nil {
		return fmt.Errorf("result is nil")
	}

	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}

	render(
		[]string{"Install Dir", "Total", "Listed", "Skipped"},
		[][]string{{
			result.InstallDir,
			fmt.Sprintf("%d", result.Total),
			fmt.Sprintf("%d", result.Listed),
			fmt.Sprintf("%d", result.Skipped),
		}},
	)

	rows := make([][]string, 0, len(result.Items))
	for _, item := range result.Items {
		rows = append(rows, []string{
			item.UUID,
			item.Name,
			item.TeamID,
			item.BundleID,
			item.ExpiresAt.Format(time.RFC3339),
			fmt.Sprintf("%t", item.Expired),
			item.Path,
		})
	}
	render([]string{"UUID", "Name", "Team ID", "Bundle ID", "Expires At", "Expired", "Path"}, rows)

	if len(result.SkippedItems) > 0 {
		skippedRows := make([][]string, 0, len(result.SkippedItems))
		for _, item := range result.SkippedItems {
			skippedRows = append(skippedRows, []string{item.Path, item.Reason})
		}
		render([]string{"Skipped Path", "Reason"}, skippedRows)
	}
	return nil
}

func renderLocalCleanResult(result *localCleanResult, markdown bool) error {
	if result == nil {
		return fmt.Errorf("result is nil")
	}

	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}

	render(
		[]string{"Install Dir", "Expired", "Dry Run", "Planned", "Deleted", "Skipped"},
		[][]string{{
			result.InstallDir,
			fmt.Sprintf("%t", result.Expired),
			fmt.Sprintf("%t", result.DryRun),
			fmt.Sprintf("%d", result.Planned),
			fmt.Sprintf("%d", result.Deleted),
			fmt.Sprintf("%d", result.Skipped),
		}},
	)

	if len(result.Items) > 0 {
		rows := make([][]string, 0, len(result.Items))
		for _, item := range result.Items {
			rows = append(rows, []string{item.UUID, item.Name, item.Path})
		}
		render([]string{"UUID", "Name", "Path"}, rows)
	}

	if len(result.SkippedItems) > 0 {
		rows := make([][]string, 0, len(result.SkippedItems))
		for _, item := range result.SkippedItems {
			rows = append(rows, []string{item.UUID, item.Name, item.Path, item.Reason})
		}
		render([]string{"UUID", "Name", "Path", "Reason"}, rows)
	}
	return nil
}
