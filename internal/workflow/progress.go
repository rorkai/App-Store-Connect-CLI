package workflow

// ProgressEvent identifies native workflow execution boundaries without command,
// output, environment, or step-label content.
type ProgressEvent struct {
	Event         string `json:"event"`
	RunID         string `json:"runId,omitempty"`
	StepID        string `json:"stepId,omitempty"`
	Status        string `json:"status,omitempty"`
	Invocation    int    `json:"invocation,omitempty"`
	Attempt       int    `json:"attempt,omitempty"`
	FailureReason string `json:"failureReason,omitempty"`
	DryRun        bool   `json:"dryRun,omitempty"`
}

func (r *runner) emitProgress(event ProgressEvent) {
	if r.opts.Progress == nil {
		return
	}
	event.RunID = r.result.RunID
	event.DryRun = r.opts.DryRun
	r.opts.Progress(event)
}

func (r *runner) recordStep(stepKey string, step StepResult) {
	r.result.Steps = append(r.result.Steps, step)
	r.emitProgress(ProgressEvent{Event: "step_finished", StepID: stepKey, Status: step.Status, FailureReason: step.FailureReason})
}
