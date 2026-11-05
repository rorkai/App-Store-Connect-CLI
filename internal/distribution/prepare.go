package distribution

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

var (
	ErrNotEligible         = errors.New("IPA metadata is not eligible for release-testing preparation")
	ErrBundleConflict      = errors.New("distribution bundle conflicts with existing output")
	ErrIPAIdentityMismatch = errors.New("IPA snapshot does not match expected identity")

	afterOutputParentsCreatedForTest func()
	verifyCompleteSigningForTest     func(*Inspection)
	duringPublicationCopyForTest     func()
	syncPreparedDirectory            = syncPreparedRootDirectory
	renamePreparedBundleNoReplace    = secureopen.RenameNoReplaceInRoot
	inspectExactBundleForTest        func(string) error
)

type Descriptor struct {
	SchemaVersion      string   `json:"schemaVersion"`
	Platform           string   `json:"platform"`
	DistributionMethod string   `json:"distributionMethod"`
	App                App      `json:"app"`
	Artifact           Artifact `json:"artifact"`
	Signing            Signing  `json:"signing"`
	Source             *Source  `json:"source,omitempty"`
}

type PrepareOptions struct {
	Root           string
	OutputDir      string
	Title          string
	Channel        string
	SourceRevision string
	SourceURL      string
}

type PrepareResult struct {
	BundlePath string     `json:"bundlePath"`
	Reused     bool       `json:"reused"`
	Descriptor Descriptor `json:"descriptor"`
}

// ExpectedIPA binds agent-driven preparation to an exact upstream artifact.
// SHA256 must be the lowercase hexadecimal digest of exactly SizeBytes bytes.
type ExpectedIPA struct {
	SHA256    string
	SizeBytes int64
}

// PrepareIPAContext validates an already-open IPA and publishes an immutable
// local bundle without replacing an existing destination.
func PrepareIPAContext(ctx context.Context, file *os.File, size int64, options PrepareOptions) (result PrepareResult, resultErr error) {
	return prepareIPA(ctx, file, size, options, nil)
}

