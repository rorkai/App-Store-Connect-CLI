package signing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

// appleSiliconMacUDID matches the provisioning UDID format of Apple silicon
// Macs (for example 00008103-001A2B3C4D5E6F70). Intel Macs use the hardware
// UUID format and cannot run iOS apps, so they are never added to iOS
// profiles.
var appleSiliconMacUDID = regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{16}$`)

// isDeviceProfileType reports whether a profile type carries an explicit
// device list that --force-for-new-devices can refresh.
func isDeviceProfileType(profileType string) bool {
	switch strings.ToUpper(strings.TrimSpace(profileType)) {
	case "IOS_APP_DEVELOPMENT", "IOS_APP_ADHOC",
		"TVOS_APP_DEVELOPMENT", "TVOS_APP_ADHOC",
		"MAC_APP_DEVELOPMENT", "MAC_CATALYST_APP_DEVELOPMENT":
		return true
	default:
		return false
	}
}

// supportsMacDeviceInclusion reports whether Apple silicon Macs can run the
// iOS app a profile type provisions.
func supportsMacDeviceInclusion(profileType string) bool {
	switch strings.ToUpper(strings.TrimSpace(profileType)) {
	case "IOS_APP_DEVELOPMENT", "IOS_APP_ADHOC":
		return true
	default:
		return false
	}
}

// profileTypeAcceptsDevice reports whether an enabled device belongs in a
// profile of the given type.
func profileTypeAcceptsDevice(profileType string, device asc.DeviceAttributes, includeMac bool) bool {
	class := asc.DeviceClass(strings.ToUpper(strings.TrimSpace(string(device.DeviceClass))))
	switch strings.ToUpper(strings.TrimSpace(profileType)) {
	case "IOS_APP_DEVELOPMENT", "IOS_APP_ADHOC":
		switch class {
		case asc.DeviceClassIPhone, asc.DeviceClassIPad, asc.DeviceClassIPod, asc.DeviceClassAppleWatch, "APPLE_VISION_PRO":
			return true
		case asc.DeviceClassMac:
			return includeMac && appleSiliconMacUDID.MatchString(strings.TrimSpace(device.UDID))
		default:
			return false
		}
	case "TVOS_APP_DEVELOPMENT", "TVOS_APP_ADHOC":
		return class == asc.DeviceClassAppleTV
	case "MAC_APP_DEVELOPMENT", "MAC_CATALYST_APP_DEVELOPMENT":
		return class == asc.DeviceClassMac
	default:
		return false
	}
}

// listEnabledProfileDeviceIDs returns the sorted IDs of every enabled device
// that belongs in a profile of the given type.
func listEnabledProfileDeviceIDs(ctx context.Context, client *asc.Client, profileType string, includeMac bool) ([]string, error) {
	ids := []string{}
	next := ""
	page := 1
	seenNext := make(map[string]struct{})
	for {
		options := []asc.DevicesOption{asc.NextURL[asc.DevicesOption](next)}
		if next == "" {
			options = append(options, asc.WithDevicesFilterStatuses([]string{string(asc.DeviceStatusEnabled)}), asc.Limit[asc.DevicesOption](200))
		}
		response, err := client.GetDevices(ctx, options...)
		if err != nil {
			return nil, fmt.Errorf("list enabled devices: %w", err)
		}
		for _, device := range response.Data {
			if device.Attributes.Status != "" && device.Attributes.Status != asc.DeviceStatusEnabled {
				continue
			}
			if profileTypeAcceptsDevice(profileType, device.Attributes, includeMac) {
				ids = append(ids, strings.TrimSpace(device.ID))
			}
		}
		if strings.TrimSpace(response.Links.Next) == "" {
			break
		}
		if _, ok := seenNext[response.Links.Next]; ok {
			return nil, fmt.Errorf("list enabled devices: page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[response.Links.Next] = struct{}{}
		page++
		next = response.Links.Next
	}
	ids = uniqueSortedSigningSyncStrings(ids)
	if len(ids) == 0 {
		return nil, fmt.Errorf("no enabled devices match %s profiles; register a device or pass --device", strings.ToUpper(strings.TrimSpace(profileType)))
	}
	return ids, nil
}

// listProfileDevices returns every device a profile currently includes.
func listProfileDevices(ctx context.Context, client *asc.Client, profileID string) ([]asc.Resource[asc.DeviceAttributes], error) {
	var devices []asc.Resource[asc.DeviceAttributes]
	next := ""
	page := 1
	seenNext := make(map[string]struct{})
	for {
		options := []asc.ProfileDevicesOption{asc.WithProfileDevicesNextURL(next)}
		if next == "" {
			options = append(options, asc.WithProfileDevicesLimit(200))
		}
		response, err := client.GetProfileDevices(ctx, profileID, options...)
		if err != nil {
			return nil, fmt.Errorf("list devices for profile %s: %w", profileID, err)
		}
		devices = append(devices, response.Data...)
		if strings.TrimSpace(response.Links.Next) == "" {
			return devices, nil
		}
		if _, ok := seenNext[response.Links.Next]; ok {
			return nil, fmt.Errorf("list devices for profile %s: page %d: %w", profileID, page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[response.Links.Next] = struct{}{}
		page++
		next = response.Links.Next
	}
}

func deviceResourceIDs(devices []asc.Resource[asc.DeviceAttributes], enabledOnly bool) []string {
	ids := make([]string, 0, len(devices))
	for _, device := range devices {
		if enabledOnly && device.Attributes.Status != "" && device.Attributes.Status != asc.DeviceStatusEnabled {
			continue
		}
		ids = append(ids, strings.TrimSpace(device.ID))
	}
	return uniqueSortedSigningSyncStrings(ids)
}

// diffDeviceIDs reports the device IDs a replacement adds and removes.
func diffDeviceIDs(current, desired []string) (added, removed []string) {
	currentSet := make(map[string]struct{}, len(current))
	for _, id := range current {
		currentSet[id] = struct{}{}
	}
	desiredSet := make(map[string]struct{}, len(desired))
	for _, id := range desired {
		desiredSet[id] = struct{}{}
		if _, ok := currentSet[id]; !ok {
			added = append(added, id)
		}
	}
	for _, id := range current {
		if _, ok := desiredSet[id]; !ok {
			removed = append(removed, id)
		}
	}
	if len(added) > 0 {
		added = uniqueSortedSigningSyncStrings(added)
	}
	if len(removed) > 0 {
		removed = uniqueSortedSigningSyncStrings(removed)
	}
	return added, removed
}

// findLatestExpiredProfile returns the profile of the given type whose
// expiration date passed most recently, or nil when none has expired.
func findLatestExpiredProfile(ctx context.Context, client *asc.Client, bundleIDResourceID, profileType string) (*asc.Resource[asc.ProfileAttributes], error) {
	now := signingFetchNowFn()
	var selected *asc.Resource[asc.ProfileAttributes]
	var selectedExpiry string
	next := ""
	page := 1
	seenNext := make(map[string]struct{})
	for {
		profiles, err := client.GetBundleIDProfiles(ctx, bundleIDResourceID, asc.WithBundleIDProfilesLimit(200), asc.WithBundleIDProfilesNextURL(next))
		if err != nil {
			return nil, err
		}
		for index := range profiles.Data {
			profile := profiles.Data[index]
			if !strings.EqualFold(strings.TrimSpace(profile.Attributes.ProfileType), profileType) {
				continue
			}
			if !asc.ProfileExpirationPassed(profile.Attributes.ExpirationDate, now) {
				continue
			}
			expiry := normalizedProfileExpiry(profile.Attributes.ExpirationDate)
			if selected == nil || expiry > selectedExpiry || (expiry == selectedExpiry && profile.ID < selected.ID) {
				copyProfile := profile
				selected = &copyProfile
				selectedExpiry = expiry
			}
		}
		if strings.TrimSpace(profiles.Links.Next) == "" {
			return selected, nil
		}
		if _, ok := seenNext[profiles.Links.Next]; ok {
			return nil, fmt.Errorf("page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[profiles.Links.Next] = struct{}{}
		page++
		next = profiles.Links.Next
	}
}

func normalizedProfileExpiry(value string) string {
	parsed, ok := parseProfileExpiry(value)
	if !ok {
		return ""
	}
	return parsed.UTC().Format("2006-01-02T15:04:05.000000000Z")
}

// profileReplacement describes an existing profile that push deletes right
// before creating its same-name replacement.
type profileReplacement struct {
	Profile   asc.Resource[asc.ProfileAttributes]
	Lifecycle *asc.SigningSyncProfileLifecycle
}

// deleteReplacedProfile deletes the profile being replaced and records the
// outcome on the lifecycle receipt. A failed request leaves the state unknown
// because App Store Connect may have applied it.
func deleteReplacedProfile(ctx context.Context, client *asc.Client, replacement *profileReplacement, progress *signingAssetsProgress) error {
	if progress != nil {
		progress.ReplacementAttempted = true
	}
	if err := client.DeleteProfile(ctx, replacement.Profile.ID); err != nil {
		replacement.Lifecycle.ReplacedProfileState = asc.SigningSyncReplacedProfileUnknown
		return fmt.Errorf("delete replaced profile %s: %w", replacement.Profile.ID, err)
	}
	replacement.Lifecycle.ReplacedProfileState = asc.SigningSyncReplacedProfileDeleted
	return nil
}

// signingLifecycleReplacedProfileID returns the profile ID a lifecycle receipt
// replaced, when App Store Connect confirmed or may have applied its deletion.
func signingLifecycleReplacedProfileID(lifecycle *asc.SigningSyncProfileLifecycle) string {
	if lifecycle == nil || lifecycle.ReplacedProfileState == "" {
		return ""
	}
	return strings.TrimSpace(lifecycle.ReplacedProfileID)
}

// removeSupersededProfileArtifacts removes encrypted artifacts of a profile
// that push replaced, except the path the replacement was written to. An
// artifact is selected only by its authenticated profile resource ID, or by
// the target-scoped batch path that embeds that ID, so artifacts of other
// profiles are never touched.
func removeSupersededProfileArtifacts(store *signingpkg.GitStore, password, bundleID, profileType, replacedProfileID, keepPath string) ([]string, error) {
	replacedProfileID = strings.TrimSpace(replacedProfileID)
	if store == nil || replacedProfileID == "" {
		return nil, nil
	}
	files, err := store.ListEncryptedFiles()
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	keep := canonicalSigningPullPath(keepPath)
	batchPath := canonicalSigningPullPath(signingSyncBatchProfilePath(bundleID, profileType, replacedProfileID))
	batchLegacyPath := strings.TrimSuffix(batchPath, filepath.Ext(batchPath)) + ".mobileprovision"
	var selected []string
	for _, file := range files {
		canonical := canonicalSigningPullPath(file)
		if canonical == keep || !strings.HasPrefix(canonical, "profiles/") {
			continue
		}
		_, metadata, readErr := store.ReadEncryptedFileWithMetadata(file, password)
		if readErr != nil {
			return nil, fmt.Errorf("authenticate %s: %w", file, readErr)
		}
		switch {
		case metadata.Kind == signingProfileArtifactKind:
			if metadata.ProfileResourceID == replacedProfileID && strings.EqualFold(metadata.ProfileType, profileType) {
				selected = append(selected, file)
			}
		case metadata.Version == 0 && (canonical == batchPath || canonical == batchLegacyPath):
			selected = append(selected, file)
		}
	}
	if len(selected) == 0 {
		return nil, nil
	}
	if err := removeEncryptedSigningFiles(store, selected); err != nil {
		return nil, err
	}
	return selected, nil
}

// removeEncryptedSigningFiles deletes encrypted artifacts from the local
// working tree through the store's anchored root. Publication stages the
// deletions together with any other change in one commit.
func removeEncryptedSigningFiles(store *signingpkg.GitStore, relPaths []string) error {
	if len(relPaths) == 0 {
		return nil
	}
	root, err := rootfs.New(store.LocalDir)
	if err != nil {
		return err
	}
	defer root.Close()
	opened, err := root.OpenRoot()
	if err != nil {
		return err
	}
	defer opened.Close()
	for _, relPath := range relPaths {
		if err := signingpkg.ValidateEncryptedRepositoryPaths([]string{relPath}); err != nil {
			return err
		}
		if err := opened.Remove(filepath.FromSlash(relPath) + ".enc"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", relPath, err)
		}
	}
	return nil
}

func parseProfileExpiry(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

// validateSigningSyncLifecycleFlags rejects lifecycle flag combinations
// before any secret read, client creation, or network request.
func validateSigningSyncLifecycleFlags(profileType, storageKind, deviceIDs string, renewExpired, forceForNewDevices, includeMac bool) error {
	if forceForNewDevices && !isDeviceProfileType(profileType) {
		return shared.UsageErrorf("--force-for-new-devices requires a development or ad hoc profile type; %s profiles have no device list", profileType)
	}
	if includeMac && !forceForNewDevices {
		return shared.UsageError("--include-mac-in-profiles requires --force-for-new-devices")
	}
	if includeMac && !supportsMacDeviceInclusion(profileType) {
		return shared.UsageError("--include-mac-in-profiles supports only IOS_APP_DEVELOPMENT and IOS_APP_ADHOC profiles")
	}
	if includeMac && strings.TrimSpace(deviceIDs) != "" {
		return shared.UsageError("--include-mac-in-profiles cannot be combined with --device; list Mac device IDs in --device instead")
	}
	if (renewExpired || forceForNewDevices) && storageKind != signingSyncStorageGit {
		return shared.UsageErrorf("--renew-expired and --force-for-new-devices require --storage %s so replaced profiles can be removed from the store", signingSyncStorageGit)
	}
	return nil
}

// reportSigningProfileLifecycle prints a one-line diagnostic for a profile
// that push kept or replaced because of a lifecycle flag.
func reportSigningProfileLifecycle(bundleID string, lifecycle *asc.SigningSyncProfileLifecycle) {
	if lifecycle == nil {
		return
	}
	switch lifecycle.Action {
	case asc.SigningSyncLifecycleRenewed:
		fmt.Fprintf(os.Stderr, "Renewed expired profile %s for %s as %s\n", lifecycle.ReplacedProfileID, bundleID, lifecycle.ProfileID)
	case asc.SigningSyncLifecycleDevicesRefreshed:
		fmt.Fprintf(os.Stderr, "Recreated profile %s for %s as %s (%d device(s) added, %d removed)\n", lifecycle.ReplacedProfileID, bundleID, lifecycle.ProfileID, len(lifecycle.DevicesAdded), len(lifecycle.DevicesRemoved))
	}
}
