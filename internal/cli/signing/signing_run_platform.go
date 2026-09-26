//go:build darwin

package signing

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

const (
	signingRunDiagnosticLimit     = 64 << 10
	signingUtilityDiagnosticLimit = 4 << 10
	signingRunChildWaitDelay      = 5 * time.Second
	signingRunLockPollInterval    = 50 * time.Millisecond
	signingRunXcodebuildPath      = "/usr/bin/xcodebuild"
)

var errSigningRunStagedProfileChanged = errors.New("staged provisioning profile changed")

var (
	signingRunActiveXcodeMajorVersion     = activeSigningRunXcodeMajorVersion
	signingRunCommandContext              = exec.CommandContext
	signingRunProfileInstallDirFn         = signingRunProfileInstallDir
	signingRunStateAnchorFn               = openSigningRunStateAnchor
	signingRunRecoveryRemoveSearchEntryFn = removeKeychainSearchEntry
	signingRunRecoveryDeleteKeychainFn    = deleteSigningRunKeychain
	signingRunKillProcessGroupFn          = func(pid int, signal syscall.Signal) error {
		return syscall.Kill(-pid, signal)
	}
)

func platformSigningRunDeps() signingRunDeps {
	return signingRunDeps{
		GOOS:                      "darwin",
		Stderr:                    os.Stderr,
		RandomBytes:               signingRunRandomBytes,
		TempDir:                   createSigningRunTempDir,
		RemoveTempDir:             removeSigningRunTempDir,
		AcquireLock:               acquireSigningRunLock,
		Recover:                   recoverSigningRunJournal,
		WriteJournal:              writeSigningRunJournal,
		RemoveJournal:             removeSigningRunJournal,
		KeychainSearchList:        keychainSearchList,
		CreateKeychain:            createSigningRunKeychain,
		ImportIdentity:            importSigningRunIdentity,
		SetKeychainSearchList:     setKeychainSearchList,
		RemoveKeychainSearchEntry: removeKeychainSearchEntry,
		DeleteKeychain:            deleteSigningRunKeychain,
		InstallProfile:            installSigningRunProfile,
		RemoveProfile:             removeSigningRunProfile,
		RunChild:                  runSigningRunChild,
	}
}

func signingRunRandomBytes(size int) ([]byte, error) {
	if size <= 0 {
		return nil, fmt.Errorf("random byte count must be positive")
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, data); err != nil {
		return nil, err
	}
	return data, nil
}

func createSigningRunTempDir() (string, error) {
	path, err := os.MkdirTemp("", "asc-signing-run.")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0o700); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func removeSigningRunTempDir(path string) error {
	cleanPath := filepath.Clean(path)
	if filepath.Dir(cleanPath) != filepath.Clean(os.TempDir()) ||
		!strings.HasPrefix(filepath.Base(cleanPath), "asc-signing-run.") {
		return fmt.Errorf("refusing to remove unexpected signing directory %q", path)
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("refusing to remove unsafe signing directory %q", path)
	}
	directoryRoot, err := rootfs.New(cleanPath)
	if err != nil {
		return err
	}
	defer directoryRoot.Close()
	rooted, err := directoryRoot.OpenRoot()
	if err != nil {
		return err
	}
	defer rooted.Close()
	openedInfo, err := rooted.Stat(".")
	if err != nil {
		return err
	}
	openedStat, openedOK := openedInfo.Sys().(*syscall.Stat_t)
	initialStat, initialOK := info.Sys().(*syscall.Stat_t)
	if !openedOK || !initialOK || openedStat.Dev != initialStat.Dev || openedStat.Ino != initialStat.Ino {
		return fmt.Errorf("refusing to remove signing directory because its identity changed")
	}
	parentRoot, err := rootfs.New(filepath.Dir(cleanPath))
	if err != nil {
		return err
	}
	defer parentRoot.Close()
	parented, err := parentRoot.OpenRoot()
	if err != nil {
		return err
	}
	defer parented.Close()
	directory, err := rooted.Open(".")
	if err != nil {
		return err
	}
	entries, err := directory.ReadDir(-1)
	closeErr := directory.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to remove unexpected non-regular entry %q", entry.Name())
		}
	}
	for _, entry := range entries {
		if err := rooted.Remove(entry.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := rooted.Close(); err != nil {
		return err
	}
	return parented.Remove(filepath.Base(cleanPath))
}

type signingRunStateAnchor struct {
	root     rootfs.Root
	relative string
}

func openSigningRunStateAnchor() (*signingRunStateAnchor, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	cacheRoot, err := rootfs.New(cacheDir)
	if err != nil {
		return nil, err
	}
	const relative = "asc/signing-run/v1"
	if err := cacheRoot.MkdirAll(relative, 0o700); err != nil {
		_ = cacheRoot.Close()
		return nil, err
	}
	_, err = cacheRoot.Resolve(relative)
	if err != nil {
		_ = cacheRoot.Close()
		return nil, err
	}
	stateDir, err := cacheRoot.OpenDir(relative)
	if err != nil {
		_ = cacheRoot.Close()
		return nil, err
	}
	chmodErr := stateDir.Chmod(0o700)
	closeErr := stateDir.Close()
	if chmodErr != nil || closeErr != nil {
		_ = cacheRoot.Close()
		return nil, errors.Join(chmodErr, closeErr)
	}
	// Keep one cache-root anchor for every lock/journal operation; callers must
	// not reopen the state directory through a path after this function returns.
	return &signingRunStateAnchor{root: cacheRoot, relative: relative}, nil
}

func (anchor *signingRunStateAnchor) relativePath(name string) string {
	if anchor.relative == "." || anchor.relative == "" {
		return name
	}
	return filepath.Join(anchor.relative, name)
}

func (anchor *signingRunStateAnchor) removeFile(name string) error {
	file, err := anchor.root.OpenFile(anchor.relativePath(name))
	if err != nil {
		return err
	}
	info, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, signingRunDiagnosticLimit+1))
	closeErr := file.Close()
	if statErr != nil || readErr != nil || closeErr != nil {
		return errors.Join(statErr, readErr, closeErr)
	}
	return anchor.root.RemoveFileIfSame(anchor.relativePath(name), info, data)
}

