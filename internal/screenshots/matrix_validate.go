package screenshots

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func validateMatrixPlan(plan *MatrixPlan, base *Plan, outputBaseDir string) error {
	if plan == nil {
		return errors.New("matrix plan is required")
	}
	if plan.Version != 1 {
		return fmt.Errorf("unsupported matrix plan version %d (expected 1)", plan.Version)
	}
	if base == nil {
		return errors.New("base screenshot plan is required")
	}
	if err := validatePlan(base); err != nil {
		return fmt.Errorf("base plan: %w", err)
	}
	if err := validateMatrixBaseScreenshots(base); err != nil {
		return err
	}
	if len(plan.Devices) == 0 || len(plan.Locales) == 0 || len(plan.Appearances) == 0 || len(plan.ContentVariants) == 0 {
		return errors.New("devices, locales, appearances, and content_variants must each contain at least one item")
	}

	seenIDs := make(map[string]struct{}, len(plan.Devices))
	seenUDIDs := make(map[string]struct{}, len(plan.Devices))
	for i := range plan.Devices {
		device := &plan.Devices[i]
		device.ID = strings.TrimSpace(device.ID)
		device.UDID = strings.TrimSpace(device.UDID)
		if !isSafeMatrixPathComponent(device.ID) {
			return fmt.Errorf("device id %q must be a unique safe path component", device.ID)
		}
		idKey := strings.ToLower(device.ID)
		if _, exists := seenIDs[idKey]; exists {
			return fmt.Errorf("device id %q must be unique", device.ID)
		}
		seenIDs[idKey] = struct{}{}
		if device.UDID == "" {
			return fmt.Errorf("device %q udid is required", device.ID)
		}
		udidKey := normalizeMatrixUDID(device.UDID)
		if _, exists := seenUDIDs[udidKey]; exists {
			return fmt.Errorf("device udid values must be unique")
		}
		seenUDIDs[udidKey] = struct{}{}
	}

	seenLocales := make(map[string]struct{}, len(plan.Locales))
	for i, locale := range plan.Locales {
		normalized, err := normalizeMatrixLocale(locale)
		if err != nil {
			return fmt.Errorf("locales[%d]: %w", i, err)
		}
		if _, exists := seenLocales[normalized]; exists {
			return fmt.Errorf("locale %q must be unique after normalization", normalized)
		}
		seenLocales[normalized] = struct{}{}
		plan.Locales[i] = normalized
	}

	seenAppearances := make(map[string]struct{}, len(plan.Appearances))
	for i, appearance := range plan.Appearances {
		normalized := strings.ToLower(strings.TrimSpace(appearance))
		if normalized != "light" && normalized != "dark" {
			return fmt.Errorf("appearances[%d] must be light or dark", i)
		}
		if _, exists := seenAppearances[normalized]; exists {
			return fmt.Errorf("appearance %q must be unique", normalized)
		}
		seenAppearances[normalized] = struct{}{}
		plan.Appearances[i] = normalized
	}

	seenContent := make(map[string]struct{}, len(plan.ContentVariants))
	for i := range plan.ContentVariants {
		variant := &plan.ContentVariants[i]
		variant.ID = strings.TrimSpace(variant.ID)
		if !isSafeMatrixPathComponent(variant.ID) {
			return fmt.Errorf("content variant id %q must be a unique safe path component", variant.ID)
		}
		contentKey := strings.ToLower(variant.ID)
		if _, exists := seenContent[contentKey]; exists {
			return fmt.Errorf("content variant id %q must be unique", variant.ID)
		}
		seenContent[contentKey] = struct{}{}
		if err := validateLiteralLaunchArguments(variant.LaunchArguments); err != nil {
			return fmt.Errorf("content variant %q: %w", variant.ID, err)
		}
	}

	cellCount := len(plan.Devices) * len(plan.Locales) * len(plan.Appearances) * len(plan.ContentVariants)
	if cellCount > maxMatrixCells {
		return fmt.Errorf("matrix expands to %d cells; maximum is %d", cellCount, maxMatrixCells)
	}
	// An omitted limit defaults; an explicitly configured value must fall inside
	// the documented range, so a stated zero is rejected rather than silently
	// replaced by the default.
	if plan.Execution.MaxConcurrency < 0 || plan.Execution.MaxConcurrency > maxMatrixConcurrency ||
		(plan.Execution.maxConcurrencySet && plan.Execution.MaxConcurrency < 1) {
		return fmt.Errorf("execution.max_concurrency must be between 1 and %d when set", maxMatrixConcurrency)
	}
	if plan.Execution.MaxAttempts < 0 || plan.Execution.MaxAttempts > maxMatrixAttempts ||
		(plan.Execution.maxAttemptsSet && plan.Execution.MaxAttempts < 1) {
		return fmt.Errorf("execution.max_attempts must be between 1 and %d when set", maxMatrixAttempts)
	}
	if plan.Execution.RetryBackoffMS < 0 {
		return errors.New("execution.retry_backoff_ms must be >= 0")
	}
	if int64(plan.Execution.RetryBackoffMS) > maxMatrixRetryBackoffMS {
		return errors.New("execution.retry_backoff_ms exceeds maximum duration")
	}
	retryBackoffText := strings.TrimSpace(plan.Execution.RetryBackoff)
	if retryBackoffText != "" {
		parsed, err := time.ParseDuration(retryBackoffText)
		if err != nil || parsed < 0 {
			return errors.New("execution.retry_backoff must be a non-negative duration")
		}
	}
	// Zero milliseconds is a valid explicit no-delay value, so a nonzero test
	// cannot tell "omitted" from "stated as 0". Presence decides when the plan
	// came from a document; the value comparison still covers plans built in Go,
	// which carry no presence information.
	if (plan.Execution.retryBackoffMSSet && plan.Execution.retryBackoffSet) ||
		(retryBackoffText != "" && plan.Execution.RetryBackoffMS != 0) {
		return errors.New("set only one of execution.retry_backoff or execution.retry_backoff_ms")
	}
	if plan.Execution.retryBackoffSet && retryBackoffText == "" {
		return errors.New("execution.retry_backoff must not be empty")
	}
	if err := validateMatrixOutputPaths(plan.Output, outputBaseDir); err != nil {
		return err
	}
	if err := validateMatrixReviewDoesNotOverwritePlans(plan, outputBaseDir); err != nil {
		return err
	}
	frameMappings := make(map[string]string, len(plan.Output.Frame.DeviceByMatrixDevice))
	for matrixDevice := range plan.Output.Frame.DeviceByMatrixDevice {
		key := normalizeMatrixDeviceID(matrixDevice)
		if _, declared := seenIDs[key]; !declared {
			return fmt.Errorf("framing mapping references undeclared device %q", matrixDevice)
		}
		if _, duplicate := frameMappings[key]; duplicate {
			return fmt.Errorf("framing mapping device %q must be unique", matrixDevice)
		}
		frame := plan.Output.Frame.DeviceByMatrixDevice[matrixDevice]
		if err := validateMatrixFrameMapping(matrixDevice, frame); err != nil {
			return err
		}
		frameMappings[key] = frame
	}
	if !plan.Output.Frame.Enabled {
		return nil
	}
	for _, device := range plan.Devices {
		frame, ok := frameMappings[normalizeMatrixDeviceID(device.ID)]
		if !ok || strings.TrimSpace(frame) == "" {
			return fmt.Errorf("framing requires a frame mapping for device %q", device.ID)
		}
	}
	return nil
}