// PrepareIPAPathExact is the agent-facing preparation seam. It snapshots a
// relative, no-follow input and rejects it before inspection or output writes
// unless the private snapshot exactly matches expected.
func PrepareIPAPathExact(ctx context.Context, inputRoot rootfs.Root, ipaPath string, expected ExpectedIPA, options PrepareOptions) (PrepareResult, error) {
	if ctx == nil {
		return PrepareResult{}, fmt.Errorf("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return PrepareResult{}, err
	}
	if err := validateExpectedIPA(expected); err != nil {
		return PrepareResult{}, err
	}
	return prepareIPAPath(ctx, inputRoot, ipaPath, options, &expected)
}

func prepareIPAPath(ctx context.Context, inputRoot rootfs.Root, ipaPath string, options PrepareOptions, expected *ExpectedIPA) (PrepareResult, error) {
	if err := contextError(ctx); err != nil {
		return PrepareResult{}, err
	}
	if err := ValidatePrepareOptions(options); err != nil {
		return PrepareResult{}, err
	}
	if err := rootfs.ValidateRelative(ipaPath); err != nil {
		return PrepareResult{}, fmt.Errorf("invalid IPA path: %w", err)
	}
	if strings.TrimSpace(inputRoot.Path()) == "" {
		return PrepareResult{}, fmt.Errorf("input root is empty")
	}
	file, err := inputRoot.OpenFile(ipaPath)
	if err != nil {
		return PrepareResult{}, fmt.Errorf("open rooted IPA: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return PrepareResult{}, fmt.Errorf("stat rooted IPA: %w", err)
	}
	return prepareIPA(ctx, file, info.Size(), options, expected)
}

func prepareIPA(ctx context.Context, file *os.File, size int64, options PrepareOptions, expected *ExpectedIPA) (result PrepareResult, resultErr error) {
	if err := contextError(ctx); err != nil {
		return PrepareResult{}, err
	}
	if err := ValidatePrepareOptions(options); err != nil {
		return PrepareResult{}, err
	}
	rootPath := strings.TrimSpace(options.Root)
	if rootPath == "" {
		rootPath = "."
	}
	root, err := rootfs.New(rootPath)
	if err != nil {
		return PrepareResult{}, fmt.Errorf("prepare output root: %w", err)
	}
	defer func() {
		if err := root.Close(); resultErr == nil && err != nil {
			result = PrepareResult{}
			resultErr = fmt.Errorf("close distribution output root: %w", err)
		}
	}()
	// Select and retain the output root before the potentially long snapshot,
	// archive validation, and code-signing work.
	if err := root.MkdirAll(".", 0o755); err != nil {
		return PrepareResult{}, fmt.Errorf("pin distribution output root: %w", err)
	}
	snapshot, digest, cleanup, err := snapshotIPAContext(ctx, file, size)
	if err != nil {
		return PrepareResult{}, err
	}
	defer cleanup()
	if expected != nil && (size != expected.SizeBytes || digest != expected.SHA256) {
		return PrepareResult{}, fmt.Errorf("%w: expected %d bytes with SHA-256 %s, got %d bytes with SHA-256 %s", ErrIPAIdentityMismatch, expected.SizeBytes, expected.SHA256, size, digest)
	}
	if afterIPASnapshotForTest != nil {
		afterIPASnapshotForTest()
	}
	inspection, err := inspectSnapshotContext(ctx, snapshot, size, digest, InspectOptions{})
	if err != nil {
		if expected != nil && !isRetryableExactIPAInspectionError(err) {
			return PrepareResult{}, fmt.Errorf("%w: %w", ErrNotEligible, err)
		}
		return PrepareResult{}, err
	}
	if verifyCompleteSigningForTest != nil {
		verifyCompleteSigningForTest(&inspection)
	}
	if err := contextError(ctx); err != nil {
		return PrepareResult{}, err
	}
	if title := strings.TrimSpace(options.Title); title != "" {
		inspection.App.Title = title
		inspection.Preparation.Issues = withoutIssue(inspection.Preparation.Issues, "app title is missing")
		inspection.Preparation.MetadataEligible = len(inspection.Preparation.Issues) == 0
	}
	if expected != nil && exactIPASigningVerificationUnavailable(inspection.Signing) {
		return PrepareResult{}, fmt.Errorf("complete main-app signature verification is temporarily unavailable")
	}
	if !inspection.Preparation.MetadataEligible {
		return PrepareResult{}, fmt.Errorf("%w: %s", ErrNotEligible, strings.Join(inspection.Preparation.Issues, "; "))
	}

	descriptor := Descriptor{
		SchemaVersion:      inspection.SchemaVersion,
		Platform:           inspection.Platform,
		DistributionMethod: inspection.DistributionMethod,
		App:                inspection.App,
		Artifact: Artifact{
			RelativePath: "payload/app.ipa",
			SizeBytes:    inspection.Artifact.SizeBytes,
			SHA256:       inspection.Artifact.SHA256,
		},
		Signing: inspection.Signing,
	}
	descriptor.Signing.Devices = nil
	if source := buildSource(options); source != nil {
		descriptor.Source = source
	}
	if err := ValidateDescriptorForPublish(descriptor); err != nil {
		return PrepareResult{}, fmt.Errorf("%w: %w", ErrNotEligible, err)
	}
	descriptorData, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return PrepareResult{}, fmt.Errorf("encode bundle descriptor: %w", err)
	}
	descriptorData = append(descriptorData, '\n')

	relativeOutput, err := prepareOutputPath(inspection, options.OutputDir)
	if err != nil {
		return PrepareResult{}, err
	}
	rooted, err := root.OpenRoot()
	if err != nil {
		return PrepareResult{}, fmt.Errorf("open distribution output root: %w", err)
	}
	defer rooted.Close()
	parentRelative := filepath.Dir(relativeOutput)
	if err := contextError(ctx); err != nil {
		return PrepareResult{}, err
	}
	if err := rooted.MkdirAll(parentRelative, 0o755); err != nil {
		return PrepareResult{}, fmt.Errorf("create distribution output parent: %w", err)
	}
	if afterOutputParentsCreatedForTest != nil {
		afterOutputParentsCreatedForTest()
	}
	parent, err := rooted.OpenRoot(parentRelative)
	if err != nil {
		return PrepareResult{}, fmt.Errorf("open distribution output parent: %w", err)
	}
	defer parent.Close()
	bundlePath := filepath.Join(root.Path(), relativeOutput)
	finalName := filepath.Base(relativeOutput)
	result = PrepareResult{BundlePath: bundlePath, Descriptor: descriptor}
	if reused, exists, err := exactBundleExistsContext(ctx, parent, finalName, descriptorData, descriptor.Artifact); err != nil {
		return PrepareResult{}, err
	} else if exists {
		if !reused {
			return PrepareResult{}, fmt.Errorf("%w: %s", ErrBundleConflict, bundlePath)
		}
		result.Reused = true
		return result, nil
	}

	stageName, stage, err := createStageDirectory(parent)
	if err != nil {
		return PrepareResult{}, fmt.Errorf("create distribution staging directory: %w", err)
	}
	stageInfo, err := stage.Stat(".")
	if err != nil {
		_ = stage.Close()
		_ = parent.RemoveAll(stageName)
		return PrepareResult{}, fmt.Errorf("stat distribution staging directory: %w", err)
	}
	stageOpen := true
	cleanupStage := true
	defer func() {
		if stageOpen {
			_ = stage.Close()
		}
		if cleanupStage {
			cleanupPreparedStage(parent, stageName, stageInfo)
		}
	}()

	if err := stage.Mkdir("payload", 0o755); err != nil {
		return PrepareResult{}, fmt.Errorf("create staged payload: %w", err)
	}
	payloadRoot, err := stage.OpenRoot("payload")
	if err != nil {
		return PrepareResult{}, fmt.Errorf("open staged payload: %w", err)
	}
	if err := copySectionToNewFileContext(ctx, payloadRoot, "app.ipa", snapshot, size, descriptor.Artifact.SHA256, 0o644); err != nil {
		_ = payloadRoot.Close()
		return PrepareResult{}, fmt.Errorf("copy IPA into staged bundle: %w", err)
	}
	if err := syncPreparedDirectory(payloadRoot, "staged payload"); err != nil {
		_ = payloadRoot.Close()
		return PrepareResult{}, fmt.Errorf("sync staged payload directory: %w", err)
	}
	if err := payloadRoot.Close(); err != nil {
		return PrepareResult{}, fmt.Errorf("close staged payload: %w", err)
	}
	// Write the descriptor last so even the private staging directory never
	// advertises a payload that has not finished copying.
	if err := contextError(ctx); err != nil {
		return PrepareResult{}, err
	}
	if err := writeNewRootedFile(stage, "bundle.json", descriptorData, 0o644); err != nil {
		return PrepareResult{}, fmt.Errorf("write staged bundle descriptor: %w", err)
	}
	if err := syncPreparedDirectory(stage, "staged bundle"); err != nil {
		return PrepareResult{}, fmt.Errorf("sync staged bundle directory: %w", err)
	}
	if err := stage.Close(); err != nil {
		return PrepareResult{}, fmt.Errorf("close distribution staging directory: %w", err)
	}
	stageOpen = false

	if err := contextError(ctx); err != nil {
		return PrepareResult{}, err
	}
	if err := renamePreparedBundleNoReplace(parent, stageName, finalName); err != nil {
		if !errors.Is(err, os.ErrExist) {
			// An unclassified rename failure may be a lost success response. Never
			// delete the known staging inode after that ambiguity: another actor
			// may already have moved the published directory back to stageName.
			cleanupStage = false
		}
		reused, exists, reuseErr := exactBundleExistsContext(ctx, parent, finalName, descriptorData, descriptor.Artifact)
		if reuseErr != nil {
			return PrepareResult{}, fmt.Errorf("inspect distribution destination after ambiguous publish failure: %w", reuseErr)
		}
		if exists && reused {
			result.Reused = true
			return result, nil
		}
		if exists || errors.Is(err, os.ErrExist) {
			return PrepareResult{}, fmt.Errorf("%w: %s", ErrBundleConflict, bundlePath)
		}
		return PrepareResult{}, fmt.Errorf("publish distribution bundle without replacement: %w", err)
	}
	cleanupStage = false
	if err := syncPreparedDirectory(parent, "destination parent"); err != nil {
		return PrepareResult{}, fmt.Errorf("sync distribution destination parent after publish: %w", err)
	}
	finalInfo, err := parent.Lstat(finalName)
	if err != nil || finalInfo.Mode()&os.ModeSymlink != 0 || !finalInfo.IsDir() || !os.SameFile(stageInfo, finalInfo) {
		return PrepareResult{}, fmt.Errorf("published distribution bundle changed during destination durability sync")
	}
	return result, nil
}

func isRetryableExactIPAInspectionError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, os.ErrClosed) || errors.Is(err, os.ErrPermission) || errors.Is(err, fs.ErrNotExist) {
		return true
	}
	var pathError *fs.PathError
	if errors.As(err, &pathError) {
		return true
	}
	var linkError *os.LinkError
	if errors.As(err, &linkError) {
		return true
	}
	var errno syscall.Errno
	return errors.As(err, &errno)
}