func writeSigningRunJournal(journal signingRunJournal, overwrite bool) error {
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	anchor, err := signingRunStateAnchorFn()
	if err != nil {
		return err
	}
	defer anchor.root.Close()
	if !overwrite {
		return anchor.root.CreateNewFile(anchor.relativePath("journal.json"), data, 0o600)
	}
	return anchor.root.WriteFile(anchor.relativePath("journal.json"), data, 0o600)
}

func removeSigningRunJournalWithAnchor(anchor *signingRunStateAnchor) error {
	if anchor == nil {
		return fmt.Errorf("remove signing journal: state anchor is required")
	}
	err := anchor.removeFile("journal.json")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func removeSigningRunJournal() error {
	anchor, err := signingRunStateAnchorFn()
	if err != nil {
		return err
	}
	defer anchor.root.Close()
	return removeSigningRunJournalWithAnchor(anchor)
}

func recoverSigningRunJournal(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("recover signing environment: context is required")
	}
	anchor, err := signingRunStateAnchorFn()
	if err != nil {
		return err
	}
	defer anchor.root.Close()
	file, err := anchor.root.OpenFile(anchor.relativePath("journal.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	info, statErr := file.Stat()
	if statErr != nil {
		_ = file.Close()
		return statErr
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() || stat.Nlink != 1 || info.Mode().Perm() != 0o600 {
		_ = file.Close()
		return fmt.Errorf("%w: ownership or permissions", ErrEphemeralRecoveryJournalInvalid)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, signingRunDiagnosticLimit+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return errors.Join(readErr, closeErr)
	}
	if len(data) > signingRunDiagnosticLimit {
		return fmt.Errorf("%w: size limit exceeded", ErrEphemeralRecoveryJournalInvalid)
	}
	if err := rejectDuplicateSigningRunJSONKeys(data); err != nil {
		return fmt.Errorf("%w: %w", ErrEphemeralRecoveryJournalInvalid, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var journal signingRunJournal
	if err := decoder.Decode(&journal); err != nil {
		return fmt.Errorf("%w: %w", ErrEphemeralRecoveryJournalInvalid, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return fmt.Errorf("%w: %w", ErrEphemeralRecoveryJournalInvalid, err)
	}
	if err := validateSigningRunJournal(journal); err != nil {
		return fmt.Errorf("%w: %w", ErrEphemeralRecoveryJournalInvalid, err)
	}
	var recoveryErr error
	var preservedStagedProfileErr error
	if journal.ProfileCreated {
		stagedDigest := journal.StagedProfileDigest
		if stagedDigest == "" {
			// Journals created before staged digests were recorded can only prove
			// ownership of a fully written staged profile.
			stagedDigest = journal.ProfileDigest
		}
		stagedProfileErr := removeSigningRunStagedProfile(
			journal.StagedProfilePath, journal.ProfileDevice, journal.ProfileInode, stagedDigest,
		)
		if errors.Is(stagedProfileErr, errSigningRunStagedProfileChanged) {
			preservedStagedProfileErr = fmt.Errorf("preserved changed staged provisioning profile: %w", stagedProfileErr)
		} else {
			recoveryErr = errors.Join(recoveryErr, stagedProfileErr)
		}
		recoveryErr = errors.Join(recoveryErr, removeSigningRunProfile(signingRunProfileInstall{
			Path: journal.ProfilePath, Created: true, Digest: journal.ProfileDigest,
			Device: journal.ProfileDevice, Inode: journal.ProfileInode,
		}))
	}
	recoveryErr = errors.Join(recoveryErr, signingRunRecoveryRemoveSearchEntryFn(ctx, journal.KeychainPath))
	recoveryErr = errors.Join(recoveryErr, signingRunRecoveryDeleteKeychainFn(ctx, journal.KeychainPath))
	if recoveryErr != nil {
		return recoveryErr
	}
	if err := removeSigningRunTempDir(journal.TempDir); err != nil {
		return err
	}
	if err := removeSigningRunJournalWithAnchor(anchor); err != nil {
		return err
	}
	return preservedStagedProfileErr
}

func validateSigningRunJournal(journal signingRunJournal) error {
	if journal.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema version %d", journal.SchemaVersion)
	}
	tempDir := filepath.Clean(journal.TempDir)
	if filepath.Dir(tempDir) != filepath.Clean(os.TempDir()) ||
		!strings.HasPrefix(filepath.Base(tempDir), "asc-signing-run.") {
		return fmt.Errorf("temporary directory is outside the signing runtime root")
	}
	if filepath.Clean(journal.KeychainPath) != filepath.Join(tempDir, "signing.keychain-db") {
		return fmt.Errorf("keychain path does not match the temporary directory")
	}
	if !journal.ProfileCreated {
		if journal.ProfilePath != "" || journal.StagedProfilePath != "" || journal.ProfileDigest != "" || journal.StagedProfileDigest != "" ||
			journal.ProfileDevice != 0 || journal.ProfileInode != 0 {
			return fmt.Errorf("profile recovery fields are inconsistent")
		}
		return nil
	}
	if len(journal.ProfileDigest) != sha256.Size*2 {
		return fmt.Errorf("profile digest is invalid")
	}
	if _, err := hex.DecodeString(journal.ProfileDigest); err != nil {
		return fmt.Errorf("profile digest is invalid")
	}
	if journal.StagedProfileDigest != "" {
		if len(journal.StagedProfileDigest) != sha256.Size*2 {
			return fmt.Errorf("staged profile digest is invalid")
		}
		if _, err := hex.DecodeString(journal.StagedProfileDigest); err != nil {
			return fmt.Errorf("staged profile digest is invalid")
		}
	}
	if journal.ProfileDevice == 0 || journal.ProfileInode == 0 {
		return fmt.Errorf("profile file identity is missing")
	}
	base := filepath.Base(journal.ProfilePath)
	uuid := strings.TrimSuffix(base, ".mobileprovision")
	if base == uuid || !signingRunUUIDPattern.MatchString(uuid) {
		return fmt.Errorf("profile path has an invalid UUID")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	allowedDirs := []string{
		filepath.Join(homeDir, "Library", "Developer", "Xcode", "UserData", "Provisioning Profiles"),
		filepath.Join(homeDir, "Library", "MobileDevice", "Provisioning Profiles"),
	}
	for _, dir := range allowedDirs {
		stagedBase := filepath.Base(journal.StagedProfilePath)
		if filepath.Clean(journal.ProfilePath) == filepath.Join(dir, base) &&
			filepath.Clean(journal.StagedProfilePath) == filepath.Join(dir, stagedBase) &&
			strings.HasPrefix(stagedBase, ".asc-signing-run-profile-") {
			return nil
		}
	}
	return fmt.Errorf("profile path is outside an Xcode provisioning profile directory")
}

func acquireSigningRunLock(ctx context.Context) (func() error, error) {
	if ctx == nil {
		return nil, fmt.Errorf("acquire signing environment lock: context is required")
	}
	anchor, err := signingRunStateAnchorFn()
	if err != nil {
		return nil, err
	}
	lockName := anchor.relativePath("lock")
	file, err := anchor.root.OpenFile(lockName)
	if errors.Is(err, os.ErrNotExist) {
		rooted, rootErr := anchor.root.OpenRoot()
		if rootErr != nil {
			_ = anchor.root.Close()
			return nil, rootErr
		}
		file, err = secureopen.OpenNewFileNoFollowInRoot(rooted, lockName, 0o600)
		closeRootErr := rooted.Close()
		if errors.Is(err, os.ErrExist) {
			file, err = anchor.root.OpenFile(lockName)
		}
		if err == nil && closeRootErr != nil {
			_ = file.Close()
			err = closeRootErr
		}
	}
	if err != nil {
		_ = anchor.root.Close()
		return nil, err
	}
	for {
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			var releaseOnce sync.Once
			var releaseErr error
			return func() error {
				releaseOnce.Do(func() {
					releaseErr = errors.Join(
						unix.Flock(int(file.Fd()), unix.LOCK_UN),
						file.Close(),
						anchor.root.Close(),
					)
				})
				return releaseErr
			}, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			_ = file.Close()
			_ = anchor.root.Close()
			return nil, err
		}
		timer := time.NewTimer(signingRunLockPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = file.Close()
			_ = anchor.root.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func keychainSearchList(ctx context.Context) ([]string, error) {
	stdout, stderr, err := runSigningUtility(ctx, nil, "list-keychains", "-d", "user")
	if err != nil {
		return nil, utilityFailure("read keychain search list", stderr, err)
	}
	return parseKeychainSearchList(stdout)
}

func parseKeychainSearchList(data []byte) ([]string, error) {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		value, err := strconv.Unquote(line)
		if err != nil {
			return nil, fmt.Errorf("parse keychain search list entry: %w", err)
		}
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("parse keychain search list entry: empty path")
		}
		paths = append(paths, value)
	}
	return paths, nil
}

func createSigningRunKeychain(ctx context.Context, keychainPath string, password []byte) error {
	passwordHex := []byte(hex.EncodeToString(password))
	defer clear(passwordHex)
	if err := createKeychainWithSecurityFramework(keychainPath, passwordHex); err != nil {
		return fmt.Errorf("create keychain: %w", err)
	}
	_, stderr, err := runSigningUtility(ctx, nil, "set-keychain-settings", "-l", keychainPath)
	if err != nil {
		return utilityFailure("configure keychain", stderr, err)
	}
	return nil
}

func importSigningRunIdentity(ctx context.Context, keychainPath string, keychainPassword, identityData, importPassword []byte, expectedSHA1 string) error {
	if err := importPKCS12WithSecurityFramework(keychainPath, identityData, importPassword); err != nil {
		return fmt.Errorf("import identity: %w", err)
	}
	if err := withSigningRunPartitionPasswordInput(keychainPassword, func(stdin []byte) error {
		_, stderr, err := runSigningUtility(ctx, stdin, "set-key-partition-list", "-S", "apple-tool:,apple:", "-s", "-t", "private", keychainPath)
		if err != nil {
			return utilityFailure("restrict key partition list", stderr, err, keychainPassword)
		}
		return nil
	}); err != nil {
		return err
	}
	stdout, stderr, err := runSigningUtility(ctx, nil, "find-certificate", "-a", "-Z", keychainPath)
	if err != nil {
		return utilityFailure("verify imported certificate", stderr, err)
	}
	certificates := parseSigningRunCertificateFingerprints(stdout)
	if len(certificates) != 1 || !strings.EqualFold(certificates[0], expectedSHA1) {
		return fmt.Errorf("verify imported certificate: expected only certificate %s, found %v", expectedSHA1, certificates)
	}
	_, stderr, err = runSigningUtility(ctx, nil, "find-key", "-s", "-t", "private", keychainPath)
	if err != nil {
		return utilityFailure("verify imported private key", stderr, err)
	}
	return verifySigningRunIdentityUsable(ctx, filepath.Dir(keychainPath), keychainPath, expectedSHA1)
}

func withSigningRunPartitionPasswordInput(keychainPassword []byte, operation func([]byte) error) error {
	stdin := make([]byte, hex.EncodedLen(len(keychainPassword))+1)
	hex.Encode(stdin[:len(stdin)-1], keychainPassword)
	stdin[len(stdin)-1] = '\n'
	defer clear(stdin)
	return operation(stdin)
}

func verifySigningRunIdentityUsable(ctx context.Context, tempDir, keychainPath, expectedSHA1 string) error {
	source, err := os.Open("/usr/bin/true")
	if err != nil {
		return fmt.Errorf("open codesign probe: %w", err)
	}
	defer source.Close()
	tempRoot, err := rootfs.New(tempDir)
	if err != nil {
		return err
	}
	defer tempRoot.Close()
	const probeName = "codesign-probe"
	if _, err := tempRoot.WriteFrom(probeName, io.LimitReader(source, signingRunInputLimit), 0o700); err != nil {
		return fmt.Errorf("create codesign probe: %w", err)
	}
	probePath := filepath.Join(tempDir, probeName)
	defer func() {
		if rooted, openErr := tempRoot.OpenRoot(); openErr == nil {
			_ = rooted.Remove(probeName)
			_ = rooted.Close()
		}
	}()
	cmd := signingRunCommandContext(ctx, "/usr/bin/codesign", "--force", "--sign", expectedSHA1, "--keychain", keychainPath, probePath)
	cmd.Env = SanitizedChildEnvironment(os.Environ())
	stdout := &limitedSigningBuffer{limit: signingRunDiagnosticLimit}
	stderr := &limitedSigningBuffer{limit: signingRunDiagnosticLimit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return utilityFailure("verify imported signing identity", stderr.Bytes(), err)
	}
	return nil
}

func parseSigningRunCertificateFingerprints(data []byte) []string {
	var fingerprints []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		const prefix = "SHA-1 hash: "
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		fingerprint := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if len(fingerprint) != 40 {
			continue
		}
		if _, err := hex.DecodeString(fingerprint); err == nil {
			fingerprints = append(fingerprints, strings.ToUpper(fingerprint))
		}
	}
	return fingerprints
}

func setKeychainSearchList(ctx context.Context, paths []string) error {
	args := []string{"list-keychains", "-d", "user", "-s"}
	args = append(args, paths...)
	_, stderr, err := runSigningUtility(ctx, nil, args...)
	if err != nil {
		return utilityFailure("set keychain search list", stderr, err)
	}
	return nil
}

func removeKeychainSearchEntry(ctx context.Context, keychainPath string) error {
	paths, err := keychainSearchList(ctx)
	if err != nil {
		return err
	}
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if path != keychainPath {
			filtered = append(filtered, path)
		}
	}
	if len(filtered) == len(paths) {
		return nil
	}
	return setKeychainSearchList(ctx, filtered)
}

func deleteSigningRunKeychain(ctx context.Context, keychainPath string) error {
	_, stderr, err := runSigningUtility(ctx, nil, "delete-keychain", keychainPath)
	if err != nil && !strings.Contains(string(stderr), "could not be found") {
		return utilityFailure("delete keychain", stderr, err)
	}
	return nil
}

func installSigningRunProfile(ctx context.Context, uuid string, data []byte, digest string, recordCreate func(signingRunProfileInstall) error) (result signingRunProfileInstall, resultErr error) {
	if ctx == nil {
		return signingRunProfileInstall{}, fmt.Errorf("install provisioning profile: context is required")
	}
	if err := ctx.Err(); err != nil {
		return signingRunProfileInstall{}, err
	}
	installDir, err := signingRunProfileInstallDirFn(ctx)
	if err != nil {
		return signingRunProfileInstall{}, err
	}
	if err := ctx.Err(); err != nil {
		return signingRunProfileInstall{}, err
	}
	installRoot, err := rootfs.New(installDir)
	if err != nil {
		return signingRunProfileInstall{}, err
	}
	defer installRoot.Close()
	if err := ctx.Err(); err != nil {
		return signingRunProfileInstall{}, err
	}
	if err := installRoot.MkdirAll(".", 0o755); err != nil {
		return signingRunProfileInstall{}, err
	}
	name := strings.ToLower(uuid) + ".mobileprovision"
	path := filepath.Join(installDir, name)
	existingFile, err := installRoot.OpenFile(name)
	if err == nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			_ = existingFile.Close()
			return signingRunProfileInstall{}, ctxErr
		}
		info, statErr := existingFile.Stat()
		if statErr != nil {
			_ = existingFile.Close()
			return signingRunProfileInstall{}, statErr
		}
		if info.Size() > signingRunInputLimit {
			_ = existingFile.Close()
			return signingRunProfileInstall{}, fmt.Errorf("profile destination exceeds the size limit")
		}
		existing, readErr := io.ReadAll(io.LimitReader(existingFile, signingRunInputLimit+1))
		closeErr := existingFile.Close()
		if readErr != nil || closeErr != nil {
			return signingRunProfileInstall{}, errors.Join(readErr, closeErr)
		}
		if len(existing) > signingRunInputLimit {
			return signingRunProfileInstall{}, fmt.Errorf("profile destination exceeds the size limit")
		}
		existingDigest := sha256.Sum256(existing)
		if !strings.EqualFold(hex.EncodeToString(existingDigest[:]), digest) {
			return signingRunProfileInstall{}, fmt.Errorf("profile destination already contains different content")
		}
		return signingRunProfileInstall{Path: path, Digest: digest}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return signingRunProfileInstall{}, err
	}
	rooted, err := installRoot.OpenRoot()
	if err != nil {
		return signingRunProfileInstall{}, err
	}
	defer rooted.Close()
	if err := ctx.Err(); err != nil {
		return signingRunProfileInstall{}, err
	}
	file, stagedName, err := secureopen.CreateTempNoFollowInRoot(rooted, ".", ".asc-signing-run-profile-*", 0o600)
	if err != nil {
		return signingRunProfileInstall{}, err
	}
	stagedPath := filepath.Join(installDir, stagedName)
	planned := signingRunProfileInstall{
		Path: path, StagedPath: stagedPath, Created: true, Digest: digest,
	}
	emptyDigest := sha256.Sum256(nil)
	planned.StagedDigest = hex.EncodeToString(emptyDigest[:])
	fileOpen := true
	closeFile := func() error {
		if !fileOpen {
			return nil
		}
		fileOpen = false
		return file.Close()
	}
	stagedExists := true
	defer func() {
		if !stagedExists {
			resultErr = errors.Join(resultErr, closeFile())
			return
		}
		if planned.Device == 0 && planned.Inode == 0 {
			if retryInfo, retryErr := file.Stat(); retryErr == nil {
				if retryStat, ok := retryInfo.Sys().(*syscall.Stat_t); ok {
					planned.Device = uint64(retryStat.Dev)
					planned.Inode = retryStat.Ino
				}
			}
		}
		resultErr = errors.Join(resultErr, closeFile())
		if err := removeSigningRunStagedProfile(planned.StagedPath, planned.Device, planned.Inode, planned.StagedDigest); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove staged provisioning profile: %w", err))
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return signingRunProfileInstall{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return signingRunProfileInstall{}, fmt.Errorf("inspect installed provisioning profile identity")
	}
	planned.Device = uint64(stat.Dev)
	planned.Inode = stat.Ino
	if err := recordCreate(planned); err != nil {
		return planned, errors.Join(fmt.Errorf("journal profile installation: %w", err), closeFile())
	}
	if err := ctx.Err(); err != nil {
		return planned, errors.Join(err, closeFile())
	}
	planned.StagedDigest, err = writeSigningRunStagedProfile(file, data)
	if err != nil {
		return planned, errors.Join(err, closeFile())
	}
	if err := file.Sync(); err != nil {
		return planned, errors.Join(err, closeFile())
	}
	if err := recordCreate(planned); err != nil {
		return planned, errors.Join(fmt.Errorf("journal written profile staging: %w", err), closeFile())
	}
	if err := ctx.Err(); err != nil {
		return planned, errors.Join(err, closeFile())
	}
	if err := secureopen.RenameNoReplaceInRoot(rooted, stagedName, name); err != nil {
		return planned, err
	}
	stagedExists = false
	if err := verifySigningRunProfileEntry(rooted, name, planned); err != nil {
		verifyErr := fmt.Errorf("verify published provisioning profile: %w", err)
		if rollbackErr := secureopen.RenameNoReplaceInRoot(rooted, name, stagedName); rollbackErr != nil {
			return planned, errors.Join(verifyErr, fmt.Errorf("restore rejected staged provisioning profile: %w", rollbackErr))
		}
		stagedExists = true
		return planned, verifyErr
	}
	if err := closeFile(); err != nil {
		return planned, err
	}
	planned.StagedPath = ""
	return planned, nil
}

func removeSigningRunProfile(install signingRunProfileInstall) error {
	var cleanupErr error
	if install.StagedPath != "" {
		stagedDigest := install.StagedDigest
		if stagedDigest == "" {
			stagedDigest = install.Digest
		}
		if err := removeSigningRunStagedProfile(install.StagedPath, install.Device, install.Inode, stagedDigest); err != nil {
			cleanupErr = fmt.Errorf("remove staged provisioning profile: %w", err)
		}
	}
	if !install.Created {
		return cleanupErr
	}
	return errors.Join(cleanupErr, removeSigningRunProfileWithHook(install, nil))
}

func removeSigningRunProfileWithHook(install signingRunProfileInstall, afterVerify func() error) error {
	if !install.Created {
		return nil
	}
	parentRoot, err := rootfs.New(filepath.Dir(install.Path))
	if err != nil {
		return err
	}
	defer parentRoot.Close()
	rooted, err := parentRoot.OpenRoot()
	if err != nil {
		return err
	}
	defer rooted.Close()
	name := filepath.Base(install.Path)
	quarantineName := ".asc-signing-run-profile-remove-" + name

	err = verifySigningRunProfileEntry(rooted, name, install)
	if errors.Is(err, os.ErrNotExist) {
		err = verifySigningRunProfileEntry(rooted, quarantineName, install)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("refusing to remove quarantined profile: %w", err)
		}
		return removeSigningRunProfileEntry(parentRoot, quarantineName, install)
	}
	if err != nil {
		return err
	}
	if afterVerify != nil {
		if err := afterVerify(); err != nil {
			return err
		}
		if err := removeSigningRunProfileEntry(parentRoot, name, install); err != nil {
			return fmt.Errorf("refusing to remove profile because it changed during cleanup: %w", err)
		}
		return nil
	}
	return removeSigningRunProfileEntry(parentRoot, name, install)
}

// removeSigningRunProfileEntry removes a profile only through the descriptor-
// backed rootfs identity transaction. The earlier verification is useful for
// diagnostics, but it is not sufficient for the final pathname mutation: a
// same-user process can replace the quarantine entry after that check.
func removeSigningRunProfileEntry(parentRoot rootfs.Root, name string, install signingRunProfileInstall) error {
	identity, err := parentRoot.CaptureFileLimited(name, signingRunInputLimit)
	if err != nil {
		return err
	}
	if err := signingRunProfileIdentityMatches(identity, install); err != nil {
		return err
	}
	return parentRoot.RemoveFileIfSameIdentity(name, identity)
}

func signingRunProfileIdentityMatches(identity *rootfs.FileIdentity, install signingRunProfileInstall) error {
	if identity == nil || identity.Info() == nil {
		return fmt.Errorf("refusing to remove profile because its file identity is unavailable")
	}
	stat, ok := identity.Info().Sys().(*syscall.Stat_t)
	if !ok || uint64(stat.Dev) != install.Device || stat.Ino != install.Inode {
		return fmt.Errorf("refusing to remove profile because its file identity changed")
	}
	digest := sha256.Sum256(identity.Data())
	if !strings.EqualFold(hex.EncodeToString(digest[:]), install.Digest) {
		return fmt.Errorf("refusing to remove profile because its content changed")
	}
	return nil
}

func verifySigningRunProfileEntry(rooted *os.Root, name string, install signingRunProfileInstall) error {
	file, err := secureopen.OpenExistingNoFollowInRoot(rooted, name)
	if err != nil {
		return err
	}
	info, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, signingRunInputLimit+1))
	closeErr := file.Close()
	if statErr != nil || readErr != nil || closeErr != nil {
		return errors.Join(statErr, readErr, closeErr)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to remove profile because it is not a regular file")
	}
	if len(data) > signingRunInputLimit {
		return fmt.Errorf("refusing to remove profile because it exceeds the size limit")
	}
	digest := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), install.Digest) {
		return fmt.Errorf("refusing to remove profile because its content changed")
	}
	if install.Device != 0 || install.Inode != 0 {
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || uint64(stat.Dev) != install.Device || stat.Ino != install.Inode {
			return fmt.Errorf("refusing to remove profile because its file identity changed")
		}
	}
	return nil
}

