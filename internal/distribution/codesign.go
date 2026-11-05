package distribution

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"debug/macho"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
	"howett.net/plist"
)

const (
	maxMainAppExpandedBytes   int64 = 4 << 30
	maxToolOutputBytes              = 1 << 20
	codeSignInvocationTimeout       = 30 * time.Second
	// CodeSignatureScopeCompleteMainApp is the exact verification scope required
	// before a prepared bundle may be published.
	CodeSignatureScopeCompleteMainApp = "complete-main-app-code-resources-entitlements-and-profile-certificate-binding"
	mainCodeSignatureScope            = CodeSignatureScopeCompleteMainApp
)

var (
	runCodeSignTool                   = runBoundedTool
	duringMaterializationForTest      func(string)
	duringMaterializedCopyForTest     func()
	materializeMainAppForVerification func(*os.Root, []*zip.File, string) error
)

var errCodeVerificationInfrastructure = errors.New("code-signature verification infrastructure failure")

func verifyMainAppCodeSignatureContext(ctx context.Context, members []*zip.File, appDir, executable, bundleID string, profile parsedProfile) CodeSignatureVerification {
	result := CodeSignatureVerification{Status: CodeSignatureNotVerified, Scope: mainCodeSignatureScope}
	if err := contextError(ctx); err != nil {
		result.Reason = err.Error()
		return result
	}
	if runtime.GOOS != "darwin" {
		result.Reason = "complete main-app code-signature verification is available only on macOS"
		return result
	}
	if err := validateExecutableName(strings.TrimSpace(executable)); err != nil {
		result.Status, result.Reason = CodeSignatureInvalid, err.Error()
		return result
	}
	teamID := onlyTrimmed(profile.TeamIdentifier)
	if err := validateTeamIdentifier(teamID); err != nil {
		result.Status, result.Reason = CodeSignatureInvalid, err.Error()
		return result
	}
	directory, err := os.MkdirTemp("", ".asc-distribute-codesign-")
	if err != nil {
		result.Reason = "could not create a private code-signature verification directory"
		return result
	}
	defer os.RemoveAll(directory)
	if err := os.Chmod(directory, 0o700); err != nil {
		result.Reason = "could not secure the code-signature verification directory"
		return result
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		result.Reason = "could not open the code-signature verification directory"
		return result
	}
	defer root.Close()
	if err := root.Mkdir("Verify.app", 0o700); err != nil {
		result.Reason = "could not create the code-signature verification app"
		return result
	}
	app, err := root.OpenRoot("Verify.app")
	if err != nil {
		result.Reason = "could not open the code-signature verification app"
		return result
	}
	defer app.Close()
	materialize := func() error {
		if materializeMainAppForVerification != nil {
			return materializeMainAppForVerification(app, members, appDir)
		}
		return materializeMainAppContext(ctx, app, members, appDir)
	}
	if err := materialize(); err != nil {
		if isRetryableCodeVerificationError(err) {
			result.Reason = "temporary code-signature verification workspace failure"
			return result
		}
		result.Status, result.Reason = CodeSignatureInvalid, "could not safely materialize the complete bounded main app: "+err.Error()
		return result
	}
	appPath := path.Join(directory, "Verify.app")
	if _, err := runCodeSignInvocation(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", "--all-architectures", "--verbose=4", appPath); err != nil {
		if isRetryableCodeVerificationError(err) {
			result.Reason = "codesign verification is temporarily unavailable"
			return result
		}
		result.Status, result.Reason = CodeSignatureInvalid, "codesign rejected the complete main app"
		return result
	}
	mainExecutablePath := filepath.Join(appPath, executable)
	codePaths, err := enumerateMachOFilesContext(ctx, appPath)
	if err != nil {
		if isRetryableCodeVerificationError(err) {
			result.Reason = "temporary code-signature verification workspace failure"
			return result
		}
		result.Status, result.Reason = CodeSignatureInvalid, "could not enumerate nested signed code: "+err.Error()
		return result
	}
	if !slices.Contains(codePaths, mainExecutablePath) {
		result.Status, result.Reason = CodeSignatureInvalid, "CFBundleExecutable is not a Mach-O file in the main app"
		return result
	}
	mainArchitectures, err := verifyMainExecutableEntitlements(ctx, mainExecutablePath, bundleID, profile)
	if err != nil {
		if isRetryableCodeVerificationError(err) {
			result.Status, result.Reason = codeVerificationFailure(err, "main executable")
		} else {
			result.Status, result.Reason = CodeSignatureInvalid, err.Error()
		}
		return result
	}
	allowed := certificateFingerprintSet(profile.DeveloperCertificates)
	mainFingerprints, err := codeObjectFingerprintsForArchitectures(ctx, directory, 0, mainExecutablePath, mainArchitectures)
	if err != nil {
		result.Status, result.Reason = codeVerificationFailure(err, "main executable")
		return result
	}
	for _, fingerprint := range mainFingerprints {
		if _, ok := allowed[fingerprint]; !ok {
			result.Status, result.Reason = CodeSignatureInvalid, "main executable signer is not permitted by the embedded profile"
			return result
		}
	}
	mainFingerprintSet := stringSet(mainFingerprints)
	teamRequirement := `anchor apple generic and certificate leaf[subject.OU] = "` + teamID + `"`
	for index, codePath := range codePaths {
		if err := contextError(ctx); err != nil {
			result.Reason = err.Error()
			return result
		}
		if _, err := runCodeSignInvocation(ctx, "/usr/bin/codesign", "--verify", "--strict", "--all-architectures", "-R="+teamRequirement, codePath); err != nil {
			if isRetryableCodeVerificationError(err) {
				result.Reason = "nested code-signature verification is temporarily unavailable"
				return result
			}
			result.Status, result.Reason = CodeSignatureInvalid, "nested signed code does not satisfy the main app signing-team requirement"
			return result
		}
		if codePath == mainExecutablePath {
			continue
		}
		fingerprints, err := codeObjectFingerprints(ctx, directory, index+1, codePath)
		if err != nil {
			result.Status, result.Reason = codeVerificationFailure(err, "nested signed code")
			return result
		}
		for _, fingerprint := range fingerprints {
			if _, ok := allowed[fingerprint]; !ok {
				result.Status, result.Reason = CodeSignatureInvalid, "nested signed code signer is not permitted by the embedded profile"
				return result
			}
			if _, ok := mainFingerprintSet[fingerprint]; !ok {
				result.Status, result.Reason = CodeSignatureInvalid, "nested signed code signer differs from the main executable signer"
				return result
			}
		}
	}
	result.Status = CodeSignatureVerified
	result.Reason = "complete main app, every nested Mach-O code object, signed entitlements, and profile certificate binding verified"
	result.SignerCertificateSHA256Fingerprints = canonicalSet(mainFingerprints)
	return result
}

