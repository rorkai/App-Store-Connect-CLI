package signing

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const deviceWithoutCreateMissingDeprecationWarning = "Warning: --device without --create-missing is deprecated and ignored because device IDs are only applied when creating a profile. Add --create-missing so they can be applied if a profile must be created. This combination will be rejected in 5.0.0."

func warnDeviceWithoutCreateMissing(deviceIDs string, createMissing bool) {
	if !createMissing && strings.TrimSpace(deviceIDs) != "" {
		fmt.Fprintln(os.Stderr, deviceWithoutCreateMissingDeprecationWarning)
	}
}

// SigningFetchCommand returns the signing fetch subcommand.
func SigningFetchCommand() *ffcli.Command {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (optional, or ASC_APP_ID env)")
	bundleID := fs.String("bundle-id", "", "Bundle identifier (e.g., com.example.app) - required")
	profileType := fs.String("profile-type", "", "Profile type: IOS_APP_STORE, IOS_APP_DEVELOPMENT, MAC_APP_STORE, etc. (required)")
	deviceIDs := fs.String("device", "", "Device ID(s), comma-separated (required with --create-missing for development profiles; deprecated and ignored without it until 5.0.0)")
	certType := fs.String("certificate-type", "", "Certificate type filter (optional)")
	outputPath := fs.String("output", "./signing", "Output directory for signing files")
	createMissing := fs.Bool("create-missing", false, "Create missing profiles")
	output := shared.BindOutputFlagsWith(fs, "format", shared.DefaultOutputFormat(), "Output format for metadata: json, table, markdown")

	return &ffcli.Command{
		Name:       "fetch",
		ShortUsage: "asc signing fetch [flags]",
		ShortHelp:  "Fetch signing files (certificates + profiles) for an app.",
		LongHelp: `Fetch signing certificates and provisioning profiles for an app.

This command resolves the bundle ID, finds matching certificates and profiles,
and writes them to the output directory.

With --create-missing, it will create a new profile if none exist for the
specified configuration. Devices are only applied to profiles this command
creates. In 4.x, passing --device without --create-missing prints a deprecation
warning and ignores the device IDs; 5.0.0 will reject that combination.

Examples:
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_STORE --output ./signing
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_DEVELOPMENT --create-missing --device "DEVICE1,DEVICE2"
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_STORE --create-missing`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			bundle := strings.TrimSpace(*bundleID)
			if bundle == "" {
				fmt.Fprintln(os.Stderr, "Error: --bundle-id is required")
				return shared.MissingRequiredUsageError()
			}

			profType := strings.TrimSpace(*profileType)
			if profType == "" {
				fmt.Fprintln(os.Stderr, "Error: --profile-type is required")
				return shared.MissingRequiredUsageError()
			}
			profType = strings.ToUpper(profType)
			warnDeviceWithoutCreateMissing(*deviceIDs, *createMissing)
			if *createMissing && isDevelopmentProfile(profType) && strings.TrimSpace(*deviceIDs) == "" {
				fmt.Fprintln(os.Stderr, "Error: --device is required for development profiles")
				return shared.MissingRequiredUsageError()
			}

			outputDir := strings.TrimSpace(*outputPath)
			if outputDir == "" {
				outputDir = "./signing"
			}
			prepareOutputDir := onceAfterSuccess(func() error {
				if err := os.MkdirAll(outputDir, 0o755); err != nil {
					return fmt.Errorf("create output dir: %w", err)
				}
				return nil
			})
			// signing fetch never overwrites, so every colliding output file has
			// to be detected before the profile is created in App Store Connect.
			// Otherwise a failed write leaves a stray profile in the account.
			preflightOutput := func(profileName, profileID string, certificates []asc.Resource[asc.CertificateAttributes]) error {
				if err := prepareOutputDir(); err != nil {
					return err
				}
				return ensureOutputPathsAreFree(signingOutputPaths(outputDir, profileName, profileID, certificates))
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("signing fetch: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID != "" {
				if err := validateBundleIDMatchesApp(requestCtx, client, resolvedAppID, bundle); err != nil {
					return fmt.Errorf("signing fetch: %w", err)
				}
			}

			result := &asc.SigningFetchResult{
				BundleID:    bundle,
				ProfileType: profType,
				OutputPath:  outputDir,
			}

			bundleIDResp, err := findBundleID(requestCtx, client, bundle)
			if err != nil {
				return fmt.Errorf("signing fetch: %w", err)
			}
			result.BundleIDResource = bundleIDResp.Data.ID

			profile, certs, created, err := resolveSigningAssets(
				requestCtx,
				client,
				signingAssetsOptions{
					BundleIDResourceID: bundleIDResp.Data.ID,
					BundleIdentifier:   bundle,
					ProfileType:        profType,
					CertificateType:    *certType,
					DeviceIDs:          shared.SplitCSV(*deviceIDs),
					CreateMissing:      *createMissing,
					BeforeCreate: func(plan profileCreatePlan) error {
						return preflightOutput(plan.ProfileName, "", plan.Certificates)
					},
				},
			)
			if err != nil {
				return fmt.Errorf("signing fetch: %w", err)
			}
			result.CertificateIDs = extractIDs(certs.Data)
			result.ProfileID = profile.Data.ID
			result.Created = created

			if err := preflightOutput(profile.Data.Attributes.Name, profile.Data.ID, certs.Data); err != nil {
				return fmt.Errorf("signing fetch: %w", err)
			}

			profilePath := profileOutputPath(outputDir, profile.Data.Attributes.Name, profile.Data.ID)
			profileContent, err := decodeBase64Content("profile", profile.Data.Attributes.ProfileContent)
			if err != nil {
				return fmt.Errorf("signing fetch: decode profile: %w", err)
			}
			if err := shared.WriteProfileFile(profilePath, profileContent); err != nil {
				return fmt.Errorf("signing fetch: write profile: %w", err)
			}
			result.ProfileFile = profilePath

			for _, cert := range certs.Data {
				certPath := certificateOutputPath(outputDir, cert)
				certContent, err := decodeBase64Content("certificate", cert.Attributes.CertificateContent)
				if err != nil {
					return fmt.Errorf("signing fetch: decode certificate: %w", err)
				}
				if err := writeBinaryFile(certPath, certContent); err != nil {
					return fmt.Errorf("signing fetch: write certificate: %w", err)
				}
				result.CertificateFiles = append(result.CertificateFiles, certPath)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func validateBundleIDMatchesApp(ctx context.Context, client *asc.Client, appID, bundleID string) error {
	app, err := client.GetApp(ctx, appID)
	if err != nil {
		return fmt.Errorf("fetch app: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(app.Data.Attributes.BundleID), strings.TrimSpace(bundleID)) {
		return fmt.Errorf("bundle ID %s does not match app %s (expected %s)", bundleID, appID, app.Data.Attributes.BundleID)
	}
	return nil
}

func findBundleID(ctx context.Context, client *asc.Client, identifier string) (*asc.BundleIDResponse, error) {
	resp, err := client.GetBundleIDs(ctx, asc.WithBundleIDsFilterIdentifier(identifier))
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("bundle ID not found: %s", identifier)
	}
	return &asc.BundleIDResponse{Data: resp.Data[0]}, nil
}

func findCertificates(ctx context.Context, client *asc.Client, profileType, certType string) (*asc.CertificatesResponse, error) {
	certType = strings.TrimSpace(certType)
	if certType == "" {
		inferred, err := inferCertificateType(profileType)
		if err != nil {
			return nil, err
		}
		certType = inferred
	}

	var (
		all   []asc.Resource[asc.CertificateAttributes]
		links asc.Links
		next  string
	)
	for {
		resp, err := client.GetCertificates(
			ctx,
			asc.WithCertificatesFilterType(certType),
			asc.WithCertificatesNextURL(next),
		)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Data...)
		links = resp.Links
		if strings.TrimSpace(resp.Links.Next) == "" {
			break
		}
		next = resp.Links.Next
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no certificates found for type %s", certType)
	}
	return &asc.CertificatesResponse{Data: all, Links: links}, nil
}

type signingAssetsOptions struct {
	BundleIDResourceID string
	BundleIdentifier   string
	ProfileType        string
	CertificateType    string
	DeviceIDs          []string
	CreateMissing      bool
	BeforeCreate       func(profileCreatePlan) error
	CreateContext      func() (context.Context, context.CancelFunc)
	CertificateFilter  func(asc.Resource[asc.CertificateAttributes]) bool
}

// profileCreatePlan describes the profile that is about to be created so callers
// can fail before App Store Connect is mutated.
type profileCreatePlan struct {
	ProfileName  string
	Certificates []asc.Resource[asc.CertificateAttributes]
}

var errNoMatchingProfileCertificates = errors.New("profile has no matching associated certificates")

var supportedSigningCertificateTypes = map[string]struct{}{
	"APPLE_PAY":                   {},
	"APPLE_PAY_MERCHANT_IDENTITY": {},
	"APPLE_PAY_PSP_IDENTITY":      {},
	"APPLE_PAY_RSA":               {},
	"DEVELOPER_ID_KEXT":           {},
	"DEVELOPER_ID_KEXT_G2":        {},
	"DEVELOPER_ID_APPLICATION":    {},
	"DEVELOPER_ID_APPLICATION_G2": {},
	"DEVELOPMENT":                 {},
	"DISTRIBUTION":                {},
	"IDENTITY_ACCESS":             {},
	"IOS_DEVELOPMENT":             {},
	"IOS_DISTRIBUTION":            {},
	"MAC_APP_DISTRIBUTION":        {},
	"MAC_INSTALLER_DISTRIBUTION":  {},
	"MAC_APP_DEVELOPMENT":         {},
	"PASS_TYPE_ID":                {},
	"PASS_TYPE_ID_WITH_NFC":       {},
}

func resolveSigningAssets(ctx context.Context, client *asc.Client, options signingAssetsOptions) (*asc.ProfileResponse, *asc.CertificatesResponse, bool, error) {
	certificateType, err := resolveSigningCertificateTypes(options.ProfileType, options.CertificateType)
	if err != nil {
		return nil, nil, false, err
	}

	profiles, err := findActiveProfiles(ctx, client, options.BundleIDResourceID, options.ProfileType)
	if err != nil {
		return nil, nil, false, err
	}
	var certificateMatchErr error
	for _, profileResource := range profiles {
		profile := &asc.ProfileResponse{Data: profileResource}
		certificates, err := findProfileCertificates(ctx, client, profile.Data.ID, certificateType)
		if err == nil {
			certificates.Data = filterSigningCertificates(certificates.Data, options.CertificateFilter)
			if len(certificates.Data) == 0 {
				certificateMatchErr = fmt.Errorf("profile %s has no associated certificate matching the local signing identity: %w", profile.Data.ID, errNoMatchingProfileCertificates)
				continue
			}
			return profile, certificates, false, nil
		}
		if !errors.Is(err, errNoMatchingProfileCertificates) {
			return nil, nil, false, err
		}
		certificateMatchErr = err
	}

	if !options.CreateMissing {
		if certificateMatchErr != nil {
			return nil, nil, false, certificateMatchErr
		}
		return nil, nil, false, fmt.Errorf(
			"no active %s profile found for bundle ID %s; use --create-missing to create one",
			options.ProfileType,
			options.BundleIdentifier,
		)
	}

	certificates, err := findCertificates(ctx, client, options.ProfileType, certificateType)
	if err != nil {
		return nil, nil, false, err
	}
	certificates.Data = filterSigningCertificates(certificates.Data, options.CertificateFilter)
	certificates.Data = certificatesForProfileCreation(certificates.Data, options.ProfileType, time.Now())
	if len(certificates.Data) == 0 {
		return nil, nil, false, fmt.Errorf(
			"no active, unexpired certificates available to create %s profile",
			options.ProfileType,
		)
	}
	profileName := profileCreateName(options.ProfileType, time.Now())
	if options.BeforeCreate != nil {
		plan := profileCreatePlan{ProfileName: profileName, Certificates: certificates.Data}
		if err := options.BeforeCreate(plan); err != nil {
			return nil, nil, false, fmt.Errorf("preflight before creating profile: %w", err)
		}
	}

	createCtx := ctx
	cancelCreate := func() {}
	if options.CreateContext != nil {
		createCtx, cancelCreate = options.CreateContext()
		if createCtx == nil {
			return nil, nil, false, fmt.Errorf("profile create context is nil")
		}
	}
	profile, err := createProfile(
		createCtx,
		client,
		options.BundleIDResourceID,
		profileName,
		options.ProfileType,
		extractIDs(certificates.Data),
		options.DeviceIDs,
	)
	cancelCreate()
	if err != nil {
		return nil, nil, false, err
	}
	return profile, certificates, true, nil
}

func filterSigningCertificates(certificates []asc.Resource[asc.CertificateAttributes], filter func(asc.Resource[asc.CertificateAttributes]) bool) []asc.Resource[asc.CertificateAttributes] {
	if filter == nil {
		return certificates
	}
	filtered := make([]asc.Resource[asc.CertificateAttributes], 0, len(certificates))
	for _, certificate := range certificates {
		if filter(certificate) {
			filtered = append(filtered, certificate)
		}
	}
	return filtered
}

func certificatesForProfileCreation(certificates []asc.Resource[asc.CertificateAttributes], profileType string, now time.Time) []asc.Resource[asc.CertificateAttributes] {
	type candidate struct {
		certificate asc.Resource[asc.CertificateAttributes]
		expiresAt   time.Time
	}

	candidates := make([]candidate, 0, len(certificates))
	for _, certificate := range certificates {
		activated := certificate.Attributes.Activated
		if activated != nil && !*activated {
			continue
		}
		expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(certificate.Attributes.ExpirationDate))
		if err != nil || !expiresAt.After(now) {
			continue
		}
		candidates = append(candidates, candidate{
			certificate: certificate,
			expiresAt:   expiresAt,
		})
	}

	if len(candidates) == 0 {
		return nil
	}
	if !isSingleCertificateProfile(profileType) {
		eligible := make([]asc.Resource[asc.CertificateAttributes], 0, len(candidates))
		for _, candidate := range candidates {
			eligible = append(eligible, candidate.certificate)
		}
		return eligible
	}

	selected := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.expiresAt.After(selected.expiresAt) ||
			(candidate.expiresAt.Equal(selected.expiresAt) && candidate.certificate.ID < selected.certificate.ID) {
			selected = candidate
		}
	}
	return []asc.Resource[asc.CertificateAttributes]{selected.certificate}
}

func isSingleCertificateProfile(profileType string) bool {
	switch strings.ToUpper(strings.TrimSpace(profileType)) {
	case "IOS_APP_STORE", "IOS_APP_ADHOC", "IOS_APP_INHOUSE",
		"TVOS_APP_STORE", "TVOS_APP_ADHOC", "TVOS_APP_INHOUSE",
		"MAC_APP_STORE", "MAC_CATALYST_APP_STORE":
		return true
	default:
		return false
	}
}

func resolveSigningCertificateTypes(profileType, raw string) (string, error) {
	certificateTypes := shared.SplitCSVUpper(raw)
	if len(certificateTypes) == 0 {
		inferred, err := inferCertificateType(profileType)
		if err != nil {
			return "", err
		}
		certificateTypes = shared.SplitCSVUpper(inferred)
	}

	for _, certificateType := range certificateTypes {
		if _, ok := supportedSigningCertificateTypes[certificateType]; !ok {
			return "", fmt.Errorf("unsupported certificate type %s", certificateType)
		}
	}
	return strings.Join(certificateTypes, ","), nil
}

func findActiveProfiles(ctx context.Context, client *asc.Client, bundleIDResourceID, profileType string) ([]asc.Resource[asc.ProfileAttributes], error) {
	var matches []asc.Resource[asc.ProfileAttributes]
	next := ""
	for {
		profiles, err := client.GetBundleIDProfiles(
			ctx,
			bundleIDResourceID,
			asc.WithBundleIDProfilesNextURL(next),
		)
		if err != nil {
			return nil, err
		}

		for _, profile := range profiles.Data {
			if profile.Attributes.ProfileState != asc.ProfileStateActive {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(profile.Attributes.ProfileType), profileType) {
				matches = append(matches, profile)
			}
		}

		if strings.TrimSpace(profiles.Links.Next) == "" {
			return matches, nil
		}
		next = profiles.Links.Next
	}
}

func findProfileCertificates(ctx context.Context, client *asc.Client, profileID, certificateType string) (*asc.CertificatesResponse, error) {
	var (
		all   []asc.Resource[asc.CertificateAttributes]
		links asc.Links
		next  string
	)
	for {
		response, err := client.GetProfileCertificates(
			ctx,
			profileID,
			asc.WithProfileCertificatesNextURL(next),
		)
		if err != nil {
			return nil, err
		}
		all = append(all, response.Data...)
		links = response.Links
		if strings.TrimSpace(response.Links.Next) == "" {
			break
		}
		next = response.Links.Next
	}

	requestedTypes := shared.SplitCSVUpper(certificateType)
	if len(requestedTypes) > 0 {
		requestedTypeSet := make(map[string]struct{}, len(requestedTypes))
		for _, requestedType := range requestedTypes {
			requestedTypeSet[requestedType] = struct{}{}
		}
		filtered := make([]asc.Resource[asc.CertificateAttributes], 0, len(all))
		for _, certificate := range all {
			certificateType := strings.ToUpper(strings.TrimSpace(certificate.Attributes.CertificateType))
			if _, matches := requestedTypeSet[certificateType]; matches {
				filtered = append(filtered, certificate)
			}
		}
		all = filtered
	}
	if len(all) == 0 {
		if len(requestedTypes) > 0 {
			return nil, fmt.Errorf("profile %s has no associated certificates of type %s: %w", profileID, strings.Join(requestedTypes, ","), errNoMatchingProfileCertificates)
		}
		return nil, fmt.Errorf("profile %s has no associated certificates: %w", profileID, errNoMatchingProfileCertificates)
	}

	usable := usableProfileCertificates(all, time.Now())
	if len(usable) == 0 {
		return nil, fmt.Errorf("profile %s has no active, unexpired associated certificates: %w", profileID, errNoMatchingProfileCertificates)
	}
	return &asc.CertificatesResponse{Data: usable, Links: links}, nil
}

// usableProfileCertificates drops the certificates an existing profile is
// associated with that App Store Connect reports as deactivated or expired, so
// signing fetch never writes and signing sync push never publishes a dead
// certificate. Unlike the creation path this keeps certificates whose metadata
// does not prove they are unusable, because a resolved profile must stay
// usable when the API omits those attributes.
func usableProfileCertificates(certificates []asc.Resource[asc.CertificateAttributes], now time.Time) []asc.Resource[asc.CertificateAttributes] {
	usable := make([]asc.Resource[asc.CertificateAttributes], 0, len(certificates))
	for _, certificate := range certificates {
		if activated := certificate.Attributes.Activated; activated != nil && !*activated {
			continue
		}
		expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(certificate.Attributes.ExpirationDate))
		if err == nil && !expiresAt.After(now) {
			continue
		}
		usable = append(usable, certificate)
	}
	return usable
}

