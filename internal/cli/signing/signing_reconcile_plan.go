package signing

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"go.mozilla.org/pkcs7"
	"howett.net/plist"
)

func executeSigningReconcilePlan(ctx context.Context, options signingReconcilePlanOptions) (signingReconcilePlanArtifact, error) {
	paths := reconcilePaths(options)
	if err := validateSigningReconcileInputPaths(paths); err != nil {
		return signingReconcilePlanArtifact{}, err
	}
	deviceBytes, err := readProtectedFile(paths.DevicesFile)
	if err != nil {
		return signingReconcilePlanArtifact{}, protectedDevicesFileUsageError(err)
	}
	devicesFile, err := decodeSigningDevicesFile(deviceBytes)
	if err != nil {
		return signingReconcilePlanArtifact{}, invalidDevicesFileUsageError(err)
	}
	archive, err := readSigningArchiveRequirements(paths.ArchivePath)
	if err != nil {
		return signingReconcilePlanArtifact{}, newReconcilePlanInvalid(fmt.Errorf("inspect archive: %w", sanitizeReconcileError(err, devicesFile)))
	}
	if len(archive.Targets) == 0 {
		return signingReconcilePlanArtifact{}, newReconcilePlanInvalid(fmt.Errorf("archive contains no signing targets"))
	}

	client, err := sharedASCClient()
	if err != nil {
		return signingReconcilePlanArtifact{}, err
	}
	requestCtx, cancel := signingRequestContext(ctx)
	defer cancel()

	plan := signingReconcilePlanArtifact{
		SchemaVersion:       signingReconcileSchemaV1,
		GeneratedAt:         nowRFC3339(),
		Command:             "signing reconcile plan",
		Ready:               true,
		TeamID:              archive.TeamID,
		MinimumValidityDays: options.MinimumValidityDays,
		MaxMutations:        options.MaxMutations,
		Paths:               paths,
		Targets:             archive.Targets,
		DeviceSetSHA256:     digestSigningDeviceInputs(devicesFile.Devices).SHA256,
	}

	remoteDevices, err := getAllReconcileDevices(requestCtx, client)
	if err != nil {
		return signingReconcilePlanArtifact{}, fmt.Errorf("list devices: %w", sanitizeReconcileError(err, devicesFile))
	}
	resolvedDevices, deviceActions, deviceBlockers := planDesiredDevices(devicesFile.Devices, remoteDevices)
	plan.Devices = resolvedDevices
	plan.Actions = append(plan.Actions, deviceActions...)
	plan.Blockers = append(plan.Blockers, deviceBlockers...)

	certificates, err := getAllReconcileCertificates(requestCtx, client)
	if err != nil {
		return signingReconcilePlanArtifact{}, fmt.Errorf("list certificates: %w", err)
	}
	selectedCertificate, certBlockers := selectReconcileCertificateWithFingerprint(certificates, options.CertificateID, options.CertificateSHA256, time.Now(), options.MinimumValidityDays)
	plan.Certificate = selectedCertificate
	plan.Blockers = append(plan.Blockers, certBlockers...)
	if selectedCertificate != nil && selectedCertificate.TeamID != archive.TeamID {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("certificate %s belongs to team %s, archive uses %s", selectedCertificate.ID, selectedCertificate.TeamID, archive.TeamID))
	}

	for _, target := range plan.Targets {
		observed, actions, blockers, err := planSigningTarget(requestCtx, client, target, plan.Devices, selectedCertificate, options.MinimumValidityDays)
		if err != nil {
			return signingReconcilePlanArtifact{}, fmt.Errorf("bundle %s: %w", target.BundleID, err)
		}
		plan.ObservedBundles = append(plan.ObservedBundles, observed)
		plan.Actions = append(plan.Actions, actions...)
		plan.Blockers = append(plan.Blockers, blockers...)
	}

	for _, action := range plan.Actions {
		switch action.Kind {
		case actionRegisterDevice, actionCreateBundleID, actionCreateProfile:
			plan.MutationCount++
		}
	}
	if plan.MutationCount > plan.MaxMutations {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("plan requires %d mutations, exceeding --max-mutations %d", plan.MutationCount, plan.MaxMutations))
	}
	plan.Blockers = uniqueSortedStrings(plan.Blockers)
	plan.Ready = len(plan.Blockers) == 0
	plan.PlanHash, err = hashSigningReconcilePlan(plan)
	if err != nil {
		return signingReconcilePlanArtifact{}, newReconcilePlanInvalid(fmt.Errorf("hash plan: %w", err))
	}
	if err := writeSigningStateJSON(plan.Paths.StateDir, "plan.json", plan, options.Overwrite); err != nil {
		if errors.Is(err, os.ErrExist) {
			return signingReconcilePlanArtifact{}, newReconcilePlanInvalid(fmt.Errorf("write %s: %w", plan.Paths.PlanPath, err))
		}
		return signingReconcilePlanArtifact{}, fmt.Errorf("write %s: %w", plan.Paths.PlanPath, err)
	}
	return plan, nil
}

