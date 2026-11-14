package signing

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"debug/macho"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

const (
	signingResignMaxArchiveEntries                       = 20_000
	signingResignMaxArchiveMemberNameLen                 = 4096
	signingResignMaxExpandedBytes                 uint64 = 16 << 30
	signingResignMaxIPABytes                      int64  = 8 << 30
	signingResignSwiftSupportMaxBytes             int64  = 1 << 30
	signingResignMaxTargetCount                          = 256
	signingResignMaxCentralDirectoryBytes         int64  = 1 << 30
	signingResignMaxCentralDirectoryMetadataBytes uint64 = 128 << 20
)

type signingResignTarget struct {
	Kind                 string
	RelativePath         string
	BundleID             string
	Executable           string
	ProfileMode          os.FileMode
	ExistingEntitlements map[string]any
	Profile              signingResignProfile
	EntitlementsPath     string
	EntitlementRewrites  []signingResignEntitlementRewrite
}

type signingResignArchive struct {
	MainPath string
	Targets  []signingResignTarget
}

// preflightSigningResignArchive bounds the central-directory inventory before
// archive/zip allocates one *zip.File per declared entry.
func preflightSigningResignArchive(ctx context.Context, file *os.File, size int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if file == nil || size < 22 {
		return fmt.Errorf("IPA archive is missing or truncated")
	}
	const tailSize int64 = 22 + 65535
	readSize := size
	if readSize > tailSize {
		readSize = tailSize
	}
	buf := make([]byte, readSize)
	if _, err := file.ReadAt(buf, size-readSize); err != nil && err != io.EOF {
		return fmt.Errorf("read IPA directory trailer: %w", err)
	}
	for i := len(buf) - 22; i >= 0; i-- {
		if binary.LittleEndian.Uint32(buf[i:i+4]) != 0x06054b50 {
			continue
		}
		commentLength := int(binary.LittleEndian.Uint16(buf[i+20 : i+22]))
		if i+22+commentLength != len(buf) {
			continue
		}
		eocdOffset := size - readSize + int64(i)
		directoryEndOffset := eocdOffset
		entries := uint64(binary.LittleEndian.Uint16(buf[i+10 : i+12]))
		directoryBytes := uint64(binary.LittleEndian.Uint32(buf[i+12 : i+16]))
		directoryOffset := uint64(binary.LittleEndian.Uint32(buf[i+16 : i+20]))
		if entries == 0xffff || directoryBytes == 0xffffffff || directoryOffset == 0xffffffff {
			if eocdOffset < 20 {
				return fmt.Errorf("IPA ZIP64 locator is missing")
			}
			locator := make([]byte, 20)
			if _, err := file.ReadAt(locator, eocdOffset-20); err != nil {
				return fmt.Errorf("read IPA ZIP64 locator: %w", err)
			}
			if binary.LittleEndian.Uint32(locator[0:4]) != 0x07064b50 {
				return fmt.Errorf("IPA ZIP64 locator is malformed")
			}
			recordOffsetValue := binary.LittleEndian.Uint64(locator[8:16])
			if recordOffsetValue > uint64(^uint64(0)>>1) || size < 56 || recordOffsetValue > uint64(size-56) {
				return fmt.Errorf("IPA ZIP64 EOCD offset is out of bounds")
			}
			recordOffset := int64(recordOffsetValue)
			record := make([]byte, 56)
			if _, err := file.ReadAt(record, recordOffset); err != nil {
				return fmt.Errorf("read IPA ZIP64 EOCD: %w", err)
			}
			if binary.LittleEndian.Uint32(record[0:4]) != 0x06064b50 {
				return fmt.Errorf("IPA ZIP64 EOCD is malformed")
			}
			recordSize := binary.LittleEndian.Uint64(record[4:12])
			if recordSize < 44 || recordSize > uint64(size-recordOffset-12) {
				return fmt.Errorf("IPA ZIP64 EOCD size is invalid")
			}
			directoryEndOffset = recordOffset
			entries = binary.LittleEndian.Uint64(record[32:40])
			directoryBytes = binary.LittleEndian.Uint64(record[40:48])
			directoryOffset = binary.LittleEndian.Uint64(record[48:56])
		}
		if entries > signingResignMaxArchiveEntries {
			return fmt.Errorf("IPA contains too many archive entries")
		}
		if directoryBytes > uint64(signingResignMaxCentralDirectoryBytes) {
			return fmt.Errorf("IPA central directory exceeds %d bytes", signingResignMaxCentralDirectoryBytes)
		}
		const maxInt64 = uint64(^uint64(0) >> 1)
		if directoryOffset > maxInt64 || directoryBytes > maxInt64 || directoryBytes > maxInt64-directoryOffset {
			return fmt.Errorf("IPA central directory is out of bounds")
		}
		baseOffset := directoryEndOffset - int64(directoryBytes) - int64(directoryOffset)
		physicalDirectoryOffset := directoryEndOffset - int64(directoryBytes)
		// Keep the same zero-base fallback as archive/zip.Reader: some ZIP
		// writers emit a non-zero base estimate even though directoryOffset is
		// already an absolute file offset.
		if baseOffset > 0 && size >= 46 && directoryOffset <= uint64(size-46) {
			var header [46]byte
			if _, err := file.ReadAt(header[:], int64(directoryOffset)); err == nil && binary.LittleEndian.Uint32(header[0:4]) == 0x02014b50 {
				baseOffset = 0
				physicalDirectoryOffset = int64(directoryOffset)
			}
		}
		if physicalDirectoryOffset < 0 || physicalDirectoryOffset > directoryEndOffset || physicalDirectoryOffset >= size || directoryEndOffset > size {
			return fmt.Errorf("IPA central directory is out of bounds")
		}
		// Keep baseOffset live in this calculation so the relationship stays
		// explicit and cannot drift from archive/zip's seek position.
		if baseOffset+int64(directoryOffset) != physicalDirectoryOffset {
			return fmt.Errorf("IPA central directory is out of bounds")
		}
		physicalDirectoryBytes := uint64(directoryEndOffset - physicalDirectoryOffset)
		if physicalDirectoryBytes > uint64(signingResignMaxCentralDirectoryBytes) {
			return fmt.Errorf("IPA central directory exceeds %d bytes", signingResignMaxCentralDirectoryBytes)
		}
		position := physicalDirectoryOffset
		actualEntries := uint64(0)
		metadataBytes := uint64(0)
		for position < directoryEndOffset {
			if err := ctx.Err(); err != nil {
				return err
			}
			if actualEntries >= signingResignMaxArchiveEntries {
				return fmt.Errorf("IPA contains too many archive entries")
			}
			header := make([]byte, 46)
			if _, err := file.ReadAt(header, int64(position)); err != nil {
				return fmt.Errorf("read IPA central directory: %w", err)
			}
			if binary.LittleEndian.Uint32(header[0:4]) != 0x02014b50 {
				return fmt.Errorf("IPA central directory record is malformed")
			}
			nameBytes := uint64(binary.LittleEndian.Uint16(header[28:30]))
			extraBytes := uint64(binary.LittleEndian.Uint16(header[30:32]))
			commentBytes := uint64(binary.LittleEndian.Uint16(header[32:34]))
			recordSize := uint64(46) + uint64(binary.LittleEndian.Uint16(header[28:30])) + uint64(binary.LittleEndian.Uint16(header[30:32])) + uint64(binary.LittleEndian.Uint16(header[32:34]))
			if recordSize > uint64(directoryEndOffset)-uint64(position) {
				return fmt.Errorf("IPA central directory record is truncated")
			}
			recordMetadata := nameBytes + extraBytes + commentBytes
			if recordMetadata > signingResignMaxCentralDirectoryMetadataBytes-metadataBytes {
				return fmt.Errorf("IPA central directory metadata exceeds %d bytes", signingResignMaxCentralDirectoryMetadataBytes)
			}
			metadataBytes += recordMetadata
			position += int64(recordSize)
			actualEntries++
		}
		if position != directoryEndOffset {
			return fmt.Errorf("IPA central directory is malformed")
		}
		return nil
	}
	return fmt.Errorf("IPA archive is missing the end-of-central-directory record")
}

