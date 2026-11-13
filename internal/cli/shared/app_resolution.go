package shared

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

var appStoreVersionStringPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)

// IsAppStoreVersionString reports whether a selector is a numeric dotted version string.
func IsAppStoreVersionString(value string) bool {
	return appStoreVersionStringPattern.MatchString(value)
}

// ResolveAppStoreVersionIDAndState finds a version ID and state by version string and platform.
func ResolveAppStoreVersionIDAndState(ctx context.Context, client *asc.Client, appID, version, platform string) (string, string, error) {
	opts := []asc.AppStoreVersionsOption{
		asc.WithAppStoreVersionsVersionStrings([]string{version}),
		asc.WithAppStoreVersionsPlatforms([]string{platform}),
		asc.Limit[asc.AppStoreVersionsOption](10),
	}
	resp, err := client.GetAppStoreVersions(ctx, appID, opts...)
	if err != nil {
		return "", "", err
	}
	if resp == nil {
		return "", "", NewErrorWithCause(
			fmt.Errorf("app store version not found for version %q and platform %q", version, platform),
			asc.ErrNotFound,
		)
	}
	pageHasNext := strings.TrimSpace(resp.Links.Next) != ""
	if pageHasNext {
		return "", "", MarkAmbiguousSelectionSample(AmbiguousAppStoreVersionError(version, platform, resp.Data, "", "--version-id"))
	}
	if len(resp.Data) == 0 {
		return "", "", WithAppStoreVersionNotFoundDiagnostics(ctx, client, appID, version, platform, NewErrorWithCause(
			fmt.Errorf("app store version not found for version %q and platform %q", version, platform),
			asc.ErrNotFound,
		))
	}
	if len(resp.Data) > 1 {
		return "", "", AmbiguousAppStoreVersionError(version, platform, resp.Data, "", "--version-id")
	}
	return resp.Data[0].ID, asc.ResolveAppStoreVersionState(resp.Data[0].Attributes), nil
}

// ResolveAppStoreVersionID finds a version ID by version string and platform.
func ResolveAppStoreVersionID(ctx context.Context, client *asc.Client, appID, version, platform string) (string, error) {
	versionID, _, err := ResolveAppStoreVersionIDAndState(ctx, client, appID, version, platform)
	return versionID, err
}

// ResolveOwnedAppStoreVersionByID fetches a version by ID and validates its app ownership and optional platform.
func ResolveOwnedAppStoreVersionByID(ctx context.Context, client *asc.Client, appID, versionID, platform string) (asc.Resource[asc.AppStoreVersionAttributes], error) {
	trimmedVersionID := strings.TrimSpace(versionID)
	resp, err := client.GetAppStoreVersion(ctx, trimmedVersionID, asc.WithAppStoreVersionInclude([]string{"app"}))
	if err != nil {
		return asc.Resource[asc.AppStoreVersionAttributes]{}, err
	}
	relatedAppID, err := asc.AppStoreVersionAppID(resp)
	if err != nil {
		return asc.Resource[asc.AppStoreVersionAttributes]{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(relatedAppID), strings.TrimSpace(appID)) {
		mismatch := fmt.Errorf("version %q belongs to app %q, not %q", trimmedVersionID, relatedAppID, appID)
		return asc.Resource[asc.AppStoreVersionAttributes]{}, WithDiagnostic(NewValidationError(mismatch), DiagnosticInvalidInput, "--version-id")
	}
	resolvedPlatform := strings.TrimSpace(string(resp.Data.Attributes.Platform))
	if strings.TrimSpace(platform) != "" && !strings.EqualFold(resolvedPlatform, strings.TrimSpace(platform)) {
		mismatch := fmt.Errorf("version %q is on platform %q, not %q", strings.TrimSpace(resp.Data.ID), resolvedPlatform, strings.TrimSpace(platform))
		return asc.Resource[asc.AppStoreVersionAttributes]{}, WithDiagnostic(NewValidationError(mismatch), DiagnosticInvalidInput, "--version-id")
	}
	return resp.Data, nil
}

// ResolveAppInfoID resolves the app info ID, optionally using a provided
// override supplied through --app-info.
func ResolveAppInfoID(ctx context.Context, client *asc.Client, appID, appInfoID string) (string, error) {
	return ResolveAppInfoIDWithFlag(ctx, client, appID, appInfoID, "--app-info")
}

// ResolveAppInfoIDWithFlag resolves the app info ID, optionally using a
// provided override. appInfoFlag names the command's override flag so an
// ambiguity error can tell the caller which flag accepts one candidate ID.
func ResolveAppInfoIDWithFlag(ctx context.Context, client *asc.Client, appID, appInfoID, appInfoFlag string) (string, error) {
	if strings.TrimSpace(appInfoID) != "" {
		return strings.TrimSpace(appInfoID), nil
	}
	if strings.TrimSpace(appID) == "" {
		return "", fmt.Errorf("app id is required")
	}

	resp, err := client.GetAppInfos(ctx, appID)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("empty app infos response for app %q", appID)
	}
	pageHasNext := strings.TrimSpace(resp.Links.Next) != ""
	if pageHasNext {
		return "", MarkAmbiguousSelectionSample(AmbiguousAppInfoError(appID, appInfoFlag, asc.AppInfoCandidates(resp.Data)))
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("no app info found for app %q", appID)
	}
	candidates := asc.AppInfoCandidates(resp.Data)
	if current := asc.CurrentAppInfoCandidates(candidates); len(current) > 0 {
		candidates = current
	}
	if len(candidates) == 1 {
		return candidates[0].ID, nil
	}
	if selected, ok := asc.AutoResolveAppInfoIDByVersionState(candidates, "PREPARE_FOR_SUBMISSION"); ok {
		fmt.Fprintf(os.Stderr, "Multiple app infos found for app %s, auto-selected %s (PREPARE_FOR_SUBMISSION).\n", appID, selected)
		return selected, nil
	}
	return "", AmbiguousAppInfoError(appID, appInfoFlag, candidates)
}

// ResolveOwnedAppInfoID resolves the app info ID and verifies that an explicit
// override belongs to the requested app.
func ResolveOwnedAppInfoID(ctx context.Context, client *asc.Client, appID, appInfoID string) (string, error) {
	resolvedAppInfoID, err := ResolveAppInfoID(ctx, client, appID, appInfoID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(appInfoID) == "" {
		return resolvedAppInfoID, nil
	}

	resp, err := client.GetAppInfo(ctx, resolvedAppInfoID, asc.WithAppInfoInclude([]string{"app"}))
	if err != nil {
		return "", err
	}
	ownerAppID, err := asc.AppInfoAppID(resp)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(strings.TrimSpace(ownerAppID), strings.TrimSpace(appID)) {
		return "", fmt.Errorf("app info %q belongs to app %q, not %q", resolvedAppInfoID, ownerAppID, strings.TrimSpace(appID))
	}
	return resolvedAppInfoID, nil
}
