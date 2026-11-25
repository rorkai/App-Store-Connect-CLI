package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/urlsanitize"
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

// Statuses recorded on ApplyAction. metadataActionStatusSkipped is additive:
// it is recorded when --if-exists skip resolved a create conflict, or when an
// update had nothing left to send after App Store Connect refused whatsNew,
// and it reuses the shared idempotent-write vocabulary from
// asc.IdempotentWriteActionSkipped.
const (
	metadataActionStatusSucceeded = "succeeded"
	metadataActionStatusFailed    = "failed"
	metadataActionStatusSkipped   = asc.IdempotentWriteActionSkipped
)

// ApplyAction represents one executed mutation action.
//
// AlreadyExists and IfExists are additive --if-exists fields. They are omitted
// on every path that does not hit an existence conflict, so the default
// receipt is unchanged.
type ApplyAction struct {
	Scope          string            `json:"scope"`
	Locale         string            `json:"locale"`
	Version        string            `json:"version,omitempty"`
	Action         string            `json:"action"`
	Status         string            `json:"status,omitempty"`
	LocalizationID string            `json:"localizationId,omitempty"`
	Error          string            `json:"error,omitempty"`
	DesiredFields  map[string]string `json:"desiredFields,omitempty"`
	AlreadyExists  bool              `json:"alreadyExists,omitempty"`
	IfExists       string            `json:"ifExists,omitempty"`
	// SkippedFields lists fields App Store Connect refused in this state and
	// push left out of the write (whatsNew on an app's first version).
	SkippedFields       []string `json:"skippedFields,omitempty"`
	reconciledDuplicate bool
	convertedFromCreate bool
}

// PushPlanResult is the push dry-run output artifact.
type PushPlanResult struct {
	AssetResults         []asc.StoreAssetResult `json:"assetResults,omitempty"`
	AppID                string                 `json:"appId"`
	AppInfoID            string                 `json:"appInfoId"`
	Version              string                 `json:"version"`
	VersionID            string                 `json:"versionId"`
	Dir                  string                 `json:"dir"`
	DryRun               bool                   `json:"dryRun"`
	Applied              bool                   `json:"applied,omitempty"`
	Includes             []string               `json:"includes"`
	Adds                 []PlanItem             `json:"adds"`
	Updates              []PlanItem             `json:"updates"`
	Deletes              []PlanItem             `json:"deletes"`
	APICalls             []PlanAPICall          `json:"apiCalls,omitempty"`
	Actions              []ApplyAction          `json:"actions,omitempty"`
	Total                int                    `json:"total,omitempty"`
	Succeeded            int                    `json:"succeeded,omitempty"`
	Skipped              int                    `json:"skipped,omitempty"`
	Failed               int                    `json:"failed,omitempty"`
	FailureArtifactPath  string                 `json:"failureArtifactPath,omitempty"`
	FailureArtifactError string                 `json:"failureArtifactError,omitempty"`
}

type metadataPushFailureArtifact struct {
	SchemaVersion int           `json:"schemaVersion"`
	Command       string        `json:"command"`
	AppID         string        `json:"appId"`
	AppInfoID     string        `json:"appInfoId"`
	Version       string        `json:"version"`
	VersionID     string        `json:"versionId"`
	Dir           string        `json:"dir"`
	GeneratedAt   string        `json:"generatedAt"`
	Failures      []ApplyAction `json:"failures"`
}

type scopeCallCounts struct {
	create int
	update int
	delete int
	list   int
}

type localMetadataBundle struct {
	appInfo        map[string]appInfoLocalPatch
	version        map[string]versionLocalPatch
	defaultAppInfo *appInfoLocalPatch
	defaultVersion *versionLocalPatch
	// appInfoManaged and versionManaged report whether the scope directory
	// exists locally. An absent scope directory leaves that scope unmanaged:
	// its remote localizations are neither planned nor mutated.
	appInfoManaged bool
	versionManaged bool
}

type localPlanFields struct {
	setFields   map[string]string
	clearFields map[string]struct{}
}

type appInfoLocalPatch struct {
	localization AppInfoLocalization
	setFields    map[string]string
	clearFields  map[string]struct{}
}

type versionLocalPatch struct {
	localization       VersionLocalization
	createLocalization VersionLocalization
	setFields          map[string]string
	clearFields        map[string]struct{}
}

type metadataMutationCommandConfig struct {
	name      string
	verbTitle string
}

func newMetadataMutationCommand(d *shared.Deps, cfg metadataMutationCommandConfig) *ffcli.Command {
	fs := d.NewFlagSet("metadata "+cfg.name, flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appInfoID := shared.BindResourceIDFlag(fs, "app-info", "appInfos", "App Info ID (optional override)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	dir := fs.String("dir", "", "Metadata root directory (required)")
	include := fs.String("include", includeLocalizations, metadataIncludeFlagUsage())
	dryRun := fs.Bool("dry-run", false, "Preview changes without mutating App Store Connect")
	allowDeletes := fs.Bool("allow-deletes", false, "Allow destructive delete operations when applying changes (disables default locale fallback for missing locales)")
	confirm := fs.Bool("confirm", false, "Confirm mutations (required for deletes, field clears, --review-dir, or asset scopes)")
	ifExists := shared.BindIfExistsFlag(fs, shared.IfExistsSkip, shared.IfExistsUpdate)
	var reviewDir *string
	if cfg.name == "apply" {
		reviewDir = fs.String("review-dir", "", "Apply only after verifying metadata review artifacts in this directory")
	}
	output := d.BindOutputFlags(fs)
	reviewExample := ""
	if cfg.name == "apply" {
		reviewExample = fmt.Sprintf(`
  asc metadata %s --app "APP_ID" --version "1.2.3" --dir "./metadata" --review-dir ".asc/metadata/review" --confirm`, cfg.name)
	}

	return &ffcli.Command{
		Name:       cfg.name,
		ShortUsage: fmt.Sprintf(`asc metadata %s --app "APP_ID" --version "1.2.3" --dir "./metadata" [--app-info "APP_INFO_ID"] [--dry-run]`, cfg.name),
		ShortHelp:  fmt.Sprintf("%s metadata changes from canonical files.", cfg.verbTitle),
		LongHelp: fmt.Sprintf(
			`%s metadata changes from canonical files.

Examples:
  asc metadata %s --app "APP_ID" --version "1.2.3" --dir "./metadata" --dry-run
  asc metadata %s --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata" --dry-run
  asc metadata %s --app "APP_ID" --app-info "APP_INFO_ID" --version "1.2.3" --platform IOS --dir "./metadata" --dry-run
  asc metadata %s --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata %s --app "APP_ID" --version "1.2.3" --dir "./metadata" --allow-deletes --confirm%s

Notes:
  - Add --include localizations,app-clip,previews to include store assets; applying assets requires --confirm.
  - Opt-in scopes read one file each below --dir, plan only the fields present, and
    fail when App Store Connect reads back a different value after apply:
%s
  - Preview validation requires ffprobe on PATH (provided by FFmpeg).
  - default.json fallback is applied only when --allow-deletes is not set.
  - with --allow-deletes, remote locales missing locally are planned as deletes.
  - a missing app-info/ or version/<version>/ directory leaves that scope unmanaged;
    an existing directory manages every locale in that scope.
  - --dir must contain at least one metadata .json file or selected store asset; an empty tree is rejected.
  - applying an explicit null field clear requires --confirm.
  - omitted fields are treated as no-op; they do not imply deletion.
  - creating an app-info locale makes App Store Connect create an empty version
    localization; a version create planned for that locale is applied as an update.`,
			cfg.verbTitle,
			cfg.name,
			cfg.name,
			cfg.name,
			cfg.name,
			cfg.name,
			reviewExample,
			metadataSettingsHelp("      ", false),
		),
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			// Validate the raw flag before any side effect. The flag defaults to
			// fail, so an empty value here was supplied explicitly; the execution
			// path treats an unset options field as fail for in-process callers.
			if _, err := d.ParseIfExistsMode(*ifExists, shared.IfExistsSkip, shared.IfExistsUpdate); err != nil {
				return err
			}
			opts := PushExecutionOptions{
				CommandName:  cfg.name,
				AppID:        *appID,
				AppInfoID:    *appInfoID,
				Version:      *version,
				Platform:     *platform,
				Dir:          *dir,
				Include:      *include,
				DryRun:       *dryRun,
				AllowDeletes: *allowDeletes,
				Confirm:      *confirm,
				IfExists:     *ifExists,
			}
			if cfg.name == "apply" && reviewDir != nil && strings.TrimSpace(*reviewDir) != "" {
				opts.ReviewDir = *reviewDir
			}
			result, warnings, err := ExecutePushWithWarnings(ctx, d, opts)
			if err != nil && len(result.Actions) == 0 {
				return err
			}
			if err := d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return printPushPlanTable(d, result) },
				func() error { return printPushPlanMarkdown(d, result) },
			); err != nil {
				return err
			}
			if warningErr := shared.PrintSubmitReadinessCreateWarnings(d.IO.ErrOut, warnings); warningErr != nil {
				return warningErr
			}
			if err != nil || result.FailureArtifactError != "" {
				message := fmt.Sprintf("metadata %s: %d localization(s) failed", cfg.name, result.Failed)
				if result.Failed == 0 && err != nil {
					message = fmt.Sprintf("metadata %s: operation incomplete: %v", cfg.name, err)
				}
				if result.FailureArtifactError != "" {
					message += "; write failure artifact: " + result.FailureArtifactError
				}
				reportedErr := errors.New(message)
				if err != nil {
					reportedErr = shared.NewErrorWithCause(reportedErr, representativeMetadataMutationError(err))
				}
				return shared.NewReportedError(reportedErr)
			}
			return nil
		},
	}
}

