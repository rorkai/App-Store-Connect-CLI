package screenshots

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
	"github.com/tidwall/jsonc"
)

const (
	maxMatrixPlanBytes       = 1 << 20
	maxMatrixAppearanceBytes = 64 << 10
	maxMatrixReviewBytes     = 8 << 20
	maxMatrixArtifactBytes   = 1 << 30
	maxMatrixInventoryBytes  = 4 << 20
	maxMatrixCells           = 256
	maxMatrixConcurrency     = 8
	maxMatrixAttempts        = 3
	// Keep millisecond retry values within time.Duration's nanosecond range
	// before converting them to a duration for scheduling.
	maxMatrixRetryBackoffMS  = (1<<63 - 1) / int64(time.Millisecond)
	matrixSubprocessTimeout  = 30 * time.Second
	defaultMatrixConcurrency = 1
	defaultMatrixAttempts    = 1
	defaultMatrixRawDir      = "./screenshots/matrix/raw"
	defaultMatrixFramedDir   = "./screenshots/matrix/framed"
	defaultMatrixReviewDir   = "./screenshots/matrix/review"
)

var matrixPathComponentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var (
	// ErrMatrixPlanRead indicates that a matrix plan could not be read.
	ErrMatrixPlanRead = errors.New("read matrix plan")
	// ErrMatrixPlanParseJSON indicates that a matrix plan is not valid JSON/JSONC.
	ErrMatrixPlanParseJSON = errors.New("parse matrix plan JSON")
	// ErrMatrixInventoryTimeout indicates that the bounded simulator inventory
	// command reached its own deadline without caller cancellation.
	ErrMatrixInventoryTimeout = errors.New("simulator inventory timed out")
)

// MatrixValidationError marks failures that are deterministic input errors and
// must be reported with CLI usage semantics before any run side effect.
type MatrixValidationError struct {
	Err error
}

func (e *MatrixValidationError) Error() string {
	if e == nil || e.Err == nil {
		return "invalid matrix plan"
	}
	return e.Err.Error()
}

func (e *MatrixValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newMatrixValidationError(err error) error {
	if err == nil {
		return nil
	}
	return &MatrixValidationError{Err: err}
}

// MatrixDevice identifies an already-existing, booted simulator.
type MatrixDevice struct {
	ID   string `json:"id"`
	UDID string `json:"udid"`
}

// MatrixContentVariant supplies literal launch arguments for one content fixture.
type MatrixContentVariant struct {
	ID              string   `json:"id"`
	LaunchArguments []string `json:"launch_arguments,omitempty"`
}

// MatrixExecution controls matrix scheduling and retry behavior.
type MatrixExecution struct {
	MaxConcurrency int    `json:"max_concurrency,omitempty"`
	MaxAttempts    int    `json:"max_attempts,omitempty"`
	RetryBackoffMS int    `json:"retry_backoff_ms,omitempty"`
	RetryBackoff   string `json:"retry_backoff,omitempty"`

	// maxConcurrencySet and maxAttemptsSet record whether the operator stated
	// the bounded limit explicitly, so an omitted field can default while an
	// explicit zero is rejected as out of the documented 1-N range.
	maxConcurrencySet bool
	maxAttemptsSet    bool

	// retryBackoffMSSet and retryBackoffSet record which retry-backoff encoding
	// was stated. Zero milliseconds and an empty duration are both meaningful
	// values, so only presence can tell that both encodings were supplied.
	retryBackoffMSSet bool
	retryBackoffSet   bool
}

// UnmarshalJSON decodes execution settings while recording which bounded limits
// were stated explicitly. encoding/json cannot otherwise distinguish
// "max_concurrency": 0 from an omitted field, so a mistaken zero would silently
// run with the default instead of being reported. Unknown-field strictness is
// preserved here because a custom unmarshaler bypasses the outer decoder's
// DisallowUnknownFields.
func (e *MatrixExecution) UnmarshalJSON(data []byte) error {
	type executionFields MatrixExecution
	var decoded executionFields
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	*e = MatrixExecution(decoded)
	var present map[string]json.RawMessage
	if err := json.Unmarshal(data, &present); err != nil {
		return err
	}
	_, e.maxConcurrencySet = present["max_concurrency"]
	_, e.maxAttemptsSet = present["max_attempts"]
	_, e.retryBackoffMSSet = present["retry_backoff_ms"]
	_, e.retryBackoffSet = present["retry_backoff"]
	return nil
}

// MatrixFrame configures optional local framing for matrix artifacts.
type MatrixFrame struct {
	Enabled              bool              `json:"enabled"`
	DeviceByMatrixDevice map[string]string `json:"device_by_matrix_device,omitempty"`
}

// MatrixOutput configures the three local artifact directories.
type MatrixOutput struct {
	RawDir    string      `json:"raw_dir,omitempty"`
	FramedDir string      `json:"framed_dir,omitempty"`
	ReviewDir string      `json:"review_dir,omitempty"`
	Frame     MatrixFrame `json:"frame,omitempty"`
}

// MatrixPlan describes the Cartesian product to execute over a base screenshot plan.
type MatrixPlan struct {
	Version         int                    `json:"version"`
	BasePlan        string                 `json:"base_plan"`
	Devices         []MatrixDevice         `json:"devices"`
	Locales         []string               `json:"locales"`
	Appearances     []string               `json:"appearances"`
	ContentVariants []MatrixContentVariant `json:"content_variants"`
	Execution       MatrixExecution        `json:"execution,omitempty"`
	Output          MatrixOutput           `json:"output,omitempty"`

	sourcePath string
}

// MatrixCell is an expanded matrix invocation. UDID and launch arguments are
// intentionally internal and are never serialized in result or review artifacts.
type MatrixCell struct {
	ID              string
	Device          string
	UDID            string
	Locale          string
	Appearance      string
	Content         string
	LaunchArguments []string
	RawDir          string
	FramedDir       string
	RawPaths        []string
	FramedPaths     []string
}

const (
	MatrixCellSuccess       = "success"
	MatrixCellFailed        = "failed"
	MatrixCellCanceled      = "canceled"
	MatrixCellCleanupFailed = "cleanup_failed"
)

// MatrixCellResult is the privacy-safe result for one cell.
type MatrixCellResult struct {
	ID           string                   `json:"id"`
	Device       string                   `json:"device"`
	Locale       string                   `json:"locale"`
	Appearance   string                   `json:"appearance"`
	Content      string                   `json:"contentVariant"`
	Status       string                   `json:"status"`
	Attempts     int                      `json:"attempts"`
	DurationMS   int64                    `json:"durationMs"`
	RawPaths     []string                 `json:"rawPaths,omitempty"`
	FramedPaths  []string                 `json:"framedPaths,omitempty"`
	Screenshots  []MatrixScreenshotResult `json:"screenshots,omitempty"`
	Steps        []RunStepResult          `json:"steps,omitempty"`
	FailureStage string                   `json:"failureStage,omitempty"`
	FailureCode  string                   `json:"failureCode,omitempty"`
	Error        *MatrixCellError         `json:"error,omitempty"`

	framedArtifacts map[string]matrixArtifactInfo
	rawArtifacts    map[string]matrixArtifactInfo
}

// MatrixCellError is a sanitized, stable failure contract. It intentionally
// has no raw subprocess output, simulator identifier, or launch arguments.
type MatrixCellError struct {
	Stage   string `json:"stage"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// MatrixScreenshotResult describes one screenshot step in a cell review.
type MatrixScreenshotResult struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	RawPath    string `json:"rawPath,omitempty"`
	FramedPath string `json:"framedPath,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
}

// MatrixResult is printed after a matrix run and is also the source for review artifacts.
type MatrixResult struct {
	PlanPath      string              `json:"planPath"`
	BundleID      string              `json:"bundleId,omitempty"`
	RawDir        string              `json:"rawDir"`
	FramedDir     string              `json:"framedDir"`
	ReviewDir     string              `json:"reviewDir"`
	Status        string              `json:"status"`
	TotalCells    int                 `json:"totalCells"`
	Succeeded     int                 `json:"succeeded"`
	Failed        int                 `json:"failed"`
	Canceled      int                 `json:"canceled"`
	Retried       int                 `json:"retried"`
	CleanupFailed int                 `json:"cleanupFailed,omitempty"`
	Cells         []MatrixCellResult  `json:"cells"`
	Review        *MatrixReviewResult `json:"review,omitempty"`

	// Total is retained internally for callers that build reports directly;
	// the public output uses totalCells.
	Total int `json:"-"`
}

// MatrixOptions contains command-line overrides. Zero values use plan defaults.
type MatrixOptions struct {
	MaxConcurrency    int
	MaxConcurrencySet bool
	MaxAttempts       int
	MaxAttemptsSet    bool
	RetryBackoff      time.Duration
	RetryBackoffSet   bool
}

// MatrixAppearance controls simulator appearance state around a cell.
type MatrixAppearance interface {
	Snapshot(ctx context.Context, udid string) (state string, err error)
	Set(ctx context.Context, udid, appearance string) error
	Restore(ctx context.Context, udid, state string) error
}

// MatrixDependencies makes external execution replaceable by tests without
// changing the normal command behavior.
type MatrixDependencies struct {
	RunPlan func(context.Context, *Plan) (*RunResult, error)
	// RunPlanRooted is the matrix-only provider contract. The destination root
	// is retained for the whole attempt and must be used for every output write.
	// It is additive so existing test/integration callbacks keep compiling while
	// normal matrix execution avoids handing a replaceable pathname to adapters.
	// RunPlan and RunPlanRooted are mutually exclusive; supplying both is a
	// validation error rather than an implicit precedence choice.
	RunPlanRooted func(context.Context, *Plan, rootfs.Root) (*RunResult, error)
	Frame         func(context.Context, FrameRequest) (*FrameResult, error)
	// FrameRooted is the rooted counterpart of Frame. Implementations must use
	// the supplied root for the final output publication. Frame and FrameRooted
	// are mutually exclusive for the same reason as the plan callbacks.
	FrameRooted func(context.Context, FrameRequest, rootfs.Root) (*FrameResult, error)
	Appearance  MatrixAppearance
	CheckDevice func(context.Context, MatrixDevice) error
}

// matrixOutputRoots keep operator-selected artifact paths anchored for the
// entire run. Paths below these roots are checked and written without
// following symlinks, while the absolute paths remain available to the
// existing simulator and framing adapters.
type matrixOutputRoots struct {
	raw        rootfs.Root
	rawPath    string
	framed     rootfs.Root
	framedPath string
	hasFramed  bool
}

var matrixTemporarySequence atomic.Uint64

// This seam lets focused tests model bounded per-artifact verification and is
// intentionally nil in production.
var matrixRawVerificationBeforeArtifactForTest func()

// matrixRawVerificationContextForTest observes each fresh per-artifact raw
// verification context without adding timing dependencies to tests.
var matrixRawVerificationContextForTest func(context.Context)

// matrixFramedVerificationBeforeArtifactForTest is the framed counterpart of
// the raw verification seam. It is intentionally nil in production.
var matrixFramedVerificationBeforeArtifactForTest func()

// matrixFramedVerificationContextForTest observes each fresh per-artifact
// verification context without adding timing dependencies to tests.
var matrixFramedVerificationContextForTest func(context.Context)

// matrixArtifactBeforeOpenForTest observes the last point before a
// revalidation read. It is intentionally nil in production and lets tests
// prove a replaced output root is rejected before any artifact is opened.
var matrixArtifactBeforeOpenForTest func(rootfs.Root, string)

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

// matrixOutputRootBeforeChildRootForTest is a test-only seam placed after the
// parent has anchored the selected child and before the child root is opened.
var matrixOutputRootBeforeChildRootForTest func()

// matrixPrivateAttemptRootBeforeChildRootForTest is a test-only seam placed
// after the selected attempt directory is pinned and before its parent is
// reopened. It models a directory-entry replacement in the construction
// window without changing production behavior.
var matrixPrivateAttemptRootBeforeChildRootForTest func(string)

// matrixPrivateAttemptBeforePinForTest is a test-only seam placed after the
// anchored child descriptor has been checked but before rootfs.New pins the
// selected pathname. It models a replacement in the initial pin window.
var matrixPrivateAttemptBeforePinForTest func(string)

// matrixPrivateAttemptBeforeParentLockForTest is a test-only seam placed after
// the child has been reopened/validated and immediately before the parent
// directory is locked. It models the final construction-window replacement.
var matrixPrivateAttemptBeforeParentLockForTest func(string)

// matrixPrivateAttemptBeforeChildLockForTest is a test-only seam placed just
// before the provider child is locked. The subsequent identity check must bind
// the held child to its current parent entry even if that entry was replaced
// in this last transition window.
var matrixPrivateAttemptBeforeChildLockForTest func(string)

// matrixPrivateAttemptBeforeCleanupRemoveForTest is a test-only seam placed
// after cleanup has validated an entry identity and before it quarantines the
// entry. It models a replacement in the final publication/cleanup window.
var matrixPrivateAttemptBeforeCleanupRemoveForTest func(string)

// These seams inject cleanup/close failures after the real resource handling
// has run, so tests can verify that a successful attempt never hides resource
// uncertainty without leaking temporary directories.
var (
	matrixPrivateAttemptCleanupForTest func(string) error
	matrixPrivateAttemptCloseForTest   func(string) error
)

// matrixPrivateAttemptOperationForTest is a test-only seam that can inject a
// construction failure immediately before a rooted filesystem operation. It
// deliberately does not replace the production operation or allow tests to
// select a different path.
var matrixPrivateAttemptOperationForTest func(stage, path string) error

// matrixPrivateAttemptParentCreatedForTest observes the freshly-created
// private parent so tests can replace its directory entry while an early
// construction failure is being injected.
var matrixPrivateAttemptParentCreatedForTest func(string)

// matrixPrivateAttemptOutputBeforeRootForTest runs after the writable child
// output directory exists but before its rooted handle is opened. It is kept
// narrow so tests can model a replacement in the same construction window as
// the production identity check.
var matrixPrivateAttemptOutputBeforeRootForTest func(string)

var errMatrixPrivateAttemptCleanupUncertain = errors.New("private matrix attempt cleanup uncertain")

// matrixOutputLocksAcquiredForTest is a test-only seam placed after all
// execution/review roots have been opened and their filesystem identities have
// been locked. It models a path replacement between lock acquisition and the
// first rooted operation without changing production behavior.
var matrixOutputLocksAcquiredForTest func()

// matrixOutputLockRootOpenedForTest observes compatibility-wrapper roots so
// tests can prove that a later open failure closes earlier descriptors.
var matrixOutputLockRootOpenedForTest func(*os.Root)

// matrixOutputLockReleaseErrForTest injects a release failure after the real
// output locks have been dropped, so tests can prove the printed result and
// the published review both stop reporting success when the command still
// returns an error.
var matrixOutputLockReleaseErrForTest error

// matrixBeforeReviewPublishForTest runs after cells are counted and before
// the review transaction, while output locks are still held.
var matrixBeforeReviewPublishForTest func()

func openMatrixOutputRoot(path string) (rootfs.Root, error) {
	if strings.TrimSpace(path) == "" {
		return rootfs.Root{}, errors.New("matrix output path is required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return rootfs.Root{}, err
	}
	absPath = filepath.Clean(absPath)
	parentPath := filepath.Dir(absPath)
	name := filepath.Base(absPath)
	parent, err := rootfs.New(parentPath)
	if err != nil {
		return rootfs.Root{}, err
	}
	defer func() { _ = parent.Close() }()
	if err := parent.MkdirAll(".", 0o755); err != nil {
		return rootfs.Root{}, err
	}
	if err := parent.CheckContained(name); err != nil {
		return rootfs.Root{}, err
	}
	if err := parent.MkdirAll(name, 0o755); err != nil {
		return rootfs.Root{}, err
	}
	if err := parent.CheckContained(name); err != nil {
		return rootfs.Root{}, err
	}
	anchoredChild, err := parent.OpenDir(name)
	if err != nil {
		return rootfs.Root{}, err
	}
	defer anchoredChild.Close()
	if matrixOutputRootBeforeChildRootForTest != nil {
		matrixOutputRootBeforeChildRootForTest()
	}
	root, err := rootfs.New(absPath)
	if err != nil {
		return rootfs.Root{}, err
	}
	openedChild, openErr := root.OpenRoot()
	if openErr != nil {
		_ = root.Close()
		return rootfs.Root{}, openErr
	}
	anchoredInfo, anchorErr := anchoredChild.Stat()
	openedInfo, openedErr := openedChild.Stat(".")
	closeErr := openedChild.Close()
	if anchorErr != nil || openedErr != nil || closeErr != nil {
		_ = root.Close()
		return rootfs.Root{}, errors.Join(anchorErr, openedErr, closeErr)
	}
	if !os.SameFile(anchoredInfo, openedInfo) {
		_ = root.Close()
		return rootfs.Root{}, errors.New("matrix output root changed while opening")
	}
	if err := root.MkdirAll(".", 0o755); err != nil {
		_ = root.Close()
		return rootfs.Root{}, err
	}
	return root, nil
}

func relativeMatrixOutputPath(rootPath, path string) (string, error) {
	rootPath = filepath.Clean(rootPath)
	path = filepath.Clean(path)
	relative, err := filepath.Rel(rootPath, path)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path %q escapes matrix output root", path)
	}
	return relative, nil
}

func pinMatrixFrameInputFromRetainedRoot(ctx context.Context, rawRoot rootfs.Root, inputPath string, destRoot rootfs.Root, destDir string, index int) (string, error) {
	relative, err := relativeMatrixOutputPath(rawRoot.Path(), inputPath)
	if err != nil {
		return "", err
	}
	src, err := rawRoot.OpenFile(relative)
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf(".asc-matrix-frame-input-%d%s", index, filepath.Ext(relative))
	written, writeErr := destRoot.WriteFromPreservingMode(name, &matrixContextReader{ctx: ctx, reader: io.LimitReader(src, maxMatrixArtifactBytes+1)}, 0o600)
	closeErr := src.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if written > maxMatrixArtifactBytes {
		return "", errors.New("raw screenshot exceeds the artifact size limit")
	}
	return filepath.Join(destDir, name), nil
}

// matrixPrivateAttemptRoot retains both the rooted API used by the provider
// and an independently held parent/child descriptor. The latter lets cleanup
// find and remove the original directory after a provider renames it, while
// never recursively deleting a replacement that now occupies the old path.
type matrixPrivateAttemptRoot struct {
	root              rootfs.Root
	path              string
	grandparent       *os.Root
	parent            *os.Root
	pinned            *os.Root
	child             *os.Root
	childCreator      *os.File
	outputCreator     *os.File
	fileDACLs         []*matrixPrivateAttemptDACLHandle
	parentID          os.FileInfo
	identity          os.FileInfo
	namespaceID       os.FileInfo
	namespacePath     string
	output            *rootfs.Root
	grandparentDACL   *matrixPrivateAttemptDACLHandle
	parentDACL        *matrixPrivateAttemptDACLHandle
	childDACL         *matrixPrivateAttemptDACLHandle
	parentLocked      bool
	childLocked       bool
	grandparentLocked bool
}

func matrixPrivateAttemptOperation(stage, path string) error {
	if matrixPrivateAttemptOperationForTest != nil {
		return matrixPrivateAttemptOperationForTest(stage, path)
	}
	return nil
}

func closeMatrixPrivateAttemptRoots(roots ...*os.Root) error {
	var closeErr error
	for _, root := range roots {
		if root != nil {
			closeErr = errors.Join(closeErr, root.Close())
		}
	}
	return closeErr
}

func matrixPrivateAttemptConstructionFailure(primary, cleanupErr error, uncertain bool, roots ...*os.Root) error {
	// Before a rooted identity has been captured, the freshly-created parent
	// cannot be removed by pathname: that entry may already name a replacement.
	// In that fail-closed case the private temporary tree can remain for the
	// host's temporary-directory cleanup, and the closed uncertainty sentinel
	// tells callers not to report construction cleanup as complete.
	closeErr := closeMatrixPrivateAttemptRoots(roots...)
	if cleanupErr != nil || closeErr != nil {
		uncertain = true
	}
	var errs []error
	if primary != nil {
		errs = append(errs, primary)
	}
	if uncertain {
		errs = append(errs, errMatrixPrivateAttemptCleanupUncertain)
	}
	if cleanupErr != nil {
		errs = append(errs, cleanupErr)
	}
	if closeErr != nil {
		errs = append(errs, closeErr)
	}
	return errors.Join(errs...)
}

// lockMatrixPrivateAttemptParentRetained removes the directory-entry mutation rights
// needed to rename or replace the provider's destination. The child remains
// writable, so legacy path-based adapters can create their files, but a same
// user rename+symlink swap cannot redirect those writes outside the pinned
// child root. The platform helper applies a real protected DACL on Windows;
// mode bits alone are not a confidentiality or identity boundary there.
func lockMatrixPrivateAttemptParentRetained(parent *os.Root) (*matrixPrivateAttemptDACLHandle, error) {
	if parent == nil {
		return nil, errors.New("private matrix attempt parent is unavailable")
	}
	return lockMatrixPrivateAttemptDirectoryRetained(parent)
}

func restoreMatrixPrivateAttemptDirectory(handle *matrixPrivateAttemptDACLHandle, root *os.Root) error {
	if err := unlockMatrixPrivateAttemptDirectoryRetained(handle, root); err != nil {
		// Keep the retained WRITE_DAC handle open so the final close path can
		// retry a transient restoration failure.
		return err
	}
	return closeMatrixPrivateAttemptDACLHandle(handle)
}

type matrixPrivateAttemptDirectoryUnlockFunc func(*matrixPrivateAttemptDACLHandle, *os.Root) error

type matrixPrivateAttemptDACLHandleCloseFunc func(*matrixPrivateAttemptDACLHandle) error

func finalizeMatrixPrivateAttemptDACLHandle(handle *matrixPrivateAttemptDACLHandle) error {
	return finalizeMatrixPrivateAttemptDACLHandleWith(handle, closeMatrixPrivateAttemptDACLHandle)
}

func finalizeMatrixPrivateAttemptDACLHandleWith(handle *matrixPrivateAttemptDACLHandle, closeHandle matrixPrivateAttemptDACLHandleCloseFunc) error {
	firstErr := closeHandle(handle)
	if firstErr == nil {
		return nil
	}
	return errors.Join(firstErr, closeHandle(handle))
}

func retryMatrixPrivateAttemptDirectoryRestore(handle *matrixPrivateAttemptDACLHandle, root *os.Root) error {
	return retryMatrixPrivateAttemptDirectoryRestoreWith(handle, root, unlockMatrixPrivateAttemptDirectoryRetained)
}

func retryMatrixPrivateAttemptDirectoryRestoreWith(handle *matrixPrivateAttemptDACLHandle, root *os.Root, unlock matrixPrivateAttemptDirectoryUnlockFunc) error {
	firstErr := unlock(handle, root)
	if firstErr == nil {
		return finalizeMatrixPrivateAttemptDACLHandle(handle)
	}
	if retryErr := unlock(handle, root); retryErr != nil {
		return errors.Join(firstErr, retryErr)
	}
	return finalizeMatrixPrivateAttemptDACLHandle(handle)
}

// finalizeMatrixPrivateAttemptDirectory is for construction failures that do
// not return an attempt with a later close phase. Retry a transient restore
// once, then close the retained handle even if the directory is still locked.
func finalizeMatrixPrivateAttemptDirectory(handle *matrixPrivateAttemptDACLHandle, root *os.Root) error {
	restoreErr := retryMatrixPrivateAttemptDirectoryRestore(handle, root)
	if restoreErr == nil {
		return nil
	}
	return errors.Join(restoreErr, finalizeMatrixPrivateAttemptDACLHandle(handle))
}

func unlockMatrixPrivateAttemptParentRetained(handle *matrixPrivateAttemptDACLHandle, parent *os.Root) error {
	return restoreMatrixPrivateAttemptDirectory(handle, parent)
}

func lockMatrixPrivateAttemptChild(attempt *matrixPrivateAttemptRoot) error {
	if attempt == nil || attempt.pinned == nil {
		return errors.New("private matrix attempt root is unavailable")
	}
	if matrixPrivateAttemptBeforeChildLockForTest != nil {
		matrixPrivateAttemptBeforeChildLockForTest(attempt.path)
	}
	creator := attempt.childCreator
	attempt.childCreator = nil
	handle, err := lockMatrixPrivateAttemptDirectoryCreated(creator, attempt.pinned)
	if err != nil {
		return err
	}
	attempt.childDACL = handle
	attempt.childLocked = true
	if err := verifyMatrixPrivateAttemptChildIdentity(attempt); err != nil {
		unlockErr := unlockMatrixPrivateAttemptChild(attempt)
		return errors.Join(err, unlockErr)
	}
	return nil
}

func verifyMatrixPrivateAttemptChildIdentity(attempt *matrixPrivateAttemptRoot) error {
	if attempt == nil || attempt.grandparent == nil || attempt.parent == nil || attempt.pinned == nil || attempt.child == nil || attempt.identity == nil || attempt.parentID == nil {
		return errors.New("private matrix attempt root is unavailable")
	}
	childInfo, err := attempt.pinned.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.identity, childInfo) {
		return errors.New("private matrix attempt root changed while locking")
	}
	heldChildInfo, err := attempt.child.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.identity, heldChildInfo) {
		return errors.New("private matrix attempt root changed while locking")
	}
	parentInfo, err := attempt.parent.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.parentID, parentInfo) {
		return errors.New("private matrix attempt parent changed while locking")
	}
	parentName := filepath.Base(filepath.Dir(attempt.path))
	currentParent, err := attempt.grandparent.Lstat(parentName)
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.parentID, currentParent) {
		return errors.New("private matrix attempt parent changed while locking")
	}
	childName := filepath.Base(attempt.path)
	currentChild, err := attempt.parent.Lstat(childName)
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.identity, currentChild) {
		return errors.New("private matrix attempt root changed while locking")
	}
	reopened, err := attempt.parent.OpenRoot(childName)
	if err != nil {
		return err
	}
	reopenedInfo, statErr := reopened.Stat(".")
	closeErr := reopened.Close()
	if statErr != nil || closeErr != nil {
		return errors.Join(statErr, closeErr)
	}
	if !os.SameFile(attempt.identity, reopenedInfo) {
		return errors.New("private matrix attempt root changed while locking")
	}
	if attempt.output != nil {
		outputRoot := *attempt.output
		if err := verifyMatrixRetainedOutputRoot(outputRoot); err != nil {
			return err
		}
		openedOutput, err := outputRoot.OpenRoot()
		if err != nil {
			return err
		}
		_, statErr := openedOutput.Stat(".")
		closeErr := openedOutput.Close()
		if statErr != nil || closeErr != nil {
			return errors.Join(statErr, closeErr)
		}
	}
	return nil
}