func exactIPASigningVerificationUnavailable(signing Signing) bool {
	return signing.ProfileIntegrityVerification.Status == CodeSignatureVerified &&
		signing.ProfileTrustVerification.Status == CodeSignatureVerified &&
		signing.CodeSignatureVerification.Status == CodeSignatureNotVerified
}

func validateExpectedIPA(expected ExpectedIPA) error {
	if expected.SizeBytes <= 0 || expected.SizeBytes > MaxIPABytes {
		return fmt.Errorf("expected IPA size must be between 1 and %d bytes", MaxIPABytes)
	}
	if !isCanonicalDigest(expected.SHA256) {
		return fmt.Errorf("expected IPA SHA-256 must be 64 lowercase hexadecimal characters")
	}
	return nil
}

// ValidateDescriptorForPublish is the canonical preparation-evidence gate for
// in-process consumers. It deliberately centralizes the exact verification
// scope so workflow and publisher code do not duplicate a security string.
func ValidateDescriptorForPublish(descriptor Descriptor) error {
	if descriptor.SchemaVersion != "1" || descriptor.Platform != "IOS" || descriptor.DistributionMethod != "release-testing" {
		return fmt.Errorf("descriptor requires schemaVersion 1, platform IOS, and release-testing distribution")
	}
	if descriptor.App.BundleID == "" || descriptor.App.Title == "" || descriptor.App.Version == "" || descriptor.App.BuildNumber == "" {
		return fmt.Errorf("descriptor app requires bundleId, title, version, and buildNumber")
	}
	if descriptor.Artifact.RelativePath != "payload/app.ipa" || descriptor.Artifact.SizeBytes <= 0 || descriptor.Artifact.SizeBytes > MaxIPABytes || !isCanonicalDigest(descriptor.Artifact.SHA256) {
		return fmt.Errorf("descriptor artifact evidence is invalid")
	}
	if descriptor.Signing.ProfileClass != ProfileClassAdHoc || strings.TrimSpace(descriptor.Signing.ProfileUUID) == "" || strings.TrimSpace(descriptor.Signing.TeamID) == "" || strings.TrimSpace(descriptor.Signing.ExpiresAt) == "" {
		return fmt.Errorf("descriptor requires exact ad-hoc profile identity")
	}
	if descriptor.Signing.DeviceCount <= 0 || !isCanonicalDigest(descriptor.Signing.DeviceSetSHA256) || !isCanonicalDigest(descriptor.Signing.EmbeddedProfileSHA256) {
		return fmt.Errorf("descriptor requires device-set and embedded-profile SHA-256 evidence")
	}
	if len(descriptor.Signing.Devices) != 0 {
		return fmt.Errorf("descriptor must not contain raw device identifiers")
	}
	if descriptor.Signing.ProfileIntegrityVerification.Status != CodeSignatureVerified ||
		descriptor.Signing.ProfileTrustVerification.Status != CodeSignatureVerified ||
		descriptor.Signing.CodeSignatureVerification.Status != CodeSignatureVerified ||
		descriptor.Signing.CodeSignatureVerification.Scope != CodeSignatureScopeCompleteMainApp {
		return fmt.Errorf("provisioning profile trust and complete main-app signature verification are required")
	}
	profileCertificates := make(map[string]struct{}, len(descriptor.Signing.ProfileCertificateSHA256Fingerprints))
	for _, fingerprint := range descriptor.Signing.ProfileCertificateSHA256Fingerprints {
		if !isCanonicalDigest(fingerprint) {
			return fmt.Errorf("descriptor profile certificate fingerprint is invalid")
		}
		profileCertificates[fingerprint] = struct{}{}
	}
	if len(profileCertificates) == 0 || len(descriptor.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints) == 0 {
		return fmt.Errorf("descriptor requires profile and signer certificate fingerprints")
	}
	for _, fingerprint := range descriptor.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints {
		if !isCanonicalDigest(fingerprint) {
			return fmt.Errorf("descriptor signer certificate fingerprint is invalid")
		}
		if _, ok := profileCertificates[fingerprint]; !ok {
			return fmt.Errorf("descriptor signer certificate is not bound to the embedded profile")
		}
	}
	return nil
}