func createProfile(ctx context.Context, client *asc.Client, bundleIDResourceID, profileName, profileType string, certIDs, deviceIDs []string) (*asc.ProfileResponse, error) {
	if len(certIDs) == 0 {
		return nil, fmt.Errorf("no certificates available to create profile")
	}
	return client.CreateProfile(ctx, asc.ProfileCreateAttributes{
		Name:        profileName,
		ProfileType: profileType,
	}, bundleIDResourceID, certIDs, deviceIDs)
}

func profileCreateName(profileType string, now time.Time) string {
	return fmt.Sprintf("%s-%s", profileType, now.Format("20060102"))
}

func isDevelopmentProfile(profileType string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(profileType))
	return strings.Contains(normalized, "DEVELOPMENT") ||
		strings.Contains(normalized, "ADHOC") ||
		strings.Contains(normalized, "AD_HOC")
}

func inferCertificateType(profileType string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(profileType))

	switch {
	case strings.Contains(normalized, "IOS_APP_DEVELOPMENT"):
		return "IOS_DEVELOPMENT,DEVELOPMENT", nil
	case strings.Contains(normalized, "IOS_APP_STORE"),
		strings.Contains(normalized, "IOS_APP_ADHOC"),
		strings.Contains(normalized, "IOS_APP_INHOUSE"):
		return "IOS_DISTRIBUTION,DISTRIBUTION", nil
	case strings.Contains(normalized, "TVOS_APP_DEVELOPMENT"):
		return "IOS_DEVELOPMENT,DEVELOPMENT", nil
	case strings.Contains(normalized, "TVOS_APP_STORE"),
		strings.Contains(normalized, "TVOS_APP_ADHOC"),
		strings.Contains(normalized, "TVOS_APP_INHOUSE"):
		return "IOS_DISTRIBUTION,DISTRIBUTION", nil
	case strings.Contains(normalized, "MAC_CATALYST_APP_DEVELOPMENT"):
		return "MAC_APP_DEVELOPMENT,DEVELOPMENT", nil
	case strings.Contains(normalized, "MAC_CATALYST_APP_STORE"):
		return "MAC_APP_DISTRIBUTION,DISTRIBUTION", nil
	case strings.Contains(normalized, "MAC_CATALYST_APP_DIRECT"):
		return "DEVELOPER_ID_APPLICATION", nil
	case strings.Contains(normalized, "MAC_APP_DEVELOPMENT"):
		return "MAC_APP_DEVELOPMENT,DEVELOPMENT", nil
	case strings.Contains(normalized, "MAC_APP_STORE"):
		return "MAC_APP_DISTRIBUTION,DISTRIBUTION", nil
	case strings.Contains(normalized, "MAC_APP_DIRECT"):
		return "DEVELOPER_ID_APPLICATION", nil
	default:
		return "", fmt.Errorf("unable to infer certificate type for profile type %s; use --certificate-type", profileType)
	}
}

