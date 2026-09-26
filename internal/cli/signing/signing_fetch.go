package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const deviceWithoutCreateMissingError = "--device requires --create-missing because device IDs are only applied to profiles this command creates"

// maxProfileNameLength is the issue #2520 proposed guard, measured in Unicode
// code points. The OpenAPI snapshot has no maxLength; generated names stay
// within 64 and explicit --name values longer than that are rejected before
// any API call.
const maxProfileNameLength = 64

const profileNameHashSuffixLen = 6

var signingFetchNowFn = time.Now

// rejectDeviceWithoutCreateMissing fails before any App Store Connect call when
// device IDs were supplied but could never be applied.
func rejectDeviceWithoutCreateMissing(deviceIDs string, createMissing bool) error {
	if !createMissing && strings.TrimSpace(deviceIDs) != "" {
		return shared.UsageError(deviceWithoutCreateMissingError)
	}
	return nil
}

// SigningFetchCommand returns the signing fetch subcommand.
func SigningFetchCommand() *ffcli.Command {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (optional); when set, --bundle-id must be this app's bundle ID")
	bundleID := fs.String("bundle-id", "", "Bundle identifier (e.g., com.example.app) - required")
	profileType := fs.String("profile-type", "", "Profile type: IOS_APP_STORE, IOS_APP_DEVELOPMENT, MAC_APP_STORE, etc. (required)")
	deviceIDs := fs.String("device", "", "Device ID(s), comma-separated (requires --create-missing; required for development profiles)")
	certType := fs.String("certificate-type", "", "Certificate type filter (optional)")
	outputPath := fs.String("output", "./signing", "Output directory for signing files")
	createMissing := fs.Bool("create-missing", false, "Create missing profiles")
	matchExtensions := fs.Bool("match-extensions", false, "Also fetch profiles for registered bundle IDs that extend this identifier, such as <id>.widget")
	strictMatch := fs.Bool("strict-match-identifier", false, "Fetch only the exact bundle identifier")
	deleteStale := fs.Bool("delete-stale-profiles", false, "Delete expired or invalid profiles for this bundle ID and profile type before matching (requires --confirm or --dry-run)")
	confirm := fs.Bool("confirm", false, "Confirm --delete-stale-profiles deletions")
	dryRun := fs.Bool("dry-run", false, "With --delete-stale-profiles, print the stale-profile plan and exit without deleting, creating, or writing anything")
	createMissingCertificate := fs.Bool("create-missing-certificate", false, "Create a certificate when none are active, then create the profile")
	identityPasswordFile := fs.String("identity-password-file", "", "Protected 0600 file containing the PKCS#12 password")
	keyOut := fs.String("key-out", "", "Private key output path (default: <output>/<type>.key)")
	csrOut := fs.String("csr-out", "", "CSR output path (default: <output>/<type>.csr)")
	p12Out := fs.String("p12-out", "", "PKCS#12 output path (default: <output>/<type>.p12)")
	force := fs.Bool("force", false, "Replace existing key, CSR, and p12 output files")
	output := shared.BindOutputFlagsWith(fs, "format", shared.DefaultOutputFormat(), "Output format for metadata: json, table, markdown")

	return &ffcli.Command{
		Name:       "fetch",
		ShortUsage: "asc signing fetch [flags]",
		ShortHelp:  "Fetch signing files (certificates + profiles) for an app.",
		LongHelp: `Fetch signing certificates and provisioning profiles for an app.

This command resolves the bundle ID, finds matching certificates and profiles,
and writes them to the output directory.

Native macOS profiles are written as .provisionprofile files. iOS and tvOS
profiles continue to use .mobileprovision files.

With --create-missing, it will create a new profile if none exist for the
specified configuration. Devices are only applied to profiles this command
creates, so --device without --create-missing is rejected with a usage error.
--create-missing-certificate also creates a key, CSR, certificate, and
password-protected .p12 when no active certificate exists. It requires
--create-missing and --identity-password-file.

--match-extensions also fetches a profile for every registered bundle ID that
extends --bundle-id with a dotted suffix (com.example.app.widget,
com.example.app.clip); wildcard identifiers never match. Each target gets its
own request timeout, a certificate shared by several targets is written once,
and the JSON receipt lists matchedBundleIds, results, and failures. The command
exits 1 after finishing the other targets if any target fails. It cannot be
combined with --create-missing-certificate. --strict-match-identifier is the
explicit exact-match default.

Active profiles whose expiration date has passed are never selected.
--delete-stale-profiles deletes expired or INVALID profiles for this bundle ID
and profile type before matching; it requires --confirm. Every stale profile is
listed before any deletion, and staleProfiles in the output separates planned,
deleted, and failed profiles. If any deletion fails, the command stops before
fetching or creating anything. --dry-run (only with --delete-stale-profiles)
prints the plan and exits without deleting, creating, or writing files. With
--match-extensions, stale profiles are planned for every matched bundle ID
before any deletion, and each result carries its own staleProfiles receipt.

Examples:
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_STORE --output ./signing
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_DEVELOPMENT --create-missing --device "DEVICE1,DEVICE2"
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_STORE --create-missing --create-missing-certificate --identity-password-file ./secrets/p12-password
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_STORE --match-extensions --output ./signing
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_STORE --delete-stale-profiles --dry-run
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_STORE --delete-stale-profiles --confirm --create-missing`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) (runErr error) {
			bundle := strings.TrimSpace(*bundleID)
			if bundle == "" {
				fmt.Fprintln(os.Stderr, "Error: --bundle-id is required")
				return shared.MissingRequiredUsageError("--bundle-id")
			}

			profType := strings.TrimSpace(*profileType)
			if profType == "" {
				fmt.Fprintln(os.Stderr, "Error: --profile-type is required")
				return shared.MissingRequiredUsageError("--profile-type")
			}
			profType = strings.ToUpper(profType)
			if *matchExtensions && *strictMatch {
				return shared.UsageError("--match-extensions and --strict-match-identifier are mutually exclusive")
			}
			if *matchExtensions && *createMissingCertificate {
				const message = "--match-extensions cannot be combined with --create-missing-certificate; create the certificate with a single-bundle fetch first"
				fmt.Fprintln(os.Stderr, "Error: "+message)
				return shared.UsageError(message)
			}
			if !*deleteStale {
				for _, dependent := range []struct {
					name string
					set  bool
				}{{"--confirm", *confirm}, {"--dry-run", *dryRun}} {
					if dependent.set {
						message := dependent.name + " requires --delete-stale-profiles"
						fmt.Fprintln(os.Stderr, "Error: "+message)
						return shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message)
					}
				}
			}
			if *deleteStale && !*confirm && !*dryRun {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required with --delete-stale-profiles")
				return shared.MissingRequiredUsageError("--confirm")
			}
			if *deleteStale {
				// Deletions are irreversible, so every value that would make the
				// later fetch fail deterministically is rejected before them.
				if _, err := resolveSigningCertificateTypes(profType, *certType); err != nil {
					message := fmt.Sprintf("--delete-stale-profiles: %v", err)
					fmt.Fprintln(os.Stderr, "Error: "+message)
					return shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message)
				}
			}
			if err := rejectDeviceWithoutCreateMissing(*deviceIDs, *createMissing); err != nil {
				return err
			}
			if *createMissingCertificate && !*createMissing {
				fmt.Fprintln(os.Stderr, "Error: --create-missing-certificate requires --create-missing")
				return shared.UsageError("--create-missing-certificate requires --create-missing")
			}
			if *createMissingCertificate && strings.TrimSpace(*identityPasswordFile) == "" {
				fmt.Fprintln(os.Stderr, "Error: --identity-password-file is required")
				return shared.MissingRequiredUsageError("--identity-password-file")
			}
			if *createMissing && isDevelopmentProfile(profType) && strings.TrimSpace(*deviceIDs) == "" {
				fmt.Fprintln(os.Stderr, "Error: --device is required for development profiles")
				return shared.MissingRequiredUsageError("--device")
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
				return ensureOutputPathsAreFree(signingOutputPaths(outputDir, profileName, profileID, profType, certificates))
			}
			var password []byte
			if *createMissingCertificate {
				var err error
				password, err = readNonEmptyIdentityPasswordFile(*identityPasswordFile)
				if err != nil {
					return fmt.Errorf("signing fetch: %w", err)
				}
				defer clear(password)
			}
			certSlug := "distribution"
			if isDevelopmentProfile(profType) {
				certSlug = "development"
			}
			keyPath := firstNonEmpty(*keyOut, filepath.Join(outputDir, certSlug+".key"))
			csrPath := firstNonEmpty(*csrOut, filepath.Join(outputDir, certSlug+".csr"))
			p12Path := firstNonEmpty(*p12Out, filepath.Join(outputDir, certSlug+".p12"))
			certificateOutputs := &signingCertificateOutputs{BasePath: outputDir}
			defer func() { _ = certificateOutputs.Close() }()

			var result *asc.SigningFetchResult
			emitted := false
			emit := func() error {
				emitted = true
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}
			// Stale-profile deletions are irreversible, so their receipt is
			// printed even when a later step of the fetch fails.
			defer func() {
				if runErr == nil || emitted || result == nil || result.StaleProfiles == nil || len(result.StaleProfiles.Deleted) == 0 {
					return
				}
				_ = emit()
			}()

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("signing fetch: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// Only an explicit --app is cross-checked. The ASC_APP_ID or config
			// default app must not veto the bundle ID the caller named.
			if explicitAppID := strings.TrimSpace(*appID); explicitAppID != "" {
				if err := validateBundleIDMatchesApp(requestCtx, client, explicitAppID, bundle); err != nil {
					return fmt.Errorf("signing fetch: %w", err)
				}
			}

			result = &asc.SigningFetchResult{
				BundleID:    bundle,
				ProfileType: profType,
				OutputPath:  outputDir,
			}

			bundleIDs, err := listSigningBundleIDs(requestCtx, client, bundle, *matchExtensions && !*strictMatch)
			if err != nil {
				return fmt.Errorf("signing fetch: %w", err)
			}
			if len(bundleIDs) > 1 {
				batch, batchErr := fetchMatchedSigningBundles(ctx, client, bundleIDs, matchedSigningFetchOptions{
					ProfileType:     profType,
					OutputDir:       outputDir,
					CertificateType: *certType,
					DeviceIDs:       shared.SplitCSV(*deviceIDs),
					CreateMissing:   *createMissing,
					DeleteStale:     *deleteStale,
					DryRun:          *dryRun,
					PrepareOutput:   prepareOutputDir,
				})
				if *dryRun {
					fmt.Fprintf(os.Stderr, "Dry run: would delete %d stale profile(s) across %d bundle ID(s); nothing was deleted, created, or written\n", batchStalePlanned(batch), len(bundleIDs))
				}
				if err := shared.PrintOutput(batch, *output.Output, *output.Pretty); err != nil {
					return err
				}
				return batchErr
			}
			bundleIDResp := &asc.BundleIDResponse{Data: bundleIDs[0]}
			result.BundleIDResource = bundleIDResp.Data.ID

			if *deleteStale {
				planned, err := findStaleSigningProfiles(requestCtx, client, bundleIDResp.Data.ID, profType)
				if err != nil {
					return fmt.Errorf("signing fetch: list stale profiles: %w", err)
				}
				result.StaleProfiles = &asc.SigningFetchStaleProfiles{
					DryRun:  *dryRun,
					Planned: planned,
					Deleted: []asc.SigningStaleProfile{},
				}
				if *dryRun {
					fmt.Fprintf(os.Stderr, "Dry run: would delete %d stale profile(s); nothing was deleted, created, or written\n", len(planned))
					return emit()
				}
				// The output directory is the one deterministic local check that
				// does not depend on which profile is later resolved, so it runs
				// before the irreversible deletions.
				if len(planned) > 0 {
					if err := prepareOutputDir(); err != nil {
						return fmt.Errorf("signing fetch: %w; no stale profiles were deleted", err)
					}
				}
				deleteStaleSigningProfiles(requestCtx, client, result.StaleProfiles)
				if failed := len(result.StaleProfiles.Failed); failed > 0 {
					_ = emit()
					return fmt.Errorf("signing fetch: failed to delete %d of %d stale profile(s); nothing was fetched or created", failed, len(planned))
				}
			}

			var createdIdentity createdSigningIdentity
			createdFlag := false
			progress := &signingAssetsProgress{}

			if *createMissingCertificate {
				if err := prepareOutputDir(); err != nil {
					return fmt.Errorf("signing fetch: %w", err)
				}
			}
			profile, certs, created, err := resolveSigningAssets(
				requestCtx,
				client,
				signingAssetsOptions{
					BundleIDResourceID:       bundleIDResp.Data.ID,
					BundleIdentifier:         bundle,
					ProfileType:              profType,
					CertificateType:          *certType,
					DeviceIDs:                shared.SplitCSV(*deviceIDs),
					CreateMissing:            *createMissing,
					CreateMissingCertificate: *createMissingCertificate,
					CertificateCreate: signingCertificateCreateRequest{
						Outputs:  certificateOutputs,
						KeyPath:  keyPath,
						CSRPath:  csrPath,
						P12Path:  p12Path,
						Password: password,
						Force:    *force,
					},
					CreatedIdentity: &createdIdentity,
					Progress:        progress,
					BeforeCertificateCreate: func(plan profileCreatePlan) error {
						if err := preflightOutput(plan.ProfileName, "", nil); err != nil {
							return err
						}
						if err := certificateOutputs.Prepare([]string{keyPath, csrPath, p12Path}, *force); err != nil {
							return err
						}
						plannedProfilePath := profileOutputPath(outputDir, plan.ProfileName, "", profType)
						if err := certificateOutputs.CheckDistinct(plannedProfilePath); err != nil {
							return err
						}
						metadataPath := filepath.Join(outputDir, "profiles.json")
						if err := certificateOutputs.CheckDistinct(metadataPath); err != nil {
							return err
						}
						return ensureOutputPathsAreFree([]string{metadataPath})
					},
					BeforeCreate: func(plan profileCreatePlan) error {
						return preflightOutput(plan.ProfileName, "", plan.Certificates)
					},
				},
			)
			createdFlag = createdIdentity.CertificateID != "" && createdIdentity.P12Path != ""
			if err != nil {
				if createdFlag || createdIdentity.CertificateAttempted || progress.ProfileCreateAttempted {
					result.Partial = true
					result.CertificateIDs = extractIDs(progress.Certificates)
					if createdIdentity.CertificateAttempted {
						result.CertificateCreationState = "unknown"
					}
					if createdFlag {
						result.CertificateCreationState = "created"
					}
					applyCreatedIdentity(result, createdIdentity, createdFlag)
					if progress.ProfileCreateAttempted {
						result.ProfileCreationState = "unknown"
					}
					if createdFlag {
						metadataPath := filepath.Join(outputDir, "profiles.json")
						if err := writeSigningProfilesMetadata(metadataPath, signingProfilesMetadata{
							CertificateID:     createdIdentity.CertificateID,
							CertificateSHA256: createdIdentity.CertificateSHA256,
							P12Path:           createdIdentity.P12Path,
							PrivateKeyPath:    createdIdentity.PrivateKeyPath,
						}); err == nil {
							result.ProfilesMetadataPath = metadataPath
						}
					}
					_ = emit()
				}
				return fmt.Errorf("signing fetch: %w", err)
			}
			result.CertificateIDs = extractIDs(certs.Data)
			result.ProfileID = profile.Data.ID
			result.Created = created
			if createdFlag {
				result.CertificateCreationState = "created"
			} else if *createMissingCertificate {
				result.CertificateCreationState = "reused"
				falseValue := false
				result.CertificateCreated = &falseValue
			}
			if created {
				result.ProfileCreationState = "created"
			} else if *createMissing {
				result.ProfileCreationState = "reused"
			}
			reportPartial := func(primary error) error {
				if !created && !progress.ProfileCreateAttempted && result.ProfileFile == "" && len(result.CertificateFiles) == 0 {
					return fmt.Errorf("signing fetch: %w", primary)
				}
				result.Partial = true
				if created {
					result.ProfileCreationState = "created"
				}
				applyCreatedIdentity(result, createdIdentity, createdFlag)
				_ = emit()
				return fmt.Errorf("signing fetch: %w", primary)
			}

			if err := preflightOutput(profile.Data.Attributes.Name, profile.Data.ID, certs.Data); err != nil {
				return reportPartial(err)
			}

			profilePath := profileOutputPath(outputDir, profile.Data.Attributes.Name, profile.Data.ID, profType)
			profileContent, err := decodeBase64Content("profile", profile.Data.Attributes.ProfileContent)
			if err != nil {
				return reportPartial(fmt.Errorf("decode profile: %w", err))
			}
			if err := shared.WriteProfileFile(profilePath, profileContent); err != nil {
				return reportPartial(fmt.Errorf("write profile: %w", err))
			}
			result.ProfileFile = profilePath

			for _, cert := range certs.Data {
				certPath := certificateOutputPath(outputDir, cert)
				certContent, err := decodeBase64Content("certificate", cert.Attributes.CertificateContent)
				if err != nil {
					return reportPartial(fmt.Errorf("decode certificate: %w", err))
				}
				if err := writeBinaryFile(certPath, certContent); err != nil {
					return reportPartial(fmt.Errorf("write certificate: %w", err))
				}
				result.CertificateFiles = append(result.CertificateFiles, certPath)
			}
			if createdFlag {
				applyCreatedIdentity(result, createdIdentity, createdFlag)
				metadataPath := filepath.Join(outputDir, "profiles.json")
				certificateID := ""
				if len(result.CertificateIDs) > 0 {
					certificateID = result.CertificateIDs[0]
				}
				if err := writeSigningProfilesMetadata(metadataPath, signingProfilesMetadata{
					CertificateID:     certificateID,
					CertificateSHA256: createdIdentity.CertificateSHA256,
					P12Path:           createdIdentity.P12Path,
					PrivateKeyPath:    createdIdentity.PrivateKeyPath,
					ProfilePath:       profilePath,
				}); err != nil {
					return reportPartial(fmt.Errorf("write profiles.json: %w", err))
				}
				result.ProfilesMetadataPath = metadataPath
			}

			return emit()
		},
	}
}