func representativeMetadataMutationError(err error) error {
	// Aggregation and reconciliation both use joined errors. The action marker
	// identifies the first failed action without relying on the surrounding
	// wrapper shape, then classification stays within that action's causes.
	actionCause := firstMetadataMutationActionCause(err)
	if actionCause == nil {
		return firstMetadataMutationErrorLeaf(err)
	}
	if statusErr := firstMetadataMutationHTTPError(actionCause); statusErr != nil {
		return statusErr
	}
	return firstMetadataMutationErrorLeaf(actionCause)
}

type metadataMutationActionError struct {
	cause error
}

func newMetadataMutationActionError(cause error) error {
	if cause == nil {
		return nil
	}
	return &metadataMutationActionError{cause: cause}
}

func (e *metadataMutationActionError) Error() string {
	return e.cause.Error()
}

func (e *metadataMutationActionError) Unwrap() error {
	return e.cause
}

func firstMetadataMutationActionCause(err error) error {
	var actionErr *metadataMutationActionError
	if !errors.As(err, &actionErr) {
		return nil
	}
	return actionErr.cause
}

func firstMetadataMutationHTTPError(err error) error {
	if err == nil {
		return nil
	}
	// Inspect this node directly so a status-zero wrapper does not stop the
	// depth-first search before a later child with a valid HTTP status.
	if statusErr, ok := err.(interface { //nolint:errorlint
		error
		HTTPStatusCode() int
	}); ok {
		status := statusErr.HTTPStatusCode()
		if status >= 400 && status <= 599 {
			return statusErr
		}
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if statusErr := firstMetadataMutationHTTPError(child); statusErr != nil {
				return statusErr
			}
		}
		return nil
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return firstMetadataMutationHTTPError(wrapped.Unwrap())
	}
	return nil
}

func firstMetadataMutationErrorLeaf(err error) error {
	if err == nil {
		return nil
	}
	switch err.(type) { //nolint:errorlint
	case *url.Error, *net.OpError, *urlsanitize.TransportError, shared.ValidationFailure:
		return err
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if leaf := firstMetadataMutationErrorLeaf(child); leaf != nil {
				return leaf
			}
		}
		return nil
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok && wrapped.Unwrap() != nil {
		return firstMetadataMutationErrorLeaf(wrapped.Unwrap())
	}
	return err
}

// newMetadataPushCommand returns the metadata push subcommand.
func newMetadataPushCommand(d *shared.Deps) *ffcli.Command {
	return newMetadataMutationCommand(d, metadataMutationCommandConfig{
		name:      "push",
		verbTitle: "Push",
	})
}