func unlockMatrixPrivateAttemptChild(attempt *matrixPrivateAttemptRoot) error {
	if attempt == nil || !attempt.childLocked {
		return nil
	}
	if attempt.pinned == nil {
		return errors.New("private matrix attempt root is unavailable")
	}
	if err := restoreMatrixPrivateAttemptDirectory(attempt.childDACL, attempt.pinned); err != nil {
		return err
	}
	attempt.childDACL = nil
	attempt.childLocked = false
	return nil
}

// createMatrixPrivateAttemptRoot creates an owner-only staging directory for
// capture or framing adapters. Provider paths are still required by the
// existing adapter contracts, but they no longer sit below user-selected
// artifact roots where another process could replace an attempt directory and
// redirect pathname-based writes outside the selected root.
func createMatrixPrivateAttemptRoot() (result matrixPrivateAttemptRoot, returnErr error) {
	parentPath, grandparentCreator, parentCreator, err := createMatrixPrivateAttemptParentWithHandles()
	if err != nil {
		return matrixPrivateAttemptRoot{}, err
	}
	var childCreator *os.File
	defer func() {
		for _, file := range []*os.File{childCreator, parentCreator, grandparentCreator} {
			if file != nil {
				returnErr = errors.Join(returnErr, file.Close())
			}
		}
	}()
	cleanupMatrixPrivateAttemptConstruction := func(parent, grandparent, pinned *os.Root, identity, parentID os.FileInfo) error {
		var creatorErr error
		for _, file := range []*os.File{childCreator, parentCreator, grandparentCreator} {
			if file != nil {
				creatorErr = errors.Join(creatorErr, file.Close())
			}
		}
		childCreator = nil
		parentCreator = nil
		grandparentCreator = nil
		return errors.Join(creatorErr, cleanupMatrixPrivateAttemptConstructionAfterHandles(parent, grandparent, pinned, identity, parentID))
	}
	createdParentID, err := os.Lstat(parentPath)
	if err != nil || !createdParentID.IsDir() {
		if err == nil {
			err = errors.New("private matrix attempt parent is not a directory")
		}
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true)
	}
	if matrixPrivateAttemptParentCreatedForTest != nil {
		matrixPrivateAttemptParentCreatedForTest(parentPath)
	}
	grandparentPath := filepath.Dir(parentPath)
	if err := matrixPrivateAttemptOperation("grandparent_open", grandparentPath); err != nil {
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true)
	}
	grandparent, err := os.OpenRoot(grandparentPath)
	if err != nil {
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true)
	}
	parentName := filepath.Base(parentPath)
	if err := matrixPrivateAttemptOperation("parent_open", parentPath); err != nil {
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true, grandparent)
	}
	parent, err := grandparent.OpenRoot(parentName)
	if err != nil {
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true, grandparent)
	}
	if err := matrixPrivateAttemptOperation("parent_stat", parentPath); err != nil {
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true, parent, grandparent)
	}
	parentID, err := parent.Stat(".")
	if err != nil {
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true, parent, grandparent)
	}
	if !os.SameFile(createdParentID, parentID) {
		identityErr := errors.New("private matrix attempt parent changed while opening")
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(identityErr, nil, true, parent, grandparent)
	}
	name := fmt.Sprintf(".asc-matrix-attempt-%d", matrixTemporarySequence.Add(1))
	path := filepath.Join(parentPath, name)
	if err := matrixPrivateAttemptOperation("mkdir", path); err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, nil, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, parent, grandparent)
	}
	childCreator, err = createMatrixPrivateAttemptChildRetained(parent, parentPath, name)
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, nil, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, parent, grandparent)
	}
	if err := matrixPrivateAttemptOperation("child_lstat", path); err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, nil, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, true, parent, grandparent)
	}
	createdID, err := parent.Lstat(name)
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, nil, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, true, parent, grandparent)
	}
	if err := matrixPrivateAttemptOperation("child_open", path); err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, createdID, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, parent, grandparent)
	}
	anchoredChild, err := parent.OpenRoot(name)
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, createdID, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, parent, grandparent)
	}
	if err := matrixPrivateAttemptOperation("child_stat", path); err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, createdID, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, anchoredChild, parent, grandparent)
	}
	anchoredID, err := anchoredChild.Stat(".")
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, createdID, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, anchoredChild, parent, grandparent)
	}
	if !os.SameFile(createdID, anchoredID) {
		identityErr := errors.New("private matrix attempt root changed before child open")
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(identityErr, nil, true, anchoredChild, parent, grandparent)
	}
	if matrixPrivateAttemptBeforePinForTest != nil {
		matrixPrivateAttemptBeforePinForTest(path)
	}
	root, err := rootfs.New(path)
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, anchoredChild, anchoredID, parentID)
		closeErr := errors.Join(anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(err, cleanupErr, closeErr)
	}
	pinned, err := root.OpenRoot()
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, anchoredChild, anchoredID, parentID)
		closeErr := errors.Join(root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(err, cleanupErr, closeErr)
	}
	pinnedID, err := pinned.Stat(".")
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, anchoredChild, anchoredID, parentID)
		closeErr := errors.Join(pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(err, cleanupErr, closeErr)
	}
	if !os.SameFile(anchoredID, pinnedID) {
		identityErr := errors.New("private matrix attempt root changed while opening")
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, anchoredChild, anchoredID, parentID)
		closeErr := errors.Join(pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(identityErr, cleanupErr, closeErr)
	}
	if matrixPrivateAttemptRootBeforeChildRootForTest != nil {
		matrixPrivateAttemptRootBeforeChildRootForTest(path)
	}
	reopenedChild, err := parent.OpenRoot(name)
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		closeErr := errors.Join(pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(err, cleanupErr, closeErr)
	}
	identity, err := reopenedChild.Stat(".")
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		closeErr := errors.Join(reopenedChild.Close(), pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(err, cleanupErr, closeErr)
	}
	if !os.SameFile(pinnedID, identity) {
		identityErr := errors.New("private matrix attempt root changed while opening")
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		closeErr := errors.Join(reopenedChild.Close(), pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(identityErr, cleanupErr, closeErr)
	}
	if matrixPrivateAttemptBeforeParentLockForTest != nil {
		matrixPrivateAttemptBeforeParentLockForTest(path)
	}
	if err := matrixPrivateAttemptOperation("parent_lock", parentPath); err != nil {
		closeErr := reopenedChild.Close()
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		constructionErr := matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, pinned, anchoredChild, parent, grandparent)
		return matrixPrivateAttemptRoot{}, errors.Join(constructionErr, closeErr, root.Close())
	}
	parentCreationHandle := parentCreator
	parentCreator = nil
	parentDACL, err := lockMatrixPrivateAttemptDirectoryCreated(parentCreationHandle, parent)
	if err != nil {
		childCloseErr := reopenedChild.Close()
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		closeErr := errors.Join(childCloseErr, pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, errors.Join(cleanupErr, closeErr), false)
	}
	grandparentCreationHandle := grandparentCreator
	grandparentCreator = nil
	grandparentDACL, err := lockMatrixPrivateAttemptDirectoryCreated(grandparentCreationHandle, grandparent)
	if err != nil {
		unlockErr := finalizeMatrixPrivateAttemptDirectory(parentDACL, parent)
		childCloseErr := reopenedChild.Close()
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		closeErr := errors.Join(childCloseErr, pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, errors.Join(unlockErr, cleanupErr, closeErr), false)
	}
	currentParentEntry, parentEntryErr := grandparent.Lstat(parentName)
	currentParentID, parentStatErr := parent.Stat(".")
	currentChildEntry, childEntryErr := parent.Lstat(name)
	validatedChild, childOpenErr := parent.OpenRoot(name)
	var validatedChildID os.FileInfo
	var validatedChildCloseErr error
	if childOpenErr == nil {
		validatedChildID, childOpenErr = validatedChild.Stat(".")
		validatedChildCloseErr = validatedChild.Close()
	}
	if parentEntryErr != nil || parentStatErr != nil || childEntryErr != nil || childOpenErr != nil || validatedChildCloseErr != nil || !os.SameFile(createdParentID, currentParentEntry) || !os.SameFile(createdParentID, currentParentID) || !os.SameFile(pinnedID, currentChildEntry) || !os.SameFile(pinnedID, validatedChildID) {
		identityErr := errors.Join(
			errors.New("private matrix attempt root changed before parent lock"),
			parentEntryErr,
			parentStatErr,
			childEntryErr,
			childOpenErr,
			validatedChildCloseErr,
		)
		unlockErr := errors.Join(
			finalizeMatrixPrivateAttemptDirectory(grandparentDACL, grandparent),
			finalizeMatrixPrivateAttemptDirectory(parentDACL, parent),
		)
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		closeErr := errors.Join(reopenedChild.Close(), pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(identityErr, errors.Join(unlockErr, cleanupErr, closeErr), false)
	}
	closeErr := reopenedChild.Close()
	if closeErr != nil {
		unlockErr := errors.Join(
			finalizeMatrixPrivateAttemptDirectory(grandparentDACL, grandparent),
			finalizeMatrixPrivateAttemptDirectory(parentDACL, parent),
		)
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		cleanupFailure := errors.Join(unlockErr, cleanupErr, pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(closeErr, cleanupFailure, false)
	}
	namespaceID, namespaceErr := grandparent.Stat(".")
	if namespaceErr != nil {
		unlockErr := errors.Join(
			finalizeMatrixPrivateAttemptDirectory(grandparentDACL, grandparent),
			finalizeMatrixPrivateAttemptDirectory(parentDACL, parent),
		)
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		cleanupFailure := errors.Join(unlockErr, cleanupErr, pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(namespaceErr, cleanupFailure, false)
	}
	result = matrixPrivateAttemptRoot{
		root: root, path: path, grandparent: grandparent,
		parent: parent, pinned: pinned, child: anchoredChild, childCreator: childCreator, parentID: parentID, identity: identity,
		namespaceID: namespaceID, namespacePath: grandparent.Name(),
		grandparentDACL: grandparentDACL, parentDACL: parentDACL,
		parentLocked: true, grandparentLocked: true,
	}
	childCreator = nil
	return result, nil
}

// openMatrixPrivateAttemptOutputRoot creates and pins the writable directory
// handed to a rooted plan/frame adapter. The attempt child itself is locked
// before the returned root is used by an external path-based process, so the
// child/output path pair cannot be redirected by replacing the nested output
// entry. The identity comparison also closes the construction race before the
// child lock is acquired.
func openMatrixPrivateAttemptOutputRoot(attempt *matrixPrivateAttemptRoot) (rootfs.Root, error) {
	if attempt == nil || attempt.pinned == nil {
		return rootfs.Root{}, errors.New("private matrix attempt root is unavailable")
	}
	outputCreator, err := createMatrixPrivateAttemptOutputDirInRootRetained(attempt.pinned)
	if err != nil {
		return rootfs.Root{}, err
	}
	attempt.outputCreator = outputCreator
	outputPath := filepath.Join(attempt.path, "output")
	if matrixPrivateAttemptOutputBeforeRootForTest != nil {
		matrixPrivateAttemptOutputBeforeRootForTest(outputPath)
	}
	anchored, err := attempt.pinned.OpenRoot("output")
	if err != nil {
		return rootfs.Root{}, err
	}
	anchoredInfo, anchoredErr := anchored.Stat(".")
	if anchoredErr != nil {
		_ = anchored.Close()
		return rootfs.Root{}, anchoredErr
	}
	output, err := rootfs.New(outputPath)
	if err != nil {
		_ = anchored.Close()
		return rootfs.Root{}, err
	}
	opened, err := output.OpenRoot()
	if err != nil {
		_ = output.Close()
		_ = anchored.Close()
		return rootfs.Root{}, err
	}
	openedInfo, openedErr := opened.Stat(".")
	closeErr := opened.Close()
	anchoredCloseErr := anchored.Close()
	if anchoredCloseErr != nil {
		closeErr = errors.Join(closeErr, anchoredCloseErr)
	}
	if openedErr != nil || closeErr != nil || !os.SameFile(anchoredInfo, openedInfo) {
		_ = output.Close()
		if openedErr != nil || closeErr != nil {
			return rootfs.Root{}, errors.Join(openedErr, closeErr)
		}
		return rootfs.Root{}, errors.New("private matrix attempt output changed while opening")
	}
	attempt.output = &output
	return output, nil
}

func cleanupMatrixPrivateAttemptConstructionAfterHandles(parent, grandparent, pinned *os.Root, identity, parentID os.FileInfo) error {
	var cleanupErr error
	if pinned != nil {
		entries, err := fs.ReadDir(pinned.FS(), ".")
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, err)
		} else {
			for _, entry := range entries {
				cleanupErr = errors.Join(cleanupErr, pinned.RemoveAll(entry.Name()))
			}
		}
	}
	if parent == nil {
		return cleanupErr
	}
	cleanupErr = errors.Join(cleanupErr, removeMatrixExpectedEntry(parent, identity, nil))
	if empty, err := matrixPrivateAttemptDirectoryEmpty(parent); err != nil {
		cleanupErr = errors.Join(cleanupErr, err)
	} else if empty && grandparent != nil && parentID != nil {
		cleanupErr = errors.Join(cleanupErr, removeMatrixExpectedEntry(grandparent, parentID, nil))
	}
	return cleanupErr
}