type matchedSigningFetchOptions struct {
	ProfileType     string
	OutputDir       string
	CertificateType string
	DeviceIDs       []string
	CreateMissing   bool
	DeleteStale     bool
	DryRun          bool
	PrepareOutput   func() error
}

// writtenSigningFile is a file this run created, kept so a failed target can
// remove exactly what it wrote.
type writtenSigningFile struct {
	path string
	data []byte
}

// fetchMatchedSigningBundles fetches one profile per matched bundle ID. Each
// target gets its own request timeout. A certificate shared by several
// targets is written once and reported for every target that uses it. A
// target that fails removes only the files it created, never a shared
// certificate an earlier target already wrote.
func fetchMatchedSigningBundles(
	ctx context.Context,
	client *asc.Client,
	bundleIDs []asc.Resource[asc.BundleIDAttributes],
	opts matchedSigningFetchOptions,
) (*asc.SigningFetchBatchResult, error) {
	batch := &asc.SigningFetchBatchResult{
		MatchedBundleIDs: make([]string, 0, len(bundleIDs)),
		Results:          make([]asc.SigningFetchResult, 0, len(bundleIDs)),
	}
	identifiers := make([]string, 0, len(bundleIDs))
	for _, item := range bundleIDs {
		identifiers = append(identifiers, strings.TrimSpace(item.Attributes.Identifier))
	}
	batch.MatchedBundleIDs = append(batch.MatchedBundleIDs, identifiers...)

	stale := make([]*asc.SigningFetchStaleProfiles, len(bundleIDs))
	if opts.DeleteStale {
		done, err := deleteMatchedStaleProfiles(ctx, client, bundleIDs, identifiers, opts, stale, batch)
		if done || err != nil {
			return batch, err
		}
	}

	writtenCertificates := make(map[string]string)
	for i, item := range bundleIDs {
		result, err := fetchMatchedSigningBundle(ctx, client, item, identifiers[i], opts, writtenCertificates)
		if err != nil {
			failure := asc.SigningFetchBatchFailure{BundleID: identifiers[i], Error: err.Error(), StaleProfiles: stale[i]}
			if result != nil {
				failure.ProfileID = result.ProfileID
				failure.ProfileCreationState = result.ProfileCreationState
			}
			batch.Failures = append(batch.Failures, failure)
			continue
		}
		result.StaleProfiles = stale[i]
		batch.Results = append(batch.Results, *result)
	}
	if len(batch.Failures) > 0 {
		return batch, fmt.Errorf("signing fetch: %d of %d bundle ID(s) failed", len(batch.Failures), len(bundleIDs))
	}
	return batch, nil
}

