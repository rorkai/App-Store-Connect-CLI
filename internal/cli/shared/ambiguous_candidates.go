package shared

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// AppCandidates converts apps into ambiguity candidates (ID, name, bundle ID).
func AppCandidates(apps []asc.Resource[asc.AppAttributes]) []AmbiguousCandidate {
	candidates := make([]AmbiguousCandidate, 0, len(apps))
	for _, app := range apps {
		candidates = append(candidates, appCandidate(app))
	}
	return candidates
}

func appCandidate(app asc.Resource[asc.AppAttributes]) AmbiguousCandidate {
	return AmbiguousCandidate{
		ID:    strings.TrimSpace(app.ID),
		Label: strings.TrimSpace(app.Attributes.Name),
		Extra: strings.TrimSpace(app.Attributes.BundleID),
	}
}

// AppStoreVersionCandidates converts App Store versions into ambiguity
// candidates (ID, "<version> <platform>", state).
func AppStoreVersionCandidates(versions []asc.Resource[asc.AppStoreVersionAttributes]) []AmbiguousCandidate {
	candidates := make([]AmbiguousCandidate, 0, len(versions))
	for _, version := range versions {
		label := strings.TrimSpace(version.Attributes.VersionString)
		if platform := strings.TrimSpace(string(version.Attributes.Platform)); platform != "" {
			label = strings.TrimSpace(label + " " + platform)
		}
		candidates = append(candidates, AmbiguousCandidate{
			ID:    strings.TrimSpace(version.ID),
			Label: label,
			Extra: asc.ResolveAppStoreVersionState(version.Attributes),
		})
	}
	return candidates
}

// AppInfoAmbiguousCandidates converts app info candidates into ambiguity
// candidates (ID, state).
func AppInfoAmbiguousCandidates(appInfos []asc.AppInfoCandidate) []AmbiguousCandidate {
	candidates := make([]AmbiguousCandidate, 0, len(appInfos))
	for _, appInfo := range appInfos {
		state := strings.TrimSpace(appInfo.State)
		if state == "" {
			state = "unknown"
		}
		candidates = append(candidates, AmbiguousCandidate{ID: strings.TrimSpace(appInfo.ID), Label: state})
	}
	return candidates
}

// BuildCandidates converts builds into ambiguity candidates
// (ID, build number, uploaded date and processing state).
func BuildCandidates(builds []asc.Resource[asc.BuildAttributes]) []AmbiguousCandidate {
	candidates := make([]AmbiguousCandidate, 0, len(builds))
	for _, build := range builds {
		candidates = append(candidates, BuildCandidate(build))
	}
	return candidates
}

// BuildCandidate converts one build into an ambiguity candidate.
func BuildCandidate(build asc.Resource[asc.BuildAttributes]) AmbiguousCandidate {
	extra := strings.TrimSpace(build.Attributes.UploadedDate)
	if state := strings.TrimSpace(build.Attributes.ProcessingState); state != "" {
		extra = strings.TrimSpace(extra + " " + state)
	}
	return AmbiguousCandidate{
		ID:    strings.TrimSpace(build.ID),
		Label: strings.TrimSpace(build.Attributes.Version),
		Extra: extra,
	}
}

// BetaTesterCandidates converts beta testers into ambiguity candidates
// (ID, email, name).
func BetaTesterCandidates(testers []asc.Resource[asc.BetaTesterAttributes]) []AmbiguousCandidate {
	candidates := make([]AmbiguousCandidate, 0, len(testers))
	for _, tester := range testers {
		candidates = append(candidates, AmbiguousCandidate{
			ID:    strings.TrimSpace(tester.ID),
			Label: strings.TrimSpace(tester.Attributes.Email),
			Extra: strings.TrimSpace(strings.TrimSpace(tester.Attributes.FirstName) + " " + strings.TrimSpace(tester.Attributes.LastName)),
		})
	}
	return candidates
}

