package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var (
	appInfoPlanFields = []string{
		"name",
		"subtitle",
		"privacyPolicyUrl",
		"privacyChoicesUrl",
		"privacyPolicyText",
	}
	versionPlanFields = []string{
		"description",
		"keywords",
		"marketingUrl",
		"promotionalText",
		"supportUrl",
		"whatsNew",
	}
)

// PlanItem represents one deterministic metadata change entry.
type PlanItem struct {
	Key     string `json:"key"`
	Scope   string `json:"scope"`
	Locale  string `json:"locale"`
	Version string `json:"version,omitempty"`
	Field   string `json:"field"`
	Reason  string `json:"reason"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
}

// PlanAPICall is an estimated API call summary for the plan.
type PlanAPICall struct {
	Operation string `json:"operation"`
	Scope     string `json:"scope"`
	Count     int    `json:"count"`
}

// ApplyAction represents one executed mutation action.
type ApplyAction struct {
	Scope          string `json:"scope"`
	Locale         string `json:"locale"`
	Version        string `json:"version,omitempty"`
	Action         string `json:"action"`
	LocalizationID string `json:"localizationId,omitempty"`
}

// PushPlanResult is the push dry-run output artifact.
type PushPlanResult struct {
	AppID     string        `json:"appId"`
	AppInfoID string        `json:"appInfoId"`
	Version   string        `json:"version"`
	VersionID string        `json:"versionId"`
	Dir       string        `json:"dir"`
	DryRun    bool          `json:"dryRun"`
	Applied   bool          `json:"applied,omitempty"`
	Includes  []string      `json:"includes"`
	Adds      []PlanItem    `json:"adds"`
	Updates   []PlanItem    `json:"updates"`
	Deletes   []PlanItem    `json:"deletes"`
	APICalls  []PlanAPICall `json:"apiCalls,omitempty"`
	Actions   []ApplyAction `json:"actions,omitempty"`
}

type scopeCallCounts struct {
	create int
	update int
	delete int
}

type localMetadataBundle struct {
	appInfo        map[string]appInfoLocalPatch
	version        map[string]versionLocalPatch
	defaultAppInfo *appInfoLocalPatch
	defaultVersion *versionLocalPatch
}

type localPlanFields struct {
	setFields map[string]string
}

type appInfoLocalPatch struct {
	localization AppInfoLocalization
	setFields    map[string]string
}

type versionLocalPatch struct {
	localization       VersionLocalization
	createLocalization VersionLocalization
	setFields          map[string]string
}

// MetadataPushCommand returns the metadata push subcommand.
func MetadataPushCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata push", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appInfoID := fs.String("app-info", "", "App Info ID (optional override)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	dir := fs.String("dir", "", "Metadata root directory (required)")
	include := fs.String("include", includeLocalizations, "Included metadata scopes (comma-separated)")
	dryRun := fs.Bool("dry-run", false, "Preview changes without mutating App Store Connect")
	allowDeletes := fs.Bool("allow-deletes", false, "Allow destructive delete operations when applying changes (disables default locale fallback for missing locales)")
	confirm := fs.Bool("confirm", false, "Confirm destructive operations (required with --allow-deletes)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "push",
		ShortUsage: "asc metadata push --app \"APP_ID\" --version \"1.2.3\" --dir \"./metadata\" [--app-info \"APP_INFO_ID\"] [--dry-run]",
		ShortHelp:  "Push metadata changes from canonical files.",
		LongHelp: `Push metadata changes from canonical files.

Examples:
  asc metadata push --app "APP_ID" --version "1.2.3" --dir "./metadata" --dry-run
  asc metadata push --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata" --dry-run
  asc metadata push --app "APP_ID" --app-info "APP_INFO_ID" --version "1.2.3" --platform IOS --dir "./metadata" --dry-run
  asc metadata push --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata push --app "APP_ID" --version "1.2.3" --dir "./metadata" --allow-deletes --confirm

Notes:
  - default.json fallback is applied only when --allow-deletes is not set.
  - with --allow-deletes, remote locales missing locally are planned as deletes.
  - omitted fields are treated as no-op; they do not imply deletion.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata push does not accept positional arguments")
			}
			result, err := ExecutePush(ctx, PushExecutionOptions{
				AppID:        *appID,
				AppInfoID:    *appInfoID,
				Version:      *version,
				Platform:     *platform,
				Dir:          *dir,
				Include:      *include,
				DryRun:       *dryRun,
				AllowDeletes: *allowDeletes,
				Confirm:      *confirm,
			})
			if err != nil {
				return err
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return printPushPlanTable(result) },
				func() error { return printPushPlanMarkdown(result) },
			)
		},
	}
}