func validateSigningReconcileInputPaths(paths signingReconcilePaths) error {
	devicesPath, err := filepath.Abs(paths.DevicesFile)
	if err != nil {
		return shared.UsageErrorf("invalid --devices-file path")
	}
	planPath, err := filepath.Abs(paths.PlanPath)
	if err != nil {
		return shared.UsageErrorf("invalid --state-dir path")
	}
	if devicesPath == planPath {
		return shared.UsageErrorf("--devices-file must be distinct from the generated plan path")
	}
	devicesInfo, devicesErr := os.Stat(devicesPath)
	planInfo, planErr := os.Stat(planPath)
	if devicesErr == nil && planErr == nil && os.SameFile(devicesInfo, planInfo) {
		return shared.UsageErrorf("--devices-file must be distinct from the generated plan path")
	}
	return nil
}

// These indirections make the planning core independently testable while the
// public command continues to use the shared auth and timeout contracts.
var (
	sharedASCClient       = func() (*asc.Client, error) { return shared.GetASCClient() }
	signingRequestContext = func(ctx context.Context) (context.Context, context.CancelFunc) {
		return shared.ContextWithResolvedTimeout(ctx, reconcileWorkflowTimeout)
	}
)

func getAllReconcileDevices(ctx context.Context, client *asc.Client) ([]asc.Resource[asc.DeviceAttributes], error) {
	var result []asc.Resource[asc.DeviceAttributes]
	next := ""
	for {
		page, err := client.GetDevices(
			ctx,
			asc.WithDevicesPlatforms([]string{string(asc.DevicePlatformIOS)}),
			asc.WithDevicesFields([]string{"name", "udid", "platform", "status"}),
			asc.WithDevicesLimit(200),
			asc.WithDevicesNextURL(next),
		)
		if err != nil {
			return nil, err
		}
		result = append(result, page.Data...)
		if strings.TrimSpace(page.Links.Next) == "" {
			break
		}
		next = page.Links.Next
	}
	return result, nil
}

func getAllReconcileCertificates(ctx context.Context, client *asc.Client) ([]asc.Resource[asc.CertificateAttributes], error) {
	var result []asc.Resource[asc.CertificateAttributes]
	next := ""
	for {
		page, err := client.GetCertificates(
			ctx,
			asc.WithCertificatesFilterType(reconcileCertificateType),
			asc.WithCertificatesLimit(200),
			asc.WithCertificatesNextURL(next),
		)
		if err != nil {
			return nil, err
		}
		result = append(result, page.Data...)
		if strings.TrimSpace(page.Links.Next) == "" {
			break
		}
		next = page.Links.Next
	}
	return result, nil
}

func planDesiredDevices(desired []signingDeviceInput, remote []asc.Resource[asc.DeviceAttributes]) ([]signingDesiredDevice, []signingAction, []string) {
	byUDID := make(map[string][]asc.Resource[asc.DeviceAttributes])
	for _, device := range remote {
		normalized := normalizeReconcileUDID(device.Attributes.UDID)
		if normalized != "" {
			byUDID[normalized] = append(byUDID[normalized], device)
		}
	}
	var result []signingDesiredDevice
	var actions []signingAction
	var blockers []string
	for _, input := range desired {
		item := signingDesiredDevice{Platform: input.Platform, Fingerprint: input.Fingerprint, NameSHA256: fingerprintReconcileName(input.Name)}
		matches := byUDID[normalizeReconcileUDID(input.UDID)]
		var enabled []asc.Resource[asc.DeviceAttributes]
		var disabled []asc.Resource[asc.DeviceAttributes]
		for _, match := range matches {
			if match.Attributes.Status == asc.DeviceStatusEnabled {
				enabled = append(enabled, match)
			} else {
				disabled = append(disabled, match)
			}
		}
		sort.Slice(enabled, func(i, j int) bool { return enabled[i].ID < enabled[j].ID })
		switch {
		case len(enabled) == 1:
			item.ResourceID = enabled[0].ID
			item.Status = string(enabled[0].Attributes.Status)
		case len(enabled) > 1:
			blockers = append(blockers, fmt.Sprintf("device %s resolves to multiple enabled resources", input.Fingerprint))
		case len(disabled) > 0:
			item.ResourceID = disabled[0].ID
			item.Status = string(disabled[0].Attributes.Status)
			blockers = append(blockers, fmt.Sprintf("device %s is registered but disabled; this additive command will not re-enable it", input.Fingerprint))
		default:
			actions = append(actions, signingAction{
				ID: "device:" + input.Fingerprint, Kind: actionRegisterDevice,
				DeviceFingerprint: input.Fingerprint, Platform: input.Platform,
			})
		}
		result = append(result, item)
	}
	return result, actions, blockers
}