// LocalizationCandidates converts resources into ambiguity candidates keyed
// by ID with the given locale as the label.
func LocalizationCandidates[T any](items []asc.Resource[T], locale func(T) string) []AmbiguousCandidate {
	candidates := make([]AmbiguousCandidate, 0, len(items))
	for _, item := range items {
		candidates = append(candidates, AmbiguousCandidate{
			ID:    strings.TrimSpace(item.ID),
			Label: strings.TrimSpace(locale(item.Attributes)),
		})
	}
	return candidates
}

func sortAmbiguousCandidatesByID(candidates []AmbiguousCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].ID < candidates[j].ID
	})
}

// AmbiguousAppInfoError reports that an app has several app infos and names
// the flag that accepts one app info ID.
func AmbiguousAppInfoError(appID, flag string, appInfos []asc.AppInfoCandidate) error {
	appID = strings.TrimSpace(appID)
	return &AmbiguousSelectionError{
		Kind:        "app info",
		Description: fmt.Sprintf("app %q", appID),
		Flag:        flag,
		Candidates:  AppInfoAmbiguousCandidates(appInfos),
		Hint:        fmt.Sprintf("Inspect them with `asc apps info list --app %q`.", appID),
	}
}

// AmbiguousLocalizationError reports duplicate localizations for one locale.
// App Store Connect should never return more than one, so no flag can select
// between them; the candidates are listed so the duplicate can be repaired.
func AmbiguousLocalizationError(kind, locale string, candidates []AmbiguousCandidate) error {
	return &AmbiguousSelectionError{
		Kind:        kind,
		Description: fmt.Sprintf("locale %q", strings.TrimSpace(locale)),
		Candidates:  candidates,
		Hint:        "App Store Connect returned duplicate localizations for this locale; remove the duplicate before retrying.",
	}
}

// AmbiguousAppStoreVersionError reports several App Store versions for one
// version string. When the matches span platforms and platformFlag is set,
// the candidates are the platform values to pass; otherwise the candidates
// are the version IDs for versionIDFlag (which may be empty when the command
// accepts no version ID).
func AmbiguousAppStoreVersionError(version, platform string, versions []asc.Resource[asc.AppStoreVersionAttributes], platformFlag, versionIDFlag string) error {
	description := fmt.Sprintf("version %q", strings.TrimSpace(version))
	if platform = strings.TrimSpace(platform); platform != "" {
		description += fmt.Sprintf(" on platform %q", platform)
	}
	platforms := distinctAppStoreVersionPlatforms(versions)
	// A platform value only disambiguates when it maps to exactly one version;
	// duplicates on one platform stay ambiguous after --platform is passed.
	if platform == "" && platformFlag != "" && len(platforms) > 1 && len(platforms) == len(versions) {
		candidates := make([]AmbiguousCandidate, 0, len(versions))
		for _, item := range versions {
			candidates = append(candidates, AmbiguousCandidate{
				ID:    strings.TrimSpace(string(item.Attributes.Platform)),
				Label: "version " + strings.TrimSpace(item.ID),
				Extra: asc.ResolveAppStoreVersionState(item.Attributes),
			})
		}
		return &AmbiguousSelectionError{
			Kind:        "app store version",
			Description: description,
			Flag:        platformFlag,
			Candidates:  candidates,
		}
	}
	hint := ""
	if versionIDFlag == "" && platformFlag != "" && len(platforms) > 1 {
		hint = fmt.Sprintf("More than one version matches on a single platform, so %s cannot select one on its own.", platformFlag)
	}
	return &AmbiguousSelectionError{
		Kind:        "app store version",
		Description: description,
		Flag:        versionIDFlag,
		Candidates:  AppStoreVersionCandidates(versions),
		Hint:        hint,
	}
}

func distinctAppStoreVersionPlatforms(versions []asc.Resource[asc.AppStoreVersionAttributes]) []string {
	seen := make(map[string]struct{}, len(versions))
	platforms := make([]string, 0, len(versions))
	for _, item := range versions {
		platform := strings.TrimSpace(string(item.Attributes.Platform))
		if platform == "" {
			continue
		}
		if _, ok := seen[platform]; ok {
			continue
		}
		seen[platform] = struct{}{}
		platforms = append(platforms, platform)
	}
	return platforms
}
