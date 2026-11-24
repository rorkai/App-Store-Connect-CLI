package localizations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// localizationEditableVersionStates are the appVersionState values in which
// App Store Connect accepts edits to a version's description, keywords,
// whatsNew, supportUrl and marketingUrl.
var localizationEditableVersionStates = []string{
	"PREPARE_FOR_SUBMISSION",
	"DEVELOPER_REJECTED",
	"REJECTED",
	"METADATA_REJECTED",
	"INVALID_BINARY",
}

// localizationReleasedVersionStates are the states of a version that has
// been on the App Store. An app with none of them has never been released,
// and App Store Connect takes What's New only for updates.
var localizationReleasedVersionStates = []string{
	"READY_FOR_DISTRIBUTION",
	"READY_FOR_SALE",
	"REPLACED_WITH_NEW_VERSION",
	"REMOVED_FROM_SALE",
	"DEVELOPER_REMOVED_FROM_SALE",
	"PREORDER_READY_FOR_SALE",
}

type localizationVersionRow struct {
	id, versionString, platform, state string
}

// withVersionLocalizationConflictFacts adds what App Store Connect's 409 on
// PATCH /v1/appStoreVersionLocalizations/{id} does not say: the state of the
// version the localization belongs to, and which version the fields can be
// edited on instead. The reads are best effort; when one fails, patchErr is
// returned unchanged.
func withVersionLocalizationConflictFacts(ctx context.Context, d *shared.Deps, client *asc.Client, localizationID, versionID, locale string, fields []string, patchErr error) error {
	// Apple sends STATE_ERROR when the version's state rejects the field.
	apiErr, ok := errors.AsType[*asc.APIError](patchErr)
	if !ok || apiErr.StatusCode != http.StatusConflict || !strings.HasPrefix(apiErr.Code, "STATE_ERROR") {
		return patchErr
	}
	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()

	if versionID == "" {
		id, err := localizationVersionID(requestCtx, client, localizationID)
		if err != nil || id == "" {
			return patchErr
		}
		versionID = id
	}
	version, err := client.GetAppStoreVersion(requestCtx, versionID, asc.WithAppStoreVersionInclude([]string{"app"}))
	if err != nil {
		return patchErr
	}
	appID := relationshipID(version.Data.Relationships, "app")
	current := localizationVersionRow{
		id:            versionID,
		versionString: version.Data.Attributes.VersionString,
		platform:      string(version.Data.Attributes.Platform),
		state:         localizationVersionState(version.Data.Attributes),
	}
	if appID == "" || current.state == "" {
		return patchErr
	}
	rows, err := appVersionRows(requestCtx, client, appID, current.platform)
	if err != nil {
		return patchErr
	}

	if slices.Contains(localizationEditableVersionStates, current.state) {
		if !slices.Contains(fields, "whatsNew") || slices.ContainsFunc(rows, func(row localizationVersionRow) bool {
			return slices.Contains(localizationReleasedVersionStates, row.state)
		}) {
			return patchErr
		}
		return &shared.HintedError{Err: patchErr, Hint: fmt.Sprintf("Version %s (%s) is %s, but the app has no released version on %s yet, and App Store Connect takes whatsNew only for an update to a released app. Retry without --whats-new.",
			current.versionString, current.platform, current.state, current.platform)}
	}

	line := fmt.Sprintf("Version %s (%s, id %s) is %s. App Store Connect edits description, keywords, whatsNew, supportUrl and marketingUrl only while a version is %s; promotionalText can change in any state.",
		current.versionString, current.platform, current.id, current.state, strings.Join(localizationEditableVersionStates, ", "))
	editable := slices.IndexFunc(rows, func(row localizationVersionRow) bool {
		return slices.Contains(localizationEditableVersionStates, row.state)
	})
	if editable >= 0 {
		row := rows[editable]
		localeFlag := "--locale LOCALE"
		if locale != "" {
			localeFlag = fmt.Sprintf("--locale %q", locale)
		}
		line += fmt.Sprintf("\nVersion %s (%s) is editable: retry with --version %q %s.", row.versionString, row.state, row.id, localeFlag)
	} else {
		line += fmt.Sprintf("\nNo %s version is editable: create the next one with `asc versions create --app %q --version NEXT_VERSION --platform %s`, then retry on it.", current.platform, appID, current.platform)
	}
	return &shared.HintedError{Err: patchErr, Hint: line}
}

// localizationVersionState prefers appVersionState, which replaces the
// deprecated appStoreState.
func localizationVersionState(attrs asc.AppStoreVersionAttributes) string {
	if state := strings.TrimSpace(attrs.AppVersionState); state != "" {
		return state
	}
	return strings.TrimSpace(attrs.AppStoreState)
}

