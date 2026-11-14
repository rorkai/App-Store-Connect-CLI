package signing

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

type signingResignCodePlan struct {
	Path             string
	EntitlementsPath string
	EntitlementsData []byte
}

type signingResignPreparedTree struct {
	Archive      signingResignArchive
	CodePlans    []signingResignCodePlan
	SwiftSupport []signingResignSwiftSupportEntry
}

type signingResignSwiftSupportEntry struct {
	RelativePath string
	SizeBytes    int64
	SHA256       string
	Mode         os.FileMode
}

// ErrSigningResignPublicationAmbiguous means the destination file was created
// but its post-publication validation did not complete successfully. Callers
// must inspect the reported artifact before retrying.
var ErrSigningResignPublicationAmbiguous = errors.New("re-signed IPA publication is ambiguous")

// signingResignBeforePublishedHashFn is a no-op production hook used by the
// package tests to make the post-publication cancellation boundary
// deterministic.
var signingResignBeforePublishedHashFn = func() {}

func executeSigningResignImplementation(ctx context.Context, options signingResignOptions) (result signingResignResult, resultErr error) {
	publicStage := signingResignStagePreparation
	publicCode := signingResignCodeFilesystem
	defer func() {
		if resultErr == nil || isSigningResignUsageError(resultErr) {
			return
		}
		resultErr = wrapSigningResignOperationalError(publicStage, publicCode, resultErr)
	}()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if runtime.GOOS != "darwin" {
		return result, fmt.Errorf("signing resign is supported only on macOS")
	}
	ctx, stopSignals := platformSigningRunContext(ctx)
	defer stopSignals()
	if err := validateSigningResignOptions(options); err != nil {
		return result, signingResignUsage(err)
	}

	inputPath, err := filepath.Abs(filepath.Clean(options.IPAPath))
	if err != nil {
		return result, fmt.Errorf("resolve IPA input: %w", err)
	}
	outputPath, err := filepath.Abs(filepath.Clean(options.OutputPath))
	if err != nil {
		return result, fmt.Errorf("resolve IPA output: %w", err)
	}
	manifestPath, err := filepath.Abs(filepath.Clean(options.ProfilesManifestPath))
	if err != nil {
		return result, fmt.Errorf("resolve profiles manifest: %w", err)
	}
	if filepath.Clean(inputPath) == filepath.Clean(outputPath) {
		return result, fmt.Errorf("IPA input and output must be different paths")
	}

	inputRoot, err := rootfs.New(filepath.Dir(inputPath))
	if err != nil {
		return result, fmt.Errorf("open IPA input directory: %w", err)
	}
	defer inputRoot.Close()
	source, err := inputRoot.OpenFile(filepath.Base(inputPath))
	if err != nil {
		return result, fmt.Errorf("open IPA input: %w", err)
	}
	defer source.Close()
	sourceInfo, err := source.Stat()
	if err != nil {
		return result, fmt.Errorf("inspect IPA input: %w", err)
	}

	outputRoot, err := rootfs.New(filepath.Dir(outputPath))
	if err != nil {
		return result, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactPublish,
			fmt.Errorf("open IPA output directory: %w", err),
		)
	}
	defer outputRoot.Close()
	if err := outputRoot.CheckCreateNewFile(filepath.Base(outputPath)); err != nil {
		return result, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactPublish,
			fmt.Errorf("preflight IPA output: %w", err),
		)
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 {
		return result, fmt.Errorf("IPA input is a symbolic link")
	}

	stageDir, err := os.MkdirTemp("", "asc-signing-resign.")
	if err != nil {
		return result, fmt.Errorf("create private re-signing directory: %w", err)
	}
	if err := os.Chmod(stageDir, 0o700); err != nil {
		_ = removeSigningResignStage(stageDir)
		return result, fmt.Errorf("secure private re-signing directory: %w", err)
	}
	defer func() {
		cleanupErr := removeSigningResignStage(stageDir)
		if cleanupErr == nil {
			return
		}
		// A cleanup failure after the artifact reached its create-only
		// destination must keep the publication visible to the caller, exactly
		// like the environment-cleanup-after-publication path below.
		published := result.Output.Path != "" || errors.Is(resultErr, ErrSigningResignPublicationAmbiguous)
		resultErr = errors.Join(resultErr, signingResignStageCleanupFailure(published, cleanupErr))
	}()

	stageRoot, err := rootfs.New(stageDir)
	if err != nil {
		return result, fmt.Errorf("open private re-signing directory: %w", err)
	}
	defer stageRoot.Close()
	stageOS, err := stageRoot.OpenRoot()
	if err != nil {
		return result, fmt.Errorf("open private re-signing root: %w", err)
	}
	defer stageOS.Close()
	if err := stageRoot.MkdirAll("tree", 0o700); err != nil {
		return result, fmt.Errorf("create private IPA staging tree: %w", err)
	}
	treeRoot, err := rootfs.New(filepath.Join(stageDir, "tree"))
	if err != nil {
		return result, fmt.Errorf("open private IPA staging tree: %w", err)
	}
	defer treeRoot.Close()
	treeOS, err := stageOS.OpenRoot("tree")
	if err != nil {
		return result, fmt.Errorf("open private IPA staging tree root: %w", err)
	}
	defer treeOS.Close()

	snapshot, inputDigest, err := snapshotSigningResignIPA(ctx, source, sourceInfo.Size(), stageOS)
	if err != nil {
		return result, err
	}
	defer snapshot.Close()
	snapshotInfo, err := snapshot.Stat()
	if err != nil {
		return result, fmt.Errorf("inspect IPA snapshot: %w", err)
	}
	if err := preflightSigningResignArchive(ctx, snapshot, snapshotInfo.Size()); err != nil {
		return result, err
	}
	archiveReader, err := zip.NewReader(snapshot, snapshotInfo.Size())
	if err != nil {
		return result, fmt.Errorf("read IPA archive: %w", err)
	}
	if err := validateSigningResignArchive(ctx, archiveReader); err != nil {
		return result, err
	}
	if err := materializeSigningResignArchive(ctx, archiveReader, treeOS); err != nil {
		return result, fmt.Errorf("materialize IPA: %w", err)
	}
	archive, err := discoverSigningResignArchive(ctx, archiveReader, treeRoot)
	if err != nil {
		return result, err
	}
	if err := validateSigningResignTargetIDs(archive.Targets); err != nil {
		return result, err
	}

	manifest, err := readSigningResignManifest(manifestPath)
	if err != nil {
		return result, err
	}
	targetIDs := buildSigningResignTargetIDs(archive.Targets)
	if err := validateSigningResignManifestTargets(manifest, targetIDs); err != nil {
		return result, err
	}
	profiles, err := readSigningResignProfiles(manifestPath, manifest)
	if err != nil {
		return result, err
	}
	for _, profile := range profiles {
		defer clear(profile.Data)
	}
	identityData, err := readBoundedSigningRunFile(options.IdentityPath, signingRunInputLimit, true)
	if err != nil {
		return result, fmt.Errorf("read signing identity failed")
	}
	defer clear(identityData)
	var passwordData []byte
	if strings.TrimSpace(options.IdentityPasswordPath) != "" {
		passwordData, err = readBoundedSigningRunFile(options.IdentityPasswordPath, signingRunPasswordLimit, true)
		if err != nil {
			return result, fmt.Errorf("read signing identity password failed")
		}
		defer clear(passwordData)
	}
	identityPassword := bytes.TrimSuffix(passwordData, []byte("\n"))
	identityPassword = bytes.TrimSuffix(identityPassword, []byte("\r"))
	identity, err := inspectSigningRunIdentity(identityData, identityPassword, signingRunNowFn())
	if err != nil {
		return result, fmt.Errorf("inspect signing identity: %w", err)
	}
	if err := validateSigningResignProfileSet(profiles, identity); err != nil {
		return result, err
	}
	prepared, err := prepareSigningResignTree(ctx, stageRoot, treeRoot, archive, profiles, options.RebaseTeamClaims)
	if err != nil {
		return result, err
	}

	teamID, err := signingRunCertificateTeamID(identity.Certificate)
	if err != nil {
		return result, err
	}
	result = signingResignResult{
		SchemaVersion: 1,
		Command:       "signing resign",
		Input: signingResignInputResult{
			SizeBytes: sourceInfo.Size(),
			SHA256:    strings.ToUpper(inputDigest),
		},
		Identity: signingResignIdentityResult{
			CertificateSHA256: identity.CertificateSHA256,
			TeamID:            teamID,
		},
		Verification: signingResignVerification{
			Status: "pending",
			Scope:  "complete-main-app-code-resources-entitlements-profile-and-certificate-binding",
		},
	}
	result.Targets = make([]signingResignTargetResult, 0, len(prepared.Archive.Targets))
	var entitlementRewrites []signingResignOutputEntitlementRewrite
	if options.RebaseTeamClaims {
		entitlementRewrites = make([]signingResignOutputEntitlementRewrite, 0)
		result.EntitlementRewrites = &entitlementRewrites
	}
	for _, target := range prepared.Archive.Targets {
		profile := profiles[target.BundleID]
		result.Targets = append(result.Targets, signingResignTargetResult{
			Kind: target.Kind, RelativePath: target.RelativePath, BundleID: target.BundleID,
			ProfileClass: profile.Class, ProfileUUID: profile.UUID, ProfileSHA256: profile.SHA256,
			Status: "pending",
		})
		if !options.RebaseTeamClaims {
			continue
		}
		for _, rewrite := range target.EntitlementRewrites {
			*result.EntitlementRewrites = append(*result.EntitlementRewrites, signingResignOutputEntitlementRewrite{
				TargetRelativePath: target.RelativePath,
				BundleID:           target.BundleID,
				Key:                rewrite.Key,
				ElementIndex:       rewrite.Index,
				From:               rewrite.From,
				To:                 rewrite.To,
			})
		}
	}
	if result.EntitlementRewrites != nil {
		sort.SliceStable(*result.EntitlementRewrites, func(left, right int) bool {
			return signingResignOutputEntitlementRewriteLess((*result.EntitlementRewrites)[left], (*result.EntitlementRewrites)[right])
		})
	}

	var outputArtifact signingResignArtifactResult
	publicStage = signingResignStageEnvironment
	publicCode = signingResignCodeEnvironment
	if err := runSigningResignEnvironment(ctx, identity, func(signingContext context.Context, keychainPath string) error {
		publicStage = signingResignStageSigning
		publicCode = signingResignCodeSigning
		if err := signSigningResignTree(signingContext, treeRoot.Path(), prepared, identity.CertificateSHA1, keychainPath); err != nil {
			return wrapSigningResignOperationalError(signingResignStageSigning, signingResignCodeSigning, err)
		}
		publicStage = signingResignStageVerification
		publicCode = signingResignCodeVerification
		if err := verifySigningResignTree(signingContext, treeRoot, prepared, teamID, identity.CertificateSHA256); err != nil {
			return wrapSigningResignOperationalError(signingResignStageVerification, signingResignCodeVerification, err)
		}
		publicStage = signingResignStageArtifact
		publicCode = signingResignCodeArtifactRead
		packedPath, packedSize, packedDigest, err := repackSigningResignTree(signingContext, stageRoot, treeRoot)
		if err != nil {
			return wrapSigningResignOperationalError(signingResignStageArtifact, signingResignCodeFilesystem, err)
		}
		if err := validatePackedSigningResignIPA(signingContext, packedPath, packedSize); err != nil {
			return wrapSigningResignOperationalError(signingResignStageVerification, signingResignCodeVerification, err)
		}
		if err := verifyPackedSigningResignIPA(signingContext, packedPath, packedSize, stageRoot, treeRoot.Path(), prepared, teamID, identity.CertificateSHA256); err != nil {
			return wrapSigningResignOperationalError(signingResignStageVerification, signingResignCodeVerification, err)
		}
		publicStage = signingResignStageArtifact
		publicCode = signingResignCodeArtifactPublish
		if err := outputRoot.MkdirAll(".", 0o755); err != nil {
			return wrapSigningResignOperationalError(
				signingResignStageArtifact,
				signingResignCodeArtifactPublish,
				fmt.Errorf("create IPA output directory: %w", err),
			)
		}
		if err := outputRoot.CheckCreateNewFile(filepath.Base(outputPath)); err != nil {
			return wrapSigningResignOperationalError(
				signingResignStageArtifact,
				signingResignCodeArtifactPublish,
				fmt.Errorf("preflight IPA output: %w", err),
			)
		}
		outputArtifact, err = publishSigningResignOutput(signingContext, outputRoot, filepath.Base(outputPath), packedPath, packedSize, packedDigest)
		return err
	}); err != nil {
		if outputArtifact.Path != "" {
			return result, fmt.Errorf("%w: re-signed IPA was published but environment cleanup failed: %w", ErrSigningResignPublicationAmbiguous, err)
		}
		return result, err
	}
	result.Output = outputArtifact
	result.Verification.Status = "verified"
	for index := range result.Targets {
		result.Targets[index].Status = "verified"
	}
	return result, nil
}