func (attempt matrixPrivateAttemptRoot) matrixPrivateAttemptRootIsStable() error {
	if attempt.pinned == nil || attempt.child == nil || attempt.identity == nil {
		return errors.New("private matrix attempt root is unavailable")
	}
	pinnedInfo, err := attempt.pinned.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.identity, pinnedInfo) {
		return errors.New("private matrix attempt root identity changed")
	}
	current, err := attempt.child.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.identity, current) {
		return errors.New("private matrix attempt root identity changed")
	}
	opened, err := attempt.root.OpenRoot()
	if err != nil {
		return err
	}
	return opened.Close()
}

func (attempt *matrixPrivateAttemptRoot) cleanup() error {
	if attempt == nil {
		return nil
	}
	if attempt.grandparent == nil || attempt.parent == nil || attempt.child == nil || attempt.identity == nil {
		return nil
	}
	var restoreErr error
	if attempt.childLocked {
		restoreErr = errors.Join(restoreErr, retryMatrixPrivateAttemptDirectoryRestore(attempt.childDACL, attempt.pinned))
	}
	if attempt.parentLocked {
		restoreErr = errors.Join(restoreErr, retryMatrixPrivateAttemptDirectoryRestore(attempt.parentDACL, attempt.parent))
	}
	if attempt.grandparentLocked {
		restoreErr = errors.Join(restoreErr, retryMatrixPrivateAttemptDirectoryRestore(attempt.grandparentDACL, attempt.grandparent))
	}
	if restoreErr != nil {
		return restoreErr
	}
	for index, handle := range attempt.fileDACLs {
		restoreErr = errors.Join(restoreErr, finalizeMatrixPrivateAttemptFile(handle))
		attempt.fileDACLs[index] = nil
	}
	if attempt.outputCreator != nil {
		restoreErr = errors.Join(restoreErr, attempt.outputCreator.Close())
		attempt.outputCreator = nil
	}
	if attempt.childCreator != nil {
		restoreErr = errors.Join(restoreErr, attempt.childCreator.Close())
		attempt.childCreator = nil
	}
	if restoreErr != nil {
		return restoreErr
	}
	// Remove entries through the held child descriptor, so a rename cannot
	// redirect recursive cleanup to an unrelated directory.
	entries, err := fs.ReadDir(attempt.child.FS(), ".")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, entry := range entries {
		if err := attempt.child.RemoveAll(entry.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := removeMatrixExpectedEntry(attempt.parent, attempt.identity, matrixPrivateAttemptBeforeCleanupRemoveForTest); err != nil {
		return err
	}
	empty, err := matrixPrivateAttemptDirectoryEmpty(attempt.parent)
	if err != nil {
		return err
	}
	if !empty {
		return nil
	}
	return removeMatrixExpectedEntry(attempt.grandparent, attempt.parentID, nil)
}

func (attempt *matrixPrivateAttemptRoot) close() error {
	if attempt == nil {
		return nil
	}
	var closeErr error
	closeErr = errors.Join(closeErr, finalizeMatrixPrivateAttemptDirectory(attempt.childDACL, attempt.pinned))
	closeErr = errors.Join(closeErr, finalizeMatrixPrivateAttemptDirectory(attempt.parentDACL, attempt.parent))
	closeErr = errors.Join(closeErr, finalizeMatrixPrivateAttemptDirectory(attempt.grandparentDACL, attempt.grandparent))
	for index, handle := range attempt.fileDACLs {
		closeErr = errors.Join(closeErr, finalizeMatrixPrivateAttemptFile(handle))
		attempt.fileDACLs[index] = nil
	}
	for _, file := range []*os.File{attempt.outputCreator, attempt.childCreator} {
		if file != nil {
			closeErr = errors.Join(closeErr, file.Close())
		}
	}
	attempt.outputCreator = nil
	attempt.childCreator = nil
	if attempt.child != nil {
		closeErr = errors.Join(closeErr, attempt.child.Close())
	}
	if attempt.pinned != nil {
		closeErr = errors.Join(closeErr, attempt.pinned.Close())
	}
	closeErr = errors.Join(closeErr, attempt.root.Close())
	if attempt.parent != nil {
		closeErr = errors.Join(closeErr, attempt.parent.Close())
	}
	if attempt.grandparent != nil {
		closeErr = errors.Join(closeErr, attempt.grandparent.Close())
	}
	if nsErr := removeEmptyMatrixPrivateAttemptNamespace(attempt.namespacePath, attempt.namespaceID); nsErr != nil && !errors.Is(nsErr, os.ErrPermission) {
		// An empty leftover namespace is host-temp cleanup, not a failed
		// matrix cell. Windows can still see a sharing violation immediately
		// after the last handle is closed.
		if !isMatrixPrivateNamespaceBusy(nsErr) {
			closeErr = errors.Join(closeErr, nsErr)
		}
	}
	return closeErr
}

func isMatrixPrivateNamespaceBusy(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "being used by another process") || strings.Contains(message, "resource busy")
}

func removeEmptyMatrixPrivateAttemptNamespace(path string, identity os.FileInfo) error {
	if strings.TrimSpace(path) == "" || identity == nil {
		return nil
	}
	current, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !os.SameFile(identity, current) {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(entries) != 0 {
		return nil
	}
	var removeErr error
	for pass := 0; pass < 5; pass++ {
		removeErr = os.Remove(path)
		if removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
			return nil
		}
		if !isMatrixPrivateNamespaceBusy(removeErr) {
			return removeErr
		}
		time.Sleep(10 * time.Millisecond)
	}
	return removeErr
}

func cleanupMatrixPrivateAttemptForExecution(attempt *matrixPrivateAttemptRoot) error {
	if attempt == nil {
		return nil
	}
	err := attempt.cleanup()
	if matrixPrivateAttemptCleanupForTest != nil {
		err = errors.Join(err, matrixPrivateAttemptCleanupForTest(attempt.path))
	}
	return err
}

func closeMatrixPrivateAttemptForExecution(attempt *matrixPrivateAttemptRoot) error {
	if attempt == nil {
		return nil
	}
	err := attempt.close()
	if matrixPrivateAttemptCloseForTest != nil {
		err = errors.Join(err, matrixPrivateAttemptCloseForTest(attempt.path))
	}
	return err
}

func joinMatrixAttemptResourceErrors(attempt *matrixAttemptResult, returnErr *error, resourceErr error) {
	if resourceErr == nil {
		return
	}
	attempt.CleanupFailed = true
	*returnErr = errors.Join(*returnErr, resourceErr)
	if attempt.FailureCode == "" {
		attempt.FailureStage = "cleanup"
		attempt.FailureCode = "temporary_output_cleanup_failed"
		attempt.Error = "temporary screenshot output cleanup failed"
		return
	}
	if attempt.Error == "" {
		attempt.Error = "screenshot attempt failed; temporary output cleanup is uncertain"
		return
	}
	attempt.Error += "; temporary output cleanup is uncertain"
}

func removeMatrixExpectedEntry(parent *os.Root, expected os.FileInfo, before func(string)) error {
	if parent == nil || expected == nil {
		return nil
	}
	for pass := 0; pass < 4; pass++ {
		entries, err := fs.ReadDir(parent.FS(), ".")
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		found := false
		for _, entry := range entries {
			current, statErr := parent.Lstat(entry.Name())
			if statErr != nil || !os.SameFile(expected, current) {
				continue
			}
			current, statErr = parent.Lstat(entry.Name())
			if statErr != nil || !os.SameFile(expected, current) {
				continue
			}
			found = true
			if before != nil {
				before(filepath.Join(parent.Name(), entry.Name()))
			}
			if err := quarantineMatrixExpectedEntry(parent, entry.Name(), expected); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return err
			}
		}
		if !found {
			return nil
		}
	}
	return errors.New("matrix private attempt cleanup did not converge")
}