func isCanonicalDigest(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func prepareOutputPath(inspection Inspection, requested string) (string, error) {
	if requested = strings.TrimSpace(requested); requested != "" {
		if err := rootfs.ValidateRelative(requested); err != nil {
			return "", fmt.Errorf("invalid output directory: %w", err)
		}
		return filepath.Clean(requested), nil
	}
	bundleID, err := safePathComponent(inspection.App.BundleID)
	if err != nil {
		return "", fmt.Errorf("invalid bundle identifier path component: %w", err)
	}
	version, err := safePathComponent(inspection.App.Version)
	if err != nil {
		return "", fmt.Errorf("invalid version path component: %w", err)
	}
	build, err := safePathComponent(inspection.App.BuildNumber)
	if err != nil {
		return "", fmt.Errorf("invalid build number path component: %w", err)
	}
	identity := fmt.Sprintf("%s-%s-%s", version, build, inspection.Artifact.SHA256[:12])
	return filepath.Join(".asc", "distribution", bundleID, identity), nil
}

func safePathComponent(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("value is empty")
	}
	var builder strings.Builder
	for _, b := range []byte(value) {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' {
			builder.WriteByte(b)
		} else {
			fmt.Fprintf(&builder, "~%02X", b)
		}
	}
	result := builder.String()
	if result == "." || result == ".." {
		result = strings.Repeat("~2E", len(result))
	}
	return result, nil
}

