package xcode

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/bitrise-io/go-xcode/xcodeproject/serialized"
	"github.com/google/uuid"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const (
	signingPlanSchemaVersion                      = 1
	signingSettingsMaxBytes                       = 1 << 20
	signingPlanMaxBytes                           = 8 << 20
	signingPlanMaxFiles                           = 4096
	signingPlanMaxMissingOptionalIncludePathBytes = 4096

	signingPlanCommand = "asc xcode signing plan"
)

var (
	signingTeamIDPattern    = regexp.MustCompile(`^[A-Z0-9]{10}$`)
	signingBundleIDPattern  = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)+$`)
	signingReferencePattern = regexp.MustCompile(`\$\(([^):]+)(?::([^)]*))?\)|\$\{([^}:]+)(?::([^}]*))?\}`)
)

// signingInputError marks deterministic manifest or artifact-shape failures
// that the CLI can report as usage errors. Filesystem, parser, and staging
// failures intentionally remain ordinary runtime errors.
type signingInputError struct {
	err error
}

func (e signingInputError) Error() string {
	return e.err.Error()
}

func (e signingInputError) Unwrap() error {
	return e.err
}

func newSigningInputError(err error) error {
	if err == nil {
		return nil
	}
	return signingInputError{err: err}
}

type signingArtifactAliasError struct {
	err error
}

func (e signingArtifactAliasError) Error() string {
	return e.err.Error()
}

func (e signingArtifactAliasError) Unwrap() error {
	return e.err
}

func newSigningArtifactAliasError(err error) error {
	if err == nil {
		return nil
	}
	return signingArtifactAliasError{err: err}
}

const signingUnauthorizedExternalXCConfigMessage = "unauthorized external xcconfig cannot be safely inventoried without --allow-external-xcconfig"

// signingUnauthorizedExternalXCConfigError marks an external xcconfig that
// was discovered but not authorized for reading. Its contents are unknown, so
// the planner cannot safely publish even a blocked artifact. Keep the cause
// available for internal classification while exposing only stable text.
type signingUnauthorizedExternalXCConfigError struct {
	err error
}

func (e signingUnauthorizedExternalXCConfigError) Error() string {
	return signingUnauthorizedExternalXCConfigMessage
}

func (e signingUnauthorizedExternalXCConfigError) Unwrap() error {
	return e.err
}

func newSigningUnauthorizedExternalXCConfigError(err error) error {
	return signingUnauthorizedExternalXCConfigError{err: err}
}

const signingIncompleteInternalXCConfigMessage = "incomplete xcconfig collection cannot be safely inventoried"

// signingIncompleteInternalXCConfigError marks an internal xcconfig that could
// not be fully read or parsed. Unread assignments may name an artifact path,
// so even a blocked plan is unsafe to serialize.
type signingIncompleteInternalXCConfigError struct {
	err error
}

func (e signingIncompleteInternalXCConfigError) Error() string {
	if e.err == nil {
		return signingIncompleteInternalXCConfigMessage
	}
	return fmt.Sprintf("%s: %v", signingIncompleteInternalXCConfigMessage, e.err)
}

func (e signingIncompleteInternalXCConfigError) Unwrap() error {
	return e.err
}

func newSigningIncompleteInternalXCConfigError(err error) error {
	return signingIncompleteInternalXCConfigError{err: err}
}

// signingConditionalEntitlementError marks a conditional entitlement value
// whose reference graph could not be inventoried safely. Such a value cannot
// be represented by a blocked plan because an artifact path may be hidden
// behind the unresolved expression.
type signingConditionalEntitlementError struct {
	err error
}

func (e signingConditionalEntitlementError) Error() string {
	return "conditional CODE_SIGN_ENTITLEMENTS cannot be safely inventoried"
}

func (e signingConditionalEntitlementError) Unwrap() error {
	return e.err
}

func newSigningConditionalEntitlementError(err error) error {
	return signingConditionalEntitlementError{err: err}
}

// NewSigningInputError adapts a deterministic signing-input failure for a
// command boundary. It is primarily useful to keep adapters and tests aligned
// with the same usage classification as the built-in manifest validator.
func NewSigningInputError(err error) error {
	return newSigningInputError(err)
}

// IsSigningInputError reports whether err is a deterministic signing-manifest
// or signing-artifact validation failure suitable for usage classification.
func IsSigningInputError(err error) bool {
	var inputErr signingInputError
	return errors.As(err, &inputErr)
}

// SigningPlanOptions controls generation of a deterministic local Xcode
// signing-settings plan. Paths are operator-selected; no remote input is
// consulted by this workflow.
type SigningPlanOptions struct {
	ProjectPath           string
	SettingsFilePath      string
	ProfilePaths          []string
	Configuration         string
	ExportMethod          string
	SkipTargets           []string
	StateDir              string
	PlanPath              string
	ReceiptPath           string
	AllowExternalXCConfig bool
}

// SigningApplyOptions controls application of a previously generated plan.
type SigningApplyOptions struct {
	PlanPath              string
	AllowExternalXCConfig bool
}

// SigningPlan is the stable JSON artifact consumed by signing apply. Fields
// are intentionally additive: the plan records enough provenance to reject a
// stale or redirected apply before touching the project.
type SigningPlan struct {
	SchemaVersion         int                       `json:"schemaVersion"`
	Command               string                    `json:"command"`
	GeneratedAt           string                    `json:"generatedAt"`
	PlanHash              string                    `json:"planHash"`
	Ready                 bool                      `json:"ready"`
	ProjectPath           string                    `json:"projectPath"`
	SettingsFilePath      string                    `json:"settingsFilePath"`
	PlanPath              string                    `json:"planPath"`
	ReceiptPath           string                    `json:"receiptPath"`
	AllowExternalXCConfig bool                      `json:"allowExternalXCConfig"`
	ProfilePaths          []string                  `json:"profilePaths,omitempty"`
	Configuration         string                    `json:"configuration,omitempty"`
	ExportMethod          string                    `json:"exportMethod,omitempty"`
	SkipTargets           []string                  `json:"skipTargets,omitempty"`
	Inferences            []SigningPlanInference    `json:"inferences,omitempty"`
	ExportOptions         *SigningPlanExportOptions `json:"exportOptions,omitempty"`
	Desired               []SigningPlanTarget       `json:"desired"`
	Files                 []SigningPlanFile         `json:"files"`
	Changes               []SigningSettingChange    `json:"changes"`
	// MissingOptionalIncludes records bounded lexical paths for optional
	// xcconfig includes that were absent during planning. Apply rechecks these
	// assertions before any ordinary write and immediately before publishing a
	// receipt so a late-created source cannot collide with an artifact.
	MissingOptionalIncludes []string `json:"missingOptionalIncludes,omitempty"`
	Blockers                []string `json:"blockers"`
	Warnings                []string `json:"warnings"`
}

// SigningPlanTarget describes the requested target/configuration scope.
type SigningPlanTarget struct {
	Target         string                     `json:"target"`
	Configurations []SigningPlanConfiguration `json:"configurations"`
}

// SigningPlanConfiguration describes normalized desired signing settings.
// A null value means remove a direct assignment where the setting supports
// removal.
type SigningPlanConfiguration struct {
	Name     string               `json:"name"`
	Settings []SigningPlanSetting `json:"settings"`
}

// SigningPlanSetting is one normalized desired build setting.
type SigningPlanSetting struct {
	Key   string  `json:"key"`
	Value *string `json:"value"`
}

// SigningPlanFile records a source file digest bound into the plan.
type SigningPlanFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Source string `json:"source"`
}

// SigningSettingChange records one concrete setting operation.
type SigningSettingChange struct {
	Target        string  `json:"target"`
	Configuration string  `json:"configuration"`
	Setting       string  `json:"setting"`
	Operation     string  `json:"operation"`
	Resolution    string  `json:"resolution"`
	OldValue      *string `json:"oldValue"`
	NewValue      *string `json:"newValue"`
	Path          string  `json:"path"`
	Source        string  `json:"source"`
}

// SigningFileChange binds each written file to its before and after digest in
// the apply receipt without including any signing asset bytes.
type SigningFileChange struct {
	Path         string `json:"path"`
	Source       string `json:"source"`
	BeforeSHA256 string `json:"beforeSha256"`
	AfterSHA256  string `json:"afterSha256"`
}

// SigningApplyResult is written as the receipt after a successful apply.
type SigningApplyResult struct {
	SchemaVersion int                    `json:"schemaVersion"`
	AppliedAt     string                 `json:"appliedAt"`
	Completed     bool                   `json:"completed"`
	PlanHash      string                 `json:"planHash"`
	PlanPath      string                 `json:"planPath"`
	ReceiptPath   string                 `json:"receiptPath"`
	ChangedFiles  []string               `json:"changedFiles"`
	Files         []SigningFileChange    `json:"files"`
	Changes       []SigningSettingChange `json:"changes"`
}

type signingSettingsManifest struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Targets       []signingManifestTarget `json:"targets"`
}

type signingXCConfigAccessError struct {
	path     string
	err      error
	external bool
}

func (e *signingXCConfigAccessError) Error() string {
	if e.external {
		return fmt.Sprintf("external xcconfig %s requires --allow-external-xcconfig: %v", e.path, e.err)
	}
	return fmt.Sprintf("xcconfig %s cannot be read: %v", e.path, e.err)
}

func (e *signingXCConfigAccessError) Unwrap() error {
	return e.err
}

type signingManifestTarget struct {
	Name           string                         `json:"name"`
	Configurations []signingManifestConfiguration `json:"configurations"`
}

type signingManifestConfiguration struct {
	Name     string                     `json:"name"`
	Settings map[string]json.RawMessage `json:"settings"`
}

type signingDesiredSetting struct {
	key   string
	value *string
}

type signingRequest struct {
	target        string
	configuration string
	settings      []signingDesiredSetting
}

type signingCandidate struct {
	configuration *versionConfiguration
	setting       string
	desired       *string
	old           *string
	mode          string
	paths         []string
	resolution    string
	noOp          bool
}

type signingPlanOperation struct {
	SigningSettingChange
	configuration *versionConfiguration
}

type signingPlanBuild struct {
	plan           *SigningPlan
	project        *structuredVersionProject
	operations     []signingPlanOperation
	fileIdentities map[string]string
}

// BuildSigningPlan resolves the requested settings and returns a plan without
// mutating any project, xcconfig, or artifact file.
func BuildSigningPlan(opts SigningPlanOptions) (*SigningPlan, error) {
	built, err := buildSigningPlan(opts)
	if err != nil {
		return nil, err
	}
	return built.plan, nil
}

func buildSigningPlan(opts SigningPlanOptions) (*signingPlanBuild, error) {
	if strings.TrimSpace(opts.ProjectPath) == "" {
		return nil, fmt.Errorf("--project is required")
	}
	var settingsPath string
	var settings *signingSettingsManifest
	if strings.TrimSpace(opts.SettingsFilePath) != "" {
		var pathErr error
		settingsPath, pathErr = canonicalSigningPath(opts.SettingsFilePath, "settings file")
		if pathErr != nil {
			return nil, pathErr
		}
		var readErr error
		settings, readErr = readSigningSettingsManifest(settingsPath)
		if readErr != nil {
			return nil, readErr
		}
	}
	if settings == nil && len(opts.ProfilePaths) == 0 {
		return nil, fmt.Errorf("--settings-file or --profile is required")
	}
	project, err := openSigningStructuredVersionProject(opts.ProjectPath)
	if err != nil {
		return nil, err
	}
	if err := validateSigningProjectFile(project); err != nil {
		return nil, err
	}
	var inference *signingProfileInference
	if len(opts.ProfilePaths) > 0 {
		inference, err = inferSigningSettings(project, opts, settings)
		if err != nil {
			return nil, err
		}
		settings = inference.manifest
	}

	stateDir := opts.StateDir
	if strings.TrimSpace(stateDir) == "" {
		stateDir = filepath.Join(".asc", "xcode", "signing")
	}
	stateDir, err = canonicalSigningPath(stateDir, "state directory")
	if err != nil {
		return nil, err
	}
	planPath := opts.PlanPath
	if strings.TrimSpace(planPath) == "" {
		planPath = filepath.Join(stateDir, "plan.json")
	}
	planPath, err = canonicalSigningPath(planPath, "plan file")
	if err != nil {
		return nil, err
	}
	receiptPath := opts.ReceiptPath
	if strings.TrimSpace(receiptPath) == "" {
		receiptPath = filepath.Join(stateDir, "receipt.json")
	}
	receiptPath, err = canonicalSigningPath(receiptPath, "receipt file")
	if err != nil {
		return nil, err
	}
	if err := validateSigningArtifactPaths(planPath, receiptPath, project.pbxprojPath, settingsPath); err != nil {
		return nil, newSigningInputError(err)
	}
	if settings == nil || len(settings.Targets) == 0 {
		return blockedSigningProfilePlan(opts, project, settingsPath, planPath, receiptPath, inference)
	}

	requests, desired, err := normalizeSigningRequests(settings)
	if err != nil {
		return nil, newSigningInputError(err)
	}
	selectedIDs := make(map[string]bool, len(requests))
	requestedSettings := make(map[string]map[string]bool, len(requests))
	for _, request := range requests {
		configuration, configurationErr := signingConfigurationFor(project, request.target, request.configuration)
		if configurationErr == nil {
			selectedIDs[configuration.id] = true
			if requestedSettings[configuration.id] == nil {
				requestedSettings[configuration.id] = make(map[string]bool)
			}
			for _, setting := range request.settings {
				requestedSettings[configuration.id][setting.key] = true
			}
		}
	}
	plan := &SigningPlan{
		SchemaVersion:         signingPlanSchemaVersion,
		Command:               signingPlanCommand,
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339Nano),
		Ready:                 true,
		ProjectPath:           project.projectPath,
		SettingsFilePath:      settingsPath,
		PlanPath:              planPath,
		ReceiptPath:           receiptPath,
		AllowExternalXCConfig: opts.AllowExternalXCConfig,
		Desired:               desired,
		Blockers:              []string{},
		Warnings:              []string{},
	}
	var inputBlockers []string
	for _, request := range requests {
		configuration, configurationErr := signingConfigurationFor(project, request.target, request.configuration)
		if configurationErr != nil {
			continue
		}
		for _, setting := range request.settings {
			if setting.key != "CODE_SIGN_ENTITLEMENTS" || setting.value == nil {
				continue
			}
			if err := validateSigningEntitlementsPath(project, *setting.value); err != nil {
				inputBlockers = append(inputBlockers, signingSettingBlocker(configuration, setting.key, fmt.Errorf("validate path %q: %w", *setting.value, err)))
			}
		}
	}

	fileConsumers, configFiles, fileIdentities, uncertainConsumers, protectedConfigPaths, blockedExternalPaths, lexicalConfigPaths, unauthorizedExternal, missingOptionalIncludes, err := project.signingXCConfigConsumersWithOptionalMissing(selectedIDs, opts.AllowExternalXCConfig)
	plan.MissingOptionalIncludes = append([]string(nil), missingOptionalIncludes...)
	authorizedProtectedConfigPaths := make([]string, 0)
	for _, protectedPath := range protectedConfigPaths {
		contained := false
		if !signingPathDefinitelyExternal(project, protectedPath) {
			contained = signingPathLexicallyContained(project, protectedPath)
		}
		if !opts.AllowExternalXCConfig && !contained {
			continue
		}
		if validateSigningXCConfigPath(project, protectedPath, opts.AllowExternalXCConfig) == nil ||
			// An explicitly opted-in external path may be missing or otherwise
			// uncollectable, but its prospective parent still must participate in
			// alias checks. The collector has already authorized this path, so a
			// no-follow prospective lookup is within the requested scope.
			(opts.AllowExternalXCConfig && !contained) {
			authorizedProtectedConfigPaths = appendUniqueSigningPaths(authorizedProtectedConfigPaths, protectedPath)
		}
	}
	hasUnauthorizedExternal := func(paths []string) bool {
		return !opts.AllowExternalXCConfig && unauthorizedExternal && len(paths) > 0
	}
	hasIncompleteInternalCollection := func(paths []string) bool {
		return signingHasIncompleteInternalXCConfig(project, paths, opts.AllowExternalXCConfig)
	}
	if err != nil {
		if len(blockedExternalPaths) > 0 {
			inputPaths, externalEntitlementPaths, inputPathBlockers, inputErr := signingProjectInputPaths(project, settingsPath, configFiles, fileIdentities, requests, opts.AllowExternalXCConfig, lexicalConfigPaths)
			if inputErr != nil {
				return nil, inputErr
			}
			inputPaths = appendSigningProfileInputs(inputPaths, inference)
			protectedConfigPaths = appendUniqueSigningPaths(protectedConfigPaths, externalEntitlementPaths...)
			// Direct external entitlement values are not readable through this
			// workflow, but their prospective path still came from the project
			// input and must be checked for physical artifact aliases. This is a
			// no-content rooted-path inspection, not authorization to read the
			// entitlement itself.
			authorizedProtectedConfigPaths = appendUniqueSigningPaths(authorizedProtectedConfigPaths, externalEntitlementPaths...)
			if aliasErr := validateSigningArtifactAliasesWithAuthorizedProtectedPaths(planPath, receiptPath, inputPaths, protectedConfigPaths, authorizedProtectedConfigPaths); aliasErr != nil {
				return nil, aliasErr
			}
			// An unauthorized external xcconfig is not merely an uncertain
			// consumer. Its unread contents may define an entitlement input, so
			// the artifact alias set cannot be complete without reading it. Do
			// not serialize a blocked plan: even a distinct plan path could later
			// be changed to collide with an undiscovered input. Lexical alias
			// failures above remain the more precise, no-read diagnostic.
			if hasUnauthorizedExternal(blockedExternalPaths) {
				return nil, newSigningUnauthorizedExternalXCConfigError(err)
			}
			if hasIncompleteInternalCollection(blockedExternalPaths) {
				return nil, newSigningIncompleteInternalXCConfigError(err)
			}
			plan.Blockers = append(plan.Blockers, fmt.Sprintf("selected xcconfig collection failed: %v", err))
			plan.Blockers = append(plan.Blockers, inputPathBlockers...)
			for _, path := range blockedExternalPaths {
				plan.Blockers = append(plan.Blockers, signingXCConfigCollectionBlocker(project, path, opts.AllowExternalXCConfig))
			}
			recordSigningProfileInference(plan, inference, opts)
			plan.Ready = false
			plan.PlanHash = signingPlanHash(plan)
			return &signingPlanBuild{plan: plan, project: project}, nil
		}
		return nil, err
	}
	inputPaths, externalEntitlementPaths, inputPathBlockers, err := signingProjectInputPaths(project, settingsPath, configFiles, fileIdentities, requests, opts.AllowExternalXCConfig, lexicalConfigPaths)
	if err != nil {
		return nil, err
	}
	inputPaths = appendSigningProfileInputs(inputPaths, inference)
	protectedConfigPaths = appendUniqueSigningPaths(protectedConfigPaths, externalEntitlementPaths...)
	// See the error branch above: an external direct-entitlement path is not
	// content-authorized, but it is an explicitly discovered path whose
	// prospective physical alias must be validated before writing any artifact.
	authorizedProtectedConfigPaths = appendUniqueSigningPaths(authorizedProtectedConfigPaths, externalEntitlementPaths...)
	if err := validateSigningArtifactAliasesWithAuthorizedProtectedPaths(
		planPath,
		receiptPath,
		inputPaths,
		protectedConfigPaths,
		authorizedProtectedConfigPaths,
	); err != nil {
		return nil, err
	}
	if hasUnauthorizedExternal(blockedExternalPaths) {
		// Unselected configurations deliberately do not make collection errors
		// the primary return value. They are still fatal here: without reading
		// an unauthorized source, its contents cannot be inventoried and a
		// blocked plan is unsafe to publish.
		return nil, newSigningUnauthorizedExternalXCConfigError(nil)
	}
	if hasIncompleteInternalCollection(blockedExternalPaths) {
		return nil, newSigningIncompleteInternalXCConfigError(nil)
	}
	inputBlockers = append(inputBlockers, inputPathBlockers...)
	if len(inputBlockers) > 0 {
		plan.Blockers = append(plan.Blockers, inputBlockers...)
		recordSigningProfileInference(plan, inference, opts)
		plan.Ready = false
		plan.PlanHash = signingPlanHash(plan)
		return &signingPlanBuild{plan: plan, project: project}, nil
	}
	for _, path := range externalEntitlementPaths {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("external CODE_SIGN_ENTITLEMENTS input %s cannot be read or authorized by this signing workflow", path))
	}
	for _, path := range blockedExternalPaths {
		plan.Blockers = append(plan.Blockers, signingXCConfigCollectionBlocker(project, path, opts.AllowExternalXCConfig))
	}
	if len(blockedExternalPaths) > 0 || len(externalEntitlementPaths) > 0 {
		recordSigningProfileInference(plan, inference, opts)
		plan.Ready = false
		plan.PlanHash = signingPlanHash(plan)
		return &signingPlanBuild{plan: plan, project: project}, nil
	}

	var candidates []signingCandidate
	for _, request := range requests {
		configuration, err := signingConfigurationFor(project, request.target, request.configuration)
		if err != nil {
			plan.Blockers = append(plan.Blockers, err.Error())
			continue
		}
		for _, setting := range request.settings {
			candidate, blocker, warning := inspectSigningCandidate(
				project,
				configuration,
				setting,
				configFiles,
				fileConsumers,
				fileIdentities,
				requestedSettings,
				uncertainConsumers,
				opts.AllowExternalXCConfig,
				lexicalConfigPaths,
			)
			if warning != "" {
				plan.Warnings = append(plan.Warnings, warning)
			}
			if blocker != "" {
				plan.Blockers = append(plan.Blockers, blocker)
				continue
			}
			// Keep no-op candidates for the staged resolution pass. A setting
			// whose current value is correct can become a real operation after
			// another requested setting changes one of its references.
			if candidate.mode != "" || candidate.noOp {
				candidates = append(candidates, candidate)
			}
		}
	}

	var operations []signingPlanOperation
	var operationBlockers []string
	baselineResolver := newSigningSettingResolver(project, configFiles, opts.AllowExternalXCConfig, lexicalConfigPaths)
	converged := false
	maxIterations := len(candidates) + 1
	for iteration := 0; iteration < maxIterations; iteration++ {
		operations, operationBlockers = buildSigningPlanOperations(
			project,
			candidates,
			configFiles,
			fileIdentities,
			opts.AllowExternalXCConfig,
			lexicalConfigPaths,
		)
		if len(operationBlockers) > 0 {
			break
		}
		stagedProject, stagedResolver, stageErr := stageSigningPlan(
			project,
			operations,
			configFiles,
			fileIdentities,
			opts.AllowExternalXCConfig,
			lexicalConfigPaths,
		)
		if stageErr != nil {
			operationBlockers = append(operationBlockers, fmt.Sprintf("stage signing plan: %v", stageErr))
			break
		}
		reclassified, resolutionBlockers := reclassifySigningNoOps(candidates, project, stagedProject, stagedResolver, baselineResolver)
		if len(resolutionBlockers) > 0 && reclassified == 0 {
			operationBlockers = append(operationBlockers, resolutionBlockers...)
			break
		}
		if reclassified == 0 {
			converged = true
			break
		}
	}
	plan.Blockers = append(plan.Blockers, operationBlockers...)
	if !converged && len(operationBlockers) == 0 {
		plan.Blockers = append(plan.Blockers, "could not resolve signing settings after staged dependency pass")
	}
	plan.Changes = make([]SigningSettingChange, 0, len(operations))
	for _, operation := range operations {
		plan.Changes = append(plan.Changes, operation.SigningSettingChange)
	}

	files := map[string]SigningPlanFile{}
	addFile := func(path, source string) {
		key := signingXCConfigOperationKey(path, fileIdentities)
		if _, exists := files[key]; exists {
			return
		}
		digest, digestErr := signingFileDigest(path)
		if digestErr != nil {
			plan.Blockers = append(plan.Blockers, fmt.Sprintf("digest %s: %v", path, digestErr))
			return
		}
		files[key] = SigningPlanFile{Path: path, SHA256: digest, Source: source}
	}
	addFile(project.pbxprojPath, "pbxproj")
	if settingsPath != "" {
		addFile(settingsPath, "settings")
	}
	if inference != nil {
		for _, path := range inference.paths {
			addFile(path, "profile")
		}
	}
	for _, operation := range operations {
		if operation.Source == "xcconfig" {
			addFile(operation.Path, "xcconfig")
		}
	}
	// Bind every successfully collected xcconfig consumer graph, not only the
	// files this plan rewrites or the selected configuration resolves through.
	// Consumer analysis uses unselected graphs to decide whether a source is
	// shared and safe to rewrite, so changing any consulted input must stale the
	// plan before commit.
	resolutionInputs := make([]string, 0)
	for _, paths := range configFiles {
		resolutionInputs = append(resolutionInputs, paths...)
	}
	sort.Strings(resolutionInputs)
	for _, path := range resolutionInputs {
		addFile(path, "xcconfig")
	}
	for _, file := range files {
		plan.Files = append(plan.Files, file)
	}
	sort.Slice(plan.Files, func(left, right int) bool { return plan.Files[left].Path < plan.Files[right].Path })
	if len(plan.Files) > signingPlanMaxFiles {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("signing plan source graph contains %d files, exceeding the limit of %d", len(plan.Files), signingPlanMaxFiles))
	}
	recordSigningProfileInference(plan, inference, opts)
	plan.Ready = len(plan.Blockers) == 0
	plan.PlanHash = signingPlanHash(plan)

	return &signingPlanBuild{plan: plan, project: project, operations: operations, fileIdentities: fileIdentities}, nil
}

func canonicalSigningPath(path, label string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%s path is empty", label)
	}
	if strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("%s path contains NUL", label)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s path: %w", label, err)
	}
	return filepath.Clean(absolute), nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValueForDuplicateKeys(decoder); err != nil {
		return err
	}
	return nil
}

func scanJSONValueForDuplicateKeys(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON object key is not a string")
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate JSON object key %q", key)
			}
			seen[key] = struct{}{}
			if err := scanJSONValueForDuplicateKeys(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValueForDuplicateKeys(decoder); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}

	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	expectedClosing := json.Delim('}')
	if delimiter == '[' {
		expectedClosing = ']'
	}
	if closing != expectedClosing {
		return fmt.Errorf("unexpected JSON delimiter %q", closing)
	}
	return nil
}

func readSigningSettingsManifest(path string) (*signingSettingsManifest, error) {
	data, err := readSigningRegularFile(path, signingSettingsMaxBytes)
	if err != nil {
		return nil, fmt.Errorf("read settings file %s: %w", path, err)
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return nil, newSigningInputError(fmt.Errorf("decode settings file %s: %w", path, err))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest signingSettingsManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, newSigningInputError(fmt.Errorf("decode settings file %s: %w", path, err))
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, newSigningInputError(fmt.Errorf("decode settings file %s: multiple JSON values", path))
		}
		return nil, newSigningInputError(fmt.Errorf("decode settings file %s: %w", path, err))
	}
	if manifest.SchemaVersion != signingPlanSchemaVersion {
		return nil, newSigningInputError(fmt.Errorf("settings file schemaVersion must be %d", signingPlanSchemaVersion))
	}
	if len(manifest.Targets) == 0 {
		return nil, newSigningInputError(fmt.Errorf("settings file targets must not be empty"))
	}
	return &manifest, nil
}

func normalizeSigningRequests(manifest *signingSettingsManifest) ([]signingRequest, []SigningPlanTarget, error) {
	requests := make([]signingRequest, 0)
	desired := make([]SigningPlanTarget, 0, len(manifest.Targets))
	seenTargets := make(map[string]bool)
	for _, target := range manifest.Targets {
		name := strings.TrimSpace(target.Name)
		if err := validateSigningName(name, "target"); err != nil {
			return nil, nil, err
		}
		if seenTargets[name] {
			return nil, nil, fmt.Errorf("settings file contains duplicate target %q", name)
		}
		seenTargets[name] = true
		if len(target.Configurations) == 0 {
			return nil, nil, fmt.Errorf("target %q configurations must not be empty", name)
		}
		seenConfigurations := make(map[string]bool)
		planTarget := SigningPlanTarget{Target: name}
		for _, configuration := range target.Configurations {
			configurationName := strings.TrimSpace(configuration.Name)
			if err := validateSigningName(configurationName, "configuration"); err != nil {
				return nil, nil, fmt.Errorf("target %q: %w", name, err)
			}
			if seenConfigurations[configurationName] {
				return nil, nil, fmt.Errorf("target %q contains duplicate configuration %q", name, configurationName)
			}
			seenConfigurations[configurationName] = true
			if len(configuration.Settings) == 0 {
				return nil, nil, fmt.Errorf("target %q configuration %q settings must not be empty", name, configurationName)
			}
			settings, err := normalizeSigningSettings(configuration.Settings)
			if err != nil {
				return nil, nil, fmt.Errorf("target %q configuration %q: %w", name, configurationName, err)
			}
			request := signingRequest{target: name, configuration: configurationName, settings: settings}
			requests = append(requests, request)
			planConfiguration := SigningPlanConfiguration{Name: configurationName}
			for _, setting := range settings {
				planConfiguration.Settings = append(planConfiguration.Settings, SigningPlanSetting{Key: setting.key, Value: cloneSigningString(setting.value)})
			}
			planTarget.Configurations = append(planTarget.Configurations, planConfiguration)
		}
		desired = append(desired, planTarget)
	}
	sort.Slice(desired, func(left, right int) bool { return desired[left].Target < desired[right].Target })
	for index := range desired {
		sort.Slice(desired[index].Configurations, func(left, right int) bool {
			return desired[index].Configurations[left].Name < desired[index].Configurations[right].Name
		})
	}
	return requests, desired, nil
}

func normalizeSigningSettings(raw map[string]json.RawMessage) ([]signingDesiredSetting, error) {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	settings := make([]signingDesiredSetting, 0, len(keys))
	for _, key := range keys {
		if !allowedSigningSetting(key) {
			return nil, fmt.Errorf("unsupported signing setting %q", key)
		}
		value, err := normalizeSigningValue(key, raw[key])
		if err != nil {
			return nil, err
		}
		settings = append(settings, signingDesiredSetting{key: key, value: value})
	}
	return settings, nil
}

func normalizeSigningValue(key string, raw json.RawMessage) (*string, error) {
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		if !signingSettingAllowsRemoval(key) {
			return nil, fmt.Errorf("%s does not support null removal", key)
		}
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, fmt.Errorf("%s must be a string or null", key)
	}
	if err := validateSigningStaticValue(key, value); err != nil {
		return nil, err
	}
	value = strings.TrimSpace(value)
	if value == "" && (key == "CODE_SIGN_STYLE" || key == "DEVELOPMENT_TEAM" || key == "PRODUCT_BUNDLE_IDENTIFIER") {
		return nil, fmt.Errorf("%s must not be empty", key)
	}
	switch key {
	case "CODE_SIGN_STYLE":
		switch strings.ToLower(value) {
		case "automatic":
			value = "Automatic"
		case "manual":
			value = "Manual"
		default:
			return nil, fmt.Errorf("CODE_SIGN_STYLE must be automatic or manual")
		}
	case "DEVELOPMENT_TEAM":
		value = strings.ToUpper(value)
		if !signingTeamIDPattern.MatchString(value) {
			return nil, fmt.Errorf("DEVELOPMENT_TEAM must be a 10-character alphanumeric team ID")
		}
	case "PROVISIONING_PROFILE":
		parsed, err := uuid.Parse(value)
		if err != nil || parsed == uuid.Nil {
			return nil, fmt.Errorf("PROVISIONING_PROFILE must be a UUID")
		}
		value = parsed.String()
	case "PRODUCT_BUNDLE_IDENTIFIER":
		if !signingBundleIDPattern.MatchString(value) {
			return nil, fmt.Errorf("PRODUCT_BUNDLE_IDENTIFIER must be a reverse-DNS bundle identifier")
		}
	case "CODE_SIGN_ENTITLEMENTS":
		if err := validateSigningRelativePath(value); err != nil {
			return nil, fmt.Errorf("CODE_SIGN_ENTITLEMENTS: %w", err)
		}
	}
	return stringPtr(value), nil
}

func validateSigningStaticValue(key, value string) error {
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%s must not contain NUL", key)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must not contain a newline", key)
	}
	if strings.Contains(value, "//") || strings.Contains(value, "/*") || strings.Contains(value, "*/") {
		return fmt.Errorf("%s must not contain comment syntax", key)
	}
	if strings.Contains(value, "$(") || strings.Contains(value, "${") {
		return fmt.Errorf("%s must be a static value without build-setting references", key)
	}
	return nil
}

func validateSigningRelativePath(value string) error {
	if value == "" {
		return fmt.Errorf("path must not be empty")
	}
	if pathpkg.IsAbs(value) || strings.HasPrefix(value, "~") || strings.Contains(value, "\\") || isWindowsDrivePath(value) {
		return fmt.Errorf("path must be relative and use POSIX separators")
	}
	clean := pathpkg.Clean(value)
	if clean == "." || clean != value {
		return fmt.Errorf("path must not contain traversal or redundant components")
	}
	for _, component := range strings.Split(value, "/") {
		if component == ".." || component == "" {
			return fmt.Errorf("path must stay within the project")
		}
	}
	return nil
}

func isWindowsDrivePath(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':'
}

func validateSigningName(value, label string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", label)
	}
	if strings.ContainsRune(value, '\x00') || strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must not contain control characters", label)
	}
	return nil
}

func allowedSigningSetting(key string) bool {
	switch key {
	case "CODE_SIGN_STYLE", "DEVELOPMENT_TEAM", "CODE_SIGN_IDENTITY", "PROVISIONING_PROFILE_SPECIFIER", "PROVISIONING_PROFILE", "CODE_SIGN_ENTITLEMENTS", "PRODUCT_BUNDLE_IDENTIFIER":
		return true
	default:
		return false
	}
}

func signingSettingAllowsRemoval(key string) bool {
	switch key {
	case "CODE_SIGN_IDENTITY", "PROVISIONING_PROFILE_SPECIFIER", "PROVISIONING_PROFILE", "CODE_SIGN_ENTITLEMENTS":
		return true
	default:
		return false
	}
}

func signingConfigurationFor(project *structuredVersionProject, target, configuration string) (*versionConfiguration, error) {
	targetMatches := 0
	for _, candidate := range project.project.Proj.Targets {
		if candidate.Name == target {
			targetMatches++
		}
	}
	if targetMatches > 1 {
		return nil, fmt.Errorf("project contains multiple targets named %q", target)
	}
	var match *versionConfiguration
	for _, candidate := range project.configurations {
		if !candidate.projectLevel && candidate.target == target && candidate.name == configuration {
			if match != nil {
				return nil, fmt.Errorf("target %q contains multiple configurations named %q", target, configuration)
			}
			match = candidate
		}
	}
	if match != nil {
		return match, nil
	}
	return nil, fmt.Errorf("configuration %q not found for target %q", configuration, target)
}

func inspectSigningCandidate(
	project *structuredVersionProject,
	configuration *versionConfiguration,
	setting signingDesiredSetting,
	configFiles map[string][]string,
	fileConsumers map[string]map[string]bool,
	fileIdentities map[string]string,
	requestedSettings map[string]map[string]bool,
	uncertainConsumers bool,
	allowExternal bool,
	lexicalConfigPaths map[string][]string,
) (signingCandidate, string, string) {
	candidate := signingCandidate{configuration: configuration, setting: setting.key, desired: cloneSigningString(setting.value)}
	if !signingConfigurationSourcesAuthorized(project, configuration, configFiles) {
		return candidate, signingSettingBlocker(configuration, setting.key, errors.New("configuration inherits from an xcconfig that was not authorized or could not be read")), ""
	}
	resolver := newSigningSettingResolver(project, configFiles, allowExternal, lexicalConfigPaths)
	if setting.key == "CODE_SIGN_ENTITLEMENTS" && setting.value != nil {
		if err := validateSigningEntitlementsPath(project, *setting.value); err != nil {
			return candidate, signingSettingBlocker(configuration, setting.key, fmt.Errorf("validate path %q: %w", *setting.value, err)), ""
		}
	}
	keys := matchingBuildSettingKeys(configuration.buildSettings, setting.key)
	if len(keys) > 0 {
		old, _, err := resolver.resolveSetting(configuration, setting.key)
		if err != nil {
			return candidate, signingSettingBlocker(configuration, setting.key, err), ""
		}
		candidate.old = stringPtr(old)
		candidate.resolution = "direct"
		if signingValuesEqual(setting.value, candidate.old) {
			// A direct assignment that still defers to $(inherited) keeps a live
			// dependency on the xcconfig supplying that value. Retain it as a
			// no-op consumer so shared-file resolution can see the disagreement
			// when a sibling configuration wants a different value in the same
			// file; otherwise rewriting that file would silently change this
			// configuration's effective value.
			if setting.value != nil && signingDirectValueInherits(configuration, setting.key) {
				assignmentFiles, assignmentErr := xcconfigFilesDefiningWithReader(configFiles[configuration.id], setting.key, resolver.readXCConfig)
				if assignmentErr != nil {
					return candidate, signingSettingBlocker(configuration, setting.key, assignmentErr), ""
				}
				if len(assignmentFiles) > 0 {
					candidate.mode = "xcconfig"
					candidate.paths = append(candidate.paths, assignmentFiles...)
				}
			}
			candidate.noOp = true
			return candidate, "", ""
		}
		candidate.mode = "pbxproj"
		return candidate, "", ""
	}

	assignmentFiles, err := xcconfigFilesDefiningWithReader(configFiles[configuration.id], setting.key, resolver.readXCConfig)
	if err != nil {
		return candidate, signingSettingBlocker(configuration, setting.key, err), ""
	}
	old, _, resolveErr := resolver.resolveSetting(configuration, setting.key)
	if resolveErr == nil {
		candidate.old = stringPtr(old)
		if len(assignmentFiles) > 0 {
			candidate.resolution = "xcconfig"
		} else {
			candidate.resolution = "inherited"
		}
	} else if !errors.Is(resolveErr, errVersionSettingNotFound) {
		return candidate, signingSettingBlocker(configuration, setting.key, resolveErr), ""
	} else {
		candidate.resolution = "missing"
	}
	if signingValuesEqual(setting.value, candidate.old) {
		if len(assignmentFiles) > 0 && setting.value != nil {
			candidate.mode = "xcconfig"
			candidate.paths = append(candidate.paths, assignmentFiles...)
		}
		candidate.noOp = true
		return candidate, "", ""
	}

	if setting.value == nil {
		return candidate, fmt.Sprintf("target %q configuration %q cannot remove inherited %s; only a direct project assignment can be removed", configuration.target, configuration.name, setting.key), ""
	}
	if len(assignmentFiles) > 0 {
		if uncertainConsumers {
			candidate.mode = "pbxproj"
			return candidate, "", "xcconfig consumer scope is uncertain; using a target-level override for " + setting.key
		}
		if !consumersAuthorizeSetting(assignmentFiles, fileConsumers, fileIdentities, requestedSettings, setting.key) {
			candidate.mode = "pbxproj"
			return candidate, "", "shared xcconfig is consumed by a configuration that did not request " + setting.key + "; using a target-level override"
		}
		warning := ""
		for _, path := range assignmentFiles {
			if err := project.checkXCConfigWritable(path, allowExternal); err != nil {
				return candidate, fmt.Sprintf("target %q configuration %q cannot update %s in xcconfig %s: %v", configuration.target, configuration.name, setting.key, path, err), ""
			}
			if err := validateSigningXCConfigPath(project, path, allowExternal); err != nil {
				return candidate, fmt.Sprintf("target %q configuration %q cannot update xcconfig %s: %v", configuration.target, configuration.name, path, err), ""
			}
			if allowExternal && !signingPathContained(project, path) {
				if warning == "" {
					warning = fmt.Sprintf("external xcconfig %s is authorized for %s", path, setting.key)
				}
			}
		}
		candidate.mode = "xcconfig"
		candidate.paths = append(candidate.paths, assignmentFiles...)
		return candidate, "", warning
	}

	// Project-level inheritance is deliberately shadowed at the selected
	// target/configuration. This avoids widening a change to other targets.
	candidate.mode = "pbxproj"
	return candidate, "", ""
}

// signingDirectValueInherits reports whether a direct project assignment for
// setting still defers to a lower-level value through $(inherited). Such a
// configuration keeps depending on the xcconfig that supplies the inherited
// value even when its resolved value already matches the requested one.
func signingDirectValueInherits(configuration *versionConfiguration, setting string) bool {
	for _, key := range matchingBuildSettingKeys(configuration.buildSettings, setting) {
		switch value := configuration.buildSettings[key].(type) {
		case string:
			if signingValueInherits(value) {
				return true
			}
		case []any:
			for _, element := range value {
				if text, ok := element.(string); ok && signingValueInherits(text) {
					return true
				}
			}
		}
	}
	return false
}

func signingValueInherits(value string) bool {
	return strings.Contains(value, "$(inherited)") || strings.Contains(value, "${inherited}")
}

func signingConfigurationSourcesAuthorized(
	project *structuredVersionProject,
	configuration *versionConfiguration,
	configFiles map[string][]string,
) bool {
	for current := configuration; current != nil; {
		if current.baseReferenceID != "" {
			if _, ok := configFiles[current.id]; !ok {
				return false
			}
		}
		if current.projectLevel {
			break
		}
		current = project.projectConfiguration(current.name)
	}
	return true
}

func signingConfigurationDefinesUnconditionalEntitlement(
	project *structuredVersionProject,
	configuration *versionConfiguration,
	configFiles map[string][]string,
	resolver *signingSettingResolver,
) bool {
	value, definesUnconditional := configuration.buildSettings["CODE_SIGN_ENTITLEMENTS"].(string)
	if definesUnconditional && signingValueInherits(value) {
		definesUnconditional = false
	}
	if !definesUnconditional && signingConfigurationSourcesAuthorized(project, configuration, configFiles) {
		if files := configFiles[configuration.id]; len(files) > 0 {
			definesUnconditional = signingConfigurationXCConfigHasLiveUnconditionalEntitlementOverride(configuration, files, resolver)
		}
	}
	return definesUnconditional
}

func signingConfigurationDefinesUnconditionalPBXEntitlement(configuration *versionConfiguration) bool {
	value, ok := configuration.buildSettings["CODE_SIGN_ENTITLEMENTS"].(string)
	return ok && !signingValueInherits(value)
}

// signingConfigurationXCConfigHasLiveUnconditionalEntitlementOverride keeps
// project fallback protection only when the target xcconfig's effective
// unconditional slot is a literal replacement. The sentinel probe rejects
// effective inherited values, while the candidate traversal excludes
// conditional-only assignments without replaying the xcconfig graph in a
// second resolver.
func signingConfigurationXCConfigHasLiveUnconditionalEntitlementOverride(
	configuration *versionConfiguration,
	files []string,
	resolver *signingSettingResolver,
) bool {
	if len(files) == 0 || resolver.xcconfigDependsOnFallback(configuration, files[0], "CODE_SIGN_ENTITLEMENTS") {
		return false
	}
	candidates, _, _, err := signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
		configuration,
		files,
		resolver,
		xcconfigResolvedValue{},
	)
	if err != nil {
		return false
	}
	for _, filePath := range files {
		data, readErr := resolver.readXCConfig(filePath)
		if readErr != nil {
			return false
		}
		document, parseErr := parseXCConfig(data)
		if parseErr != nil {
			return false
		}
		for _, assignment := range document.assignments {
			if assignment.baseKey != "CODE_SIGN_ENTITLEMENTS" ||
				assignment.key != "CODE_SIGN_ENTITLEMENTS" ||
				!candidates[normalizeSigningLexicalPath(filePath)][assignment.lineIndex] {
				continue
			}
			if signingValueInherits(assignment.value) {
				continue
			}
			if assignment.operator == "=" {
				return true
			}
		}
	}
	return false
}

// signingSettingResolver is the signing workflow's authorization-aware
// counterpart to structuredVersionProject.resolveSetting. Its xcconfig reads
// are limited to paths successfully collected for this plan; this keeps a
// later setting-resolution pass from reopening an unselected or redirected
// external include through the generic resolver.
type signingSettingResolver struct {
	project     *structuredVersionProject
	configFiles map[string][]string
	// lexicalConfigPaths retains every path the collector observed for each
	// configuration, including a directly missing optional include. It is used
	// only to authorize the later no-follow existence check; reads still
	// require membership in authorizedPath (the successfully collected files).
	lexicalConfigPaths map[string][]string
	authorizedPath     map[string]bool
	allowExternal      bool
	// stagedXCConfig contains private bytes for xcconfigs changed by the
	// current planning pass. The map is keyed by normalized lexical path;
	// source existence and authorization remain bound to the collected files.
	stagedXCConfig map[string][]byte
}

func newSigningSettingResolver(project *structuredVersionProject, configFiles map[string][]string, allowExternal bool, lexicalConfigPaths map[string][]string) *signingSettingResolver {
	resolver := &signingSettingResolver{
		project:            project,
		configFiles:        configFiles,
		lexicalConfigPaths: lexicalConfigPaths,
		authorizedPath:     make(map[string]bool),
		allowExternal:      allowExternal,
	}
	for _, paths := range configFiles {
		for _, path := range paths {
			resolver.authorizedPath[normalizeSigningLexicalPath(path)] = true
		}
	}
	return resolver
}

func (resolver *signingSettingResolver) readXCConfig(path string) ([]byte, error) {
	absolute := normalizeSigningLexicalPath(path)
	if !resolver.authorizedPath[normalizeSigningLexicalPath(absolute)] {
		return nil, fmt.Errorf("xcconfig %s was not collected for this signing plan", absolute)
	}
	if err := resolver.authorizeXCConfigPath(absolute); err != nil {
		return nil, err
	}
	if resolver.stagedXCConfig != nil {
		if data, ok := resolver.stagedXCConfig[absolute]; ok {
			return append([]byte(nil), data...), nil
		}
	}
	return signingXCConfigReadFileFn(absolute, signingPlanMaxBytes)
}

// configurationXCConfigPath authorizes a path before any case-volume or
// identity probe, then resolves it only against files collected for the
// supplied configuration. A path collected for a different configuration is
// never readable through this boundary.
func (resolver *signingSettingResolver) configurationXCConfigPath(
	configuration *versionConfiguration,
	path string,
) (string, bool, error) {
	absolute := normalizeSigningLexicalPath(path)
	if err := resolver.authorizeXCConfigPath(absolute); err != nil {
		return absolute, false, err
	}
	if configuration == nil {
		return absolute, false, nil
	}
	for _, collected := range resolver.configFiles[configuration.id] {
		collected = normalizeSigningLexicalPath(collected)
		if collected == absolute {
			return collected, true, nil
		}
	}
	for _, collected := range resolver.configFiles[configuration.id] {
		collected = normalizeSigningLexicalPath(collected)
		if !signingPathCaseEquivalent(absolute, collected) {
			continue
		}
		candidateInfo, candidateErr := signingXCConfigIdentityFn(absolute)
		collectedInfo, collectedErr := signingXCConfigIdentityFn(collected)
		if candidateErr == nil && collectedErr == nil && candidateInfo != nil && collectedInfo != nil && os.SameFile(candidateInfo, collectedInfo) {
			return collected, true, nil
		}
	}
	return absolute, false, nil
}

func (resolver *signingSettingResolver) configurationLexicallyObservedPath(configuration *versionConfiguration, path string) bool {
	if configuration == nil {
		return false
	}
	key := normalizeSigningLexicalPath(path)
	for _, observed := range resolver.lexicalConfigPaths[configuration.id] {
		if normalizeSigningLexicalPath(observed) == key {
			return true
		}
	}
	return false
}

func (resolver *signingSettingResolver) statXCConfigFor(configuration *versionConfiguration, path string) (os.FileInfo, error) {
	absolute := normalizeSigningLexicalPath(path)
	canonical, collected, err := resolver.configurationXCConfigPath(configuration, absolute)
	if err != nil {
		return nil, err
	}
	if !collected {
		if !resolver.configurationLexicallyObservedPath(configuration, absolute) {
			return nil, fmt.Errorf("xcconfig %s was not collected for this signing plan", absolute)
		}
		info, err := signingXCConfigStatFileFn(absolute)
		if err != nil {
			return nil, err
		}
		if info != nil {
			return nil, fmt.Errorf("xcconfig %s appeared after configuration collection", absolute)
		}
		return nil, os.ErrNotExist
	}
	return signingXCConfigStatFileFn(canonical)
}

func (resolver *signingSettingResolver) readXCConfigFor(configuration *versionConfiguration, path string) ([]byte, error) {
	canonical, collected, err := resolver.configurationXCConfigPath(configuration, path)
	if err != nil {
		return nil, err
	}
	if !collected {
		return nil, fmt.Errorf("xcconfig %s was not collected for this configuration", normalizeSigningLexicalPath(path))
	}
	return resolver.readXCConfig(canonical)
}

func (resolver *signingSettingResolver) authorizeXCConfigPath(path string) error {
	path = normalizeSigningLexicalPath(path)
	if !resolver.allowExternal {
		root := normalizeSigningLexicalPath(resolver.project.rootDir)
		if !signingNormalizedPathContained(root, path) {
			// A case-folded prefix is only a candidate for the platform-aware
			// containment check. Reject clearly external paths first so an
			// unauthorized Darwin/Windows path cannot trigger case-volume
			// metadata probes.
			if !signingPathCaseFoldedPrefixContained(root, path) || !signingPathLexicallyContained(resolver.project, path) {
				return fmt.Errorf("external xcconfig %s requires --allow-external-xcconfig", path)
			}
		}
	}
	return validateSigningXCConfigPath(resolver.project, path, resolver.allowExternal)
}

func (resolver *signingSettingResolver) resolveSetting(configuration *versionConfiguration, setting string) (string, string, error) {
	return resolver.resolveSettingWithContext(configuration, configuration, setting)
}

// resolveSettingWithContext locates a value in configuration while expanding
// references in expansionConfiguration. A target configuration can inherit a
// value from its project-level configuration, but Xcode evaluates references
// in that inherited value against the target's effective settings.
func (resolver *signingSettingResolver) resolveSettingWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	setting string,
) (string, string, error) {
	value, ok, err := directBuildSetting(configuration.buildSettings, setting)
	if err != nil {
		return "", "", err
	}
	if ok {
		return resolver.expandDirectSettingWithContext(configuration, expansionConfiguration, setting, value, map[string]bool{setting: true})
	}
	if configuration.baseReferenceID != "" {
		path, err := resolver.project.fileReferencePath(configuration.baseReferenceID)
		if err != nil {
			return "", "", err
		}
		resolved, err := resolver.resolveConfigurationXCConfigWithContext(configuration, expansionConfiguration, path, setting)
		if err != nil {
			return "", "", err
		}
		if resolved.found {
			value, _, err := resolver.expandSettingReferences(expansionConfiguration, resolved.value, map[string]bool{setting: true})
			return value, resolved.path, err
		}
	}
	if !configuration.projectLevel {
		if projectConfiguration := resolver.project.projectConfiguration(configuration.name); projectConfiguration != nil {
			return resolver.resolveSettingWithContext(projectConfiguration, expansionConfiguration, setting)
		}
	}
	return "", "", fmt.Errorf("%s: %w", setting, errVersionSettingNotFound)
}

func (resolver *signingSettingResolver) expandDirectSettingWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	setting, value string,
	stack map[string]bool,
) (string, string, error) {
	return resolver.expandDirectAssignmentWithContext(configuration, expansionConfiguration, setting, setting, value, stack)
}

// expandDirectAssignmentWithContext expands one specific assignment of setting.
// assignmentKey names the build-setting key the value was written under, so a
// conditional assignment such as CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] stays
// distinguishable from the unconditional CODE_SIGN_ENTITLEMENTS slot it
// inherits from. A caller holding an already-effective value, or one that
// cannot name the originating slot, passes setting itself.
func (resolver *signingSettingResolver) expandDirectAssignmentWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	setting, assignmentKey, value string,
	stack map[string]bool,
) (string, string, error) {
	return resolver.expandDirectAssignmentWithSourceContext(configuration, expansionConfiguration, setting, assignmentKey, value, stack, false)
}

func (resolver *signingSettingResolver) expandDirectAssignmentWithSourceContext(
	configuration, expansionConfiguration *versionConfiguration,
	setting, assignmentKey, value string,
	stack map[string]bool,
	xcconfigSource bool,
) (string, string, error) {
	if strings.Contains(value, "$(inherited)") || strings.Contains(value, "${inherited}") {
		inherited, err := resolver.resolveInheritedSettingValue(configuration, expansionConfiguration, setting, assignmentKey, value, stack, xcconfigSource)
		if err != nil {
			return "", "", fmt.Errorf("resolve inherited %s: %w", setting, err)
		}
		value = strings.ReplaceAll(value, "$(inherited)", inherited)
		value = strings.ReplaceAll(value, "${inherited}", inherited)
	}
	return resolver.expandSettingReferences(expansionConfiguration, value, stack)
}

func (resolver *signingSettingResolver) resolveInheritedSettingValue(
	configuration, expansionConfiguration *versionConfiguration,
	setting, assignmentKey, currentValue string,
	stack map[string]bool,
	xcconfigSource bool,
) (string, error) {
	// Xcode resolves $(inherited) to the value the setting holds at the next
	// level up for the assignment being expanded. A conditional PBX assignment
	// such as CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(inherited)Suffix
	// therefore composes through the same object's unconditional
	// CODE_SIGN_ENTITLEMENTS, which only then falls through to the xcconfig and
	// project layers.
	//
	// A conditional assignmentKey names that unconditional slot directly, so
	// identical expression text at different conditions no longer collapses into
	// one resolution. The text inequality stays as the conservative fallback for
	// a caller that cannot name the originating slot; without a key it is the
	// only available guard against recursing on the assignment being expanded.
	// Recursion terminates either way because the nested expansion names the
	// unconditional slot and passes its own text as currentValue.
	if !xcconfigSource {
		if raw, ok := configuration.buildSettings[setting].(string); ok &&
			(assignmentKey != setting || strings.TrimSpace(raw) != strings.TrimSpace(currentValue)) {
			expanded, _, err := resolver.expandDirectAssignmentWithContext(configuration, expansionConfiguration, setting, setting, raw, stack)
			if err != nil {
				return "", err
			}
			return expanded, nil
		}
	}
	inherited, err := resolver.resolveLowerSettingWithContext(configuration, expansionConfiguration, setting)
	if err == nil {
		return inherited, nil
	}
	if errors.Is(err, errVersionSettingNotFound) {
		if implicit, ok := signingImplicitSettingValue(resolver.project, expansionConfiguration, setting); ok {
			return implicit, nil
		}
	}
	if fallback := resolver.project.projectConfiguration(configuration.name); fallback != nil && fallback != configuration {
		for _, key := range matchingBuildSettingKeys(fallback.buildSettings, setting) {
			literal, ok := fallback.buildSettings[key].(string)
			if ok && strings.TrimSpace(literal) != "" && !strings.Contains(literal, "$(") && !strings.Contains(literal, "${") {
				return literal, nil
			}
		}
	}
	return "", err
}

func (resolver *signingSettingResolver) resolveLowerSettingWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	setting string,
) (string, error) {
	if configuration.baseReferenceID != "" {
		path, err := resolver.project.fileReferencePath(configuration.baseReferenceID)
		if err != nil {
			return "", err
		}
		resolved, err := resolver.resolveConfigurationXCConfigWithContext(configuration, expansionConfiguration, path, setting)
		if err != nil {
			return "", err
		}
		if resolved.found {
			value, _, err := resolver.expandSettingReferences(expansionConfiguration, resolved.value, map[string]bool{setting: true})
			return value, err
		}
	}
	if !configuration.projectLevel {
		if fallback := resolver.project.projectConfiguration(configuration.name); fallback != nil {
			value, _, err := resolver.resolveSettingWithContext(fallback, expansionConfiguration, setting)
			return value, err
		}
	}
	return "", fmt.Errorf("%s: %w", setting, errVersionSettingNotFound)
}

func (resolver *signingSettingResolver) resolveSettingReference(configuration *versionConfiguration, setting string, stack map[string]bool) (string, string, error) {
	return resolver.resolveSettingReferenceWithContext(configuration, configuration, setting, stack)
}

func (resolver *signingSettingResolver) resolveSettingReferenceWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	setting string,
	stack map[string]bool,
) (string, string, error) {
	value, ok, err := directBuildSetting(configuration.buildSettings, setting)
	if err != nil {
		return "", "", err
	}
	if ok {
		return resolver.expandDirectSettingWithContext(configuration, expansionConfiguration, setting, value, stack)
	}
	if configuration.baseReferenceID != "" {
		path, err := resolver.project.fileReferencePath(configuration.baseReferenceID)
		if err != nil {
			return "", "", err
		}
		resolved, err := resolver.resolveConfigurationXCConfigWithContext(configuration, expansionConfiguration, path, setting)
		if err != nil {
			return "", "", err
		}
		if resolved.found {
			value, _, err := resolver.expandSettingReferences(expansionConfiguration, resolved.value, stack)
			return value, resolved.path, err
		}
	}
	if !configuration.projectLevel {
		if fallback := resolver.project.projectConfiguration(configuration.name); fallback != nil {
			return resolver.resolveSettingReferenceWithContext(fallback, expansionConfiguration, setting, stack)
		}
	}
	// Only after every explicit pbxproj and xcconfig layer has been searched
	// does the implicit context apply, so a project that assigns one of these
	// names keeps Xcode's precedence.
	if value, ok := signingImplicitSettingValue(resolver.project, expansionConfiguration, setting); ok {
		return value, resolver.project.pbxprojPath, nil
	}
	return "", "", fmt.Errorf("setting not found")
}

// signingImplicitSettingValue supplies the build settings Xcode defines for
// every project from the project's own location. Xcode sets them before any
// pbxproj or xcconfig assignment is read, so a reference such as
// $(SRCROOT)/App.entitlements is valid in a project that never assigns SRCROOT
// itself and must not block an otherwise resolvable signing plan.
//
// Only values derivable from the selected .xcodeproj without running a build
// are returned. Anything that depends on a build context - CONFIGURATION,
// PLATFORM_NAME, SDKROOT, BUILT_PRODUCTS_DIR, and every other
// xcodebuild-supplied setting - stays unresolved so the plan fails closed
// instead of guessing which file it inventoried. A resolved path is still
// bound to the project root by the caller's rooted, no-follow containment and
// artifact-alias checks, so an implicit variable cannot widen the plan's
// reach.
func signingImplicitSettingValue(
	project *structuredVersionProject,
	configuration *versionConfiguration,
	setting string,
) (string, bool) {
	if project == nil {
		return "", false
	}
	switch setting {
	case "SRCROOT", "SOURCE_ROOT", "PROJECT_DIR":
		return project.rootDir, project.rootDir != ""
	case "PROJECT_FILE_PATH":
		return project.projectPath, project.projectPath != ""
	case "PROJECT_NAME":
		base := filepath.Base(project.projectPath)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		if name == "" || name == "." || name == string(filepath.Separator) {
			return "", false
		}
		return name, true
	case "TARGET_NAME":
		// A project-level configuration is shared by every target, so Xcode
		// has no single TARGET_NAME to define there.
		if configuration == nil || configuration.projectLevel || configuration.target == "" {
			return "", false
		}
		return configuration.target, true
	}
	return "", false
}

// resolveXCConfigBaseWithContext returns the lower-layer state that the
// target xcconfig resolver will use. Keeping this lookup shared with the
// resolver prevents the conservative entitlement-inventory pass from
// treating a target-level ?= as live when a project-level exact value already
// supplies the setting.
func (resolver *signingSettingResolver) resolveXCConfigBaseWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	path, setting string,
) (xcconfigResolvedValue, error) {
	base := xcconfigResolvedValue{}
	if configuration == nil || configuration.projectLevel {
		return base, nil
	}
	if fallback := resolver.project.projectConfiguration(configuration.name); fallback != nil {
		value, source, err := resolver.resolveSettingWithContext(fallback, expansionConfiguration, setting)
		if err == nil {
			// resolveSettingWithContext only returns a value after the lower
			// layer has produced a usable result. Preserve the resolver's
			// existing exact bit while exposing the real found state to the
			// conservative assignment scanner.
			base = xcconfigResolvedValue{value: value, path: source, found: true}
		} else if !errors.Is(err, errVersionSettingNotFound) {
			// A direct '=' assignment in the target xcconfig overrides the
			// project-level value and does not depend on it. Probe with and
			// without a private sentinel so only ?=/+=/inherited resolution
			// paths retain a fallback error as a blocker.
			if resolver.xcconfigDependsOnFallback(configuration, path, setting) {
				return xcconfigResolvedValue{}, fmt.Errorf("resolve project-level fallback for %s: %w", setting, err)
			}
		}
	}
	return base, nil
}

func (resolver *signingSettingResolver) identifyXCConfigFor(configuration *versionConfiguration, path string) (os.FileInfo, error) {
	canonical, collected, err := resolver.configurationXCConfigPath(configuration, path)
	if err != nil {
		return nil, err
	}
	if !collected {
		if resolver.configurationLexicallyObservedPath(configuration, path) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("xcconfig %s was not collected for this configuration", normalizeSigningLexicalPath(path))
	}
	return signingXCConfigIdentityFn(canonical)
}

func (resolver *signingSettingResolver) resolveXCConfigSettingStateWithContext(
	configuration *versionConfiguration,
	path, setting string,
	observe xcconfigAssignmentObserver,
	base xcconfigResolvedValue,
) (xcconfigResolvedValue, bool, error) {
	stat := func(includePath string) (os.FileInfo, error) {
		return resolver.statXCConfigFor(configuration, includePath)
	}
	read := func(includePath string) ([]byte, error) {
		return resolver.readXCConfigFor(configuration, includePath)
	}
	var identify func(string) (os.FileInfo, error)
	if xcconfigUsesIdentityTraversal() {
		identify = func(includePath string) (os.FileInfo, error) {
			return resolver.identifyXCConfigFor(configuration, includePath)
		}
	}
	return resolveXCConfigSettingStateWithReaderAndIdentity(path, setting, base, read, stat, identify, observe, nil)
}

func (resolver *signingSettingResolver) resolveConfigurationXCConfigWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	path, setting string,
) (xcconfigResolvedValue, error) {
	base, err := resolver.resolveXCConfigBaseWithContext(configuration, expansionConfiguration, path, setting)
	if err != nil {
		return xcconfigResolvedValue{}, err
	}
	stat := func(includePath string) (os.FileInfo, error) {
		return resolver.statXCConfigFor(configuration, includePath)
	}
	read := func(includePath string) ([]byte, error) {
		return resolver.readXCConfigFor(configuration, includePath)
	}
	var identify func(string) (os.FileInfo, error)
	if xcconfigUsesIdentityTraversal() {
		identify = func(includePath string) (os.FileInfo, error) {
			return resolver.identifyXCConfigFor(configuration, includePath)
		}
	}
	return resolveXCConfigSettingWithBaseReaderAndIdentityAndLookup(
		path,
		setting,
		base,
		read,
		stat,
		identify,
		func(name string) (string, bool) {
			return signingImplicitSettingValue(resolver.project, expansionConfiguration, name)
		},
	)
}

// xcconfigDependsOnFallback distinguishes a target xcconfig that semantically
// consumes its project-level base from one that replaces it with a direct
// assignment. The probe is read-only and uses the same authorization-aware
// callbacks as normal resolution. A sentinel is intentionally private: it is
// only compared inside this helper and is never surfaced in a plan or error.
func (resolver *signingSettingResolver) xcconfigDependsOnFallback(configuration *versionConfiguration, path, setting string) bool {
	stat := func(includePath string) (os.FileInfo, error) {
		return resolver.statXCConfigFor(configuration, includePath)
	}
	read := func(includePath string) ([]byte, error) {
		return resolver.readXCConfigFor(configuration, includePath)
	}
	withoutBase, withoutErr := resolveXCConfigSettingWithBaseReader(
		path,
		setting,
		xcconfigResolvedValue{},
		read,
		stat,
	)
	const sentinel = "__asc_signing_project_fallback_sentinel__"
	withBase, withErr := resolveXCConfigSettingWithBaseReader(
		path,
		setting,
		xcconfigResolvedValue{value: sentinel, path: resolver.project.pbxprojPath, found: true, exact: true},
		read,
		stat,
	)
	if withoutErr != nil || withErr != nil {
		// If one probe succeeds and the other does not, the base changes the
		// result. When both fail, the target's own error is returned by the
		// normal resolution below and the fallback error is not needed.
		return (withoutErr == nil) != (withErr == nil)
	}
	return withoutBase.value != withBase.value ||
		withoutBase.found != withBase.found ||
		withoutBase.exact != withBase.exact ||
		withoutBase.missingInherited != withBase.missingInherited
}

func (resolver *signingSettingResolver) expandSettingReferences(configuration *versionConfiguration, value string, stack map[string]bool) (string, string, error) {
	resolved := value
	for iteration := 0; iteration < 32; iteration++ {
		match := signingReferencePattern.FindStringSubmatchIndex(resolved)
		if match == nil {
			if strings.Contains(resolved, "$(") || strings.Contains(resolved, "${") {
				return "", "", fmt.Errorf("incomplete build-setting reference")
			}
			return strings.TrimSpace(resolved), resolver.project.pbxprojPath, nil
		}
		name := ""
		if match[2] >= 0 {
			name = resolved[match[2]:match[3]]
		} else {
			name = resolved[match[6]:match[7]]
		}
		if match[4] >= 0 || match[8] >= 0 {
			return "", "", fmt.Errorf("build-setting reference modifier for %s is unsupported", name)
		}
		if stack[name] {
			return "", "", fmt.Errorf("build-setting reference cycle at %s", name)
		}
		nextStack := make(map[string]bool, len(stack)+1)
		for key, set := range stack {
			nextStack[key] = set
		}
		nextStack[name] = true
		replacement, _, err := resolver.resolveSettingReference(configuration, name, nextStack)
		if err != nil {
			return "", "", fmt.Errorf("unresolved build-setting reference %s", name)
		}
		resolved = resolved[:match[0]] + replacement + resolved[match[1]:]
	}
	return "", "", fmt.Errorf("too many nested build-setting references")
}

// validateSigningEntitlementsPath binds a non-null CODE_SIGN_ENTITLEMENTS
// value to the selected project root. It requires an existing regular file
// and rejects symlinked parent or final components before a plan is ready.
func validateSigningEntitlementsPath(project *structuredVersionProject, path string) error {
	root, err := rootfs.New(project.rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := root.OpenFile(path)
	if err != nil {
		return err
	}
	return file.Close()
}

func resolveSigningSharedCandidates(candidates []signingCandidate, fileIdentities map[string]string) {
	desiredByFile := make(map[string]string)
	conflictByFile := make(map[string]bool)
	changedByFile := make(map[string]bool)
	noOpByFile := make(map[string]bool)
	for _, candidate := range candidates {
		if candidate.mode != "xcconfig" || candidate.desired == nil {
			continue
		}
		for _, path := range candidate.paths {
			// Shared-file conflict detection is identity-based: two consumers of
			// one inode must not receive different values through an xcconfig
			// mutation. Publication and receipt grouping below deliberately use the
			// path-preserving operation key so hard-linked path intents remain
			// separate writes.
			key := signingXCConfigPhysicalKey(path, fileIdentities) + "\x00" + candidate.setting
			if candidate.noOp {
				noOpByFile[key] = true
				continue
			}
			value := *candidate.desired
			if !signingValuesEqual(candidate.old, candidate.desired) {
				changedByFile[key] = true
			}
			if previous, exists := desiredByFile[key]; exists && previous != value {
				conflictByFile[key] = true
				continue
			}
			desiredByFile[key] = value
		}
	}
	for index := range candidates {
		if candidates[index].mode != "xcconfig" {
			continue
		}
		for _, path := range candidates[index].paths {
			key := signingXCConfigPhysicalKey(path, fileIdentities) + "\x00" + candidates[index].setting
			if !candidates[index].noOp && noOpByFile[key] && changedByFile[key] {
				// A no-op direct/inherited consumer is still semantically tied to
				// this shared setting. Keep the shared source unchanged and
				// materialize the requested value at each changing target instead;
				// this preserves the no-op consumer without inventing a lower-layer
				// value or relying on algebraic inherited-value assumptions.
				candidates[index].mode = "pbxproj"
				candidates[index].paths = nil
				candidates[index].noOp = false
				break
			}
			if conflictByFile[key] {
				candidates[index].mode = "pbxproj"
				candidates[index].paths = nil
				candidates[index].noOp = false
				break
			}
		}
	}
}

// buildSigningPlanOperations materializes the current candidate decisions
// into concrete operations. It deliberately runs shared-file arbitration on
// every pass: a candidate promoted to a target-level override must no longer
// participate as an xcconfig write in a later pass.
func buildSigningPlanOperations(
	project *structuredVersionProject,
	candidates []signingCandidate,
	configFiles map[string][]string,
	fileIdentities map[string]string,
	allowExternal bool,
	lexicalConfigPaths map[string][]string,
) ([]signingPlanOperation, []string) {
	resolveSigningSharedCandidates(candidates, fileIdentities)
	resolver := newSigningSettingResolver(project, configFiles, allowExternal, lexicalConfigPaths)
	operations := make([]signingPlanOperation, 0, len(candidates))
	var blockers []string
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.mode == "" || candidate.noOp {
			continue
		}
		if candidate.mode == "xcconfig" {
			var validationErr error
			for _, path := range candidate.paths {
				if err := validateSigningXCConfigWrite(resolver, path, candidate.setting, candidate.desired); err != nil {
					validationErr = err
					break
				}
			}
			if validationErr != nil {
				blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, validationErr))
				continue
			}
			for _, path := range candidate.paths {
				operations = append(operations, signingPlanOperation{
					SigningSettingChange: signingChange(candidate, path, "xcconfig"),
					configuration:        candidate.configuration,
				})
			}
			continue
		}
		operations = append(operations, signingPlanOperation{
			SigningSettingChange: signingChange(candidate, project.pbxprojPath, "pbxproj"),
			configuration:        candidate.configuration,
		})
	}
	sortSigningPlanOperations(operations)
	sort.Strings(blockers)
	return operations, blockers
}

// stageSigningPlan applies all concrete operations to a private project copy
// and xcconfig byte overlay. The source project and files are never mutated;
// the resulting resolver therefore observes the same state that apply will
// publish if the plan converges.
func stageSigningPlan(
	project *structuredVersionProject,
	operations []signingPlanOperation,
	configFiles map[string][]string,
	fileIdentities map[string]string,
	allowExternal bool,
	lexicalConfigPaths map[string][]string,
) (*structuredVersionProject, *signingSettingResolver, error) {
	stagedProject := cloneSigningStructuredVersionProject(project)
	configurations := make(map[string]*versionConfiguration, len(stagedProject.configurations))
	for _, configuration := range stagedProject.configurations {
		if configuration != nil {
			configurations[configuration.id] = configuration
		}
	}
	for _, operation := range operations {
		if operation.Source != "pbxproj" {
			continue
		}
		if operation.configuration == nil {
			return nil, nil, fmt.Errorf("missing configuration for %s", operation.Setting)
		}
		configuration := configurations[operation.configuration.id]
		if configuration == nil {
			return nil, nil, fmt.Errorf("configuration %s is missing from staged project", operation.configuration.id)
		}
		stagedOperation := operation
		stagedOperation.configuration = configuration
		if err := applySigningPBXOperation(stagedOperation); err != nil {
			return nil, nil, err
		}
	}

	resolver := newSigningSettingResolver(stagedProject, configFiles, allowExternal, lexicalConfigPaths)
	overlayByIdentity := make(map[string][]byte)
	for _, operation := range operations {
		if operation.Source != "xcconfig" {
			continue
		}
		if operation.NewValue == nil {
			return nil, nil, fmt.Errorf("xcconfig removal is not supported for %s", operation.Setting)
		}
		identityKey := signingXCConfigOperationKey(operation.Path, fileIdentities)
		data, ok := overlayByIdentity[identityKey]
		if !ok {
			var err error
			data, err = resolver.readXCConfig(operation.Path)
			if err != nil {
				return nil, nil, fmt.Errorf("read xcconfig %s while staging: %w", operation.Path, err)
			}
		}
		updated, _, _, err := editXCConfig(data, operation.Setting, *operation.NewValue)
		if err != nil {
			return nil, nil, fmt.Errorf("edit xcconfig %s while staging: %w", operation.Path, err)
		}
		if _, err := parseXCConfig(updated); err != nil {
			return nil, nil, fmt.Errorf("validate xcconfig %s while staging: %w", operation.Path, err)
		}
		overlayByIdentity[identityKey] = append([]byte(nil), updated...)
	}

	// A case-insensitive filesystem may expose one collected file under more
	// than one lexical spelling. Mirror the staged bytes to every collected
	// spelling that has the same identity so resolver reads cannot escape the
	// overlay on the next pass.
	overlay := make(map[string][]byte, len(overlayByIdentity))
	for _, paths := range configFiles {
		for _, path := range paths {
			identityKey := signingXCConfigOperationKey(path, fileIdentities)
			data, ok := overlayByIdentity[identityKey]
			if !ok {
				continue
			}
			overlay[normalizeSigningLexicalPath(path)] = append([]byte(nil), data...)
		}
	}
	resolver.stagedXCConfig = overlay
	return stagedProject, resolver, nil
}

// reclassifySigningNoOps validates every requested setting against the fully
// staged project. A no-op whose effective value changed is promoted to a
// target-level literal operation; preserving the original candidate metadata
// keeps the public plan's old-value and resolution fields stable.
func reclassifySigningNoOps(
	candidates []signingCandidate,
	originalProject *structuredVersionProject,
	stagedProject *structuredVersionProject,
	resolver *signingSettingResolver,
	baselineResolver *signingSettingResolver,
) (int, []string) {
	configurations := make(map[string]*versionConfiguration, len(stagedProject.configurations))
	for _, configuration := range stagedProject.configurations {
		if configuration != nil {
			configurations[configuration.id] = configuration
		}
	}
	reclassified := 0
	var blockers []string
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.configuration == nil {
			blockers = append(blockers, fmt.Sprintf("cannot stage signing setting %s without a configuration", candidate.setting))
			continue
		}
		configuration := configurations[candidate.configuration.id]
		if configuration == nil {
			blockers = append(blockers, fmt.Sprintf("target %q configuration %q is missing from staged signing project", candidate.configuration.target, candidate.configuration.name))
			continue
		}
		if candidate.desired == nil {
			if len(matchingBuildSettingKeys(configuration.buildSettings, candidate.setting)) > 0 {
				blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, errors.New("staged project still has a direct assignment")))
				continue
			}
			resolved, _, err := resolver.resolveSetting(configuration, candidate.setting)
			if err != nil && !errors.Is(err, errVersionSettingNotFound) {
				blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, err))
			} else if err == nil && baselineResolver != nil {
				baselineProject := cloneSigningStructuredVersionProject(originalProject)
				var baselineConfiguration *versionConfiguration
				for _, candidateConfiguration := range baselineProject.configurations {
					if candidateConfiguration != nil && candidateConfiguration.id == candidate.configuration.id {
						baselineConfiguration = candidateConfiguration
						break
					}
				}
				if baselineConfiguration == nil {
					continue
				}
				for _, key := range matchingBuildSettingKeys(baselineConfiguration.buildSettings, candidate.setting) {
					delete(baselineConfiguration.buildSettings, key)
				}
				baselineResolver = newSigningSettingResolver(baselineProject, baselineResolver.configFiles, baselineResolver.allowExternal, baselineResolver.lexicalConfigPaths)
				baseline, _, baselineErr := baselineResolver.resolveSetting(baselineConfiguration, candidate.setting)
				if signingRemovalFallbackChanged(resolved, err, baseline, baselineErr) && baselineErr == nil {
					blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, fmt.Errorf("staged value %q differs from value after removal alone %q; another operation in this plan would change the fallback", resolved, baseline)))
				} else if signingRemovalFallbackChanged(resolved, err, baseline, baselineErr) {
					blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, fmt.Errorf("staged value %q appears after removal alone left the setting unresolved; another operation in this plan would create the fallback", resolved)))
				}
			}
			continue
		}

		resolved, _, err := resolver.resolveSetting(configuration, candidate.setting)
		if err != nil {
			if candidate.noOp {
				// A previously resolvable no-op can become unresolved when a
				// staged dependency is removed (for example, a direct setting
				// that expands a setting removed by the same plan). Materialize
				// the requested value at the target level so the no-op's
				// effective value remains stable. Do this for every staged
				// resolution failure: the initial pass already proved that the
				// candidate resolved, so a new error necessarily comes from the
				// staged dependency state.
				candidate.mode = "pbxproj"
				candidate.paths = nil
				candidate.noOp = false
				reclassified++
				continue
			}
			blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, fmt.Errorf("staged resolution failed: %w", err)))
			continue
		}
		if resolved == *candidate.desired {
			continue
		}
		if candidate.noOp {
			candidate.mode = "pbxproj"
			candidate.paths = nil
			candidate.noOp = false
			reclassified++
			continue
		}
		blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, fmt.Errorf("staged value %q does not match desired value %q", resolved, *candidate.desired)))
	}
	sort.Strings(blockers)
	return reclassified, blockers
}

func signingRemovalFallbackChanged(staged string, stagedErr error, baseline string, baselineErr error) bool {
	if stagedErr != nil {
		return false
	}
	if baselineErr != nil {
		return true
	}
	return staged != baseline
}

func cloneSigningStructuredVersionProject(project *structuredVersionProject) *structuredVersionProject {
	if project == nil {
		return nil
	}
	clone := *project
	clone.objects = cloneSigningSerializedObject(project.objects)
	clone.project.RawProj = cloneSigningSerializedObject(project.project.RawProj)
	clone.parentByChild = make(map[string]string, len(project.parentByChild))
	for child, parent := range project.parentByChild {
		clone.parentByChild[child] = parent
	}
	clone.configurations = make([]*versionConfiguration, 0, len(project.configurations))
	for _, configuration := range project.configurations {
		if configuration == nil {
			clone.configurations = append(clone.configurations, nil)
			continue
		}
		configurationClone := *configuration
		configurationClone.buildSettings = cloneSigningSerializedObject(configuration.buildSettings)
		clone.configurations = append(clone.configurations, &configurationClone)
	}
	return &clone
}

func cloneSigningSerializedObject(source serialized.Object) serialized.Object {
	if source == nil {
		return nil
	}
	clone := make(serialized.Object, len(source))
	for key, value := range source {
		clone[key] = cloneSigningValue(value)
	}
	return clone
}

func cloneSigningValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case serialized.Object:
		return cloneSigningSerializedObject(typed)
	case map[string]interface{}:
		clone := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			clone[key] = cloneSigningValue(nested)
		}
		return clone
	case []interface{}:
		clone := make([]interface{}, len(typed))
		for index, nested := range typed {
			clone[index] = cloneSigningValue(nested)
		}
		return clone
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}

// signingXCConfigPhysicalKey groups authorized xcconfig paths by the
// filesystem identity captured during collection. It is used only for
// consumer/conflict decisions: a hard link is one physical source even when
// the project names it through two paths.
func signingXCConfigPhysicalKey(path string, fileIdentities map[string]string) string {
	if identity, ok := signingFileIdentity(path, fileIdentities); ok && identity != "" {
		return "identity:" + identity
	}
	return "path:" + signingLexicalPathKey(path)
}

// signingXCConfigOperationKey is the path-preserving key used for plan files,
// prepared writes, and receipts. A physical identity alone is insufficient:
// hard-linked paths are distinct directory entries, and an atomic rename of
// one must not silently satisfy or discard the other. Parent-symlink aliases
// such as Configs/Shared.xcconfig and AliasDir/Shared.xcconfig resolve to the
// same directory entry; those must coalesce or the first rename changes the
// inode and the second write fails with "source changed before commit".
func signingXCConfigOperationKey(path string, fileIdentities map[string]string) string {
	entryKey := signingXCConfigDirectoryEntryKey(path)
	if identity, ok := signingFileIdentity(path, fileIdentities); ok && identity != "" {
		return "identity:" + identity + "\x00entry:" + entryKey
	}
	return "path:" + entryKey
}

// signingXCConfigDirectoryEntryKey identifies the directory entry that a
// requested xcconfig path names. Intermediate directory symlinks are resolved
// so in-project aliases of one parent share a key, while the final component
// stays lexical so two hard links in the same directory remain distinct.
func signingXCConfigDirectoryEntryKey(path string) string {
	normalized := normalizeSigningLexicalPath(path)
	parent := filepath.Dir(normalized)
	base := filepath.Base(normalized)
	if resolved, err := filepath.EvalSymlinks(parent); err == nil {
		parent = resolved
	}
	return signingXCConfigPathIntentKey(filepath.Join(parent, base))
}

func signingXCConfigPathIntentKey(path string) string {
	normalized := normalizeSigningLexicalPath(path)
	caseInsensitive, known := signingCaseInsensitiveVolumeFn(filepath.Dir(normalized))
	if known && caseInsensitive {
		return strings.ToLower(normalized)
	}
	return normalized
}

// signingFileIdentity first uses the exact normalized spelling emitted by the
// collector. The lexical-key fallback keeps compatibility with older callers
// and hand-built test maps, but an exact entry always wins so two proven
// case-distinct files on a Windows case-sensitive directory cannot collide.
func signingFileIdentity(path string, fileIdentities map[string]string) (string, bool) {
	if fileIdentities == nil {
		return "", false
	}
	if identity, ok := fileIdentities[normalizeSigningLexicalPath(path)]; ok {
		return identity, identity != ""
	}
	legacyKey := signingLexicalPathKey(path)
	if identity, ok := fileIdentities[legacyKey]; ok {
		return identity, identity != ""
	}
	return "", false
}

func signingSettingBlocker(configuration *versionConfiguration, setting string, err error) string {
	return fmt.Sprintf("target %q configuration %q cannot resolve %s: %v", configuration.target, configuration.name, setting, err)
}

func validateSigningXCConfigWrite(resolver *signingSettingResolver, path, setting string, desired *string) error {
	if desired == nil {
		return nil
	}
	data, err := resolver.readXCConfig(path)
	if err != nil {
		return err
	}
	document, err := parseXCConfig(data)
	if err != nil {
		return err
	}
	if !xcconfigValueHasLineContinuation(*desired) {
		return nil
	}
	for _, assignment := range document.assignments {
		if assignment.baseKey != setting {
			continue
		}
		if assignment.quote == "" {
			return fmt.Errorf("desired value has a trailing backslash that would continue the xcconfig assignment")
		}
	}
	return nil
}

func signingValuesEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func signingChange(candidate *signingCandidate, path, source string) SigningSettingChange {
	operation := "set"
	if candidate.desired == nil {
		operation = "remove"
	}
	return SigningSettingChange{
		Target:        candidate.configuration.target,
		Configuration: candidate.configuration.name,
		Setting:       candidate.setting,
		Operation:     operation,
		OldValue:      cloneSigningString(candidate.old),
		NewValue:      cloneSigningString(candidate.desired),
		Path:          path,
		Source:        source,
		Resolution:    candidate.resolution,
	}
}

func signingPathContained(project *structuredVersionProject, path string) bool {
	root, err := rootfs.New(project.rootDir)
	if err != nil {
		return false
	}
	defer root.Close()
	return root.AllowingInternalSymlinks().CheckContained(path) == nil
}

// normalizeSigningLexicalPath returns the absolute, cleaned spelling used for
// lexical path decisions. It intentionally does not resolve symlinks: lexical
// protection must also cover a path that does not exist yet.
func normalizeSigningLexicalPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(absolute)
}

// signingLexicalPathKey applies the host filesystem's equality semantics to a
// normalized path. Windows paths are case-insensitive even when the protected
// or artifact path is still missing, so SameFile cannot be relied on there.
func signingLexicalPathKey(path string) string {
	path = normalizeSigningLexicalPath(path)
	if runtimeGOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func signingLexicalPathEqual(left, right string) bool {
	return signingLexicalPathKey(left) == signingLexicalPathKey(right)
}

// signingArtifactLexicalPathEqual applies filesystem case semantics only at
// the artifact-alias boundary. General signing path keys retain their
// platform-independent spelling rules; this helper additionally protects
// missing paths on case-insensitive Darwin volumes where SameFile cannot
// inspect an inode yet. Unknown volume metadata is treated conservatively as
// case-insensitive so an uncertain alias cannot reach an artifact write.
func signingArtifactLexicalPathEqual(left, right string) bool {
	left = normalizeSigningLexicalPath(left)
	right = normalizeSigningLexicalPath(right)
	if signingLexicalPathEqual(left, right) {
		return true
	}
	if !strings.EqualFold(left, right) {
		return false
	}
	caseInsensitive, known := signingCaseInsensitiveVolumeFn(left)
	return !known || caseInsensitive
}

func signingPathLexicallyContained(project *structuredVersionProject, path string) bool {
	root := normalizeSigningLexicalPath(project.rootDir)
	absolute := normalizeSigningLexicalPath(path)
	if signingNormalizedPathContained(root, absolute) {
		return true
	}
	if runtimeGOOS != "windows" && runtimeGOOS != "darwin" && runtimeGOOS != "linux" {
		return false
	}
	// A path outside the exact normalized root and outside its case-folded
	// component prefix is definitely external. Reject it without consulting
	// per-volume case metadata; that metadata is reserved for the narrow case
	// variant that could genuinely name the selected root on an insensitive
	// filesystem.
	if !signingPathCaseFoldedPrefixContained(root, absolute) {
		return false
	}
	// A case-folded containment result is safe only when both the project
	// root and the candidate's containing directory are proven
	// case-insensitive. Windows supports case-sensitive directories on an
	// otherwise case-insensitive volume, so a global lowercase comparison can
	// incorrectly authorize C:\\project when the root is C:\\Project.
	rootInsensitive, rootKnown := signingCaseInsensitiveVolumeFn(root)
	pathInsensitive, pathKnown := signingCaseInsensitiveVolumeFn(filepath.Dir(absolute))
	if !rootKnown || !pathKnown || !rootInsensitive || !pathInsensitive {
		return false
	}
	return signingNormalizedPathContained(strings.ToLower(root), strings.ToLower(absolute))
}

func signingPathCaseFoldedPrefixContained(root, absolute string) bool {
	root = normalizeSigningLexicalPath(root)
	absolute = normalizeSigningLexicalPath(absolute)
	if strings.EqualFold(root, absolute) {
		return true
	}
	if len(absolute) <= len(root) || !strings.EqualFold(root, absolute[:len(root)]) {
		return false
	}
	return absolute[len(root)] == filepath.Separator
}

// signingPathDefinitelyExternal answers the lexical portion of authorization
// without consulting filesystem metadata. A case-folded root prefix remains
// unresolved until signingPathLexicallyContained can inspect the relevant
// volume semantics; every other non-contained path is safely external.
func signingHasIncompleteInternalXCConfig(project *structuredVersionProject, paths []string, allowExternal bool) bool {
	if len(paths) == 0 {
		return false
	}
	if allowExternal {
		// An opted-in external graph that still failed collection cannot be
		// inventoried. A blocked plan is unsafe to serialize over an unread
		// entitlement assignment.
		return true
	}
	for _, path := range paths {
		if !signingPathDefinitelyExternal(project, path) {
			return true
		}
	}
	return false
}

func signingPathDefinitelyExternal(project *structuredVersionProject, path string) bool {
	root := normalizeSigningLexicalPath(project.rootDir)
	absolute := normalizeSigningLexicalPath(path)
	if signingNormalizedPathContained(root, absolute) {
		return false
	}
	if runtimeGOOS != "windows" && runtimeGOOS != "darwin" && runtimeGOOS != "linux" {
		return true
	}
	return !signingPathCaseFoldedPrefixContained(root, absolute)
}

func signingNormalizedPathContained(root, absolute string) bool {
	root = filepath.Clean(root)
	absolute = filepath.Clean(absolute)
	if runtimeGOOS == "windows" {
		// filepath.Rel uses case-folded component comparisons on Windows. That
		// is unsafe for a case-sensitive directory, so containment must first
		// use the exact normalized spelling and separator boundaries.
		return signingPathComponentsContained(root, absolute, string(filepath.Separator))
	}
	relative, err := signingPathRelFn(root, absolute)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// signingPathComponentsContained compares already-normalized absolute paths
// without filepath.Rel's platform-specific case folding. The separator is
// explicit so Windows component semantics can be tested on other hosts.
func signingPathComponentsContained(root, absolute, separator string) bool {
	if root == absolute {
		return true
	}
	if root == separator || strings.HasSuffix(root, separator) {
		return strings.HasPrefix(absolute, root)
	}
	return strings.HasPrefix(absolute, root+separator)
}

func signingXCConfigCollectionBlocker(project *structuredVersionProject, path string, allowExternal bool) string {
	if signingPathDefinitelyExternal(project, path) {
		if !allowExternal {
			return fmt.Sprintf("xcconfig %s is external and could not be read without --allow-external-xcconfig", path)
		}
		return fmt.Sprintf("xcconfig %s is external and could not be safely collected", path)

	}
	if !signingPathLexicallyContained(project, path) {
		if !allowExternal {
			return fmt.Sprintf("xcconfig %s is external and could not be read without --allow-external-xcconfig", path)
		}
		return fmt.Sprintf("xcconfig %s is external and could not be safely collected", path)
	}
	return fmt.Sprintf("xcconfig %s could not be safely collected; signing scope is uncertain", path)
}

func appendUniqueSigningPaths(paths []string, additions ...string) []string {
	for _, path := range additions {
		path = normalizeSigningLexicalPath(path)
		if path == "" {
			continue
		}
		found := false
		for _, existing := range paths {
			// This list is also populated from collector onPath callbacks before
			// authorization and filesystem identity checks. Keep deduplication
			// purely lexical so an untrusted external path cannot trigger a case
			// volume probe merely because it resembles an earlier spelling.
			if normalizeSigningLexicalPath(existing) == path {
				found = true
				break
			}
		}
		if !found {
			paths = append(paths, path)
		}
	}
	return paths
}

func sortSigningPlanOperations(operations []signingPlanOperation) {
	sort.Slice(operations, func(left, right int) bool {
		first, second := operations[left], operations[right]
		if first.Path != second.Path {
			return first.Path < second.Path
		}
		if first.Target != second.Target {
			return first.Target < second.Target
		}
		if first.Configuration != second.Configuration {
			return first.Configuration < second.Configuration
		}
		return first.Setting < second.Setting
	})
}

func validateSigningProjectFile(project *structuredVersionProject) error {
	projectConfigurationNames := make(map[string]struct{})
	configurationConsumers := make(map[string]string)
	for _, configuration := range project.configurations {
		consumer := "project"
		if !configuration.projectLevel {
			consumer = "target " + configuration.target
		}
		consumer += " configuration " + configuration.name
		if previous, seen := configurationConsumers[configuration.id]; seen {
			return fmt.Errorf(
				"XCBuildConfiguration %q is reused by multiple project or target consumers (%s and %s)",
				configuration.id,
				previous,
				consumer,
			)
		}
		configurationConsumers[configuration.id] = consumer
		if !configuration.projectLevel {
			continue
		}
		if _, seen := projectConfigurationNames[configuration.name]; seen {
			return fmt.Errorf("project contains multiple configurations named %q", configuration.name)
		}
		projectConfigurationNames[configuration.name] = struct{}{}
	}

	root, err := rootfs.New(project.rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.AllowingInternalSymlinks().CheckContained(project.pbxprojPath); err != nil {
		return fmt.Errorf("refusing to use Xcode project file %s: %w", project.pbxprojPath, err)
	}
	return nil
}

func validateSigningArtifactPaths(planPath, receiptPath, projectPath, settingsPath string) error {
	if signingArtifactLexicalPathEqual(planPath, receiptPath) {
		return fmt.Errorf("plan and receipt paths must be different")
	}
	for _, candidate := range []struct {
		label string
		path  string
	}{{"plan", planPath}, {"receipt", receiptPath}} {
		if signingArtifactLexicalPathEqual(candidate.path, projectPath) {
			return fmt.Errorf("%s path must not replace the Xcode project file", candidate.label)
		}
		if signingArtifactLexicalPathEqual(candidate.path, settingsPath) {
			return fmt.Errorf("%s path must not replace the settings file", candidate.label)
		}
	}
	return nil
}

// validateSigningArtifactAliases rejects an existing plan or receipt that
// resolves to any source consumed while building the plan. Lexical path
// comparisons do not catch hard-link aliases, while rooted no-follow identity
// checks plus os.SameFile identify existing aliases without mutating the
// filesystem; symlink aliases are rejected by the rooted opener.
func validateSigningArtifactAliases(planPath, receiptPath string, inputPaths, protectedPaths []string) error {
	return validateSigningArtifactAliasesWithAuthorizedProtectedPaths(planPath, receiptPath, inputPaths, protectedPaths, protectedPaths)
}

func validateSigningArtifactAliasesWithAuthorizedProtectedPaths(planPath, receiptPath string, inputPaths, protectedPaths, authorizedProtectedPaths []string) error {
	normalize := func(path string) string {
		return normalizeSigningLexicalPath(path)
	}
	type artifact struct {
		label string
		path  string
	}
	artifacts := []artifact{
		{label: "plan", path: normalize(planPath)},
		{label: "receipt", path: normalize(receiptPath)},
	}
	// Resolve lexical collisions before inspecting any filesystem path. This
	// keeps an exact collision with an unauthorized protected path entirely
	// no-touch while still allowing prospective physical resolution for paths
	// that are safe to inspect.
	for _, artifact := range artifacts {
		for _, protectedPath := range protectedPaths {
			if signingArtifactLexicalPathEqual(artifact.path, protectedPath) {
				return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("%s path aliases protected project input %s", artifact.label, artifact.path)))
			}
		}
	}
	if signingArtifactLexicalPathEqual(artifacts[0].path, artifacts[1].path) {
		return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("plan and receipt paths must not alias the same file")))
	}
	artifactPhysicalPaths := make(map[string]string, len(artifacts))
	for _, artifact := range artifacts {
		physical, err := signingResolveProspectivePathFn(artifact.path)
		if err != nil {
			return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("resolve prospective %s path %s: %w", artifact.label, artifact.path, err)))
		}
		artifactPhysicalPaths[artifact.label] = physical
	}
	authorizedPathIndex := newSigningAuthorizedPathIndex(authorizedProtectedPaths)
	protectedPhysicalPaths := make(map[string]string, len(protectedPaths))
	for _, protectedPath := range protectedPaths {
		if !authorizedPathIndex.contains(protectedPath) {
			continue
		}
		// The rooted inspector never follows a final symlink, so an aliased
		// protected input surfaces as rootfs.ErrSymlink rather than symlink
		// FileInfo. Both mean the same thing here: the prospective-path
		// comparison below cannot see through the link, so accepting the path
		// would let an artifact write land on the link target. Any other
		// inspection failure leaves the alias set unknown and fails closed as
		// an ordinary I/O error instead of a spurious alias claim.
		info, infoErr := signingArtifactPathInfoFn(protectedPath)
		switch {
		case infoErr == nil:
			if info.Mode()&os.ModeSymlink != 0 {
				return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("protected project input %s is a symlink and cannot be aliased by an artifact", protectedPath)))
			}
		case errors.Is(infoErr, rootfs.ErrSymlink):
			return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("protected project input %s is a symlink and cannot be aliased by an artifact", protectedPath)))
		case errors.Is(infoErr, os.ErrNotExist):
			// A protected input that does not exist yet is still covered by the
			// prospective-path comparison below.
		default:
			return fmt.Errorf("inspect protected project input %s: %w", protectedPath, infoErr)
		}
		physical, err := signingResolveProspectivePathFn(protectedPath)
		if err != nil {
			return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("resolve prospective protected project input %s: %w", protectedPath, err)))
		}
		protectedPhysicalPaths[protectedPath] = physical
	}
	for _, artifact := range artifacts {
		for _, protectedPath := range protectedPaths {
			protectedPhysicalPath, ok := protectedPhysicalPaths[protectedPath]
			if ok && signingArtifactLexicalPathEqual(artifactPhysicalPaths[artifact.label], protectedPhysicalPath) {
				return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("%s path aliases protected project input %s", artifact.label, artifact.path)))
			}
		}
	}
	if signingArtifactLexicalPathEqual(artifacts[0].path, artifacts[1].path) ||
		signingArtifactLexicalPathEqual(artifactPhysicalPaths[artifacts[0].label], artifactPhysicalPaths[artifacts[1].label]) {
		return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("plan and receipt paths must not alias the same file")))
	}
	artifactInfos := make(map[string]os.FileInfo, len(artifacts))
	for _, artifact := range artifacts {
		info, err := signingArtifactPathInfoFn(artifact.path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("inspect %s artifact path %s: %w", artifact.label, artifact.path, err)))
		}
		artifactInfos[artifact.label] = info
	}
	if planInfo, planOK := artifactInfos["plan"]; planOK {
		if receiptInfo, receiptOK := artifactInfos["receipt"]; receiptOK && os.SameFile(planInfo, receiptInfo) {
			return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("plan and receipt paths must not alias the same file")))
		}
	}

	seenInputs := make([]string, 0, len(inputPaths))
	for _, inputPath := range inputPaths {
		if strings.TrimSpace(inputPath) == "" {
			continue
		}
		inputPath = normalize(inputPath)
		duplicate := false
		for _, seenInput := range seenInputs {
			if signingArtifactLexicalPathEqual(seenInput, inputPath) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		seenInputs = append(seenInputs, inputPath)
		for _, artifact := range artifacts {
			if signingArtifactLexicalPathEqual(inputPath, artifact.path) {
				return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("%s path aliases project input %s", artifact.label, inputPath)))
			}
		}
		physicalInputPath, err := signingResolveProspectivePathFn(inputPath)
		if err != nil {
			return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("resolve prospective project input %s: %w", inputPath, err)))
		}
		for _, artifact := range artifacts {
			if signingArtifactLexicalPathEqual(physicalInputPath, artifactPhysicalPaths[artifact.label]) {
				return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("%s path aliases project input %s", artifact.label, inputPath)))
			}
		}
		inputInfo, err := signingArtifactPathInfoFn(inputPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect project input %s: %w", inputPath, err)
		}
		for _, artifact := range artifacts {
			artifactInfo, ok := artifactInfos[artifact.label]
			if ok && os.SameFile(artifactInfo, inputInfo) {
				return newSigningInputError(newSigningArtifactAliasError(fmt.Errorf("%s path aliases project input %s", artifact.label, inputPath)))
			}
		}
	}
	return nil
}

type signingAuthorizedPathIndex struct {
	exact  map[string]struct{}
	folded map[string][]string
}

// newSigningAuthorizedPathIndex builds a per-validation lexical membership
// index. Exact normalized paths are the common case. Folded buckets only
// narrow candidates; signingPathCaseEquivalentNormalized remains the authority
// for case-sensitive versus case-insensitive directory semantics.
func newSigningAuthorizedPathIndex(paths []string) signingAuthorizedPathIndex {
	index := signingAuthorizedPathIndex{
		exact:  make(map[string]struct{}, len(paths)),
		folded: make(map[string][]string, len(paths)),
	}
	for _, path := range paths {
		normalized := normalizeSigningLexicalPath(path)
		index.exact[normalized] = struct{}{}
		key := signingPathCaseFoldKey(normalized)
		index.folded[key] = append(index.folded[key], normalized)
	}
	return index
}

func (index signingAuthorizedPathIndex) contains(path string) bool {
	normalized := normalizeSigningLexicalPath(path)
	if _, ok := index.exact[normalized]; ok {
		return true
	}
	for _, authorized := range index.folded[signingPathCaseFoldKey(normalized)] {
		if signingPathCaseEquivalentNormalized(normalized, authorized) {
			return true
		}
	}
	return false
}

// signingPathCaseFoldKey is an EqualFold-compatible bucket key. It is only an
// index accelerator: authorization still requires the platform-aware
// comparator above. Simple-folding avoids false negatives for Unicode path
// spellings that strings.ToLower does not coalesce.
func signingPathCaseFoldKey(path string) string {
	return strings.Map(func(r rune) rune {
		folded := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < folded {
				folded = next
			}
		}
		return folded
	}, path)
}

func signingProjectInputPaths(
	project *structuredVersionProject,
	settingsPath string,
	configFiles map[string][]string,
	fileIdentities map[string]string,
	requests []signingRequest,
	allowExternal bool,
	lexicalConfigPaths map[string][]string,
) ([]string, []string, []string, error) {
	// Paths that failed collection or authorization are intentionally not added
	// to the readable input set. The caller protects their lexical paths
	// separately, so alias validation never stats an unauthorized or missing
	// path.
	externalEntitlementPaths := make([]string, 0)
	inputBlockers := make([]string, 0)
	uncertainEntitlementConfigurations := make(map[string]bool)
	paths := make([]string, 0, 2)
	if strings.TrimSpace(settingsPath) != "" {
		paths = append(paths, project.pbxprojPath, settingsPath)
	} else {
		paths = []string{project.pbxprojPath}
	}
	selectedIDs := make(map[string]bool, len(requests))
	for _, request := range requests {
		configuration, err := signingConfigurationFor(project, request.target, request.configuration)
		if err == nil {
			selectedIDs[configuration.id] = true
		}
	}
	selectedXCConfigSources := make(map[string]bool)
	resolvedEntitlementConfigurations := make(map[string]bool)
	for _, configuration := range project.configurations {
		if !selectedIDs[configuration.id] {
			continue
		}
		for current := configuration; current != nil; {
			for _, filePath := range configFiles[current.id] {
				selectedXCConfigSources[signingXCConfigOperationKey(filePath, fileIdentities)] = true
			}
			if current.projectLevel {
				break
			}
			current = project.projectConfiguration(current.name)
		}
	}
	appendEntitlements := func(value string) error {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		// Raw project/xcconfig scans can encounter the expression that the
		// authorization-aware resolver already expanded. Do not turn that
		// expression into a synthetic filesystem path; a selected unresolved
		// expression is still reported by the resolver above.
		if strings.Contains(value, "$(") || strings.Contains(value, "${") {
			return nil
		}
		if validateSigningRelativePath(value) == nil {
			paths = append(paths, filepath.Join(project.rootDir, filepath.FromSlash(value)))
			return nil
		}
		if filepath.IsAbs(value) {
			absolute := filepath.Clean(value)
			if !signingPathLexicallyContained(project, absolute) {
				externalEntitlementPaths = appendUniqueSigningPaths(externalEntitlementPaths, absolute)
				return nil
			}
			paths = append(paths, absolute)
			return nil
		}
		return fmt.Errorf("CODE_SIGN_ENTITLEMENTS path %q is invalid and cannot be protected", value)
	}
	appendLexicalEntitlementCandidate := func(value string) error {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		// An unresolved inherited expression can still have a concrete path
		// after the inherited token is empty (the Xcode semantics when no lower
		// layer supplies it). Preserve that bounded candidate for lexical alias
		// checks. Any other unresolved reference may expand to an arbitrary path;
		// no finite alias set is sound, so fail before a blocked artifact can be
		// written.
		value = strings.ReplaceAll(value, "$(inherited)", "")
		value = strings.ReplaceAll(value, "${inherited}", "")
		if strings.ContainsRune(value, '\x00') || strings.Contains(value, "$(") || strings.Contains(value, "${") {
			return fmt.Errorf("CODE_SIGN_ENTITLEMENTS value cannot be safely inventoried while a build-setting reference is unresolved")
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("CODE_SIGN_ENTITLEMENTS value cannot be safely inventoried while its inherited value is unresolved")
		}
		candidate := value
		if !filepath.IsAbs(candidate) && !pathpkg.IsAbs(candidate) && !isWindowsDrivePath(candidate) {
			if err := validateSigningRelativePath(candidate); err != nil {
				return fmt.Errorf("CODE_SIGN_ENTITLEMENTS value cannot be safely inventoried: %w", err)
			}
			candidate = filepath.Join(project.rootDir, filepath.FromSlash(candidate))
		}
		externalEntitlementPaths = appendUniqueSigningPaths(externalEntitlementPaths, filepath.Clean(candidate))
		return nil
	}
	isConditionalEntitlementKey := func(key string) bool {
		return key != "CODE_SIGN_ENTITLEMENTS" && xcconfigBaseKey(key) == "CODE_SIGN_ENTITLEMENTS"
	}
	for _, files := range configFiles {
		paths = append(paths, files...)
	}
	for _, request := range requests {
		for _, setting := range request.settings {
			if setting.key == "CODE_SIGN_ENTITLEMENTS" && setting.value != nil {
				if err := appendEntitlements(*setting.value); err != nil {
					return nil, externalEntitlementPaths, inputBlockers, err
				}
			}
		}
	}
	resolver := newSigningSettingResolver(project, configFiles, allowExternal, lexicalConfigPaths)
	// assignmentKey names the build-setting key the value was written under.
	// A conditional key such as CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] composes
	// its $(inherited) through the object's unconditional assignment; callers
	// holding an already-effective value pass "CODE_SIGN_ENTITLEMENTS".
	appendResolvedEntitlements := func(configuration *versionConfiguration, assignmentKey, value string, xcconfigSource bool) error {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		if strings.Contains(value, "$(") || strings.Contains(value, "${") {
			if configuration == nil {
				return fmt.Errorf("CODE_SIGN_ENTITLEMENTS reference cannot be resolved without a configuration")
			}
			// Use the same source-aware expansion as direct build settings. In
			// particular, $(inherited) must resolve against the lower project or
			// xcconfig layer rather than being treated as an ordinary setting
			// reference. This keeps raw PBX and xcconfig scans aligned with the
			// effective resolver and prevents an inherited entitlement path from
			// disappearing from the protected-input inventory.
			expanded, _, err := resolver.expandDirectAssignmentWithSourceContext(
				configuration,
				configuration,
				"CODE_SIGN_ENTITLEMENTS",
				assignmentKey,
				value,
				map[string]bool{"CODE_SIGN_ENTITLEMENTS": true},
				xcconfigSource,
			)
			if err != nil {
				return err
			}
			value = expanded
		}
		return appendEntitlements(value)
	}
	for _, configuration := range project.configurations {
		selected := selectedIDs[configuration.id]
		for _, key := range matchingBuildSettingKeys(configuration.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
			if _, ok := configuration.buildSettings[key].(string); !ok {
				// Xcode project values can be arbitrary serialized objects, but
				// signing can only safely inventory a scalar entitlement path. Do
				// not silently drop a list/object value: it may name a protected
				// input that would otherwise be overwritten by an artifact.
				return nil, externalEntitlementPaths, inputBlockers, fmt.Errorf("target %q configuration %q has a non-string CODE_SIGN_ENTITLEMENTS value", configuration.target, configuration.name)
			}
		}
		authorized := signingConfigurationSourcesAuthorized(project, configuration, configFiles)
		if configuration.projectLevel {
			// Do not resolve the project configuration itself: it cannot see
			// target-supplied settings such as PRODUCT_NAME. Still inventory
			// literal project assignments, including conditional-only paths,
			// before artifact publication. Expressions are expanded later
			// through each inheriting target configuration.
			for _, key := range matchingBuildSettingKeys(configuration.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
				value, ok := configuration.buildSettings[key].(string)
				if !ok || strings.Contains(value, "$(") || strings.Contains(value, "${") {
					continue
				}
				if err := appendEntitlements(value); err != nil {
					return nil, externalEntitlementPaths, inputBlockers, err
				}
			}
			continue
		}
		if authorized {
			value, _, err := resolver.resolveSetting(configuration, "CODE_SIGN_ENTITLEMENTS")
			if err == nil {
				if err := appendResolvedEntitlements(configuration, "CODE_SIGN_ENTITLEMENTS", value, false); err != nil {
					if lexicalErr := appendLexicalEntitlementCandidate(value); lexicalErr != nil {
						return nil, externalEntitlementPaths, inputBlockers, lexicalErr
					}
					if selected {
						return nil, externalEntitlementPaths, inputBlockers, err
					}
					inputBlockers = append(inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved CODE_SIGN_ENTITLEMENTS input: %v", configuration.target, configuration.name, err))
				} else {
					resolvedEntitlementConfigurations[configuration.id] = true
				}
			} else if !errors.Is(err, errVersionSettingNotFound) {
				resolutionErr := fmt.Errorf("resolve CODE_SIGN_ENTITLEMENTS for target %q configuration %q: %w", configuration.target, configuration.name, err)
				if selected {
					return nil, externalEntitlementPaths, inputBlockers, resolutionErr
				}
				// An unselected configuration whose effective entitlement value
				// cannot be resolved may still have more than one semantically
				// possible assignment (for example, an unconditional value and a
				// divergent SDK-conditional value). The later assignment scan must
				// retain every live candidate for alias protection instead of
				// assuming the conditional assignment is the whole input set.
				uncertainEntitlementConfigurations[configuration.id] = true
				inputBlockers = append(inputBlockers, resolutionErr.Error())
			}
		}
		for _, key := range matchingBuildSettingKeys(configuration.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
			value, ok := configuration.buildSettings[key].(string)
			if ok {
				if !authorized && !selected && (strings.Contains(value, "$(") || strings.Contains(value, "${")) {
					inputBlockers = append(inputBlockers, fmt.Sprintf("target %q configuration %q has unresolved CODE_SIGN_ENTITLEMENTS reference; signing scope is uncertain", configuration.target, configuration.name))
					continue
				}
				if err := appendResolvedEntitlements(configuration, key, value, false); err != nil {
					if lexicalErr := appendLexicalEntitlementCandidate(value); lexicalErr != nil {
						return nil, externalEntitlementPaths, inputBlockers, lexicalErr
					}
					if isConditionalEntitlementKey(key) {
						if !selected {
							inputBlockers = append(inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved conditional CODE_SIGN_ENTITLEMENTS input; signing scope is uncertain", configuration.target, configuration.name))
							continue
						}
						return nil, externalEntitlementPaths, inputBlockers, newSigningConditionalEntitlementError(err)
					}
					if selected {
						return nil, externalEntitlementPaths, inputBlockers, err
					}
					inputBlockers = append(inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved CODE_SIGN_ENTITLEMENTS input: %v", configuration.target, configuration.name, err))
				}
			}
		}
		// An unconditional inherited target assignment can consume any
		// project-level entitlement assignment when the project fallback is
		// genuinely uncertain. The effective resolver intentionally reports one
		// value (or falls back to its first literal) for callers that need a
		// single setting, but the artifact alias inventory must retain every
		// concrete composition that Xcode may select. Same-object conditional
		// assignments inherit through the target's unconditional slot, and a
		// target xcconfig can provide the lower value, so neither case should be
		// cross-composed with project seeds here.
		for _, key := range matchingBuildSettingKeys(configuration.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
			if key != "CODE_SIGN_ENTITLEMENTS" {
				continue
			}
			value, ok := configuration.buildSettings[key].(string)
			if !ok || !signingValueInherits(value) {
				continue
			}
			if _, err := resolver.resolveLowerSettingWithContext(configuration, configuration, "CODE_SIGN_ENTITLEMENTS"); err == nil {
				continue
			}
			if files := configFiles[configuration.id]; len(files) > 0 {
				if signingConfigurationXCConfigHasLiveUnconditionalEntitlementOverride(configuration, files, resolver) {
					continue
				}
			}
			for _, projectCfg := range project.configurations {
				if !projectCfg.projectLevel || projectCfg.name != configuration.name {
					continue
				}
				for _, projectKey := range matchingBuildSettingKeys(projectCfg.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
					projectValue, ok := projectCfg.buildSettings[projectKey].(string)
					if !ok {
						continue
					}
					if strings.Contains(projectValue, "$(") || strings.Contains(projectValue, "${") {
						expanded, _, expandErr := resolver.expandDirectAssignmentWithContext(
							projectCfg,
							configuration,
							"CODE_SIGN_ENTITLEMENTS",
							projectKey,
							projectValue,
							map[string]bool{"CODE_SIGN_ENTITLEMENTS": true},
						)
						if expandErr != nil {
							// The later project-expression scan applies the same
							// fail-closed lexical handling for an unresolved seed.
							// Do not invent a composed path here when the target
							// context cannot resolve the project reference.
							continue
						}
						projectValue = expanded
					}
					composed := strings.ReplaceAll(value, "$(inherited)", projectValue)
					composed = strings.ReplaceAll(composed, "${inherited}", projectValue)
					if err := appendResolvedEntitlements(configuration, key, composed, false); err != nil {
						if lexicalErr := appendLexicalEntitlementCandidate(composed); lexicalErr != nil {
							return nil, externalEntitlementPaths, inputBlockers, lexicalErr
						}
						if selected {
							return nil, externalEntitlementPaths, inputBlockers, err
						}
						inputBlockers = append(inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved inherited CODE_SIGN_ENTITLEMENTS input: %v", configuration.target, configuration.name, err))
					}
				}
				projectFiles := configFiles[projectCfg.id]
				if signingConfigurationDefinesUnconditionalPBXEntitlement(projectCfg) {
					continue
				}
				if len(projectFiles) == 0 {
					continue
				}
				projectValues, projectErr := signingProjectXCConfigEntitlementValues(projectCfg, configuration, projectFiles, resolver)
				if projectErr != nil {
					return nil, externalEntitlementPaths, inputBlockers, fmt.Errorf("resolve project-level CODE_SIGN_ENTITLEMENTS for target %q configuration %q: %w", configuration.target, configuration.name, projectErr)
				}
				for _, projectValue := range projectValues {
					composed := strings.ReplaceAll(value, "$(inherited)", projectValue)
					composed = strings.ReplaceAll(composed, "${inherited}", projectValue)
					if err := appendResolvedEntitlements(configuration, key, composed, false); err != nil {
						if lexicalErr := appendLexicalEntitlementCandidate(composed); lexicalErr != nil {
							return nil, externalEntitlementPaths, inputBlockers, lexicalErr
						}
						if selected {
							return nil, externalEntitlementPaths, inputBlockers, err
						}
						inputBlockers = append(inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved inherited CODE_SIGN_ENTITLEMENTS input: %v", configuration.target, configuration.name, err))
					}
				}
			}
		}
		targetDefinesUnconditional := signingConfigurationDefinesUnconditionalEntitlement(project, configuration, configFiles, resolver)
		if targetDefinesUnconditional {
			continue
		}
		for _, projectCfg := range project.configurations {
			if !projectCfg.projectLevel || projectCfg.name != configuration.name {
				continue
			}
			for _, key := range matchingBuildSettingKeys(projectCfg.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
				value, ok := projectCfg.buildSettings[key].(string)
				if !ok || (!strings.Contains(value, "$(") && !strings.Contains(value, "${")) {
					continue
				}
				expanded, _, err := resolver.expandDirectAssignmentWithContext(
					projectCfg,
					configuration,
					"CODE_SIGN_ENTITLEMENTS",
					key,
					value,
					map[string]bool{"CODE_SIGN_ENTITLEMENTS": true},
				)
				if err == nil {
					err = appendEntitlements(expanded)
				}
				if err != nil {
					if lexicalErr := appendLexicalEntitlementCandidate(value); lexicalErr != nil {
						return nil, externalEntitlementPaths, inputBlockers, lexicalErr
					}
					if isConditionalEntitlementKey(key) {
						if !selected {
							inputBlockers = append(inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved inherited project CODE_SIGN_ENTITLEMENTS input; signing scope is uncertain", configuration.target, configuration.name))
							continue
						}
						return nil, externalEntitlementPaths, inputBlockers, newSigningConditionalEntitlementError(err)
					}
					if selected {
						return nil, externalEntitlementPaths, inputBlockers, err
					}
					inputBlockers = append(inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved inherited project CODE_SIGN_ENTITLEMENTS input: %v", configuration.target, configuration.name, err))
				}
			}
		}
	}
	entitlementCandidatesByConfiguration := make(map[string]map[string]map[int]bool)
	entitlementAssignmentExpansionsByConfiguration := make(map[string]map[string]map[int]signingXCConfigEntitlementAssignmentExpansion)
	configurationsByID := make(map[string]*versionConfiguration, len(project.configurations))
	for _, configuration := range project.configurations {
		configurationsByID[configuration.id] = configuration
	}
	for configurationID := range uncertainEntitlementConfigurations {
		files := configFiles[configurationID]
		if len(files) == 0 {
			continue
		}
		base := xcconfigResolvedValue{}
		if configuration := configurationsByID[configurationID]; configuration != nil {
			// Reuse the same lower-layer resolution state as the effective
			// resolver. If that state is itself uncertain, retaining all
			// candidates is the fail-closed choice.
			resolvedBase, baseErr := resolver.resolveXCConfigBaseWithContext(
				configuration,
				configuration,
				files[0],
				"CODE_SIGN_ENTITLEMENTS",
			)
			if baseErr == nil {
				base = resolvedBase
			}
		}
		candidates, composed, expansions, err := signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
			configurationsByID[configurationID], files, resolver, base,
		)
		if err != nil {
			return nil, externalEntitlementPaths, inputBlockers, err
		}
		entitlementCandidatesByConfiguration[configurationID] = candidates
		entitlementAssignmentExpansionsByConfiguration[configurationID] = expansions
		configuration := configurationsByID[configurationID]
		for _, value := range composed {
			if err := appendResolvedEntitlements(configuration, "CODE_SIGN_ENTITLEMENTS", value, true); err != nil {
				if lexicalErr := appendLexicalEntitlementCandidate(value); lexicalErr != nil {
					return nil, externalEntitlementPaths, inputBlockers, lexicalErr
				}
			}
		}
	}
	knownConfigurationIDs := make(map[string]bool, len(project.configurations))
	projectEntitlementCandidatesByConfiguration := make(map[string]map[string]map[int]bool)
	for _, configuration := range project.configurations {
		knownConfigurationIDs[configuration.id] = true
		files, ok := configFiles[configuration.id]
		if !ok {
			continue
		}
		for _, filePath := range files {
			// These paths came from the successful collector, but keep the
			// membership and authorization checks on this later read as well.
			// This prevents a future caller from turning configFiles into an
			// ambient path list that bypasses the signing resolver's rooted,
			// no-follow policy.
			data, err := resolver.readXCConfig(filePath)
			if err != nil {
				return nil, externalEntitlementPaths, inputBlockers, fmt.Errorf("read xcconfig %s: %w", filePath, err)
			}
			document, err := parseXCConfig(data)
			if err != nil {
				return nil, externalEntitlementPaths, inputBlockers, fmt.Errorf("parse xcconfig %s: %w", filePath, err)
			}
			for _, assignment := range document.assignments {
				selected := selectedIDs[configuration.id]
				selectedSource := selectedXCConfigSources[signingXCConfigOperationKey(filePath, fileIdentities)]
				if assignment.continued && allowedSigningSetting(assignment.baseKey) &&
					(assignment.baseKey == "CODE_SIGN_ENTITLEMENTS" || selectedSource) {
					return nil, externalEntitlementPaths, inputBlockers, fmt.Errorf("xcconfig %s uses a line continuation for signing setting %s", filePath, assignment.baseKey)
				}
				if resolvedEntitlementConfigurations[configuration.id] && assignment.baseKey == "CODE_SIGN_ENTITLEMENTS" {
					continue
				}
				if assignment.baseKey != "CODE_SIGN_ENTITLEMENTS" {
					continue
				}
				if configuration.projectLevel && signingConfigurationDefinesUnconditionalPBXEntitlement(configuration) {
					continue
				}
				if configuration.projectLevel {
					candidates, cached := projectEntitlementCandidatesByConfiguration[configuration.id]
					if !cached {
						var candidateErr error
						candidates, _, _, candidateErr = signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
							configuration,
							files,
							resolver,
							xcconfigResolvedValue{},
						)
						if candidateErr != nil {
							return nil, externalEntitlementPaths, inputBlockers, candidateErr
						}
						projectEntitlementCandidatesByConfiguration[configuration.id] = candidates
					}
					if !candidates[normalizeSigningLexicalPath(filePath)][assignment.lineIndex] {
						continue
					}
				}
				if configuration.projectLevel && (strings.Contains(assignment.value, "$(") || strings.Contains(assignment.value, "${")) {
					expandedForTarget := false
					targetExpansionFailed := false
					matchingTargetCount := 0
					shadowedTargetCount := 0
					for _, targetConfiguration := range project.configurations {
						if targetConfiguration.projectLevel || targetConfiguration.name != configuration.name {
							continue
						}
						matchingTargetCount++
						if signingConfigurationDefinesUnconditionalEntitlement(project, targetConfiguration, configFiles, resolver) {
							shadowedTargetCount++
							continue
						}
						expanded, _, expandErr := resolver.expandDirectAssignmentWithSourceContext(
							configuration,
							targetConfiguration,
							"CODE_SIGN_ENTITLEMENTS",
							assignment.key,
							assignment.value,
							map[string]bool{"CODE_SIGN_ENTITLEMENTS": true},
							true,
						)
						if expandErr != nil {
							targetExpansionFailed = true
							continue
						}
						projectSeeds, projectSeedErr := signingProjectPBXInheritedEntitlementValues(
							configuration,
							targetConfiguration,
							expanded,
							resolver,
						)
						if projectSeedErr != nil {
							targetExpansionFailed = true
							continue
						}
						for _, projectSeed := range projectSeeds {
							targetValues, targetInherited, targetValueErr := signingConfigurationInheritedEntitlementValues(
								targetConfiguration,
								configFiles,
								resolver,
								projectSeed,
							)
							if targetValueErr != nil {
								targetExpansionFailed = true
								continue
							}
							if !targetInherited {
								targetValues = []string{projectSeed}
							}
							for _, targetValue := range targetValues {
								if err := appendEntitlements(targetValue); err != nil {
									return nil, externalEntitlementPaths, inputBlockers, err
								}
							}
						}
						expandedForTarget = true
					}
					if matchingTargetCount > 0 && shadowedTargetCount == matchingTargetCount {
						continue
					}
					if targetExpansionFailed {
						if lexicalErr := appendLexicalEntitlementCandidate(assignment.value); lexicalErr != nil {
							return nil, externalEntitlementPaths, inputBlockers, lexicalErr
						}
						continue
					}
					if expandedForTarget {
						continue
					}
				}
				if !isConditionalEntitlementKey(assignment.key) {
					if !uncertainEntitlementConfigurations[configuration.id] {
						continue
					}
					if !entitlementCandidatesByConfiguration[configuration.id][normalizeSigningLexicalPath(filePath)][assignment.lineIndex] {
						continue
					}
				} else if uncertainEntitlementConfigurations[configuration.id] &&
					!entitlementCandidatesByConfiguration[configuration.id][normalizeSigningLexicalPath(filePath)][assignment.lineIndex] {
					continue
				}
				// The xcconfig layer sits below these PBX settings, so the
				// object's unconditional assignment is not this assignment's
				// inherited base. Selector-aware composition inside an xcconfig
				// is handled by signingXCConfigEntitlementAssignmentCandidates.
				// For an uncertain configuration, that pass has already expanded
				// every live assignment against the preceding xcconfig/project
				// state. Re-resolving an inherited raw assignment through the
				// configuration's whole xcconfig would use the final value of this
				// same file as its base and compose the suffix twice.
				rawValue := assignment.value
				if expansion := entitlementAssignmentExpansionsByConfiguration[configuration.id][normalizeSigningLexicalPath(filePath)][assignment.lineIndex]; expansion.inheritedResolved {
					rawValue = expansion.value
				}
				if err := appendResolvedEntitlements(configuration, "CODE_SIGN_ENTITLEMENTS", rawValue, true); err != nil {
					if lexicalErr := appendLexicalEntitlementCandidate(assignment.value); lexicalErr != nil {
						return nil, externalEntitlementPaths, inputBlockers, lexicalErr
					}
					if !selected {
						inputBlockers = append(inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved conditional CODE_SIGN_ENTITLEMENTS input; signing scope is uncertain", configuration.target, configuration.name))
						continue
					}
					return nil, externalEntitlementPaths, inputBlockers, newSigningConditionalEntitlementError(err)
				}
			}
		}
	}
	// configFiles is normally keyed only by configurations parsed from the
	// project. Keep this boundary fail-closed for injected or future collector
	// results as well: an unassociated source cannot be safely classified as an
	// unselected consumer, so it must remain readable and representable rather
	// than being silently omitted from the protected-input inventory.
	for configurationID, files := range configFiles {
		if knownConfigurationIDs[configurationID] {
			continue
		}
		for _, filePath := range files {
			data, err := resolver.readXCConfig(filePath)
			if err != nil {
				return nil, externalEntitlementPaths, inputBlockers, fmt.Errorf("read xcconfig %s: %w", filePath, err)
			}
			document, err := parseXCConfig(data)
			if err != nil {
				return nil, externalEntitlementPaths, inputBlockers, fmt.Errorf("parse xcconfig %s: %w", filePath, err)
			}
			for _, assignment := range document.assignments {
				if assignment.continued && allowedSigningSetting(assignment.baseKey) {
					return nil, externalEntitlementPaths, inputBlockers, fmt.Errorf("xcconfig %s uses a line continuation for signing setting %s", filePath, assignment.baseKey)
				}
				if assignment.baseKey != "CODE_SIGN_ENTITLEMENTS" {
					continue
				}
				if strings.Contains(assignment.value, "$(") || strings.Contains(assignment.value, "${") {
					if isConditionalEntitlementKey(assignment.key) {
						inputBlockers = append(inputBlockers, fmt.Sprintf("xcconfig %s has an unresolved conditional CODE_SIGN_ENTITLEMENTS input; signing scope is uncertain", filePath))
						continue
					}
				}
				if err := appendEntitlements(assignment.value); err != nil {
					return nil, externalEntitlementPaths, inputBlockers, err
				}
			}
		}
	}
	return paths, externalEntitlementPaths, inputBlockers, nil
}

// signingXCConfigSelectorIdentity canonicalizes only the order of bracketed
// selector conditions. Selector names and values remain byte-for-byte (and
// therefore case- and wildcard-) sensitive, because those details affect the
// build context in which Xcode applies an assignment.
func signingXCConfigSelectorIdentity(key string) string {
	base := xcconfigBaseKey(key)
	remainder := key[len(base):]
	if remainder == "" {
		return key
	}
	selectors := make([]string, 0, 2)
	for remainder != "" {
		if remainder[0] != '[' {
			return key
		}
		end := strings.IndexByte(remainder, ']')
		if end < 0 {
			return key
		}
		selectors = append(selectors, remainder[:end+1])
		remainder = remainder[end+1:]
	}
	sort.Strings(selectors)
	return base + strings.Join(selectors, "")
}

type signingXCConfigEntitlementAssignmentExpansion struct {
	value             string
	inheritedResolved bool
}

// signingProjectXCConfigEntitlementValues returns the concrete project-level
// entitlement values that an inherited target assignment can consume. The
// source configuration owns the xcconfig graph while the target configuration
// owns build-setting reference expansion; keeping those contexts separate
// prevents a target's xcconfig from being combined with a sibling project
// configuration while still resolving references such as $(PRODUCT_NAME) for
// the target that will inherit the value.
func signingProjectXCConfigEntitlementValues(
	projectConfiguration, targetConfiguration *versionConfiguration,
	files []string,
	resolver *signingSettingResolver,
) ([]string, error) {
	_, composed, _, err := signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
		projectConfiguration,
		files,
		resolver,
		xcconfigResolvedValue{},
	)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(composed))
	seen := make(map[string]bool, len(composed))
	for _, value := range composed {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.Contains(value, "$(") || strings.Contains(value, "${") {
			expanded, _, expandErr := resolver.expandSettingReferences(
				targetConfiguration,
				value,
				map[string]bool{"CODE_SIGN_ENTITLEMENTS": true},
			)
			if expandErr != nil {
				// The ordinary project xcconfig scan retains the raw assignment
				// for lexical/fail-closed handling when target context cannot
				// expand a reference. Do not guess a composed path here.
				continue
			}
			value = expanded
		}
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	if signingConfigurationDefinesUnconditionalPBXEntitlement(projectConfiguration) {
		return nil, nil
	}
	projectComposed := make([]string, 0, len(values))
	seen = make(map[string]bool, len(values))
	for _, value := range values {
		projectValues, err := signingProjectPBXInheritedEntitlementValues(
			projectConfiguration,
			targetConfiguration,
			value,
			resolver,
		)
		if err != nil {
			return nil, err
		}
		for _, projectValue := range projectValues {
			if !seen[projectValue] {
				seen[projectValue] = true
				projectComposed = append(projectComposed, projectValue)
			}
		}
	}
	return projectComposed, nil
}

type signingPBXInheritedEntitlementAssignment struct {
	key   string
	value string
}

// signingPBXInheritedEntitlementValues composes raw PBX inherited
// assignments through a supplied lower-layer value. Xcode evaluates a
// conditional assignment's $(inherited) against the same object's effective
// unconditional slot, so conditional assignments must consume the computed
// unconditional values rather than the lower layer directly. The raw key is
// retained for the resolver call; this preserves its assignment-aware
// recursion and avoids a second syntax-level composition pass.
func signingPBXInheritedEntitlementValues(
	configuration, expansionConfiguration *versionConfiguration,
	lowerValues []string,
	resolver *signingSettingResolver,
) ([]string, error) {
	normalizedLowerValues := make([]string, 0, len(lowerValues))
	seenLower := make(map[string]bool, len(lowerValues))
	for _, value := range lowerValues {
		value = strings.TrimSpace(value)
		if !seenLower[value] {
			seenLower[value] = true
			normalizedLowerValues = append(normalizedLowerValues, value)
		}
	}
	if len(normalizedLowerValues) == 0 {
		normalizedLowerValues = []string{""}
	}

	assignments := make([]signingPBXInheritedEntitlementAssignment, 0, len(configuration.buildSettings))
	for _, key := range matchingBuildSettingKeys(configuration.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
		value, ok := configuration.buildSettings[key].(string)
		if ok && signingValueInherits(value) {
			assignments = append(assignments, signingPBXInheritedEntitlementAssignment{key: key, value: value})
		}
	}
	if len(assignments) == 0 {
		return normalizedLowerValues, nil
	}

	expand := func(assignment signingPBXInheritedEntitlementAssignment, lower string) (string, error) {
		seededConfiguration := *configuration
		seededConfiguration.buildSettings = cloneSigningSerializedObject(configuration.buildSettings)
		seededConfiguration.buildSettings["CODE_SIGN_ENTITLEMENTS"] = lower
		expanded, _, err := resolver.expandDirectAssignmentWithContext(
			&seededConfiguration,
			expansionConfiguration,
			"CODE_SIGN_ENTITLEMENTS",
			assignment.key,
			assignment.value,
			map[string]bool{"CODE_SIGN_ENTITLEMENTS": true},
		)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(expanded), nil
	}

	// The unconditional assignment establishes the same-object state that a
	// conditional assignment inherits. With no unconditional assignment, a
	// conditional slot falls through to the supplied lower-layer values.
	unconditionalValues := make([]string, 0, len(normalizedLowerValues))
	seenUnconditional := make(map[string]bool, len(normalizedLowerValues))
	for _, assignment := range assignments {
		if assignment.key != "CODE_SIGN_ENTITLEMENTS" {
			continue
		}
		for _, lower := range normalizedLowerValues {
			expanded, err := expand(assignment, lower)
			if err != nil {
				return nil, err
			}
			if expanded != "" && !seenUnconditional[expanded] {
				seenUnconditional[expanded] = true
				unconditionalValues = append(unconditionalValues, expanded)
			}
		}
	}
	if len(unconditionalValues) == 0 {
		unconditionalValues = normalizedLowerValues
	}

	composed := make([]string, 0, len(assignments)*len(unconditionalValues))
	seen := make(map[string]bool, len(composed))
	for _, assignment := range assignments {
		if assignment.key == "CODE_SIGN_ENTITLEMENTS" {
			for _, value := range unconditionalValues {
				if !seen[value] {
					seen[value] = true
					composed = append(composed, value)
				}
			}
			continue
		}
		for _, lower := range unconditionalValues {
			expanded, err := expand(assignment, lower)
			if err != nil {
				return nil, err
			}
			if expanded != "" && !seen[expanded] {
				seen[expanded] = true
				composed = append(composed, expanded)
			}
		}
	}
	if len(composed) == 0 {
		return normalizedLowerValues, nil
	}
	return composed, nil
}

// signingProjectPBXInheritedEntitlementValues composes a project xcconfig
// seed through the project's raw PBX entitlement assignment. Project PBX
// inherited suffixes sit above the project xcconfig and therefore must be
// applied before a target's inherited suffix is considered.
func signingProjectPBXInheritedEntitlementValues(
	projectConfiguration, targetConfiguration *versionConfiguration,
	projectSeed string,
	resolver *signingSettingResolver,
) ([]string, error) {
	if signingConfigurationDefinesUnconditionalPBXEntitlement(projectConfiguration) {
		return nil, nil
	}
	return signingPBXInheritedEntitlementValues(
		projectConfiguration,
		targetConfiguration,
		[]string{projectSeed},
		resolver,
	)
}

// signingXCConfigInheritedEntitlementValues returns raw target-xcconfig
// entitlement assignments that remain live and still consume a lower-layer
// value. The candidate traversal supplies the same replacement semantics used
// by the uncertain-configuration inventory, so a shadowed assignment cannot
// re-enter target-context composition merely because it was parsed.
func signingXCConfigInheritedEntitlementValues(
	configuration *versionConfiguration,
	files []string,
	resolver *signingSettingResolver,
) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	candidates, _, _, err := signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
		configuration,
		files,
		resolver,
		xcconfigResolvedValue{},
	)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0)
	seen := make(map[string]bool)
	for _, filePath := range files {
		data, readErr := resolver.readXCConfig(filePath)
		if readErr != nil {
			return nil, readErr
		}
		document, parseErr := parseXCConfig(data)
		if parseErr != nil {
			return nil, fmt.Errorf("parse %s: %w", filePath, parseErr)
		}
		path := normalizeSigningLexicalPath(filePath)
		for _, assignment := range document.assignments {
			if assignment.baseKey != "CODE_SIGN_ENTITLEMENTS" ||
				(!signingValueInherits(assignment.value) && assignment.operator != "+=") ||
				!candidates[path][assignment.lineIndex] {
				continue
			}
			if !seen[assignment.value] {
				seen[assignment.value] = true
				values = append(values, assignment.value)
			}
		}
	}
	return values, nil
}

// signingConfigurationInheritedEntitlementValues composes one concrete
// project seed through the target's raw inherited entitlement sources. The
// target xcconfig graph is resolved with the supplied seed as its lower layer
// before any PBX inherited suffix is appended, which keeps a target-xcconfig
// suffix and a PBX suffix in their actual order and avoids protecting the bare
// project reference as a synthetic path.
func signingConfigurationInheritedEntitlementValues(
	configuration *versionConfiguration,
	configFiles map[string][]string,
	resolver *signingSettingResolver,
	projectSeed string,
) ([]string, bool, error) {
	values := []string{strings.TrimSpace(projectSeed)}
	targetXCConfigValues := values
	if files := configFiles[configuration.id]; len(files) > 0 {
		_, composed, _, err := signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
			configuration,
			files,
			resolver,
			xcconfigResolvedValue{value: projectSeed, found: true, exact: true},
		)
		if err != nil {
			return nil, false, err
		}
		targetXCConfigValues = make([]string, 0, len(composed))
		seen := make(map[string]bool, len(composed))
		for _, value := range composed {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if strings.Contains(value, "$(") || strings.Contains(value, "${") {
				value, _, err = resolver.expandSettingReferences(configuration, value, map[string]bool{"CODE_SIGN_ENTITLEMENTS": true})
				if err != nil {
					return nil, false, err
				}
			}
			if !seen[value] {
				seen[value] = true
				targetXCConfigValues = append(targetXCConfigValues, value)
			}
		}
		if len(targetXCConfigValues) == 0 {
			targetXCConfigValues = values
		}
	}

	if targetValues, err := signingXCConfigInheritedEntitlementValues(configuration, configFiles[configuration.id], resolver); err != nil {
		return nil, false, err
	} else {
		pbxInherited := make([]string, 0)
		for _, key := range matchingBuildSettingKeys(configuration.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
			value, ok := configuration.buildSettings[key].(string)
			if ok && signingValueInherits(value) {
				pbxInherited = append(pbxInherited, value)
			}
		}
		if len(pbxInherited) == 0 {
			return targetXCConfigValues, len(targetValues) > 0, nil
		}
		composed, err := signingPBXInheritedEntitlementValues(
			configuration,
			configuration,
			targetXCConfigValues,
			resolver,
		)
		if err != nil {
			return nil, true, err
		}
		return composed, true, nil
	}
}

// signingXCConfigEntitlementAssignmentCandidates returns the assignments that
// can still contribute to CODE_SIGN_ENTITLEMENTS when effective resolution is
// uncertain. It uses the configuration-scoped resolver's event traversal and
// rooted read/stat/identity callbacks; it does not replay a global path list or
// implement a second include walker.
func signingXCConfigEntitlementAssignmentCandidates(
	configuration *versionConfiguration,
	files []string,
	resolver *signingSettingResolver,
	base xcconfigResolvedValue,
) (map[string]map[int]bool, []string, error) {
	candidates, composed, _, err := signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
		configuration, files, resolver, base,
	)
	return candidates, composed, err
}

func signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
	configuration *versionConfiguration,
	files []string,
	resolver *signingSettingResolver,
	base xcconfigResolvedValue,
) (map[string]map[int]bool, []string, map[string]map[int]signingXCConfigEntitlementAssignmentExpansion, error) {
	type assignmentReference struct {
		path        string
		assignment  xcconfigAssignment
		selectorKey string
	}
	var active []assignmentReference
	// A found lower-layer value shadows a target-level ?= even when the
	// resolver's exact bit is false (for example, a value inherited from a
	// project xcconfig). A lower-layer resolution error intentionally supplies
	// an empty state so every target assignment remains protected.
	hasExactValue := base.found
	if len(files) == 0 {
		return map[string]map[int]bool{}, nil, nil, nil
	}
	var observerErr error
	observe := func(path string, assignment xcconfigAssignment) {
		if observerErr != nil || assignment.baseKey != "CODE_SIGN_ENTITLEMENTS" {
			return
		}
		canonicalPath, collected, err := resolver.configurationXCConfigPath(configuration, path)
		if err != nil {
			observerErr = err
			return
		}
		if !collected {
			observerErr = fmt.Errorf("xcconfig %s was not collected for this configuration", normalizeSigningLexicalPath(path))
			return
		}
		candidate := assignmentReference{
			path:        canonicalPath,
			assignment:  assignment,
			selectorKey: signingXCConfigSelectorIdentity(assignment.key),
		}
		if assignment.key == "CODE_SIGN_ENTITLEMENTS" {
			switch assignment.operator {
			case "?=":
				if hasExactValue {
					return
				}
			case "=":
				if !strings.Contains(assignment.value, "$(inherited)") && !strings.Contains(assignment.value, "${inherited}") {
					filtered := active[:0]
					for _, existing := range active {
						if existing.assignment.key == "CODE_SIGN_ENTITLEMENTS" || existing.assignment.operator == "?=" {
							continue
						}
						filtered = append(filtered, existing)
					}
					active = filtered
				}
			case "+=":
				// The prior exact value remains part of an append operation;
				// retain both path intents for conservative alias protection.
			}
			hasExactValue = true
			active = append(active, candidate)
			return
		}
		if assignment.operator == "?=" {
			if hasExactValue {
				return
			}
			for _, existing := range active {
				if existing.selectorKey == candidate.selectorKey {
					// A prior same-selector assignment wins over a later
					// conditional default; ?= never replaces a live value.
					return
				}
			}
		} else if assignment.operator != "+=" && !strings.Contains(assignment.value, "$(inherited)") && !strings.Contains(assignment.value, "${inherited}") {
			// An exact same-selector assignment replaces the earlier value.
			// Reordered conditions share selector identity, while different
			// selectors remain possible in other build contexts.
			filtered := active[:0]
			for _, existing := range active {
				if existing.selectorKey == candidate.selectorKey {
					continue
				}
				filtered = append(filtered, existing)
			}
			active = filtered
		}
		active = append(active, candidate)
	}
	resolved, _, err := resolver.resolveXCConfigSettingStateWithContext(
		configuration,
		files[0],
		"CODE_SIGN_ENTITLEMENTS",
		observe,
		base,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	if observerErr != nil {
		return nil, nil, nil, observerErr
	}
	candidates := make(map[string]map[int]bool)
	expansions := make(map[string]map[int]signingXCConfigEntitlementAssignmentExpansion)
	for _, candidate := range active {
		path := normalizeSigningLexicalPath(candidate.path)
		if candidates[path] == nil {
			candidates[path] = make(map[int]bool)
		}
		candidates[path][candidate.assignment.lineIndex] = true
		if expansions[path] == nil {
			expansions[path] = make(map[int]signingXCConfigEntitlementAssignmentExpansion)
		}
	}
	composed := make([]string, 0)
	if strings.TrimSpace(resolved.value) != "" {
		composed = append(composed, resolved.value)
	}
	accumulated := map[string]string{}
	accumulatedKnown := map[string]bool{}
	if base.found {
		accumulated["CODE_SIGN_ENTITLEMENTS"] = strings.TrimSpace(base.value)
		accumulatedKnown["CODE_SIGN_ENTITLEMENTS"] = true
	}
	lookupAccumulated := func(selector string) (string, bool) {
		previous, known := accumulated[selector], accumulatedKnown[selector]
		if !known && previous == "" && selector != "CODE_SIGN_ENTITLEMENTS" {
			previous = accumulated["CODE_SIGN_ENTITLEMENTS"]
			known = accumulatedKnown["CODE_SIGN_ENTITLEMENTS"]
		}
		return previous, known
	}
	for _, candidate := range active {
		selector := candidate.selectorKey
		value := strings.TrimSpace(candidate.assignment.value)
		inheritedResolved := false
		switch candidate.assignment.operator {
		case "+=":
			previous, previousKnown := lookupAccumulated(selector)
			if strings.Contains(value, "$(inherited)") || strings.Contains(value, "${inherited}") {
				inheritedResolved = previousKnown
				value = strings.ReplaceAll(value, "$(inherited)", previous)
				value = strings.ReplaceAll(value, "${inherited}", previous)
				accumulated[selector] = strings.TrimSpace(value)
				accumulatedKnown[selector] = true
				break
			}
			accumulated[selector] = strings.TrimSpace(strings.TrimSpace(previous) + " " + value)
			accumulatedKnown[selector] = true
		case "?=":
			if !accumulatedKnown[selector] {
				accumulated[selector] = value
				accumulatedKnown[selector] = true
			}
		default:
			if strings.Contains(value, "$(inherited)") || strings.Contains(value, "${inherited}") {
				previous, previousKnown := lookupAccumulated(selector)
				inheritedResolved = previousKnown
				value = strings.ReplaceAll(value, "$(inherited)", previous)
				value = strings.ReplaceAll(value, "${inherited}", previous)
			}
			accumulated[selector] = strings.TrimSpace(value)
			accumulatedKnown[selector] = true
		}
		path := normalizeSigningLexicalPath(candidate.path)
		if expansions[path] == nil {
			expansions[path] = make(map[int]signingXCConfigEntitlementAssignmentExpansion)
		}
		expansions[path][candidate.assignment.lineIndex] = signingXCConfigEntitlementAssignmentExpansion{
			value:             strings.TrimSpace(value),
			inheritedResolved: inheritedResolved,
		}
	}
	for _, value := range accumulated {
		if strings.TrimSpace(value) != "" {
			composed = append(composed, value)
		}
	}
	return candidates, composed, expansions, nil
}

func validateSigningXCConfigPath(project *structuredVersionProject, path string, allowExternal bool) error {
	// Do the lexical/platform check before opening any rooted handle. On
	// Windows, a case-sensitive directory can make a case-variant sibling look
	// internal to a global lowercase comparison; unknown case metadata is
	// intentionally treated as external and therefore requires explicit opt-in.
	if !signingPathLexicallyContained(project, path) && !allowExternal {
		return fmt.Errorf("xcconfig path %s is outside the project directory: %w", path, rootfs.ErrEscapesRoot)
	}
	root, err := rootfs.New(project.rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.AllowingInternalSymlinks().CheckContained(path); err == nil {
		return nil
	} else if !allowExternal {
		return err
	}
	externalRoot, err := rootfs.New(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer externalRoot.Close()
	return externalRoot.CheckContained(filepath.Base(path))
}

func signingFileDigest(path string) (string, error) {
	data, err := readSigningRegularFile(path, signingPlanMaxBytes)
	if err != nil {
		return "", err
	}
	return signingFileDigestBytes(data), nil
}

func signingFileDigestBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func readSigningRegularFile(path string, limit int64) ([]byte, error) {
	absolute, err := canonicalSigningPath(path, "file")
	if err != nil {
		return nil, err
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.ReadFileLimited(filepath.Base(absolute), limit)
}

// signingXCConfigReadFileFn keeps configuration reads behind the same rooted
// reader while allowing tests to prove that authorization rejects a path
// before any read is attempted. Production always uses readSigningRegularFile.
var signingXCConfigReadFileFn = readSigningRegularFile

// signingXCConfigStatFileFn keeps later authorization-aware existence checks
// behind the same rooted reader while allowing tests to prove that an
// unauthorized path is rejected before stat/open is attempted.
var signingXCConfigStatFileFn = signingRegularFileInfo

// signingXCConfigIdentityFn is used only after signing authorization has
// accepted a path. On Windows, per-directory case sensitivity means a global
// lowercase lexical key cannot establish whether two spellings name one file
// or two; the rooted file identity supplies that distinction.
var signingXCConfigIdentityFn = signingRegularFileInfo

// signingRegularFileInfo obtains metadata through the same rooted no-follow
// path policy used for signing reads. Callers must establish authorization
// before invoking it for any path that came from project configuration.
func signingRegularFileInfo(path string) (os.FileInfo, error) {
	absolute, err := canonicalSigningPath(path, "file")
	if err != nil {
		return nil, err
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if err := root.CheckCreateNewFile(filepath.Base(absolute)); err == nil {
		return nil, os.ErrNotExist
	} else if !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	file, err := root.OpenFile(filepath.Base(absolute))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}

// signingArtifactPathInfoFn is kept narrow so tests can inject a path
// inspection failure at the alias-validation boundary. Production alias
// checks always use signingRegularFileInfo's rooted, no-follow implementation.
var signingArtifactPathInfoFn = signingRegularFileInfo

// signingResolveProspectivePathFn keeps future-path alias resolution behind a
// narrow seam so tests can prove unauthorized external configuration paths are
// never inspected before opt-in. Production uses rootfs' parent-resolving,
// no-follow implementation.
var signingResolveProspectivePathFn = rootfs.ResolveProspectivePath

func signingPlanHash(plan *SigningPlan) string {
	copyPlan := *plan
	copyPlan.GeneratedAt = ""
	copyPlan.PlanHash = ""
	encoded, err := json.Marshal(copyPlan)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

// WriteSigningPlanArtifact atomically writes a plan artifact. Existing plans
// are replaced only when overwrite is explicitly requested.
func WriteSigningPlanArtifact(plan *SigningPlan, overwrite bool) error {
	if plan == nil {
		return fmt.Errorf("signing plan is nil")
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("encode signing plan: %w", err)
	}
	data = append(data, '\n')
	absolute, err := canonicalSigningPath(plan.PlanPath, "plan file")
	if err != nil {
		return err
	}
	parent := filepath.Dir(absolute)
	parentInfo, err := os.Lstat(parent)
	if err == nil && parentInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("write signing plan %s: %w", absolute, rootfs.ErrSymlink)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat signing plan parent %s: %w", parent, err)
	}
	root, err := rootfs.New(parent)
	if err != nil {
		return err
	}
	defer root.Close()
	name := filepath.Base(absolute)
	if overwrite {
		if err := root.WriteFile(name, data, 0o600); err != nil {
			return fmt.Errorf("write signing plan %s: %w", absolute, err)
		}
		return nil
	}
	if err := root.CreateNewFile(name, data, 0o600); err != nil {
		return fmt.Errorf("write signing plan %s: %w; use --overwrite to replace it", absolute, err)
	}
	return nil
}

// ApplySigningPlan verifies and applies a plan, then writes its receipt. No
// project write occurs until the plan hash and all source digests match a
// freshly resolved plan. Project files and the complete receipt are committed
// as one transaction so a receipt failure cannot leave signing settings
// partially applied.
func ApplySigningPlan(opts SigningApplyOptions) (*SigningApplyResult, error) {
	planPath, err := canonicalSigningPath(opts.PlanPath, "plan file")
	if err != nil {
		return nil, err
	}
	plan, err := readSigningPlanArtifact(planPath)
	if err != nil {
		return nil, err
	}
	if plan.PlanPath != planPath {
		return nil, fmt.Errorf("plan path does not match artifact location: %s", plan.PlanPath)
	}
	if plan.AllowExternalXCConfig != opts.AllowExternalXCConfig {
		return nil, fmt.Errorf("--allow-external-xcconfig does not match the plan")
	}
	if !plan.Ready {
		return nil, fmt.Errorf("plan is blocked: %s", strings.Join(plan.Blockers, "; "))
	}
	if plan.PlanHash == "" || plan.PlanHash != signingPlanHash(plan) {
		return nil, fmt.Errorf("plan hash is invalid")
	}

	built, err := buildSigningPlan(SigningPlanOptions{
		ProjectPath:           plan.ProjectPath,
		SettingsFilePath:      plan.SettingsFilePath,
		ProfilePaths:          plan.ProfilePaths,
		Configuration:         plan.Configuration,
		ExportMethod:          plan.ExportMethod,
		SkipTargets:           plan.SkipTargets,
		PlanPath:              plan.PlanPath,
		ReceiptPath:           plan.ReceiptPath,
		AllowExternalXCConfig: plan.AllowExternalXCConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("re-resolve signing plan: %w", err)
	}
	if !built.plan.Ready || built.plan.PlanHash != plan.PlanHash {
		return nil, fmt.Errorf("signing plan is stale; regenerate it before applying")
	}
	if err := preflightSigningReceipt(plan.ReceiptPath); err != nil {
		return nil, err
	}
	prepared, err := prepareSigningOperations(built)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = closeVersionWrites(prepared.writes)
		_ = prepared.projectRoot.Close()
	}()
	if beforeSigningCommitForTest != nil {
		beforeSigningCommitForTest()
	}
	if err := verifySigningPlanSources(plan, prepared.writes, built.fileIdentities); err != nil {
		return nil, fmt.Errorf("verify signing plan sources: %w", err)
	}
	fileChanges, err := signingReceiptFileChanges(plan, prepared.writes, prepared.changedFiles, built.fileIdentities)
	if err != nil {
		return nil, fmt.Errorf("prepare signing receipt: %w", err)
	}
	result := &SigningApplyResult{
		SchemaVersion: signingPlanSchemaVersion,
		AppliedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Completed:     true,
		PlanHash:      plan.PlanHash,
		PlanPath:      plan.PlanPath,
		ReceiptPath:   plan.ReceiptPath,
		ChangedFiles:  prepared.changedFiles,
		Files:         fileChanges,
		Changes:       append([]SigningSettingChange(nil), plan.Changes...),
	}
	receiptWrite, err := prepareSigningReceiptWrite(result)
	if err != nil {
		return nil, fmt.Errorf("prepare signing receipt: %w", err)
	}
	prepared.writes = append(prepared.writes, receiptWrite)
	verifyReceiptSources := func(committed []preparedVersionWrite) error {
		return verifySigningPlanSourcesBeforeReceipt(plan, committed, prepared.writes, built.fileIdentities)
	}
	if err := commitVersionWritesWithCreateChecks(prepared.writes, verifyReceiptSources, verifyReceiptSources); err != nil {
		return nil, fmt.Errorf("apply signing settings transaction: %w", err)
	}
	return result, nil
}

var beforeSigningCommitForTest func()

// verifySigningPlanSources closes the gap between re-resolution and staged
// publication. Every staged original must still match the digest captured in
// the plan, and every source must still match the bytes used to prepare the
// update before a receipt is even encoded.
func verifySigningPlanSources(plan *SigningPlan, writes []preparedVersionWrite, fileIdentities map[string]string) error {
	if plan == nil {
		return fmt.Errorf("signing plan is nil")
	}
	if err := verifySigningPlanMissingOptionalIncludes(plan); err != nil {
		return err
	}
	expected := make(map[string]SigningPlanFile, len(plan.Files))
	for _, file := range plan.Files {
		expected[signingXCConfigOperationKey(file.Path, fileIdentities)] = file
	}
	staged := make(map[string]preparedVersionWrite, len(writes))
	for _, write := range writes {
		if write.createOnly {
			continue
		}
		pathKey := signingXCConfigOperationKey(write.path, fileIdentities)
		file, ok := expected[pathKey]
		if !ok {
			return fmt.Errorf("signing plan is stale; staged source %s is not recorded in plan", write.path)
		}
		if signingFileDigestBytes(write.original) != file.SHA256 {
			return fmt.Errorf("signing plan is stale; staged source %s differs from plan", write.path)
		}
		staged[pathKey] = write
	}
	for _, file := range plan.Files {
		if write, ok := staged[signingXCConfigOperationKey(file.Path, fileIdentities)]; ok {
			if write.originalIdentity == nil {
				return fmt.Errorf("signing plan is stale; source %s has no captured identity", file.Path)
			}
			if err := write.root.CheckFileIdentity(write.name, write.originalIdentity); err != nil {
				return fmt.Errorf("signing plan is stale; source %s identity changed after preparation: %w", file.Path, err)
			}
			current := write.originalIdentity.Data()
			if !bytes.Equal(current, write.original) || signingFileDigestBytes(current) != file.SHA256 {
				return fmt.Errorf("signing plan is stale; source %s changed after preparation", file.Path)
			}
			continue
		}
		return fmt.Errorf("signing plan is stale; source %s has no prepared identity", file.Path)
	}
	return nil
}

// verifySigningPlanSourcesBeforeReceipt rechecks every planned source after
// ordinary project writes have completed and immediately before the receipt is
// published. Changed files are expected to contain this transaction's staged
// bytes; untouched files must still match their plan digest. A concurrent save
// in this final window therefore fails the transaction and is handled by the
// ordinary rollback path instead of receiving a misleading successful receipt.
func verifySigningPlanSourcesBeforeReceipt(plan *SigningPlan, committed, prepared []preparedVersionWrite, fileIdentities map[string]string) error {
	if plan == nil {
		return fmt.Errorf("signing plan is nil")
	}
	if err := verifySigningPlanMissingOptionalIncludes(plan); err != nil {
		return err
	}
	committedByPath := make(map[string]preparedVersionWrite, len(committed))
	for _, write := range committed {
		if !write.createOnly {
			committedByPath[signingXCConfigOperationKey(write.path, fileIdentities)] = write
		}
	}
	preparedByPath := make(map[string]preparedVersionWrite, len(prepared))
	for _, write := range prepared {
		if !write.createOnly {
			preparedByPath[signingXCConfigOperationKey(write.path, fileIdentities)] = write
		}
	}
	for _, file := range plan.Files {
		if write, ok := committedByPath[signingXCConfigOperationKey(file.Path, fileIdentities)]; ok {
			if write.committedIdentity == nil {
				return fmt.Errorf("written source %s has no committed identity", file.Path)
			}
			if err := write.root.CheckFileIdentity(write.name, write.committedIdentity); err != nil {
				return fmt.Errorf("written source %s identity changed before receipt: %w", file.Path, err)
			}
			// CheckFileIdentity validates the current rooted entry and its bytes
			// against the retained publication token. Reuse the token's immutable
			// snapshot instead of opening the path again between identity and
			// content checks, which would reintroduce a same-content replacement
			// window.
			current := write.committedIdentity.Data()
			if !bytes.Equal(current, write.updated) || signingFileDigestBytes(current) != signingFileDigestBytes(write.updated) {
				return fmt.Errorf("written source %s changed before receipt", file.Path)
			}
			continue
		}
		write, ok := preparedByPath[signingXCConfigOperationKey(file.Path, fileIdentities)]
		if !ok || write.originalIdentity == nil {
			return fmt.Errorf("source %s has no captured identity before receipt", file.Path)
		}
		if err := write.root.CheckFileIdentity(write.name, write.originalIdentity); err != nil {
			return fmt.Errorf("source %s identity changed before receipt: %w", file.Path, err)
		}
		current := write.originalIdentity.Data()
		if signingFileDigestBytes(current) != file.SHA256 {
			return fmt.Errorf("source %s differs from plan before receipt", file.Path)
		}
	}
	return nil
}

// verifySigningPlanMissingOptionalIncludes rechecks the lexical absence
// assertions captured during collection. It deliberately performs only a
// rooted no-follow create preflight: an optional include that appeared after
// preparation is a source race, regardless of whether the new entry is a
// regular file, directory, or symlink.
func verifySigningPlanMissingOptionalIncludes(plan *SigningPlan) error {
	if len(plan.MissingOptionalIncludes) == 0 {
		return nil
	}
	if len(plan.MissingOptionalIncludes) > signingPlanMaxMissingOptionalIncludes {
		return fmt.Errorf("signing plan contains too many missing optional xcconfig includes")
	}
	for _, path := range plan.MissingOptionalIncludes {
		absolute, err := canonicalSigningPath(path, "optional xcconfig include")
		if err != nil {
			return fmt.Errorf("verify optional xcconfig include absence: %w", err)
		}
		root, err := rootfs.New(filepath.Dir(absolute))
		if err != nil {
			return fmt.Errorf("verify optional xcconfig include absence: %w", err)
		}
		err = root.CheckCreateNewFile(filepath.Base(absolute))
		closeErr := root.Close()
		if err != nil {
			return fmt.Errorf("optional xcconfig include appeared before signing commit: %w", err)
		}
		if closeErr != nil {
			return fmt.Errorf("verify optional xcconfig include absence: %w", closeErr)
		}
	}
	return nil
}

func signingReceiptFileChanges(plan *SigningPlan, writes []preparedVersionWrite, changedFiles []string, fileIdentities map[string]string) ([]SigningFileChange, error) {
	before := make(map[string]SigningPlanFile, len(plan.Files))
	for _, file := range plan.Files {
		before[signingXCConfigOperationKey(file.Path, fileIdentities)] = file
	}
	updated := make(map[string][]byte, len(writes))
	for _, write := range writes {
		if write.createOnly || bytes.Equal(write.original, write.updated) {
			continue
		}
		updated[signingXCConfigOperationKey(write.path, fileIdentities)] = write.updated
	}
	changes := make([]SigningFileChange, 0, len(changedFiles))
	for _, path := range changedFiles {
		pathKey := signingXCConfigOperationKey(path, fileIdentities)
		file, ok := before[pathKey]
		if !ok {
			return nil, fmt.Errorf("changed file %s was not present in the plan", path)
		}
		data, ok := updated[pathKey]
		if !ok {
			return nil, fmt.Errorf("changed file %s was not prepared for the transaction", path)
		}
		changes = append(changes, SigningFileChange{
			Path:         path,
			Source:       file.Source,
			BeforeSHA256: file.SHA256,
			AfterSHA256:  signingFileDigestBytes(data),
		})
	}
	return changes, nil
}

func readSigningPlanArtifact(path string) (*SigningPlan, error) {
	data, err := readSigningRegularFile(path, signingPlanMaxBytes)
	if err != nil {
		return nil, fmt.Errorf("read signing plan %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var plan SigningPlan
	if err := decoder.Decode(&plan); err != nil {
		return nil, newSigningInputError(fmt.Errorf("decode signing plan %s: %w", path, err))
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, newSigningInputError(fmt.Errorf("decode signing plan %s: multiple JSON values", path))
		}
		return nil, newSigningInputError(fmt.Errorf("decode signing plan %s: %w", path, err))
	}
	if plan.SchemaVersion != signingPlanSchemaVersion {
		return nil, newSigningInputError(fmt.Errorf("plan schemaVersion must be %d", signingPlanSchemaVersion))
	}
	if plan.Command != signingPlanCommand {
		return nil, newSigningInputError(fmt.Errorf("plan command is not %q", signingPlanCommand))
	}
	if len(plan.Files) > signingPlanMaxFiles {
		return nil, newSigningInputError(fmt.Errorf("plan contains %d signing source files, exceeding the limit of %d", len(plan.Files), signingPlanMaxFiles))
	}
	if len(plan.MissingOptionalIncludes) > signingPlanMaxMissingOptionalIncludes {
		return nil, newSigningInputError(fmt.Errorf("plan contains too many missing optional xcconfig includes"))
	}
	for _, includePath := range plan.MissingOptionalIncludes {
		if len(includePath) > signingPlanMaxMissingOptionalIncludePathBytes {
			return nil, newSigningInputError(fmt.Errorf("plan contains an oversized missing optional xcconfig include path"))
		}
		if _, err := canonicalSigningPath(includePath, "optional xcconfig include"); err != nil {
			return nil, newSigningInputError(fmt.Errorf("plan contains an invalid missing optional xcconfig include path: %w", err))
		}
	}
	return &plan, nil
}

func preflightSigningReceipt(path string) error {
	absolute, err := canonicalSigningPath(path, "receipt file")
	if err != nil {
		return err
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.CheckCreateNewFile(filepath.Base(absolute)); err != nil {
		return fmt.Errorf("preflight receipt %s: %w", absolute, err)
	}
	return nil
}

func encodeSigningReceipt(result *SigningApplyResult) ([]byte, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func prepareSigningReceiptWrite(result *SigningApplyResult) (preparedVersionWrite, error) {
	data, err := encodeSigningReceipt(result)
	if err != nil {
		return preparedVersionWrite{}, err
	}
	absolute, err := canonicalSigningPath(result.ReceiptPath, "receipt file")
	if err != nil {
		return preparedVersionWrite{}, err
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return preparedVersionWrite{}, err
	}
	name := filepath.Base(absolute)
	if err := root.CheckCreateNewFile(name); err != nil {
		_ = root.Close()
		return preparedVersionWrite{}, err
	}
	return preparedVersionWrite{
		path:       absolute,
		updated:    data,
		mode:       0o600,
		root:       root,
		name:       name,
		createOnly: true,
	}, nil
}

type preparedSigningOperations struct {
	writes       []preparedVersionWrite
	changedFiles []string
	projectRoot  rootfs.Root
}

func prepareSigningOperations(built *signingPlanBuild) (*preparedSigningOperations, error) {
	if built == nil || built.project == nil {
		return nil, fmt.Errorf("signing plan build is nil")
	}
	project := built.project
	projectRoot, err := rootfs.New(project.rootDir)
	if err != nil {
		return nil, err
	}
	projectRoot = projectRoot.AllowingInternalSymlinks()
	prepared := &preparedSigningOperations{projectRoot: projectRoot}
	fail := func(failure error) (*preparedSigningOperations, error) {
		_ = closeVersionWrites(prepared.writes)
		_ = prepared.projectRoot.Close()
		return nil, failure
	}
	pbxprojChanged := false
	for _, operation := range built.operations {
		if operation.Source != "pbxproj" {
			continue
		}
		if err := applySigningPBXOperation(operation); err != nil {
			return fail(err)
		}
		pbxprojChanged = true
	}
	if pbxprojChanged {
		write, err := project.preparePBXProjWrite(projectRoot, true)
		if err != nil {
			return fail(err)
		}
		write.preserveMetadata = true
		prepared.writes = append(prepared.writes, write)
	}

	xcconfigMutations := make(map[string]map[string]xcconfigMutation)
	xcconfigPaths := make(map[string]string)
	for _, operation := range built.operations {
		if operation.Source != "xcconfig" {
			continue
		}
		if operation.NewValue == nil {
			return fail(fmt.Errorf("xcconfig removal is not supported for %s", operation.Setting))
		}
		pathKey := signingXCConfigOperationKey(operation.Path, built.fileIdentities)
		if xcconfigMutations[pathKey] == nil {
			xcconfigMutations[pathKey] = make(map[string]xcconfigMutation)
			xcconfigPaths[pathKey] = operation.Path
		}
		mutation := xcconfigMutations[pathKey][operation.Setting]
		if mutation.setting != "" && mutation.value != *operation.NewValue {
			return fail(fmt.Errorf("conflicting xcconfig operations for %s", operation.Path))
		}
		mutation.setting = operation.Setting
		mutation.value = *operation.NewValue
		mutation.configurations = appendVersionConfigurationOnce(mutation.configurations, operation.configuration)
		xcconfigMutations[pathKey][operation.Setting] = mutation
	}
	paths := make([]string, 0, len(xcconfigMutations))
	for pathKey := range xcconfigMutations {
		paths = append(paths, pathKey)
	}
	sort.Slice(paths, func(left, right int) bool {
		return xcconfigPaths[paths[left]] < xcconfigPaths[paths[right]]
	})
	for _, pathKey := range paths {
		path := xcconfigPaths[pathKey]
		target, err := project.versionFileTarget(projectRoot, path, built.plan.AllowExternalXCConfig)
		if err != nil {
			if target.root.Path() != "" && target.ownsRoot {
				_ = target.root.Close()
			}
			return fail(fmt.Errorf("prepare xcconfig %s: %w", path, err))
		}
		target.strictIdentity = true
		write, _, changed, err := prepareXCConfigWrite(target, xcconfigMutations[pathKey])
		if err != nil {
			_ = target.root.Close()
			return fail(err)
		}
		if changed {
			write.strictIdentity = true
			write.preserveMetadata = true
			prepared.writes = append(prepared.writes, write)
		} else if target.ownsRoot {
			_ = target.root.Close()
		}
	}
	// Preserve metadata for every ordinary destination before any transaction
	// can begin. WriteFilePreservingMode repeats these checks at commit time,
	// but preflighting the complete set keeps a late hard-link, symlink, or
	// ownership/permission failure from occurring after an earlier file was
	// already replaced.
	for _, write := range prepared.writes {
		if !write.preserveMetadata {
			continue
		}
		if err := write.root.CheckWriteFilePreservingMode(write.name); err != nil {
			return fail(fmt.Errorf("preflight metadata preservation for %s: %w", write.path, err))
		}
	}
	prepared.changedFiles = make([]string, 0, len(prepared.writes))
	for _, write := range prepared.writes {
		if !write.createOnly && !bytes.Equal(write.original, write.updated) {
			prepared.changedFiles = append(prepared.changedFiles, write.path)
		}
	}
	sort.Strings(prepared.changedFiles)
	watches, err := prepareSigningSourceWatches(built.plan, prepared.writes, built.fileIdentities)
	if err != nil {
		return fail(err)
	}
	prepared.writes = append(prepared.writes, watches...)
	return prepared, nil
}

// prepareSigningSourceWatches captures descriptor-backed identities for every
// plan input that is not already represented by a prepared write. The watches
// stay open through receipt publication so a same-content pathname replacement
// remains observable during both the pre- and post-publication checks.
func prepareSigningSourceWatches(plan *SigningPlan, writes []preparedVersionWrite, fileIdentities map[string]string) ([]preparedVersionWrite, error) {
	if plan == nil {
		return nil, fmt.Errorf("signing plan is nil")
	}
	seen := make(map[string]struct{}, len(writes)+len(plan.Files))
	for _, write := range writes {
		if !write.createOnly {
			seen[signingXCConfigOperationKey(write.path, fileIdentities)] = struct{}{}
		}
	}
	var watches []preparedVersionWrite
	fail := func(err error) ([]preparedVersionWrite, error) {
		return nil, errors.Join(err, closeVersionWrites(watches))
	}
	for _, file := range plan.Files {
		key := signingXCConfigOperationKey(file.Path, fileIdentities)
		if _, ok := seen[key]; ok {
			continue
		}
		absolute, err := canonicalSigningPath(file.Path, "signing source")
		if err != nil {
			return fail(err)
		}
		root, err := rootfs.New(filepath.Dir(absolute))
		if err != nil {
			return fail(fmt.Errorf("capture signing source %s: %w", file.Path, err))
		}
		identity, err := root.CaptureFileLimited(filepath.Base(absolute), signingPlanMaxBytes)
		if err != nil {
			_ = root.Close()
			return fail(fmt.Errorf("capture signing source %s: %w", file.Path, err))
		}
		data := identity.Data()
		if signingFileDigestBytes(data) != file.SHA256 {
			_ = root.Close()
			return fail(fmt.Errorf("signing plan is stale; source %s differs from plan", file.Path))
		}
		watches = append(watches, preparedVersionWrite{
			path:             absolute,
			original:         data,
			updated:          bytes.Clone(data),
			mode:             identity.Mode(),
			root:             root,
			name:             filepath.Base(absolute),
			ownsRoot:         true,
			originalIdentity: identity,
			strictIdentity:   true,
		})
		seen[key] = struct{}{}
	}
	return watches, nil
}

func applySigningPBXOperation(operation signingPlanOperation) error {
	configuration := operation.configuration
	if configuration == nil {
		return fmt.Errorf("missing configuration for %s", operation.Setting)
	}
	keys := matchingBuildSettingKeys(configuration.buildSettings, operation.Setting)
	if operation.Operation == "remove" {
		for _, key := range keys {
			delete(configuration.buildSettings, key)
		}
		return nil
	}
	if operation.NewValue == nil {
		return fmt.Errorf("missing new value for %s", operation.Setting)
	}
	if len(keys) == 0 {
		configuration.buildSettings[operation.Setting] = *operation.NewValue
		return nil
	}
	for _, key := range keys {
		configuration.buildSettings[key] = *operation.NewValue
	}
	return nil
}

func cloneSigningString(value *string) *string {
	if value == nil {
		return nil
	}
	return stringPtr(*value)
}

func stringPtr(value string) *string {
	return &value
}