// deleteMatchedStaleProfiles applies --delete-stale-profiles to every matched
// bundle ID. Every target is planned before anything is deleted. A dry run
// records the plans and stops. If any deletion fails, every target is
// reported as a failure with its receipt and nothing is fetched or created.
// done reports that the batch is complete and must not continue to fetch.
func deleteMatchedStaleProfiles(
	ctx context.Context,
	client *asc.Client,
	bundleIDs []asc.Resource[asc.BundleIDAttributes],
	identifiers []string,
	opts matchedSigningFetchOptions,
	stale []*asc.SigningFetchStaleProfiles,
	batch *asc.SigningFetchBatchResult,
) (bool, error) {
	for i, item := range bundleIDs {
		planCtx, cancel := shared.ContextWithTimeout(ctx)
		planned, err := findStaleSigningProfiles(planCtx, client, item.ID, opts.ProfileType)
		cancel()
		if err != nil {
			batch.Failures = append(batch.Failures, asc.SigningFetchBatchFailure{BundleID: identifiers[i], Error: fmt.Sprintf("list stale profiles: %v", err)})
			return true, fmt.Errorf("signing fetch: list stale profiles for %s: %w; nothing was deleted", identifiers[i], err)
		}
		stale[i] = &asc.SigningFetchStaleProfiles{DryRun: opts.DryRun, Planned: planned, Deleted: []asc.SigningStaleProfile{}}
	}
	if opts.DryRun {
		for i, item := range bundleIDs {
			batch.Results = append(batch.Results, asc.SigningFetchResult{
				BundleID:         identifiers[i],
				BundleIDResource: item.ID,
				ProfileType:      opts.ProfileType,
				OutputPath:       opts.OutputDir,
				StaleProfiles:    stale[i],
			})
		}
		return true, nil
	}
	for _, plan := range stale {
		if len(plan.Planned) == 0 {
			continue
		}
		// Check the output directory before the irreversible deletions.
		if err := opts.PrepareOutput(); err != nil {
			batch.Failures = append(batch.Failures, asc.SigningFetchBatchFailure{BundleID: identifiers[0], Error: err.Error()})
			return true, fmt.Errorf("signing fetch: %w; no stale profiles were deleted", err)
		}
		break
	}
	failed, planned := 0, 0
	for i := range bundleIDs {
		deleteCtx, cancel := shared.ContextWithTimeout(ctx)
		deleteStaleSigningProfiles(deleteCtx, client, stale[i])
		cancel()
		failed += len(stale[i].Failed)
		planned += len(stale[i].Planned)
	}
	if failed == 0 {
		return false, nil
	}
	for i := range bundleIDs {
		message := "not fetched: stale profile deletion failed for another bundle ID"
		if count := len(stale[i].Failed); count > 0 {
			message = fmt.Sprintf("failed to delete %d stale profile(s)", count)
		}
		batch.Failures = append(batch.Failures, asc.SigningFetchBatchFailure{BundleID: identifiers[i], Error: message, StaleProfiles: stale[i]})
	}
	return true, fmt.Errorf("signing fetch: failed to delete %d of %d stale profile(s); nothing was fetched or created", failed, planned)
}