func verifyMainExecutableEntitlements(ctx context.Context, codePath, bundleID string, profile parsedProfile) ([]string, error) {
	architectures, err := codeObjectArchitectures(ctx, codePath)
	if err != nil {
		return nil, err
	}
	for _, architecture := range architectures {
		entitlementsData, err := runCodeSignInvocation(
			ctx,
			"/usr/bin/codesign",
			"-d",
			"-a",
			architecture,
			"--entitlements",
			":-",
			codePath,
		)
		if err != nil {
			return nil, fmt.Errorf("could not extract signed main-app entitlements for architecture %s: %w", architecture, err)
		}
		if err := validateSignedMainAppEntitlements(entitlementsData, bundleID, profile); err != nil {
			return nil, err
		}
	}
	return architectures, nil
}

func validateSignedMainAppEntitlements(entitlementsData []byte, bundleID string, profile parsedProfile) error {
	var entitlements map[string]any
	if err := decodeBoundedPlist(entitlementsData, &entitlements); err != nil {
		return fmt.Errorf("signed main-app entitlements are invalid")
	}
	appIdentifier, ok := entitlements["application-identifier"].(string)
	if !ok || strings.TrimSpace(appIdentifier) == "" {
		return fmt.Errorf("signed main-app application identifier is missing")
	}
	teamIdentifier, ok := entitlements["com.apple.developer.team-identifier"].(string)
	if !ok || strings.TrimSpace(teamIdentifier) == "" {
		return fmt.Errorf("signed main-app team identifier is missing")
	}
	teamID := onlyTrimmed(profile.TeamIdentifier)
	if err := validateTeamIdentifier(teamID); err != nil {
		return err
	}
	applicationIdentifierPrefix := declaredSingle(profile.ApplicationIdentifierPrefix)
	if err := validateApplicationIdentifierPrefix(applicationIdentifierPrefix); err != nil {
		return err
	}
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return fmt.Errorf("inspected CFBundleIdentifier is missing")
	}
	profileApplicationID, _ := profile.Entitlements["application-identifier"].(string)
	if teamIdentifier != teamID || !entitlementValuePermits(profileApplicationID, appIdentifier) {
		return fmt.Errorf("signed main-app entitlements do not match the embedded profile team and application identifier")
	}
	if appIdentifier != applicationIdentifierPrefix+"."+bundleID {
		return fmt.Errorf("signed main-app application identifier does not match inspected CFBundleIdentifier")
	}
	if profileDebug, ok := profile.Entitlements["get-task-allow"].(bool); !ok {
		return fmt.Errorf("embedded profile get-task-allow entitlement is missing or invalid")
	} else if signedDebug, exists := entitlements["get-task-allow"]; exists {
		debug, ok := signedDebug.(bool)
		if !ok || debug != profileDebug {
			return fmt.Errorf("signed get-task-allow entitlement is not permitted by the embedded profile")
		}
	} else if profileDebug {
		return fmt.Errorf("signed get-task-allow entitlement is missing for a development profile")
	}
	for key, signedValue := range entitlements {
		profileValue, exists := profile.Entitlements[key]
		if !exists || !entitlementValuePermits(profileValue, signedValue) {
			return fmt.Errorf("signed main-app entitlement is not permitted by the embedded profile: %s", key)
		}
	}
	return nil
}