func selectReconcileCertificate(certificates []asc.Resource[asc.CertificateAttributes], explicitID string, now time.Time, minimumValidityDays int) (*signingCertificateRef, []string) {
	return selectReconcileCertificateWithFingerprint(certificates, explicitID, "", now, minimumValidityDays)
}

func selectReconcileCertificateWithFingerprint(certificates []asc.Resource[asc.CertificateAttributes], explicitID, explicitSHA256 string, now time.Time, minimumValidityDays int) (*signingCertificateRef, []string) {
	minimumExpiration := now.Add(time.Duration(minimumValidityDays) * 24 * time.Hour)
	var eligible []asc.Resource[asc.CertificateAttributes]
	for _, certificate := range certificates {
		certificateType := strings.ToUpper(strings.TrimSpace(certificate.Attributes.CertificateType))
		if certificateType != "IOS_DISTRIBUTION" && certificateType != "DISTRIBUTION" {
			continue
		}
		// Live certificate list responses omit activated for valid signing
		// certificates. Reject only an explicit false, then bind and validate the
		// actual X.509 certificate content and validity below.
		if certificate.Attributes.Activated != nil && !*certificate.Attributes.Activated {
			continue
		}
		expires, err := time.Parse(time.RFC3339, strings.TrimSpace(certificate.Attributes.ExpirationDate))
		if err != nil || !expires.After(minimumExpiration) {
			continue
		}
		eligible = append(eligible, certificate)
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].ID < eligible[j].ID })
	explicitID = strings.TrimSpace(explicitID)
	explicitSHA256 = strings.ToLower(strings.TrimSpace(explicitSHA256))
	if explicitSHA256 != "" {
		decoded, err := hex.DecodeString(explicitSHA256)
		if err != nil || len(decoded) != sha256.Size {
			return nil, []string{"certificate SHA-256 must be exactly 64 hexadecimal characters"}
		}
		var matches []*signingCertificateRef
		for _, certificate := range eligible {
			candidate, err := certificatePlanRef(certificate)
			if err != nil {
				continue
			}
			if candidate.SHA256 == explicitSHA256 {
				matches = append(matches, candidate)
			}
		}
		if len(matches) == 0 {
			return nil, []string{"no eligible iOS distribution certificate matches the requested SHA-256"}
		}
		if len(matches) > 1 {
			return nil, []string{shared.AmbiguousError("eligible iOS distribution certificate", "--certificate", explicitSHA256, signingCertificateRefCandidates(matches)).Error()}
		}
		if explicitID != "" && matches[0].ID != explicitID {
			return nil, []string{fmt.Sprintf("certificate %s does not match the requested SHA-256", explicitID)}
		}
		return matches[0], nil
	}
	if explicitID != "" {
		for _, certificate := range eligible {
			if certificate.ID == explicitID {
				selected, err := certificatePlanRef(certificate)
				if err != nil {
					return nil, []string{fmt.Sprintf("certificate %s content is missing or invalid: %v", explicitID, err)}
				}
				return selected, nil
			}
		}
		return nil, []string{fmt.Sprintf("certificate %s is missing, inactive, expired, or not an iOS distribution certificate", explicitID)}
	}
	if len(eligible) == 0 {
		return nil, []string{"no active unexpired iOS distribution certificate is available"}
	}
	if len(eligible) > 1 {
		return nil, []string{(&shared.AmbiguousSelectionError{
			Kind:        "eligible iOS distribution certificate",
			Description: "the signing request",
			Flag:        "--certificate",
			Candidates:  signingCertificateCandidates(eligible),
		}).Error()}
	}
	selected, err := certificatePlanRef(eligible[0])
	if err != nil {
		return nil, []string{fmt.Sprintf("certificate %s content is missing or invalid: %v", eligible[0].ID, err)}
	}
	return selected, nil
}

