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
	xcodecore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

const profilesLocalDirectoryHelp = `On macOS, the default directory follows the active Xcode:
  Xcode 16 or newer: ~/Library/Developer/Xcode/UserData/Provisioning Profiles
  Xcode 15 or older: ~/Library/MobileDevice/Provisioning Profiles

When no full Xcode is active (for example a Command Line Tools only host),
~/Library/MobileDevice/Provisioning Profiles is used and a note is written to
stderr. Use --install-dir to choose a directory explicitly.`

const profilesLocalExtensionHelp = `macOS profiles use the .provisionprofile extension;
iOS, tvOS, and legacy profiles may use .mobileprovision.`

// profileUUIDValidationRegex ensures the UUID from a provisioning profile is safe to use
// as a filename component (prevents absolute paths / path traversal).
var profileUUIDValidationRegex = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var (
	errProfilesInstallDirRequired = errors.New("--install-dir is required on non-macOS platforms")
	profilesRuntimeGOOS           = runtime.GOOS
	profilesUserHomeDirFn         = os.UserHomeDir
	activeXcodeMajorVersionFn     = xcodecore.ActiveXcodeMajorVersion
)

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

` + profilesLocalDirectoryHelp + `

` + profilesLocalExtensionHelp + `

Examples:
  asc profiles local install --path "./profile.mobileprovision"
  asc profiles local install --path "./profile.provisionprofile"
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

	sourcePath := fs.String("path", "", "Path to a .mobileprovision or .provisionprofile file to install")
	profileID := fs.String("id", "", "Profile ID to download and install")
	installDir := fs.String("install-dir", "", "Directory to use (defaults by active Xcode version on macOS)")
	force := fs.Bool("force", false, "Overwrite an existing installed profile with the same UUID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "install",
		ShortUsage: "asc profiles local install (--path \"./profile.mobileprovision\" | --id \"PROFILE_ID\") [flags]",
		ShortHelp:  "Install a provisioning profile locally.",
		LongHelp: `Install a provisioning profile locally.

` + profilesLocalDirectoryHelp + `

` + profilesLocalExtensionHelp + `

Examples:
  asc profiles local install --path "./profile.mobileprovision"
  asc profiles local install --path "./profile.provisionprofile"
  asc profiles local install --id "PROFILE_ID"
  asc profiles local install --path "./profile.mobileprovision" --force`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pathValue := strings.TrimSpace(*sourcePath)
			idValue := strings.TrimSpace(*profileID)
			if pathValue == "" && idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --path or --id is required")
				return shared.MissingRequiredUsageError("")
			}
			if pathValue != "" && idValue != "" {
				return shared.UsageError("--path and --id are mutually exclusive")
			}

			resolvedInstallDir, err := resolveProfilesInstallDirForCommand(ctx, *installDir, "profiles local install")
			if err != nil {
				return err
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
				defer file.Close()

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

			extension := shared.ProvisioningProfileExtensionForPlatforms(parsed.Platform, "")
			destPath, hadExisting, err := resolveProfileInstallPath(resolvedInstallDir, uuid, extension)
			if err != nil {
				return fmt.Errorf("profiles local install: %w", err)
			}
			action := "installed"

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

func resolveProfileInstallPath(installDir, uuid, preferredExtension string) (string, bool, error) {
	preferredPath := filepath.Join(installDir, uuid+preferredExtension)
	alternateExtension := shared.IOSProvisioningProfileExtension
	if preferredExtension == shared.IOSProvisioningProfileExtension {
		alternateExtension = shared.MacOSProvisioningProfileExtension
	}
	alternatePath := filepath.Join(installDir, uuid+alternateExtension)

	preferredExists, err := profileInstallPathExists(preferredPath)
	if err != nil {
		return "", false, err
	}
	alternateExists, err := profileInstallPathExists(alternatePath)
	if err != nil {
		return "", false, err
	}
	if preferredExists && alternateExists {
		return "", false, fmt.Errorf("duplicate installed profiles exist for UUID %q at %q and %q", uuid, preferredPath, alternatePath)
	}
	if preferredExists {
		return preferredPath, true, nil
	}
	if alternateExists {
		// Preserve an existing legacy filename instead of creating a second file
		// for the same profile UUID. A later forced install replaces it in place.
		return alternatePath, true, nil
	}
	return preferredPath, false, nil
}

func profileInstallPathExists(path string) (bool, error) {
	if _, err := os.Lstat(path); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat output path: %w", err)
	}
	return false, nil
}