func quarantineMatrixExpectedEntry(parent *os.Root, name string, expected os.FileInfo) error {
	// Quarantining first keeps a replacement at the original name out of the
	// cleanup path and the repeated identity checks narrow the race window. The
	// final Remove is still path-based; rootfs has no compare-and-unlink
	// primitive, so a replacement after the last Lstat can race with removal.
	// Callers must treat any resulting cleanup error/uncertainty as a failed
	// cleanup, not as proof of atomic deletion.
	for attempt := 0; attempt < 100; attempt++ {
		quarantine := fmt.Sprintf(".asc-matrix-cleanup-%d", matrixTemporarySequence.Add(1))
		if _, err := parent.Lstat(quarantine); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := parent.Rename(name, quarantine); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return err
		}
		quarantined, err := parent.Lstat(quarantine)
		if err != nil {
			return err
		}
		if !os.SameFile(expected, quarantined) {
			restoreErr := secureopen.RenameNoReplaceInRoot(parent, quarantine, name)
			if restoreErr != nil {
				return errors.Join(errors.New("matrix private attempt entry changed during cleanup"), restoreErr)
			}
			return nil
		}
		latest, err := parent.Lstat(quarantine)
		if err != nil {
			return err
		}
		if !os.SameFile(expected, latest) {
			restoreErr := secureopen.RenameNoReplaceInRoot(parent, quarantine, name)
			if restoreErr != nil {
				return errors.Join(errors.New("matrix private attempt entry changed during cleanup"), restoreErr)
			}
			return nil
		}
		if err := parent.Remove(quarantine); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		return nil
	}
	return errors.New("matrix private attempt cleanup quarantine unavailable")
}

func matrixPrivateAttemptDirectoryEmpty(root *os.Root) (bool, error) {
	if root == nil {
		return true, nil
	}
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	return len(entries) == 0, nil
}

func matrixPrivateAttemptRootIsStable(attempt matrixPrivateAttemptRoot) error {
	return attempt.matrixPrivateAttemptRootIsStable()
}

// ValidateMatrixPlan validates all matrix inputs that can be checked without
// executing commands. The base plan must already be loaded and validated.
func ValidateMatrixPlan(plan *MatrixPlan, base *Plan) error {
	return validateMatrixPlan(plan, base, "")
}

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

// ExpandMatrix returns cells in declaration order: device, locale, appearance,
// then content variant. Paths are logical paths until RunMatrix resolves them.
func ExpandMatrix(plan *MatrixPlan, base *Plan) ([]MatrixCell, error) {
	return expandMatrix(plan, base, "")
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

// RunMatrix loads the base plan, validates the complete matrix, executes local
// cells, and writes a report even when execution is partially unsuccessful.
func RunMatrix(ctx context.Context, matrixPath string, matrixPlan *MatrixPlan, options MatrixOptions) (*MatrixResult, error) {
	return RunMatrixWithDependencies(ctx, matrixPath, matrixPlan, options, MatrixDependencies{})
}

// RunMatrixWithDependencies is the testable implementation of RunMatrix.
func RunMatrixWithDependencies(ctx context.Context, matrixPath string, matrixPlan *MatrixPlan, options MatrixOptions, dependencies MatrixDependencies) (*MatrixResult, error) {
	if dependencies.RunPlan != nil && dependencies.RunPlanRooted != nil {
		return nil, newMatrixValidationError(errors.New("RunPlan and RunPlanRooted are mutually exclusive"))
	}
	if dependencies.Frame != nil && dependencies.FrameRooted != nil {
		return nil, newMatrixValidationError(errors.New("Frame and FrameRooted are mutually exclusive"))
	}
	if matrixPlan == nil {
		return nil, newMatrixValidationError(errors.New("matrix plan is required"))
	}
	if matrixPlan.Version != 1 {
		return nil, newMatrixValidationError(fmt.Errorf("unsupported matrix plan version %d (expected 1)", matrixPlan.Version))
	}
	if strings.TrimSpace(matrixPlan.BasePlan) == "" {
		return nil, newMatrixValidationError(errors.New("base_plan is required"))
	}
	baseDir := matrixPlanSourceDir(matrixPath, matrixPlan.sourcePath)
	base, err := loadMatrixBasePlan(matrixPath, matrixPlan)
	if err != nil {
		return nil, newMatrixValidationError(fmt.Errorf("load base plan: %w", err))
	}
	if err := validateMatrixPlan(matrixPlan, base, baseDir); err != nil {
		return nil, newMatrixValidationError(err)
	}
	concurrency, attempts, backoff, err := resolveMatrixExecution(matrixPlan.Execution, options)
	if err != nil {
		return nil, newMatrixValidationError(err)
	}
	cells, err := expandMatrix(matrixPlan, base, baseDir)
	if err != nil {
		return nil, newMatrixValidationError(err)
	}
	if err := validateMatrixArtifactPathsDoNotOverwritePlans(matrixPlan, matrixPath, baseDir, cells); err != nil {
		return nil, newMatrixValidationError(err)
	}
	for i := range cells {
		cells[i].RawDir = resolveMatrixArtifactPath(baseDir, cells[i].RawDir)
		cells[i].FramedDir = resolveMatrixArtifactPath(baseDir, cells[i].FramedDir)
		for j := range cells[i].RawPaths {
			cells[i].RawPaths[j] = resolveMatrixArtifactPath(baseDir, cells[i].RawPaths[j])
			cells[i].FramedPaths[j] = resolveMatrixArtifactPath(baseDir, cells[i].FramedPaths[j])
		}
	}
	rawDir := resolveMatrixArtifactPath(baseDir, matrixPlan.Output.RawDir)
	if strings.TrimSpace(matrixPlan.Output.RawDir) == "" {
		rawDir = resolveMatrixArtifactPath(baseDir, defaultMatrixRawDir)
	}
	framedDir := resolveMatrixArtifactPath(baseDir, matrixPlan.Output.FramedDir)
	if strings.TrimSpace(matrixPlan.Output.FramedDir) == "" {
		framedDir = resolveMatrixArtifactPath(baseDir, defaultMatrixFramedDir)
	}
	reviewDir := resolveMatrixArtifactPath(baseDir, matrixPlan.Output.ReviewDir)
	if strings.TrimSpace(matrixPlan.Output.ReviewDir) == "" {
		reviewDir = resolveMatrixArtifactPath(baseDir, defaultMatrixReviewDir)
	}
	planPath := matrixPath
	if strings.TrimSpace(planPath) == "" {
		planPath = matrixPlan.sourcePath
	}
	if absolutePlanPath, pathErr := filepath.Abs(planPath); pathErr == nil {
		planPath = absolutePlanPath
	}
	result := &MatrixResult{
		PlanPath:   planPath,
		BundleID:   base.App.BundleID,
		RawDir:     rawDir,
		FramedDir:  framedDir,
		ReviewDir:  reviewDir,
		Total:      len(cells),
		TotalCells: len(cells),
		Cells:      make([]MatrixCellResult, len(cells)),
	}
	for i, cell := range cells {
		result.Cells[i] = newMatrixCellResult(cell)
	}

	deps := dependencies
	useDefaultDeviceCheck := deps.RunPlan == nil && deps.RunPlanRooted == nil && deps.Frame == nil && deps.FrameRooted == nil && deps.Appearance == nil && deps.CheckDevice == nil
	if deps.RunPlan == nil && deps.RunPlanRooted == nil {
		deps.RunPlanRooted = runPlanWithRoot
	}
	if deps.Frame == nil && deps.FrameRooted == nil {
		deps.FrameRooted = frameIntoRoot
	}
	if deps.Appearance == nil {
		deps.Appearance = &simctlMatrixAppearance{}
	}
	deviceFailures := make(map[string]matrixDeviceFailure)
	var preflightErr error
	if deps.CheckDevice == nil && useDefaultDeviceCheck {
		deviceFailures, preflightErr = checkMatrixDevices(ctx, matrixPlan)
	} else if deps.CheckDevice != nil {
		for _, device := range matrixPlan.Devices {
			if err := deps.CheckDevice(ctx, device); err != nil {
				if isMatrixContextTermination(err) {
					preflightErr = err
					break
				}
				deviceFailures[device.ID] = matrixDeviceFailure{
					Code:    matrixPreflightSimulatorNotReady,
					Message: "target simulator is not ready",
				}
			}
		}
	}
	if preflightErr != nil {
		markMatrixCellsCanceled(result)
	} else {
		for index, cell := range cells {
			failure, failed := deviceFailures[cell.Device]
			if !failed {
				continue
			}
			result.Cells[index].Status = MatrixCellFailed
			result.Cells[index].FailureStage = "preflight"
			if failure.Code == "" {
				failure.Code = matrixPreflightSimulatorNotReady
			}
			if failure.Message == "" {
				failure.Message = "target simulator is not ready"
			}
			result.Cells[index].FailureCode = failure.Code
			result.Cells[index].Error = newMatrixCellError("preflight", failure.Code, failure.Message)
			setMatrixScreenshotStatuses(&result.Cells[index])
		}
	}
	runErr := preflightErr
	var outputRoots matrixOutputRoots
	var reviewRoot rootfs.Root
	var reviewRootReady bool
	var outputRootErr error
	var releaseOutputLocks func() error
	var outputLockErr error
	rawRoot, rawRootErr := openMatrixOutputRoot(rawDir)
	if rawRootErr != nil {
		outputRootErr = fmt.Errorf("create raw output directory: %w", rawRootErr)
	} else {
		rawOpened, rootOpenErr := rawRoot.OpenRoot()
		if rootOpenErr != nil {
			_ = rawRoot.Close()
			outputRootErr = fmt.Errorf("open raw output directory: %w", rootOpenErr)
		} else {
			defer func() { _ = rawRoot.Close() }()
			defer func() { _ = rawOpened.Close() }()
			outputRoots = matrixOutputRoots{raw: rawRoot, rawPath: rawDir}
			var framedRoot rootfs.Root
			var framedOpened *os.Root
			if matrixPlan.Output.Frame.Enabled {
				framedRoot, rootOpenErr = openMatrixOutputRoot(framedDir)
				if rootOpenErr != nil {
					outputRootErr = fmt.Errorf("create framed output directory: %w", rootOpenErr)
				} else {
					framedOpened, rootOpenErr = framedRoot.OpenRoot()
					if rootOpenErr != nil {
						_ = framedRoot.Close()
						outputRootErr = fmt.Errorf("open framed output directory: %w", rootOpenErr)
					} else {
						defer func() { _ = framedRoot.Close() }()
						defer func() { _ = framedOpened.Close() }()
						outputRoots.framed = framedRoot
						outputRoots.framedPath = framedDir
						outputRoots.hasFramed = true
					}
				}
			}
			var reviewOpened *os.Root
			if outputRootErr == nil {
				reviewRoot, rootOpenErr = openMatrixOutputRoot(reviewDir)
				if rootOpenErr != nil {
					outputRootErr = fmt.Errorf("create matrix review output directory: %w", rootOpenErr)
				} else {
					reviewOpened, rootOpenErr = reviewRoot.OpenRoot()
					if rootOpenErr != nil {
						_ = reviewRoot.Close()
						outputRootErr = fmt.Errorf("open matrix review output directory: %w", rootOpenErr)
					} else {
						reviewRootReady = true
						defer func() { _ = reviewRoot.Close() }()
						defer func() { _ = reviewOpened.Close() }()
					}
				}
			}
			if outputRootErr == nil {
				targets := []matrixOutputLockTarget{{root: rawRoot, opened: rawOpened}, {root: reviewRoot, opened: reviewOpened}}
				if matrixPlan.Output.Frame.Enabled {
					targets = append(targets, matrixOutputLockTarget{root: outputRoots.framed, opened: framedOpened})
				}
				releaseOutputLocks, outputLockErr = acquireMatrixOutputLocksForRoots(ctx, targets)
				if outputLockErr != nil {
					releaseOutputLocks = nil
				} else if matrixOutputLocksAcquiredForTest != nil {
					matrixOutputLocksAcquiredForTest()
				}
			}
		}
	}
	if outputLockErr != nil {
		outputRootErr = matrixLockError("output", outputLockErr)
	}
	if outputRootErr != nil {
		if isMatrixContextTermination(outputRootErr) {
			markMatrixCellsCanceled(result)
		} else {
			markMatrixOutputFailure(result)
		}
		if runErr == nil {
			runErr = outputRootErr
		} else {
			runErr = errors.Join(runErr, outputRootErr)
		}
	}
	if runErr == nil {
		runErr = executeMatrixCells(ctx, cells, deviceFailures, base, matrixPlan, concurrency, attempts, backoff, deps, outputRoots, result)
	}
	rawVerificationCtx := ctx
	if ctx.Err() != nil {
		rawVerificationCtx = context.WithoutCancel(ctx)
	}
	var retainedRawRoot, retainedFramedRoot *rootfs.Root
	if outputRoots.raw.Path() != "" {
		retainedRawRoot = &outputRoots.raw
	}
	if outputRoots.hasFramed && outputRoots.framed.Path() != "" {
		retainedFramedRoot = &outputRoots.framed
	}
	if rawErr := revalidateMatrixRawPathsWithRoot(rawVerificationCtx, result, matrixSubprocessTimeout, retainedRawRoot); rawErr != nil {
		if runErr == nil {
			runErr = rawErr
		} else {
			runErr = errors.Join(runErr, rawErr)
		}
	}
	framedVerificationCtx := ctx
	if ctx.Err() != nil {
		// A run can finish its last cell successfully and be canceled during
		// appearance restoration immediately afterward. Preserve those already
		// completed framed artifacts by verifying them with bounded per-artifact
		// contexts detached from the canceled caller. Cancellation that arrives
		// after verification starts still propagates through the live ctx above.
		framedVerificationCtx = context.WithoutCancel(ctx)
	}
	if framedErr := revalidateMatrixFramedPathsWithRoot(framedVerificationCtx, result, matrixSubprocessTimeout, retainedFramedRoot); framedErr != nil {
		if runErr == nil {
			runErr = framedErr
		} else {
			runErr = errors.Join(runErr, framedErr)
		}
	}
	countMatrixResultStatuses(result)
	if matrixBeforeReviewPublishForTest != nil {
		matrixBeforeReviewPublishForTest()
	}
	reviewCtx := context.WithoutCancel(ctx)
	if outputLockErr == nil {
		review, reviewErr := publishMatrixReview(reviewCtx, result, reviewDir, reviewRoot, reviewRootReady, ctx)
		if reviewErr == nil {
			result.Review = review
		} else if runErr == nil {
			result.Status = MatrixCellFailed
			runErr = fmt.Errorf("write matrix review: %w", reviewErr)
		} else {
			result.Status = MatrixCellFailed
			runErr = errors.Join(runErr, fmt.Errorf("write matrix review: %w", reviewErr))
		}
	}
	if releaseOutputLocks != nil {
		releaseErr := releaseOutputLocks()
		if matrixOutputLockReleaseErrForTest != nil {
			releaseErr = errors.Join(releaseErr, matrixOutputLockReleaseErrForTest)
		}
		if releaseErr != nil {
			lockErr := matrixLockError("output release", releaseErr)
			if result.Status == MatrixCellSuccess {
				result.Status = MatrixCellFailed
			}
			if runErr == nil {
				runErr = lockErr
			} else {
				runErr = errors.Join(runErr, lockErr)
			}
			if republished, republishErr := publishMatrixReview(reviewCtx, result, reviewDir, reviewRoot, reviewRootReady, ctx); republishErr == nil {
				result.Review = republished
			} else {
				runErr = errors.Join(runErr, fmt.Errorf("write matrix review: %w", republishErr))
			}
		}
	}
	return result, runErr
}

func publishMatrixReview(ctx context.Context, result *MatrixResult, reviewDir string, reviewRoot rootfs.Root, reviewRootReady bool, lockCtx context.Context) (*MatrixReviewResult, error) {
	reviewRequest := MatrixReviewRequest{Result: result, OutputDir: reviewDir, LockContext: lockCtx}
	if reviewRootReady {
		if err := verifyMatrixRetainedOutputRoot(reviewRoot); err != nil {
			return nil, err
		}
		return generateMatrixReviewWithRoot(ctx, reviewRequest, reviewRoot)
	}
	return GenerateMatrixReview(ctx, reviewRequest)
}

func resolveMatrixExecution(execution MatrixExecution, options MatrixOptions) (int, int, time.Duration, error) {
	concurrency := execution.MaxConcurrency
	if options.MaxConcurrencySet || options.MaxConcurrency != 0 {
		concurrency = options.MaxConcurrency
	}
	if concurrency == 0 && !options.MaxConcurrencySet {
		concurrency = defaultMatrixConcurrency
	}
	if concurrency < 1 || concurrency > maxMatrixConcurrency {
		return 0, 0, 0, fmt.Errorf("max concurrency must be between 1 and %d", maxMatrixConcurrency)
	}
	attempts := execution.MaxAttempts
	if options.MaxAttemptsSet || options.MaxAttempts != 0 {
		attempts = options.MaxAttempts
	}
	if attempts == 0 && !options.MaxAttemptsSet {
		attempts = defaultMatrixAttempts
	}
	if attempts < 1 || attempts > maxMatrixAttempts {
		return 0, 0, 0, fmt.Errorf("max attempts must be between 1 and %d", maxMatrixAttempts)
	}
	if int64(execution.RetryBackoffMS) > maxMatrixRetryBackoffMS {
		return 0, 0, 0, errors.New("retry backoff milliseconds exceeds maximum duration")
	}
	backoff := time.Duration(execution.RetryBackoffMS) * time.Millisecond
	if strings.TrimSpace(execution.RetryBackoff) != "" {
		parsed, err := time.ParseDuration(strings.TrimSpace(execution.RetryBackoff))
		if err != nil {
			return 0, 0, 0, errors.New("retry backoff must be a valid duration")
		}
		backoff = parsed
	}
	if options.RetryBackoffSet || options.RetryBackoff != 0 {
		backoff = options.RetryBackoff
	}
	if backoff < 0 {
		return 0, 0, 0, errors.New("retry backoff must be >= 0")
	}
	return concurrency, attempts, backoff, nil
}

func executeMatrixCells(ctx context.Context, cells []MatrixCell, deviceFailures map[string]matrixDeviceFailure, base *Plan, matrixPlan *MatrixPlan, concurrency, attempts int, backoff time.Duration, deps MatrixDependencies, outputRoots matrixOutputRoots, result *MatrixResult) error {
	jobs := make(chan int)
	var workers sync.WaitGroup
	guards := make(map[string]*matrixSimulatorGuard)
	for _, cell := range cells {
		guardKey := normalizeMatrixUDID(cell.UDID)
		if _, ok := guards[guardKey]; !ok {
			guards[guardKey] = &matrixSimulatorGuard{}
		}
	}
	workerCount := concurrency
	if workerCount > len(cells) {
		workerCount = len(cells)
	}
	if workerCount == 0 {
		return nil
	}
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				if _, failed := deviceFailures[cells[index].Device]; failed {
					continue
				}
				cellResult := executeMatrixCell(ctx, cells[index], base, matrixPlan, attempts, backoff, deps, outputRoots, guards[normalizeMatrixUDID(cells[index].UDID)])
				result.Cells[index] = cellResult
			}
		}()
	}
	for index := range cells {
		select {
		case jobs <- index:
		case <-ctx.Done():
			break
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobs)
	workers.Wait()
	if ctx.Err() != nil {
		canceledCells := false
		for i := range result.Cells {
			if result.Cells[i].Status != MatrixCellCanceled {
				continue
			}
			canceledCells = true
			if result.Cells[i].Attempts == 0 {
				result.Cells[i].FailureStage = "execution"
				result.Cells[i].FailureCode = "canceled"
				result.Cells[i].Error = newMatrixCellError("execution", "canceled", "cell canceled")
			}
		}
		if canceledCells {
			return ctx.Err()
		}
	}
	for _, cell := range result.Cells {
		if cell.Status == MatrixCellFailed || cell.Status == MatrixCellCleanupFailed {
			return errors.New("one or more matrix cells failed")
		}
	}
	return nil
}

