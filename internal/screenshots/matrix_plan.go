package screenshots

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/tidwall/jsonc"
)

// LoadMatrixPlan reads a JSON or JSONC matrix plan without resolving its base plan.
func LoadMatrixPlan(path string) (*MatrixPlan, error) {
	file, err := rootfs.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMatrixPlanRead, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxMatrixPlanBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMatrixPlanRead, err)
	}
	if len(data) > maxMatrixPlanBytes {
		return nil, fmt.Errorf("%w: matrix plan exceeds the %d-byte size limit", ErrMatrixPlanRead, maxMatrixPlanBytes)
	}
	jsonData := jsonc.ToJSON(data)
	if err := validateMatrixPlanJSONFields(jsonData); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMatrixPlanParseJSON, err)
	}
	var plan MatrixPlan
	decoder := json.NewDecoder(bytes.NewReader(jsonData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMatrixPlanParseJSON, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("%w: multiple JSON values", ErrMatrixPlanParseJSON)
		}
		return nil, fmt.Errorf("%w: %w", ErrMatrixPlanParseJSON, err)
	}
	plan.sourcePath, _ = filepath.Abs(path)
	return &plan, nil
}

type matrixJSONScope string

const (
	matrixJSONScopePlan           matrixJSONScope = "plan"
	matrixJSONScopeDevice         matrixJSONScope = "device"
	matrixJSONScopeContentVariant matrixJSONScope = "content_variant"
	matrixJSONScopeExecution      matrixJSONScope = "execution"
	matrixJSONScopeOutput         matrixJSONScope = "output"
	matrixJSONScopeFrame          matrixJSONScope = "frame"
	matrixJSONScopeFrameMapping   matrixJSONScope = "frame_mapping"
	matrixJSONScopeFrameMapValue  matrixJSONScope = "frame_mapping_value"
	matrixJSONScopeGeneric        matrixJSONScope = "generic"
)

var matrixJSONFieldScopes = map[matrixJSONScope]map[string]matrixJSONScope{
	matrixJSONScopePlan: {
		"version":          matrixJSONScopeGeneric,
		"base_plan":        matrixJSONScopeGeneric,
		"devices":          matrixJSONScopeDevice,
		"locales":          matrixJSONScopeGeneric,
		"appearances":      matrixJSONScopeGeneric,
		"content_variants": matrixJSONScopeContentVariant,
		"execution":        matrixJSONScopeExecution,
		"output":           matrixJSONScopeOutput,
	},
	matrixJSONScopeDevice: {
		"id":   matrixJSONScopeGeneric,
		"udid": matrixJSONScopeGeneric,
	},
	matrixJSONScopeContentVariant: {
		"id":               matrixJSONScopeGeneric,
		"launch_arguments": matrixJSONScopeGeneric,
	},
	matrixJSONScopeExecution: {
		"max_concurrency":  matrixJSONScopeGeneric,
		"max_attempts":     matrixJSONScopeGeneric,
		"retry_backoff_ms": matrixJSONScopeGeneric,
		"retry_backoff":    matrixJSONScopeGeneric,
	},
	matrixJSONScopeOutput: {
		"raw_dir":    matrixJSONScopeGeneric,
		"framed_dir": matrixJSONScopeGeneric,
		"review_dir": matrixJSONScopeGeneric,
		"frame":      matrixJSONScopeFrame,
	},
	matrixJSONScopeFrame: {
		"enabled":                 matrixJSONScopeGeneric,
		"device_by_matrix_device": matrixJSONScopeFrameMapping,
	},
	matrixJSONScopeFrameMapping: {},
}

// validateMatrixPlanJSONFields rejects duplicate keys and accepts only the
// exact snake_case spelling of matrix-plan fields. encoding/json otherwise
// accepts case-insensitive field matches and silently keeps the last duplicate
// value, which makes operator typos and ambiguous plans unsafe to review.
func validateMatrixPlanJSONFields(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := walkMatrixJSONValue(decoder, matrixJSONScopePlan); err != nil {
		return err
	}
	return nil
}

func walkMatrixJSONValue(decoder *json.Decoder, scope matrixJSONScope) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token == nil {
		if scope == matrixJSONScopeGeneric {
			return fmt.Errorf("matrix plan %s must not be null", scope)
		}
		switch scope {
		case matrixJSONScopeExecution, matrixJSONScopeOutput, matrixJSONScopeFrame, matrixJSONScopeFrameMapping:
			return fmt.Errorf("matrix plan %s must be an object", scope)
		case matrixJSONScopeFrameMapValue:
			return errors.New("matrix plan frame mapping values must not be null")
		}
		return nil
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return nil
	}
	switch delim {
	case '{':
		return walkMatrixJSONObject(decoder, scope)
	case '[':
		for decoder.More() {
			if err := walkMatrixJSONValue(decoder, scope); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return fmt.Errorf("matrix plan JSON array is malformed")
		}
		return nil
	default:
		return fmt.Errorf("matrix plan JSON value is malformed")
	}
}