// ValidatePrepareOptions validates optional metadata before preparation opens
// or writes any filesystem path.
func ValidatePrepareOptions(options PrepareOptions) error {
	for _, field := range []struct {
		name  string
		value string
		limit int
	}{
		{name: "--title", value: options.Title, limit: 256},
		{name: "--channel", value: options.Channel, limit: 256},
		{name: "--source-revision", value: options.SourceRevision, limit: 1024},
	} {
		if err := validateDescriptorText(field.name, field.value, field.limit); err != nil {
			return err
		}
	}
	if err := validateDescriptorText("--source-url", options.SourceURL, 2048); err != nil {
		return err
	}
	return validateSourceURL(options.SourceURL)
}

func validateDescriptorText(name, value string, limit int) error {
	if len(value) > limit {
		return fmt.Errorf("invalid %s: must be at most %d bytes", name, limit)
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.In(r, unicode.Bidi_Control) || unicode.Is(unicode.Cf, r) {
			return fmt.Errorf("invalid %s: control characters are not allowed", name)
		}
	}
	return nil
}

func validateSourceURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if len(raw) > 2048 {
		return fmt.Errorf("invalid --source-url: must be at most 2048 bytes")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid --source-url: %w", err)
	}
	if parsed.User != nil {
		return fmt.Errorf("invalid --source-url: user information is not allowed")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("invalid --source-url: query and fragment are not allowed")
	}
	if parsed.Scheme != "https" || parsed.Hostname() == "" {
		return fmt.Errorf("invalid --source-url: must be an absolute HTTPS URL")
	}
	return nil
}