func validateMatrixOutputPaths(output MatrixOutput, baseDir string) error {
	rawDir := output.RawDir
	if strings.TrimSpace(rawDir) == "" {
		rawDir = defaultMatrixRawDir
	}
	framedDir := output.FramedDir
	if strings.TrimSpace(framedDir) == "" {
		framedDir = defaultMatrixFramedDir
	}
	reviewDir := output.ReviewDir
	if strings.TrimSpace(reviewDir) == "" {
		reviewDir = defaultMatrixReviewDir
	}
	paths := []struct {
		name string
		path string
	}{
		{name: "raw_dir", path: rawDir}, {name: "framed_dir", path: framedDir}, {name: "review_dir", path: reviewDir},
	}
	for i := range paths {
		for j := i + 1; j < len(paths); j++ {
			left := resolveMatrixValidationPath(baseDir, paths[i].path)
			right := resolveMatrixValidationPath(baseDir, paths[j].path)
			if sameMatrixDirectory(left, right) {
				return fmt.Errorf("output.%s and output.%s must be different directories", paths[i].name, paths[j].name)
			}
		}
	}
	return nil
}

// sameMatrixDirectory reports whether two output paths resolve to the same
// directory, including when one path reaches the other through a symlinked
// ancestor or a platform alias such as /tmp versus /private/tmp. Output roots
// do not need to exist yet, so the comparison resolves the nearest existing
// ancestor and appends the not-yet-created suffix without creating anything.
func sameMatrixDirectory(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if matrixLexicalPathsEqual(left, right) {
		return true
	}
	if leftInfo, leftErr := os.Stat(left); leftErr == nil {
		if rightInfo, rightErr := os.Stat(right); rightErr == nil && os.SameFile(leftInfo, rightInfo) {
			return true
		}
	}
	leftPhysical, leftOK := resolveMatrixPhysicalPath(left)
	rightPhysical, rightOK := resolveMatrixPhysicalPath(right)
	return leftOK && rightOK && matrixLexicalPathsEqual(leftPhysical, rightPhysical)
}