func enumerateMachOFilesContext(ctx context.Context, appPath string) ([]string, error) {
	var result []string
	err := filepath.WalkDir(appPath, func(candidate string, entry os.DirEntry, walkErr error) error {
		if err := contextError(ctx); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link found in materialized app")
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular file found in materialized app")
		}
		file, err := os.Open(candidate)
		if err != nil {
			return err
		}
		var magic [4]byte
		_, readErr := io.ReadFull(file, magic[:])
		isMachO := false
		var classificationErr error
		if readErr == nil {
			isMachO, classificationErr = classifyMachOFile(file, magic, info.Size())
		}
		closeErr := file.Close()
		if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if classificationErr != nil {
			return fmt.Errorf("classify Mach-O file %q: %w", candidate, classificationErr)
		}
		if isMachO {
			result = append(result, candidate)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(result)
	return result, nil
}

func classifyMachOFile(file *os.File, magic [4]byte, fileSize int64) (bool, error) {
	switch magic {
	case [4]byte{0xfe, 0xed, 0xfa, 0xce}, [4]byte{0xce, 0xfa, 0xed, 0xfe},
		[4]byte{0xfe, 0xed, 0xfa, 0xcf}, [4]byte{0xcf, 0xfa, 0xed, 0xfe}:
		return isLoadableThinMachO(file, fileSize), nil
	case [4]byte{0xca, 0xfe, 0xba, 0xbe}:
		return classifyFatMachO(file, fileSize, binary.BigEndian, 20)
	case [4]byte{0xbe, 0xba, 0xfe, 0xca}:
		return classifyFatMachO(file, fileSize, binary.LittleEndian, 20)
	case [4]byte{0xca, 0xfe, 0xba, 0xbf}:
		return classifyFatMachO(file, fileSize, binary.BigEndian, 32)
	case [4]byte{0xbf, 0xba, 0xfe, 0xca}:
		return classifyFatMachO(file, fileSize, binary.LittleEndian, 32)
	default:
		return false, nil
	}
}

func isLoadableThinMachO(file *os.File, fileSize int64) bool {
	if fileSize <= 0 {
		return false
	}
	image, err := macho.NewFile(io.NewSectionReader(file, 0, fileSize))
	return err == nil && isLoadableMachOImage(image)
}

func isLoadableMachOImage(file *macho.File) bool {
	switch file.Type {
	case macho.TypeExec, macho.TypeDylib, macho.TypeBundle:
		return true
	default:
		return false
	}
}

func classifyFatMachO(file *os.File, fileSize int64, order binary.ByteOrder, archHeaderSize int64) (bool, error) {
	var countBytes [4]byte
	if _, err := file.ReadAt(countBytes[:], 4); err != nil {
		return false, nil
	}
	architectureCount := order.Uint32(countBytes[:])
	if architectureCount == 0 || architectureCount > 64 {
		return false, nil
	}
	tableEnd := int64(8) + int64(architectureCount)*archHeaderSize
	if tableEnd > fileSize {
		return false, nil
	}
	header := make([]byte, archHeaderSize)
	hasLoadableSlice := false
	hasNonLoadableSlice := false
	for index := uint32(0); index < architectureCount; index++ {
		if _, err := file.ReadAt(header, int64(8)+int64(index)*archHeaderSize); err != nil {
			return false, nil
		}
		var offset, size uint64
		if archHeaderSize == 20 {
			offset = uint64(order.Uint32(header[8:12]))
			size = uint64(order.Uint32(header[12:16]))
		} else {
			offset = order.Uint64(header[8:16])
			size = order.Uint64(header[16:24])
		}
		if offset < uint64(tableEnd) || size == 0 || offset > uint64(fileSize) || size > uint64(fileSize)-offset {
			return false, nil
		}
		slice, err := macho.NewFile(io.NewSectionReader(file, int64(offset), int64(size)))
		if err != nil {
			return false, fmt.Errorf("fat Mach-O contains a malformed architecture slice")
		}
		if uint32(slice.Cpu) != order.Uint32(header[:4]) {
			return false, fmt.Errorf("fat Mach-O architecture header does not match its slice")
		}
		if isLoadableMachOImage(slice) {
			hasLoadableSlice = true
		} else {
			hasNonLoadableSlice = true
		}
		if hasLoadableSlice && hasNonLoadableSlice {
			return false, fmt.Errorf("fat Mach-O mixes loadable and non-loadable image types")
		}
	}
	return hasLoadableSlice, nil
}

func codeObjectFingerprints(ctx context.Context, directory string, objectIndex int, codePath string) ([]string, error) {
	architectures, err := codeObjectArchitectures(ctx, codePath)
	if err != nil {
		return nil, err
	}
	return codeObjectFingerprintsForArchitectures(ctx, directory, objectIndex, codePath, architectures)
}

func codeObjectArchitectures(ctx context.Context, codePath string) ([]string, error) {
	archOutput, err := runCodeSignInvocation(ctx, "/usr/bin/lipo", "-archs", codePath)
	if err != nil {
		return nil, err
	}
	architectures := strings.Fields(string(archOutput))
	if len(architectures) == 0 || len(architectures) > 64 {
		return nil, fmt.Errorf("invalid architecture list")
	}
	seen := make(map[string]struct{}, len(architectures))
	for _, architecture := range architectures {
		if err := validateArchitecture(architecture); err != nil {
			return nil, err
		}
		if _, exists := seen[architecture]; exists {
			return nil, fmt.Errorf("duplicate architecture")
		}
		seen[architecture] = struct{}{}
	}
	return architectures, nil
}

func codeObjectFingerprintsForArchitectures(ctx context.Context, directory string, objectIndex int, codePath string, architectures []string) ([]string, error) {
	var fingerprints []string
	for architectureIndex, architecture := range architectures {
		prefix := path.Join(directory, fmt.Sprintf("certificate-%d-%d-", objectIndex, architectureIndex))
		if _, err := runCodeSignInvocation(ctx, "/usr/bin/codesign", "-d", "-a", architecture, "--extract-certificates="+prefix, codePath); err != nil {
			return nil, err
		}
		leaf, err := os.ReadFile(prefix + "0")
		if err != nil {
			return nil, err
		}
		fingerprint, err := certificateFingerprint(leaf)
		if err != nil {
			return nil, err
		}
		fingerprints = append(fingerprints, fingerprint)
	}
	return canonicalSet(fingerprints), nil
}

func codeVerificationFailure(err error, subject string) (CodeSignatureVerificationStatus, string) {
	if isRetryableCodeVerificationError(err) {
		return CodeSignatureNotVerified, "required code-signature verification is temporarily unavailable"
	}
	return CodeSignatureInvalid, "could not verify " + subject + " architectures and signer certificates"
}

func isRetryableCodeVerificationError(err error) bool {
	if err == nil {
		return false
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ProcessState == nil || !exitError.Exited()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) ||
		errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrClosed) ||
		errors.Is(err, errCodeVerificationInfrastructure) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.EAGAIN, syscall.EINTR, syscall.EIO, syscall.ENOSPC,
			syscall.EDQUOT, syscall.EMFILE, syscall.ENFILE, syscall.EBUSY,
			syscall.ETIMEDOUT, syscall.ENOMEM, syscall.ESTALE:
			return true
		default:
			return false
		}
	}
	var pathError *os.PathError
	if errors.As(err, &pathError) {
		return true
	}
	var linkError *os.LinkError
	return errors.As(err, &linkError)
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func entitlementValuePermits(profileValue, signedValue any) bool {
	profileString, profileIsString := profileValue.(string)
	signedString, signedIsString := signedValue.(string)
	if profileIsString && signedIsString {
		if strings.HasSuffix(profileString, "*") {
			prefix := strings.TrimSuffix(profileString, "*")
			return strings.HasPrefix(signedString, prefix) && len(signedString) > len(prefix)
		}
		return signedString == profileString
	}
	profileList, profileIsList := entitlementList(profileValue)
	signedList, signedIsList := entitlementList(signedValue)
	if profileIsList && signedIsList {
		for _, signedItem := range signedList {
			permitted := false
			for _, profileItem := range profileList {
				if entitlementValuePermits(profileItem, signedItem) {
					permitted = true
					break
				}
			}
			if !permitted {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(profileValue, signedValue)
}

func entitlementList(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []string:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = item
		}
		return result, true
	default:
		return nil, false
	}
}

func materializeMainAppContext(ctx context.Context, destination *os.Root, members []*zip.File, appDir string) error {
	prefix := appDir + "/"
	var total int64
	for _, member := range members {
		if duringMaterializationForTest != nil {
			duringMaterializationForTest(member.Name)
		}
		if err := contextError(ctx); err != nil {
			return err
		}
		if !strings.HasPrefix(member.Name, prefix) {
			continue
		}
		relative := strings.TrimPrefix(member.Name, prefix)
		if relative == "" {
			continue
		}
		if member.FileInfo().IsDir() {
			if err := destination.MkdirAll(strings.TrimSuffix(relative, "/"), 0o700); err != nil {
				return err
			}
			continue
		}
		if member.UncompressedSize64 > uint64(maxMainAppExpandedBytes) || total > maxMainAppExpandedBytes-int64(member.UncompressedSize64) {
			return fmt.Errorf("expanded main app exceeds %d bytes", maxMainAppExpandedBytes)
		}
		total += int64(member.UncompressedSize64)
		if err := destination.MkdirAll(path.Dir(relative), 0o700); err != nil {
			return err
		}
		if err := copyZipMemberToNewFileContext(ctx, destination, relative, member, int64(member.UncompressedSize64)); err != nil {
			return err
		}
	}
	return nil
}

func decodeBoundedPlist(data []byte, destination any) error {
	if len(data) == 0 || len(data) > 4<<20 {
		return fmt.Errorf("plist size is invalid")
	}
	if err := infoplist.ValidateStructure(data); err != nil {
		return err
	}
	_, err := plist.Unmarshal(data, destination)
	return err
}

func certificateFingerprintSet(certificates [][]byte) map[string]struct{} {
	result := make(map[string]struct{}, len(certificates))
	for _, certificate := range certificates {
		if fingerprint, err := certificateFingerprint(certificate); err == nil {
			result[fingerprint] = struct{}{}
		}
	}
	return result
}

func certificateFingerprint(data []byte) (string, error) {
	if _, err := x509.ParseCertificate(data); err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func validateExecutableName(value string) error {
	if value == "" || len(value) > 255 || path.Base(value) != value || value == "." || value == ".." {
		return fmt.Errorf("CFBundleExecutable is not a single safe filename")
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.In(r, unicode.Bidi_Control) {
			return fmt.Errorf("CFBundleExecutable contains control or formatting characters")
		}
	}
	return nil
}

func validateTeamIdentifier(value string) error {
	if value == "" || len(value) > 32 {
		return fmt.Errorf("embedded profile team identifier is not a single safe value")
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return fmt.Errorf("embedded profile team identifier contains unsupported characters")
		}
	}
	return nil
}

func validateApplicationIdentifierPrefix(value string) error {
	if value == "" || len(value) > 32 {
		return fmt.Errorf("embedded profile application identifier prefix is not a single safe value")
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return fmt.Errorf("embedded profile application identifier prefix contains unsupported characters")
		}
	}
	return nil
}

