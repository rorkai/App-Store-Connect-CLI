package screenshots

import (
	"path/filepath"
	"strings"
	"time"
)

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