// matrixLexicalPathsEqual reports whether two cleaned paths name the same
// location using the target filesystem's case semantics. EqualFold is only
// applied when an existing ancestor aliases a case variant.
func matrixLexicalPathsEqual(left, right string) bool {
	if left == right {
		return true
	}
	if !strings.EqualFold(left, right) {
		return false
	}
	return matrixFilesystemCaseInsensitive(left) || matrixFilesystemCaseInsensitive(right)
}

// resolveMatrixPhysicalPath resolves the existing prefix of a possibly
// missing path. This is intentionally a read-only identity check: it does not
// create output directories or follow a final path during publication. The
// rooted writers still enforce their no-follow contract when they open roots
// and files.
func resolveMatrixPhysicalPath(path string) (string, bool) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	absPath = filepath.Clean(absPath)
	missing := make([]string, 0, 4)
	current := absPath
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", false
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), true
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", false
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

// validateMatrixReviewDoesNotOverwritePlans refuses a review directory whose
// generated files would atomically replace one of the plan documents that
// produced them.
//
// Comparing only the three output directories is not enough: GenerateMatrixReview
// always publishes fixed filenames into review_dir, so a matrix plan at
// config/manifest.json with review_dir "." is structurally valid yet destroys its
// own input on the first run, and every later run then fails to parse it. The
// comparison is case-folded to match the case-insensitive filesystems this runs
// on, consistent with validateMatrixOutputPaths.
func validateMatrixReviewDoesNotOverwritePlans(plan *MatrixPlan, baseDir string) error {
	if strings.TrimSpace(plan.sourcePath) == "" {
		// A programmatically constructed plan has no on-disk source to protect.
		return nil
	}
	planPath := plan.sourcePath
	planDir := filepath.Dir(planPath)
	if strings.TrimSpace(baseDir) == "" {
		baseDir = planDir
	}
	reviewDir := plan.Output.ReviewDir
	if strings.TrimSpace(reviewDir) == "" {
		reviewDir = defaultMatrixReviewDir
	}
	resolvedReviewDir := filepath.Clean(resolveMatrixValidationPath(baseDir, reviewDir))

	type matrixPlanInput struct {
		label string
		path  string
	}
	inputs := []matrixPlanInput{{label: "matrix plan", path: filepath.Clean(planPath)}}
	if reference := plan.BasePlan; strings.TrimSpace(reference) != "" && !filepath.IsAbs(strings.TrimSpace(reference)) {
		inputs = append(inputs, matrixPlanInput{
			label: "base plan",
			path:  filepath.Clean(resolveMatrixArtifactPath(planDir, reference)),
		})
	}
	for _, generated := range matrixReviewGeneratedFiles {
		generatedPath := filepath.Clean(filepath.Join(resolvedReviewDir, generated))
		for _, input := range inputs {
			if matrixLexicalPathsEqual(generatedPath, input.path) || sameMatrixFile(generatedPath, input.path) {
				return fmt.Errorf(
					"output.review_dir would overwrite the %s at %q with the generated %s; use a different review_dir or plan filename",
					input.label, input.path, generated,
				)
			}
		}
	}
	return nil
}

// sameMatrixFile reports whether two paths name the same existing file.
//
// Cleaned strings cannot see through a symlinked ancestor or a platform alias
// such as /tmp versus /private/tmp, and openMatrixOutputRoot resolves the review
// directory's parent physically, so filesystem identity is what actually decides
// whether publishing would land on a plan input. If the generated path does not
// exist yet it cannot already be the plan, so a failed stat is not a collision.
//
// Lstat rather than Stat is deliberate: a symlink sitting at the generated path
// is refused by the rooted, no-follow writer instead of being followed to the
// plan, so comparing the link itself is the accurate test.
func sameMatrixFile(left, right string) bool {
	leftInfo, err := os.Lstat(left)
	if err != nil {
		return false
	}
	rightInfo, err := os.Lstat(right)
	if err != nil {
		return false
	}
	return os.SameFile(leftInfo, rightInfo)
}