// signingResignOutputEntitlementRewriteLess is the canonical receipt order.
// It intentionally does not depend on archive discovery order or Go map
// iteration: target path, bundle identifier, allowlist key rank, scalar vs
// array element, element index, and canonical old/new values are compared in
// that order.
func signingResignOutputEntitlementRewriteLess(first, second signingResignOutputEntitlementRewrite) bool {
	if first.TargetRelativePath != second.TargetRelativePath {
		return first.TargetRelativePath < second.TargetRelativePath
	}
	if first.BundleID != second.BundleID {
		return first.BundleID < second.BundleID
	}
	firstRank, secondRank := signingResignEntitlementRewriteKeyRank(first.Key), signingResignEntitlementRewriteKeyRank(second.Key)
	if firstRank != secondRank {
		return firstRank < secondRank
	}
	if first.Key != second.Key {
		return first.Key < second.Key
	}
	if (first.ElementIndex == nil) != (second.ElementIndex == nil) {
		return first.ElementIndex == nil
	}
	if first.ElementIndex != nil && second.ElementIndex != nil && *first.ElementIndex != *second.ElementIndex {
		return *first.ElementIndex < *second.ElementIndex
	}
	firstFrom, secondFrom := signingResignRewriteValueSortKey(first.From), signingResignRewriteValueSortKey(second.From)
	if firstFrom != secondFrom {
		return firstFrom < secondFrom
	}
	return signingResignRewriteValueSortKey(first.To) < signingResignRewriteValueSortKey(second.To)
}

func signingResignEntitlementRewriteKeyRank(key string) int {
	switch key {
	case signingResignKeychainGroupsEntitlement:
		return 0
	case signingResignKVStoreEntitlement:
		return 1
	case signingResignParentEntitlement:
		return 2
	default:
		return 3
	}
}

func signingResignRewriteValueSortKey(value any) string {
	data, err := json.Marshal(value)
	if err == nil {
		return string(data)
	}
	return fmt.Sprintf("%T:%v", value, value)
}

func validateSigningResignOptions(options signingResignOptions) error {
	required := []struct {
		label string
		value string
	}{
		{label: "IPA input", value: options.IPAPath},
		{label: "IPA output", value: options.OutputPath},
		{label: "signing identity", value: options.IdentityPath},
		{label: "profiles manifest", value: options.ProfilesManifestPath},
	}
	for _, item := range required {
		if strings.TrimSpace(item.value) == "" {
			return fmt.Errorf("%s is required", item.label)
		}
		if strings.ContainsRune(item.value, 0) {
			return fmt.Errorf("%s contains a NUL byte", item.label)
		}
	}
	if strings.ContainsRune(options.IdentityPasswordPath, 0) {
		return fmt.Errorf("identity password path contains a NUL byte")
	}
	return nil
}

func prepareSigningResignTree(ctx context.Context, stageRoot, treeRoot rootfs.Root, archive signingResignArchive, profiles map[string]signingResignProfile, rebaseTeamClaims ...bool) (signingResignPreparedTree, error) {
	strictNested := len(rebaseTeamClaims) > 0 && rebaseTeamClaims[0]
	if err := ctx.Err(); err != nil {
		return signingResignPreparedTree{}, err
	}
	for _, target := range archive.Targets {
		if err := validateSigningResignExistingEntitlements(target.ExistingEntitlements, target.BundleID); err != nil {
			return signingResignPreparedTree{}, fmt.Errorf("target %s existing entitlements: %w", target.BundleID, err)
		}
	}
	prepared := signingResignPreparedTree{Archive: archive}
	prepared.Archive.Targets = append([]signingResignTarget(nil), archive.Targets...)
	rebase := len(rebaseTeamClaims) > 0 && rebaseTeamClaims[0]
	targetPlans, err := planSigningResignEntitlements(archive, profiles, rebase)
	if err != nil {
		return signingResignPreparedTree{}, err
	}

	codePaths, err := enumerateSigningResignMachOFiles(ctx, treeRoot.Path())
	if err != nil {
		return signingResignPreparedTree{}, fmt.Errorf("enumerate Mach-O code failed")
	}
	mainPrefix := filepath.Join(treeRoot.Path(), filepath.FromSlash(prepared.Archive.MainPath)) + string(filepath.Separator)
	targetExecutablePaths := make(map[string]struct{}, len(prepared.Archive.Targets))
	for _, target := range prepared.Archive.Targets {
		targetExecutablePaths[targetExecutablePath(treeRoot.Path(), target)] = struct{}{}
	}
	if err := validateSigningResignPreservedExternalDirectories(ctx, treeRoot.Path()); err != nil {
		return signingResignPreparedTree{}, err
	}
	prepared.SwiftSupport, err = captureSigningResignPreservedInventory(ctx, treeRoot.Path())
	if err != nil {
		return signingResignPreparedTree{}, fmt.Errorf("capture preserved support inventory: %w", err)
	}
	codePlans := make([]signingResignCodePlan, 0, len(codePaths))
	for _, codePath := range codePaths {
		if err := ctx.Err(); err != nil {
			return signingResignPreparedTree{}, err
		}
		if !strings.HasPrefix(codePath, mainPrefix) {
			if isSigningResignPreservedExternalCodePath(treeRoot.Path(), codePath) {
				// SwiftSupport/iphoneos contains Apple-supplied Swift runtime
				// libraries that are distributed beside the app payload. They
				// were provenance-checked as a complete directory above and
				// remain byte-for-byte untouched.
				continue
			}
			return signingResignPreparedTree{}, fmt.Errorf("Mach-O code exists outside the main app")
		}
		if _, isTargetExecutable := targetExecutablePaths[filepath.Clean(codePath)]; isTargetExecutable {
			continue
		}
		target, ok := signingResignTargetForCodePath(prepared.Archive.Targets, treeRoot.Path(), codePath)
		if !ok {
			return signingResignPreparedTree{}, fmt.Errorf("Mach-O code is not contained by an app-like target")
		}
		if err := validateSigningResignNestedExecutableMode(ctx, treeRoot, codePath); err != nil {
			return signingResignPreparedTree{}, err
		}
		entitlements, err := readSigningResignEntitlements(ctx, codePath)
		if err != nil {
			return signingResignPreparedTree{}, fmt.Errorf("read nested code entitlements failed")
		}
		if err := validateSigningResignNestedEntitlements(entitlements, profiles[target.BundleID].Entitlements, strictNested); err != nil {
			displayPath, _ := filepath.Rel(treeRoot.Path(), codePath)
			return signingResignPreparedTree{}, fmt.Errorf("nested code %s entitlements: %w", filepath.ToSlash(displayPath), err)
		}
		var entitlementsData []byte
		if len(entitlements) > 0 {
			entitlementsData, err = marshalSigningResignEntitlements(entitlements)
			if err != nil {
				return signingResignPreparedTree{}, err
			}
		}
		codePlans = append(codePlans, signingResignCodePlan{Path: codePath, EntitlementsData: entitlementsData})
	}

	// All target and nested-code decisions above are read-only. Only after the
	// entire IPA has a valid plan do we create generated documents, embed
	// profiles, and publish the plan into the mutable staging tree.
	if err := stageRoot.MkdirAll("entitlements", 0o700); err != nil {
		return signingResignPreparedTree{}, fmt.Errorf("create private entitlements directory failed")
	}
	for index := range prepared.Archive.Targets {
		target := &prepared.Archive.Targets[index]
		profile := profiles[target.BundleID]
		entitlementsName := filepath.Join("entitlements", fmt.Sprintf("target-%03d.plist", index))
		if err := stageRoot.WriteFile(entitlementsName, targetPlans[index].EntitlementsData, 0o600); err != nil {
			return signingResignPreparedTree{}, fmt.Errorf("write target %s entitlements failed", target.BundleID)
		}
		profileName := filepath.FromSlash(path.Join(target.RelativePath, "embedded.mobileprovision"))
		profileMode := target.ProfileMode
		if profileMode == 0 {
			profileMode = 0o644
		}
		if err := treeRoot.WriteFile(profileName, profile.Data, profileMode); err != nil {
			return signingResignPreparedTree{}, fmt.Errorf("embed profile for target %s failed", target.BundleID)
		}
		target.Profile = profile
		target.EntitlementRewrites = append([]signingResignEntitlementRewrite(nil), targetPlans[index].Rewrites...)
		target.EntitlementsPath = filepath.Join(stageRoot.Path(), entitlementsName)
	}
	for index := range codePlans {
		if len(codePlans[index].EntitlementsData) == 0 {
			continue
		}
		name := filepath.Join("entitlements", fmt.Sprintf("code-%06d.plist", index))
		if err := stageRoot.WriteFile(name, codePlans[index].EntitlementsData, 0o600); err != nil {
			return signingResignPreparedTree{}, fmt.Errorf("write nested code entitlements failed")
		}
		codePlans[index].EntitlementsPath = filepath.Join(stageRoot.Path(), name)
	}
	prepared.CodePlans = codePlans
	return prepared, nil
}

