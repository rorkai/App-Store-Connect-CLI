package xcode

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

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
	defer beginSigningPlanRootScope()()
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
	if err := validateSigningArtifactPaths(planPath, receiptPath, project.projectFilePath, settingsPath); err != nil {
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
			authorizedProtectedConfigPaths = append(authorizedProtectedConfigPaths, protectedPath)
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
	addFile(project.projectFilePath, project.projectSource)
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
	if err := root.AllowingInternalSymlinks().CheckContained(project.projectFilePath); err != nil {
		return fmt.Errorf("refusing to use Xcode project file %s: %w", project.projectFilePath, err)
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
