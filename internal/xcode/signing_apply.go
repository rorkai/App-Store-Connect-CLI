package xcode

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

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
		if operation.Source != project.projectSource {
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