func loadLocalMetadata(dir, version string) (localMetadataBundle, error) {
	localAppInfo := make(map[string]appInfoLocalPatch)
	localVersion := make(map[string]versionLocalPatch)
	var defaultAppInfo *appInfoLocalPatch
	var defaultVersion *versionLocalPatch
	filesSeen := 0

	appInfoDir := filepath.Join(dir, appInfoDirName)
	appInfoEntries, err := os.ReadDir(appInfoDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return localMetadataBundle{}, fmt.Errorf("metadata push: failed to read %s: %w", appInfoDir, err)
	}
	if err == nil {
		seenAppInfoLocales := make(map[string]string)
		for _, entry := range appInfoEntries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			locale := strings.TrimSuffix(entry.Name(), ".json")
			resolvedLocale, localeErr := validateLocale(locale)
			if localeErr != nil {
				return localMetadataBundle{}, shared.UsageErrorf("invalid app-info localization file %q: %v", entry.Name(), localeErr)
			}
			if err := recordCanonicalLocaleFile(seenAppInfoLocales, resolvedLocale, entry.Name()); err != nil {
				return localMetadataBundle{}, shared.UsageError(err.Error())
			}
			filePath := filepath.Join(appInfoDir, entry.Name())
			patch, readErr := readAppInfoLocalizationPatchFromFile(filePath)
			if readErr != nil {
				return localMetadataBundle{}, shared.UsageErrorf("invalid metadata schema in %s: %v", filePath, readErr)
			}
			if resolvedLocale == DefaultLocale {
				value := patch
				defaultAppInfo = &value
				filesSeen++
				continue
			}
			localAppInfo[resolvedLocale] = patch
			filesSeen++
		}
	}

	resolvedVersion, err := validatePathSegment("version", version)
	if err != nil {
		return localMetadataBundle{}, shared.UsageError(err.Error())
	}
	versionDir := filepath.Join(dir, versionDirName, resolvedVersion)
	versionEntries, err := os.ReadDir(versionDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return localMetadataBundle{}, fmt.Errorf("metadata push: failed to read %s: %w", versionDir, err)
	}
	if err == nil {
		seenVersionLocales := make(map[string]string)
		for _, entry := range versionEntries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			locale := strings.TrimSuffix(entry.Name(), ".json")
			resolvedLocale, localeErr := validateLocale(locale)
			if localeErr != nil {
				return localMetadataBundle{}, shared.UsageErrorf("invalid version localization file %q: %v", entry.Name(), localeErr)
			}
			if err := recordCanonicalLocaleFile(seenVersionLocales, resolvedLocale, entry.Name()); err != nil {
				return localMetadataBundle{}, shared.UsageError(err.Error())
			}
			filePath := filepath.Join(versionDir, entry.Name())
			patch, readErr := readVersionLocalizationPatchFromFile(filePath)
			if readErr != nil {
				return localMetadataBundle{}, shared.UsageErrorf("invalid metadata schema in %s: %v", filePath, readErr)
			}
			if resolvedLocale == DefaultLocale {
				value := patch
				defaultVersion = &value
				filesSeen++
				continue
			}
			localVersion[resolvedLocale] = patch
			filesSeen++
		}
	}

	if filesSeen == 0 {
		return localMetadataBundle{}, shared.UsageError("no metadata .json files found")
	}
	return localMetadataBundle{
		appInfo:        localAppInfo,
		version:        localVersion,
		defaultAppInfo: defaultAppInfo,
		defaultVersion: defaultVersion,
	}, nil
}

type exampleBuilderFunc func(appID, version, platform, dir, appInfoID string) string

func resolveMetadataPushAppInfoID(
	ctx context.Context,
	client *asc.Client,
	appID string,
	appInfoID string,
	version string,
	platform string,
	dir string,
	versionState string,
) (string, error) {
	return resolveMetadataAppInfoID(ctx, client, appID, appInfoID, version, platform, dir, versionState, func(aid, v, p, d, infoID string) string {
		return buildMetadataAppInfoExample("push", aid, v, p, d, infoID) + " --dry-run"
	})
}

