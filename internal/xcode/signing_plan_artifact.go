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
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

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

// validateSigningArtifactAliasesWithAuthorizedProtectedPaths rejects an
// existing plan or receipt that resolves to any source consumed while building
// the plan. Lexical path comparisons do not catch hard-link aliases, while
// rooted no-follow identity checks plus os.SameFile identify existing aliases
// without mutating the filesystem; symlink aliases are rejected by the rooted
// opener.
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

	seenInputKeys := make(map[string]struct{}, len(inputPaths))
	seenInputsByFold := make(map[string][]string, len(inputPaths))
	for _, inputPath := range inputPaths {
		if strings.TrimSpace(inputPath) == "" {
			continue
		}
		inputPath = normalize(inputPath)
		inputKey := signingLexicalPathKey(inputPath)
		if _, duplicate := seenInputKeys[inputKey]; duplicate {
			continue
		}
		foldKey := signingPathCaseFoldKey(inputPath)
		duplicate := false
		for _, seenInput := range seenInputsByFold[foldKey] {
			if signingArtifactLexicalPathEqual(seenInput, inputPath) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		seenInputKeys[inputKey] = struct{}{}
		seenInputsByFold[foldKey] = append(seenInputsByFold[foldKey], inputPath)
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