func certificatePlanRef(certificate asc.Resource[asc.CertificateAttributes]) (*signingCertificateRef, error) {
	der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(certificate.Attributes.CertificateContent))
	if err != nil || len(der) == 0 {
		return nil, fmt.Errorf("decode certificateContent")
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse certificateContent DER: %w", err)
	}
	apiExpiration, err := time.Parse(time.RFC3339, strings.TrimSpace(certificate.Attributes.ExpirationDate))
	if err != nil || !parsed.NotAfter.After(time.Now()) || parsed.NotAfter.UTC().Truncate(time.Second) != apiExpiration.UTC().Truncate(time.Second) {
		return nil, fmt.Errorf("certificateContent validity differs from API expiration")
	}
	digest := sha256.Sum256(parsed.Raw)
	teamIDs := uniqueSortedStrings(parsed.Subject.OrganizationalUnit)
	if len(teamIDs) != 1 {
		return nil, fmt.Errorf("certificate subject must contain exactly one team organizational unit")
	}
	return &signingCertificateRef{
		ID: certificate.ID, CertificateType: certificate.Attributes.CertificateType,
		SerialNumber: certificate.Attributes.SerialNumber, ExpirationDate: certificate.Attributes.ExpirationDate,
		SHA256: hex.EncodeToString(digest[:]),
		TeamID: teamIDs[0],
	}, nil
}

func planSigningTarget(ctx context.Context, client *asc.Client, target signingTarget, devices []signingDesiredDevice, certificate *signingCertificateRef, minimumValidityDays int) (signingObservedBundle, []signingAction, []string, error) {
	observed := signingObservedBundle{BundleID: target.BundleID}
	bundle, err := findExactReconcileBundleID(ctx, client, target.BundleID)
	if err != nil {
		return observed, nil, nil, err
	}
	var actions []signingAction
	var blockers []string
	canCreateProfile := true
	requiredCapabilities, unverifiedEntitlements := signingCapabilitiesForEntitlements(target.Entitlements)
	if bundle == nil {
		if target.AppIDPrefix != "" && target.AppIDPrefix != plistString(target.Entitlements["com.apple.developer.team-identifier"]) {
			blockers = append(blockers, fmt.Sprintf("bundle ID %s is missing and target uses legacy App ID prefix %s; create it explicitly with the correct seed ID", target.BundleID, target.AppIDPrefix))
		} else if len(requiredCapabilities) > 0 || len(unverifiedEntitlements) > 0 {
			details := append([]string(nil), requiredCapabilities...)
			details = append(details, unverifiedEntitlements...)
			blockers = append(blockers, fmt.Sprintf("bundle ID %s is missing and target requires non-baseline entitlements: %s", target.BundleID, strings.Join(details, ",")))
		} else {
			actions = append(actions, signingAction{
				ID: "bundle:" + target.BundleID, Kind: actionCreateBundleID,
				BundleID: target.BundleID, Platform: string(asc.BundleIDPlatformIOS),
			})
		}
	} else {
		observed.ResourceID = bundle.ID
		observed.Platform = string(bundle.Attributes.Platform)
		if seedErr := validateReconcileBundleSeed(*bundle, target); seedErr != nil {
			blockers = append(blockers, seedErr.Error())
			canCreateProfile = false
		}
		if bundle.Attributes.Platform != asc.BundleIDPlatformIOS && bundle.Attributes.Platform != asc.BundleIDPlatformUniversal {
			blockers = append(blockers, fmt.Sprintf("bundle ID %s has incompatible platform %s", target.BundleID, bundle.Attributes.Platform))
		}
		capabilities, capabilityErr := getAllBundleIDCapabilities(ctx, client, bundle.ID)
		if capabilityErr != nil {
			return observed, nil, nil, fmt.Errorf("list bundle ID capabilities: %w", capabilityErr)
		}
		for _, capability := range capabilities {
			observed.EnabledCapabilities = append(observed.EnabledCapabilities, strings.ToUpper(strings.TrimSpace(capability.Attributes.CapabilityType)))
		}
		observed.EnabledCapabilities = uniqueSortedStrings(observed.EnabledCapabilities)
		for _, required := range requiredCapabilities {
			if !containsAllStrings(observed.EnabledCapabilities, []string{required}) {
				blockers = append(blockers, fmt.Sprintf("bundle ID %s is missing required capability %s; this command will not enable it", target.BundleID, required))
			}
		}
		if len(unverifiedEntitlements) > 0 {
			blockers = append(blockers, fmt.Sprintf("bundle ID %s uses entitlements whose capability state cannot be verified safely: %s", target.BundleID, strings.Join(unverifiedEntitlements, ",")))
		}
	}

	if certificate == nil {
		return observed, actions, blockers, nil
	}
	var selected *profileCandidate
	if bundle != nil {
		candidates, err := getProfileCandidates(ctx, client, *bundle, target, devices, *certificate, minimumValidityDays)
		if err != nil {
			return observed, nil, nil, err
		}
		for _, candidate := range candidates {
			observed.Profiles = append(observed.Profiles, signingObservedProfile{
				ID: candidate.Profile.ID, State: string(candidate.Profile.Attributes.ProfileState),
				ExpirationDate: candidate.Profile.Attributes.ExpirationDate,
				CertificateIDs: append([]string(nil), candidate.CertificateIDs...),
				DeviceCount:    len(candidate.DeviceIDs), DeviceSetHash: hashSortedStrings(candidate.DeviceIDs), Suitable: candidate.Suitable,
			})
		}
		sort.Slice(observed.Profiles, func(i, j int) bool { return observed.Profiles[i].ID < observed.Profiles[j].ID })
		if len(candidates) > 0 && candidates[0].Suitable {
			selected = &candidates[0]
			observed.SelectedProfileID = selected.Profile.ID
		}
	}

	if selected != nil {
		output := profileOutputRelativePath(selected.Profile.Attributes.UUID)
		if output == "" {
			blockers = append(blockers, fmt.Sprintf("profile %s is suitable but has no safe UUID", selected.Profile.ID))
		} else {
			actions = append(actions, signingAction{
				ID: "download:" + target.BundleID, Kind: actionDownloadProfile,
				BundleID: target.BundleID, ResourceID: observed.ResourceID, CertificateID: certificate.ID,
				ProfileID: selected.Profile.ID, OutputRelativePath: output,
			})
		}
		return observed, actions, blockers, nil
	}

	if !canCreateProfile {
		return observed, actions, blockers, nil
	}
	actions = append(actions, signingAction{
		ID: "profile:" + target.BundleID, Kind: actionCreateProfile,
		BundleID: target.BundleID, ResourceID: observed.ResourceID,
		CertificateID: certificate.ID, DeviceResourceIDs: resolvedDeviceIDs(devices),
		ProfileName: deterministicProfileName(target.BundleID, certificate.ID, devices),
	})
	return observed, actions, blockers, nil
}