// ProfilesLocalListCommand returns the profiles local list subcommand.
func ProfilesLocalListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	installDir := fs.String("install-dir", "", "Directory to use (defaults by active Xcode version on macOS)")
	bundleID := fs.String("bundle-id", "", "Filter by bundle ID")
	teamID := fs.String("team-id", "", "Filter by team ID")
	expiredOnly := fs.Bool("expired", false, "Show only expired profiles")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc profiles local list [flags]",
		ShortHelp:  "List locally installed provisioning profiles.",
		LongHelp: `List locally installed provisioning profiles.

` + profilesLocalDirectoryHelp + `

` + profilesLocalExtensionHelp + `

Examples:
  asc profiles local list
  asc profiles local list --expired
  asc profiles local list --bundle-id "com.example.app" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedInstallDir, err := resolveProfilesInstallDirForCommand(ctx, *installDir, "profiles local list")
			if err != nil {
				return err
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

	installDir := fs.String("install-dir", "", "Directory to use (defaults by active Xcode version on macOS)")
	expiredOnly := fs.Bool("expired", false, "Delete expired profiles")
	dryRun := fs.Bool("dry-run", false, "Show deletion plan without deleting")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "clean",
		ShortUsage: "asc profiles local clean --expired --dry-run | asc profiles local clean --expired --confirm",
		ShortHelp:  "Clean up locally installed provisioning profiles.",
		LongHelp: `Clean up locally installed provisioning profiles.

` + profilesLocalDirectoryHelp + `

` + profilesLocalExtensionHelp + `

Examples:
  asc profiles local clean --expired --dry-run
  asc profiles local clean --expired --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*dryRun && !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}
			if !*expiredOnly {
				// At least one clean mode is required; currently only --expired exists.
				return shared.UsageError("at least one clean mode is required (e.g. --expired)")
			}

			resolvedInstallDir, err := resolveProfilesInstallDirForCommand(ctx, *installDir, "profiles local clean")
			if err != nil {
				return err
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

func resolveProfilesInstallDirForCommand(ctx context.Context, value, commandName string) (string, error) {
	installDir, err := resolveProfilesInstallDir(ctx, value)
	if err == nil {
		return installDir, nil
	}
	if errors.Is(err, errProfilesInstallDirRequired) {
		return "", shared.UsageError(err.Error())
	}
	return "", fmt.Errorf("%s: resolve install directory: %w", commandName, err)
}

func resolveProfilesInstallDir(ctx context.Context, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" {
		return filepath.Clean(trimmed), nil
	}
	if profilesRuntimeGOOS != "darwin" {
		return "", errProfilesInstallDirRequired
	}

	home, err := profilesUserHomeDirFn()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	legacyDir := filepath.Join(home, "Library", "MobileDevice", "Provisioning Profiles")

	major, err := activeXcodeMajorVersionFn(ctx)
	if err == nil && major < 1 {
		err = fmt.Errorf("invalid major version %d", major)
	}
	if err != nil {
		// A cancelled or expired context is the caller giving up, not a host
		// without a full Xcode.
		if ctxErr := context.Cause(ctx); ctxErr != nil {
			return "", ctxErr
		}
		// Hosts with only Command Line Tools cannot report an Xcode version.
		// Keep the pre-4.0 default working there instead of failing closed.
		fmt.Fprintf(
			os.Stderr,
			"Note: could not determine the active Xcode version (%v); using %s. Pass --install-dir to choose another directory.\n",
			err,
			legacyDir,
		)
		return legacyDir, nil
	}

	if major >= 16 {
		return filepath.Join(home, "Library", "Developer", "Xcode", "UserData", "Provisioning Profiles"), nil
	}
	return legacyDir, nil
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
		if !shared.IsProvisioningProfilePath(entry.Name()) {
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