func buildSource(options PrepareOptions) *Source {
	result := &Source{Channel: strings.TrimSpace(options.Channel), Revision: strings.TrimSpace(options.SourceRevision), URL: strings.TrimSpace(options.SourceURL)}
	if result.Channel == "" && result.Revision == "" && result.URL == "" {
		return nil
	}
	return result
}

func withoutIssue(issues []string, remove string) []string {
	result := make([]string, 0, len(issues))
	for _, issue := range issues {
		if issue != remove {
			result = append(result, issue)
		}
	}
	return result
}

func createStageDirectory(parent *os.Root) (string, *os.Root, error) {
	for range 100 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", nil, err
		}
		name := ".asc-distribute-stage-" + hex.EncodeToString(random[:])
		if err := parent.Mkdir(name, 0o700); err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return "", nil, err
		}
		child, err := parent.OpenRoot(name)
		if err != nil {
			_ = parent.RemoveAll(name)
			return "", nil, err
		}
		return name, child, nil
	}
	return "", nil, fmt.Errorf("could not allocate a unique staging directory")
}

func cleanupPreparedStage(parent *os.Root, stageName string, stageInfo os.FileInfo) {
	current, err := parent.Lstat(stageName)
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(stageInfo, current) {
		return
	}
	_ = parent.RemoveAll(stageName)
}