func snapshotSigningResignIPA(ctx context.Context, source *os.File, size int64, destination *os.Root) (*os.File, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if source == nil {
		return nil, "", fmt.Errorf("IPA input is missing")
	}
	if size <= 0 || size > signingResignMaxIPABytes {
		return nil, "", fmt.Errorf("IPA size must be between 1 and %d bytes", signingResignMaxIPABytes)
	}
	before, err := source.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("inspect IPA input: %w", err)
	}
	if err := validateSigningResignIPAFileInfo(before, size); err != nil {
		return nil, "", err
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return nil, "", fmt.Errorf("seek IPA input: %w", err)
	}
	snapshot, err := secureopen.OpenNewFileNoFollowInRoot(destination, "input.ipa", 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("create private IPA snapshot: %w", err)
	}
	hash := newSHA256Writer()
	written, copyErr := copySigningResignWithContext(ctx, io.MultiWriter(snapshot, hash), io.LimitReader(source, size+1), size)
	if copyErr == nil && written != size {
		copyErr = fmt.Errorf("IPA changed while it was being snapshotted")
	}
	if copyErr == nil {
		copyErr = snapshot.Sync()
	}
	closeErr := snapshot.Close()
	if copyErr != nil || closeErr != nil {
		_ = destination.Remove("input.ipa")
		return nil, "", errors.Join(copyErr, closeErr)
	}
	after, err := source.Stat()
	if err != nil {
		_ = destination.Remove("input.ipa")
		return nil, "", fmt.Errorf("reinspect IPA input after snapshot: %w", err)
	}
	if err := validateStableSigningResignIPA(before, after, size); err != nil {
		_ = destination.Remove("input.ipa")
		return nil, "", err
	}
	pinned, err := secureopen.OpenExistingNoFollowInRoot(destination, "input.ipa")
	if err != nil {
		_ = destination.Remove("input.ipa")
		return nil, "", fmt.Errorf("reopen private IPA snapshot: %w", err)
	}
	return pinned, hash.String(), nil
}