func validateSigningResignNestedExecutableMode(ctx context.Context, tree rootfs.Root, codePath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	relative, err := filepath.Rel(tree.Path(), codePath)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("nested executable is outside the staging tree")
	}
	file, err := tree.OpenFile(relative)
	if err != nil {
		return fmt.Errorf("inspect nested executable mode: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect nested executable mode: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("nested executable is not a regular file")
	}
	if info.Mode().Perm()&0o100 == 0 {
		return fmt.Errorf("nested executable file mode is missing the owner-execute permission")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func isSigningResignPreservedExternalCodePath(treeRoot, codePath string) bool {
	relative, err := filepath.Rel(treeRoot, codePath)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	return isSigningResignPreservedExternalCodeRelativePath(filepath.ToSlash(relative))
}

func isSigningResignPreservedExternalCodeRelativePath(relative string) bool {
	if relative == "WatchKitSupport2/WK" {
		// App Store-exported Watch IPAs carry the distribution-side WK shim
		// binary beside the payload. It is provenance-checked and preserved
		// byte-for-byte, never re-signed.
		return true
	}
	const prefix = "SwiftSupport/iphoneos/"
	if !strings.HasPrefix(relative, prefix) {
		return false
	}
	name := strings.TrimPrefix(relative, prefix)
	return name != "" && !strings.ContainsRune(name, '/') && name != ".dylib" && strings.HasSuffix(name, ".dylib")
}

func verifySigningResignPreservedExternalCode(ctx context.Context, codePath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := runSigningResignToolFn(ctx, "/usr/bin/codesign", "--verify", "--strict", "--all-architectures", "-R=anchor apple generic", codePath); err != nil {
		return err
	}
	return nil
}

// verifySigningResignPreservedExternalCodeOpen verifies the bytes already
// selected by a rooted descriptor. codesign cannot consume an fd on macOS,
// so copy the descriptor into a private temporary regular file first.
func verifySigningResignPreservedExternalCodeOpen(ctx context.Context, source *os.File, tempRoot string) (resultErr error) {
	if source == nil {
		return fmt.Errorf("preserved code descriptor is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tempOwner, err := rootfs.New(tempRoot)
	if err != nil {
		return err
	}
	defer tempOwner.Close()
	tempDirectory, err := tempOwner.OpenRoot()
	if err != nil {
		return err
	}
	defer tempDirectory.Close()
	temp, tempName, err := secureopen.CreateTempNoFollowInRoot(tempDirectory, ".", ".signing-resign-verify-*", 0o600)
	if err != nil {
		return err
	}
	name := filepath.Join(tempOwner.Path(), tempName)
	defer func() {
		if cleanupErr := tempDirectory.Remove(tempName); cleanupErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove temporary provenance copy: %w", cleanupErr))
		}
	}()
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		temp.Close()
		return err
	}
	var copied int64
	expectedHash := sha256.New()
	buf := make([]byte, 128*1024)
	for {
		if err := ctx.Err(); err != nil {
			temp.Close()
			return err
		}
		n, readErr := source.Read(buf)
		if n > 0 {
			copied += int64(n)
			if copied > signingResignSwiftSupportMaxBytes {
				temp.Close()
				return fmt.Errorf("preserved code exceeds %d bytes", signingResignSwiftSupportMaxBytes)
			}
			written, err := temp.Write(buf[:n])
			if err != nil {
				temp.Close()
				return err
			}
			if written != n {
				temp.Close()
				return io.ErrShortWrite
			}
			if _, err := expectedHash.Write(buf[:n]); err != nil {
				temp.Close()
				return err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			temp.Close()
			return readErr
		}
	}
	tempInfo, err := temp.Stat()
	if err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	retained, err := secureopen.OpenExistingNoFollowInRoot(tempDirectory, tempName)
	if err != nil {
		return err
	}
	defer retained.Close()
	before, err := tempDirectory.Lstat(tempName)
	if err != nil || !before.Mode().IsRegular() || !os.SameFile(tempInfo, before) || before.Size() != copied {
		return fmt.Errorf("temporary provenance copy changed before verification")
	}
	expectedDigest := strings.ToUpper(hex.EncodeToString(expectedHash.Sum(nil)))
	beforeDigest, err := hashSigningResignOpenFile(ctx, retained, copied)
	if err != nil || !strings.EqualFold(beforeDigest, expectedDigest) {
		return errors.Join(err, fmt.Errorf("temporary provenance copy changed before verification"))
	}
	if err := checkSigningResignRootIdentity(tempOwner); err != nil {
		return fmt.Errorf("temporary provenance directory changed before verification: %w", err)
	}
	toolErr := verifySigningResignPreservedExternalCode(ctx, name)
	after, pathErr := tempDirectory.Lstat(tempName)
	retainedInfo, retainedErr := retained.Stat()
	afterDigest, digestErr := hashSigningResignOpenFile(ctx, retained, copied)
	rootErr := checkSigningResignRootIdentity(tempOwner)
	if pathErr != nil || retainedErr != nil || digestErr != nil || rootErr != nil || !after.Mode().IsRegular() ||
		!os.SameFile(tempInfo, after) || !os.SameFile(tempInfo, retainedInfo) || after.Size() != copied || retainedInfo.Size() != copied {
		return errors.Join(toolErr, pathErr, retainedErr, digestErr, rootErr, fmt.Errorf("temporary provenance copy changed during verification"))
	}
	if !strings.EqualFold(afterDigest, expectedDigest) {
		return errors.Join(toolErr, fmt.Errorf("temporary provenance copy changed during verification"))
	}
	return toolErr
}

func checkSigningResignRootIdentity(root rootfs.Root) error {
	opened, err := root.OpenRoot()
	if err != nil {
		return err
	}
	return opened.Close()
}

func openSigningResignRegularNoFollow(root *os.Root, name string) (*os.File, os.FileInfo, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("entry is not a regular file")
	}
	file, err := secureopen.OpenExistingNoFollowInRoot(root, name)
	if err != nil {
		return nil, nil, err
	}
	opened, statErr := file.Stat()
	latest, lstatErr := root.Lstat(name)
	if statErr != nil || lstatErr != nil || !opened.Mode().IsRegular() ||
		latest.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, opened) || !os.SameFile(opened, latest) {
		return nil, nil, errors.Join(statErr, lstatErr, fmt.Errorf("regular file changed during rooted open"), file.Close())
	}
	return file, opened, nil
}

func openSigningResignTreeRoot(treeRoot string) (rootfs.Root, *os.Root, error) {
	root, err := rootfs.New(treeRoot)
	if err != nil {
		return rootfs.Root{}, nil, err
	}
	opened, err := root.OpenRoot()
	if err != nil {
		root.Close()
		return rootfs.Root{}, nil, err
	}
	return root, opened, nil
}

func validateSigningResignSwiftSupport(ctx context.Context, treeRoot string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	owner, root, err := openSigningResignTreeRoot(treeRoot)
	if err != nil {
		return fmt.Errorf("open staging tree: %w", err)
	}
	defer root.Close()
	defer owner.Close()
	return validateSigningResignSwiftSupportRoot(ctx, treeRoot, root)
}

func validateSigningResignSwiftSupportRoot(ctx context.Context, treeRoot string, root *os.Root) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	before, statErr := root.Lstat("SwiftSupport")
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect SwiftSupport directory: %w", statErr)
	}
	if statErr == nil && before.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("SwiftSupport directory is a symbolic link")
	}
	swift, err := root.OpenRoot("SwiftSupport")
	if errors.Is(err, os.ErrNotExist) {
		if before == nil {
			return nil
		}
		return fmt.Errorf("SwiftSupport directory disappeared during rooted open")
	}
	if err != nil {
		return fmt.Errorf("inspect SwiftSupport directory: %w", err)
	}
	defer swift.Close()
	if info, statErr := root.Lstat("SwiftSupport"); statErr != nil || info.Mode()&os.ModeSymlink != 0 || before == nil || !os.SameFile(before, info) {
		if statErr != nil {
			return fmt.Errorf("inspect SwiftSupport directory: %w", statErr)
		}
		return fmt.Errorf("SwiftSupport directory changed during rooted open")
	}
	if after, statErr := swift.Stat("."); statErr != nil || !os.SameFile(before, after) {
		return fmt.Errorf("SwiftSupport directory changed during rooted open")
	}
	swiftDir, err := swift.Open(".")
	if err != nil {
		return fmt.Errorf("read SwiftSupport directory: %w", err)
	}
	defer swiftDir.Close()
	entries, err := swiftDir.ReadDir(-1)
	if err != nil {
		return fmt.Errorf("read SwiftSupport directory: %w", err)
	}
	if len(entries) != 1 || entries[0].Name() != "iphoneos" {
		return fmt.Errorf("SwiftSupport must contain only the iphoneos directory")
	}
	beforePlatform, statErr := swift.Lstat("iphoneos")
	if statErr != nil {
		return fmt.Errorf("inspect SwiftSupport/iphoneos directory: %w", statErr)
	}
	if beforePlatform.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("SwiftSupport/iphoneos directory is a symbolic link")
	}
	platform, err := swift.OpenRoot("iphoneos")
	if err != nil {
		return fmt.Errorf("inspect SwiftSupport/iphoneos directory: %w", err)
	}
	defer platform.Close()
	if info, statErr := swift.Lstat("iphoneos"); statErr != nil || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(beforePlatform, info) {
		if statErr != nil {
			return fmt.Errorf("inspect SwiftSupport/iphoneos directory: %w", statErr)
		}
		return fmt.Errorf("SwiftSupport/iphoneos directory changed during rooted open")
	}
	if after, statErr := platform.Stat("."); statErr != nil || !os.SameFile(beforePlatform, after) {
		return fmt.Errorf("SwiftSupport/iphoneos directory changed during rooted open")
	}
	platformDir, err := platform.Open(".")
	if err != nil {
		return fmt.Errorf("read SwiftSupport/iphoneos directory: %w", err)
	}
	defer platformDir.Close()
	entries, err = platformDir.ReadDir(-1)
	if err != nil {
		return fmt.Errorf("read SwiftSupport/iphoneos directory: %w", err)
	}
	// Complete structural validation before invoking codesign. Directory
	// enumeration order is filesystem-specific, so an invalid later entry must
	// not be hidden by an earlier unsigned dylib verification failure.
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect SwiftSupport/iphoneos entry: %w", err)
		}
		if entryInfo.Mode()&os.ModeSymlink != 0 || !entryInfo.Mode().IsRegular() {
			return fmt.Errorf("SwiftSupport/iphoneos contains a nested or symbolic-link entry")
		}
		name := entry.Name()
		if name == ".dylib" || !strings.HasSuffix(name, ".dylib") {
			return fmt.Errorf("SwiftSupport/iphoneos contains an unsupported entry")
		}
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := entry.Name()
		file, _, err := openSigningResignRegularNoFollow(platform, name)
		if err != nil {
			return fmt.Errorf("SwiftSupport/iphoneos contains a nested or symbolic-link entry: %w", err)
		}
		if err := verifySigningResignPreservedExternalCodeOpen(ctx, file, treeRoot); err != nil {
			file.Close()
			return fmt.Errorf("verify preserved SwiftSupport code failed: %w", err)
		}
		file.Close()
	}
	return nil
}