type matrixSimulatorGuard struct {
	mu      sync.Mutex
	blocked bool
}

func executeMatrixCellWithSimulatorLock(ctx context.Context, cell MatrixCell, base *Plan, matrixPlan *MatrixPlan, maxAttempts int, backoff time.Duration, deps MatrixDependencies, outputRoots matrixOutputRoots, guard *matrixSimulatorGuard) MatrixCellResult {
	started := time.Now()
	result := newMatrixCellResult(cell)
	result.Status = MatrixCellFailed
	if guard == nil {
		guard = &matrixSimulatorGuard{}
	}
	guard.mu.Lock()
	defer guard.mu.Unlock()
	if guard.blocked {
		result.FailureStage = "appearance"
		result.FailureCode = "simulator_blocked_after_cleanup_failure"
		result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "simulator blocked after appearance cleanup failure")
		setMatrixScreenshotStatuses(&result)
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}

	if err := ctx.Err(); err != nil {
		return finishMatrixCellFailure(result, started, "execution", "canceled", "cell canceled")
	}
	state, err := deps.Appearance.Snapshot(ctx, cell.UDID)
	if err != nil {
		if ctx.Err() != nil {
			return finishMatrixCellFailure(result, started, "execution", "canceled", "cell canceled")
		}
		return finishMatrixCellFailure(result, started, "appearance", "snapshot_failed", "appearance state could not be read")
	}
	if err := deps.Appearance.Set(ctx, cell.UDID, cell.Appearance); err != nil {
		if ctx.Err() != nil {
			result = finishMatrixCellFailure(result, started, "execution", "canceled", "cell canceled")
		} else {
			result = finishMatrixCellFailure(result, started, "appearance", "set_failed", "requested appearance could not be applied")
		}
		if restoreErr := restoreMatrixAppearance(deps.Appearance, cell.UDID, state); restoreErr != nil {
			guard.blocked = true
			result.Status = MatrixCellCleanupFailed
			result.FailureStage = "cleanup"
			result.FailureCode = "appearance_restore_failed"
			result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "simulator appearance could not be restored")
		}
		setMatrixScreenshotStatuses(&result)
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result.Attempts = attempt
		if err := ctx.Err(); err != nil {
			result.Status = MatrixCellCanceled
			result.FailureStage = "execution"
			result.FailureCode = "canceled"
			result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "cell canceled")
			break
		}
		attemptResult, attemptErr := executeMatrixCellAttempt(ctx, cell, base, matrixPlan, deps, outputRoots)
		result.Steps = attemptResult.Steps
		mergeMatrixAttemptResult(&result, cell, attemptResult)
		if attemptErr == nil {
			result.Status = MatrixCellSuccess
			result.FailureStage = ""
			result.FailureCode = ""
			result.Error = nil
			break
		}
		if ctx.Err() != nil && !attemptResult.CleanupFailed {
			result.Status = MatrixCellCanceled
			if attemptResult.FailureStage == "framing" {
				result.FailureStage = "framing"
			} else {
				result.FailureStage = "execution"
			}
			result.FailureCode = "canceled"
			result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "cell canceled")
			break
		}
		if attemptResult.CleanupFailed {
			result.Status = MatrixCellCleanupFailed
		}
		result.FailureStage = attemptResult.FailureStage
		result.FailureCode = attemptResult.FailureCode
		result.Error = newMatrixCellError(attemptResult.FailureStage, attemptResult.FailureCode, attemptResult.Error)
		if attempt == maxAttempts || attemptResult.CleanupFailed || (attemptResult.FailureStage != "execution" && attemptResult.FailureStage != "framing") {
			break
		}
		if err := waitContext(ctx, backoff); err != nil {
			result.Status = MatrixCellCanceled
			if result.FailureStage != "framing" {
				result.FailureStage = "execution"
			}
			result.FailureCode = "canceled"
			result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "cell canceled")
			break
		}
	}

	restoreErr := restoreMatrixAppearance(deps.Appearance, cell.UDID, state)
	if restoreErr != nil {
		guard.blocked = true
		result.Status = MatrixCellCleanupFailed
		result.FailureStage = "cleanup"
		result.FailureCode = "appearance_restore_failed"
		result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "simulator appearance could not be restored")
	} else if result.Status == MatrixCellFailed && result.FailureCode == "" {
		result.FailureStage = "execution"
		result.FailureCode = "execution_failed"
		result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "cell execution failed")
	}
	setMatrixScreenshotStatuses(&result)
	result.DurationMS = time.Since(started).Milliseconds()
	return result
}

func restoreMatrixAppearance(appearance MatrixAppearance, udid, state string) error {
	restoreCtx, cancel := context.WithTimeout(context.Background(), matrixSubprocessTimeout)
	defer cancel()
	return appearance.Restore(restoreCtx, udid, state)
}

type matrixAttemptResult struct {
	RawPaths        []string
	RawArtifacts    map[string]matrixArtifactInfo
	FramedPaths     []string
	FramedArtifacts map[string]matrixArtifactInfo
	CleanupFailed   bool
	Screenshots     []MatrixScreenshotResult
	Steps           []RunStepResult
	FailureStage    string
	FailureCode     string
	Error           string
}

func matrixPrivateAttemptConstructionResult(stage, message string, err error) (matrixAttemptResult, error) {
	result := matrixAttemptResult{FailureStage: stage, FailureCode: "temporary_output_failed", Error: message}
	if errors.Is(err, errMatrixPrivateAttemptCleanupUncertain) {
		result.CleanupFailed = true
		result.FailureStage = "cleanup"
		result.FailureCode = "temporary_output_cleanup_failed"
		result.Error = "temporary screenshot output cleanup is uncertain"
	}
	return result, err
}