// validateMatrixArtifactPathsDoNotOverwritePlans checks every expanded raw
// and framed artifact path before any execution or output-root creation. Plan
// and base-plan files are inputs to the run, so allowing a generated artifact
// to alias either one would turn the first run into a destructive rewrite.
func validateMatrixArtifactPathsDoNotOverwritePlans(plan *MatrixPlan, matrixPath, baseDir string, cells []MatrixCell) error {
	if plan == nil {
		return errors.New("matrix plan is required")
	}
	planPath := plan.sourcePath
	if strings.TrimSpace(planPath) == "" {
		planPath = matrixPath
	}
	planDir := strings.TrimSpace(baseDir)
	if planPath != "" {
		if absolute, err := filepath.Abs(planPath); err == nil {
			planPath = filepath.Clean(absolute)
		}
		planDir = filepath.Dir(planPath)
	}
	if planDir == "" {
		planDir = "."
	}

	type matrixPlanInput struct {
		label string
		path  string
	}
	inputs := make([]matrixPlanInput, 0, 2)
	if planPath != "" {
		inputs = append(inputs, matrixPlanInput{label: "matrix plan", path: filepath.Clean(planPath)})
	}
	if reference := plan.BasePlan; strings.TrimSpace(reference) != "" && !filepath.IsAbs(strings.TrimSpace(reference)) {
		inputs = append(inputs, matrixPlanInput{
			label: "base plan",
			path:  filepath.Clean(resolveMatrixArtifactPath(planDir, reference)),
		})
	}
	if len(inputs) == 0 {
		return nil
	}

	for _, cell := range cells {
		artifacts := []struct {
			kind  string
			paths []string
		}{
			{kind: "raw", paths: cell.RawPaths},
		}
		if plan.Output.Frame.Enabled {
			artifacts = append(artifacts, struct {
				kind  string
				paths []string
			}{kind: "framed", paths: cell.FramedPaths})
		}
		for _, artifact := range artifacts {
			for _, path := range artifact.paths {
				resolvedPath := filepath.Clean(resolveMatrixArtifactPath(baseDir, path))
				for _, input := range inputs {
					if sameMatrixPath(resolvedPath, input.path) {
						return fmt.Errorf(
							"output %s artifact %q would overwrite the %s at %q; choose distinct output paths",
							artifact.kind, resolvedPath, input.label, input.path,
						)
					}
				}
			}
		}
	}
	return nil
}

// sameMatrixPath reports whether two paths identify the same existing input,
// or resolve to the same physical path when the destination does not exist
// yet. The latter matters for aliases such as /tmp versus /private/tmp and
// symlinked ancestors: a future artifact must not be allowed to replace a
// plan simply because its final directory entry has not been created.
func sameMatrixPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if matrixLexicalPathsEqual(left, right) || sameMatrixFile(left, right) {
		return true
	}
	leftPhysical, leftOK := resolveMatrixPhysicalPath(left)
	rightPhysical, rightOK := resolveMatrixPhysicalPath(right)
	return leftOK && rightOK && matrixLexicalPathsEqual(leftPhysical, rightPhysical)
}

func resolveMatrixValidationPath(baseDir, path string) string {
	if baseDir != "" {
		return resolveMatrixArtifactPath(baseDir, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func validateMatrixBaseScreenshots(base *Plan) error {
	seen := make(map[string]struct{})
	for i, step := range base.Steps {
		if step.Action != ActionScreenshot {
			continue
		}
		name := strings.TrimSpace(stringValue(step.Name))
		if !isSafeMatrixPathComponent(name) {
			return fmt.Errorf("base plan screenshot at steps[%d] has unsafe name %q", i+1, name)
		}
		nameKey := strings.ToLower(name)
		if _, exists := seen[nameKey]; exists {
			return fmt.Errorf("base plan screenshot name %q must be unique", name)
		}
		seen[nameKey] = struct{}{}
	}
	if len(seen) == 0 {
		return errors.New("base plan must contain at least one screenshot step")
	}
	if err := validateLiteralLaunchArguments(base.App.LaunchArguments); err != nil {
		return fmt.Errorf("base plan app.launch_arguments: %w", err)
	}
	return nil
}