func walkMatrixJSONObject(decoder *json.Decoder, scope matrixJSONScope) error {
	allowed := matrixJSONFieldScopes[scope]
	seen := make(map[string]string)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return fmt.Errorf("matrix plan JSON object key is malformed")
		}
		keyFolded := strings.ToLower(key)
		if previous, exists := seen[keyFolded]; exists {
			return fmt.Errorf("matrix plan contains duplicate fields %q and %q", previous, key)
		}
		childScope, exact := allowed[key]
		if scope == matrixJSONScopeFrameMapping {
			childScope, exact = matrixJSONScopeFrameMapValue, true
		}
		if len(allowed) > 0 && !exact {
			for expected := range allowed {
				if strings.EqualFold(expected, key) {
					return fmt.Errorf("matrix plan field %q must use exact spelling %q", key, expected)
				}
			}
			return fmt.Errorf("matrix plan contains unknown field %q", key)
		}
		seen[keyFolded] = key
		if err := walkMatrixJSONValue(decoder, childScope); err != nil {
			return err
		}
	}
	end, err := decoder.Token()
	if err != nil {
		return err
	}
	if end != json.Delim('}') {
		return fmt.Errorf("matrix plan JSON object is malformed")
	}
	return nil
}

// loadMatrixBasePlan loads a base screenshot plan from the matrix plan's
// directory. Matrix plans intentionally do not permit absolute or escaping
// references: the directory is the operator-selected trust boundary for all
// matrix inputs. Rooted reads reject symlinks, non-regular files, and files
// larger than the bounded plan limit.
func loadMatrixBasePlan(matrixPath string, matrixPlan *MatrixPlan) (*Plan, error) {
	if matrixPlan == nil {
		return nil, fmt.Errorf("%w: matrix plan is required", ErrPlanRead)
	}
	baseReference := matrixPlan.BasePlan
	if strings.TrimSpace(baseReference) == "" {
		return nil, fmt.Errorf("%w: base_plan is required", ErrPlanRead)
	}
	if filepath.IsAbs(baseReference) || filepath.VolumeName(baseReference) != "" {
		return nil, fmt.Errorf("%w: base_plan must be relative to the matrix plan", ErrPlanRead)
	}
	baseRelative := filepath.Clean(baseReference)
	if baseRelative == ".." || strings.HasPrefix(baseRelative, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("%w: base_plan must stay below the matrix plan directory", ErrPlanRead)
	}
	baseRoot, err := rootfs.New(matrixPlanSourceDir(matrixPath, matrixPlan.sourcePath))
	if err != nil {
		return nil, fmt.Errorf("%w: open matrix plan directory: %w", ErrPlanRead, err)
	}
	defer func() { _ = baseRoot.Close() }()
	data, err := baseRoot.ReadFileLimited(baseRelative, maxMatrixPlanBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPlanRead, err)
	}
	var plan Plan
	data = jsonc.ToJSON(data)
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPlanParseJSON, err)
	}
	// Keep the existing base-plan compatibility contract: omitted version
	// fields are treated as v1. The matrix envelope itself remains strict and
	// must explicitly declare version 1.
	if plan.Version == 0 {
		plan.Version = 1
	}
	if err := validatePlan(&plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

func expandMatrix(plan *MatrixPlan, base *Plan, outputBaseDir string) ([]MatrixCell, error) {
	if err := validateMatrixPlan(plan, base, outputBaseDir); err != nil {
		return nil, err
	}
	rawDir := plan.Output.RawDir
	if strings.TrimSpace(rawDir) == "" {
		rawDir = defaultMatrixRawDir
	}
	framedDir := plan.Output.FramedDir
	if strings.TrimSpace(framedDir) == "" {
		framedDir = defaultMatrixFramedDir
	}
	cells := make([]MatrixCell, 0, len(plan.Devices)*len(plan.Locales)*len(plan.Appearances)*len(plan.ContentVariants))
	for _, device := range plan.Devices {
		for _, locale := range plan.Locales {
			for _, appearance := range plan.Appearances {
				for _, variant := range plan.ContentVariants {
					id := strings.Join([]string{device.ID, locale, appearance, variant.ID}, "|")
					launchArguments, err := BuildLocaleLaunchArguments(locale)
					if err != nil {
						return nil, err
					}
					launchArguments = append(launchArguments, variant.LaunchArguments...)
					cell := MatrixCell{
						ID:              id,
						Device:          device.ID,
						UDID:            device.UDID,
						Locale:          locale,
						Appearance:      appearance,
						Content:         variant.ID,
						LaunchArguments: append([]string(nil), launchArguments...),
						RawDir:          filepath.Join(rawDir, locale, device.ID, appearance, variant.ID),
						FramedDir:       filepath.Join(framedDir, locale, device.ID, appearance, variant.ID),
					}
					for _, step := range base.Steps {
						if step.Action != ActionScreenshot {
							continue
						}
						name := strings.TrimSpace(stringValue(step.Name))
						cell.RawPaths = append(cell.RawPaths, filepath.Join(cell.RawDir, name+".png"))
						cell.FramedPaths = append(cell.FramedPaths, filepath.Join(cell.FramedDir, name+".png"))
					}
					cells = append(cells, cell)
				}
			}
		}
	}
	return cells, nil
}

// BuildLocaleLaunchArguments returns arguments accepted by simctl launch.
func BuildLocaleLaunchArguments(locale string) ([]string, error) {
	normalized, err := normalizeMatrixLocale(locale)
	if err != nil {
		return nil, err
	}
	localeParts := strings.Split(normalized, "-")
	language := localeParts[0]
	if len(localeParts) > 1 && len(localeParts[1]) == 4 && isASCIIAlpha(localeParts[1]) {
		language += "-" + localeParts[1]
	}
	return []string{"-AppleLanguages", "(" + language + ")", "-AppleLocale", strings.ReplaceAll(normalized, "-", "_")}, nil
}

func resolveMatrixArtifactPath(baseDir, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	abs, err := filepath.Abs(filepath.Join(baseDir, path))
	if err != nil {
		return filepath.Clean(filepath.Join(baseDir, path))
	}
	return abs
}

func matrixPlanSourceDir(matrixPath, sourcePath string) string {
	path := matrixPath
	if strings.TrimSpace(path) == "" {
		path = sourcePath
	}
	if path == "" {
		return "."
	}
	return filepath.Dir(path)
}

func isSafeMatrixPathComponent(value string) bool {
	return value != "." && value != ".." && matrixPathComponentPattern.MatchString(strings.TrimSpace(value))
}

func validateLiteralLaunchArguments(arguments []string) error {
	for _, argument := range arguments {
		trimmed := strings.TrimSpace(argument)
		if trimmed == "-AppleLocale" || trimmed == "-AppleLanguages" || strings.HasPrefix(trimmed, "-AppleLocale=") || strings.HasPrefix(trimmed, "-AppleLanguages=") {
			return errors.New("launch arguments must not override AppleLocale or AppleLanguages")
		}
	}
	return nil
}

func normalizeMatrixLocale(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "_", "-"))
	if value == "" {
		return "", errors.New("locale is required")
	}
	parts := strings.Split(value, "-")
	if len(parts) == 0 || len(parts[0]) < 2 || len(parts[0]) > 3 || !isASCIIAlpha(parts[0]) {
		return "", fmt.Errorf("locale %q must start with a language code such as en or en-US", value)
	}
	parts[0] = strings.ToLower(parts[0])
	next := 1
	if next < len(parts) && len(parts[next]) == 4 && isASCIIAlpha(parts[next]) {
		part := strings.ToLower(parts[next])
		parts[next] = strings.ToUpper(part[:1]) + part[1:]
		next++
	}
	if next < len(parts) && (len(parts[next]) == 2 && isASCIIAlpha(parts[next]) || len(parts[next]) == 3 && isASCIIDigit(parts[next])) {
		parts[next] = strings.ToUpper(parts[next])
		next++
	}
	if next != len(parts) {
		return "", fmt.Errorf("locale %q must contain at most one script followed by one region", value)
	}
	return strings.Join(parts, "-"), nil
}

func isASCIIAlpha(value string) bool {
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

func isASCIIDigit(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validateMatrixFrameMapping(matrixDevice, frame string) error {
	// Device-family compatibility is checked after simulator inventory is read;
	// a matrix ID is only a logical label and must not determine the family.
	if _, err := ParseFrameDevice(frame); err != nil {
		return fmt.Errorf("device %q: %w", matrixDevice, err)
	}
	return nil
}

func matrixDeviceFamily(value string) string {
	value = strings.ToLower(value)
	switch {
	case strings.Contains(value, "ipad"), strings.Contains(value, "tablet"):
		return "ipad"
	case strings.Contains(value, "mac"):
		return "mac"
	case strings.Contains(value, "iphone"), strings.Contains(value, "phone"):
		return "iphone"
	default:
		return "unknown"
	}
}