func resolveMetadataAppInfoID(
	ctx context.Context,
	client *asc.Client,
	appID string,
	appInfoID string,
	version string,
	platform string,
	dir string,
	versionState string,
	buildExample exampleBuilderFunc,
) (string, error) {
	if appInfoID != "" {
		return appInfoID, nil
	}

	resp, err := client.GetAppInfos(ctx, appID)
	if err != nil {
		return "", err
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("no app info found for app %q", appID)
	}
	if len(resp.Data) == 1 {
		return strings.TrimSpace(resp.Data[0].ID), nil
	}

	candidates := asc.AppInfoCandidates(resp.Data)

	if resolvedID, ok := asc.AutoResolveAppInfoIDByVersionState(candidates, versionState); ok {
		return resolvedID, nil
	}

	exampleAppInfoID := "<APP_INFO_ID>"
	for _, candidate := range candidates {
		if candidate.ID != "" {
			exampleAppInfoID = candidate.ID
			break
		}
	}
	exampleCommand := buildExample(appID, version, platform, dir, exampleAppInfoID)
	return "", shared.UsageErrorf(
		"multiple app infos found for app %q (%s). Run `asc apps info list --app %q` to inspect candidates, then re-run with --app-info. Example: %s",
		appID,
		asc.FormatAppInfoCandidates(candidates),
		appID,
		exampleCommand,
	)
}

func buildMetadataAppInfoExample(command, appID, version, platform, dir, appInfoID string) string {
	parts := []string{
		fmt.Sprintf("asc metadata %s", command),
		fmt.Sprintf(`--app %q`, appID),
		fmt.Sprintf(`--version %q`, version),
	}
	if strings.TrimSpace(platform) != "" {
		parts = append(parts, fmt.Sprintf("--platform %s", strings.TrimSpace(platform)))
	}

	dirValue := strings.TrimSpace(dir)
	if dirValue == "" {
		dirValue = "./metadata"
	}
	parts = append(parts,
		fmt.Sprintf(`--dir %q`, dirValue),
		fmt.Sprintf(`--app-info %q`, appInfoID),
	)
	return strings.Join(parts, " ")
}

func readAppInfoLocalizationPatchFromFile(path string) (appInfoLocalPatch, error) {
	data, err := readFileNoFollow(path)
	if err != nil {
		return appInfoLocalPatch{}, err
	}

	var raw map[string]json.RawMessage
	if err := decodeStrictJSON(data, &raw); err != nil {
		return appInfoLocalPatch{}, err
	}

	setFields := make(map[string]string)
	loc := AppInfoLocalization{}
	for key, rawValue := range raw {
		canonicalKey, err := canonicalStringFieldPatchKey(key, appInfoPlanFields)
		if err != nil {
			return appInfoLocalPatch{}, err
		}
		if _, exists := setFields[canonicalKey]; exists {
			return appInfoLocalPatch{}, fmt.Errorf("json: duplicate field %q", canonicalKey)
		}
		value, err := decodeStringFieldPatch(canonicalKey, rawValue)
		if err != nil {
			return appInfoLocalPatch{}, err
		}
		setFields[canonicalKey] = value
		switch canonicalKey {
		case "name":
			loc.Name = value
		case "subtitle":
			loc.Subtitle = value
		case "privacyPolicyUrl":
			loc.PrivacyPolicyURL = value
		case "privacyChoicesUrl":
			loc.PrivacyChoicesURL = value
		case "privacyPolicyText":
			loc.PrivacyPolicyText = value
		}
	}

	if len(setFields) == 0 {
		return appInfoLocalPatch{}, fmt.Errorf("at least one app-info field is required")
	}

	return appInfoLocalPatch{
		localization: NormalizeAppInfoLocalization(loc),
		setFields:    setFields,
	}, nil
}

func readVersionLocalizationPatchFromFile(path string) (versionLocalPatch, error) {
	data, err := readFileNoFollow(path)
	if err != nil {
		return versionLocalPatch{}, err
	}

	var raw map[string]json.RawMessage
	if err := decodeStrictJSON(data, &raw); err != nil {
		return versionLocalPatch{}, err
	}

	setFields := make(map[string]string)
	loc := VersionLocalization{}
	for key, rawValue := range raw {
		canonicalKey, err := canonicalStringFieldPatchKey(key, versionPlanFields)
		if err != nil {
			return versionLocalPatch{}, err
		}
		if _, exists := setFields[canonicalKey]; exists {
			return versionLocalPatch{}, fmt.Errorf("json: duplicate field %q", canonicalKey)
		}
		value, err := decodeStringFieldPatch(canonicalKey, rawValue)
		if err != nil {
			return versionLocalPatch{}, err
		}
		setFields[canonicalKey] = value
		switch canonicalKey {
		case "description":
			loc.Description = value
		case "keywords":
			loc.Keywords = value
		case "marketingUrl":
			loc.MarketingURL = value
		case "promotionalText":
			loc.PromotionalText = value
		case "supportUrl":
			loc.SupportURL = value
		case "whatsNew":
			loc.WhatsNew = value
		}
	}

	if len(setFields) == 0 {
		return versionLocalPatch{}, fmt.Errorf("at least one version metadata field is required")
	}

	return versionLocalPatch{
		localization: NormalizeVersionLocalization(loc),
		setFields:    setFields,
	}, nil
}