func executeMatrixCellAttempt(ctx context.Context, cell MatrixCell, base *Plan, matrixPlan *MatrixPlan, deps MatrixDependencies, outputRoots matrixOutputRoots) (attempt matrixAttemptResult, returnErr error) {
	rawRelative, err := relativeMatrixOutputPath(outputRoots.rawPath, cell.RawDir)
	if err != nil {
		return matrixAttemptResult{FailureStage: "execution", FailureCode: "temporary_output_failed", Error: "temporary output directory could not be created"}, err
	}
	if err := outputRoots.raw.MkdirAll(rawRelative, 0o755); err != nil {
		return matrixAttemptResult{FailureStage: "execution", FailureCode: "temporary_output_failed", Error: "temporary output directory could not be created"}, err
	}
	attemptRoot, err := createMatrixPrivateAttemptRoot()
	if err != nil {
		return matrixPrivateAttemptConstructionResult("execution", "temporary output directory could not be created", err)
	}
	defer func() {
		cleanupErr := cleanupMatrixPrivateAttemptForExecution(&attemptRoot)
		closeErr := closeMatrixPrivateAttemptForExecution(&attemptRoot)
		joinMatrixAttemptResourceErrors(&attempt, &returnErr, errors.Join(cleanupErr, closeErr))
	}()
	attemptDir := attemptRoot.path
	var planOutputRoot rootfs.Root
	planUsesRootedOutput := deps.RunPlan == nil
	if planUsesRootedOutput {
		planOutputRoot, err = openMatrixPrivateAttemptOutputRoot(&attemptRoot)
		if err != nil {
			attempt.FailureStage = "execution"
			attempt.FailureCode = "temporary_output_failed"
			attempt.Error = "temporary output directory could not be created"
			return attempt, err
		}
		defer func() {
			if closeErr := planOutputRoot.Close(); closeErr != nil {
				joinMatrixAttemptResourceErrors(&attempt, &returnErr, closeErr)
			}
		}()
		if err := lockMatrixPrivateAttemptChild(&attemptRoot); err != nil {
			attempt.FailureStage = "execution"
			attempt.FailureCode = "temporary_output_failed"
			attempt.Error = "temporary output directory could not be protected"
			return attempt, err
		}
	}
	plan, err := cloneScreenshotPlan(base)
	if err != nil {
		return matrixAttemptResult{FailureStage: "execution", FailureCode: "plan_clone_failed", Error: "base plan could not be prepared"}, err
	}
	plan.App.UDID = cell.UDID
	if planUsesRootedOutput {
		plan.App.OutputDir = planOutputRoot.Path()
	} else {
		plan.App.OutputDir = attemptDir
	}
	plan.App.LaunchArguments = append(append([]string(nil), base.App.LaunchArguments...), cell.LaunchArguments...)
	plan.App.terminateRunningProcess = true
	ensureMatrixLaunchStep(plan)
	var runResult *RunResult
	if deps.RunPlan != nil {
		runResult, err = deps.RunPlan(ctx, plan)
	} else {
		runResult, err = deps.RunPlanRooted(ctx, plan, planOutputRoot)
	}
	attempt = matrixAttemptResult{}
	if runResult != nil {
		attempt.Steps = sanitizeMatrixSteps(runResult.Steps)
	}
	stabilityErr := matrixPrivateAttemptRootIsStable(attemptRoot)
	if err != nil {
		if stabilityErr != nil {
			attempt.FailureStage = "execution"
			attempt.FailureCode = "temporary_output_failed"
			attempt.Error = "screenshot capture output changed during execution"
			return attempt, errors.Join(err, stabilityErr)
		}
		attempt.FailureStage = "execution"
		attempt.FailureCode = "plan_failed"
		attempt.Error = "screenshot plan execution failed"
		return attempt, err
	}
	if stabilityErr != nil {
		attempt.FailureStage = "execution"
		attempt.FailureCode = "temporary_output_failed"
		attempt.Error = "screenshot capture output changed during execution"
		return attempt, stabilityErr
	}
	if runResult == nil {
		attempt.FailureStage = "execution"
		attempt.FailureCode = "empty_result"
		attempt.Error = "screenshot plan returned no result"
		return attempt, errors.New("empty screenshot result")
	}
	sourceRoot := attemptRoot.root
	sourceRootPath := attemptDir
	if planUsesRootedOutput {
		sourceRoot = planOutputRoot
		sourceRootPath = planOutputRoot.Path()
	}
	sources := make([]string, 0, len(cell.RawPaths))
	names := make([]string, 0, len(cell.RawPaths))
	dimensionsList := make([]asc.ImageDimensions, 0, len(cell.RawPaths))
	for _, rawPath := range cell.RawPaths {
		name := filepath.Base(rawPath)
		source := filepath.Join(sourceRootPath, name)
		sourceRelative := name
		sourceFile, openErr := sourceRoot.OpenFile(sourceRelative)
		if openErr != nil {
			attempt.FailureStage = "execution"
			attempt.FailureCode = "missing_screenshot"
			attempt.Error = "screenshot plan did not produce every requested image"
			return attempt, openErr
		}
		dimensions, imageErr := readMatrixImageDimensions(sourceFile, source)
		closeErr := sourceFile.Close()
		if imageErr != nil || closeErr != nil {
			attempt.FailureStage = "execution"
			attempt.FailureCode = "invalid_screenshot"
			attempt.Error = "screenshot plan produced an invalid image"
			if imageErr != nil {
				return attempt, imageErr
			}
			return attempt, closeErr
		}
		sources = append(sources, source)
		names = append(names, strings.TrimSuffix(name, filepath.Ext(name)))
		dimensionsList = append(dimensionsList, dimensions)
	}
	for index, rawPath := range cell.RawPaths {
		source := sources[index]
		destination := rawPath
		artifact, err := promoteMatrixArtifactWithInfoFromRoots(ctx, sourceRoot, sourceRootPath, source, outputRoots.raw, outputRoots.rawPath, destination)
		if err != nil {
			attempt.FailureStage = "execution"
			attempt.FailureCode = "raw_output_failed"
			attempt.Error = "raw screenshot could not be promoted"
			return attempt, err
		}
		if attempt.RawArtifacts == nil {
			attempt.RawArtifacts = make(map[string]matrixArtifactInfo)
		}
		attempt.RawPaths = append(attempt.RawPaths, destination)
		attempt.RawArtifacts[destination] = artifact
		attempt.Screenshots = append(attempt.Screenshots, MatrixScreenshotResult{
			Name: names[index], Status: MatrixCellSuccess,
			RawPath: destination, Width: dimensionsList[index].Width, Height: dimensionsList[index].Height,
		})
	}

	if !matrixPlan.Output.Frame.Enabled {
		return attempt, nil
	}
	framedRelative, err := relativeMatrixOutputPath(outputRoots.framedPath, cell.FramedDir)
	if err != nil {
		attempt.FailureStage = "framing"
		attempt.FailureCode = "framed_output_failed"
		attempt.Error = "framed output directory could not be created"
		return attempt, err
	}
	if err := outputRoots.framed.MkdirAll(framedRelative, 0o755); err != nil {
		attempt.FailureStage = "framing"
		attempt.FailureCode = "framed_output_failed"
		attempt.Error = "framed output directory could not be created"
		return attempt, err
	}
	frameAttemptRoot, err := createMatrixPrivateAttemptRoot()
	if err != nil {
		construction, constructionErr := matrixPrivateAttemptConstructionResult("framing", "temporary frame output directory could not be created", err)
		attempt.CleanupFailed = construction.CleanupFailed
		attempt.FailureStage = construction.FailureStage
		attempt.FailureCode = construction.FailureCode
		attempt.Error = construction.Error
		return attempt, constructionErr
	}
	defer func() {
		cleanupErr := cleanupMatrixPrivateAttemptForExecution(&frameAttemptRoot)
		closeErr := closeMatrixPrivateAttemptForExecution(&frameAttemptRoot)
		joinMatrixAttemptResourceErrors(&attempt, &returnErr, errors.Join(cleanupErr, closeErr))
	}()
	frameAttemptDir := frameAttemptRoot.path
	var frameOutputRoot rootfs.Root
	frameUsesRootedOutput := deps.Frame == nil
	if frameUsesRootedOutput {
		frameOutputRoot, err = openMatrixPrivateAttemptOutputRoot(&frameAttemptRoot)
		if err != nil {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "temporary_output_failed"
			attempt.Error = "temporary frame output directory could not be created"
			return attempt, err
		}
		defer func() {
			if closeErr := frameOutputRoot.Close(); closeErr != nil {
				joinMatrixAttemptResourceErrors(&attempt, &returnErr, closeErr)
			}
		}()
		if err := lockMatrixPrivateAttemptChild(&frameAttemptRoot); err != nil {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "temporary_output_failed"
			attempt.Error = "temporary frame output directory could not be protected"
			return attempt, err
		}
	}
	frameDevice, frameMappingFound := matrixFrameMappingForDevice(matrixPlan.Output.Frame.DeviceByMatrixDevice, cell.Device)
	if !frameMappingFound || strings.TrimSpace(frameDevice) == "" {
		attempt.FailureStage = "framing"
		attempt.FailureCode = "framing_mapping_missing"
		attempt.Error = "framing device mapping could not be resolved"
		return attempt, errors.New("framing device mapping is missing")
	}
	frameDevice = strings.TrimSpace(frameDevice)
	frameSourceRoot := frameAttemptRoot.root
	frameSourceRootPath := frameAttemptDir
	if frameUsesRootedOutput {
		frameSourceRoot = frameOutputRoot
		frameSourceRootPath = frameOutputRoot.Path()
	}
	if err := verifyMatrixRetainedOutputRoot(outputRoots.raw); err != nil {
		attempt.FailureStage = "framing"
		attempt.FailureCode = "raw_output_failed"
		attempt.Error = "raw screenshot root changed before framing"
		return attempt, err
	}
	frameSources := make([]string, 0, len(attempt.RawPaths))
	for index, inputPath := range attempt.RawPaths {
		contained, containsErr := outputRoots.raw.ContainsPath(inputPath)
		if containsErr != nil || !contained {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "raw_output_failed"
			attempt.Error = "raw screenshot is no longer inside the retained raw root"
			if containsErr != nil {
				return attempt, containsErr
			}
			return attempt, errors.New("raw screenshot left the retained raw root")
		}
		tempFrame := filepath.Join(frameSourceRootPath, filepath.Base(cell.FramedPaths[index]))
		frameInputPath := inputPath
		if deps.Frame == nil {
			pinnedInput, pinErr := pinMatrixFrameInputFromRetainedRoot(ctx, outputRoots.raw, inputPath, frameSourceRoot, frameSourceRootPath, index)
			if pinErr != nil {
				attempt.FailureStage = "framing"
				attempt.FailureCode = "raw_output_failed"
				attempt.Error = "raw screenshot could not be pinned for framing"
				return attempt, pinErr
			}
			frameInputPath = pinnedInput
		}
		frameRequest := FrameRequest{InputPath: frameInputPath, OutputPath: tempFrame, Device: frameDevice}
		var frameResult *FrameResult
		var frameErr error
		if deps.Frame != nil {
			frameResult, frameErr = deps.Frame(ctx, frameRequest)
		} else {
			frameResult, frameErr = deps.FrameRooted(ctx, frameRequest, frameOutputRoot)
		}
		stabilityErr := matrixPrivateAttemptRootIsStable(frameAttemptRoot)
		if frameErr != nil || frameResult == nil {
			attempt.FailureStage = "framing"
			if stabilityErr != nil {
				attempt.FailureCode = "temporary_output_failed"
				attempt.Error = "framing output changed during execution"
				return attempt, errors.Join(frameErr, stabilityErr)
			}
			attempt.FailureCode = "frame_failed"
			attempt.Error = "screenshot framing failed"
			if frameErr != nil {
				return attempt, frameErr
			}
			return attempt, errors.New("frame failed")
		}
		if stabilityErr != nil {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "temporary_output_failed"
			attempt.Error = "framing output changed during execution"
			return attempt, stabilityErr
		}
		tempFrameFile, openErr := frameSourceRoot.OpenFile(filepath.Base(tempFrame))
		if openErr != nil {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "invalid_frame"
			attempt.Error = "screenshot framing produced an invalid image"
			return attempt, openErr
		}
		_, imageErr := readMatrixImageDimensions(tempFrameFile, tempFrame)
		closeErr := tempFrameFile.Close()
		if imageErr != nil || closeErr != nil {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "invalid_frame"
			attempt.Error = "screenshot framing produced an invalid image"
			if imageErr != nil {
				return attempt, imageErr
			}
			return attempt, closeErr
		}
		frameSources = append(frameSources, tempFrame)
	}
	for index, tempFrame := range frameSources {
		artifact, err := promoteMatrixArtifactWithInfoFromRoots(ctx, frameSourceRoot, frameSourceRootPath, tempFrame, outputRoots.framed, outputRoots.framedPath, cell.FramedPaths[index])
		if err != nil {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "framed_output_failed"
			attempt.Error = "framed screenshot could not be promoted"
			return attempt, err
		}
		if attempt.FramedArtifacts == nil {
			attempt.FramedArtifacts = make(map[string]matrixArtifactInfo)
		}
		attempt.FramedPaths = append(attempt.FramedPaths, cell.FramedPaths[index])
		attempt.FramedArtifacts[cell.FramedPaths[index]] = artifact
		attempt.Screenshots[index].FramedPath = cell.FramedPaths[index]
	}
	return attempt, nil
}

func sanitizeMatrixSteps(steps []RunStepResult) []RunStepResult {
	if len(steps) == 0 {
		return nil
	}
	sanitized := make([]RunStepResult, len(steps))
	for index, step := range steps {
		sanitized[index] = RunStepResult{
			Index: step.Index, Action: step.Action, Status: step.Status, DurationMS: step.DurationMS,
		}
		if strings.TrimSpace(step.Error) != "" {
			sanitized[index].Error = "step execution failed"
		}
	}
	return sanitized
}

// ensureMatrixLaunchStep guarantees the cell's app session is established with
// this cell's locale and content-variant launch arguments before any step that
// observes or drives the app runs.
//
// A plain "does the plan launch anywhere" check is not sufficient: a valid base
// plan may place a screenshot or interaction before a later launch, and those
// early steps would then run against whatever session the simulator already had,
// producing artifacts mislabeled for the requested axes. Only leading
// app-independent steps may precede the launch.
func ensureMatrixLaunchStep(plan *Plan) {
	for _, step := range plan.Steps {
		if matrixStepIsAppIndependent(step.Action) {
			continue
		}
		if step.Action == ActionLaunch {
			return
		}
		break
	}
	plan.Steps = append([]PlanStep{{Action: ActionLaunch}}, plan.Steps...)
}

// matrixStepIsAppIndependent reports whether a step neither observes nor drives
// the app under test, and may therefore precede the matrix launch. Only an
// unconditional delay qualifies; every other action reads or manipulates app
// state. ActionWaitFor is excluded because it polls for on-screen content.
func matrixStepIsAppIndependent(action StepAction) bool {
	return action == ActionWait
}

func cloneScreenshotPlan(base *Plan) (*Plan, error) {
	data, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	var clone Plan
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}

func promoteMatrixArtifact(outputRoot rootfs.Root, outputRootPath, source, destination string) error {
	return promoteMatrixArtifactFromRoots(context.Background(), outputRoot, outputRootPath, source, outputRoot, outputRootPath, destination)
}

func promoteMatrixArtifactFromRoots(ctx context.Context, sourceRoot rootfs.Root, sourceRootPath, source string, destinationRoot rootfs.Root, destinationRootPath, destination string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	sourceRelative, err := relativeMatrixOutputPath(sourceRootPath, source)
	if err != nil {
		return err
	}
	sourceFile, err := sourceRoot.OpenFile(sourceRelative)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	destinationRelative, err := relativeMatrixOutputPath(destinationRootPath, destination)
	if err != nil {
		return err
	}
	written, err := destinationRoot.WriteFromPreservingMode(destinationRelative, &matrixContextReader{ctx: ctx, reader: io.LimitReader(sourceFile, maxMatrixArtifactBytes+1)}, 0o644)
	if err != nil {
		return err
	}
	if written > maxMatrixArtifactBytes {
		return errors.New("screenshot exceeds the artifact size limit")
	}
	return nil
}

type matrixArtifactInfo struct {
	identity os.FileInfo
	size     int64
	digest   [sha256.Size]byte
}

func promoteMatrixArtifactWithInfo(ctx context.Context, outputRoot rootfs.Root, outputRootPath, source, destination string) (matrixArtifactInfo, error) {
	return promoteMatrixArtifactWithInfoFromRoots(ctx, outputRoot, outputRootPath, source, outputRoot, outputRootPath, destination)
}

func promoteMatrixArtifactWithInfoFromRoots(ctx context.Context, sourceRoot rootfs.Root, sourceRootPath, source string, destinationRoot rootfs.Root, destinationRootPath, destination string) (matrixArtifactInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return matrixArtifactInfo{}, err
	}
	if err := promoteMatrixArtifactFromRoots(ctx, sourceRoot, sourceRootPath, source, destinationRoot, destinationRootPath, destination); err != nil {
		return matrixArtifactInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return matrixArtifactInfo{}, err
	}
	return inspectMatrixArtifactWithContext(ctx, destinationRoot, destinationRootPath, destination)
}

func inspectMatrixArtifact(outputRoot rootfs.Root, outputRootPath, path string) (matrixArtifactInfo, error) {
	return inspectMatrixArtifactWithContext(context.Background(), outputRoot, outputRootPath, path)
}

func inspectMatrixArtifactWithContext(ctx context.Context, outputRoot rootfs.Root, outputRootPath, path string) (matrixArtifactInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return matrixArtifactInfo{}, err
	}
	relative, err := relativeMatrixOutputPath(outputRootPath, path)
	if err != nil {
		return matrixArtifactInfo{}, err
	}
	if matrixArtifactBeforeOpenForTest != nil {
		matrixArtifactBeforeOpenForTest(outputRoot, path)
	}
	file, err := outputRoot.OpenFile(relative)
	if err != nil {
		return matrixArtifactInfo{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return matrixArtifactInfo{}, err
	}
	if info.Size() > maxMatrixArtifactBytes {
		return matrixArtifactInfo{}, errors.New("matrix artifact exceeds the size limit")
	}
	hasher := sha256.New()
	size, err := io.Copy(hasher, io.LimitReader(&matrixContextReader{ctx: ctx, reader: file}, maxMatrixArtifactBytes+1))
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return matrixArtifactInfo{}, contextErr
		}
		return matrixArtifactInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return matrixArtifactInfo{}, err
	}
	if size > maxMatrixArtifactBytes {
		return matrixArtifactInfo{}, errors.New("matrix artifact exceeds the size limit")
	}
	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))
	return matrixArtifactInfo{identity: info, size: size, digest: digest}, nil
}

type matrixContextReader struct {
	ctx    context.Context
	reader io.Reader
}

// matrixArtifactBudgetContext combines a per-artifact timer with the caller's
// context while forwarding Err directly to the caller. The latter matters for
// context implementations whose Done channel is nil but whose Err can still
// report cancellation (and keeps cancellation observable during hashing).
type matrixArtifactBudgetContext struct {
	parent context.Context
	timer  context.Context
	done   chan struct{}

	closeOnce  sync.Once
	stopParent func() bool
	stopTimer  func() bool
}

func newMatrixArtifactBudgetContext(parent context.Context, budget time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	timer, cancelTimer := context.WithTimeout(context.Background(), budget)
	combined := &matrixArtifactBudgetContext{parent: parent, timer: timer, done: make(chan struct{})}
	combined.stopTimer = context.AfterFunc(timer, combined.closeDone)
	if parent.Done() != nil {
		combined.stopParent = context.AfterFunc(parent, combined.closeDone)
	}
	return combined, func() {
		cancelTimer()
		if combined.stopParent != nil {
			combined.stopParent()
		}
		if combined.stopTimer != nil {
			combined.stopTimer()
		}
		combined.closeDone()
	}
}

func (c *matrixArtifactBudgetContext) closeDone() {
	c.closeOnce.Do(func() { close(c.done) })
}

func (c *matrixArtifactBudgetContext) Deadline() (time.Time, bool) {
	parentDeadline, parentOK := c.parent.Deadline()
	timerDeadline, timerOK := c.timer.Deadline()
	switch {
	case !parentOK:
		return timerDeadline, timerOK
	case !timerOK:
		return parentDeadline, true
	case parentDeadline.Before(timerDeadline):
		return parentDeadline, true
	default:
		return timerDeadline, true
	}
}

func (c *matrixArtifactBudgetContext) Done() <-chan struct{} { return c.done }

func (c *matrixArtifactBudgetContext) Err() error {
	if err := c.parent.Err(); err != nil {
		return err
	}
	return c.timer.Err()
}

func (c *matrixArtifactBudgetContext) Value(key any) any { return c.parent.Value(key) }

func (r *matrixContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if contextErr := r.ctx.Err(); contextErr != nil {
		return n, contextErr
	}
	return n, err
}

