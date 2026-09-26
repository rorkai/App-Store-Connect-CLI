package metadata

import (
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func effectiveVersionCreateLocalization(localPatch versionLocalPatch) VersionLocalization {
	createLoc := localPatch.localization
	if hasVersionContent(localPatch.createLocalization) {
		createLoc = localPatch.createLocalization
	}
	return NormalizeVersionLocalization(createLoc)
}

func versionCreateWarningForPatch(locale string, localPatch versionLocalPatch, remoteExists bool, mode string, opts shared.SubmitReadinessOptions) (shared.SubmitReadinessCreateWarning, bool) {
	if remoteExists {
		return shared.SubmitReadinessCreateWarning{}, false
	}
	return shared.SubmitReadinessCreateWarningForLocaleWithOptions(
		locale,
		versionAttributes(locale, effectiveVersionCreateLocalization(localPatch), true),
		mode,
		opts,
	)
}

func versionCreateWarningsForPatches(local map[string]versionLocalPatch, remote map[string]VersionLocalization, mode string, opts shared.SubmitReadinessOptions) []shared.SubmitReadinessCreateWarning {
	warnings := make([]shared.SubmitReadinessCreateWarning, 0, len(local))
	for _, locale := range sortedKeys(local) {
		warning, ok := versionCreateWarningForPatch(locale, local[locale], hasRemoteVersionLocalization(remote, locale), mode, opts)
		if !ok {
			continue
		}
		warnings = append(warnings, warning)
	}
	return shared.NormalizeSubmitReadinessCreateWarnings(warnings)
}

func successfulVersionCreateWarnings(
	warnings []shared.SubmitReadinessCreateWarning,
	actions []ApplyAction,
	remote map[string]VersionLocalization,
) []shared.SubmitReadinessCreateWarning {
	succeeded := make(map[string]struct{})
	for _, action := range actions {
		if action.Scope != versionDirName || action.Status != metadataActionStatusSucceeded || action.AlreadyExists || action.reconciledDuplicate || hasRemoteVersionLocalization(remote, action.Locale) {
			continue
		}
		succeeded[action.Locale] = struct{}{}
	}

	filtered := make([]shared.SubmitReadinessCreateWarning, 0, len(warnings))
	for _, warning := range warnings {
		if _, ok := succeeded[warning.Locale]; ok {
			filtered = append(filtered, warning)
		}
	}
	return shared.NormalizeSubmitReadinessCreateWarnings(filtered)
}

func versionCreateWarningsNeedUpdateContext(local map[string]versionLocalPatch, remote map[string]VersionLocalization) bool {
	for _, locale := range sortedKeys(local) {
		if hasRemoteVersionLocalization(remote, locale) {
			continue
		}
		attrs := versionAttributes(locale, effectiveVersionCreateLocalization(local[locale]), true)
		if strings.TrimSpace(attrs.WhatsNew) == "" {
			return true
		}
	}
	return false
}

func hasRemoteVersionLocalization(remote map[string]VersionLocalization, locale string) bool {
	if len(remote) == 0 {
		return false
	}
	_, ok := remote[locale]
	return ok
}