// validateSigningResignWatchKitSupport enforces the standard Watch
// distribution layout: an optional WatchKitSupport2 directory containing
// exactly the regular, non-symlink WK binary, whose Apple provenance is
// verified before it is preserved.
func validateSigningResignWatchKitSupport(ctx context.Context, treeRoot string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	owner, root, err := openSigningResignTreeRoot(treeRoot)
	if err != nil {
		return fmt.Errorf("open staging tree: %w", err)
	}
	defer root.Close()
	defer owner.Close()
	return validateSigningResignWatchKitSupportRoot(ctx, treeRoot, root)
}

func validateSigningResignWatchKitSupportRoot(ctx context.Context, treeRoot string, root *os.Root) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	before, statErr := root.Lstat("WatchKitSupport2")
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect WatchKitSupport2 directory: %w", statErr)
	}
	if statErr == nil && before.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("WatchKitSupport2 directory is a symbolic link")
	}
	watch, err := root.OpenRoot("WatchKitSupport2")
	if errors.Is(err, os.ErrNotExist) {
		if before == nil {
			return nil
		}
		return fmt.Errorf("WatchKitSupport2 directory disappeared during rooted open")
	}
	if err != nil {
		return fmt.Errorf("inspect WatchKitSupport2 directory: %w", err)
	}
	defer watch.Close()
	if info, statErr := root.Lstat("WatchKitSupport2"); statErr != nil || info.Mode()&os.ModeSymlink != 0 || before == nil || !os.SameFile(before, info) {
		if statErr != nil {
			return fmt.Errorf("inspect WatchKitSupport2 directory: %w", statErr)
		}
		return fmt.Errorf("WatchKitSupport2 directory changed during rooted open")
	}
	if after, statErr := watch.Stat("."); statErr != nil || !os.SameFile(before, after) {
		return fmt.Errorf("WatchKitSupport2 directory changed during rooted open")
	}
	watchDir, err := watch.Open(".")
	if err != nil {
		return fmt.Errorf("read WatchKitSupport2 directory: %w", err)
	}
	defer watchDir.Close()
	entries, err := watchDir.ReadDir(-1)
	if err != nil {
		return fmt.Errorf("read WatchKitSupport2 directory: %w", err)
	}
	if len(entries) != 1 || entries[0].Name() != "WK" {
		return fmt.Errorf("WatchKitSupport2 must contain only the WK binary")
	}
	if entries[0].Type()&os.ModeSymlink != 0 {
		return fmt.Errorf("WatchKitSupport2 contains a nested or symbolic-link entry")
	}
	file, info, err := openSigningResignRegularNoFollow(watch, "WK")
	if err != nil {
		return fmt.Errorf("WatchKitSupport2 contains a nested or symbolic-link entry: %w", err)
	}
	defer file.Close()
	if info.Mode().Perm()&0o100 == 0 {
		return fmt.Errorf("WatchKitSupport2/WK is missing the owner-execute permission")
	}
	if err := verifySigningResignPreservedExternalCodeOpen(ctx, file, treeRoot); err != nil {
		return fmt.Errorf("verify preserved WatchKitSupport2 code failed: %w", err)
	}
	return nil
}

func captureSigningResignWatchKitSupportInventoryRoot(ctx context.Context, root *os.Root) ([]signingResignSwiftSupportEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	before, statErr := root.Lstat("WatchKitSupport2")
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect WatchKitSupport2 directory: %w", statErr)
	}
	if statErr == nil && before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("WatchKitSupport2 directory is a symbolic link")
	}
	watch, err := root.OpenRoot("WatchKitSupport2")
	if errors.Is(err, os.ErrNotExist) {
		if before == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("WatchKitSupport2 directory disappeared during rooted open")
	}
	if err != nil {
		return nil, fmt.Errorf("inspect WatchKitSupport2 directory: %w", err)
	}
	defer watch.Close()
	if after, statErr := watch.Stat("."); statErr != nil || before == nil || !os.SameFile(before, after) {
		return nil, fmt.Errorf("WatchKitSupport2 directory changed during rooted open")
	}
	file, entryInfo, err := openSigningResignRegularNoFollow(watch, "WK")
	if err != nil {
		return nil, fmt.Errorf("inspect WatchKitSupport2 entry: %w", err)
	}
	defer file.Close()
	if entryInfo.Size() > signingResignSwiftSupportMaxBytes {
		return nil, fmt.Errorf("WatchKitSupport2 entry exceeds %d bytes", signingResignSwiftSupportMaxBytes)
	}
	digest, err := hashSigningResignOpenFile(ctx, file, entryInfo.Size())
	if err != nil {
		return nil, fmt.Errorf("hash WatchKitSupport2 entry: %w", err)
	}
	return []signingResignSwiftSupportEntry{{
		RelativePath: "WatchKitSupport2/WK",
		SizeBytes:    entryInfo.Size(),
		SHA256:       digest,
		Mode:         entryInfo.Mode().Perm(),
	}}, nil
}

