package testflight

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type trackedStringFlag struct {
	value string
	set   bool
}

func (f *trackedStringFlag) String() string {
	if f == nil {
		return ""
	}
	return f.value
}

func (f *trackedStringFlag) Set(value string) error {
	f.value = value
	f.set = true
	return nil
}

func (f *trackedStringFlag) Used() bool {
	return f != nil && f.set
}

func (f *trackedStringFlag) Value() string {
	if f == nil {
		return ""
	}
	return strings.TrimSpace(f.value)
}

func bindHiddenStringFlag(fs *flag.FlagSet, name, usage string) *trackedStringFlag {
	value := &trackedStringFlag{}
	fs.Var(value, name, usage)
	shared.HideFlagFromHelp(fs.Lookup(name))
	return value
}

func bindBuildIDFlag(fs *flag.FlagSet, usage string) (*string, *trackedStringFlag) {
	return fs.String("build-id", "", usage), bindHiddenStringFlag(fs, "build", "DEPRECATED: use --build-id")
}

const legacyBuildIDWarning = "Warning: `--build` is deprecated. Use `--build-id`."

func applyLegacyBuildIDAlias(buildID *string, legacyBuildID *trackedStringFlag) error {
	if buildID == nil || legacyBuildID == nil || !legacyBuildID.Used() {
		return nil
	}

	legacyValue := legacyBuildID.Value()
	canonicalValue := strings.TrimSpace(*buildID)
	if canonicalValue != "" && legacyValue != "" && canonicalValue != legacyValue {
		return shared.WithDiagnostic(
			shared.UsageError("--build conflicts with --build-id; use only --build-id"),
			shared.DiagnosticConflictingInput,
			"",
		)
	}
	if canonicalValue == "" {
		*buildID = legacyValue
	}

	fmt.Fprintln(os.Stderr, legacyBuildIDWarning)
	return nil
}
