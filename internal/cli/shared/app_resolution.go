package shared

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// ResolveAppStoreVersionIDAndState finds a version ID and state by version string and platform.
func ResolveAppStoreVersionIDAndState(ctx context.Context, client *asc.Client, appID, version, platform string) (string, string, error) {
	opts := []asc.AppStoreVersionsOption{
		asc.WithAppStoreVersionsVersionStrings([]string{version}),
		asc.WithAppStoreVersionsPlatforms([]string{platform}),
		asc.WithAppStoreVersionsLimit(10),
	}
	resp, err := client.GetAppStoreVersions(ctx, appID, opts...)
	if err != nil {
		return "", "", err
	}
	if resp == nil || len(resp.Data) == 0 {
		return "", "", fmt.Errorf("app store version not found for version %q and platform %q", version, platform)
	}
	if len(resp.Data) > 1 {
		return "", "", fmt.Errorf("multiple app store versions found for version %q and platform %q (use --version-id)", version, platform)
	}
	return resp.Data[0].ID, asc.ResolveAppStoreVersionState(resp.Data[0].Attributes), nil
}

// ResolveAppStoreVersionID finds a version ID by version string and platform.
func ResolveAppStoreVersionID(ctx context.Context, client *asc.Client, appID, version, platform string) (string, error) {
	versionID, _, err := ResolveAppStoreVersionIDAndState(ctx, client, appID, version, platform)
	return versionID, err
}

// ResolveAppInfoID resolves the app info ID, optionally using a provided override.
func ResolveAppInfoID(ctx context.Context, client *asc.Client, appID, appInfoID string) (string, error) {
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
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("no app info found for app %q", appID)
	}
	if len(resp.Data) > 1 {
		selected, reason := autoSelectEditableAppInfoID(resp)
		if selected == "" {
			candidates := asc.FormatAppInfoCandidates(asc.AppInfoCandidates(resp.Data))
			return "", fmt.Errorf("multiple app infos found for app %q (%s); run `asc apps info list --app %q` to inspect candidates, then pass the explicit app info ID", appID, candidates, appID)
		}
		fmt.Fprintf(os.Stderr, "Multiple app infos found for app %s, auto-selected %s (%s).\n", appID, selected, reason)
		return selected, nil
	}
	return resp.Data[0].ID, nil
}

func autoSelectEditableAppInfoID(appInfos *asc.AppInfosResponse) (string, string) {
	if appInfos == nil {
		return "", ""
	}

	const targetState = "PREPARE_FOR_SUBMISSION"

	matches := make([]string, 0, 1)
	for _, info := range appInfos.Data {
		state := strings.ToUpper(appInfoAttrString(info.Attributes, "state"))
		appStoreState := strings.ToUpper(appInfoAttrString(info.Attributes, "appStoreState"))
		if state != targetState && appStoreState != targetState {
			continue
		}
		if trimmedID := strings.TrimSpace(info.ID); trimmedID != "" {
			matches = append(matches, trimmedID)
		}
	}

	if len(matches) != 1 {
		return "", ""
	}

	return matches[0], targetState
}
