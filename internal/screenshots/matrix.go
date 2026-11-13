package screenshots

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"regexp"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
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
