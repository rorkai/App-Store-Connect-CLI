package asc

// Receipt actions reported by create commands that support --if-exists.
const (
	IdempotentWriteActionCreated = "created"
	IdempotentWriteActionSkipped = "skipped"
	IdempotentWriteActionUpdated = "updated"
)

// IdempotentWriteReceipt carries the additive --if-exists fields on mutation
// receipts. AlreadyExists is omitted on the plain create path so existing
// consumers only gain the action key.
type IdempotentWriteReceipt struct {
	AlreadyExists bool   `json:"alreadyExists,omitempty"`
	Action        string `json:"action,omitempty"`
}