func writeNewRootedFile(root *os.Root, name string, data []byte, mode os.FileMode) error {
	file, err := secureopen.OpenNewFileNoFollowInRoot(root, name, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func copySectionToNewFileContext(ctx context.Context, root *os.Root, name string, source *os.File, size int64, expectedSHA256 string, mode os.FileMode) error {
	destination, err := secureopen.OpenNewFileNoFollowInRoot(root, name, mode)
	if err != nil {
		return err
	}
	hash := sha256.New()
	written, err := copyWithContext(ctx, io.MultiWriter(destination, hash), io.NewSectionReader(source, 0, size), duringPublicationCopyForTest)
	if err != nil {
		_ = destination.Close()
		return err
	}
	if written != size || hex.EncodeToString(hash.Sum(nil)) != expectedSHA256 {
		_ = destination.Close()
		return fmt.Errorf("IPA changed while it was being prepared")
	}
	if err := destination.Sync(); err != nil {
		_ = destination.Close()
		return err
	}
	return destination.Close()
}

func exactBundleExistsContext(ctx context.Context, parent *os.Root, name string, wantDescriptor []byte, artifact Artifact) (bool, bool, error) {
	if err := contextError(ctx); err != nil {
		return false, false, err
	}
	info, err := parent.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, true, nil
	}
	if err := inspectExactBundle("open existing bundle"); err != nil {
		return false, true, err
	}
	bundle, err := parent.OpenRoot(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, true, nil
		}
		return false, true, err
	}
	defer bundle.Close()
	bundleInfo, err := bundle.Stat(".")
	if err != nil {
		return false, true, err
	}
	if !os.SameFile(info, bundleInfo) {
		return false, true, nil
	}
	if err := inspectExactBundle("read existing bundle directory"); err != nil {
		return false, true, err
	}
	entries, err := readDirectory(bundle)
	if err != nil {
		return false, true, err
	}
	if !exactEntries(entries, map[string]bool{"bundle.json": false, "payload": true}) {
		return false, true, nil
	}
	if err := inspectExactBundle("open existing payload"); err != nil {
		return false, true, err
	}
	payload, err := bundle.OpenRoot("payload")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, true, nil
		}
		return false, true, err
	}
	defer payload.Close()
	payloadInfo, err := payload.Stat(".")
	if err != nil {
		return false, true, err
	}
	payloadPathInfo, err := bundle.Lstat("payload")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, true, nil
		}
		return false, true, err
	}
	if payloadPathInfo.Mode()&os.ModeSymlink != 0 || !payloadPathInfo.IsDir() || !os.SameFile(payloadInfo, payloadPathInfo) {
		return false, true, nil
	}
	if err := inspectExactBundle("read existing payload directory"); err != nil {
		return false, true, err
	}
	payloadEntries, err := readDirectory(payload)
	if err != nil {
		return false, true, err
	}
	if !exactEntries(payloadEntries, map[string]bool{"app.ipa": false}) {
		return false, true, nil
	}
	if err := inspectExactBundle("read existing descriptor"); err != nil {
		return false, true, err
	}
	descriptor, err := secureopen.OpenExistingNoFollowInRoot(bundle, "bundle.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, true, nil
		}
		return false, true, err
	}
	defer descriptor.Close()
	descriptorInfo, err := descriptor.Stat()
	if err != nil {
		return false, true, err
	}
	if !descriptorInfo.Mode().IsRegular() || descriptorInfo.Size() != int64(len(wantDescriptor)) {
		return false, true, nil
	}
	var gotDescriptor bytes.Buffer
	_, readErr := copyWithContext(ctx, &gotDescriptor, io.LimitReader(descriptor, int64(len(wantDescriptor))+1), nil)
	descriptorAfterRead, statErr := descriptor.Stat()
	if readErr != nil {
		return false, true, readErr
	}
	if statErr != nil {
		return false, true, statErr
	}
	if !stablePreparedFile(descriptorInfo, descriptorAfterRead) || !bytes.Equal(gotDescriptor.Bytes(), wantDescriptor) {
		return false, true, nil
	}
	if err := inspectExactBundle("hash existing IPA"); err != nil {
		return false, true, err
	}
	ipa, err := secureopen.OpenExistingNoFollowInRoot(payload, "app.ipa")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, true, nil
		}
		return false, true, err
	}
	defer ipa.Close()
	ipaInfo, statErr := ipa.Stat()
	if statErr != nil {
		return false, true, statErr
	}
	if !ipaInfo.Mode().IsRegular() || ipaInfo.Size() != artifact.SizeBytes {
		return false, true, nil
	}
	hash := sha256.New()
	_, hashErr := copyWithContext(ctx, hash, ipa, nil)
	ipaAfterHash, statErr := ipa.Stat()
	if hashErr != nil {
		return false, true, hashErr
	}
	if statErr != nil {
		return false, true, statErr
	}
	if !stablePreparedFile(ipaInfo, ipaAfterHash) || hex.EncodeToString(hash.Sum(nil)) != artifact.SHA256 {
		return false, true, nil
	}
	// Sync the exact directories while retaining the same rooted handles used
	// to validate their entries. Reopening by pathname after validation would
	// permit a replacement race to be mistaken for a durable exact reuse.
	if err := syncPreparedDirectory(payload, "existing payload"); err != nil {
		return false, true, err
	}
	if err := syncPreparedDirectory(bundle, "existing bundle"); err != nil {
		return false, true, err
	}
	if err := syncPreparedDirectory(parent, "destination parent"); err != nil {
		return false, true, err
	}
	// Re-read exact bytes through the retained validated handles after directory
	// syncing. This catches in-place writes as well as pathname replacement.
	if _, err := descriptor.Seek(0, io.SeekStart); err != nil {
		return false, true, err
	}
	descriptorAfterSync, err := io.ReadAll(io.LimitReader(descriptor, int64(len(wantDescriptor))+1))
	descriptorFinalInfo, statErr := descriptor.Stat()
	if err != nil {
		return false, true, err
	}
	if statErr != nil {
		return false, true, statErr
	}
	if !stablePreparedFile(descriptorAfterRead, descriptorFinalInfo) || !bytes.Equal(descriptorAfterSync, wantDescriptor) {
		return false, true, nil
	}
	if _, err := ipa.Seek(0, io.SeekStart); err != nil {
		return false, true, err
	}
	finalHash := sha256.New()
	written, err := io.Copy(finalHash, io.LimitReader(ipa, artifact.SizeBytes+1))
	ipaFinalInfo, statErr := ipa.Stat()
	if err != nil {
		return false, true, err
	}
	if statErr != nil {
		return false, true, statErr
	}
	if !stablePreparedFile(ipaAfterHash, ipaFinalInfo) || written != artifact.SizeBytes || hex.EncodeToString(finalHash.Sum(nil)) != artifact.SHA256 {
		return false, true, nil
	}
	if entries, err := readDirectory(bundle); err != nil {
		return false, true, err
	} else if !exactEntries(entries, map[string]bool{"bundle.json": false, "payload": true}) {
		return false, true, nil
	}
	if entries, err := readDirectory(payload); err != nil {
		return false, true, err
	} else if !exactEntries(entries, map[string]bool{"app.ipa": false}) {
		return false, true, nil
	}
	currentInfo, err := parent.Lstat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, true, nil
		}
		return false, true, err
	}
	if currentInfo.Mode()&os.ModeSymlink != 0 || !currentInfo.IsDir() || !os.SameFile(bundleInfo, currentInfo) {
		return false, true, nil
	}
	currentPayload, err := bundle.Lstat("payload")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, true, nil
		}
		return false, true, err
	}
	if currentPayload.Mode()&os.ModeSymlink != 0 || !currentPayload.IsDir() || !os.SameFile(payloadInfo, currentPayload) {
		return false, true, nil
	}
	currentDescriptor, err := bundle.Lstat("bundle.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, true, nil
		}
		return false, true, err
	}
	if currentDescriptor.Mode()&os.ModeSymlink != 0 || !currentDescriptor.Mode().IsRegular() || !os.SameFile(descriptorInfo, currentDescriptor) {
		return false, true, nil
	}
	currentIPA, err := payload.Lstat("app.ipa")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, true, nil
		}
		return false, true, err
	}
	if currentIPA.Mode()&os.ModeSymlink != 0 || !currentIPA.Mode().IsRegular() || !os.SameFile(ipaInfo, currentIPA) {
		return false, true, nil
	}
	return true, true, nil
}

func inspectExactBundle(step string) error {
	if inspectExactBundleForTest != nil {
		return inspectExactBundleForTest(step)
	}
	return nil
}

func stablePreparedFile(before, after os.FileInfo) bool {
	return os.SameFile(before, after) && before.Size() == after.Size() && before.Mode() == after.Mode() && before.ModTime().Equal(after.ModTime())
}

func readDirectory(root *os.Root) ([]os.DirEntry, error) {
	dir, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	entries, readErr := dir.ReadDir(-1)
	closeErr := dir.Close()
	if readErr != nil {
		return nil, readErr
	}
	return entries, closeErr
}

func exactEntries(entries []os.DirEntry, expected map[string]bool) bool {
	if len(entries) != len(expected) {
		return false
	}
	for _, entry := range entries {
		wantDir, ok := expected[entry.Name()]
		if !ok || entry.Type()&os.ModeSymlink != 0 || entry.IsDir() != wantDir {
			return false
		}
		if !wantDir && !entry.Type().IsRegular() {
			return false
		}
	}
	return true
}