func removeSigningRunStagedProfile(path string, device, inode uint64, digest string) error {
	if path == "" {
		return nil
	}
	if device == 0 && inode == 0 {
		return fmt.Errorf("refusing to remove staged profile because its file identity is unavailable")
	}
	digest = strings.TrimSpace(digest)
	if len(digest) != sha256.Size*2 {
		return fmt.Errorf("refusing to remove staged profile because its content identity is unavailable")
	}
	installRoot, err := rootfs.New(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer installRoot.Close()
	rooted, err := installRoot.OpenRoot()
	if err != nil {
		return err
	}
	defer rooted.Close()
	name := filepath.Base(path)
	if err := verifySigningRunStagedProfileEntry(rooted, name, device, inode, digest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return removeSigningRunStagedProfileEntry(installRoot, name, device, inode, digest)
}

func removeSigningRunStagedProfileEntry(installRoot rootfs.Root, name string, device, inode uint64, digest string) error {
	return removeSigningRunStagedProfileEntryWithHook(installRoot, name, device, inode, digest, nil)
}

func removeSigningRunStagedProfileEntryWithHook(installRoot rootfs.Root, name string, device, inode uint64, digest string, afterCapture func() error) error {
	return removeSigningRunStagedProfileEntryWithCaptureHook(
		installRoot,
		name,
		device,
		inode,
		digest,
		func() (*rootfs.FileIdentity, error) {
			return installRoot.CaptureFileLimited(name, signingRunInputLimit)
		},
		afterCapture,
	)
}

func removeSigningRunStagedProfileEntryWithCaptureHook(
	installRoot rootfs.Root,
	name string,
	device, inode uint64,
	digest string,
	capture func() (*rootfs.FileIdentity, error),
	afterCapture func() error,
) error {
	identity, err := capture()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if (errors.Is(err, rootfs.ErrFileIdentityChanged) && signingRunStagedProfileIdentityConflictOnly(err)) ||
			errors.Is(err, rootfs.ErrFileIdentityDataTooLarge) {
			return fmt.Errorf("%w: refusing to remove changed staged profile: %w", errSigningRunStagedProfileChanged, err)
		}
		matches, inspectErr := signingRunStagedProfileEntryMatches(installRoot, name, device, inode)
		if inspectErr != nil {
			if errors.Is(inspectErr, os.ErrNotExist) {
				return nil
			}
			return errors.Join(err, fmt.Errorf("inspect staged profile after capture failure: %w", inspectErr))
		}
		if !matches {
			return fmt.Errorf("%w: refusing to remove changed staged profile: %w", errSigningRunStagedProfileChanged, err)
		}
		return err
	}
	info := identity.Info()
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || uint64(stat.Dev) != device || stat.Ino != inode {
		return fmt.Errorf("%w: refusing to remove staged profile because its file identity changed", errSigningRunStagedProfileChanged)
	}
	if err := signingRunStagedProfileContentMatches(identity.Data(), digest); err != nil {
		return err
	}
	if afterCapture != nil {
		if err := afterCapture(); err != nil {
			return err
		}
	}
	if err := installRoot.RemoveFileIfSameIdentity(name, identity); err != nil {
		return classifySigningRunStagedProfileRemovalError(err)
	}
	return nil
}

func classifySigningRunStagedProfileRemovalError(err error) error {
	if errors.Is(err, rootfs.ErrFileIdentityChanged) && signingRunStagedProfileIdentityConflictOnly(err) {
		return fmt.Errorf("%w: refusing to remove replaced staged profile: %w", errSigningRunStagedProfileChanged, err)
	}
	return err
}

func signingRunStagedProfileIdentityConflictOnly(err error) bool {
	if err == nil {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if !signingRunStagedProfileIdentityConflictOnly(child) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return signingRunStagedProfileIdentityConflictOnly(wrapped.Unwrap())
	}
	return errors.Is(err, rootfs.ErrFileIdentityChanged) ||
		errors.Is(err, rootfs.ErrFileIdentityRemoved) || errors.Is(err, os.ErrNotExist)
}

func signingRunStagedProfileEntryMatches(installRoot rootfs.Root, name string, device, inode uint64) (bool, error) {
	rooted, err := installRoot.OpenRoot()
	if err != nil {
		return false, err
	}
	defer rooted.Close()
	info, err := rooted.Lstat(name)
	if err != nil {
		return false, err
	}
	return signingRunStagedProfileInfoMatches(info, device, inode), nil
}

func signingRunStagedProfileInfoMatches(info os.FileInfo, device, inode uint64) bool {
	if info == nil || !info.Mode().IsRegular() {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && uint64(stat.Dev) == device && uint64(stat.Ino) == inode
}

func verifySigningRunStagedProfileEntry(rooted *os.Root, name string, device, inode uint64, digest string) error {
	rootedInfo, err := rooted.Lstat(name)
	if err != nil {
		return err
	}
	if !signingRunStagedProfileInfoMatches(rootedInfo, device, inode) {
		return fmt.Errorf("%w: refusing to remove staged profile because its file identity changed", errSigningRunStagedProfileChanged)
	}
	file, err := secureopen.OpenExistingNoFollowInRoot(rooted, name)
	if err != nil {
		latestInfo, inspectErr := rooted.Lstat(name)
		if inspectErr == nil && !signingRunStagedProfileInfoMatches(latestInfo, device, inode) {
			return fmt.Errorf("%w: refusing to remove changed staged profile: %w", errSigningRunStagedProfileChanged, err)
		}
		if inspectErr != nil {
			if errors.Is(inspectErr, os.ErrNotExist) {
				return inspectErr
			}
			return errors.Join(err, fmt.Errorf("inspect staged profile after open failure: %w", inspectErr))
		}
		return err
	}
	info, statErr := file.Stat()
	if statErr != nil {
		return errors.Join(statErr, file.Close())
	}
	if !signingRunStagedProfileInfoMatches(info, device, inode) {
		return errors.Join(
			fmt.Errorf("%w: refusing to remove staged profile because its file identity changed", errSigningRunStagedProfileChanged),
			file.Close(),
		)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, signingRunInputLimit+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return errors.Join(readErr, closeErr)
	}
	if len(data) > signingRunInputLimit {
		return fmt.Errorf("%w: refusing to remove staged profile because it exceeds the size limit", errSigningRunStagedProfileChanged)
	}
	return signingRunStagedProfileContentMatches(data, digest)
}

func signingRunStagedProfileContentMatches(data []byte, digest string) error {
	actual := sha256.Sum256(data)
	if strings.EqualFold(hex.EncodeToString(actual[:]), digest) {
		return nil
	}
	return fmt.Errorf("%w: refusing to remove staged profile because its content changed", errSigningRunStagedProfileChanged)
}

func writeSigningRunStagedProfile(file io.Writer, data []byte) (string, error) {
	written, err := file.Write(data)
	if written < 0 || written > len(data) {
		return "", fmt.Errorf("write staged provisioning profile returned invalid byte count %d", written)
	}
	digest := sha256.Sum256(data[:written])
	if err != nil {
		return hex.EncodeToString(digest[:]), err
	}
	if written != len(data) {
		return hex.EncodeToString(digest[:]), io.ErrShortWrite
	}
	return hex.EncodeToString(digest[:]), nil
}

var signingRunXcodeVersionPattern = regexp.MustCompile(`(?m)^Xcode[\t ]+([0-9]+)(?:[.][0-9]+)*(?:[\t ].*)?$`)

func activeSigningRunXcodeMajorVersion(ctx context.Context) (int, error) {
	if ctx == nil {
		return 0, fmt.Errorf("inspect active Xcode version: context is required")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	cmd := signingRunCommandContext(ctx, signingRunXcodebuildPath, "-version")
	cmd.Env = SanitizedChildEnvironment(os.Environ())
	stdout := &limitedSigningBuffer{limit: signingRunDiagnosticLimit}
	stderr := &limitedSigningBuffer{limit: signingRunDiagnosticLimit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, ctxErr
		}
		return 0, utilityFailure("inspect active Xcode version", stderr.Bytes(), err)
	}
	match := signingRunXcodeVersionPattern.FindSubmatch(stdout.Bytes())
	if len(match) != 2 {
		return 0, fmt.Errorf("inspect active Xcode version: unexpected xcodebuild output")
	}
	major, err := strconv.Atoi(string(match[1]))
	if err != nil || major < 1 {
		return 0, fmt.Errorf("inspect active Xcode version: invalid major version")
	}
	return major, nil
}

func signingRunProfileInstallDir(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("discover provisioning profile directory: context is required")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	major, err := signingRunActiveXcodeMajorVersion(ctx)
	if err == nil {
		if major >= 16 {
			return filepath.Join(homeDir, "Library", "Developer", "Xcode", "UserData", "Provisioning Profiles"), nil
		}
		return filepath.Join(homeDir, "Library", "MobileDevice", "Provisioning Profiles"), nil
	}
	if !errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("inspect active Xcode version: %w", err)
	}
	// xcodebuild is absent on machines without the Xcode command-line tools;
	// the legacy profile directory remains the only useful destination.
	return filepath.Join(homeDir, "Library", "MobileDevice", "Provisioning Profiles"), nil
}

func runSigningRunChild(ctx context.Context, argv []string) error {
	if ctx == nil {
		return fmt.Errorf("run signing child: context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return fmt.Errorf("run signing child: command is required")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = SanitizedChildEnvironment(os.Environ())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	pid := cmd.Process.Pid
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		_ = signingRunKillProcessGroupFn(pid, signingRunCancellationSignal(ctx))
		timer := time.NewTimer(signingRunChildWaitDelay)
		select {
		case err = <-done:
			timer.Stop()
		case <-timer.C:
			_ = signingRunKillProcessGroupFn(pid, syscall.SIGKILL)
			err = <-done
		}
	}
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code := exitErr.ExitCode(); code >= 0 {
			return shared.NewProcessExitError(code)
		}
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return shared.NewProcessExitError(128 + int(status.Signal()))
		}
	}
	return err
}