func validateReconcileBundleSeed(bundle asc.Resource[asc.BundleIDAttributes], target signingTarget) error {
	targetPrefix := strings.TrimSpace(target.AppIDPrefix)
	bundleSeedID := strings.TrimSpace(bundle.Attributes.SeedID)
	if targetPrefix == "" || bundleSeedID == targetPrefix {
		return nil
	}
	displaySeedID := bundleSeedID
	if displaySeedID == "" {
		displaySeedID = "<missing>"
	}
	return fmt.Errorf("bundle ID %s has seed ID %s, but target requires App ID prefix %s; refusing profile creation", target.BundleID, displaySeedID, targetPrefix)
}

func hashSortedStrings(values []string) string {
	values = append([]string(nil), values...)
	sort.Strings(values)
	digest := sha256.Sum256([]byte(strings.Join(values, "\n")))
	return hex.EncodeToString(digest[:])
}

func getAllBundleIDCapabilities(ctx context.Context, client *asc.Client, bundleID string) ([]asc.Resource[asc.BundleIDCapabilityAttributes], error) {
	var result []asc.Resource[asc.BundleIDCapabilityAttributes]
	next := ""
	for {
		page, err := client.GetBundleIDCapabilities(ctx, bundleID, asc.WithBundleIDCapabilitiesNextURL(next))
		if err != nil {
			return nil, err
		}
		result = append(result, page.Data...)
		if strings.TrimSpace(page.Links.Next) == "" {
			return result, nil
		}
		next = page.Links.Next
	}
}

func findExactReconcileBundleID(ctx context.Context, client *asc.Client, identifier string) (*asc.Resource[asc.BundleIDAttributes], error) {
	next := ""
	var exact []asc.Resource[asc.BundleIDAttributes]
	for {
		page, err := client.GetBundleIDs(ctx, asc.WithBundleIDsFilterIdentifier(identifier), asc.WithBundleIDsLimit(200), asc.WithBundleIDsNextURL(next))
		if err != nil {
			return nil, err
		}
		for _, bundle := range page.Data {
			if bundle.Attributes.Identifier == identifier {
				exact = append(exact, bundle)
			}
		}
		if strings.TrimSpace(page.Links.Next) == "" {
			break
		}
		next = page.Links.Next
	}
	if len(exact) > 1 {
		return nil, fmt.Errorf("bundle identifier resolves to multiple resources")
	}
	if len(exact) == 0 {
		return nil, nil
	}
	return &exact[0], nil
}

type profileCandidate struct {
	Profile        asc.Resource[asc.ProfileAttributes]
	CertificateIDs []string
	DeviceIDs      []string
	ExtraDevices   int
	Suitable       bool
}