type signingResignSHA256Writer struct{ hash hash.Hash }

func newSHA256Writer() *signingResignSHA256Writer {
	return &signingResignSHA256Writer{hash: sha256.New()}
}

func (writer *signingResignSHA256Writer) Write(data []byte) (int, error) {
	return writer.hash.Write(data)
}

func (writer *signingResignSHA256Writer) String() string {
	return strings.ToUpper(fmt.Sprintf("%x", writer.hash.Sum(nil)))
}

func validateSigningResignIPAFileInfo(info os.FileInfo, size int64) error {
	if info == nil || !info.Mode().IsRegular() {
		return fmt.Errorf("IPA input is not a regular file")
	}
	if info.Size() != size {
		return fmt.Errorf("IPA size changed before snapshot")
	}
	if err := validateSigningRunInputPermissions("IPA input", info, false); err != nil {
		return err
	}
	return nil
}

func validateStableSigningResignIPA(before, after os.FileInfo, size int64) error {
	if !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return fmt.Errorf("IPA input changed while it was being snapshotted")
	}
	return validateSigningResignIPAFileInfo(after, size)
}

func copySigningResignWithContext(ctx context.Context, destination io.Writer, source io.Reader, expected int64) (int64, error) {
	buffer := make([]byte, 64<<10)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			if written > expected-int64(read) {
				return written, fmt.Errorf("IPA exceeds its declared size")
			}
			count, writeErr := destination.Write(buffer[:read])
			written += int64(count)
			if writeErr != nil {
				return written, writeErr
			}
			if count != read {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return written, nil
			}
			return written, readErr
		}
	}
}

func validateSigningResignArchive(ctx context.Context, reader *zip.Reader) error {
	if reader == nil {
		return fmt.Errorf("IPA archive is missing")
	}
	if len(reader.File) > signingResignMaxArchiveEntries {
		return fmt.Errorf("IPA contains too many archive entries")
	}
	seen := make(map[string]bool, len(reader.File))
	descendants := make(map[string]struct{}, len(reader.File))
	var declared uint64
	for _, member := range reader.File {
		if err := validateSigningResignArchiveMember(member); err != nil {
			return err
		}
		if member.UncompressedSize64 > signingResignMaxExpandedBytes-declared {
			return fmt.Errorf("IPA declared expansion exceeds %d bytes", signingResignMaxExpandedBytes)
		}
		declared += member.UncompressedSize64
		key := strings.ToLower(strings.TrimSuffix(member.Name, "/"))
		if _, exists := seen[key]; exists {
			return fmt.Errorf("IPA contains duplicate path")
		}
		isDirectory := member.FileInfo().IsDir()
		for ancestor := path.Dir(key); ancestor != "."; ancestor = path.Dir(ancestor) {
			if ancestorIsDirectory, exists := seen[ancestor]; exists && !ancestorIsDirectory {
				return fmt.Errorf("IPA contains a file/directory path collision")
			}
		}
		if !isDirectory {
			if _, exists := descendants[key]; exists {
				return fmt.Errorf("IPA contains a file/directory path collision")
			}
		}
		seen[key] = isDirectory
		for ancestor := path.Dir(key); ancestor != "."; ancestor = path.Dir(ancestor) {
			descendants[ancestor] = struct{}{}
		}
	}
	var expanded uint64
	for _, member := range reader.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		opened, err := member.Open()
		if err != nil {
			return fmt.Errorf("read IPA archive member: %w", err)
		}
		remaining := signingResignMaxExpandedBytes - expanded
		written, readErr := copySigningResignWithContext(ctx, io.Discard, io.LimitReader(opened, int64(remaining)+1), int64(remaining))
		closeErr := opened.Close()
		if readErr != nil || closeErr != nil {
			return errors.Join(readErr, closeErr)
		}
		if written < 0 || uint64(written) > remaining || uint64(written) != member.UncompressedSize64 {
			return fmt.Errorf("IPA archive member has invalid expanded contents")
		}
		expanded += uint64(written)
		if member.FileInfo().IsDir() && written != 0 {
			return fmt.Errorf("IPA directory member contains data")
		}
	}
	return nil
}

