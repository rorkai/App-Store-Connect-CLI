package auth

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kballard/go-shellquote"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// privateKeyFileMode is the owner-only mode both `auth doctor --fix` and
// `auth login --fix-permissions` apply to a credential file.
const privateKeyFileMode fs.FileMode = 0o600

func filePermissionsTooPermissive(mode fs.FileMode) bool {
	return filePermissionsTooPermissiveForOS(mode, runtime.GOOS)
}

func filePermissionsTooPermissiveForOS(mode fs.FileMode, goos string) bool {
	return goos != "windows" && mode.Perm()&0o077 != 0
}

// FilePermissionRemediationCommand returns the exact command an operator can
// run to tighten an over-permissive credential file. Callers print it so a
// failed read names the one command that repairs the file, and so the
// recommendation text never drifts between `auth doctor` and `auth login`.
//
// The path is quoted for a POSIX shell, so a path holding metacharacters such
// as `$(...)`, a backtick, or `$VAR` is pasted literally instead of being
// expanded, and a leading dash is anchored so chmod reads it as a file rather
// than a flag. A path carrying control or display-reordering characters cannot
// be rendered as a trustworthy copy-paste command, so callers receive
// safe=false and must offer a command that does not embed the path.
func FilePermissionRemediationCommand(path string) (command string, safe bool) {
	target, safe := remediationTarget(path)
	if !safe {
		return "", false
	}
	return shellquote.Join("chmod", "600", target), true
}

func remediationTarget(path string) (string, bool) {
	if path == "" || !utf8.ValidString(path) || containsNonDisplayableRune(path) {
		return "", false
	}
	if strings.ContainsRune("-#=", rune(path[0])) {
		return "./" + path, true
	}
	return path, true
}

// PrivateKeyFileFix describes how to make a refused private key file readable
// by its user alone.
type PrivateKeyFileFix struct {
	// Mode is the file's current permission bits.
	Mode fs.FileMode
	// OwnedByCaller is false when another user owns the file, so neither
	// chmod nor --fix-permissions can change it until its owner changes.
	OwnedByCaller bool
	// Commands repair the file in order. It is empty when the path cannot be
	// rendered as a trustworthy copy-paste command.
	Commands []string
}

// PrivateKeyFileFixFor inspects the key file at path, without changing it, and
// returns the commands that give the current user sole read and write access.
func PrivateKeyFileFixFor(path string) (PrivateKeyFileFix, error) {
	return privateKeyFileFixForUID(path, os.Getuid())
}

func privateKeyFileFixForUID(path string, uid int) (PrivateKeyFileFix, error) {
	info, err := os.Stat(path)
	if err != nil {
		return PrivateKeyFileFix{}, err
	}
	fix := PrivateKeyFileFix{Mode: info.Mode().Perm(), OwnedByCaller: !fileOwnedByOtherUser(info, uid)}
	chmod, safe := FilePermissionRemediationCommand(path)
	if !safe {
		return fix, nil
	}
	if !fix.OwnedByCaller {
		target, _ := remediationTarget(path)
		fix.Commands = append(fix.Commands, `sudo chown "$(id -un)" `+shellquote.Join(target))
	}
	fix.Commands = append(fix.Commands, chmod)
	return fix, nil
}

func containsNonDisplayableRune(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			return true
		}
	}
	return false
}

// FixPrivateKeyFilePermissions tightens an over-permissive private key file to
// 0600 using the same rule ValidateKeyFile and `auth doctor` apply, and reports
// whether the mode changed so callers can tell the operator what was altered.
// A file that is already owner-only is left exactly as it is. The mode is
// applied through the rooted traversal used elsewhere for operator-selected
// paths, so a symlinked component, a directory, or any other non-regular file
// is rejected instead of being changed. Platforms that cannot securely open a
// key which denies its owner read access fail closed. Failures carry
// PrivateKeyError kinds, keeping the existing private-key diagnostics.
func FixPrivateKeyFilePermissions(path string) (bool, error) {
	return fixPrivateKeyFilePermissionsForOS(path, runtime.GOOS)
}

func fixPrivateKeyFilePermissionsForOS(path, goos string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, newPrivateKeyError(privateKeyAccessErrorKind(err), fmt.Errorf("failed to stat key file: %w", err))
	}
	if info.IsDir() {
		return false, newPrivateKeyError(PrivateKeyInvalidFormat, errors.New("private key path is a directory"))
	}
	if !info.Mode().IsRegular() {
		return false, newPrivateKeyError(PrivateKeyInvalidFormat, errors.New("private key path is not a regular file"))
	}
	if !filePermissionsTooPermissiveForOS(info.Mode(), goos) {
		return false, nil
	}
	if err := rootfs.ChmodFileIfSame(path, info, privateKeyFileMode); err != nil {
		return false, newPrivateKeyError(privateKeyAccessErrorKind(err), fmt.Errorf("failed to change key file permissions: %w", err))
	}
	return true, nil
}