func mergeMatrixRawArtifacts(result *MatrixCellResult, attempt matrixAttemptResult) {
	if len(attempt.RawArtifacts) == 0 {
		return
	}
	if result.rawArtifacts == nil {
		result.rawArtifacts = make(map[string]matrixArtifactInfo, len(attempt.RawArtifacts))
	}
	for path, artifact := range attempt.RawArtifacts {
		result.rawArtifacts[path] = artifact
	}
}

func revalidateMatrixRawPaths(ctx context.Context, result *MatrixResult) error {
	return revalidateMatrixRawPathsWithBudget(ctx, result, matrixSubprocessTimeout)
}

func revalidateMatrixRawPathsWithBudget(ctx context.Context, result *MatrixResult, budget time.Duration) error {
	return revalidateMatrixRawPathsWithRoot(ctx, result, budget, nil)
}

func revalidateMatrixRawPathsWithRoot(ctx context.Context, result *MatrixResult, budget time.Duration, retainedRoot *rootfs.Root) error {
	if result == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	hasRawPaths := false
	for _, cell := range result.Cells {
		if len(cell.RawPaths) > 0 {
			hasRawPaths = true
			break
		}
	}
	if !hasRawPaths {
		return nil
	}
	rawRootPath := result.RawDir
	var rawRoot rootfs.Root
	var rootErr error
	if retainedRoot != nil {
		rawRoot = *retainedRoot
		rawRootPath = rawRoot.Path()
		rootErr = verifyMatrixRetainedOutputRoot(rawRoot)
	} else {
		rawRoot, rootErr = rootfs.New(result.RawDir)
		if rootErr == nil {
			defer rawRoot.Close()
		}
	}
	invalid := false
	var contextErr error
	for cellIndex := range result.Cells {
		cell := &result.Cells[cellIndex]
		validPaths := make([]string, 0, len(cell.RawPaths))
		validSet := make(map[string]struct{}, len(cell.RawPaths))
		for _, path := range cell.RawPaths {
			if contextErr == nil {
				contextErr = ctx.Err()
			}
			if contextErr != nil {
				invalid = true
				break
			}
			expected, known := cell.rawArtifacts[path]
			if rootErr != nil || !known {
				invalid = true
				continue
			}
			artifactCtx := ctx
			var cancelArtifact context.CancelFunc
			if budget > 0 {
				artifactCtx, cancelArtifact = newMatrixArtifactBudgetContext(ctx, budget)
			}
			if matrixRawVerificationContextForTest != nil {
				matrixRawVerificationContextForTest(artifactCtx)
			}
			if matrixRawVerificationBeforeArtifactForTest != nil {
				matrixRawVerificationBeforeArtifactForTest()
			}
			current, err := inspectMatrixArtifactWithContext(artifactCtx, rawRoot, rawRootPath, path)
			if cancelArtifact != nil {
				cancelArtifact()
			}
			if contextErr == nil {
				contextErr = ctx.Err()
			}
			if err != nil || contextErr != nil || !matrixArtifactMatches(expected, current) {
				invalid = true
				if contextErr != nil {
					break
				}
				continue
			}
			validPaths = append(validPaths, path)
			validSet[path] = struct{}{}
		}
		if len(validPaths) != len(cell.RawPaths) {
			cell.RawPaths = validPaths
			if contextErr != nil && cell.Status == MatrixCellSuccess {
				cell.Status = MatrixCellCanceled
				cell.FailureStage = "execution"
				cell.FailureCode = "canceled"
				cell.Error = newMatrixCellError(cell.FailureStage, cell.FailureCode, "cell canceled")
			} else if cell.Status == MatrixCellSuccess {
				cell.Status = MatrixCellFailed
				cell.FailureStage = "execution"
				cell.FailureCode = "raw_output_unavailable"
				cell.Error = newMatrixCellError(cell.FailureStage, cell.FailureCode, "raw screenshot became unavailable")
			}
			for screenshotIndex := range cell.Screenshots {
				path := cell.Screenshots[screenshotIndex].RawPath
				if path == "" {
					continue
				}
				if _, ok := validSet[path]; !ok {
					cell.Screenshots[screenshotIndex].RawPath = ""
				}
			}
			setMatrixScreenshotStatuses(cell)
		}
	}
	if retainedRoot != nil && rootErr == nil {
		if err := verifyMatrixRetainedOutputRoot(rawRoot); err != nil {
			invalid = true
			for cellIndex := range result.Cells {
				cell := &result.Cells[cellIndex]
				if len(cell.RawPaths) == 0 {
					continue
				}
				cell.RawPaths = nil
				for screenshotIndex := range cell.Screenshots {
					cell.Screenshots[screenshotIndex].RawPath = ""
				}
				if cell.Status == MatrixCellSuccess {
					cell.Status = MatrixCellFailed
					cell.FailureStage = "execution"
					cell.FailureCode = "raw_output_unavailable"
					cell.Error = newMatrixCellError(cell.FailureStage, cell.FailureCode, "raw screenshot became unavailable")
				}
				setMatrixScreenshotStatuses(cell)
			}
		}
	}
	if contextErr != nil {
		return contextErr
	}
	if invalid {
		return errors.New("one or more raw screenshots became unavailable")
	}
	return nil
}

func mergeMatrixFramedArtifacts(result *MatrixCellResult, attempt matrixAttemptResult) {
	if len(attempt.FramedArtifacts) == 0 {
		return
	}
	if result.framedArtifacts == nil {
		result.framedArtifacts = make(map[string]matrixArtifactInfo, len(attempt.FramedArtifacts))
	}
	for path, artifact := range attempt.FramedArtifacts {
		result.framedArtifacts[path] = artifact
	}
}

func revalidateMatrixFramedPaths(result *MatrixResult) error {
	return revalidateMatrixFramedPathsWithBudget(context.Background(), result, matrixSubprocessTimeout)
}

func revalidateMatrixFramedPathsWithBudget(ctx context.Context, result *MatrixResult, budget time.Duration) error {
	return revalidateMatrixFramedPathsWithRoot(ctx, result, budget, nil)
}

func revalidateMatrixFramedPathsWithRoot(ctx context.Context, result *MatrixResult, budget time.Duration, retainedRoot *rootfs.Root) error {
	if result == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	hasFramedPaths := false
	for _, cell := range result.Cells {
		if len(cell.FramedPaths) > 0 {
			hasFramedPaths = true
			break
		}
	}
	if !hasFramedPaths {
		return nil
	}
	framedRootPath := result.FramedDir
	var framedRoot rootfs.Root
	var rootErr error
	if retainedRoot != nil {
		framedRoot = *retainedRoot
		framedRootPath = framedRoot.Path()
		rootErr = verifyMatrixRetainedOutputRoot(framedRoot)
	} else {
		framedRoot, rootErr = rootfs.New(result.FramedDir)
		if rootErr == nil {
			defer framedRoot.Close()
		}
	}
	invalid := false
	var contextErr error
	for cellIndex := range result.Cells {
		cell := &result.Cells[cellIndex]
		validPaths := make([]string, 0, len(cell.FramedPaths))
		validSet := make(map[string]struct{}, len(cell.FramedPaths))
		for _, path := range cell.FramedPaths {
			if contextErr == nil {
				contextErr = ctx.Err()
			}
			if contextErr != nil {
				invalid = true
				break
			}
			expected, known := cell.framedArtifacts[path]
			if rootErr != nil || !known {
				invalid = true
				continue
			}
			artifactCtx := ctx
			var cancelArtifact context.CancelFunc
			if budget > 0 {
				artifactCtx, cancelArtifact = newMatrixArtifactBudgetContext(ctx, budget)
			}
			if matrixFramedVerificationContextForTest != nil {
				matrixFramedVerificationContextForTest(artifactCtx)
			}
			if matrixFramedVerificationBeforeArtifactForTest != nil {
				matrixFramedVerificationBeforeArtifactForTest()
			}
			current, err := inspectMatrixArtifactWithContext(artifactCtx, framedRoot, framedRootPath, path)
			if cancelArtifact != nil {
				cancelArtifact()
			}
			if contextErr == nil {
				contextErr = ctx.Err()
			}
			if err != nil || contextErr != nil || !matrixArtifactMatches(expected, current) {
				invalid = true
				if contextErr != nil {
					break
				}
				continue
			}
			validPaths = append(validPaths, path)
			validSet[path] = struct{}{}
		}
		if len(validPaths) != len(cell.FramedPaths) {
			cell.FramedPaths = validPaths
			if contextErr != nil && cell.Status == MatrixCellSuccess {
				cell.Status = MatrixCellCanceled
				cell.FailureStage = "execution"
				cell.FailureCode = "canceled"
				cell.Error = newMatrixCellError(cell.FailureStage, cell.FailureCode, "cell canceled")
			} else if cell.Status == MatrixCellSuccess {
				cell.Status = MatrixCellFailed
				cell.FailureStage = "framing"
				cell.FailureCode = "framed_output_unavailable"
				cell.Error = newMatrixCellError(cell.FailureStage, cell.FailureCode, "framed screenshot became unavailable")
			}
		}
		for screenshotIndex := range cell.Screenshots {
			path := cell.Screenshots[screenshotIndex].FramedPath
			if path == "" {
				continue
			}
			if _, ok := validSet[path]; !ok {
				cell.Screenshots[screenshotIndex].FramedPath = ""
			}
		}
		setMatrixScreenshotStatuses(cell)
		for screenshotIndex := range cell.Screenshots {
			if cell.Screenshots[screenshotIndex].FramedPath != "" {
				continue
			}
			// A requested framed artifact that was just invalidated is not a
			// successful screenshot merely because its raw counterpart remains
			// available. Keep the public per-screenshot projection aligned with
			// the cell's framing/cancellation failure.
			if contextErr != nil && cell.FailureStage == "execution" && cell.FailureCode == "canceled" {
				cell.Screenshots[screenshotIndex].Status = MatrixCellCanceled
			} else if cell.FailureStage == "framing" {
				cell.Screenshots[screenshotIndex].Status = MatrixCellFailed
			}
		}
	}
	if retainedRoot != nil && rootErr == nil {
		if err := verifyMatrixRetainedOutputRoot(framedRoot); err != nil {
			invalid = true
			for cellIndex := range result.Cells {
				cell := &result.Cells[cellIndex]
				if len(cell.FramedPaths) == 0 {
					continue
				}
				cell.FramedPaths = nil
				for screenshotIndex := range cell.Screenshots {
					cell.Screenshots[screenshotIndex].FramedPath = ""
				}
				if cell.Status == MatrixCellSuccess {
					cell.Status = MatrixCellFailed
					cell.FailureStage = "framing"
					cell.FailureCode = "framed_output_unavailable"
					cell.Error = newMatrixCellError(cell.FailureStage, cell.FailureCode, "framed screenshot became unavailable")
				}
				setMatrixScreenshotStatuses(cell)
				for screenshotIndex := range cell.Screenshots {
					if cell.FailureStage == "framing" {
						cell.Screenshots[screenshotIndex].Status = MatrixCellFailed
					}
				}
			}
		}
	}
	if contextErr != nil {
		return contextErr
	}
	if invalid {
		return errors.New("one or more framed screenshots became unavailable")
	}
	return nil
}

func verifyMatrixRetainedOutputRoot(root rootfs.Root) error {
	if root.Path() == "" {
		return errors.New("matrix output root is unavailable")
	}
	ok, err := root.ContainsPath(root.Path())
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("matrix output root changed during execution")
	}
	return nil
}

func matrixArtifactMatches(expected, current matrixArtifactInfo) bool {
	if expected.identity == nil || current.identity == nil {
		return false
	}
	return os.SameFile(expected.identity, current.identity) && expected.size == current.size && expected.digest == current.digest
}

func readMatrixImageDimensions(file *os.File, path string) (asc.ImageDimensions, error) {
	if file == nil {
		return asc.ImageDimensions{}, errors.New("image file is required")
	}
	info, err := file.Stat()
	if err != nil {
		return asc.ImageDimensions{}, err
	}
	if !info.Mode().IsRegular() {
		return asc.ImageDimensions{}, fmt.Errorf("expected regular image file %q", path)
	}
	if info.Size() <= 0 {
		return asc.ImageDimensions{}, fmt.Errorf("image file %q is empty", path)
	}
	if info.Size() > maxMatrixArtifactBytes {
		return asc.ImageDimensions{}, fmt.Errorf("image file %q exceeds the size limit", path)
	}
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return asc.ImageDimensions{}, fmt.Errorf("decode image dimensions: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return asc.ImageDimensions{}, fmt.Errorf("image %q has invalid dimensions", path)
	}
	return asc.ImageDimensions{Width: config.Width, Height: config.Height}, nil
}

func finishMatrixCellFailure(result MatrixCellResult, started time.Time, stage, code, message string) MatrixCellResult {
	result.Status = MatrixCellFailed
	if code == "canceled" {
		result.Status = MatrixCellCanceled
	}
	result.FailureStage = stage
	result.FailureCode = code
	result.Error = newMatrixCellError(stage, code, message)
	setMatrixScreenshotStatuses(&result)
	result.DurationMS = time.Since(started).Milliseconds()
	return result
}

func markMatrixCellsCanceled(result *MatrixResult) {
	if result == nil {
		return
	}
	for i := range result.Cells {
		if result.Cells[i].Status == MatrixCellSuccess {
			continue
		}
		result.Cells[i].Status = MatrixCellCanceled
		result.Cells[i].FailureStage = "execution"
		result.Cells[i].FailureCode = "canceled"
		result.Cells[i].Error = newMatrixCellError("execution", "canceled", "cell canceled")
		setMatrixScreenshotStatuses(&result.Cells[i])
	}
}

func markMatrixOutputFailure(result *MatrixResult) {
	if result == nil {
		return
	}
	for i := range result.Cells {
		// Preserve a more specific preflight or cancellation result already known
		// for this cell; output-root setup is an additional run-level failure.
		if result.Cells[i].FailureCode != "" {
			continue
		}
		result.Cells[i].Status = MatrixCellFailed
		result.Cells[i].FailureStage = "execution"
		result.Cells[i].FailureCode = "output_root_failed"
		result.Cells[i].Error = newMatrixCellError("execution", "output_root_failed", "matrix output root could not be prepared")
		setMatrixScreenshotStatuses(&result.Cells[i])
	}
}

func setMatrixScreenshotStatuses(result *MatrixCellResult) {
	for i := range result.Screenshots {
		switch result.Status {
		case MatrixCellSuccess:
			result.Screenshots[i].Status = MatrixCellSuccess
		case MatrixCellCanceled:
			if result.Screenshots[i].RawPath == "" || matrixScreenshotMissingRequestedFrame(result, result.Screenshots[i]) {
				result.Screenshots[i].Status = MatrixCellCanceled
			} else {
				result.Screenshots[i].Status = MatrixCellSuccess
			}
		default:
			if result.Screenshots[i].RawPath == "" || matrixScreenshotMissingRequestedFrame(result, result.Screenshots[i]) {
				result.Screenshots[i].Status = MatrixCellFailed
			} else {
				result.Screenshots[i].Status = MatrixCellSuccess
			}
		}
	}
}

func matrixScreenshotMissingRequestedFrame(result *MatrixCellResult, screenshot MatrixScreenshotResult) bool {
	return result != nil && result.FailureStage == "framing" && screenshot.FramedPath == ""
}

func mergeMatrixAttemptResult(result *MatrixCellResult, cell MatrixCell, attempt matrixAttemptResult) {
	if result == nil {
		return
	}
	result.RawPaths = mergeMatrixPaths(result.RawPaths, attempt.RawPaths, cell.RawPaths)
	result.FramedPaths = mergeMatrixPaths(result.FramedPaths, attempt.FramedPaths, cell.FramedPaths)
	mergeMatrixRawArtifacts(result, attempt)
	mergeMatrixFramedArtifacts(result, attempt)
	mergeMatrixScreenshots(result, cell, attempt.Screenshots)
}