func validateSigningResignArchiveMember(member *zip.File) error {
	if member == nil {
		return fmt.Errorf("IPA contains a missing archive member")
	}
	name := member.Name
	if name == "" || len(name) > signingResignMaxArchiveMemberNameLen || !utf8.ValidString(name) || strings.ContainsRune(name, '\\') {
		return fmt.Errorf("IPA contains an unsafe archive path")
	}
	for _, character := range name {
		if character == 0 || unicode.IsControl(character) || unicode.Is(unicode.Cf, character) || unicode.In(character, unicode.Bidi_Control) {
			return fmt.Errorf("IPA contains an unsafe archive path")
		}
	}
	trimmed := strings.TrimSuffix(name, "/")
	if trimmed == "" || path.IsAbs(name) || path.Clean(trimmed) != trimmed || trimmed == ".." || strings.HasPrefix(trimmed, "../") {
		return fmt.Errorf("IPA contains a non-canonical archive path")
	}
	if member.Flags&1 != 0 {
		return fmt.Errorf("IPA contains an encrypted archive member")
	}
	if member.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("IPA contains a symbolic link")
	}
	if !member.FileInfo().IsDir() && !member.Mode().IsRegular() {
		return fmt.Errorf("IPA contains a non-regular member")
	}
	if _, err := signingResignArchiveMemberMode(member); err != nil {
		return err
	}
	return nil
}

func signingResignSafeFileMode(mode os.FileMode, isDirectory bool) (os.FileMode, error) {
	permissions := mode.Perm()
	if mode&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return 0, fmt.Errorf("IPA contains an unsafe archive special mode")
	}
	// Group/world write bits are not safe to carry into an installed app.
	// Reject them instead of silently changing a validated archive member's
	// metadata; all accepted modes are then preserved exactly through repack.
	if permissions&0o022 != 0 {
		return 0, fmt.Errorf("IPA contains an unsafe archive permission mode")
	}
	if isDirectory {
		if permissions&0o500 != 0o500 {
			return 0, fmt.Errorf("IPA contains an unreadable or untraversable directory mode")
		}
		// Preparation must replace embedded profiles and codesign must rewrite
		// _CodeSignature inside these directories, and validated modes are
		// preserved exactly through staging and repack. A read-only directory
		// would pass validation and then fail mid-signing, so reject it up
		// front instead of silently widening its mode.
		if permissions&0o200 == 0 {
			return 0, fmt.Errorf("IPA contains a directory mode that is not owner-writable")
		}
	} else if permissions&0o400 == 0 {
		return 0, fmt.Errorf("IPA contains an unreadable archive file mode")
	}
	return permissions, nil
}

func signingResignArchiveMemberMode(member *zip.File) (os.FileMode, error) {
	if member == nil {
		return 0, fmt.Errorf("IPA contains a missing archive member")
	}
	isDirectory := member.FileInfo().IsDir()
	mode := member.Mode()
	if member.CreatorVersion>>8 == 0 {
		// A DOS-created member carries no Unix permission metadata. Use safe
		// defaults only for that explicitly identified case; an explicit Unix
		// mode of 000 remains an error rather than being silently widened.
		if isDirectory {
			return 0o700, nil
		}
		return 0o644, nil
	}
	return signingResignSafeFileMode(mode, isDirectory)
}

func materializeSigningResignArchive(ctx context.Context, reader *zip.Reader, destination *os.Root) error {
	if err := destination.MkdirAll(".", 0o700); err != nil {
		return err
	}
	members := append([]*zip.File(nil), reader.File...)
	sort.Slice(members, func(i, j int) bool { return members[i].Name < members[j].Name })
	type directoryMode struct {
		name string
		mode os.FileMode
	}
	var directories []directoryMode
	for _, member := range members {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := filepath.FromSlash(strings.TrimSuffix(member.Name, "/"))
		if member.FileInfo().IsDir() {
			if err := destination.MkdirAll(name, 0o700); err != nil {
				return err
			}
			mode, err := signingResignArchiveMemberMode(member)
			if err != nil {
				return err
			}
			directories = append(directories, directoryMode{name: name, mode: mode})
			continue
		}
		mode, err := signingResignArchiveMemberMode(member)
		if err != nil {
			return err
		}
		if err := destination.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			return err
		}
		file, err := secureopen.OpenNewFileNoFollowInRoot(destination, name, 0o600)
		if err != nil {
			return fmt.Errorf("materialize IPA member: %w", err)
		}
		opened, openErr := member.Open()
		if openErr == nil {
			_, openErr = copySigningResignWithContext(ctx, file, io.LimitReader(opened, int64(member.UncompressedSize64)+1), int64(member.UncompressedSize64))
			closeMemberErr := opened.Close()
			openErr = errors.Join(openErr, closeMemberErr)
		}
		if openErr == nil {
			if err := file.Sync(); err != nil {
				openErr = err
			}
		}
		closeErr := file.Close()
		if openErr != nil || closeErr != nil {
			return errors.Join(openErr, closeErr)
		}
		if err := destination.Chmod(name, mode); err != nil {
			return err
		}
	}
	for _, directory := range directories {
		if err := destination.Chmod(directory.name, directory.mode); err != nil {
			return err
		}
	}
	return nil
}

