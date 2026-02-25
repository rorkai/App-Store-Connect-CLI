package asc

// Result types for the publish workflow.
type TestFlightPublishResult struct {
	BuildID         string   `json:"buildId"`
	BuildVersion    string   `json:"buildVersion,omitempty"`
	BuildNumber     string   `json:"buildNumber,omitempty"`
	GroupIDs        []string `json:"groupIds,omitempty"`
	Uploaded        bool     `json:"uploaded"`
	ProcessingState string   `json:"processingState,omitempty"`
	Notified        bool     `json:"notified,omitempty"`
}

// AppStorePublishResult captures the App Store publish workflow output.
type AppStorePublishResult struct {
	BuildID      string `json:"buildId"`
	VersionID    string `json:"versionId"`
	SubmissionID string `json:"submissionId,omitempty"`
	Uploaded     bool   `json:"uploaded"`
	Attached     bool   `json:"attached"`
	Submitted    bool   `json:"submitted"`
}

// Build processing states to poll for.
const (
	BuildProcessingStateProcessing = "PROCESSING"
	BuildProcessingStateFailed     = "FAILED"
	BuildProcessingStateValid      = "VALID"
	BuildProcessingStateInvalid    = "INVALID"
)