func getProfileCandidates(ctx context.Context, client *asc.Client, bundle asc.Resource[asc.BundleIDAttributes], target signingTarget, desired []signingDesiredDevice, certificate signingCertificateRef, minimumValidityDays int) ([]profileCandidate, error) {
	profiles, err := getAllBundleProfiles(ctx, client, bundle.ID)
	if err != nil {
		return nil, err
	}
	desiredIDs := resolvedDeviceIDs(desired)
	minimumExpiration := time.Now().Add(time.Duration(minimumValidityDays) * 24 * time.Hour)
	var candidates []profileCandidate
	for _, profile := range profiles {
		if profile.Attributes.ProfileType != reconcileProfileType || profile.Attributes.ProfileState != asc.ProfileStateActive {
			continue
		}
		candidate := profileCandidate{Profile: profile}
		candidate.CertificateIDs, err = getAllProfileCertificateIDs(ctx, client, profile.ID)
		if err != nil {
			return nil, err
		}
		candidate.DeviceIDs, err = getAllProfileDeviceIDs(ctx, client, profile.ID)
		if err != nil {
			return nil, err
		}
		candidate.ExtraDevices = countExtraIDs(candidate.DeviceIDs, desiredIDs)
		expires, expirationErr := time.Parse(time.RFC3339, strings.TrimSpace(profile.Attributes.ExpirationDate))
		certificateMatches := len(candidate.CertificateIDs) == 1 && candidate.CertificateIDs[0] == certificate.ID
		devicesMatch := sameStringSet(candidate.DeviceIDs, desiredIDs) && len(desiredIDs) == len(desired)
		entitlementsMatch := false
		if expirationErr == nil && expires.After(minimumExpiration) && certificateMatches && devicesMatch {
			fullProfile, getErr := client.GetProfile(ctx, profile.ID)
			if getErr != nil {
				return nil, getErr
			}
			candidate.Profile = fullProfile.Data
			entitlementsMatch = profileContentMatchesTarget(fullProfile.Data.Attributes.ProfileContent, target, desired, certificate.SHA256, minimumExpiration)
		}
		candidate.Suitable = expirationErr == nil && expires.After(minimumExpiration) && certificateMatches && devicesMatch && entitlementsMatch
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Suitable != candidates[j].Suitable {
			return candidates[i].Suitable
		}
		if candidates[i].ExtraDevices != candidates[j].ExtraDevices {
			return candidates[i].ExtraDevices < candidates[j].ExtraDevices
		}
		left, _ := time.Parse(time.RFC3339, candidates[i].Profile.Attributes.ExpirationDate)
		right, _ := time.Parse(time.RFC3339, candidates[j].Profile.Attributes.ExpirationDate)
		if !left.Equal(right) {
			return left.After(right)
		}
		return candidates[i].Profile.ID < candidates[j].Profile.ID
	})
	return candidates, nil
}

func getAllBundleProfiles(ctx context.Context, client *asc.Client, bundleID string) ([]asc.Resource[asc.ProfileAttributes], error) {
	var result []asc.Resource[asc.ProfileAttributes]
	next := ""
	for {
		page, err := client.GetBundleIDProfiles(ctx, bundleID, asc.WithBundleIDProfilesLimit(200), asc.WithBundleIDProfilesNextURL(next))
		if err != nil {
			return nil, err
		}
		result = append(result, page.Data...)
		if strings.TrimSpace(page.Links.Next) == "" {
			break
		}
		next = page.Links.Next
	}
	return result, nil
}

func getAllProfileCertificateIDs(ctx context.Context, client *asc.Client, profileID string) ([]string, error) {
	var result []string
	next := ""
	for {
		page, err := client.GetProfileCertificates(ctx, profileID, asc.WithProfileCertificatesLimit(200), asc.WithProfileCertificatesNextURL(next))
		if err != nil {
			return nil, err
		}
		for _, item := range page.Data {
			result = append(result, item.ID)
		}
		if strings.TrimSpace(page.Links.Next) == "" {
			break
		}
		next = page.Links.Next
	}
	sort.Strings(result)
	return result, nil
}

func getAllProfileDeviceIDs(ctx context.Context, client *asc.Client, profileID string) ([]string, error) {
	var result []string
	next := ""
	for {
		page, err := client.GetProfileDevices(ctx, profileID, asc.WithProfileDevicesLimit(200), asc.WithProfileDevicesNextURL(next))
		if err != nil {
			return nil, err
		}
		for _, item := range page.Data {
			result = append(result, item.ID)
		}
		if strings.TrimSpace(page.Links.Next) == "" {
			break
		}
		next = page.Links.Next
	}
	sort.Strings(result)
	return result, nil
}

func profileContentMatchesTarget(content string, target signingTarget, desired []signingDesiredDevice, certificateSHA256 string, minimumExpiration time.Time) bool {
	if len(strings.TrimSpace(content)) > base64.StdEncoding.EncodedLen(reconcileProfileMaxBytes) {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(content))
	if err != nil {
		return false
	}
	profile, err := decodeReconcileMobileProvision(decoded)
	if err != nil {
		return false
	}
	if !entitlementsContain(profile.Entitlements, target.Entitlements) {
		return false
	}
	if !profile.ExpirationDate.After(minimumExpiration) {
		return false
	}
	if !mobileProvisionContainsCertificate(profile, certificateSHA256) {
		return false
	}
	provisioned := make(map[string]struct{}, len(profile.ProvisionedDevices))
	for _, udid := range profile.ProvisionedDevices {
		provisioned[fingerprintDevice(normalizeReconcileUDID(udid))] = struct{}{}
	}
	if len(provisioned) != len(desired) {
		return false
	}
	for _, device := range desired {
		if _, ok := provisioned[device.Fingerprint]; !ok {
			return false
		}
	}
	return true
}

