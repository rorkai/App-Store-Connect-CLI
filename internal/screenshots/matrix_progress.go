package screenshots

import "fmt"

// ValidateMatrixProgressPath keeps the progress stream separate from files
// published by the matrix, including aliases through an existing parent.
func ValidateMatrixProgressPath(path string, outputPaths []string) error {
	for _, outputPath := range outputPaths {
		if sameMatrixPath(path, outputPath) {
			return fmt.Errorf("path conflicts with matrix output %q; choose a different progress file", outputPath)
		}
	}
	return nil
}

// MatrixProgressEvent identifies native matrix execution boundaries without
// provider identifiers, launch arguments, paths, or subprocess output.
type MatrixProgressEvent struct {
	Event        string `json:"event"`
	CellID       string `json:"cellId,omitempty"`
	Attempt      int    `json:"attempt,omitempty"`
	Status       string `json:"status,omitempty"`
	FailureStage string `json:"failureStage,omitempty"`
	FailureCode  string `json:"failureCode,omitempty"`
	TotalCells   int    `json:"totalCells,omitempty"`
}

func (d MatrixDependencies) emitProgress(event MatrixProgressEvent) {
	if d.progress != nil {
		d.progress(event)
	}
}

func (d MatrixDependencies) finishAttemptProgress(cell MatrixCellResult) {
	d.emitProgress(MatrixProgressEvent{Event: "attempt_finished", CellID: cell.ID, Attempt: cell.Attempts, Status: cell.Status, FailureStage: cell.FailureStage, FailureCode: cell.FailureCode})
}