func canonicalStringFieldPatchKey(field string, allowed []string) (string, error) {
	for _, key := range allowed {
		if field == key {
			return key, nil
		}
	}
	for _, key := range allowed {
		if strings.EqualFold(field, key) {
			return key, nil
		}
	}
	return "", fmt.Errorf("json: unknown field %q", field)
}

func decodeStringFieldPatch(field string, raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "__ASC_DELETE__" {
		return "", fmt.Errorf("field %q uses unsupported clear token __ASC_DELETE__; omit the key to keep the remote value", field)
	}
	if trimmed == "" {
		return "", fmt.Errorf("field %q cannot be empty; omit the key to leave the remote value unchanged", field)
	}
	return trimmed, nil
}

func applyDefaultAppInfoFallback(
	explicit map[string]appInfoLocalPatch,
	defaultValue *appInfoLocalPatch,
	remote map[string]AppInfoLocalization,
	allowDeletes bool,
) map[string]appInfoLocalPatch {
	result := make(map[string]appInfoLocalPatch, len(explicit))
	for locale, value := range explicit {
		result[locale] = cloneAppInfoLocalPatch(value)
	}
	if defaultValue == nil || allowDeletes {
		return result
	}
	for locale := range remote {
		if locale == DefaultLocale {
			continue
		}
		if _, ok := result[locale]; ok {
			continue
		}
		result[locale] = cloneAppInfoLocalPatch(*defaultValue)
	}
	return result
}

func applyDefaultVersionFallback(
	explicit map[string]versionLocalPatch,
	defaultValue *versionLocalPatch,
	remote map[string]VersionLocalization,
	allowDeletes bool,
) map[string]versionLocalPatch {
	result := make(map[string]versionLocalPatch, len(explicit))
	for locale, value := range explicit {
		result[locale] = cloneVersionLocalPatch(value)
	}
	if defaultValue == nil || allowDeletes {
		return result
	}
	for locale := range remote {
		if locale == DefaultLocale {
			continue
		}
		if _, ok := result[locale]; ok {
			continue
		}
		result[locale] = cloneVersionLocalPatch(*defaultValue)
	}
	return result
}

func cloneAppInfoLocalPatch(patch appInfoLocalPatch) appInfoLocalPatch {
	return appInfoLocalPatch{
		localization: patch.localization,
		setFields:    cloneStringMap(patch.setFields),
	}
}

