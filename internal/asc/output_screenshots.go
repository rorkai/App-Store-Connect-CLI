package asc

import "strconv"

// ScreenshotLocationResult acknowledges a simulated location setting.
type ScreenshotLocationResult struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Status    string  `json:"status"`
}

// MatrixResult is the stable output contract for a local screenshot matrix
// run. It deliberately contains logical device labels and artifact paths, but
// no simulator identifiers or launch arguments.
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
}

// MatrixCellResult is the privacy-safe output for one matrix cell.
type MatrixCellResult struct {
	ID           string                    `json:"id"`
	Device       string                    `json:"device"`
	Locale       string                    `json:"locale"`
	Appearance   string                    `json:"appearance"`
	Content      string                    `json:"contentVariant"`
	Status       string                    `json:"status"`
	Attempts     int                       `json:"attempts"`
	DurationMS   int64                     `json:"durationMs"`
	RawPaths     []string                  `json:"rawPaths,omitempty"`
	FramedPaths  []string                  `json:"framedPaths,omitempty"`
	Screenshots  []MatrixScreenshotResult  `json:"screenshots,omitempty"`
	Steps        []MatrixStepResult        `json:"steps,omitempty"`
	Location     *ScreenshotLocationResult `json:"location,omitempty"`
	FailureStage string                    `json:"failureStage,omitempty"`
	FailureCode  string                    `json:"failureCode,omitempty"`
	Error        *MatrixCellError          `json:"error,omitempty"`
}

// MatrixCellError is the sanitized, stable failure contract for one cell.
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

// MatrixStepResult describes one sanitized base-plan step in a cell review.
type MatrixStepResult struct {
	Index      int    `json:"index"`
	Action     string `json:"action"`
	Status     string `json:"status"`
	DurationMS int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

// MatrixReviewResult identifies the generated offline report files.
type MatrixReviewResult struct {
	ManifestPath string `json:"manifestPath"`
	HTMLPath     string `json:"htmlPath"`
	Total        int    `json:"total"`
	Succeeded    int    `json:"succeeded"`
	Failed       int    `json:"failed"`
	Canceled     int    `json:"canceled"`
}

// MatrixReviewManifest is the stable JSON artifact written beside the offline
// HTML review. Its field names intentionally match MatrixResult's output
// contract so consumers do not need a second naming convention.
type MatrixReviewManifest struct {
	GeneratedAt string `json:"generatedAt"`
	// HTMLSHA256 binds the manifest commit marker to the exact offline report
	// bytes so a torn or externally mixed publication is rejected.
	HTMLSHA256 string `json:"htmlSha256,omitempty"`
	PlanPath   string `json:"planPath"`
	BundleID   string `json:"bundleId"`
	RawDir     string `json:"rawDir"`
	FramedDir  string `json:"framedDir,omitempty"`
	OutputDir  string `json:"outputDir"`
	Status     string `json:"status"`
	TotalCells int    `json:"totalCells"`
	Succeeded  int    `json:"succeeded"`
	Failed     int    `json:"failed"`
	Canceled   int    `json:"canceled"`
	Retried    int    `json:"retried"`
	// CleanupFailed mirrors MatrixResult.CleanupFailed: the subset of Failed
	// whose cells completed capture but could not restore simulator state.
	CleanupFailed int                `json:"cleanupFailed,omitempty"`
	Cells         []MatrixCellResult `json:"cells"`
}

func matrixResultTables(result *MatrixResult, render func([]string, [][]string)) error {
	if result == nil {
		result = &MatrixResult{}
	}
	render([]string{"Field", "Value"}, [][]string{
		{"Plan", result.PlanPath},
		{"Status", result.Status},
		{"Total Cells", strconv.Itoa(result.TotalCells)},
		{"Succeeded", strconv.Itoa(result.Succeeded)},
		{"Failed", strconv.Itoa(result.Failed)},
		{"Cleanup Failed", strconv.Itoa(result.CleanupFailed)},
		{"Canceled", strconv.Itoa(result.Canceled)},
		{"Retried", strconv.Itoa(result.Retried)},
	})
	renderMatrixCellRows(result.Cells, render)
	if result.Review != nil {
		h, rows := matrixReviewResultRows(result.Review)
		render(h, rows)
	}
	return nil
}

func matrixCellResultRows(result *MatrixCellResult) ([]string, [][]string) {
	if result == nil {
		result = &MatrixCellResult{}
	}
	return []string{"CELL", "DEVICE", "LOCALE", "APPEARANCE", "CONTENT", "STATUS", "ATTEMPTS", "FAILURE"}, [][]string{{
		result.ID,
		result.Device,
		result.Locale,
		result.Appearance,
		result.Content,
		result.Status,
		strconv.Itoa(result.Attempts),
		matrixFailureDisplay(result),
	}}
}

func renderMatrixCellRows(cells []MatrixCellResult, render func([]string, [][]string)) {
	rows := make([][]string, 0, len(cells))
	for index := range cells {
		cell := &cells[index]
		rows = append(rows, []string{
			cell.ID,
			cell.Device,
			cell.Locale,
			cell.Appearance,
			cell.Content,
			cell.Status,
			strconv.Itoa(cell.Attempts),
			matrixFailureDisplay(cell),
		})
	}
	render([]string{"CELL", "DEVICE", "LOCALE", "APPEARANCE", "CONTENT", "STATUS", "ATTEMPTS", "FAILURE"}, rows)
}

func matrixReviewResultRows(result *MatrixReviewResult) ([]string, [][]string) {
	if result == nil {
		result = &MatrixReviewResult{}
	}
	return []string{"Manifest", "HTML", "Total", "Succeeded", "Failed", "Canceled"}, [][]string{{
		result.ManifestPath,
		result.HTMLPath,
		strconv.Itoa(result.Total),
		strconv.Itoa(result.Succeeded),
		strconv.Itoa(result.Failed),
		strconv.Itoa(result.Canceled),
	}}
}

func matrixReviewManifestTables(manifest *MatrixReviewManifest, render func([]string, [][]string)) error {
	if manifest == nil {
		manifest = &MatrixReviewManifest{}
	}
	render([]string{"Field", "Value"}, [][]string{
		{"Plan", manifest.PlanPath},
		{"Status", manifest.Status},
		{"Total Cells", strconv.Itoa(manifest.TotalCells)},
		{"Succeeded", strconv.Itoa(manifest.Succeeded)},
		{"Failed", strconv.Itoa(manifest.Failed)},
		{"Cleanup Failed", strconv.Itoa(manifest.CleanupFailed)},
		{"Canceled", strconv.Itoa(manifest.Canceled)},
		{"Retried", strconv.Itoa(manifest.Retried)},
	})
	renderMatrixCellRows(manifest.Cells, render)
	return nil
}

func matrixFailureDisplay(result *MatrixCellResult) string {
	if result == nil {
		return "-"
	}
	if result.FailureCode != "" {
		return result.FailureCode
	}
	if result.Error != nil && result.Error.Code != "" {
		return result.Error.Code
	}
	return "-"
}

// ScreenshotPlanValidationResult reports successful offline capture-plan checks.
type ScreenshotPlanValidationResult struct {
	PlanPath   string `json:"planPath"`
	Valid      bool   `json:"valid"`
	TotalSteps int    `json:"totalSteps"`
}

func screenshotPlanValidationRows(result *ScreenshotPlanValidationResult) ([]string, [][]string) {
	return []string{"Plan", "Valid", "Steps"}, [][]string{{compactWhitespace(result.PlanPath), strconv.FormatBool(result.Valid), strconv.Itoa(result.TotalSteps)}}
}