// localizationVersionID reads the version a localization belongs to.
func localizationVersionID(ctx context.Context, client *asc.Client, localizationID string) (string, error) {
	query := url.Values{}
	query.Set("include", "appStoreVersion")
	query.Set("fields[appStoreVersionLocalizations]", "appStoreVersion")
	body, err := client.RawRequestWithQuery(ctx, http.MethodGet, "/v1/appStoreVersionLocalizations/"+url.PathEscape(localizationID), query, nil)
	if err != nil {
		return "", err
	}
	var response struct {
		Data struct {
			Relationships json.RawMessage `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	return relationshipID(response.Data.Relationships, "appStoreVersion"), nil
}

func verifyVersionLocalizationOwner(ctx context.Context, d *shared.Deps, client *asc.Client, localizationID string) error {
	selectedAppID := strings.TrimSpace(d.ResolveAppID(""))
	if selectedAppID == "" {
		return nil
	}
	ownerAppID, err := shared.ResolveAppIDWithLookup(ctx, client, selectedAppID)
	if err != nil {
		return fmt.Errorf("resolve selected app: %w", err)
	}
	versionID, err := localizationVersionOwnershipID(ctx, client, localizationID)
	if err != nil {
		return fmt.Errorf("verify localization ownership: %w", err)
	}
	if _, err := shared.ResolveOwnedAppStoreVersionByID(ctx, client, ownerAppID, versionID, ""); err != nil {
		return fmt.Errorf("verify localization ownership: %w", err)
	}
	return nil
}

func localizationVersionOwnershipID(ctx context.Context, client *asc.Client, localizationID string) (string, error) {
	query := url.Values{}
	query.Set("include", "appStoreVersion")
	query.Set("fields[appStoreVersionLocalizations]", "appStoreVersion")
	body, err := client.RawRequestWithQuery(ctx, http.MethodGet, "/v1/appStoreVersionLocalizations/"+url.PathEscape(localizationID), query, nil)
	if err != nil {
		return "", err
	}
	var response struct {
		Data struct {
			Type          asc.ResourceType `json:"type"`
			ID            string           `json:"id"`
			Relationships json.RawMessage  `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	return typedLocalizationVersionID(response.Data.Type, response.Data.ID, response.Data.Relationships, localizationID)
}

func typedLocalizationVersionID(resourceType asc.ResourceType, resourceID string, relationships json.RawMessage, expectedID string) (string, error) {
	if resourceType != asc.ResourceTypeAppStoreVersionLocalizations {
		return "", fmt.Errorf("expected resource type %q, got %q for localization %q", asc.ResourceTypeAppStoreVersionLocalizations, resourceType, expectedID)
	}
	if resourceID != strings.TrimSpace(expectedID) {
		return "", fmt.Errorf("expected localization %q, got %q", expectedID, resourceID)
	}
	return typedRelationshipID(relationships, "appStoreVersion", asc.ResourceTypeAppStoreVersions)
}

func typedRelationshipID(relationships json.RawMessage, name string, expectedType asc.ResourceType) (string, error) {
	var parsed map[string]struct {
		Data *struct {
			Type asc.ResourceType `json:"type"`
			ID   string           `json:"id"`
		} `json:"data"`
	}
	if len(relationships) == 0 {
		return "", fmt.Errorf("relationship %q is missing", name)
	}
	if err := json.Unmarshal(relationships, &parsed); err != nil {
		return "", err
	}
	rel, ok := parsed[name]
	if !ok || rel.Data == nil {
		return "", fmt.Errorf("relationship %q is missing", name)
	}
	if rel.Data.Type != expectedType {
		return "", fmt.Errorf("expected %s relationship type %q, got %q", name, expectedType, rel.Data.Type)
	}
	if strings.TrimSpace(rel.Data.ID) == "" {
		return "", fmt.Errorf("relationship %q has no ID", name)
	}
	return strings.TrimSpace(rel.Data.ID), nil
}

func relationshipID(relationships json.RawMessage, name string) string {
	var parsed map[string]struct {
		Data *struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if len(relationships) == 0 || json.Unmarshal(relationships, &parsed) != nil {
		return ""
	}
	if rel, ok := parsed[name]; ok && rel.Data != nil {
		return strings.TrimSpace(rel.Data.ID)
	}
	return ""
}

// appVersionRows lists the app's versions on one platform, newest first as
// App Store Connect returns them.
func appVersionRows(ctx context.Context, client *asc.Client, appID, platform string) ([]localizationVersionRow, error) {
	opts := []asc.AppStoreVersionsOption{asc.Limit[asc.AppStoreVersionsOption](200)}
	if platform != "" {
		opts = append(opts, asc.WithAppStoreVersionsPlatforms([]string{platform}))
	}
	versions, err := client.GetAppStoreVersions(ctx, appID, opts...)
	if err != nil {
		return nil, err
	}
	rows := make([]localizationVersionRow, 0, len(versions.Data))
	for _, version := range versions.Data {
		rows = append(rows, localizationVersionRow{
			id:            strings.TrimSpace(version.ID),
			versionString: version.Attributes.VersionString,
			platform:      string(version.Attributes.Platform),
			state:         localizationVersionState(version.Attributes),
		})
	}
	return rows, nil
}