func discoverSigningResignArchive(ctx context.Context, reader *zip.Reader, tree rootfs.Root) (signingResignArchive, error) {
	return discoverSigningResignArchiveWithEntitlements(ctx, reader, tree, true)
}

func discoverSigningResignArchiveRooted(ctx context.Context, reader *zip.Reader, tree rootfs.Root) (signingResignArchive, error) {
	return discoverSigningResignArchiveWithEntitlements(ctx, reader, tree, false)
}

func discoverSigningResignArchiveWithEntitlements(ctx context.Context, reader *zip.Reader, tree rootfs.Root, readEntitlements bool) (signingResignArchive, error) {
	if reader == nil || tree.Path() == "" {
		return signingResignArchive{}, fmt.Errorf("IPA archive or staging root is missing")
	}
	directories := make(map[string]struct{})
	for _, member := range reader.File {
		name := strings.TrimSuffix(member.Name, "/")
		// The pipeline validates every archive member before discovery, and
		// all reads below go through the rooted staging tree. Keep discovery
		// independently fail-closed anyway: reject entry names that could
		// resolve outside the tree before any derived path reaches a
		// filesystem operation.
		if name == "" || !filepath.IsLocal(filepath.FromSlash(name)) {
			return signingResignArchive{}, fmt.Errorf("IPA contains a non-local archive path")
		}
		// Only real and implied directories may participate in target
		// discovery. A regular member is never a bundle, so start at its
		// parent: otherwise a resource whose name ends in .app or .appex
		// would be rejected as an unsupported nested target.
		candidate := name
		if !member.FileInfo().IsDir() {
			candidate = path.Dir(name)
		}
		for ; candidate != "." && candidate != ""; candidate = path.Dir(candidate) {
			directories[candidate] = struct{}{}
		}
	}
	var mains []string
	for directory := range directories {
		parts := strings.Split(directory, "/")
		if len(parts) == 2 && parts[0] == "Payload" && strings.HasSuffix(parts[1], ".app") {
			mains = append(mains, directory)
		}
	}
	if len(mains) != 1 {
		return signingResignArchive{}, fmt.Errorf("IPA must contain exactly one Payload/*.app")
	}
	mainPath := mains[0]
	accepted := map[string]string{mainPath: "application"}
	for directory := range directories {
		if directory == mainPath+"/PlugIns" || directory == mainPath+"/Watch" || directory == mainPath+"/AppClips" {
			continue
		}
		if strings.HasPrefix(directory, mainPath+"/PlugIns/") && strings.Count(strings.TrimPrefix(directory, mainPath+"/PlugIns/"), "/") == 0 && strings.HasSuffix(directory, ".appex") {
			accepted[directory] = "app-extension"
		}
		if strings.HasPrefix(directory, mainPath+"/Watch/") && strings.Count(strings.TrimPrefix(directory, mainPath+"/Watch/"), "/") == 0 && strings.HasSuffix(directory, ".app") {
			accepted[directory] = "watch-application"
		}
		if strings.HasPrefix(directory, mainPath+"/Watch/") && strings.Contains(directory, "/PlugIns/") && strings.HasSuffix(directory, ".appex") {
			relative := strings.TrimPrefix(directory, mainPath+"/Watch/")
			parts := strings.Split(relative, "/")
			if len(parts) == 3 && strings.HasSuffix(parts[0], ".app") && parts[1] == "PlugIns" {
				accepted[directory] = "watch-extension"
			}
		}
		if strings.HasPrefix(directory, mainPath+"/AppClips/") && strings.Count(strings.TrimPrefix(directory, mainPath+"/AppClips/"), "/") == 0 && strings.HasSuffix(directory, ".app") {
			accepted[directory] = "app-clip"
		}
	}
	for directory := range directories {
		if (strings.HasSuffix(directory, ".app") || strings.HasSuffix(directory, ".appex")) && accepted[directory] == "" {
			return signingResignArchive{}, fmt.Errorf("IPA contains an unsupported nested app target")
		}
	}
	if len(accepted) > signingResignMaxTargetCount {
		return signingResignArchive{}, fmt.Errorf("IPA contains too many app-like targets")
	}
	targetPaths := make([]string, 0, len(accepted))
	for targetPath := range accepted {
		targetPaths = append(targetPaths, targetPath)
	}
	sort.Slice(targetPaths, func(i, j int) bool {
		if targetPaths[i] == mainPath {
			return false
		}
		if targetPaths[j] == mainPath {
			return true
		}
		depthI, depthJ := strings.Count(targetPaths[i], "/"), strings.Count(targetPaths[j], "/")
		if depthI != depthJ {
			return depthI > depthJ
		}
		return targetPaths[i] < targetPaths[j]
	})
	archive := signingResignArchive{MainPath: mainPath}
	for _, targetPath := range targetPaths {
		target, err := inspectSigningResignTargetWithEntitlements(ctx, tree, targetPath, accepted[targetPath], readEntitlements)
		if err != nil {
			return signingResignArchive{}, fmt.Errorf("inspect target %s: %w", targetPath, err)
		}
		if err := checkSigningResignRootIdentity(tree); err != nil {
			return signingResignArchive{}, fmt.Errorf("staging tree identity changed during target discovery: %w", err)
		}
		archive.Targets = append(archive.Targets, target)
	}
	return archive, nil
}