func cloneVersionLocalPatch(patch versionLocalPatch) versionLocalPatch {
	return versionLocalPatch{
		localization:       patch.localization,
		createLocalization: patch.createLocalization,
		setFields:          cloneStringMap(patch.setFields),
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func appInfoToPlanFields(values map[string]appInfoLocalPatch) map[string]localPlanFields {
	result := make(map[string]localPlanFields, len(values))
	for locale, value := range values {
		result[locale] = localPlanFields{
			setFields: cloneStringMap(value.setFields),
		}
	}
	return result
}

func versionToPlanFields(values map[string]versionLocalPatch) map[string]localPlanFields {
	result := make(map[string]localPlanFields, len(values))
	for locale, value := range values {
		result[locale] = localPlanFields{
			setFields: cloneStringMap(value.setFields),
		}
	}
	return result
}

type remoteLocalizationState struct {
	id     string
	fields map[string]string
}

func applyMetadataPlan(
	ctx context.Context,
	client *asc.Client,
	appInfoID string,
	versionID string,
	version string,
	localAppInfo map[string]appInfoLocalPatch,
	localVersion map[string]versionLocalPatch,
	remoteAppInfoItems []asc.Resource[asc.AppInfoLocalizationAttributes],
	remoteVersionItems []asc.Resource[asc.AppStoreVersionLocalizationAttributes],
	allowDeletes bool,
) ([]ApplyAction, error) {
	actions := make([]ApplyAction, 0)

	appInfoActions, err := applyAppInfoChanges(ctx, client, appInfoID, localAppInfo, remoteAppInfoItems, allowDeletes)
	if err != nil {
		return nil, err
	}
	actions = append(actions, appInfoActions...)

	versionActions, err := applyVersionChanges(ctx, client, versionID, version, localVersion, remoteVersionItems, allowDeletes)
	if err != nil {
		return nil, err
	}
	actions = append(actions, versionActions...)

	return actions, nil
}

func applyAppInfoChanges(
	ctx context.Context,
	client *asc.Client,
	appInfoID string,
	local map[string]appInfoLocalPatch,
	remoteItems []asc.Resource[asc.AppInfoLocalizationAttributes],
	allowDeletes bool,
) ([]ApplyAction, error) {
	remoteByLocale := make(map[string]remoteLocalizationState, len(remoteItems))
	for _, item := range remoteItems {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		remoteByLocale[locale] = remoteLocalizationState{
			id: item.ID,
			fields: appInfoFields(AppInfoLocalization{
				Name:              item.Attributes.Name,
				Subtitle:          item.Attributes.Subtitle,
				PrivacyPolicyURL:  item.Attributes.PrivacyPolicyURL,
				PrivacyChoicesURL: item.Attributes.PrivacyChoicesURL,
				PrivacyPolicyText: item.Attributes.PrivacyPolicyText,
			}),
		}
	}

	locales := sortedLocaleUnion(local, remoteByLocale)

	actions := make([]ApplyAction, 0)
	for _, locale := range locales {
		localPatch, localExists := local[locale]
		remoteState, remoteExists := remoteByLocale[locale]

		if !localExists && remoteExists {
			if !allowDeletes {
				return nil, fmt.Errorf("delete operations require --allow-deletes")
			}
			if err := client.DeleteAppInfoLocalization(ctx, remoteState.id); err != nil {
				return nil, fmt.Errorf("delete app-info localization %s: %w", locale, err)
			}
			actions = append(actions, ApplyAction{
				Scope:          appInfoDirName,
				Locale:         locale,
				Action:         "delete",
				LocalizationID: remoteState.id,
			})
			continue
		}
		if !localExists {
			continue
		}

		remoteFields := cloneStringMap(remoteState.fields)
		adds, updates := countIntentChanges(appInfoPlanFields, localPatch.setFields, remoteFields)
		if adds == 0 && updates == 0 {
			continue
		}

		switch {
		case !remoteExists:
			if strings.TrimSpace(localPatch.localization.Name) == "" {
				return nil, fmt.Errorf("cannot create app-info localization %q without name", locale)
			}
			resp, err := client.CreateAppInfoLocalization(ctx, appInfoID, appInfoAttributes(locale, localPatch.localization, true))
			if err != nil {
				return nil, fmt.Errorf("create app-info localization %s: %w", locale, err)
			}
			actions = append(actions, ApplyAction{
				Scope:          appInfoDirName,
				Locale:         locale,
				Action:         "create",
				LocalizationID: resp.Data.ID,
			})
		case remoteExists:
			resp, err := client.UpdateAppInfoLocalization(ctx, remoteState.id, appInfoAttributes(locale, localPatch.localization, false))
			if err != nil {
				return nil, fmt.Errorf("update app-info localization %s: %w", locale, err)
			}
			actions = append(actions, ApplyAction{
				Scope:          appInfoDirName,
				Locale:         locale,
				Action:         "update",
				LocalizationID: resp.Data.ID,
			})
		}
	}

	return actions, nil
}

func applyVersionChanges(
	ctx context.Context,
	client *asc.Client,
	versionID string,
	version string,
	local map[string]versionLocalPatch,
	remoteItems []asc.Resource[asc.AppStoreVersionLocalizationAttributes],
	allowDeletes bool,
) ([]ApplyAction, error) {
	remoteByLocale := make(map[string]remoteLocalizationState, len(remoteItems))
	for _, item := range remoteItems {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		remoteByLocale[locale] = remoteLocalizationState{
			id: item.ID,
			fields: versionFields(VersionLocalization{
				Description:     item.Attributes.Description,
				Keywords:        item.Attributes.Keywords,
				MarketingURL:    item.Attributes.MarketingURL,
				PromotionalText: item.Attributes.PromotionalText,
				SupportURL:      item.Attributes.SupportURL,
				WhatsNew:        item.Attributes.WhatsNew,
			}),
		}
	}

	locales := sortedLocaleUnion(local, remoteByLocale)

	actions := make([]ApplyAction, 0)
	for _, locale := range locales {
		localPatch, localExists := local[locale]
		remoteState, remoteExists := remoteByLocale[locale]

		if !localExists && remoteExists {
			if !allowDeletes {
				return nil, fmt.Errorf("delete operations require --allow-deletes")
			}
			if err := client.DeleteAppStoreVersionLocalization(ctx, remoteState.id); err != nil {
				return nil, fmt.Errorf("delete version localization %s: %w", locale, err)
			}
			actions = append(actions, ApplyAction{
				Scope:          versionDirName,
				Locale:         locale,
				Version:        version,
				Action:         "delete",
				LocalizationID: remoteState.id,
			})
			continue
		}
		if !localExists {
			continue
		}

		remoteFields := cloneStringMap(remoteState.fields)
		adds, updates := countIntentChanges(versionPlanFields, localPatch.setFields, remoteFields)
		if adds == 0 && updates == 0 {
			continue
		}

		switch {
		case !remoteExists:
			createLoc := localPatch.localization
			if hasVersionContent(localPatch.createLocalization) {
				createLoc = localPatch.createLocalization
			}
			resp, err := client.CreateAppStoreVersionLocalization(ctx, versionID, versionAttributes(locale, createLoc, true))
			if err != nil {
				return nil, fmt.Errorf("create version localization %s: %w", locale, err)
			}
			actions = append(actions, ApplyAction{
				Scope:          versionDirName,
				Locale:         locale,
				Version:        version,
				Action:         "create",
				LocalizationID: resp.Data.ID,
			})
		case remoteExists:
			resp, err := client.UpdateAppStoreVersionLocalization(ctx, remoteState.id, versionAttributes(locale, localPatch.localization, false))
			if err != nil {
				return nil, fmt.Errorf("update version localization %s: %w", locale, err)
			}
			actions = append(actions, ApplyAction{
				Scope:          versionDirName,
				Locale:         locale,
				Version:        version,
				Action:         "update",
				LocalizationID: resp.Data.ID,
			})
		}
	}

	return actions, nil
}

func countIntentChanges(fields []string, localSet map[string]string, remote map[string]string) (int, int) {
	adds := 0
	updates := 0
	for _, field := range fields {
		localValue, localHasField := localSet[field]
		if !localHasField {
			continue
		}
		remoteValue, remoteHasField := remote[field]
		switch {
		case !remoteHasField:
			adds++
		case remoteValue != localValue:
			updates++
		}
	}
	return adds, updates
}

func sortedLocaleUnion[T any](local map[string]T, remote map[string]remoteLocalizationState) []string {
	locales := make([]string, 0, len(local)+len(remote))
	localeSet := make(map[string]struct{})
	for locale := range local {
		localeSet[locale] = struct{}{}
	}
	for locale := range remote {
		localeSet[locale] = struct{}{}
	}
	for locale := range localeSet {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	return locales
}

func appInfoAttributes(locale string, loc AppInfoLocalization, includeLocale bool) asc.AppInfoLocalizationAttributes {
	normalized := NormalizeAppInfoLocalization(loc)
	attrs := asc.AppInfoLocalizationAttributes{
		Name:              normalized.Name,
		Subtitle:          normalized.Subtitle,
		PrivacyPolicyURL:  normalized.PrivacyPolicyURL,
		PrivacyChoicesURL: normalized.PrivacyChoicesURL,
		PrivacyPolicyText: normalized.PrivacyPolicyText,
	}
	if includeLocale {
		attrs.Locale = locale
	}
	return attrs
}

func versionAttributes(locale string, loc VersionLocalization, includeLocale bool) asc.AppStoreVersionLocalizationAttributes {
	normalized := NormalizeVersionLocalization(loc)
	attrs := asc.AppStoreVersionLocalizationAttributes{
		Description:     normalized.Description,
		Keywords:        normalized.Keywords,
		MarketingURL:    normalized.MarketingURL,
		PromotionalText: normalized.PromotionalText,
		SupportURL:      normalized.SupportURL,
		WhatsNew:        normalized.WhatsNew,
	}
	if includeLocale {
		attrs.Locale = locale
	}
	return attrs
}

func appInfoToFieldMap(values map[string]AppInfoLocalization) map[string]map[string]string {
	result := make(map[string]map[string]string, len(values))
	for locale, value := range values {
		result[locale] = appInfoFields(value)
	}
	return result
}

func versionToFieldMap(values map[string]VersionLocalization) map[string]map[string]string {
	result := make(map[string]map[string]string, len(values))
	for locale, value := range values {
		result[locale] = versionFields(value)
	}
	return result
}

func appInfoFields(value AppInfoLocalization) map[string]string {
	fields := make(map[string]string)
	normalized := NormalizeAppInfoLocalization(value)
	if normalized.Name != "" {
		fields["name"] = normalized.Name
	}
	if normalized.Subtitle != "" {
		fields["subtitle"] = normalized.Subtitle
	}
	if normalized.PrivacyPolicyURL != "" {
		fields["privacyPolicyUrl"] = normalized.PrivacyPolicyURL
	}
	if normalized.PrivacyChoicesURL != "" {
		fields["privacyChoicesUrl"] = normalized.PrivacyChoicesURL
	}
	if normalized.PrivacyPolicyText != "" {
		fields["privacyPolicyText"] = normalized.PrivacyPolicyText
	}
	return fields
}

func versionFields(value VersionLocalization) map[string]string {
	fields := make(map[string]string)
	normalized := NormalizeVersionLocalization(value)
	if normalized.Description != "" {
		fields["description"] = normalized.Description
	}
	if normalized.Keywords != "" {
		fields["keywords"] = normalized.Keywords
	}
	if normalized.MarketingURL != "" {
		fields["marketingUrl"] = normalized.MarketingURL
	}
	if normalized.PromotionalText != "" {
		fields["promotionalText"] = normalized.PromotionalText
	}
	if normalized.SupportURL != "" {
		fields["supportUrl"] = normalized.SupportURL
	}
	if normalized.WhatsNew != "" {
		fields["whatsNew"] = normalized.WhatsNew
	}
	return fields
}

func buildScopePlan(
	scope string,
	version string,
	fields []string,
	local map[string]localPlanFields,
	remote map[string]map[string]string,
) ([]PlanItem, []PlanItem, []PlanItem, scopeCallCounts) {
	localesMap := make(map[string]struct{})
	for locale := range local {
		localesMap[locale] = struct{}{}
	}
	for locale := range remote {
		localesMap[locale] = struct{}{}
	}

	locales := make([]string, 0, len(localesMap))
	for locale := range localesMap {
		locales = append(locales, locale)
	}
	sort.Strings(locales)

	adds := make([]PlanItem, 0)
	updates := make([]PlanItem, 0)
	deletes := make([]PlanItem, 0)
	callCounts := scopeCallCounts{}

	for _, locale := range locales {
		localValues, localExists := local[locale]
		remoteValues, remoteExists := remote[locale]

		if !localExists && remoteExists {
			localeChanged := false
			for _, field := range fields {
				remoteValue, remoteHasField := remoteValues[field]
				if !remoteHasField {
					continue
				}
				deletes = append(deletes, PlanItem{
					Key:     buildPlanKey(scope, version, locale, field),
					Scope:   scope,
					Locale:  locale,
					Version: version,
					Field:   field,
					Reason:  "localization missing locally",
					From:    remoteValue,
				})
				localeChanged = true
			}
			if localeChanged {
				callCounts.delete++
			}
			continue
		}

		localeChanged := false

		for _, field := range fields {
			localValue, localHasField := localValues.setFields[field]
			remoteValue, remoteHasField := remoteValues[field]
			if !localHasField {
				continue
			}

			itemKey := buildPlanKey(scope, version, locale, field)
			switch {
			case !remoteHasField && localHasField:
				adds = append(adds, PlanItem{
					Key:     itemKey,
					Scope:   scope,
					Locale:  locale,
					Version: version,
					Field:   field,
					Reason:  "field exists locally but not remotely",
					To:      localValue,
				})
				localeChanged = true
			case remoteHasField && localHasField && remoteValue != localValue:
				updates = append(updates, PlanItem{
					Key:     itemKey,
					Scope:   scope,
					Locale:  locale,
					Version: version,
					Field:   field,
					Reason:  "field value differs",
					From:    remoteValue,
					To:      localValue,
				})
				localeChanged = true
			}
		}

		if !localeChanged {
			continue
		}
		switch {
		case localExists && !remoteExists:
			callCounts.create++
		default:
			callCounts.update++
		}
	}

	return adds, updates, deletes, callCounts
}

func buildPlanKey(scope, version, locale, field string) string {
	if scope == appInfoDirName {
		return fmt.Sprintf("%s:%s:%s", scope, locale, field)
	}
	return fmt.Sprintf("%s:%s:%s:%s", scope, version, locale, field)
}

func buildAPICallSummary(appInfoCounts, versionCounts scopeCallCounts) []PlanAPICall {
	summary := make([]PlanAPICall, 0, 6)
	appendCalls := func(scope string, counts scopeCallCounts) {
		if counts.create > 0 {
			summary = append(summary, PlanAPICall{
				Operation: "create_localization",
				Scope:     scope,
				Count:     counts.create,
			})
		}
		if counts.update > 0 {
			summary = append(summary, PlanAPICall{
				Operation: "update_localization",
				Scope:     scope,
				Count:     counts.update,
			})
		}
		if counts.delete > 0 {
			summary = append(summary, PlanAPICall{
				Operation: "delete_localization",
				Scope:     scope,
				Count:     counts.delete,
			})
		}
	}
	appendCalls(appInfoDirName, appInfoCounts)
	appendCalls(versionDirName, versionCounts)

	sort.Slice(summary, func(i, j int) bool {
		if summary[i].Scope == summary[j].Scope {
			return summary[i].Operation < summary[j].Operation
		}
		return summary[i].Scope < summary[j].Scope
	})
	return summary
}

func sortPlanItems(items []PlanItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})
}

func printPushPlanTable(result PushPlanResult) error {
	fmt.Printf("App ID: %s\n", result.AppID)
	fmt.Printf("Version: %s\n", result.Version)
	fmt.Printf("Dir: %s\n", result.Dir)
	fmt.Printf("Dry Run: %t\n\n", result.DryRun)
	if result.Applied {
		fmt.Printf("Applied: %t\n\n", result.Applied)
	}

	headers := []string{"change", "key", "scope", "locale", "version", "field", "reason", "from", "to"}
	rows := buildPlanRows(result)
	asc.RenderTable(headers, rows)

	if len(result.APICalls) > 0 {
		fmt.Println()
		asc.RenderTable([]string{"operation", "scope", "count"}, buildAPICallRows(result.APICalls))
	}
	if len(result.Actions) > 0 {
		fmt.Println()
		asc.RenderTable([]string{"scope", "locale", "version", "action", "localizationId"}, buildApplyActionRows(result.Actions))
	}
	return nil
}

func printPushPlanMarkdown(result PushPlanResult) error {
	fmt.Printf("**App ID:** %s\n\n", result.AppID)
	fmt.Printf("**Version:** %s\n\n", result.Version)
	fmt.Printf("**Dir:** %s\n\n", result.Dir)
	fmt.Printf("**Dry Run:** %t\n\n", result.DryRun)
	if result.Applied {
		fmt.Printf("**Applied:** %t\n\n", result.Applied)
	}

	headers := []string{"change", "key", "scope", "locale", "version", "field", "reason", "from", "to"}
	rows := buildPlanRows(result)
	asc.RenderMarkdown(headers, rows)

	if len(result.APICalls) > 0 {
		fmt.Println()
		asc.RenderMarkdown([]string{"operation", "scope", "count"}, buildAPICallRows(result.APICalls))
	}
	if len(result.Actions) > 0 {
		fmt.Println()
		asc.RenderMarkdown([]string{"scope", "locale", "version", "action", "localizationId"}, buildApplyActionRows(result.Actions))
	}
	return nil
}

func buildPlanRows(result PushPlanResult) [][]string {
	rows := make([][]string, 0, len(result.Adds)+len(result.Updates)+len(result.Deletes))
	for _, item := range result.Adds {
		rows = append(rows, []string{
			"add",
			item.Key,
			item.Scope,
			item.Locale,
			item.Version,
			item.Field,
			item.Reason,
			"",
			sanitizePlanCell(item.To),
		})
	}
	for _, item := range result.Updates {
		rows = append(rows, []string{
			"update",
			item.Key,
			item.Scope,
			item.Locale,
			item.Version,
			item.Field,
			item.Reason,
			sanitizePlanCell(item.From),
			sanitizePlanCell(item.To),
		})
	}
	for _, item := range result.Deletes {
		rows = append(rows, []string{
			"delete",
			item.Key,
			item.Scope,
			item.Locale,
			item.Version,
			item.Field,
			item.Reason,
			sanitizePlanCell(item.From),
			"",
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"none", "", "", "", "", "", "no changes", "", ""})
	}
	return rows
}

func buildAPICallRows(calls []PlanAPICall) [][]string {
	rows := make([][]string, 0, len(calls))
	for _, call := range calls {
		rows = append(rows, []string{call.Operation, call.Scope, fmt.Sprintf("%d", call.Count)})
	}
	return rows
}

func buildApplyActionRows(actions []ApplyAction) [][]string {
	rows := make([][]string, 0, len(actions))
	for _, action := range actions {
		rows = append(rows, []string{
			action.Scope,
			action.Locale,
			action.Version,
			action.Action,
			action.LocalizationID,
		})
	}
	return rows
}

func sanitizePlanCell(value string) string {
	normalized := strings.ReplaceAll(value, "\n", "\\n")
	const maxLen = 80
	if len([]rune(normalized)) <= maxLen {
		return normalized
	}
	runes := []rune(normalized)
	return string(runes[:77]) + "..."
}