type reconcileMobileProvision struct {
	UUID                  string         `plist:"UUID"`
	ExpirationDate        time.Time      `plist:"ExpirationDate"`
	ProvisionedDevices    []string       `plist:"ProvisionedDevices"`
	Entitlements          map[string]any `plist:"Entitlements"`
	DeveloperCertificates [][]byte       `plist:"DeveloperCertificates"`
}

func decodeReconcileMobileProvision(data []byte) (reconcileMobileProvision, error) {
	p7, err := pkcs7.Parse(data)
	if err != nil {
		return reconcileMobileProvision{}, fmt.Errorf("parse CMS: %w", err)
	}
	if err := p7.Verify(); err != nil {
		return reconcileMobileProvision{}, fmt.Errorf("verify CMS signature: %w", err)
	}
	if len(p7.Content) == 0 {
		return reconcileMobileProvision{}, fmt.Errorf("CMS content is empty")
	}
	data = p7.Content
	var result reconcileMobileProvision
	if _, err := plist.Unmarshal(data, &result); err != nil {
		return result, err
	}
	return result, nil
}

func mobileProvisionContainsCertificate(profile reconcileMobileProvision, wantedSHA256 string) bool {
	wantedSHA256 = strings.ToLower(strings.TrimSpace(wantedSHA256))
	if wantedSHA256 == "" {
		return false
	}
	for _, der := range profile.DeveloperCertificates {
		certificate, err := x509.ParseCertificate(der)
		if err != nil {
			continue
		}
		digest := sha256.Sum256(certificate.Raw)
		if hex.EncodeToString(digest[:]) == wantedSHA256 {
			return true
		}
	}
	return false
}

func entitlementsContain(profile, target map[string]any) bool {
	for key, wanted := range target {
		actual, ok := profile[key]
		if !ok || !entitlementValueContains(actual, wanted) {
			return false
		}
	}
	return true
}

func entitlementValueContains(actual, wanted any) bool {
	if reflect.DeepEqual(actual, wanted) {
		return true
	}
	actualString, actualOK := actual.(string)
	wantedString, wantedOK := wanted.(string)
	if actualOK && wantedOK {
		if strings.HasSuffix(actualString, "*") {
			return strings.HasPrefix(wantedString, strings.TrimSuffix(actualString, "*"))
		}
		return actualString == wantedString
	}
	actualSlice := anySlice(actual)
	wantedSlice := anySlice(wanted)
	if actualSlice != nil && wantedSlice != nil {
		for _, wantedItem := range wantedSlice {
			matched := false
			for _, actualItem := range actualSlice {
				if entitlementValueContains(actualItem, wantedItem) {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		}
		return true
	}
	actualMap, actualMapOK := actual.(map[string]any)
	wantedMap, wantedMapOK := wanted.(map[string]any)
	return actualMapOK && wantedMapOK && entitlementsContain(actualMap, wantedMap)
}

func anySlice(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []string:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = item
		}
		return result
	default:
		return nil
	}
}