func inspectSigningResignTargetWithEntitlements(ctx context.Context, tree rootfs.Root, relativePath, kind string, readEntitlements bool) (signingResignTarget, error) {
	if err := ctx.Err(); err != nil {
		return signingResignTarget{}, err
	}
	infoPath := filepath.FromSlash(path.Join(relativePath, "Info.plist"))
	data, err := tree.ReadFileLimited(infoPath, infoplist.MaxBytes)
	if err != nil {
		return signingResignTarget{}, fmt.Errorf("read Info.plist: %w", err)
	}
	if err := infoplist.ValidateStructure(data); err != nil {
		return signingResignTarget{}, fmt.Errorf("invalid Info.plist: %w", err)
	}
	var info map[string]any
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return signingResignTarget{}, fmt.Errorf("decode Info.plist")
	}
	bundleID := plistString(info["CFBundleIdentifier"])
	if err := validateSigningResignBundleID(bundleID); err != nil {
		return signingResignTarget{}, err
	}
	executable := plistString(info["CFBundleExecutable"])
	if err := validateSigningResignExecutable(executable); err != nil {
		return signingResignTarget{}, err
	}
	if err := validateSigningResignPlatform(info, kind); err != nil {
		return signingResignTarget{}, err
	}
	executablePath := filepath.FromSlash(path.Join(relativePath, executable))
	file, err := tree.OpenFile(executablePath)
	if err != nil {
		return signingResignTarget{}, fmt.Errorf("open executable: %w", err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() {
		return signingResignTarget{}, fmt.Errorf("executable is not a regular file")
	}
	if !isSigningResignAppExecutableFile(file, stat.Size()) {
		return signingResignTarget{}, fmt.Errorf("executable is not an executable Mach-O")
	}
	// A DOS-created archive member carries no Unix metadata and defaults to a
	// non-executable file mode, and validated modes are preserved exactly
	// through repack. Signing would then succeed while producing a bundle
	// whose executable cannot launch, so require the owner-execute bit before
	// any mutation instead of silently restoring it.
	if stat.Mode().Perm()&0o100 == 0 {
		return signingResignTarget{}, fmt.Errorf("executable file mode is missing the owner-execute permission")
	}
	profileMode := os.FileMode(0o644)
	profilePath := filepath.FromSlash(path.Join(relativePath, "embedded.mobileprovision"))
	profileFile, profileErr := tree.OpenFile(profilePath)
	switch {
	case profileErr == nil:
		profileInfo, statErr := profileFile.Stat()
		closeErr := profileFile.Close()
		if statErr != nil || closeErr != nil {
			return signingResignTarget{}, fmt.Errorf("inspect embedded profile")
		}
		profileMode, profileErr = signingResignSafeFileMode(profileInfo.Mode(), false)
		if profileErr != nil {
			return signingResignTarget{}, profileErr
		}
	case errors.Is(profileErr, os.ErrNotExist):
		// An input may be unsigned. The replacement profile is created with
		// the ordinary regular-file mode when there is no source mode to keep.
	default:
		return signingResignTarget{}, fmt.Errorf("inspect embedded profile")
	}
	var entitlements map[string]any
	if readEntitlements {
		entitlements, err = readSigningResignEntitlements(ctx, filepath.Join(tree.Path(), executablePath))
		if err != nil {
			return signingResignTarget{}, fmt.Errorf("read signed entitlements: %w", err)
		}
	}
	return signingResignTarget{Kind: kind, RelativePath: relativePath, BundleID: bundleID, Executable: executable, ProfileMode: profileMode, ExistingEntitlements: entitlements}, nil
}

func validateSigningResignExecutable(value string) error {
	if value == "" || len(value) > 255 || filepath.Base(value) != value || value == "." || value == ".." || strings.ContainsAny(value, `/\\`) {
		return fmt.Errorf("CFBundleExecutable is not a safe filename")
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.Is(unicode.Cf, character) || unicode.In(character, unicode.Bidi_Control) {
			return fmt.Errorf("CFBundleExecutable contains unsupported characters")
		}
	}
	return nil
}

func validateSigningResignPlatform(info map[string]any, kind string) error {
	expectedPlatformName := "iphoneos"
	expectedSupportedPlatform := "iPhoneOS"
	if strings.HasPrefix(kind, "watch-") {
		expectedPlatformName = "watchos"
		expectedSupportedPlatform = "WatchOS"
	}
	platformName := ""
	if value, exists := info["DTPlatformName"]; exists {
		var err error
		platformName, err = signingResignPlatformString(value, "DTPlatformName")
		if err != nil {
			return err
		}
	}
	platforms := []string(nil)
	if value, exists := info["CFBundleSupportedPlatforms"]; exists {
		var err error
		platforms, err = signingResignPlatformStrings(value)
		if err != nil {
			return err
		}
	}
	if platformName == "" && len(platforms) == 0 {
		return fmt.Errorf("target platform metadata is missing")
	}
	if platformName != "" && platformName != expectedPlatformName {
		return signingResignUsage(fmt.Errorf("target platform is not %s", expectedSupportedPlatform))
	}
	for _, platform := range platforms {
		if platform != expectedSupportedPlatform {
			return signingResignUsage(fmt.Errorf("target platform is not %s", expectedSupportedPlatform))
		}
	}
	return nil
}

func signingResignPlatformString(value any, key string) (string, error) {
	text, ok := value.(string)
	if !ok || text == "" || strings.TrimSpace(text) != text || signingResignPlatformStringHasControl(text) {
		return "", fmt.Errorf("%s must be a non-empty string without control characters", key)
	}
	return text, nil
}

func signingResignPlatformStrings(value any) ([]string, error) {
	var values []any
	switch typed := value.(type) {
	case []string:
		values = make([]any, len(typed))
		for index, item := range typed {
			values[index] = item
		}
	case []any:
		values = typed
	default:
		return nil, fmt.Errorf("CFBundleSupportedPlatforms must be an array of strings")
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("CFBundleSupportedPlatforms must contain at least one platform")
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		platform, err := signingResignPlatformString(value, "CFBundleSupportedPlatforms entry")
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(platform)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("CFBundleSupportedPlatforms contains a duplicate platform")
		}
		seen[key] = struct{}{}
		result = append(result, platform)
	}
	return result, nil
}

func signingResignPlatformStringHasControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) || unicode.Is(unicode.Cf, character) || unicode.In(character, unicode.Bidi_Control) {
			return true
		}
	}
	return false
}

func enumerateSigningResignMachOFiles(ctx context.Context, rootPath string) ([]string, error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	paths, err := enumerateSigningResignMachOFilesRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(paths))
	for index, relative := range paths {
		result[index] = filepath.Join(rootPath, filepath.FromSlash(relative))
	}
	return result, nil
}

// enumerateSigningResignMachOFilesRoot walks the directory identity already
// selected by root. It returns only relative paths so a caller never needs to
// reconstruct and reopen the root's lexical pathname after materialization.
func enumerateSigningResignMachOFilesRoot(ctx context.Context, root *os.Root) ([]string, error) {
	if root == nil {
		return nil, fmt.Errorf("staging tree root is missing")
	}
	var result []string
	var walk func(current *os.Root, prefix string) error
	walk = func(current *os.Root, prefix string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		directory, err := current.Open(".")
		if err != nil {
			return err
		}
		entries, readErr := directory.ReadDir(-1)
		closeErr := directory.Close()
		if readErr != nil || closeErr != nil {
			return errors.Join(readErr, closeErr)
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			name := entry.Name()
			if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
				return fmt.Errorf("staging tree contains an invalid entry name")
			}
			before, err := current.Lstat(name)
			if err != nil {
				return err
			}
			if before.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("staging tree contains a symbolic link")
			}
			relative := path.Join(prefix, name)
			if before.IsDir() {
				child, err := current.OpenRoot(name)
				if err != nil {
					return err
				}
				afterPath, pathErr := current.Lstat(name)
				afterRoot, rootErr := child.Stat(".")
				if pathErr != nil || rootErr != nil || !os.SameFile(before, afterPath) || !os.SameFile(before, afterRoot) {
					_ = child.Close()
					return errors.Join(pathErr, rootErr, fmt.Errorf("staging tree directory changed during rooted open"))
				}
				walkErr := walk(child, relative)
				closeErr := child.Close()
				if walkErr != nil || closeErr != nil {
					return errors.Join(walkErr, closeErr)
				}
				continue
			}
			if !before.Mode().IsRegular() {
				return fmt.Errorf("staging tree contains a non-regular file")
			}
			file, err := secureopen.OpenExistingNoFollowInRoot(current, name)
			if err != nil {
				return err
			}
			opened, statErr := file.Stat()
			latest, lstatErr := current.Lstat(name)
			if statErr != nil || lstatErr != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, latest) {
				_ = file.Close()
				return errors.Join(statErr, lstatErr, fmt.Errorf("staging tree file changed during rooted open"))
			}
			isMachO := isSigningResignMachOFile(file, opened.Size())
			if closeErr := file.Close(); closeErr != nil {
				return closeErr
			}
			if isMachO {
				result = append(result, relative)
			}
		}
		return nil
	}
	if err := walk(root, ""); err != nil {
		return nil, err
	}
	sort.Strings(result)
	return result, nil
}

