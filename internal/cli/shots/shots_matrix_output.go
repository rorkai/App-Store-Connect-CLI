package shots

import (
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

func matrixResultOutput(result *screenshots.MatrixResult) *asc.MatrixResult {
	if result == nil {
		return nil
	}
	output := &asc.MatrixResult{
		PlanPath:      result.PlanPath,
		BundleID:      result.BundleID,
		RawDir:        result.RawDir,
		FramedDir:     result.FramedDir,
		ReviewDir:     result.ReviewDir,
		Status:        result.Status,
		TotalCells:    result.TotalCells,
		Succeeded:     result.Succeeded,
		Failed:        result.Failed,
		Canceled:      result.Canceled,
		Retried:       result.Retried,
		CleanupFailed: result.CleanupFailed,
		Cells:         make([]asc.MatrixCellResult, len(result.Cells)),
	}
	for i, cell := range result.Cells {
		output.Cells[i] = matrixCellOutput(cell)
	}
	if result.Review != nil {
		output.Review = &asc.MatrixReviewResult{
			ManifestPath: result.Review.ManifestPath,
			HTMLPath:     result.Review.HTMLPath,
			Total:        result.Review.Total,
			Succeeded:    result.Review.Succeeded,
			Failed:       result.Review.Failed,
			Canceled:     result.Review.Canceled,
		}
	}
	return output
}

func matrixCellOutput(cell screenshots.MatrixCellResult) asc.MatrixCellResult {
	output := asc.MatrixCellResult{
		ID:           cell.ID,
		Device:       cell.Device,
		Locale:       cell.Locale,
		Appearance:   cell.Appearance,
		Content:      cell.Content,
		Status:       cell.Status,
		Attempts:     cell.Attempts,
		DurationMS:   cell.DurationMS,
		RawPaths:     append([]string(nil), cell.RawPaths...),
		FramedPaths:  append([]string(nil), cell.FramedPaths...),
		Screenshots:  make([]asc.MatrixScreenshotResult, len(cell.Screenshots)),
		Steps:        make([]asc.MatrixStepResult, len(cell.Steps)),
		FailureStage: cell.FailureStage,
		FailureCode:  cell.FailureCode,
	}
	output.FailureEvidence = append([]asc.ScreenshotFailureEvidence(nil), cell.FailureEvidence...)
	for i, screenshot := range cell.Screenshots {
		output.Screenshots[i] = asc.MatrixScreenshotResult{
			Name: screenshot.Name, Status: screenshot.Status, RawPath: screenshot.RawPath,
			FramedPath: screenshot.FramedPath, Width: screenshot.Width, Height: screenshot.Height,
		}
	}
	for i, step := range cell.Steps {
		output.Steps[i] = asc.MatrixStepResult{
			Index: step.Index, Action: step.Action, Status: step.Status,
			DurationMS: step.DurationMS, Error: step.Error,
		}
	}
	if cell.Error != nil {
		output.Error = &asc.MatrixCellError{
			Stage: cell.Error.Stage, Code: cell.Error.Code, Message: cell.Error.Message,
		}
	}
	return output
}