func signingCapabilitiesForEntitlements(entitlements map[string]any) ([]string, []string) {
	baseline := map[string]struct{}{
		"application-identifier": {}, "com.apple.application-identifier": {},
		"com.apple.developer.team-identifier": {}, "keychain-access-groups": {},
		"get-task-allow": {}, "beta-reports-active": {},
	}
	capabilityByEntitlement := map[string]string{
		"aps-environment":                                   "PUSH_NOTIFICATIONS",
		"com.apple.developer.associated-domains":            "ASSOCIATED_DOMAINS",
		"com.apple.developer.healthkit":                     "HEALTHKIT",
		"com.apple.developer.homekit":                       "HOMEKIT",
		"com.apple.developer.siri":                          "SIRIKIT",
		"com.apple.developer.nfc.readersession.formats":     "NFC_TAG_READING",
		"com.apple.developer.default-data-protection":       "DATA_PROTECTION",
		"com.apple.developer.applesignin":                   "APPLE_ID_AUTH",
		"com.apple.developer.networking.wifi-info":          "ACCESS_WIFI_INFORMATION",
		"com.apple.developer.networking.multipath":          "MULTIPATH",
		"com.apple.developer.kernel.increased-memory-limit": "INCREASED_MEMORY_LIMIT",
	}
	settingConstrainedEntitlements := map[string]struct{}{
		"com.apple.security.application-groups":           {},
		"com.apple.developer.in-app-payments":             {},
		"com.apple.developer.networking.networkextension": {},
		"com.apple.developer.pass-type-identifiers":       {},
	}
	var capabilities []string
	var unverified []string
	for key, value := range entitlements {
		if key == "get-task-allow" {
			allowed, ok := value.(bool)
			if !ok || allowed {
				unverified = append(unverified, key+" (must be false for ad hoc distribution)")
			}
			continue
		}
		if key == "beta-reports-active" {
			active, ok := value.(bool)
			if !ok || active {
				unverified = append(unverified, key+" (must be false for ad hoc distribution)")
			}
			continue
		}
		if key == "aps-environment" {
			environment, ok := value.(string)
			if !ok || environment != "production" {
				unverified = append(unverified, key+" (must be production for ad hoc distribution)")
			} else {
				capabilities = append(capabilities, "PUSH_NOTIFICATIONS")
			}
			continue
		}
		if _, ok := baseline[key]; ok {
			continue
		}
		_, hasEntitlementSpecificSettings := settingConstrainedEntitlements[key]
		if hasEntitlementSpecificSettings || strings.HasPrefix(key, "com.apple.developer.icloud-") || key == "com.apple.developer.ubiquity-container-identifiers" || key == "com.apple.developer.ubiquity-kvstore-identifier" || key == "com.apple.developer.default-data-protection" || key == "com.apple.developer.applesignin" {
			unverified = append(unverified, key+" (capability settings)")
			continue
		}
		if capability := capabilityByEntitlement[key]; capability != "" {
			capabilities = append(capabilities, capability)
		} else {
			unverified = append(unverified, key)
		}
	}
	return uniqueSortedStrings(capabilities), uniqueSortedStrings(unverified)
}

func resolvedDeviceIDs(devices []signingDesiredDevice) []string {
	var result []string
	for _, device := range devices {
		if device.ResourceID != "" {
			result = append(result, device.ResourceID)
		}
	}
	sort.Strings(result)
	return result
}

func deterministicProfileName(bundleID, certificateID string, devices []signingDesiredDevice) string {
	parts := []string{bundleID, certificateID}
	for _, device := range devices {
		parts = append(parts, device.Fingerprint)
	}
	sort.Strings(parts[2:])
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return fmt.Sprintf("ASC Ad Hoc %s %s", bundleID, hex.EncodeToString(sum[:])[:profileNameFingerprintLength])
}

func profileOutputRelativePath(uuid string) string {
	uuid = strings.TrimSpace(uuid)
	if !safeProfileUUID(uuid) {
		return ""
	}
	return filepath.ToSlash(filepath.Join("profiles", uuid+".mobileprovision"))
}

func safeProfileUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, char := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if char != '-' {
				return false
			}
			continue
		}
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return false
		}
	}
	return true
}

func containsAllStrings(have, wanted []string) bool {
	set := make(map[string]struct{}, len(have))
	for _, item := range have {
		set[item] = struct{}{}
	}
	for _, item := range wanted {
		if _, ok := set[item]; !ok {
			return false
		}
	}
	return true
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	return containsAllStrings(left, right) && containsAllStrings(right, left)
}

func countExtraIDs(have, wanted []string) int {
	set := make(map[string]struct{}, len(wanted))
	for _, item := range wanted {
		set[item] = struct{}{}
	}
	count := 0
	for _, item := range have {
		if _, ok := set[item]; !ok {
			count++
		}
	}
	return count
}

func uniqueSortedStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func signingCertificateCandidates(certificates []asc.Resource[asc.CertificateAttributes]) []shared.AmbiguousCandidate {
	candidates := make([]shared.AmbiguousCandidate, 0, len(certificates))
	for _, certificate := range certificates {
		candidates = append(candidates, shared.AmbiguousCandidate{
			ID:    strings.TrimSpace(certificate.ID),
			Label: strings.TrimSpace(certificate.Attributes.DisplayName),
			Extra: strings.TrimSpace("expires " + strings.TrimSpace(certificate.Attributes.ExpirationDate)),
		})
	}
	return candidates
}

func signingCertificateRefCandidates(refs []*signingCertificateRef) []shared.AmbiguousCandidate {
	candidates := make([]shared.AmbiguousCandidate, 0, len(refs))
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		candidates = append(candidates, shared.AmbiguousCandidate{
			ID:    strings.TrimSpace(ref.ID),
			Label: strings.TrimSpace(ref.SHA256),
			Extra: strings.TrimSpace("expires " + strings.TrimSpace(ref.ExpirationDate)),
		})
	}
	return candidates
}