func isSigningResignMachOFile(file *os.File, fileSize int64) bool {
	return isSigningResignMachOFileWithPredicate(file, fileSize, isSigningResignLoadableMachO)
}

func isSigningResignAppExecutableFile(file *os.File, fileSize int64) bool {
	return isSigningResignMachOFileWithPredicate(file, fileSize, isSigningResignAppExecutableMachO)
}

func isSigningResignMachOFileWithPredicate(file *os.File, fileSize int64, predicate func(*macho.File) bool) bool {
	if file == nil || fileSize < 4 || predicate == nil {
		return false
	}
	var magic [4]byte
	if _, err := file.ReadAt(magic[:], 0); err != nil {
		return false
	}
	switch magic {
	case [4]byte{0xfe, 0xed, 0xfa, 0xce}, [4]byte{0xce, 0xfa, 0xed, 0xfe}, [4]byte{0xfe, 0xed, 0xfa, 0xcf}, [4]byte{0xcf, 0xfa, 0xed, 0xfe}:
		image, err := macho.NewFile(io.NewSectionReader(file, 0, fileSize))
		return err == nil && predicate(image)
	case [4]byte{0xca, 0xfe, 0xba, 0xbe}, [4]byte{0xbe, 0xba, 0xfe, 0xca}:
		var order binary.ByteOrder = binary.BigEndian
		if magic == [4]byte{0xbe, 0xba, 0xfe, 0xca} {
			order = binary.LittleEndian
		}
		return classifySigningResignFatMachOWithPredicate(file, fileSize, order, 20, predicate)
	case [4]byte{0xca, 0xfe, 0xba, 0xbf}, [4]byte{0xbf, 0xba, 0xfe, 0xca}:
		var order binary.ByteOrder = binary.BigEndian
		if magic == [4]byte{0xbf, 0xba, 0xfe, 0xca} {
			order = binary.LittleEndian
		}
		return classifySigningResignFatMachOWithPredicate(file, fileSize, order, 32, predicate)
	default:
		return false
	}
}

func isSigningResignLoadableMachO(file *macho.File) bool {
	return file != nil && (file.Type == macho.TypeExec || file.Type == macho.TypeDylib || file.Type == macho.TypeBundle)
}

func isSigningResignAppExecutableMachO(file *macho.File) bool {
	return file != nil && file.Type == macho.TypeExec
}

func classifySigningResignFatMachOWithPredicate(file *os.File, fileSize int64, order binary.ByteOrder, headerSize int64, predicate func(*macho.File) bool) bool {
	var countBytes [4]byte
	if _, err := file.ReadAt(countBytes[:], 4); err != nil {
		return false
	}
	count := order.Uint32(countBytes[:])
	tableEnd := int64(8) + int64(count)*headerSize
	if count == 0 || count > 64 || tableEnd > fileSize {
		return false
	}
	for index := uint32(0); index < count; index++ {
		header := make([]byte, headerSize)
		if _, err := file.ReadAt(header, int64(8)+int64(index)*headerSize); err != nil {
			return false
		}
		var offset, size uint64
		if headerSize == 20 {
			offset, size = uint64(order.Uint32(header[8:12])), uint64(order.Uint32(header[12:16]))
		} else {
			offset, size = order.Uint64(header[8:16]), order.Uint64(header[16:24])
		}
		if offset < uint64(tableEnd) || size == 0 || offset > uint64(fileSize) || size > uint64(fileSize)-offset {
			return false
		}
		image, err := macho.NewFile(io.NewSectionReader(file, int64(offset), int64(size)))
		if err != nil || !predicate(image) {
			return false
		}
		if uint32(image.Cpu) != order.Uint32(header[:4]) {
			return false
		}
	}
	return true
}

func buildSigningResignTargetIDs(targets []signingResignTarget) map[string]struct{} {
	result := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		result[target.BundleID] = struct{}{}
	}
	return result
}

func validateSigningResignTargetIDs(targets []signingResignTarget) error {
	seen := make(map[string]string, len(targets))
	for _, target := range targets {
		if previous, exists := seen[target.BundleID]; exists {
			return fmt.Errorf("duplicate bundle identifier in %s and %s", previous, target.RelativePath)
		}
		seen[target.BundleID] = target.RelativePath
	}
	return nil
}

func targetExecutablePath(treeRoot string, target signingResignTarget) string {
	return filepath.Join(treeRoot, filepath.FromSlash(path.Join(target.RelativePath, target.Executable)))
}
