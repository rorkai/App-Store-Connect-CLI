package shared

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

var buildProcessingStateAllValues = []string{
	asc.BuildProcessingStateProcessing,
	asc.BuildProcessingStateFailed,
	asc.BuildProcessingStateInvalid,
	asc.BuildProcessingStateValid,
}

// BuildProcessingStateFilterOptions customizes state normalization behavior for
// command-specific flag names/help text and optional aliases.
type BuildProcessingStateFilterOptions struct {
	FlagName          string
	AllowedValuesHelp string
	Aliases           map[string]string
}

// NormalizeBuildProcessingStateFilter parses and validates CSV processing-state
// filters (including "all"), deduplicates values, and applies optional aliases.
func NormalizeBuildProcessingStateFilter(raw string, options BuildProcessingStateFilterOptions) ([]string, error) {
	return normalizeBuildProcessingStateFilterTo(os.Stderr, raw, options)
}

// NormalizeBuildProcessingStateFilter is NormalizeBuildProcessingStateFilter
// with the "Error:" line written to d's stderr.
func (d *Deps) NormalizeBuildProcessingStateFilter(raw string, options BuildProcessingStateFilterOptions) ([]string, error) {
	return normalizeBuildProcessingStateFilterTo(d.IO.ErrOut, raw, options)
}

func normalizeBuildProcessingStateFilterTo(stderr io.Writer, raw string, options BuildProcessingStateFilterOptions) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	flagName := strings.TrimSpace(options.FlagName)
	if flagName == "" {
		flagName = "--processing-state"
	}

	allowedValuesHelp := strings.TrimSpace(options.AllowedValuesHelp)
	if allowedValuesHelp == "" {
		allowedValuesHelp = "VALID, PROCESSING, FAILED, INVALID, or all"
	}

	values := SplitCSVUpper(raw)
	if len(values) == 0 {
		return nil, usageErrorfTo(stderr, "%s must include at least one state", flagName)
	}

	if len(values) == 1 && values[0] == "ALL" {
		return append([]string(nil), buildProcessingStateAllValues...), nil
	}

	allowed := map[string]struct{}{
		asc.BuildProcessingStateValid:      {},
		asc.BuildProcessingStateProcessing: {},
		asc.BuildProcessingStateFailed:     {},
		asc.BuildProcessingStateInvalid:    {},
	}

	aliasTargets := make(map[string]string, len(options.Aliases))
	for aliasRaw, targetRaw := range options.Aliases {
		alias := strings.ToUpper(strings.TrimSpace(aliasRaw))
		target := strings.ToUpper(strings.TrimSpace(targetRaw))
		if alias == "" || target == "" {
			continue
		}
		if _, ok := allowed[target]; !ok {
			return nil, fmt.Errorf("invalid build processing state alias target %q for alias %q", targetRaw, aliasRaw)
		}
		allowed[alias] = struct{}{}
		aliasTargets[alias] = target
	}

	resolved := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "ALL" {
			return nil, usageErrorfTo(stderr, "%s value \"all\" cannot be combined with other states", flagName)
		}
		if _, ok := allowed[value]; !ok {
			return nil, usageErrorfTo(stderr, "%s must be one of %s", flagName, allowedValuesHelp)
		}

		if target, ok := aliasTargets[value]; ok {
			value = target
		}
		if _, ok := seen[value]; ok {
			continue
		}

		seen[value] = struct{}{}
		resolved = append(resolved, value)
	}

	return resolved, nil
}