func loadLocalMetadataWithAssets(d *shared.Deps, dir, version string, allowEmpty bool) (localMetadataBundle, error) {
	localAppInfo := make(map[string]appInfoLocalPatch)
	localVersion := make(map[string]versionLocalPatch)
	var defaultAppInfo *appInfoLocalPatch
	var defaultVersion *versionLocalPatch
	filesSeen := 0

	appInfoDir := filepath.Join(dir, appInfoDirName)
	appInfoEntries, err := os.ReadDir(appInfoDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return localMetadataBundle{}, fmt.Errorf("failed to read %s: %w", appInfoDir, err)
	}
	appInfoManaged := err == nil
	if appInfoManaged {
		seenAppInfoLocales := make(map[string]string)
		for _, entry := range appInfoEntries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			locale := strings.TrimSuffix(entry.Name(), ".json")
			resolvedLocale, localeErr := validateLocale(locale)
			if localeErr != nil {
				return localMetadataBundle{}, d.UsageErrorf("invalid app-info localization file %q: %v", entry.Name(), localeErr)
			}
			if err := recordCanonicalLocaleFile(seenAppInfoLocales, resolvedLocale, entry.Name()); err != nil {
				return localMetadataBundle{}, d.UsageError(err.Error())
			}
			filePath := filepath.Join(appInfoDir, entry.Name())
			patch, readErr := readAppInfoLocalizationPatchFromFile(filePath)
			if readErr != nil {
				return localMetadataBundle{}, d.UsageErrorf("invalid metadata schema in %s: %v", filePath, readErr)
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
		return localMetadataBundle{}, d.UsageError(err.Error())
	}
	versionDir := filepath.Join(dir, versionDirName, resolvedVersion)
	versionEntries, err := os.ReadDir(versionDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return localMetadataBundle{}, fmt.Errorf("failed to read %s: %w", versionDir, err)
	}
	versionManaged := err == nil
	if versionManaged {
		seenVersionLocales := make(map[string]string)
		for _, entry := range versionEntries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			locale := strings.TrimSuffix(entry.Name(), ".json")
			resolvedLocale, localeErr := validateLocale(locale)
			if localeErr != nil {
				return localMetadataBundle{}, d.UsageErrorf("invalid version localization file %q: %v", entry.Name(), localeErr)
			}
			if err := recordCanonicalLocaleFile(seenVersionLocales, resolvedLocale, entry.Name()); err != nil {
				return localMetadataBundle{}, d.UsageError(err.Error())
			}
			filePath := filepath.Join(versionDir, entry.Name())
			patch, readErr := readVersionLocalizationPatchFromFile(filePath)
			if readErr != nil {
				return localMetadataBundle{}, d.UsageErrorf("invalid metadata schema in %s: %v", filePath, readErr)
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

	if filesSeen == 0 && !allowEmpty {
		if jsonFiles, _ := filepath.Glob(filepath.Join(dir, "*.json")); len(jsonFiles) > 0 {
			return localMetadataBundle{}, d.UsageErrorf("no metadata .json files found under %s; pass the metadata root (the directory containing version/ and app-info/), e.g. ./metadata", dir)
		}
		return localMetadataBundle{}, d.UsageError("no metadata .json files found")
	}
	return localMetadataBundle{
		appInfo:        localAppInfo,
		version:        localVersion,
		defaultAppInfo: defaultAppInfo,
		defaultVersion: defaultVersion,
		appInfoManaged: appInfoManaged,
		versionManaged: versionManaged,
	}, nil
}

type exampleBuilderFunc func(appID, version, platform, dir, appInfoID string) string

func resolveMetadataPushAppInfoID(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	commandName string,
	appID string,
	appInfoID string,
	version string,
	platform string,
	dir string,
	versionState string,
) (string, error) {
	return resolveMetadataAppInfoID(ctx, d, client, appID, appInfoID, version, platform, dir, versionState, func(aid, v, p, d, infoID string) string {
		return buildMetadataAppInfoExample(commandName, aid, v, p, d, infoID) + " --dry-run"
	})
}

func resolveMetadataAppInfoID(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	appID string,
	appInfoID string,
	version string,
	platform string,
	dir string,
	versionState string,
	buildExample exampleBuilderFunc,
) (string, error) {
	ctx = asc.WithNoRedirectReads(ctx)
	if appInfoID != "" {
		return d.ResolveOwnedAppInfoID(ctx, client, appID, appInfoID)
	}

	resp, err := client.GetAppInfos(ctx, appID)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("empty app infos response for app %q", appID)
	}
	if len(resp.Data) == 0 {
		if strings.TrimSpace(resp.Links.Next) != "" {
			return "", d.AmbiguousUsageError(shared.MarkAmbiguousSelectionSample(&shared.AmbiguousSelectionError{
				Kind:        "app info",
				Description: fmt.Sprintf("app %q", appID),
				Flag:        "--app-info",
				Candidates:  nil,
				Hint:        fmt.Sprintf("Inspect them with `asc apps info list --app %q` before retrying.", appID),
			}))
		}
		return "", shared.NewErrorWithCause(fmt.Errorf("no app info found for app %q", appID), asc.ErrNotFound)
	}
	if strings.TrimSpace(resp.Links.Next) != "" {
		candidates := asc.AppInfoCandidates(resp.Data)
		exampleAppInfoID := "<APP_INFO_ID>"
		for _, candidate := range candidates {
			if candidate.ID != "" {
				exampleAppInfoID = candidate.ID
				break
			}
		}
		exampleCommand := buildExample(appID, version, platform, dir, exampleAppInfoID)
		return "", d.AmbiguousUsageError(shared.MarkAmbiguousSelectionSample(&shared.AmbiguousSelectionError{
			Kind:        "app info",
			Description: fmt.Sprintf("app %q", appID),
			Flag:        "--app-info",
			Candidates:  shared.AppInfoAmbiguousCandidates(candidates),
			Hint:        fmt.Sprintf("Inspect them with `asc apps info list --app %q`. Example: %s", appID, exampleCommand),
		}))
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
	return "", d.AmbiguousUsageError(&shared.AmbiguousSelectionError{
		Kind:        "app info",
		Description: fmt.Sprintf("app %q", appID),
		Flag:        "--app-info",
		Candidates:  shared.AppInfoAmbiguousCandidates(candidates),
		Hint:        fmt.Sprintf("Inspect them with `asc apps info list --app %q`. Example: %s", appID, exampleCommand),
	})
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
	parts = append(
		parts,
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
	clearFields := make(map[string]struct{})
	loc := AppInfoLocalization{}
	for key, rawValue := range raw {
		canonicalKey, err := canonicalStringFieldPatchKey(key, appInfoPlanFields)
		if err != nil {
			return appInfoLocalPatch{}, err
		}
		if fieldPatchAlreadyDecoded(canonicalKey, setFields, clearFields) {
			return appInfoLocalPatch{}, fmt.Errorf("json: duplicate field %q", canonicalKey)
		}
		value, clear, err := decodeStringFieldPatch(canonicalKey, rawValue)
		if err != nil {
			return appInfoLocalPatch{}, err
		}
		if clear {
			clearFields[canonicalKey] = struct{}{}
			continue
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

	if len(setFields) == 0 && len(clearFields) == 0 {
		return appInfoLocalPatch{}, fmt.Errorf("at least one app-info field is required")
	}

	return appInfoLocalPatch{
		localization: NormalizeAppInfoLocalization(loc),
		setFields:    setFields,
		clearFields:  clearFields,
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
	clearFields := make(map[string]struct{})
	loc := VersionLocalization{}
	for key, rawValue := range raw {
		canonicalKey, err := canonicalStringFieldPatchKey(key, versionPlanFields)
		if err != nil {
			return versionLocalPatch{}, err
		}
		if fieldPatchAlreadyDecoded(canonicalKey, setFields, clearFields) {
			return versionLocalPatch{}, fmt.Errorf("json: duplicate field %q", canonicalKey)
		}
		value, clear, err := decodeStringFieldPatch(canonicalKey, rawValue)
		if err != nil {
			return versionLocalPatch{}, err
		}
		if clear {
			clearFields[canonicalKey] = struct{}{}
			continue
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

	if len(setFields) == 0 && len(clearFields) == 0 {
		return versionLocalPatch{}, fmt.Errorf("at least one version metadata field is required")
	}

	normalized := NormalizeVersionLocalization(loc)
	if err := shared.ValidateVersionLocalizationAttributes(versionAttributes("", normalized, false)); err != nil {
		return versionLocalPatch{}, err
	}

	return versionLocalPatch{
		localization: normalized,
		setFields:    setFields,
		clearFields:  clearFields,
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
	return "", fmt.Errorf("json: unknown field %q; %s", field, unknownLocalizationFieldHint(field, allowed))
}

// unknownLocalizationFieldHint says where a field that is not allowed in this
// localization file belongs, so the file can be fixed in one step.
func unknownLocalizationFieldHint(field string, allowed []string) string {
	belongsIn := func(fields []string) bool {
		return slices.ContainsFunc(fields, func(key string) bool { return strings.EqualFold(field, key) })
	}
	scope := "version"
	if slices.Equal(allowed, appInfoPlanFields) {
		scope = "app-info"
		if belongsIn(versionPlanFields) {
			return "it is a version localization field: move it to version/<version>/<locale>.json"
		}
	} else if belongsIn(appInfoPlanFields) {
		return "it is an app-info localization field: move it to app-info/<locale>.json"
	}
	switch strings.ToLower(field) {
	case "copyright":
		return "copyright is not a localization field: remove it and set it with `asc versions update --version-id VERSION_ID --copyright \"TEXT\"`"
	case "locale":
		return "the file name sets the locale: remove this key"
	}
	return fmt.Sprintf("%s localization fields: %s", scope, strings.Join(allowed, ", "))
}

// clearableMetadataFields lists the localization attributes App Store Connect
// models as nullable and that a metadata file may clear with an explicit null.
var clearableMetadataFields = map[string]struct{}{
	"subtitle":         {},
	"privacyPolicyUrl": {},
	"promotionalText":  {},
}

func fieldPatchAlreadyDecoded(field string, setFields map[string]string, clearFields map[string]struct{}) bool {
	if _, exists := setFields[field]; exists {
		return true
	}
	_, cleared := clearFields[field]
	return cleared
}

// decodeStringFieldPatch reports whether the field is explicitly cleared with a
// JSON null. An empty string never clears a field, so template values written by
// `asc metadata init` cannot wipe remote text.
func decodeStringFieldPatch(field string, raw json.RawMessage) (string, bool, error) {
	if isJSONNull(raw) {
		if _, clearable := clearableMetadataFields[field]; !clearable {
			return "", false, fmt.Errorf("field %q cannot be null; omit the key to leave the remote value unchanged", field)
		}
		return "", true, nil
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false, err
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "__ASC_DELETE__" {
		return "", false, fmt.Errorf("field %q uses unsupported clear token __ASC_DELETE__; omit the key to keep the remote value", field)
	}
	if trimmed == "" {
		return "", false, fmt.Errorf("field %q cannot be empty; omit the key to leave the remote value unchanged", field)
	}
	return trimmed, false, nil
}

func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
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
		clearFields:  cloneFieldSet(patch.clearFields),
	}
}

func cloneVersionLocalPatch(patch versionLocalPatch) versionLocalPatch {
	return versionLocalPatch{
		localization:       patch.localization,
		createLocalization: patch.createLocalization,
		setFields:          cloneStringMap(patch.setFields),
		clearFields:        cloneFieldSet(patch.clearFields),
	}
}

func validateMetadataCreatePrerequisites(local map[string]appInfoLocalPatch, remote map[string]AppInfoLocalization) error {
	locales := make([]string, 0, len(local))
	for locale := range local {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	for _, locale := range locales {
		if _, exists := remote[locale]; exists {
			continue
		}
		if strings.TrimSpace(local[locale].localization.Name) == "" {
			return fmt.Errorf("app-info localization %q requires name when creating a new locale", locale)
		}
	}
	return nil
}

func findLateExistingAppInfoLocalizations(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	appInfoID string,
	local map[string]appInfoLocalPatch,
	remote map[string]AppInfoLocalization,
) (map[string]string, string, error) {
	// A PATCH-only app-info file has no name for a valid create request. Under
	// --if-exists, re-read before applying anything so a locale that appeared
	// after the plan read can be skipped or updated without partial writes.
	items, err := fetchAppInfoLocalizations(ctx, d, client, appInfoID)
	if err != nil {
		return nil, "", fmt.Errorf("read back app-info localizations before create: %w", err)
	}
	refreshedIDs := make(map[string]string, len(items))
	for _, item := range items {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale != "" {
			refreshedIDs[locale] = item.ID
		}
	}

	lateExisting := make(map[string]string)
	for _, locale := range sortedKeys(local) {
		if _, exists := remote[locale]; exists || strings.TrimSpace(local[locale].localization.Name) != "" {
			continue
		}
		id, exists := refreshedIDs[locale]
		if !exists {
			return nil, locale, nil
		}
		lateExisting[locale] = id
	}
	return lateExisting, "", nil
}

func cloneStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneFieldSet(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source))
	for field := range source {
		result[field] = struct{}{}
	}
	return result
}

// nullableLocalizationPayload sends set values as strings and cleared fields as
// JSON null, leaving every other field untouched.
func nullableLocalizationPayload(setFields map[string]string, clearFields map[string]struct{}) map[string]asc.NullableString {
	fields := make(map[string]asc.NullableString, len(setFields)+len(clearFields))
	for field, value := range setFields {
		fields[field] = asc.NullableString{Value: &value}
	}
	for field := range clearFields {
		fields[field] = asc.NullableString{}
	}
	return fields
}

// desiredLocalizationFields describes the intended end state, where a cleared
// field is expected to read back as empty.
func desiredLocalizationFields(setFields map[string]string, clearFields map[string]struct{}) map[string]string {
	desired := cloneStringMap(setFields)
	for field := range clearFields {
		desired[field] = ""
	}
	return desired
}

func appInfoToPlanFields(values map[string]appInfoLocalPatch) map[string]localPlanFields {
	result := make(map[string]localPlanFields, len(values))
	for locale, value := range values {
		result[locale] = localPlanFields{
			setFields:   cloneStringMap(value.setFields),
			clearFields: cloneFieldSet(value.clearFields),
		}
	}
	return result
}

func versionToPlanFields(values map[string]versionLocalPatch) map[string]localPlanFields {
	result := make(map[string]localPlanFields, len(values))
	for locale, value := range values {
		result[locale] = localPlanFields{
			setFields:   cloneStringMap(value.setFields),
			clearFields: cloneFieldSet(value.clearFields),
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
	d *shared.Deps,
	client *asc.Client,
	appInfoID string,
	versionID string,
	version string,
	localAppInfo map[string]appInfoLocalPatch,
	localVersion map[string]versionLocalPatch,
	remoteAppInfoItems []asc.Resource[asc.AppInfoLocalizationAttributes],
	remoteVersionItems []asc.Resource[asc.AppStoreVersionLocalizationAttributes],
	allowDeletes bool,
	ifExists metadataIfExistsOptions,
) ([]ApplyAction, error) {
	actions := make([]ApplyAction, 0)
	applyErrors := make([]error, 0)

	appInfoActions, err := applyAppInfoChanges(ctx, d, client, appInfoID, localAppInfo, remoteAppInfoItems, allowDeletes, ifExists)
	actions = append(actions, appInfoActions...)
	if err != nil {
		applyErrors = append(applyErrors, err)
	}

	createdLocales := appInfoCreatedVersionLocales(appInfoActions, localVersion, remoteVersionItems)
	if len(createdLocales) > 0 {
		refreshed, err := fetchVersionLocalizations(ctx, d, client, versionID)
		if err != nil {
			fmt.Fprintf(d.IO.ErrOut, "%s: warning: refresh version localizations after app-info creates: %v\n", ifExists.prefix, err)
		}
		converted := make(map[string]struct{}, len(createdLocales))
		for _, item := range refreshed {
			locale := strings.TrimSpace(item.Attributes.Locale)
			if _, ok := createdLocales[locale]; ok {
				remoteVersionItems = append(slices.Clip(remoteVersionItems), item)
				converted[locale] = struct{}{}
			}
		}
		createdLocales = converted
	}

	versionActions, err := applyVersionChanges(ctx, d, client, versionID, version, localVersion, remoteVersionItems, allowDeletes, ifExists)
	for i := range versionActions {
		_, versionActions[i].convertedFromCreate = createdLocales[versionActions[i].Locale]
		if slices.Contains(versionActions[i].SkippedFields, "whatsNew") {
			rest := "The other fields were pushed."
			switch versionActions[i].Status {
			case metadataActionStatusSkipped:
				rest = "Nothing else changed."
			case metadataActionStatusFailed:
				rest = "Pushing the other fields failed; see the action error."
			}
			fmt.Fprintf(d.IO.ErrOut, "Warning: whatsNew was not set for locale %s of version %s: App Store Connect does not allow What's New on an app's first version (409 STATE_ERROR). %s\n", versionActions[i].Locale, version, rest)
		}
	}
	actions = append(actions, versionActions...)
	if err != nil {
		applyErrors = append(applyErrors, err)
	}

	return actions, errors.Join(applyErrors...)
}

// appInfoCreatedVersionLocales returns planned version creates whose locale
// this run created in app-info. App Store Connect creates an empty version
// localization alongside, so those creates are applied as updates.
func appInfoCreatedVersionLocales(
	actions []ApplyAction,
	localVersion map[string]versionLocalPatch,
	remoteVersionItems []asc.Resource[asc.AppStoreVersionLocalizationAttributes],
) map[string]struct{} {
	plannedVersionLocales := make(map[string]struct{}, len(remoteVersionItems))
	for _, item := range remoteVersionItems {
		plannedVersionLocales[strings.TrimSpace(item.Attributes.Locale)] = struct{}{}
	}
	createdLocales := make(map[string]struct{})
	for _, action := range actions {
		_, versionLocal := localVersion[action.Locale]
		_, versionRemote := plannedVersionLocales[action.Locale]
		if action.Scope == appInfoDirName && action.Status == metadataActionStatusSucceeded && (action.Action == "create" || action.Action == "reconcile") && !action.AlreadyExists && !action.reconciledDuplicate && versionLocal && !versionRemote {
			createdLocales[action.Locale] = struct{}{}
		}
	}
	return createdLocales
}

func applyAppInfoChanges(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	appInfoID string,
	local map[string]appInfoLocalPatch,
	remoteItems []asc.Resource[asc.AppInfoLocalizationAttributes],
	allowDeletes bool,
	ifExists metadataIfExistsOptions,
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
	if !allowDeletes {
		for _, locale := range locales {
			_, localExists := local[locale]
			remoteState, remoteExists := remoteByLocale[locale]
			if !localExists && remoteExists && hasTrackedRemoteFields(appInfoPlanFields, remoteState.fields) {
				return nil, fmt.Errorf("delete operations require --allow-deletes")
			}
		}
	}

	return runMetadataLocaleWrites(locales, func(locale string) (result metadataLocaleResult) {
		localPatch, localExists := local[locale]
		remoteState, remoteExists := remoteByLocale[locale]
		if ctxErr := ctx.Err(); ctxErr != nil {
			if action, ok := canceledAppInfoAction(locale, localPatch, remoteState, localExists, remoteExists, allowDeletes, ctxErr); ok {
				result.canceled = ctxErr
				result.actions = append(result.actions, action)
			}
			return result
		}

		if !localExists && remoteExists {
			if !hasTrackedRemoteFields(appInfoPlanFields, remoteState.fields) {
				return result
			}
			id, action, err := runMetadataMutation(
				ctx, d, "delete",
				func(requestCtx context.Context) (string, error) {
					return remoteState.id, client.DeleteAppInfoLocalization(requestCtx, remoteState.id)
				},
				func(readbackCtx context.Context) (string, bool, error) {
					_, exists, readErr := readBackAppInfoLocalization(readbackCtx, d, client, appInfoID, locale, nil)
					return remoteState.id, !exists, readErr
				},
			)
			if err != nil {
				result.actions = append(result.actions, failedMetadataAction(appInfoDirName, locale, "", "delete", remoteState.id, nil, err))
				result.errs = append(result.errs, newMetadataMutationActionError(fmt.Errorf("delete app-info localization %s: %w", locale, err)))
			} else {
				result.actions = append(result.actions, successfulMetadataAction(appInfoDirName, locale, "", action, id))
			}
			return result
		}
		if !localExists {
			return result
		}

		remoteFields := cloneStringMap(remoteState.fields)
		adds, updates := countIntentChanges(appInfoPlanFields, localPatch.setFields, localPatch.clearFields, remoteFields)
		_, lateExisting := ifExists.lateAppInfoIDs[locale]
		// A locale found only by the --if-exists preflight has no planned
		// remote state, so a clear-only file counts no intent against it. Route
		// it anyway; the existing-localization path compares the real state.
		if adds == 0 && updates == 0 && !lateExisting {
			return result
		}

		switch {
		case !remoteExists:
			conflict := metadataCreateConflict{
				options: ifExists,
				scope:   appInfoDirName,
				locale:  locale,
				desired: desiredLocalizationFields(localPatch.setFields, localPatch.clearFields),
				clears:  localPatch.clearFields,
				lookup: func(readCtx context.Context) (string, bool, error) {
					return readBackAppInfoLocalization(readCtx, d, client, appInfoID, locale, nil)
				},
				update: func(requestCtx context.Context, existingID string) (string, error) {
					resp, mutationErr := client.UpdateAppInfoLocalizationNullableFields(requestCtx, existingID, nullableLocalizationPayload(localPatch.setFields, localPatch.clearFields))
					if mutationErr != nil {
						return "", mutationErr
					}
					return resp.Data.ID, nil
				},
				readback: func(readbackCtx context.Context) (string, bool, error) {
					return readBackAppInfoLocalization(readbackCtx, d, client, appInfoID, locale, desiredLocalizationFields(localPatch.setFields, localPatch.clearFields))
				},
			}
			if existingID, exists := ifExists.lateAppInfoIDs[locale]; exists {
				outcome := handleMetadataExistingConflict(ctx, d, conflict, existingID)
				result.actions = append(result.actions, outcome.action)
				if outcome.err != nil {
					result.errs = append(result.errs, newMetadataMutationActionError(fmt.Errorf("update existing app-info localization %s (fields: %s): %w", locale, formatAttemptedFieldMap(appInfoPlanFields, conflict.desired), outcome.err)))
				}
				return result
			}
			if strings.TrimSpace(localPatch.localization.Name) == "" {
				err := fmt.Errorf("cannot create app-info localization %q without name", locale)
				result.actions = append(result.actions, failedMetadataAction(appInfoDirName, locale, "", "create", "", localPatch.setFields, err))
				result.errs = append(result.errs, newMetadataMutationActionError(err))
				return result
			}
			desired := appInfoFields(localPatch.localization)
			id, action, err := runMetadataMutation(
				ctx, d, "create",
				func(requestCtx context.Context) (string, error) {
					resp, mutationErr := client.CreateAppInfoLocalization(requestCtx, appInfoID, appInfoAttributes(locale, localPatch.localization, true).CreateAttributes())
					if mutationErr != nil {
						return "", mutationErr
					}
					return resp.Data.ID, nil
				},
				func(readbackCtx context.Context) (string, bool, error) {
					return readBackAppInfoLocalization(readbackCtx, d, client, appInfoID, locale, desired)
				},
				ifExists.mode,
			)
			if err != nil {
				outcome := resolveMetadataCreateConflict(ctx, d, err, conflict)
				if outcome.handled {
					result.actions = append(result.actions, outcome.action)
					if outcome.err != nil {
						result.errs = append(result.errs, newMetadataMutationActionError(fmt.Errorf("update existing app-info localization %s (fields: %s): %w", locale, formatAttemptedFieldMap(appInfoPlanFields, conflict.desired), outcome.err)))
					}
					return result
				}
				err = outcome.err
				result.actions = append(result.actions, failedMetadataAction(appInfoDirName, locale, "", "create", "", desired, err))
				result.errs = append(result.errs, newMetadataMutationActionError(fmt.Errorf("create app-info localization %s (fields: %s): %w", locale, formatAttemptedFieldMap(appInfoPlanFields, localPatch.setFields), err)))
			} else {
				result.actions = append(result.actions, successfulMetadataAction(appInfoDirName, locale, "", action, id))
			}
		case remoteExists:
			setFields := changedSetFields(localPatch.setFields, remoteFields)
			clearFields := changedClearFields(localPatch.clearFields, remoteFields)
			desired := desiredLocalizationFields(setFields, clearFields)
			id, action, err := runMetadataMutation(
				ctx, d, "update",
				func(requestCtx context.Context) (string, error) {
					resp, mutationErr := client.UpdateAppInfoLocalizationNullableFields(
						requestCtx,
						remoteState.id,
						nullableLocalizationPayload(setFields, clearFields),
					)
					if mutationErr != nil {
						return "", mutationErr
					}
					return resp.Data.ID, nil
				},
				func(readbackCtx context.Context) (string, bool, error) {
					return readBackAppInfoLocalization(readbackCtx, d, client, appInfoID, locale, desired)
				},
			)
			if err != nil {
				result.actions = append(result.actions, failedMetadataAction(appInfoDirName, locale, "", "update", remoteState.id, desired, err))
				result.errs = append(result.errs, newMetadataMutationActionError(fmt.Errorf("update app-info localization %s (fields: %s): %w", locale, formatAttemptedFieldMap(appInfoPlanFields, desired), err)))
			} else {
				result.actions = append(result.actions, successfulMetadataAction(appInfoDirName, locale, "", action, id))
			}
		}
		return result
	})
}

func applyVersionChanges(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	versionID string,
	version string,
	local map[string]versionLocalPatch,
	remoteItems []asc.Resource[asc.AppStoreVersionLocalizationAttributes],
	allowDeletes bool,
	ifExists metadataIfExistsOptions,
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

	if !allowDeletes {
		for _, locale := range locales {
			_, localExists := local[locale]
			remoteState, remoteExists := remoteByLocale[locale]
			if !localExists && remoteExists && hasTrackedRemoteFields(versionPlanFields, remoteState.fields) {
				return nil, fmt.Errorf("delete operations require --allow-deletes")
			}
		}
	}

	return runMetadataLocaleWrites(locales, func(locale string) (result metadataLocaleResult) {
		localPatch, localExists := local[locale]
		remoteState, remoteExists := remoteByLocale[locale]
		if ctxErr := ctx.Err(); ctxErr != nil {
			if action, ok := canceledVersionAction(locale, version, localPatch, remoteState, localExists, remoteExists, allowDeletes, ctxErr); ok {
				result.canceled = ctxErr
				result.actions = append(result.actions, action)
			}
			return result
		}

		if !localExists && remoteExists {
			if !hasTrackedRemoteFields(versionPlanFields, remoteState.fields) {
				return result
			}
			id, action, err := runMetadataMutation(
				ctx, d, "delete",
				func(requestCtx context.Context) (string, error) {
					return remoteState.id, client.DeleteAppStoreVersionLocalization(requestCtx, remoteState.id)
				},
				func(readbackCtx context.Context) (string, bool, error) {
					_, exists, readErr := readBackVersionLocalization(readbackCtx, d, client, versionID, locale, nil)
					return remoteState.id, !exists, readErr
				},
			)
			if err != nil {
				result.actions = append(result.actions, failedMetadataAction(versionDirName, locale, version, "delete", remoteState.id, nil, err))
				result.errs = append(result.errs, newMetadataMutationActionError(fmt.Errorf("delete version localization %s: %w", locale, err)))
			} else {
				result.actions = append(result.actions, successfulMetadataAction(versionDirName, locale, version, action, id))
			}
			return result
		}
		if !localExists {
			return result
		}

		remoteFields := cloneStringMap(remoteState.fields)
		adds, updates := countIntentChanges(versionPlanFields, localPatch.setFields, localPatch.clearFields, remoteFields)
		if adds == 0 && updates == 0 {
			return result
		}

		switch {
		case !remoteExists:
			createLoc := localPatch.localization
			if hasVersionContent(localPatch.createLocalization) {
				createLoc = localPatch.createLocalization
			}
			desired := versionFields(createLoc)
			id, action, err := runMetadataMutation(
				ctx, d, "create",
				func(requestCtx context.Context) (string, error) {
					resp, mutationErr := client.CreateAppStoreVersionLocalization(requestCtx, versionID, versionAttributes(locale, createLoc, true).CreateAttributes())
					if mutationErr != nil {
						return "", mutationErr
					}
					return resp.Data.ID, nil
				},
				func(readbackCtx context.Context) (string, bool, error) {
					return readBackVersionLocalization(readbackCtx, d, client, versionID, locale, desired)
				},
				ifExists.mode,
			)
			if err != nil {
				outcome := resolveMetadataCreateConflict(ctx, d, err, metadataCreateConflict{
					options: ifExists,
					scope:   versionDirName,
					locale:  locale,
					version: version,
					desired: desiredLocalizationFields(localPatch.setFields, localPatch.clearFields),
					clears:  localPatch.clearFields,
					lookup: func(readCtx context.Context) (string, bool, error) {
						return readBackVersionLocalization(readCtx, d, client, versionID, locale, nil)
					},
					update: func(requestCtx context.Context, existingID string) (string, error) {
						resp, mutationErr := client.UpdateAppStoreVersionLocalizationNullableFields(requestCtx, existingID, nullableLocalizationPayload(localPatch.setFields, localPatch.clearFields))
						if mutationErr != nil {
							return "", mutationErr
						}
						return resp.Data.ID, nil
					},
					readback: func(readbackCtx context.Context) (string, bool, error) {
						return readBackVersionLocalization(readbackCtx, d, client, versionID, locale, desiredLocalizationFields(localPatch.setFields, localPatch.clearFields))
					},
				})
				if outcome.handled {
					result.actions = append(result.actions, outcome.action)
					if outcome.err != nil {
						result.errs = append(result.errs, newMetadataMutationActionError(fmt.Errorf("update existing version localization %s (fields: %s): %w", locale, formatAttemptedFieldMap(versionPlanFields, desiredLocalizationFields(localPatch.setFields, localPatch.clearFields)), outcome.err)))
					}
					return result
				}
				err = outcome.err
				result.actions = append(result.actions, failedMetadataAction(versionDirName, locale, version, "create", "", desired, err))
				result.errs = append(result.errs, newMetadataMutationActionError(fmt.Errorf("create version localization %s (fields: %s): %w", locale, formatAttemptedFieldMap(versionPlanFields, localPatch.setFields), err)))
			} else {
				result.actions = append(result.actions, successfulMetadataAction(versionDirName, locale, version, action, id))
			}
		case remoteExists:
			setFields := changedSetFields(localPatch.setFields, remoteFields)
			clearFields := changedClearFields(localPatch.clearFields, remoteFields)
			desired := desiredLocalizationFields(setFields, clearFields)
			var skippedFields []string
			id, action, err := runMetadataMutation(
				ctx, d, "update",
				func(requestCtx context.Context) (string, error) {
					resp, mutationErr := client.UpdateAppStoreVersionLocalizationNullableFields(
						requestCtx,
						remoteState.id,
						nullableLocalizationPayload(setFields, clearFields),
					)
					// Apple has no What's New on an app's first version and
					// refuses the whole PATCH, applying nothing, so the other
					// fields can be sent again without it.
					// A later version answers a locked whatsNew with the same
					// 409, so this applies only to the app's first version.
					firstVersion := false
					if mutationErr != nil && skippedFields == nil && setFields["whatsNew"] != "" && isWhatsNewOnlyStateError(mutationErr) && ifExists.firstVersion != nil {
						var readErr error
						firstVersion, readErr = ifExists.firstVersion(ctx)
						if readErr != nil {
							mutationErr = fmt.Errorf("%w\n(could not confirm that version %s is the app's first version, where App Store Connect refuses whatsNew and push leaves it out: %w; rerun the push)", mutationErr, version, readErr)
						}
					}
					if firstVersion {
						skippedFields = []string{"whatsNew"}
						delete(setFields, "whatsNew")
						delete(desired, "whatsNew")
						if len(setFields) == 0 && len(clearFields) == 0 {
							return remoteState.id, nil
						}
						resp, mutationErr = client.UpdateAppStoreVersionLocalizationNullableFields(
							requestCtx,
							remoteState.id,
							nullableLocalizationPayload(setFields, clearFields),
						)
					}
					if mutationErr != nil {
						return "", mutationErr
					}
					return resp.Data.ID, nil
				},
				func(readbackCtx context.Context) (string, bool, error) {
					return readBackVersionLocalization(readbackCtx, d, client, versionID, locale, desired)
				},
			)
			if err != nil {
				failed := failedMetadataAction(versionDirName, locale, version, "update", remoteState.id, desired, err)
				failed.SkippedFields = skippedFields
				result.actions = append(result.actions, failed)
				result.errs = append(result.errs, newMetadataMutationActionError(fmt.Errorf("update version localization %s (fields: %s): %w", locale, formatAttemptedFieldMap(versionPlanFields, desired), err)))
			} else {
				succeeded := successfulMetadataAction(versionDirName, locale, version, action, id)
				succeeded.SkippedFields = skippedFields
				if len(desired) == 0 {
					succeeded.Status = metadataActionStatusSkipped
				}
				result.actions = append(result.actions, succeeded)
			}
		}
		return result
	})
}

const metadataWriteConcurrency = 4

type metadataLocaleResult struct {
	actions  []ApplyAction
	errs     []error
	canceled error
}

// runMetadataLocaleWrites applies independent per-locale writes with bounded
// concurrency and merges results in locale order, recording the context error
// once as the serial loop did.
func runMetadataLocaleWrites(locales []string, apply func(string) metadataLocaleResult) ([]ApplyAction, error) {
	results := make([]metadataLocaleResult, len(locales))
	limiter := make(chan struct{}, metadataWriteConcurrency)
	var workers sync.WaitGroup
	for index, locale := range locales {
		limiter <- struct{}{}
		workers.Go(func() {
			defer func() { <-limiter }()
			results[index] = apply(locale)
		})
	}
	workers.Wait()

	actions := make([]ApplyAction, 0)
	applyErrors := make([]error, 0)
	contextErrorRecorded := false
	for _, result := range results {
		if result.canceled != nil && !contextErrorRecorded {
			applyErrors = append(applyErrors, newMetadataMutationActionError(result.canceled))
			contextErrorRecorded = true
		}
		actions = append(actions, result.actions...)
		applyErrors = append(applyErrors, result.errs...)
	}
	return actions, errors.Join(applyErrors...)
}

func canceledAppInfoAction(
	locale string,
	localPatch appInfoLocalPatch,
	remoteState remoteLocalizationState,
	localExists bool,
	remoteExists bool,
	allowDeletes bool,
	err error,
) (ApplyAction, bool) {
	if !localExists {
		if remoteExists && allowDeletes && hasTrackedRemoteFields(appInfoPlanFields, remoteState.fields) {
			return failedMetadataAction(appInfoDirName, locale, "", "delete", remoteState.id, nil, err), true
		}
		return ApplyAction{}, false
	}
	adds, updates := countIntentChanges(appInfoPlanFields, localPatch.setFields, localPatch.clearFields, remoteState.fields)
	if adds == 0 && updates == 0 {
		return ApplyAction{}, false
	}
	if !remoteExists {
		return failedMetadataAction(appInfoDirName, locale, "", "create", "", appInfoFields(localPatch.localization), err), true
	}
	return failedMetadataAction(
		appInfoDirName, locale, "", "update", remoteState.id,
		desiredLocalizationFields(localPatch.setFields, localPatch.clearFields),
		err,
	), true
}

func canceledVersionAction(
	locale string,
	version string,
	localPatch versionLocalPatch,
	remoteState remoteLocalizationState,
	localExists bool,
	remoteExists bool,
	allowDeletes bool,
	err error,
) (ApplyAction, bool) {
	if !localExists {
		if remoteExists && allowDeletes && hasTrackedRemoteFields(versionPlanFields, remoteState.fields) {
			return failedMetadataAction(versionDirName, locale, version, "delete", remoteState.id, nil, err), true
		}
		return ApplyAction{}, false
	}
	adds, updates := countIntentChanges(versionPlanFields, localPatch.setFields, localPatch.clearFields, remoteState.fields)
	if adds == 0 && updates == 0 {
		return ApplyAction{}, false
	}
	if !remoteExists {
		return failedMetadataAction(versionDirName, locale, version, "create", "", versionFields(effectiveVersionCreateLocalization(localPatch)), err), true
	}
	return failedMetadataAction(
		versionDirName, locale, version, "update", remoteState.id,
		desiredLocalizationFields(localPatch.setFields, localPatch.clearFields),
		err,
	), true
}

func hasTrackedRemoteFields(fields []string, values map[string]string) bool {
	for _, field := range fields {
		if _, ok := values[field]; ok {
			return true
		}
	}
	return false
}

// metadataLocalizationExistsCodes lists the Apple 409 error codes that mean a
// localization for the locale already exists. Both
// POST /v1/appStoreVersionLocalizations and POST /v1/appInfoLocalizations
// reject a duplicate locale with ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE on
// /data/attributes/locale. Every other 409, including STATE_ERROR.* when the
// version or app info is not editable, is not an existence conflict and keeps
// failing.
var metadataLocalizationExistsCodes = []string{"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE"}

// metadataIfExistsOptions carries the resolved --if-exists mode plus the
// command prefix used in stderr diagnostics ("metadata push" or
// "metadata apply").
type metadataIfExistsOptions struct {
	mode   shared.IfExistsMode
	prefix string
	// lateAppInfoIDs are locales found by the preflight read after the plan read.
	lateAppInfoIDs map[string]string
	// firstVersion reports whether the pushed version is the app's first on
	// its platform, or the error of the read that would tell; nil means it is
	// not known to be.
	firstVersion func(context.Context) (bool, error)
}

// metadataCreateConflict describes one localization create whose failure may
// mean the locale already exists.
type metadataCreateConflict struct {
	options metadataIfExistsOptions
	scope   string
	locale  string
	version string
	// desired is the end state the routed update must produce: set values plus
	// explicit clears, which read back as empty.
	desired map[string]string
	// clears are the explicit null clears the local file requests. A create
	// cannot carry them, so they only take effect on the routed update.
	clears   map[string]struct{}
	lookup   func(context.Context) (string, bool, error)
	update   func(context.Context, string) (string, error)
	readback func(context.Context) (string, bool, error)
}

// metadataCreateConflictReadBackError carries a matching read-back through the
// existing conflict path so explicit --if-exists modes can reuse the resolved
// ID without issuing another GET.
type metadataCreateConflictReadBackError struct {
	cause          error
	localizationID string
}

func (err *metadataCreateConflictReadBackError) Error() string {
	return err.cause.Error()
}

func (err *metadataCreateConflictReadBackError) Unwrap() error {
	return err.cause
}

// metadataConflictOutcome reports how --if-exists handled a failed create.
// When handled is false the create keeps failing and err is the error to
// record; when handled is true action is the receipt entry and a non-nil err
// means the routed update itself failed.
type metadataConflictOutcome struct {
	handled bool
	action  ApplyAction
	err     error
}

// resolveMetadataCreateConflict applies --if-exists to a localization create
// that failed. It only treats the failure as "already exists" when the shared
// two-step rule holds: an HTTP 409 whose Apple code is in
// metadataLocalizationExistsCodes, and a decisive natural-key read-back that
// finds the locale. skip records the existing localization untouched; update
// routes the same desired fields to the localization's PATCH.
func resolveMetadataCreateConflict(ctx context.Context, d *shared.Deps, createErr error, conflict metadataCreateConflict) metadataConflictOutcome {
	var readBackErr *metadataCreateConflictReadBackError
	if errors.As(createErr, &readBackErr) {
		// The create read-back only compares the fields a create can carry.
		// Explicit clears are not among them, so under update the existing
		// localization is re-checked against the full desired state.
		if conflict.options.mode == shared.IfExistsUpdate && len(conflict.clears) > 0 {
			return handleMetadataExistingConflict(ctx, d, conflict, readBackErr.localizationID)
		}
		return metadataExistingConflictAlreadyMatches(d, conflict, readBackErr.localizationID)
	}

	existingID, handled, resolveErr := shared.ResolveIfExistsConflict(
		conflict.options.mode,
		createErr,
		metadataLocalizationExistsCodes,
		func() (string, bool, error) { return conflict.lookup(ctx) },
	)
	if !handled {
		return metadataConflictOutcome{err: resolveErr}
	}
	return handleMetadataExistingConflict(ctx, d, conflict, existingID)
}

func metadataExistingConflictAlreadyMatches(d *shared.Deps, conflict metadataCreateConflict, existingID string) metadataConflictOutcome {
	if conflict.options.mode != shared.IfExistsUpdate {
		reportMetadataConflictResolution(d, conflict, existingID, "left unchanged")
		return metadataConflictOutcome{
			handled: true,
			action: markMetadataConflictAction(ApplyAction{
				Scope:          conflict.scope,
				Locale:         conflict.locale,
				Version:        conflict.version,
				Action:         "create",
				Status:         metadataActionStatusSkipped,
				LocalizationID: existingID,
			}, conflict.options.mode),
		}
	}

	reportMetadataConflictResolution(d, conflict, existingID, "already matches the requested fields")
	return metadataConflictOutcome{
		handled: true,
		action: markMetadataConflictAction(successfulMetadataAction(
			conflict.scope, conflict.locale, conflict.version, "reconcile", existingID,
		), conflict.options.mode),
	}
}

func handleMetadataExistingConflict(ctx context.Context, d *shared.Deps, conflict metadataCreateConflict, existingID string) metadataConflictOutcome {
	if conflict.options.mode != shared.IfExistsUpdate {
		reportMetadataConflictResolution(d, conflict, existingID, "left unchanged")
		return metadataConflictOutcome{
			handled: true,
			action: markMetadataConflictAction(ApplyAction{
				Scope:          conflict.scope,
				Locale:         conflict.locale,
				Version:        conflict.version,
				Action:         "create",
				Status:         metadataActionStatusSkipped,
				LocalizationID: existingID,
			}, conflict.options.mode),
		}
	}
	if conflict.readback != nil {
		id, matches, err := conflict.readback(ctx)
		if err != nil && ctx.Err() != nil {
			err = ctx.Err()
			return metadataConflictOutcome{
				handled: true,
				action:  markMetadataConflictAction(failedMetadataAction(conflict.scope, conflict.locale, conflict.version, "update", existingID, conflict.desired, err), conflict.options.mode),
				err:     err,
			}
		}
		if err == nil && strings.TrimSpace(id) != "" {
			existingID = id
		}
		// This GET is only an optimization to avoid a redundant PATCH when the
		// existing resource already matches. A failed non-cancellation read
		// must not block the update after the target ID was resolved.
		if err == nil && matches {
			return metadataExistingConflictAlreadyMatches(d, conflict, existingID)
		}
	}

	id, action, err := runMetadataMutation(
		ctx, d, "update",
		func(requestCtx context.Context) (string, error) {
			return conflict.update(requestCtx, existingID)
		},
		conflict.readback,
	)
	if err != nil {
		return metadataConflictOutcome{
			handled: true,
			action:  markMetadataConflictAction(failedMetadataAction(conflict.scope, conflict.locale, conflict.version, "update", existingID, conflict.desired, err), conflict.options.mode),
			err:     err,
		}
	}
	reportMetadataConflictResolution(d, conflict, existingID, "updated in place")
	return metadataConflictOutcome{
		handled: true,
		action:  markMetadataConflictAction(successfulMetadataAction(conflict.scope, conflict.locale, conflict.version, action, id), conflict.options.mode),
	}
}

func markMetadataConflictAction(action ApplyAction, mode shared.IfExistsMode) ApplyAction {
	action.AlreadyExists = true
	action.IfExists = string(mode)
	return action
}

func reportMetadataConflictResolution(d *shared.Deps, conflict metadataCreateConflict, existingID, outcome string) {
	fmt.Fprintf(
		d.IO.ErrOut,
		"%s: %s localization %s for locale %s already exists; %s (--if-exists %s)\n",
		conflict.options.prefix,
		conflict.scope,
		existingID,
		conflict.locale,
		outcome,
		conflict.options.mode,
	)
}

func runMetadataMutation(
	ctx context.Context,
	d *shared.Deps,
	action string,
	mutate func(context.Context) (string, error),
	readback func(context.Context) (string, bool, error),
	ifExistsModes ...shared.IfExistsMode,
) (string, string, error) {
	ifExistsMode := shared.IfExistsFail
	if len(ifExistsModes) > 0 {
		ifExistsMode = ifExistsModes[0]
	}
	var lastMutationErr error
	mutationAttempts := 0
	id, status, err := shared.RunReconciledMutationWith(ctx, d, func(requestCtx context.Context) (string, error) {
		mutationAttempts++
		id, mutationErr := mutate(requestCtx)
		if mutationErr != nil {
			lastMutationErr = mutationErr
		}
		return id, mutationErr
	}, readback)
	if err != nil {
		return "", action, err
	}
	if status == shared.ReconciledMutationRecovered {
		// A duplicate 409 on the first POST identifies an existing resource.
		// After a replay, an earlier ambiguous POST may have created the locale
		// even though its read-backs were temporarily stale; preserve that
		// provenance as a create reconciliation instead of treating it as
		// pre-existing and suppressing its readiness warning.
		if action == "create" && mutationAttempts == 1 && shared.IsIfExistsConflict(lastMutationErr, metadataLocalizationExistsCodes) {
			if ifExistsMode != shared.IfExistsFail {
				if strings.TrimSpace(id) != "" {
					return "", action, &metadataCreateConflictReadBackError{
						cause:          lastMutationErr,
						localizationID: id,
					}
				}
				return "", action, lastMutationErr
			}
			return id, "reconcile-existing", nil
		}
		return id, "reconcile", nil
	}
	return id, action, nil
}

func successfulMetadataAction(scope, locale, version, action, id string) ApplyAction {
	reconciledDuplicate := action == "reconcile-existing"
	if reconciledDuplicate {
		action = "reconcile"
	}
	return ApplyAction{
		Scope:               scope,
		Locale:              locale,
		Version:             version,
		Action:              action,
		Status:              metadataActionStatusSucceeded,
		LocalizationID:      id,
		reconciledDuplicate: reconciledDuplicate,
	}
}

func failedMetadataAction(scope, locale, version, action, id string, desired map[string]string, err error) ApplyAction {
	return ApplyAction{
		Scope:          scope,
		Locale:         locale,
		Version:        version,
		Action:         action,
		Status:         metadataActionStatusFailed,
		LocalizationID: id,
		Error:          err.Error(),
		DesiredFields:  cloneStringMap(desired),
	}
}

func writeMetadataPushFailureArtifact(d *shared.Deps, result PushPlanResult, commandName string) (string, error) {
	failures := make([]ApplyAction, 0, result.Failed)
	for _, action := range result.Actions {
		if action.Status == metadataActionStatusFailed {
			failures = append(failures, action)
		}
	}

	command := strings.TrimSpace(commandName)
	switch command {
	case "apply", "push":
	default:
		command = "push"
	}
	artifact := metadataPushFailureArtifact{
		SchemaVersion: 1,
		Command:       command,
		AppID:         result.AppID,
		AppInfoID:     result.AppInfoID,
		Version:       result.Version,
		VersionID:     result.VersionID,
		Dir:           result.Dir,
		GeneratedAt:   d.Now().UTC().Format(time.RFC3339),
		Failures:      failures,
	}
	data, err := encodeCanonicalJSON(artifact)
	if err != nil {
		return "", err
	}

	path := filepath.Join(
		".asc",
		"reports",
		"metadata-"+command,
		fmt.Sprintf("failures-%d.json", d.Now().UTC().UnixNano()),
	)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := writeFileNoFollow(path, data); err != nil {
		return "", err
	}
	return path, nil
}

func readBackAppInfoLocalization(ctx context.Context, d *shared.Deps, client *asc.Client, appInfoID, locale string, desired map[string]string) (string, bool, error) {
	items, err := fetchAppInfoLocalizations(ctx, d, client, appInfoID)
	if err != nil {
		return "", false, err
	}
	for _, item := range items {
		if strings.TrimSpace(item.Attributes.Locale) != locale {
			continue
		}
		if desired == nil {
			return item.ID, true, nil
		}
		remote := appInfoFields(AppInfoLocalization{
			Name:              item.Attributes.Name,
			Subtitle:          item.Attributes.Subtitle,
			PrivacyPolicyURL:  item.Attributes.PrivacyPolicyURL,
			PrivacyChoicesURL: item.Attributes.PrivacyChoicesURL,
			PrivacyPolicyText: item.Attributes.PrivacyPolicyText,
		})
		return item.ID, metadataFieldsMatch(remote, desired), nil
	}
	return "", false, nil
}

func readBackVersionLocalization(ctx context.Context, d *shared.Deps, client *asc.Client, versionID, locale string, desired map[string]string) (string, bool, error) {
	items, err := fetchVersionLocalizations(ctx, d, client, versionID)
	if err != nil {
		return "", false, err
	}
	for _, item := range items {
		if strings.TrimSpace(item.Attributes.Locale) != locale {
			continue
		}
		if desired == nil {
			return item.ID, true, nil
		}
		remote := versionFields(VersionLocalization{
			Description:     item.Attributes.Description,
			Keywords:        item.Attributes.Keywords,
			MarketingURL:    item.Attributes.MarketingURL,
			PromotionalText: item.Attributes.PromotionalText,
			SupportURL:      item.Attributes.SupportURL,
			WhatsNew:        item.Attributes.WhatsNew,
		})
		return item.ID, metadataFieldsMatch(remote, desired), nil
	}
	return "", false, nil
}

func metadataFieldsMatch(remote, desired map[string]string) bool {
	for field, value := range desired {
		if remote[field] != value {
			return false
		}
	}
	return true
}

func countIntentChanges(fields []string, localSet map[string]string, localClears map[string]struct{}, remote map[string]string) (int, int) {
	adds := 0
	updates := 0
	for _, field := range fields {
		remoteValue, remoteHasField := remote[field]
		if _, cleared := localClears[field]; cleared {
			if remoteHasField {
				updates++
			}
			continue
		}
		localValue, localHasField := localSet[field]
		if !localHasField {
			continue
		}
		switch {
		case !remoteHasField:
			adds++
		case remoteValue != localValue:
			updates++
		}
	}
	return adds, updates
}

// newFirstVersionCheck reports whether versionID is the app's first version
// on its platform: no other App Store version exists on that platform. Apple
// never allows whatsNew on a first version, whatever its state. The answer is
// read once; a failed read returns its error, counts as not first, and is
// tried again on the next call. More than one page counts as not first.
func newFirstVersionCheck(d *shared.Deps, client *asc.Client, appID, versionID, platform string) func(context.Context) (bool, error) {
	var mu sync.Mutex
	known, first := false, false
	return func(ctx context.Context) (bool, error) {
		if strings.TrimSpace(platform) == "" {
			return false, nil
		}
		mu.Lock()
		defer mu.Unlock()
		if known {
			return first, nil
		}
		resp, err := shared.RetryReadWithFreshTimeoutWith(ctx, d, func(requestCtx context.Context) (*asc.AppStoreVersionsResponse, error) {
			return client.GetAppStoreVersions(requestCtx, appID,
				asc.WithAppStoreVersionsPlatforms([]string{platform}),
				asc.Limit[asc.AppStoreVersionsOption](200),
			)
		})
		if err != nil {
			return false, err
		}
		if resp == nil {
			return false, fmt.Errorf("empty app store versions response")
		}
		known = true
		if strings.TrimSpace(resp.Links.Next) != "" {
			return false, nil
		}
		for _, item := range resp.Data {
			if item.ID != versionID {
				return false, nil
			}
		}
		first = true
		return true, nil
	}
}

// isWhatsNewOnlyStateError reports whether App Store Connect refused a write
// only because it set whatsNew: every error is a 409 STATE_ERROR naming
// whatsNew, as Apple answers a PATCH that sets it on an app's first version.
func isWhatsNewOnlyStateError(err error) bool {
	apiErr, ok := errors.AsType[*asc.APIError](err)
	if !ok || apiErr.StatusCode != http.StatusConflict || len(apiErr.Entries) == 0 {
		return false
	}
	for _, entry := range apiErr.Entries {
		if entry.Code != "STATE_ERROR" || !strings.Contains(entry.Detail, "'whatsNew'") {
			return false
		}
	}
	return true
}

// changedSetFields keeps a PATCH limited to values that differ from the
// remote localization. App Store Connect rejects the whole PATCH with 409 when
// it carries a field the current state locks (whatsNew on an app's first
// version, everything but promotionalText on a live version), even when the
// value is unchanged.
func changedSetFields(localSet map[string]string, remote map[string]string) map[string]string {
	changed := make(map[string]string, len(localSet))
	for field, value := range localSet {
		if remoteValue, exists := remote[field]; !exists || remoteValue != value {
			changed[field] = value
		}
	}
	return changed
}

// changedClearFields keeps a PATCH limited to clears present in the plan.
// No-op clears must not piggyback on an unrelated update without confirmation.
func changedClearFields(localClears map[string]struct{}, remote map[string]string) map[string]struct{} {
	if len(localClears) == 0 {
		return nil
	}
	changed := make(map[string]struct{}, len(localClears))
	for field := range localClears {
		if _, exists := remote[field]; exists {
			changed[field] = struct{}{}
		}
	}
	return changed
}

func formatAttemptedFieldMap(orderedFields []string, values map[string]string) string {
	if len(values) == 0 {
		return "none"
	}

	fields := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, field := range orderedFields {
		if _, ok := values[field]; !ok {
			continue
		}
		fields = append(fields, field)
		seen[field] = struct{}{}
	}

	if len(fields) != len(values) {
		extra := make([]string, 0, len(values)-len(fields))
		for field := range values {
			if _, ok := seen[field]; ok {
				continue
			}
			extra = append(extra, field)
		}
		sort.Strings(extra)
		fields = append(fields, extra...)
	}

	return strings.Join(fields, ", ")
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
			_, localClearsField := localValues.clearFields[field]
			if !localHasField && !localClearsField {
				continue
			}

			itemKey := buildPlanKey(scope, version, locale, field)
			switch {
			case localClearsField:
				if !remoteHasField {
					continue
				}
				updates = append(updates, PlanItem{
					Key:     itemKey,
					Scope:   scope,
					Locale:  locale,
					Version: version,
					Field:   field,
					Reason:  "field cleared locally",
					From:    remoteValue,
				})
				localeChanged = true
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
		if counts.list > 0 {
			summary = append(summary, PlanAPICall{
				Operation: "list_localizations",
				Scope:     scope,
				Count:     counts.list,
			})
		}
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

func printPushPlanTable(d *shared.Deps, result PushPlanResult) error {
	fmt.Fprintf(d.IO.Out, "App ID: %s\n", result.AppID)
	fmt.Fprintf(d.IO.Out, "Version: %s\n", result.Version)
	fmt.Fprintf(d.IO.Out, "Dir: %s\n", result.Dir)
	fmt.Fprintf(d.IO.Out, "Dry Run: %t\n\n", result.DryRun)
	if result.Applied {
		fmt.Fprintf(d.IO.Out, "Applied: %t\n\n", result.Applied)
	}
	if len(result.Actions) > 0 || result.Failed > 0 || result.FailureArtifactError != "" {
		fmt.Fprintf(d.IO.Out, "Total: %d\n", result.Total)
		fmt.Fprintf(d.IO.Out, "Succeeded: %d\n", result.Succeeded)
		if result.Skipped > 0 {
			fmt.Fprintf(d.IO.Out, "Skipped: %d\n", result.Skipped)
		}
		fmt.Fprintf(d.IO.Out, "Failed: %d\n", result.Failed)
		if result.FailureArtifactPath != "" {
			fmt.Fprintf(d.IO.Out, "Failure Artifact: %s\n", result.FailureArtifactPath)
		}
		if result.FailureArtifactError != "" {
			fmt.Fprintf(d.IO.Out, "Failure Artifact Error: %s\n", result.FailureArtifactError)
		}
		fmt.Fprintln(d.IO.Out)
	}

	headers := []string{"change", "key", "scope", "locale", "version", "field", "reason", "from", "to"}
	rows := buildPlanRows(result)
	asc.RenderTableTo(d.IO.Out, headers, rows)

	if len(result.APICalls) > 0 {
		fmt.Fprintln(d.IO.Out)
		asc.RenderTableTo(d.IO.Out, []string{"operation", "scope", "count"}, buildAPICallRows(result.APICalls))
	}
	if len(result.Actions) > 0 {
		fmt.Fprintln(d.IO.Out)
		asc.RenderTableTo(d.IO.Out, []string{"scope", "locale", "version", "action", "status", "localizationId", "error"}, buildApplyActionRows(result.Actions))
	}
	return nil
}

func printPushPlanMarkdown(d *shared.Deps, result PushPlanResult) error {
	fmt.Fprintf(d.IO.Out, "**App ID:** %s\n\n", result.AppID)
	fmt.Fprintf(d.IO.Out, "**Version:** %s\n\n", result.Version)
	fmt.Fprintf(d.IO.Out, "**Dir:** %s\n\n", result.Dir)
	fmt.Fprintf(d.IO.Out, "**Dry Run:** %t\n\n", result.DryRun)
	if result.Applied {
		fmt.Fprintf(d.IO.Out, "**Applied:** %t\n\n", result.Applied)
	}
	if len(result.Actions) > 0 || result.Failed > 0 || result.FailureArtifactError != "" {
		fmt.Fprintf(d.IO.Out, "**Total:** %d\n\n", result.Total)
		fmt.Fprintf(d.IO.Out, "**Succeeded:** %d\n\n", result.Succeeded)
		if result.Skipped > 0 {
			fmt.Fprintf(d.IO.Out, "**Skipped:** %d\n\n", result.Skipped)
		}
		fmt.Fprintf(d.IO.Out, "**Failed:** %d\n\n", result.Failed)
		if result.FailureArtifactPath != "" {
			fmt.Fprintf(d.IO.Out, "**Failure Artifact:** %s\n\n", result.FailureArtifactPath)
		}
		if result.FailureArtifactError != "" {
			fmt.Fprintf(d.IO.Out, "**Failure Artifact Error:** %s\n\n", result.FailureArtifactError)
		}
	}

	headers := []string{"change", "key", "scope", "locale", "version", "field", "reason", "from", "to"}
	rows := buildPlanRows(result)
	asc.RenderMarkdownTo(d.IO.Out, headers, rows)

	if len(result.APICalls) > 0 {
		fmt.Fprintln(d.IO.Out)
		asc.RenderMarkdownTo(d.IO.Out, []string{"operation", "scope", "count"}, buildAPICallRows(result.APICalls))
	}
	if len(result.Actions) > 0 {
		fmt.Fprintln(d.IO.Out)
		asc.RenderMarkdownTo(d.IO.Out, []string{"scope", "locale", "version", "action", "status", "localizationId", "error"}, buildApplyActionRows(result.Actions))
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
			action.Status,
			action.LocalizationID,
			sanitizePlanCell(action.Error),
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
