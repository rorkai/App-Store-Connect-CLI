package screenshots

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
