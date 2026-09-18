package shared

import (
	"errors"
	"io"
	"os"
	"strings"
)

const (
	indirectEnvPrefix    = "@env:"
	indirectFilePrefix   = "@file:"
	indirectEscapePrefix = "@@"

	// MaxIndirectFileValueSize bounds a flag value read through @file:PATH.
	// Flag values are release notes, JSON bodies, or secrets, never bulk
	// artifacts, so a bounded read keeps a mistaken path from loading a
	// large file into memory.
	MaxIndirectFileValueSize = 1 << 20
)

// HasFlagValueIndirection reports whether value carries one of the
// indirection prefixes, so callers that must not read the environment or the
// filesystem (help rendering) can recognize such a value without resolving it.
func HasFlagValueIndirection(value string) bool {
	return strings.HasPrefix(value, indirectEnvPrefix) ||
		strings.HasPrefix(value, indirectFilePrefix) ||
		strings.HasPrefix(value, indirectEscapePrefix)
}

// ResolveFlagValueIndirection expands the value-indirection prefixes shared by
// every value-taking flag: `@env:NAME` resolves to that environment variable,
// `@file:PATH` resolves to that file's contents with one trailing line ending
// removed, and a leading `@@` escapes a literal `@`. Any other value, including
// values that merely start with `@`, is returned unchanged. flagName is the
// bare flag name used in diagnostics.
func ResolveFlagValueIndirection(flagName, value string) (string, error) {
	switch {
	case strings.HasPrefix(value, indirectEscapePrefix):
		return value[1:], nil
	case strings.HasPrefix(value, indirectEnvPrefix):
		return resolveIndirectEnvValue(flagName, value[len(indirectEnvPrefix):])
	case strings.HasPrefix(value, indirectFilePrefix):
		return resolveIndirectFileValue(flagName, value[len(indirectFilePrefix):])
	default:
		return value, nil
	}
}

func resolveIndirectEnvValue(flagName, name string) (string, error) {
	if name == "" {
		return "", indirectValueUsageError(flagName, "@env: requires an environment variable name")
	}
	value, ok := os.LookupEnv(name)
	if !ok {
		return "", indirectValueUsageError(flagName, "environment variable "+SanitizeTerminal(name)+" is not set")
	}
	if value == "" {
		return "", indirectValueUsageError(flagName, "environment variable "+SanitizeTerminal(name)+" is empty")
	}
	return value, nil
}

func resolveIndirectFileValue(flagName, path string) (string, error) {
	if path == "" {
		return "", indirectValueUsageError(flagName, "@file: requires a file path")
	}
	label := SanitizeTerminal(path)
	// An operator-supplied path follows the --password-file precedent: open
	// without following a symlink at the final component, require a regular
	// file, and bound the read.
	file, err := OpenExistingNoFollow(path)
	if err != nil {
		return "", indirectValueUsageError(flagName, "cannot read file "+label+": "+indirectFileErrorReason(err))
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", indirectValueUsageError(flagName, "cannot read file "+label+": "+indirectFileErrorReason(err))
	}
	if !info.Mode().IsRegular() {
		return "", indirectValueUsageError(flagName, "cannot read file "+label+": not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxIndirectFileValueSize+1))
	if err != nil {
		return "", indirectValueUsageError(flagName, "cannot read file "+label+": "+indirectFileErrorReason(err))
	}
	if len(data) > MaxIndirectFileValueSize {
		return "", indirectValueUsageError(flagName, "file "+label+" exceeds the 1 MiB limit")
	}
	// Remove exactly one trailing line ending: the complete CRLF pair or a
	// lone LF. A lone CR is data, not a line ending asc recognizes, so it
	// survives in secrets and payload text that legitimately end in `\r`.
	value := string(data)
	if strings.HasSuffix(value, "\r\n") {
		value = value[:len(value)-2]
	} else {
		value = strings.TrimSuffix(value, "\n")
	}
	if value == "" {
		return "", indirectValueUsageError(flagName, "file "+label+" is empty")
	}
	return value, nil
}

func indirectFileErrorReason(err error) string {
	if errors.Is(err, os.ErrNotExist) {
		return "file does not exist"
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return SanitizeTerminal(pathErr.Err.Error())
	}
	return SanitizeTerminal(err.Error())
}

// indirectValueUsageError classifies an indirection failure as an invalid
// value on the named flag. The message is returned, not printed, so the root
// runner controls the stderr line and the exit code.
func indirectValueUsageError(flagName, message string) error {
	parameter := "--" + flagName
	return WithDiagnostic(
		classifiedUsageError{kind: UsageErrorInvalidValue, message: parameter + ": " + message},
		DiagnosticInvalidInput,
		parameter,
	)
}