// validateSigningResignPreservedExternalDirectories checks every supported
// distribution-side directory that is preserved instead of re-signed.
func validateSigningResignPreservedExternalDirectories(ctx context.Context, treeRoot string) error {
	owner, root, err := openSigningResignTreeRoot(treeRoot)
	if err != nil {
		return fmt.Errorf("open staging tree: %w", err)
	}
	defer root.Close()
	defer owner.Close()
	if err := validateSigningResignSwiftSupportRoot(ctx, treeRoot, root); err != nil {
		return err
	}
	return validateSigningResignWatchKitSupportRoot(ctx, treeRoot, root)
}

// captureSigningResignPreservedInventory records the sorted path, size,
// digest, and mode of every preserved distribution-side runtime so repack can
// be held to byte-for-byte equality.
func captureSigningResignPreservedInventory(ctx context.Context, treeRoot string) ([]signingResignSwiftSupportEntry, error) {
	owner, root, err := openSigningResignTreeRoot(treeRoot)
	if err != nil {
		return nil, fmt.Errorf("open staging tree: %w", err)
	}
	defer root.Close()
	defer owner.Close()
	swift, err := captureSigningResignSwiftSupportInventoryRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	watch, err := captureSigningResignWatchKitSupportInventoryRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	combined := append(swift, watch...)
	sort.Slice(combined, func(left, right int) bool {
		return combined[left].RelativePath < combined[right].RelativePath
	})
	if len(combined) == 0 {
		return nil, nil
	}
	return combined, nil
}

func captureSigningResignPreservedInventoryRoot(ctx context.Context, root *os.Root) ([]signingResignSwiftSupportEntry, error) {
	swift, err := captureSigningResignSwiftSupportInventoryRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	watch, err := captureSigningResignWatchKitSupportInventoryRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	combined := append(swift, watch...)
	sort.Slice(combined, func(i, j int) bool { return combined[i].RelativePath < combined[j].RelativePath })
	if len(combined) == 0 {
		return nil, nil
	}
	return combined, nil
}

func captureSigningResignSwiftSupportInventory(ctx context.Context, treeRoot string) ([]signingResignSwiftSupportEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	owner, root, err := openSigningResignTreeRoot(treeRoot)
	if err != nil {
		return nil, fmt.Errorf("open staging tree: %w", err)
	}
	defer root.Close()
	defer owner.Close()
	return captureSigningResignSwiftSupportInventoryRoot(ctx, root)
}

func captureSigningResignSwiftSupportInventoryRoot(ctx context.Context, root *os.Root) ([]signingResignSwiftSupportEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	before, statErr := root.Lstat("SwiftSupport")
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect SwiftSupport directory: %w", statErr)
	}
	if statErr == nil && before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("SwiftSupport directory is a symbolic link")
	}
	swift, err := root.OpenRoot("SwiftSupport")
	if errors.Is(err, os.ErrNotExist) {
		if before == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("SwiftSupport directory disappeared during rooted open")
	}
	if err != nil {
		return nil, fmt.Errorf("inspect SwiftSupport directory: %w", err)
	}
	defer swift.Close()
	if after, statErr := swift.Stat("."); statErr != nil || before == nil || !os.SameFile(before, after) {
		return nil, fmt.Errorf("SwiftSupport directory changed during rooted open")
	}
	platformBefore, statErr := swift.Lstat("iphoneos")
	if statErr != nil {
		return nil, fmt.Errorf("inspect SwiftSupport/iphoneos directory: %w", statErr)
	}
	if statErr == nil && platformBefore.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("SwiftSupport/iphoneos directory is a symbolic link")
	}
	platform, err := swift.OpenRoot("iphoneos")
	if err != nil {
		return nil, fmt.Errorf("inspect SwiftSupport/iphoneos directory: %w", err)
	}
	defer platform.Close()
	if after, statErr := platform.Stat("."); statErr != nil || platformBefore == nil || !os.SameFile(platformBefore, after) {
		return nil, fmt.Errorf("SwiftSupport/iphoneos directory changed during rooted open")
	}
	dir, err := platform.Open(".")
	if err != nil {
		return nil, fmt.Errorf("read SwiftSupport/iphoneos directory: %w", err)
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("read SwiftSupport/iphoneos directory: %w", err)
	}
	inventory := make([]signingResignSwiftSupportEntry, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, entryInfo, err := openSigningResignRegularNoFollow(platform, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("SwiftSupport/iphoneos contains a non-regular entry: %w", err)
		}
		if entryInfo.Size() > signingResignSwiftSupportMaxBytes {
			file.Close()
			return nil, fmt.Errorf("SwiftSupport entry exceeds %d bytes", signingResignSwiftSupportMaxBytes)
		}
		digest, err := hashSigningResignOpenFile(ctx, file, entryInfo.Size())
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			return nil, fmt.Errorf("hash SwiftSupport entry: %w", errors.Join(err, closeErr))
		}
		inventory = append(inventory, signingResignSwiftSupportEntry{
			RelativePath: filepath.ToSlash(filepath.Join("SwiftSupport", "iphoneos", entry.Name())),
			SizeBytes:    entryInfo.Size(),
			SHA256:       digest,
			Mode:         entryInfo.Mode().Perm(),
		})
	}
	sort.Slice(inventory, func(left, right int) bool {
		return inventory[left].RelativePath < inventory[right].RelativePath
	})
	return inventory, nil
}

func signingResignTargetForCodePath(targets []signingResignTarget, treeRoot, codePath string) (signingResignTarget, bool) {
	var selected signingResignTarget
	selectedLength := -1
	for _, target := range targets {
		prefix := filepath.Join(treeRoot, filepath.FromSlash(target.RelativePath)) + string(filepath.Separator)
		if strings.HasPrefix(codePath, prefix) && len(prefix) > selectedLength {
			selected = target
			selectedLength = len(prefix)
		}
	}
	return selected, selectedLength >= 0
}

func validateSigningResignNestedEntitlements(entitlements, profile map[string]any, strict ...bool) error {
	for key, value := range entitlements {
		if _, identityKey := signingResignIdentityEntitlementKeys[key]; identityKey {
			return fmt.Errorf("identity entitlement %s is not allowed on nested non-app code", key)
		}
		profileValue, exists := profile[key]
		permitted := exists && signingResignEntitlementValuePermits(profileValue, value)
		if len(strict) > 0 && strict[0] {
			permitted = exists && signingResignStrictEntitlementValuePermits(profileValue, value)
		}
		if !permitted {
			return fmt.Errorf("entitlement %s is not permitted by its target profile", key)
		}
	}
	return nil
}

func signSigningResignTree(ctx context.Context, treePath string, prepared signingResignPreparedTree, identitySHA1, keychainPath string) (resultErr error) {
	defer func() {
		resultErr = wrapSigningResignOperationalError(
			signingResignStageSigning,
			signingResignCodeSigning,
			resultErr,
		)
	}()
	plans := append([]signingResignCodePlan(nil), prepared.CodePlans...)
	sortSigningResignCodePlans(plans)
	targetExecutablePaths := make(map[string]struct{}, len(prepared.Archive.Targets))
	for _, target := range prepared.Archive.Targets {
		targetExecutablePaths[targetExecutablePath(treePath, target)] = struct{}{}
	}
	for _, plan := range plans {
		if _, targetExecutable := targetExecutablePaths[filepath.Clean(plan.Path)]; targetExecutable {
			continue
		}
		if err := signSigningResignObject(ctx, plan.Path, identitySHA1, keychainPath, plan.EntitlementsPath); err != nil {
			return fmt.Errorf("sign nested code %s: %w", signingResignDisplayPath(treePath, plan.Path), err)
		}
	}
	containers := signingResignFrameworkContainers(treePath, plans)
	for _, container := range containers {
		entitlementsPath := signingResignContainerEntitlementsPath(treePath, container, plans)
		if err := signSigningResignObject(ctx, container, identitySHA1, keychainPath, entitlementsPath); err != nil {
			return fmt.Errorf("sign code container %s: %w", signingResignDisplayPath(treePath, container), err)
		}
	}
	for _, target := range prepared.Archive.Targets {
		targetPath := filepath.Join(treePath, filepath.FromSlash(target.RelativePath))
		if err := signSigningResignObject(ctx, targetPath, identitySHA1, keychainPath, target.EntitlementsPath); err != nil {
			return fmt.Errorf("sign target %s: %w", target.BundleID, err)
		}
	}
	return nil
}

