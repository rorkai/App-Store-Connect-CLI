package asc

// Receipt actions reported by create commands that support --if-exists.
const (
	IdempotentWriteActionCreated = "created"
	IdempotentWriteActionSkipped = "skipped"
	IdempotentWriteActionUpdated = "updated"
)

// IdempotentWriteReceipt carries the additive --if-exists fields on mutation
// receipts. Commands leave it empty in fail mode so the default and explicit
// fail paths preserve their historical output.
type IdempotentWriteReceipt struct {
	AlreadyExists bool   `json:"alreadyExists,omitempty"`
	Action        string `json:"action,omitempty"`
}