func runSigningUtility(ctx context.Context, stdin []byte, args ...string) ([]byte, []byte, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("run signing utility: context is required")
	}
	cmd := exec.CommandContext(ctx, "/usr/bin/security", args...)
	cmd.Env = SanitizedChildEnvironment(os.Environ())
	cmd.Stdin = bytes.NewReader(stdin)
	stdout := &limitedSigningBuffer{limit: signingRunDiagnosticLimit}
	stderr := &limitedSigningBuffer{limit: signingRunDiagnosticLimit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

type limitedSigningBuffer struct {
	data  []byte
	limit int
}

func (b *limitedSigningBuffer) Write(data []byte) (int, error) {
	if len(b.data) < b.limit {
		remaining := b.limit - len(b.data)
		b.data = append(b.data, data[:min(len(data), remaining)]...)
	}
	return len(data), nil
}

func (b *limitedSigningBuffer) Bytes() []byte { return append([]byte(nil), b.data...) }

func utilityFailure(operation string, stderr []byte, err error, sensitive ...[]byte) error {
	diagnostic := signingUtilityDiagnostic(stderr, sensitive...)
	if diagnostic == "" {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return fmt.Errorf("%s: %s: %w", operation, diagnostic, err)
}

func signingUtilityDiagnostic(stderr []byte, sensitive ...[]byte) string {
	diagnostic := string(stderr)
	for _, secret := range sensitive {
		if len(secret) == 0 {
			continue
		}
		for _, representation := range []string{
			string(secret),
			hex.EncodeToString(secret),
			strings.ToUpper(hex.EncodeToString(secret)),
		} {
			if representation == "" {
				continue
			}
			diagnostic = strings.ReplaceAll(diagnostic, representation, "[REDACTED]")
			diagnostic = strings.ReplaceAll(diagnostic, shared.SanitizeTerminal(representation), "[REDACTED]")
		}
	}
	diagnostic = strings.TrimSpace(shared.SanitizeTerminal(diagnostic))
	if len(diagnostic) <= signingUtilityDiagnosticLimit {
		return diagnostic
	}
	diagnostic = diagnostic[:signingUtilityDiagnosticLimit]
	for !utf8.ValidString(diagnostic) {
		diagnostic = diagnostic[:len(diagnostic)-1]
	}
	return diagnostic + " [truncated]"
}

func systemSigningRunRoots() (*x509.CertPool, error) { return x509.SystemCertPool() }

func validateSigningRunInputPermissions(path string, info os.FileInfo, private bool) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("%q must be owned by the current user", path)
	}
	if stat.Nlink != 1 {
		return fmt.Errorf("%q must not have multiple hard links", path)
	}
	if private && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%q must not be accessible by group or other users", path)
	}
	if !private && info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%q must not be writable by group or other users", path)
	}
	return nil
}

func platformSigningRunContext(ctx context.Context) (context.Context, func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	return contextWithSigningRunSignals(ctx, signals, func() {
		signal.Stop(signals)
	})
}

type signingRunSignalCause struct {
	signal syscall.Signal
}

func (cause *signingRunSignalCause) Error() string {
	return fmt.Sprintf("received signal %s", cause.signal)
}

func contextWithSigningRunSignals(
	parent context.Context,
	signals <-chan os.Signal,
	stopSignals func(),
) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			stopSignals()
			cancel(context.Canceled)
		})
	}
	go func() {
		select {
		case received := <-signals:
			sig, ok := received.(syscall.Signal)
			if !ok {
				cancel(fmt.Errorf("received unsupported signal %v", received))
				return
			}
			cancel(&signingRunSignalCause{signal: sig})
		case <-ctx.Done():
		}
	}()
	return ctx, stop
}

func signingRunCancellationSignal(ctx context.Context) syscall.Signal {
	var cause *signingRunSignalCause
	if errors.As(context.Cause(ctx), &cause) {
		return cause.signal
	}
	return syscall.SIGINT
}