// signingResignContainerEntitlementsPath returns the prepared entitlements
// for a container's main executable. A container is signed after its contents,
// so passing the same document preserves the claims applied to that
// executable when the container's resource seal is refreshed.
func signingResignContainerEntitlementsPath(treePath, container string, plans []signingResignCodePlan) string {
	relativeContainer, err := filepath.Rel(treePath, container)
	if err != nil || strings.HasPrefix(relativeContainer, ".."+string(filepath.Separator)) {
		return ""
	}
	infoData, err := readRootedSigningResignFile(treePath, filepath.Join(relativeContainer, "Info.plist"), infoplist.MaxBytes)
	if err != nil {
		return ""
	}
	if err := infoplist.ValidateStructure(infoData); err != nil {
		return ""
	}
	var info struct {
		Executable string `plist:"CFBundleExecutable"`
	}
	if _, err := plist.Unmarshal(infoData, &info); err != nil || strings.TrimSpace(info.Executable) == "" {
		return ""
	}
	versionedEntitlements := ""
	versionedFound := false
	for _, plan := range plans {
		if filepath.Base(plan.Path) != info.Executable {
			continue
		}
		relative, err := filepath.Rel(container, plan.Path)
		if err != nil || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		relativeSlash := filepath.ToSlash(relative)
		parts := strings.Split(relativeSlash, "/")
		if filepath.Dir(relative) == "." {
			return plan.EntitlementsPath
		}
		if len(parts) == 3 && parts[0] == "Versions" && parts[2] == info.Executable && !versionedFound {
			versionedEntitlements = plan.EntitlementsPath
			versionedFound = true
		}
	}
	return versionedEntitlements
}

// readRootedSigningResignFile reads a bounded file beneath the selected
// staging root without reopening an untrusted path outside that root.
func readRootedSigningResignFile(rootPath, relativePath string, limit int64) ([]byte, error) {
	root, err := rootfs.New(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.ReadFileLimited(relativePath, limit)
}

// isSigningResignCodeContainerName reports whether a directory name is a
// supported nested code container whose signature must be refreshed after the
// code inside it changes. App-like bundles (.app, .appex) are signed as
// discovered targets instead.
func isSigningResignCodeContainerName(name string) bool {
	for _, suffix := range []string{".framework", ".bundle", ".xpc"} {
		if name != suffix && strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func sortSigningResignCodePlans(plans []signingResignCodePlan) {
	sort.Slice(plans, func(left, right int) bool {
		leftDepth := strings.Count(plans[left].Path, string(filepath.Separator))
		rightDepth := strings.Count(plans[right].Path, string(filepath.Separator))
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return plans[left].Path < plans[right].Path
	})
}

// signingResignFrameworkContainers returns every ancestor code container of a
// scheduled code plan, deepest first, so each container's signature and
// resource seal are refreshed after its contained code changes.
func signingResignFrameworkContainers(treePath string, plans []signingResignCodePlan) []string {
	seen := make(map[string]struct{})
	for _, plan := range plans {
		candidate := filepath.Dir(plan.Path)
		for candidate != treePath && strings.HasPrefix(candidate, treePath+string(filepath.Separator)) {
			if isSigningResignCodeContainerName(filepath.Base(candidate)) {
				seen[candidate] = struct{}{}
			}
			candidate = filepath.Dir(candidate)
		}
	}
	containers := make([]string, 0, len(seen))
	for candidate := range seen {
		containers = append(containers, candidate)
	}
	sort.Slice(containers, func(left, right int) bool {
		leftDepth := strings.Count(strings.TrimPrefix(containers[left], treePath), string(filepath.Separator))
		rightDepth := strings.Count(strings.TrimPrefix(containers[right], treePath), string(filepath.Separator))
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return containers[left] < containers[right]
	})
	return containers
}

func verifySigningResignTree(ctx context.Context, treeRoot rootfs.Root, prepared signingResignPreparedTree, teamID, certificateSHA256 string) (resultErr error) {
	defer func() {
		resultErr = wrapSigningResignOperationalError(
			signingResignStageVerification,
			signingResignCodeVerification,
			resultErr,
		)
	}()
	treePath := treeRoot.Path()
	if err := checkSigningResignRootIdentity(treeRoot); err != nil {
		return fmt.Errorf("verification tree identity changed: %w", err)
	}
	plans := append([]signingResignCodePlan(nil), prepared.CodePlans...)
	for _, plan := range plans {
		if err := verifySigningResignObject(ctx, plan.Path, teamID, false); err != nil {
			return fmt.Errorf("verify nested code %s: %w", signingResignDisplayPath(treePath, plan.Path), err)
		}
		if err := checkSigningResignRootIdentity(treeRoot); err != nil {
			return fmt.Errorf("verification tree identity changed: %w", err)
		}
		if err := verifySigningResignCertificate(ctx, plan.Path, certificateSHA256); err != nil {
			return fmt.Errorf("verify nested code certificate: %w", err)
		}
		if err := checkSigningResignRootIdentity(treeRoot); err != nil {
			return fmt.Errorf("verification tree identity changed: %w", err)
		}
		if err := validateSigningResignCodeEntitlements(ctx, plan); err != nil {
			return fmt.Errorf("verify nested code entitlements: %w", err)
		}
		if err := checkSigningResignRootIdentity(treeRoot); err != nil {
			return fmt.Errorf("verification tree identity changed: %w", err)
		}
	}
	for _, target := range prepared.Archive.Targets {
		targetPath := filepath.Join(treePath, filepath.FromSlash(target.RelativePath))
		if err := verifySigningResignObject(ctx, targetPath, teamID, false); err != nil {
			return fmt.Errorf("verify target %s: %w", target.BundleID, err)
		}
		if err := checkSigningResignRootIdentity(treeRoot); err != nil {
			return fmt.Errorf("verification tree identity changed: %w", err)
		}
		if err := verifySigningResignCertificate(ctx, targetPath, certificateSHA256); err != nil {
			return fmt.Errorf("verify target %s certificate: %w", target.BundleID, err)
		}
		if err := checkSigningResignRootIdentity(treeRoot); err != nil {
			return fmt.Errorf("verification tree identity changed: %w", err)
		}
		entitlements, err := readSigningResignEntitlements(ctx, targetExecutablePath(treePath, target))
		if err != nil {
			return fmt.Errorf("read verified target %s entitlements: %w", target.BundleID, err)
		}
		if err := checkSigningResignRootIdentity(treeRoot); err != nil {
			return fmt.Errorf("verification tree identity changed: %w", err)
		}
		if strings.TrimSpace(target.EntitlementsPath) == "" {
			return fmt.Errorf("target %s generated entitlements document is missing", target.BundleID)
		}
		if err := validateSigningResignEntitlementsAgainstDocumentAtStage(entitlements, target.EntitlementsPath, fmt.Sprintf("target %s signed entitlements", target.BundleID), signingResignStageVerification); err != nil {
			return err
		}
		profileData, err := treeRoot.ReadFileLimited(filepath.FromSlash(path.Join(target.RelativePath, "embedded.mobileprovision")), signingResignProfileMaxBytes)
		if err != nil {
			return fmt.Errorf("read verified target %s profile failed", target.BundleID)
		}
		if digest := signingResignSHA256(profileData); !strings.EqualFold(digest, target.Profile.SHA256) {
			return fmt.Errorf("verified target %s profile digest changed", target.BundleID)
		}
	}
	mainPath := filepath.Join(treePath, filepath.FromSlash(prepared.Archive.MainPath))
	if err := verifySigningResignObject(ctx, mainPath, teamID, true); err != nil {
		return fmt.Errorf("verify complete main app: %w", err)
	}
	if err := checkSigningResignRootIdentity(treeRoot); err != nil {
		return fmt.Errorf("verification tree identity changed: %w", err)
	}
	return nil
}

func signingResignDisplayPath(rootPath, candidate string) string {
	relative, err := filepath.Rel(rootPath, candidate)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "staged-code"
	}
	return filepath.ToSlash(relative)
}

func validateSigningResignCodeEntitlements(ctx context.Context, plan signingResignCodePlan) error {
	actual, err := readSigningResignEntitlements(ctx, plan.Path)
	if err != nil {
		return err
	}
	return validateSigningResignEntitlementsAgainstDocumentAtStage(actual, plan.EntitlementsPath, "signed entitlements", signingResignStageVerification)
}

func validateSigningResignEntitlementsAgainstDocumentAtStage(actual map[string]any, documentPath, subject string, stage signingResignOperationalStage) error {
	want, err := readSigningResignGeneratedEntitlementsAtStage(documentPath, stage)
	if err != nil {
		return err
	}
	if !signingResignEntitlementMapsEqual(actual, want) {
		return fmt.Errorf("%s do not exactly match the generated document", subject)
	}
	return nil
}

func readSigningResignGeneratedEntitlementsAtStage(documentPath string, stage signingResignOperationalStage) (map[string]any, error) {
	want := map[string]any{}
	if strings.TrimSpace(documentPath) == "" {
		return want, nil
	}
	data, err := readBoundedSigningRunFile(documentPath, infoplist.MaxBytes, false)
	if err != nil {
		return nil, wrapSigningResignOperationalError(
			stage,
			signingResignCodeGeneratedEntitlements,
			fmt.Errorf("read generated entitlements: %w", err),
		)
	}
	defer clear(data)
	if _, err := plist.Unmarshal(data, &want); err != nil {
		return nil, fmt.Errorf("decode generated entitlements: %w", err)
	}
	if want == nil {
		want = map[string]any{}
	}
	return want, nil
}

func signingResignEntitlementMapsEqual(left, right map[string]any) bool {
	if len(left) != len(right) {
		return false
	}
	for key, expected := range right {
		actual, exists := left[key]
		if !exists || !signingResignEntitlementValuesEqual(actual, expected) {
			return false
		}
	}
	return true
}

func signingResignEntitlementValuesEqual(left, right any) bool {
	leftList, leftIsList := signingResignEntitlementList(left)
	rightList, rightIsList := signingResignEntitlementList(right)
	if leftIsList || rightIsList {
		if !leftIsList || !rightIsList || len(leftList) != len(rightList) {
			return false
		}
		for index := range leftList {
			if !signingResignEntitlementValuesEqual(leftList[index], rightList[index]) {
				return false
			}
		}
		return true
	}
	leftMap, leftIsMap := left.(map[string]any)
	rightMap, rightIsMap := right.(map[string]any)
	if leftIsMap || rightIsMap {
		if !leftIsMap || !rightIsMap || len(leftMap) != len(rightMap) {
			return false
		}
		for key, expected := range rightMap {
			actual, exists := leftMap[key]
			if !exists || !signingResignEntitlementValuesEqual(actual, expected) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(left, right)
}

func signingResignSHA256(data []byte) string {
	digest := sha256.Sum256(data)
	return strings.ToUpper(hex.EncodeToString(digest[:]))
}

func removeSigningResignStage(stagePath string) error {
	clean := filepath.Clean(stagePath)
	if filepath.Dir(clean) != filepath.Clean(os.TempDir()) || !strings.HasPrefix(filepath.Base(clean), "asc-signing-resign.") {
		return fmt.Errorf("refusing to remove unexpected re-signing directory %q", stagePath)
	}
	info, err := os.Lstat(clean)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("refusing to remove unsafe re-signing directory")
	}
	return os.RemoveAll(clean)
}

func signingResignRepackEntryLimitError(count int) error {
	if count > signingResignMaxArchiveEntries {
		return fmt.Errorf("repacked IPA would exceed the archive entry limit")
	}
	return nil
}

func repackSigningResignTree(ctx context.Context, stageRoot, treeRoot rootfs.Root) (packedPath string, packedSize int64, packedDigest string, resultErr error) {
	defer func() {
		resultErr = wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeFilesystem,
			resultErr,
		)
	}()
	if err := ctx.Err(); err != nil {
		return "", 0, "", err
	}
	type repackEntry struct {
		relative  string
		directory bool
		mode      os.FileMode
	}
	entries := make([]repackEntry, 0)
	fileCount := 0
	err := filepath.WalkDir(treeRoot.Path(), func(candidate string, entry os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("staging tree contains a symbolic link")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			relative, err := filepath.Rel(treeRoot.Path(), candidate)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return fmt.Errorf("staging tree contains an invalid relative path")
			}
			if relative == "." {
				return nil
			}
			// Directory entries carry validated modes and can be empty, so a
			// faithful repack must record them explicitly instead of relying
			// on ancestors implied by file paths.
			entries = append(entries, repackEntry{relative: relative, directory: true, mode: info.Mode().Perm()})
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("staging tree contains a non-regular file")
		}
		relative, err := filepath.Rel(treeRoot.Path(), candidate)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("staging tree contains an invalid relative path")
		}
		entries = append(entries, repackEntry{relative: relative})
		fileCount++
		return nil
	})
	if err != nil {
		return "", 0, "", err
	}
	if fileCount == 0 {
		return "", 0, "", fmt.Errorf("staging tree is empty")
	}
	if err := signingResignRepackEntryLimitError(len(entries)); err != nil {
		return "", 0, "", err
	}
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].relative < entries[right].relative
	})
	stageOS, err := stageRoot.OpenRoot()
	if err != nil {
		return "", 0, "", err
	}
	defer stageOS.Close()
	packed, err := secureopen.OpenNewFileNoFollowInRoot(stageOS, "resigned.ipa", 0o600)
	if err != nil {
		return "", 0, "", fmt.Errorf("create re-signed IPA: %w", err)
	}
	zipWriter := zip.NewWriter(packed)
	var operationErr error
	for _, item := range entries {
		if err := ctx.Err(); err != nil {
			operationErr = err
			break
		}
		if item.directory {
			header := &zip.FileHeader{Name: filepath.ToSlash(item.relative) + "/", Method: zip.Store}
			header.Modified = time.Unix(0, 0)
			header.SetMode(os.ModeDir | item.mode)
			if _, err := zipWriter.CreateHeader(header); err != nil {
				operationErr = err
				break
			}
			continue
		}
		file, err := treeRoot.OpenFile(item.relative)
		if err != nil {
			operationErr = err
			break
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			operationErr = err
			break
		}
		header := &zip.FileHeader{Name: filepath.ToSlash(item.relative), Method: zip.Deflate}
		header.Modified = time.Unix(0, 0)
		header.SetMode(info.Mode().Perm())
		entry, err := zipWriter.CreateHeader(header)
		if err == nil {
			_, err = copySigningResignWithContext(ctx, entry, io.LimitReader(file, info.Size()+1), info.Size())
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			operationErr = err
			break
		}
	}
	if operationErr == nil {
		operationErr = zipWriter.Close()
	} else {
		_ = zipWriter.Close()
	}
	if operationErr == nil {
		operationErr = packed.Sync()
	}
	closeErr := packed.Close()
	if operationErr != nil || closeErr != nil {
		_ = os.Remove(filepath.Join(stageRoot.Path(), "resigned.ipa"))
		return "", 0, "", errors.Join(operationErr, closeErr)
	}
	packedPath = filepath.Join(stageRoot.Path(), "resigned.ipa")
	info, err := os.Stat(packedPath)
	if err != nil {
		return "", 0, "", err
	}
	digest, err := hashSigningResignFile(ctx, packedPath, info.Size())
	if err != nil {
		return "", 0, "", err
	}
	return packedPath, info.Size(), digest, nil
}