func batchStalePlanned(batch *asc.SigningFetchBatchResult) int {
	total := 0
	for _, result := range batch.Results {
		if result.StaleProfiles != nil {
			total += len(result.StaleProfiles.Planned)
		}
	}
	return total
}

func fetchMatchedSigningBundle(
	ctx context.Context,
	client *asc.Client,
	item asc.Resource[asc.BundleIDAttributes],
	identifier string,
	opts matchedSigningFetchOptions,
	writtenCertificates map[string]string,
) (*asc.SigningFetchResult, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	// Certificates written for an earlier target are reused, so only the
	// remaining ones must be free before a profile is created or written.
	preflight := func(profileName, profileID string, certificates []asc.Resource[asc.CertificateAttributes]) error {
		if err := opts.PrepareOutput(); err != nil {
			return err
		}
		pending := make([]asc.Resource[asc.CertificateAttributes], 0, len(certificates))
		for _, certificate := range certificates {
			if _, ok := writtenCertificates[certificate.ID]; !ok {
				pending = append(pending, certificate)
			}
		}
		return ensureOutputPathsAreFree(signingOutputPaths(opts.OutputDir, profileName, profileID, opts.ProfileType, pending))
	}

	progress := &signingAssetsProgress{}
	profile, certs, created, err := resolveSigningAssets(requestCtx, client, signingAssetsOptions{
		BundleIDResourceID: item.ID,
		BundleIdentifier:   identifier,
		// Generated names carry the bundle ID so targets created in one run
		// do not collide in App Store Connect or in the output directory.
		ProfileName:     profileCreateNameForTarget(opts.ProfileType, identifier, signingFetchNowFn()),
		ProfileType:     opts.ProfileType,
		CertificateType: opts.CertificateType,
		DeviceIDs:       opts.DeviceIDs,
		CreateMissing:   opts.CreateMissing,
		Progress:        progress,
		BeforeCreate: func(plan profileCreatePlan) error {
			return preflight(plan.ProfileName, "", plan.Certificates)
		},
	})
	if err != nil {
		if progress.ProfileCreateAttempted {
			// The create request may have reached App Store Connect.
			return &asc.SigningFetchResult{BundleID: identifier, ProfileCreationState: "unknown"}, err
		}
		return nil, err
	}
	result := &asc.SigningFetchResult{
		BundleID:         identifier,
		BundleIDResource: item.ID,
		ProfileType:      opts.ProfileType,
		ProfileID:        profile.Data.ID,
		CertificateIDs:   extractIDs(certs.Data),
		OutputPath:       opts.OutputDir,
		Created:          created,
	}
	if created {
		result.ProfileCreationState = "created"
	}
	// Once a profile exists, failures still return the result so the batch
	// receipt can report the profile this run created.
	partial := func(cause error) (*asc.SigningFetchResult, error) {
		if created {
			return result, cause
		}
		return nil, cause
	}
	fail := func(written []writtenSigningFile, cause error) error {
		if cleanupErr := removeWrittenSigningFiles(opts.OutputDir, written); cleanupErr != nil {
			return errors.Join(cause, fmt.Errorf("remove partial files: %w", cleanupErr))
		}
		return cause
	}
	if err := preflight(profile.Data.Attributes.Name, profile.Data.ID, certs.Data); err != nil {
		return partial(err)
	}
	profilePath := profileOutputPath(opts.OutputDir, profile.Data.Attributes.Name, profile.Data.ID, opts.ProfileType)
	profileContent, err := decodeBase64Content("profile", profile.Data.Attributes.ProfileContent)
	if err != nil {
		return partial(fmt.Errorf("decode profile: %w", err))
	}
	if err := shared.WriteProfileFile(profilePath, profileContent); err != nil {
		return partial(fmt.Errorf("write profile: %w", err))
	}
	result.ProfileFile = profilePath
	written := []writtenSigningFile{{path: profilePath, data: profileContent}}
	newCertificates := make(map[string]string)
	for _, cert := range certs.Data {
		if path, ok := writtenCertificates[cert.ID]; ok {
			result.CertificateFiles = append(result.CertificateFiles, path)
			continue
		}
		certPath := certificateOutputPath(opts.OutputDir, cert)
		certContent, err := decodeBase64Content("certificate", cert.Attributes.CertificateContent)
		if err != nil {
			result.ProfileFile, result.CertificateFiles = "", nil
			return partial(fail(written, fmt.Errorf("decode certificate: %w", err)))
		}
		if err := writeBinaryFile(certPath, certContent); err != nil {
			result.ProfileFile, result.CertificateFiles = "", nil
			return partial(fail(written, fmt.Errorf("write certificate: %w", err)))
		}
		written = append(written, writtenSigningFile{path: certPath, data: certContent})
		newCertificates[cert.ID] = certPath
		result.CertificateFiles = append(result.CertificateFiles, certPath)
	}
	for id, path := range newCertificates {
		writtenCertificates[id] = path
	}
	return result, nil
}

