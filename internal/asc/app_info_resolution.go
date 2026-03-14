package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type appStoreVersionRelationships struct {
	App *Relationship `json:"app"`
}

// AppInfoCandidate describes an app info resource considered for auto-resolution.
type AppInfoCandidate struct {
	ID    string
	State string
}

// ResolveAppInfoIDForAppStoreVersion resolves the app info backing a version-scoped workflow.
func (c *Client) ResolveAppInfoIDForAppStoreVersion(ctx context.Context, versionID string) (string, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return "", fmt.Errorf("versionID is required")
	}

	versionResp, err := c.GetAppStoreVersion(ctx, versionID, WithAppStoreVersionInclude([]string{"app"}))
	if err != nil {
		return "", err
	}

	var relationships appStoreVersionRelationships
	if len(versionResp.Data.Relationships) > 0 {
		if err := json.Unmarshal(versionResp.Data.Relationships, &relationships); err != nil {
			return "", fmt.Errorf("failed to parse app store version relationships: %w", err)
		}
	}

	appID := ""
	if relationships.App != nil {
		appID = strings.TrimSpace(relationships.App.Data.ID)
	}
	if appID == "" {
		return "", fmt.Errorf("app relationship missing for app store version %q", versionID)
	}

	appInfos, err := c.GetAppInfos(ctx, appID)
	if err != nil {
		return "", err
	}
	if len(appInfos.Data) == 0 {
		return "", fmt.Errorf("no app info found for app %q", appID)
	}
	if len(appInfos.Data) == 1 {
		return strings.TrimSpace(appInfos.Data[0].ID), nil
	}

	candidates := AppInfoCandidates(appInfos.Data)

	if resolvedID, ok := AutoResolveAppInfoIDByVersionState(candidates, ResolveAppStoreVersionState(versionResp.Data.Attributes)); ok {
		return resolvedID, nil
	}

	return "", fmt.Errorf(
		"multiple app infos found for app %q (%s); run `asc apps info list --app %q` to inspect candidates and use the app-info based age-rating flow explicitly",
		appID,
		FormatAppInfoCandidates(candidates),
		appID,
	)
}

// ResolveAppStoreVersionState prefers AppVersionState and falls back to AppStoreState.
func ResolveAppStoreVersionState(attrs AppStoreVersionAttributes) string {
	if trimmed := strings.TrimSpace(attrs.AppVersionState); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(attrs.AppStoreState)
}

// AppInfoCandidates returns sorted app info resolution candidates for a response payload.
func AppInfoCandidates(appInfos []Resource[AppInfoAttributes]) []AppInfoCandidate {
	candidates := make([]AppInfoCandidate, 0, len(appInfos))
	for _, item := range appInfos {
		candidates = append(candidates, AppInfoCandidate{
			ID:    strings.TrimSpace(item.ID),
			State: appInfoState(item.Attributes),
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ID < candidates[j].ID
	})
	return candidates
}

// AutoResolveAppInfoIDByVersionState resolves a single app info candidate matching a version state.
func AutoResolveAppInfoIDByVersionState(candidates []AppInfoCandidate, versionState string) (string, bool) {
	resolvedVersionState := strings.TrimSpace(versionState)
	if resolvedVersionState == "" {
		return "", false
	}

	acceptableStates := acceptableAppInfoStatesForVersionState(resolvedVersionState)
	matches := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ID == "" || !matchesAppInfoState(candidate.State, acceptableStates) {
			continue
		}
		matches = append(matches, candidate.ID)
	}
	if len(matches) != 1 {
		return "", false
	}
	return matches[0], true
}

func acceptableAppInfoStatesForVersionState(versionState string) []string {
	resolvedVersionState := strings.TrimSpace(versionState)
	if resolvedVersionState == "" {
		return nil
	}

	switch resolvedVersionState {
	case "PENDING_DEVELOPER_RELEASE", "PENDING_APPLE_RELEASE":
		return []string{resolvedVersionState, "PENDING_RELEASE"}
	case "REPLACED_WITH_NEW_VERSION":
		return []string{resolvedVersionState, "REPLACED_WITH_NEW_INFO"}
	case "READY_FOR_SALE", "PREORDER_READY_FOR_SALE":
		return []string{resolvedVersionState, "READY_FOR_DISTRIBUTION"}
	default:
		return []string{resolvedVersionState}
	}
}

func matchesAppInfoState(candidateState string, acceptableStates []string) bool {
	resolvedCandidateState := strings.TrimSpace(candidateState)
	if resolvedCandidateState == "" {
		return false
	}

	for _, acceptableState := range acceptableStates {
		if strings.EqualFold(resolvedCandidateState, strings.TrimSpace(acceptableState)) {
			return true
		}
	}
	return false
}

func appInfoState(attributes AppInfoAttributes) string {
	for _, key := range []string{"state", "appStoreState"} {
		rawValue, exists := attributes[key]
		if !exists || rawValue == nil {
			continue
		}
		value, ok := rawValue.(string)
		if !ok {
			continue
		}
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// FormatAppInfoCandidates formats app info candidates for error messages.
func FormatAppInfoCandidates(candidates []AppInfoCandidate) string {
	if len(candidates) == 0 {
		return "none"
	}

	parts := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		state := candidate.State
		if state == "" {
			state = "unknown"
		}
		parts = append(parts, fmt.Sprintf("%s[state=%s]", candidate.ID, state))
	}
	return strings.Join(parts, ", ")
}