func validatePackedSigningResignIPA(ctx context.Context, packedPath string, size int64) error {
	if err := ctx.Err(); err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageVerification,
			signingResignCodeVerification,
			err,
		)
	}
	file, err := os.Open(packedPath)
	if err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageVerification,
			signingResignCodeArtifactRead,
			fmt.Errorf("open re-signed IPA: %w", err),
		)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() != size {
		return fmt.Errorf("re-signed IPA changed before publication")
	}
	if err := preflightSigningResignArchive(ctx, file, size); err != nil {
		return err
	}
	reader, err := zip.NewReader(file, size)
	if err != nil {
		return fmt.Errorf("read re-signed IPA: %w", err)
	}
	return validateSigningResignArchive(ctx, reader)
}

func verifyPackedSigningResignIPA(ctx context.Context, packedPath string, size int64, stageRoot rootfs.Root, originalTreePath string, original signingResignPreparedTree, teamID, certificateSHA256 string) error {
	if err := ctx.Err(); err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageVerification,
			signingResignCodeVerification,
			err,
		)
	}
	file, err := os.Open(packedPath)
	if err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageVerification,
			signingResignCodeArtifactRead,
			fmt.Errorf("open re-signed IPA for final verification: %w", err),
		)
	}
	defer file.Close()
	if err := preflightSigningResignArchive(ctx, file, size); err != nil {
		return err
	}
	reader, err := zip.NewReader(file, size)
	if err != nil {
		return fmt.Errorf("read re-signed IPA for final verification: %w", err)
	}
	if err := validateSigningResignArchive(ctx, reader); err != nil {
		return err
	}
	if err := stageRoot.MkdirAll("packed-tree", 0o700); err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeFilesystem,
			fmt.Errorf("create final verification tree: %w", err),
		)
	}
	packedTreeRoot, err := rootfs.New(filepath.Join(stageRoot.Path(), "packed-tree"))
	if err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeFilesystem,
			fmt.Errorf("bind final verification tree: %w", err),
		)
	}
	defer packedTreeRoot.Close()
	stageOS, err := stageRoot.OpenRoot()
	if err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeFilesystem,
			fmt.Errorf("open final verification root: %w", err),
		)
	}
	defer stageOS.Close()
	packedTreeOS, err := packedTreeRoot.OpenRoot()
	if err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeFilesystem,
			fmt.Errorf("open final verification tree: %w", err),
		)
	}
	defer packedTreeOS.Close()
	if err := materializeSigningResignArchive(ctx, reader, packedTreeOS); err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeFilesystem,
			fmt.Errorf("materialize final verification tree: %w", err),
		)
	}
	if err := validateSigningResignSwiftSupportRoot(ctx, stageRoot.Path(), packedTreeOS); err != nil {
		return fmt.Errorf("verify preserved SwiftSupport after repack: %w", err)
	}
	if err := validateSigningResignWatchKitSupportRoot(ctx, stageRoot.Path(), packedTreeOS); err != nil {
		return fmt.Errorf("verify preserved SwiftSupport after repack: %w", err)
	}
	packedSwiftSupport, err := captureSigningResignPreservedInventoryRoot(ctx, packedTreeOS)
	if err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactRead,
			fmt.Errorf("capture packed preserved support inventory: %w", err),
		)
	}
	if err := validateSigningResignSwiftSupportInventory(packedSwiftSupport, original.SwiftSupport); err != nil {
		return wrapSigningResignOperationalError(
			signingResignStageVerification,
			signingResignCodeVerification,
			err,
		)
	}
	if err := validateSigningResignPackedCodeInventoryRoot(ctx, packedTreeOS, originalTreePath, original); err != nil {
		return fmt.Errorf("verify packed Mach-O inventory: %w", err)
	}
	archive, err := discoverSigningResignArchiveRooted(ctx, reader, packedTreeRoot)
	if err != nil {
		return fmt.Errorf("inspect final verification targets: %w", err)
	}
	if archive.MainPath != original.Archive.MainPath || len(archive.Targets) != len(original.Archive.Targets) {
		return fmt.Errorf("re-signed IPA target inventory changed during repack")
	}
	for index, target := range archive.Targets {
		want := original.Archive.Targets[index]
		if target.Kind != want.Kind || target.RelativePath != want.RelativePath || target.BundleID != want.BundleID || target.Executable != want.Executable || target.ProfileMode.Perm() != want.ProfileMode.Perm() {
			return fmt.Errorf("re-signed IPA target inventory changed during repack")
		}
		profileData, err := packedTreeRoot.ReadFileLimited(filepath.FromSlash(path.Join(target.RelativePath, "embedded.mobileprovision")), signingResignProfileMaxBytes)
		if err != nil || !strings.EqualFold(signingResignSHA256(profileData), want.Profile.SHA256) {
			return fmt.Errorf("re-signed IPA target profile changed during repack")
		}
	}
	finalPrepared, err := rebaseSigningResignPreparedTree(original, originalTreePath, packedTreeRoot.Path())
	if err != nil {
		return fmt.Errorf("rebase final verification targets: %w", err)
	}
	if err := verifySigningResignTree(ctx, packedTreeRoot, finalPrepared, teamID, certificateSHA256); err != nil {
		return fmt.Errorf("verify re-signed IPA after repack: %w", err)
	}
	return nil
}