// removeWrittenSigningFiles removes files this run created beneath outputDir
// through a rooted handle, and only while their content is unchanged.
func removeWrittenSigningFiles(outputDir string, files []writtenSigningFile) error {
	if len(files) == 0 {
		return nil
	}
	absoluteDir, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	root, err := rootfs.New(absoluteDir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	var errs []error
	for _, written := range files {
		absolutePath, err := filepath.Abs(written.path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		name, err := filepath.Rel(absoluteDir, absolutePath)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		file, err := root.OpenFile(name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		info, statErr := file.Stat()
		data, readErr := io.ReadAll(io.LimitReader(file, int64(len(written.data))+1))
		closeErr := file.Close()
		if statErr != nil || readErr != nil || closeErr != nil {
			errs = append(errs, errors.Join(statErr, readErr, closeErr))
			continue
		}
		if !bytes.Equal(data, written.data) {
			errs = append(errs, fmt.Errorf("%s changed after it was written; leaving it in place", written.path))
			continue
		}
		if err := root.RemoveFileIfSame(name, info, data); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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

// bundleIdentifierMatches reports whether candidate is parent itself or, with
// matchExtensions, a dotted descendant such as parent.widget. Wildcard
// identifiers are never expanded or matched as extensions.
func bundleIdentifierMatches(parent, candidate string, matchExtensions bool) bool {
	parent = strings.TrimSpace(parent)
	candidate = strings.TrimSpace(candidate)
	if parent == "" || candidate == "" {
		return false
	}
	if strings.EqualFold(parent, candidate) {
		return true
	}
	if !matchExtensions || strings.Contains(parent, "*") || strings.Contains(candidate, "*") {
		return false
	}
	return strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(parent)+".")
}

// listSigningBundleIDs returns the exact bundle ID first, followed by its
// registered extension bundle IDs when matchExtensions is set. App Store
// Connect's filter[identifier] is a prefix/substring match, so the filtered
// list is read in full and narrowed client-side to the exact identifier and
// its dotted descendants.
func listSigningBundleIDs(ctx context.Context, client *asc.Client, identifier string, matchExtensions bool) ([]asc.Resource[asc.BundleIDAttributes], error) {
	if !matchExtensions || strings.Contains(identifier, "*") {
		exact, err := findBundleID(ctx, client, identifier)
		if err != nil {
			return nil, err
		}
		return []asc.Resource[asc.BundleIDAttributes]{exact.Data}, nil
	}

	var exact *asc.Resource[asc.BundleIDAttributes]
	var extensions []asc.Resource[asc.BundleIDAttributes]
	seen := make(map[string]struct{})
	seenNext := make(map[string]struct{})
	next := ""
	page := 1
	for {
		opts := []asc.BundleIDsOption{asc.WithBundleIDsFilterIdentifier(identifier), asc.WithBundleIDsLimit(200)}
		if next != "" {
			opts = []asc.BundleIDsOption{asc.WithBundleIDsNextURL(next)}
		}
		resp, err := client.GetBundleIDs(ctx, opts...)
		if err != nil {
			return nil, err
		}
		for _, item := range resp.Data {
			if _, exists := seen[item.ID]; exists {
				continue
			}
			candidate := strings.TrimSpace(item.Attributes.Identifier)
			switch {
			case strings.EqualFold(candidate, strings.TrimSpace(identifier)):
				if exact == nil {
					matched := item
					exact = &matched
				}
			case bundleIdentifierMatches(identifier, candidate, true):
				extensions = append(extensions, item)
			default:
				continue
			}
			seen[item.ID] = struct{}{}
		}
		if strings.TrimSpace(resp.Links.Next) == "" {
			break
		}
		if _, repeated := seenNext[resp.Links.Next]; repeated {
			return nil, fmt.Errorf("list bundle IDs page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[resp.Links.Next] = struct{}{}
		page++
		next = resp.Links.Next
	}
	if exact == nil {
		return nil, fmt.Errorf("bundle ID not found: %s", identifier)
	}
	return append([]asc.Resource[asc.BundleIDAttributes]{*exact}, extensions...), nil
}

// findBundleID returns the bundle ID whose identifier equals identifier.
// App Store Connect's filter[identifier] is a prefix/substring match, so the
// filtered list is read in full and the exact identifier is selected instead
// of trusting the first result.
func findBundleID(ctx context.Context, client *asc.Client, identifier string) (*asc.BundleIDResponse, error) {
	want := strings.TrimSpace(identifier)
	next := ""
	page := 1
	seenNext := make(map[string]struct{})
	for {
		opts := []asc.BundleIDsOption{asc.WithBundleIDsFilterIdentifier(identifier), asc.WithBundleIDsLimit(200)}
		if next != "" {
			opts = []asc.BundleIDsOption{asc.WithBundleIDsNextURL(next)}
		}
		resp, err := client.GetBundleIDs(ctx, opts...)
		if err != nil {
			return nil, err
		}
		for _, item := range resp.Data {
			if strings.EqualFold(strings.TrimSpace(item.Attributes.Identifier), want) {
				return &asc.BundleIDResponse{Data: item}, nil
			}
		}
		if strings.TrimSpace(resp.Links.Next) == "" {
			return nil, fmt.Errorf("bundle ID not found: %s", identifier)
		}
		if _, repeated := seenNext[resp.Links.Next]; repeated {
			return nil, fmt.Errorf("list bundle IDs page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[resp.Links.Next] = struct{}{}
		page++
		next = resp.Links.Next
	}
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
	page := 1
	seenNext := make(map[string]struct{})
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
		if _, ok := seenNext[resp.Links.Next]; ok {
			return nil, fmt.Errorf("page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[resp.Links.Next] = struct{}{}
		page++
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
	// ProfileName overrides the default name for a profile created by this
	// resolution. Batch callers use a deterministic target-scoped name while
	// single-target callers retain the historical profile type/date name.
	ProfileName              string
	CertificateType          string
	DeviceIDs                []string
	CreateMissing            bool
	CreateMissingCertificate bool
	CertificateCreate        signingCertificateCreateRequest
	CreatedIdentity          *createdSigningIdentity
	CertificateFallback      *asc.Resource[asc.CertificateAttributes]
	CreatedCertificate       *asc.Resource[asc.CertificateAttributes]
	AfterCertificateCreate   func(createdSigningIdentity) error
	BeforeCertificateCreate  func(profileCreatePlan) error
	Progress                 *signingAssetsProgress
	BeforeCreate             func(profileCreatePlan) error
	CreateContext            func() (context.Context, context.CancelFunc)
	CertificateFilter        func(asc.Resource[asc.CertificateAttributes]) bool
}

// profileCreatePlan describes the profile that is about to be created so callers
// can fail before App Store Connect is mutated.
type profileCreatePlan struct {
	ProfileName  string
	Certificates []asc.Resource[asc.CertificateAttributes]
}

type signingAssetsProgress struct {
	Certificates           []asc.Resource[asc.CertificateAttributes]
	ProfileCreateAttempted bool
}

var errNoMatchingProfileCertificates = errors.New("profile has no matching associated certificates")

func resolveSigningAssets(ctx context.Context, client *asc.Client, options signingAssetsOptions) (*asc.ProfileResponse, *asc.CertificatesResponse, bool, error) {
	certificateType, err := resolveSigningCertificateTypes(options.ProfileType, options.CertificateType)
	if err != nil {
		return nil, nil, false, err
	}
	profileName := strings.TrimSpace(options.ProfileName)
	if profileName == "" {
		profileName = profileCreateName(options.ProfileType, signingFetchNowFn())
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
	if err != nil && (!options.CreateMissingCertificate || !noSigningCertificates(err)) {
		return nil, nil, false, err
	}
	if certificates == nil {
		certificates = &asc.CertificatesResponse{}
	}
	if options.CertificateFallback != nil && options.CertificateFallback.ID != "" {
		seen := false
		for _, certificate := range certificates.Data {
			if certificate.ID == options.CertificateFallback.ID {
				seen = true
				break
			}
		}
		if !seen {
			certificates.Data = append(certificates.Data, *options.CertificateFallback)
		}
	}
	fetchedCertificateCount := len(certificates.Data)
	certificates.Data = filterSigningCertificates(certificates.Data, options.CertificateFilter)
	if options.CertificateFilter != nil && fetchedCertificateCount > 0 && len(certificates.Data) == 0 {
		return nil, nil, false, errors.New("no App Store Connect certificate matches the local signing identity or requested --identity-sha256")
	}
	certificates.Data = certificatesForProfileCreation(certificates.Data, options.ProfileType, time.Now())
	if len(certificates.Data) == 0 {
		if !options.CreateMissingCertificate {
			return nil, nil, false, fmt.Errorf(
				"no active, unexpired certificates available to create %s profile",
				options.ProfileType,
			)
		}
		primaryType, typeErr := primarySigningCertificateType(options.ProfileType, options.CertificateType)
		if typeErr != nil {
			return nil, nil, false, typeErr
		}
		if options.BeforeCertificateCreate != nil {
			plan := profileCreatePlan{ProfileName: profileName}
			if err := options.BeforeCertificateCreate(plan); err != nil {
				return nil, nil, false, fmt.Errorf("preflight before creating certificate: %w", err)
			}
		}
		request := options.CertificateCreate
		request.CertificateType = primaryType
		created, identity, createErr := createMissingSigningCertificate(ctx, client, request)
		if options.Progress != nil && created.ID != "" {
			options.Progress.Certificates = []asc.Resource[asc.CertificateAttributes]{created}
		}
		if createErr != nil {
			if options.CreatedIdentity != nil {
				*options.CreatedIdentity = identity
			}
			return nil, nil, false, createErr
		}
		if options.CreatedIdentity != nil {
			*options.CreatedIdentity = identity
		}
		if options.CreatedCertificate != nil {
			*options.CreatedCertificate = created
		}
		if options.AfterCertificateCreate != nil {
			if err := options.AfterCertificateCreate(identity); err != nil {
				return nil, nil, false, err
			}
		}
		certificates.Data = []asc.Resource[asc.CertificateAttributes]{created}
	}
	if options.BeforeCreate != nil {
		if options.Progress != nil {
			options.Progress.Certificates = append([]asc.Resource[asc.CertificateAttributes](nil), certificates.Data...)
		}
		plan := profileCreatePlan{ProfileName: profileName, Certificates: certificates.Data}
		if err := options.BeforeCreate(plan); err != nil {
			return nil, nil, false, fmt.Errorf("preflight before creating profile: %w", err)
		}
	}
	if options.Progress != nil {
		options.Progress.Certificates = append([]asc.Resource[asc.CertificateAttributes](nil), certificates.Data...)
		options.Progress.ProfileCreateAttempted = true
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

	for index, certificateType := range certificateTypes {
		canonical, ok := shared.CanonicalCertificateType(certificateType)
		if !ok {
			return "", fmt.Errorf("unsupported certificate type %s", certificateType)
		}
		certificateTypes[index] = canonical
	}
	return strings.Join(certificateTypes, ","), nil
}

func profileIsStale(profile asc.Resource[asc.ProfileAttributes], profileType string, now time.Time) bool {
	if !strings.EqualFold(strings.TrimSpace(profile.Attributes.ProfileType), profileType) {
		return false
	}
	return asc.ProfileIsStale(profile.Attributes, now)
}

// findStaleSigningProfiles collects every expired or INVALID profile of the
// given type for a bundle ID. It reads all pages before anything is deleted so
// deletions cannot shift pages and hide profiles from the plan.
func findStaleSigningProfiles(ctx context.Context, client *asc.Client, bundleIDResourceID, profileType string) ([]asc.SigningStaleProfile, error) {
	stale := []asc.SigningStaleProfile{}
	next := ""
	page := 1
	seenNext := make(map[string]struct{})
	now := signingFetchNowFn()
	for {
		profiles, err := client.GetBundleIDProfiles(ctx, bundleIDResourceID, asc.WithBundleIDProfilesNextURL(next))
		if err != nil {
			return nil, err
		}
		for _, profile := range profiles.Data {
			if !profileIsStale(profile, profileType, now) {
				continue
			}
			stale = append(stale, asc.SigningStaleProfile{
				ID:             profile.ID,
				Name:           profile.Attributes.Name,
				ExpirationDate: profile.Attributes.ExpirationDate,
				State:          string(profile.Attributes.ProfileState),
			})
		}
		if strings.TrimSpace(profiles.Links.Next) == "" {
			return stale, nil
		}
		if _, ok := seenNext[profiles.Links.Next]; ok {
			return nil, fmt.Errorf("page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[profiles.Links.Next] = struct{}{}
		page++
		next = profiles.Links.Next
	}
}

// deleteStaleSigningProfiles deletes every planned profile, recording a
// profile as deleted only after Apple confirms the deletion.
func deleteStaleSigningProfiles(ctx context.Context, client *asc.Client, report *asc.SigningFetchStaleProfiles) {
	for _, profile := range report.Planned {
		if err := client.DeleteProfile(ctx, profile.ID); err != nil {
			report.Failed = append(report.Failed, asc.SigningStaleProfileFailure{
				ID:    profile.ID,
				Name:  profile.Name,
				Error: err.Error(),
			})
			continue
		}
		report.Deleted = append(report.Deleted, profile)
	}
}

func findActiveProfiles(ctx context.Context, client *asc.Client, bundleIDResourceID, profileType string) ([]asc.Resource[asc.ProfileAttributes], error) {
	var matches []asc.Resource[asc.ProfileAttributes]
	next := ""
	page := 1
	seenNext := make(map[string]struct{})
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
			if asc.ProfileExpirationPassed(profile.Attributes.ExpirationDate, signingFetchNowFn()) {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(profile.Attributes.ProfileType), profileType) {
				matches = append(matches, profile)
			}
		}

		if strings.TrimSpace(profiles.Links.Next) == "" {
			return matches, nil
		}
		if _, ok := seenNext[profiles.Links.Next]; ok {
			return nil, fmt.Errorf("page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[profiles.Links.Next] = struct{}{}
		page++
		next = profiles.Links.Next
	}
}

func findProfileCertificates(ctx context.Context, client *asc.Client, profileID, certificateType string) (*asc.CertificatesResponse, error) {
	var (
		all   []asc.Resource[asc.CertificateAttributes]
		links asc.Links
		next  string
	)
	page := 1
	seenNext := make(map[string]struct{})
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
		if _, ok := seenNext[response.Links.Next]; ok {
			return nil, fmt.Errorf("page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[response.Links.Next] = struct{}{}
		page++
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

// profileCreateNameForTarget preserves the historical type/date prefix while
// making names created during one batch unambiguous to App Store Connect.
// Bundle IDs have already passed the manifest validation boundary, but use the
// same filename-safe component as repository paths so direct callers cannot
// introduce separators into the API name either.
func profileCreateNameForTarget(profileType, bundleIdentifier string, now time.Time) string {
	prefix := profileCreateName(profileType, now)
	component := safeFileName(bundleIdentifier, "target")
	full := prefix + "-" + component
	if utf8.RuneCountInString(full) <= maxProfileNameLength {
		return full
	}
	hash := profileNameHash(bundleIdentifier)
	budget := maxProfileNameLength - utf8.RuneCountInString(prefix) - utf8.RuneCountInString(hash) - 2
	if budget < 1 {
		budget = 1
	}
	componentRunes := []rune(component)
	if len(componentRunes) > budget {
		component = string(componentRunes[:budget])
	}
	return prefix + "-" + component + "-" + hash
}

func profileNameHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:profileNameHashSuffixLen]
}

// ValidateProfileNameLength reports a usage error when an explicit profile
// name exceeds the generated-name guard.
func ValidateProfileNameLength(name string) error {
	length := utf8.RuneCountInString(name)
	if length <= maxProfileNameLength {
		return nil
	}
	return fmt.Errorf("profile name must be at most %d characters; got %d", maxProfileNameLength, length)
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
		return "DEVELOPER_ID_APPLICATION,DEVELOPER_ID_APPLICATION_G2", nil
	case strings.Contains(normalized, "MAC_APP_DEVELOPMENT"):
		return "MAC_APP_DEVELOPMENT,DEVELOPMENT", nil
	case strings.Contains(normalized, "MAC_APP_STORE"):
		return "MAC_APP_DISTRIBUTION,DISTRIBUTION", nil
	case strings.Contains(normalized, "MAC_APP_DIRECT"):
		return "DEVELOPER_ID_APPLICATION,DEVELOPER_ID_APPLICATION_G2", nil
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
func signingOutputPaths(outputDir, profileName, profileID, profileType string, certificates []asc.Resource[asc.CertificateAttributes]) []string {
	paths := make([]string, 0, len(certificates)+1)
	paths = append(paths, profileOutputPath(outputDir, profileName, profileID, profileType))
	for _, certificate := range certificates {
		paths = append(paths, certificateOutputPath(outputDir, certificate))
	}
	return paths
}

func profileOutputPath(outputDir, profileName, profileID, profileType string) string {
	extension := shared.ProvisioningProfileExtension("", profileType)
	return filepath.Join(outputDir, safeFileName(profileName, profileID)+extension)
}

func certificateOutputPath(outputDir string, certificate asc.Resource[asc.CertificateAttributes]) string {
	return filepath.Join(outputDir, safeFileName(certificate.Attributes.SerialNumber, certificate.ID)+".cer")
}

// validateOutputPathStructure rejects output paths whose parents cannot accept
// a file and rejects aliases within one write set. These failures are
// deterministic, so callers must detect them before any remote mutation.
func validateOutputPathStructure(paths []string) error {
	seen := make(map[string]string, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("output path is empty")
		}
		clean, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve output path %s: %w", path, err)
		}
		if previous, exists := seen[clean]; exists {
			return fmt.Errorf("output paths must be distinct: %s aliases %s", path, previous)
		}
		seen[clean] = path

		parent := filepath.Dir(path)
		info, err := os.Stat(parent)
		if err != nil {
			return fmt.Errorf("inspect output parent %s: %w", parent, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("output parent is not a directory: %s", parent)
		}
		probe, err := os.CreateTemp(parent, ".asc-output-preflight-*")
		if err != nil {
			return fmt.Errorf("probe output parent %s: %w", parent, err)
		}
		probePath := probe.Name()
		if err := probe.Close(); err != nil {
			_ = os.Remove(probePath)
			return fmt.Errorf("close output preflight probe %s: %w", parent, err)
		}
		if err := os.Remove(probePath); err != nil {
			return fmt.Errorf("remove output preflight probe %s: %w", parent, err)
		}
	}
	return nil
}

// ensureOutputPathsAreFree reports the first colliding output file. Writes use
// O_EXCL, so a collision always fails the command; detecting it up front keeps
// the failure free of remote and on-disk side effects.
func ensureOutputPathsAreFree(paths []string) error {
	if err := validateOutputPathStructure(paths); err != nil {
		return err
	}
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