func decodeBase64Content(label, content string) ([]byte, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, fmt.Errorf("%s content is empty", label)
	}
	data, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	return data, nil
}

// signingOutputPaths returns every file signing fetch writes for the resolved
// assets. profileID is empty while the profile is still being planned.
func signingOutputPaths(outputDir, profileName, profileID string, certificates []asc.Resource[asc.CertificateAttributes]) []string {
	paths := make([]string, 0, len(certificates)+1)
	paths = append(paths, profileOutputPath(outputDir, profileName, profileID))
	for _, certificate := range certificates {
		paths = append(paths, certificateOutputPath(outputDir, certificate))
	}
	return paths
}

func profileOutputPath(outputDir, profileName, profileID string) string {
	return filepath.Join(outputDir, safeFileName(profileName, profileID)+".mobileprovision")
}

func certificateOutputPath(outputDir string, certificate asc.Resource[asc.CertificateAttributes]) string {
	return filepath.Join(outputDir, safeFileName(certificate.Attributes.SerialNumber, certificate.ID)+".cer")
}

// ensureOutputPathsAreFree reports the first colliding output file. Writes use
// O_EXCL, so a collision always fails the command; detecting it up front keeps
// the failure free of remote and on-disk side effects.
func ensureOutputPathsAreFree(paths []string) error {
	for _, path := range paths {
		_, err := os.Lstat(path)
		switch {
		case err == nil:
			return fmt.Errorf("output file already exists: %s: %w", path, os.ErrExist)
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return fmt.Errorf("inspect output path %s: %w", path, err)
		}
	}
	return nil
}

func writeBinaryFile(path string, data []byte) error {
	file, err := shared.OpenNewFileNoFollow(path, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("output file already exists: %w", err)
		}
		return err
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

func extractIDs[T any](items []asc.Resource[T]) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func safeFileName(value, fallback string) string {
	sanitize := func(input string) string {
		clean := strings.TrimSpace(input)
		clean = strings.ReplaceAll(clean, "/", "_")
		clean = strings.ReplaceAll(clean, "\\", "_")
		return strings.Trim(clean, ". ")
	}

	clean := sanitize(value)
	if clean == "" || clean == "." || clean == ".." {
		clean = sanitize(fallback)
	}
	return clean
}