func mergeMatrixPaths(existing, incoming, canonical []string) []string {
	if len(existing) == 0 && len(incoming) == 0 {
		return nil
	}
	merged := make([]string, 0, len(existing)+len(incoming))
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	appendUnique := func(paths []string) {
		for _, path := range paths {
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			merged = append(merged, path)
		}
	}
	appendUnique(existing)
	appendUnique(incoming)
	if len(canonical) == 0 {
		return merged
	}
	ordered := make([]string, 0, len(merged))
	for _, path := range canonical {
		if _, ok := seen[path]; !ok {
			continue
		}
		ordered = append(ordered, path)
		delete(seen, path)
	}
	for _, path := range merged {
		if _, ok := seen[path]; !ok {
			continue
		}
		ordered = append(ordered, path)
		delete(seen, path)
	}
	return ordered
}

func mergeMatrixScreenshots(result *MatrixCellResult, cell MatrixCell, incoming []MatrixScreenshotResult) {
	if len(result.Screenshots) == 0 && len(cell.RawPaths) == 0 && len(incoming) == 0 {
		return
	}
	existing := make(map[string]MatrixScreenshotResult, len(result.Screenshots))
	for _, screenshot := range result.Screenshots {
		key := strings.ToLower(strings.TrimSpace(screenshot.Name))
		if key == "" {
			continue
		}
		existing[key] = screenshot
	}
	ordered := make([]MatrixScreenshotResult, 0, len(cell.RawPaths)+len(incoming))
	seen := make(map[string]struct{}, len(cell.RawPaths)+len(incoming))
	for _, rawPath := range cell.RawPaths {
		name := strings.TrimSuffix(filepath.Base(rawPath), filepath.Ext(rawPath))
		key := strings.ToLower(strings.TrimSpace(name))
		screenshot, ok := existing[key]
		if !ok {
			screenshot = MatrixScreenshotResult{Name: name, Status: MatrixCellCanceled}
		}
		ordered = append(ordered, screenshot)
		seen[key] = struct{}{}
	}
	for _, screenshot := range result.Screenshots {
		key := strings.ToLower(strings.TrimSpace(screenshot.Name))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		ordered = append(ordered, screenshot)
		seen[key] = struct{}{}
	}
	for _, screenshot := range incoming {
		key := strings.ToLower(strings.TrimSpace(screenshot.Name))
		if key == "" {
			continue
		}
		index := -1
		for candidate := range ordered {
			if strings.EqualFold(strings.TrimSpace(ordered[candidate].Name), strings.TrimSpace(screenshot.Name)) {
				index = candidate
				break
			}
		}
		if index < 0 {
			ordered = append(ordered, screenshot)
			continue
		}
		current := &ordered[index]
		if screenshot.Name != "" {
			current.Name = screenshot.Name
		}
		if screenshot.Status != "" {
			current.Status = screenshot.Status
		}
		if screenshot.RawPath != "" {
			current.RawPath = screenshot.RawPath
		}
		if screenshot.FramedPath != "" {
			current.FramedPath = screenshot.FramedPath
		}
		if screenshot.Width > 0 {
			current.Width = screenshot.Width
		}
		if screenshot.Height > 0 {
			current.Height = screenshot.Height
		}
	}
	result.Screenshots = ordered
}

func newMatrixCellResult(cell MatrixCell) MatrixCellResult {
	result := MatrixCellResult{
		ID: cell.ID, Device: cell.Device, Locale: cell.Locale, Appearance: cell.Appearance,
		Content: cell.Content, Status: MatrixCellCanceled,
		Screenshots: make([]MatrixScreenshotResult, 0, len(cell.RawPaths)),
	}
	for _, rawPath := range cell.RawPaths {
		name := strings.TrimSuffix(filepath.Base(rawPath), filepath.Ext(rawPath))
		result.Screenshots = append(result.Screenshots, MatrixScreenshotResult{Name: name, Status: MatrixCellCanceled})
	}
	return result
}

func newMatrixCellError(stage, code, message string) *MatrixCellError {
	return &MatrixCellError{Stage: stage, Code: code, Message: message}
}

func countMatrixResultStatuses(result *MatrixResult) {
	result.Succeeded, result.Failed, result.Canceled, result.CleanupFailed = 0, 0, 0, 0
	result.Retried = 0
	for _, cell := range result.Cells {
		if cell.Attempts > 1 {
			result.Retried += cell.Attempts - 1
		}
		switch cell.Status {
		case MatrixCellSuccess:
			result.Succeeded++
		case MatrixCellCanceled:
			result.Canceled++
		case MatrixCellCleanupFailed:
			result.CleanupFailed++
			result.Failed++
		default:
			result.Failed++
		}
	}
	result.Status = MatrixCellSuccess
	if result.Failed > 0 || result.Canceled > 0 {
		result.Status = MatrixCellFailed
	}
	result.TotalCells = result.Total
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

func frameDeviceFamily(device FrameDevice) string {
	if device == FrameDeviceMac {
		return "mac"
	}
	return "iphone"
}

type matrixSimulatorDevice struct {
	UDID                 string `json:"udid"`
	State                string `json:"state"`
	IsAvailable          bool   `json:"isAvailable"`
	Name                 string `json:"name"`
	DeviceTypeIdentifier string `json:"deviceTypeIdentifier"`
}

func readMatrixSimulatorInventory(ctx context.Context) ([]matrixSimulatorDevice, error) {
	return readMatrixSimulatorInventoryWithTimeout(ctx, matrixSubprocessTimeout)
}

func readMatrixSimulatorInventoryWithTimeout(ctx context.Context, timeout time.Duration) ([]matrixSimulatorDevice, error) {
	inventoryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(inventoryCtx, "xcrun", "simctl", "list", "devices", "--json")
	var output cappedMatrixBuffer
	output.limit = maxMatrixInventoryBytes
	command.Stdout = &output
	command.Stderr = io.Discard
	err := command.Run()
	if parentErr := ctx.Err(); parentErr != nil {
		return nil, parentErr
	}
	if errors.Is(inventoryCtx.Err(), context.DeadlineExceeded) {
		return nil, ErrMatrixInventoryTimeout
	}
	if output.exceeded {
		return nil, errors.New("simulator inventory exceeded the output size limit")
	}
	if err != nil {
		return nil, errors.New("simulator inventory could not be read")
	}
	out := output.Bytes()
	var inventory struct {
		Devices map[string][]matrixSimulatorDevice `json:"devices"`
	}
	if err := json.Unmarshal(out, &inventory); err != nil {
		return nil, errors.New("simulator inventory was invalid")
	}
	devices := make([]matrixSimulatorDevice, 0)
	for _, runtimeDevices := range inventory.Devices {
		devices = append(devices, runtimeDevices...)
	}
	return devices, nil
}

func checkMatrixDevice(ctx context.Context, device MatrixDevice) error {
	devices, err := readMatrixSimulatorInventory(ctx)
	if err != nil {
		return err
	}
	wanted := normalizeMatrixUDID(device.UDID)
	for _, candidate := range devices {
		if normalizeMatrixUDID(candidate.UDID) != wanted {
			continue
		}
		return validateMatrixSimulatorDevice(candidate)
	}
	return errors.New("simulator was not found")
}

const (
	matrixPreflightSimulatorNotReady = "simulator_not_ready"
	matrixPreflightFrameMismatch     = "frame_family_mismatch"
	matrixPreflightFamilyUnknown     = "simulator_family_unknown"
	matrixPreflightMappingInvalid    = "frame_mapping_invalid"
)

type matrixDeviceFailure struct {
	Code    string
	Message string
}

type matrixFrameMappingError struct {
	device  string
	code    string
	message string
	cause   error
}

func (e *matrixFrameMappingError) Error() string {
	if e == nil {
		return "matrix frame mapping failed"
	}
	return fmt.Sprintf("device %q %s", e.device, e.message)
}

func (e *matrixFrameMappingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func checkMatrixDevices(ctx context.Context, plan *MatrixPlan) (map[string]matrixDeviceFailure, error) {
	failures := make(map[string]matrixDeviceFailure)
	devices, inventoryErr := readMatrixSimulatorInventory(ctx)
	if inventoryErr != nil && isMatrixContextTermination(inventoryErr) {
		return nil, inventoryErr
	}
	for _, device := range plan.Devices {
		if inventoryErr != nil {
			failures[device.ID] = matrixDeviceFailure{
				Code:    matrixPreflightSimulatorNotReady,
				Message: "target simulator is not ready",
			}
			continue
		}
		candidate, found := findMatrixSimulatorDevice(devices, device.UDID)
		if !found || validateMatrixSimulatorDevice(candidate) != nil {
			failures[device.ID] = matrixDeviceFailure{
				Code:    matrixPreflightSimulatorNotReady,
				Message: "target simulator is not ready",
			}
			continue
		}
		if plan.Output.Frame.Enabled {
			frame, _ := matrixFrameMappingForDevice(plan.Output.Frame.DeviceByMatrixDevice, device.ID)
			if err := validateMatrixFrameMappingForSimulator(device.ID, frame, candidate); err != nil {
				failure := matrixDeviceFailure{
					Code:    matrixPreflightFrameMismatch,
					Message: "configured frame does not match simulator family",
				}
				var mappingErr *matrixFrameMappingError
				if errors.As(err, &mappingErr) {
					failure.Code = mappingErr.code
					failure.Message = mappingErr.message
				}
				failures[device.ID] = failure
			}
		}
	}
	return failures, nil
}

func isMatrixContextTermination(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func findMatrixSimulatorDevice(devices []matrixSimulatorDevice, udid string) (matrixSimulatorDevice, bool) {
	wanted := normalizeMatrixUDID(udid)
	for _, device := range devices {
		if normalizeMatrixUDID(device.UDID) == wanted {
			return device, true
		}
	}
	return matrixSimulatorDevice{}, false
}

func validateMatrixSimulatorDevice(device matrixSimulatorDevice) error {
	if !device.IsAvailable {
		return errors.New("simulator is unavailable")
	}
	if !strings.EqualFold(strings.TrimSpace(device.State), "booted") {
		return errors.New("simulator is not booted")
	}
	return nil
}

func validateMatrixFrameMappingForSimulator(matrixDevice, frame string, simulator matrixSimulatorDevice) error {
	parsed, err := ParseFrameDevice(frame)
	if err != nil {
		return &matrixFrameMappingError{
			device:  matrixDevice,
			code:    matrixPreflightMappingInvalid,
			message: "configured frame mapping is invalid",
			cause:   err,
		}
	}
	actualFamily := matrixSimulatorFamily(simulator)
	if actualFamily == "unknown" {
		return &matrixFrameMappingError{
			device:  matrixDevice,
			code:    matrixPreflightFamilyUnknown,
			message: "simulator family could not be identified",
		}
	}
	if actualFamily == "ipad" {
		return &matrixFrameMappingError{
			device:  matrixDevice,
			code:    matrixPreflightFrameMismatch,
			message: "configured frame does not match simulator family",
		}
	}
	if actualFamily != frameDeviceFamily(parsed) {
		return &matrixFrameMappingError{
			device:  matrixDevice,
			code:    matrixPreflightFrameMismatch,
			message: "configured frame does not match simulator family",
		}
	}
	return nil
}

func matrixSimulatorFamily(device matrixSimulatorDevice) string {
	return matrixDeviceFamily(device.Name + " " + device.DeviceTypeIdentifier)
}

func normalizeMatrixUDID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeMatrixDeviceID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func matrixFrameMappingForDevice(mapping map[string]string, deviceID string) (string, bool) {
	wanted := normalizeMatrixDeviceID(deviceID)
	if wanted == "" {
		return "", false
	}
	var value string
	found := false
	for candidate, frame := range mapping {
		if normalizeMatrixDeviceID(candidate) != wanted {
			continue
		}
		if found {
			return "", false
		}
		found = true
		value = frame
	}
	return value, found
}

type cappedMatrixBuffer struct {
	data     []byte
	limit    int
	exceeded bool
}

func (b *cappedMatrixBuffer) Write(data []byte) (int, error) {
	if b.limit < 0 {
		return 0, errors.New("output limit must not be negative")
	}
	if b.exceeded {
		return len(data), nil
	}
	remaining := b.limit - len(b.data)
	if remaining <= 0 {
		b.exceeded = len(data) > 0
		return len(data), nil
	}
	if len(data) > remaining {
		b.data = append(b.data, data[:remaining]...)
		b.exceeded = true
		return len(data), nil
	}
	b.data = append(b.data, data...)
	return len(data), nil
}

func (b *cappedMatrixBuffer) Bytes() []byte { return b.data }

type simctlMatrixAppearance struct{}

func (simctlMatrixAppearance) Snapshot(ctx context.Context, udid string) (string, error) {
	out, err := runMatrixAppearanceOutput(ctx, "xcrun", "simctl", "ui", udid, "appearance")
	if err != nil {
		return "", err
	}
	switch strings.ToLower(strings.TrimSpace(out)) {
	case "dark":
		return "dark", nil
	case "light":
		return "light", nil
	default:
		return "", errors.New("simulator appearance state was invalid")
	}
}

func (simctlMatrixAppearance) Set(ctx context.Context, udid, appearance string) error {
	appearance = strings.ToLower(strings.TrimSpace(appearance))
	if appearance != "light" && appearance != "dark" {
		return errors.New("simulator appearance must be light or dark")
	}
	return runMatrixAppearance(ctx, "xcrun", "simctl", "ui", udid, "appearance", appearance)
}

func (simctlMatrixAppearance) Restore(ctx context.Context, udid, state string) error {
	state = strings.ToLower(strings.TrimSpace(state))
	if state != "light" && state != "dark" {
		return errors.New("simulator appearance state must be light or dark")
	}
	return runMatrixAppearance(ctx, "xcrun", "simctl", "ui", udid, "appearance", state)
}

func runMatrixAppearance(ctx context.Context, name string, args ...string) error {
	_, err := runMatrixAppearanceOutput(ctx, name, args...)
	return err
}

func runMatrixAppearanceOutput(ctx context.Context, name string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	stdout := &cappedMatrixBuffer{limit: maxMatrixAppearanceBytes}
	stderr := &cappedMatrixBuffer{limit: maxMatrixAppearanceBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	err := command.Run()
	if stdout.exceeded || stderr.exceeded {
		return "", errors.New("simulator appearance command exceeded the output size limit")
	}
	if err != nil {
		return "", fmt.Errorf("%s failed", name)
	}
	return string(stdout.Bytes()), nil
}

func executeMatrixCell(ctx context.Context, cell MatrixCell, base *Plan, matrixPlan *MatrixPlan, maxAttempts int, backoff time.Duration, deps MatrixDependencies, outputRoots matrixOutputRoots, guard *matrixSimulatorGuard) MatrixCellResult {
	started := time.Now()
	result := newMatrixCellResult(cell)
	result.Status = MatrixCellFailed
	release, lockErr := acquireMatrixSimulatorLock(ctx, cell.UDID)
	if lockErr != nil {
		if isMatrixContextTermination(lockErr) {
			return finishMatrixCellFailure(result, started, "execution", "canceled", "cell canceled")
		}
		result.FailureStage = "appearance"
		result.FailureCode = "simulator_lock_failed"
		result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "simulator lock could not be acquired")
		setMatrixScreenshotStatuses(&result)
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}
	result = executeMatrixCellWithSimulatorLock(ctx, cell, base, matrixPlan, maxAttempts, backoff, deps, outputRoots, guard)
	if releaseErr := release(); releaseErr != nil {
		result.Status = MatrixCellCleanupFailed
		result.FailureStage = "cleanup"
		result.FailureCode = "simulator_lock_release_failed"
		result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "simulator lock could not be released")
		setMatrixScreenshotStatuses(&result)
	}
	return result
}