func validateSigningResignSwiftSupportInventory(actual, expected []signingResignSwiftSupportEntry) error {
	if !slices.Equal(actual, expected) {
		return fmt.Errorf("re-signed IPA SwiftSupport inventory changed during repack")
	}
	return nil
}

func validateSigningResignPackedCodeInventory(ctx context.Context, packedTreePath, originalTreePath string, original signingResignPreparedTree) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	packedRoot, err := filepath.Abs(filepath.Clean(packedTreePath))
	if err != nil {
		return fmt.Errorf("resolve packed verification tree: %w", err)
	}
	root, err := os.OpenRoot(packedRoot)
	if err != nil {
		return fmt.Errorf("open packed verification tree: %w", err)
	}
	defer root.Close()
	return validateSigningResignPackedCodeInventoryRoot(ctx, root, originalTreePath, original)
}

func validateSigningResignPackedCodeInventoryRoot(ctx context.Context, packedRoot *os.Root, originalTreePath string, original signingResignPreparedTree) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if packedRoot == nil {
		return fmt.Errorf("packed verification tree root is missing")
	}
	originalRoot, err := filepath.Abs(filepath.Clean(originalTreePath))
	if err != nil {
		return fmt.Errorf("resolve original prepared tree: %w", err)
	}
	expected := make([]string, 0, len(original.Archive.Targets)+len(original.CodePlans))
	for _, target := range original.Archive.Targets {
		relative := filepath.Clean(filepath.FromSlash(path.Join(target.RelativePath, target.Executable)))
		expected = append(expected, filepath.ToSlash(relative))
	}
	for _, plan := range original.CodePlans {
		codePath := filepath.Clean(plan.Path)
		if !filepath.IsAbs(codePath) {
			return fmt.Errorf("original prepared code path is not absolute")
		}
		relative, err := filepath.Rel(originalRoot, codePath)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("original prepared code path is outside the staging tree")
		}
		expected = append(expected, filepath.ToSlash(filepath.Clean(relative)))
	}
	currentPaths, err := enumerateSigningResignMachOFilesRoot(ctx, packedRoot)
	if err != nil {
		return fmt.Errorf("enumerate packed Mach-O files: %w", err)
	}
	current := make([]string, 0, len(currentPaths))
	for _, relative := range currentPaths {
		if isSigningResignPreservedExternalCodeRelativePath(relative) {
			continue
		}
		if relative == "." || relative == ".." || strings.HasPrefix(relative, "../") || !filepath.IsLocal(filepath.FromSlash(relative)) {
			return fmt.Errorf("packed code path is outside the staging tree")
		}
		current = append(current, path.Clean(relative))
	}
	sort.Strings(expected)
	sort.Strings(current)
	if !slices.Equal(current, expected) {
		return fmt.Errorf("re-signed IPA Mach-O executable inventory changed during repack")
	}
	return nil
}

func rebaseSigningResignPreparedTree(original signingResignPreparedTree, originalTreePath, packedTreePath string) (signingResignPreparedTree, error) {
	if originalTreePath == "" || packedTreePath == "" {
		return signingResignPreparedTree{}, fmt.Errorf("prepared tree roots are missing")
	}
	originalRoot, err := filepath.Abs(filepath.Clean(originalTreePath))
	if err != nil {
		return signingResignPreparedTree{}, fmt.Errorf("resolve original prepared tree: %w", err)
	}
	packedRoot, err := filepath.Abs(filepath.Clean(packedTreePath))
	if err != nil {
		return signingResignPreparedTree{}, fmt.Errorf("resolve packed verification tree: %w", err)
	}
	rebased := original
	rebased.Archive.Targets = append([]signingResignTarget(nil), original.Archive.Targets...)
	rebased.CodePlans = append([]signingResignCodePlan(nil), original.CodePlans...)
	rebased.SwiftSupport = append([]signingResignSwiftSupportEntry(nil), original.SwiftSupport...)
	for index := range rebased.CodePlans {
		codePath := filepath.Clean(rebased.CodePlans[index].Path)
		if !filepath.IsAbs(codePath) {
			return signingResignPreparedTree{}, fmt.Errorf("prepared code path is not absolute")
		}
		relative, err := filepath.Rel(originalRoot, codePath)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return signingResignPreparedTree{}, fmt.Errorf("prepared code path is outside the original staging tree")
		}
		rebased.CodePlans[index].Path = filepath.Join(packedRoot, relative)
	}
	return rebased, nil
}

func publishSigningResignOutput(ctx context.Context, outputRoot rootfs.Root, name, packedPath string, packedSize int64, packedDigest string) (signingResignArtifactResult, error) {
	if err := ctx.Err(); err != nil {
		return signingResignArtifactResult{}, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactPublish,
			err,
		)
	}
	file, err := os.Open(packedPath)
	if err != nil {
		return signingResignArtifactResult{}, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactRead,
			fmt.Errorf("open staged re-signed IPA: %w", err),
		)
	}
	defer file.Close()
	written, err := outputRoot.CreateNewFrom(name, file, 0o600)
	if err != nil {
		// CreateNewFrom can report a durability/cleanup error after the
		// no-replace rename has already published the destination. If the
		// complete staged byte count was written and the destination is now
		// visible, preserve that uncertainty for the caller instead of
		// inviting a blind retry.
		if written == packedSize {
			if published, openErr := outputRoot.OpenFile(name); openErr == nil {
				_ = published.Close()
				return signingResignArtifactResult{}, wrapSigningResignOperationalError(
					signingResignStageArtifact,
					signingResignCodeArtifactPublish,
					signingResignPublicationAmbiguousError("publish re-signed IPA returned an uncertain result", err),
				)
			} else {
				err = errors.Join(err, openErr)
			}
		}
		return signingResignArtifactResult{}, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactPublish,
			fmt.Errorf("publish re-signed IPA: %w", err),
		)
	}
	if err := ctx.Err(); err != nil {
		return signingResignArtifactResult{}, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactPublish,
			signingResignPublicationAmbiguousError("publication completed but cancellation prevented validation", err),
		)
	}
	if written != packedSize {
		return signingResignArtifactResult{}, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactPublish,
			signingResignPublicationAmbiguousError("published re-signed IPA size is inconsistent"),
		)
	}
	published, err := outputRoot.OpenFile(name)
	if err != nil {
		return signingResignArtifactResult{}, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactPublish,
			signingResignPublicationAmbiguousError("reopen published re-signed IPA failed", err),
		)
	}
	defer func() {
		if published != nil {
			_ = published.Close()
		}
	}()
	info, err := published.Stat()
	if err != nil {
		return signingResignArtifactResult{}, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactPublish,
			signingResignPublicationAmbiguousError("inspect published re-signed IPA failed", err),
		)
	}
	signingResignBeforePublishedHashFn()
	digest, err := hashSigningResignOpenFile(ctx, published, info.Size())
	if err != nil {
		return signingResignArtifactResult{}, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactHash,
			signingResignPublicationAmbiguousError("hash published re-signed IPA failed", err),
		)
	}
	if !strings.EqualFold(digest, packedDigest) {
		return signingResignArtifactResult{}, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactHash,
			signingResignPublicationAmbiguousError("published re-signed IPA digest is inconsistent"),
		)
	}
	if err := published.Close(); err != nil {
		published = nil
		return signingResignArtifactResult{}, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactPublish,
			signingResignPublicationAmbiguousError("close published re-signed IPA failed", err),
		)
	}
	published = nil
	if err := ctx.Err(); err != nil {
		return signingResignArtifactResult{}, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactPublish,
			signingResignPublicationAmbiguousError("publication completed but cancellation prevented success", err),
		)
	}
	return signingResignArtifactResult{Path: filepath.Join(outputRoot.Path(), name), SizeBytes: info.Size(), SHA256: digest}, nil
}

func hashSigningResignFile(ctx context.Context, pathValue string, size int64) (digest string, resultErr error) {
	defer func() {
		resultErr = wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactHash,
			resultErr,
		)
	}()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	file, err := os.Open(pathValue)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return hashSigningResignOpenFile(ctx, file, size)
}

func hashSigningResignOpenFile(ctx context.Context, file *os.File, size int64) (digest string, resultErr error) {
	defer func() {
		resultErr = wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactHash,
			resultErr,
		)
	}()
	if file == nil || size < 0 {
		return "", fmt.Errorf("hash input is invalid")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	hash := sha256.New()
	written, err := copySigningResignWithContext(ctx, hash, io.LimitReader(file, size+1), size)
	if err != nil {
		return "", err
	}
	if written != size {
		return "", fmt.Errorf("hash input size changed")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(hash.Sum(nil))), nil
}