func validateArchitecture(value string) error {
	if value == "" || len(value) > 64 {
		return fmt.Errorf("invalid architecture")
	}
	for _, r := range value {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return fmt.Errorf("invalid architecture")
		}
	}
	return nil
}

func copyZipMemberToNewFileContext(ctx context.Context, root *os.Root, name string, member *zip.File, limit int64) error {
	reader, err := member.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	file, err := secureopen.OpenNewFileNoFollowInRoot(root, name, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := copyWithContext(ctx, file, io.LimitReader(reader, limit+1), duringMaterializedCopyForTest)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > limit {
		return fmt.Errorf("expanded member exceeds %d bytes", limit)
	}
	return nil
}

func runCodeSignInvocation(parent context.Context, name string, args ...string) ([]byte, error) {
	if err := contextError(parent); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(parent, codeSignInvocationTimeout)
	defer cancel()
	return runCodeSignTool(ctx, name, args...)
}

func runBoundedTool(ctx context.Context, name string, args ...string) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, name, args...)
	pipe, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: open tool stdout: %w", errCodeVerificationInfrastructure, err)
	}
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		if isRetryableCodeVerificationError(err) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: start tool: %w", errCodeVerificationInfrastructure, err)
	}
	var outputBuffer bytes.Buffer
	_, readErr := copyWithContext(ctx, &outputBuffer, io.LimitReader(pipe, maxToolOutputBytes+1), nil)
	waitErr := command.Wait()
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	output := outputBuffer.Bytes()
	if len(output) > maxToolOutputBytes {
		return nil, fmt.Errorf("%w: tool output exceeded %d bytes", errCodeVerificationInfrastructure, maxToolOutputBytes)
	}
	if readErr != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: read tool output: %w", errCodeVerificationInfrastructure, readErr)
	}
	if waitErr != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var exitError *exec.ExitError
		if errors.As(waitErr, &exitError) {
			return nil, waitErr
		}
		return nil, fmt.Errorf("%w: wait for tool: %w", errCodeVerificationInfrastructure, waitErr)
	}
	return output, nil
}
