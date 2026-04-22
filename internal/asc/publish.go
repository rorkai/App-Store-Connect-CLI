package asc

// PublishMode identifies how a publish command sourced its build.
type PublishMode string

const (
	PublishModeExistingBuild PublishMode = "existing_build"
	PublishModeIPAUpload     PublishMode = "ipa_upload"
	PublishModeLocalBuild    PublishMode = "local_build"
)

// PublishArchiveStageResult captures local archive stage details for nested
// publish command output.
type PublishArchiveStageResult struct {
	ArchivePath   string `json:"archivePath"`
	BundleID      string `json:"bundleId,omitempty"`
	Version       string `json:"version,omitempty"`
	BuildNumber   string `json:"buildNumber,omitempty"`
	Scheme        string `json:"scheme,omitempty"`
	Configuration string `json:"configuration,omitempty"`
}

// PublishExportStageResult captures local export stage details for nested
// publish command output.
type PublishExportStageResult struct {
	ArchivePath       string `json:"archivePath"`
	IPAPath           string `json:"ipaPath,omitempty"`
	BundleID          string `json:"bundleId,omitempty"`
	Version           string `json:"version,omitempty"`
	BuildNumber       string `json:"buildNumber,omitempty"`
	ExportOptionsPath string `json:"exportOptionsPath,omitempty"`
	DirectUpload      bool   `json:"directUpload"`
}

// PublishPlanStep captures a high-level dry-run step for publish flows.
type PublishPlanStep struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// TestFlightPublishStageResult duplicates the publish summary inside
// local-build mode so agents can inspect stage-specific output.
type TestFlightPublishStageResult struct {
	BuildID                string                            `json:"buildId"`
	BuildVersion           string                            `json:"buildVersion,omitempty"`
	BuildNumber            string                            `json:"buildNumber,omitempty"`
	GroupIDs               []string                          `json:"groupIds,omitempty"`
	Uploaded               bool                              `json:"uploaded"`
	ProcessingState        string                            `json:"processingState,omitempty"`
	Notified               *bool                             `json:"notified,omitempty"`
	NotificationAction     BuildBetaGroupsNotificationAction `json:"notificationAction,omitempty"`
	BetaReviewSubmitted    *bool                             `json:"betaReviewSubmitted,omitempty"`
	BetaReviewSubmissionID string                            `json:"betaReviewSubmissionId,omitempty"`
}

// AppStorePublishStageResult duplicates the publish summary inside local-build
// mode so agents can inspect stage-specific output.
type AppStorePublishStageResult struct {
	BuildVersion string `json:"buildVersion,omitempty"`
	BuildNumber  string `json:"buildNumber,omitempty"`
	BuildID      string `json:"buildId"`
	VersionID    string `json:"versionId"`
	SubmissionID string `json:"submissionId,omitempty"`
	Uploaded     bool   `json:"uploaded"`
	Attached     bool   `json:"attached"`
	Submitted    bool   `json:"submitted"`
}

// Result types for the publish workflow.
type TestFlightPublishResult struct {
	Mode                   PublishMode                       `json:"mode,omitempty"`
	BuildID                string                            `json:"buildId"`
	BuildVersion           string                            `json:"buildVersion,omitempty"`
	BuildNumber            string                            `json:"buildNumber,omitempty"`
	GroupIDs               []string                          `json:"groupIds,omitempty"`
	Uploaded               bool                              `json:"uploaded"`
	ProcessingState        string                            `json:"processingState,omitempty"`
	Notified               *bool                             `json:"notified,omitempty"`
	NotificationAction     BuildBetaGroupsNotificationAction `json:"notificationAction,omitempty"`
	BetaReviewSubmitted    *bool                             `json:"betaReviewSubmitted,omitempty"`
	BetaReviewSubmissionID string                            `json:"betaReviewSubmissionId,omitempty"`
	Archive                *PublishArchiveStageResult        `json:"archive,omitempty"`
	Export                 *PublishExportStageResult         `json:"export,omitempty"`
	Publish                *TestFlightPublishStageResult     `json:"publish,omitempty"`
}

// AppStorePublishResult captures the App Store publish workflow output.
type AppStorePublishResult struct {
	Mode         PublishMode                 `json:"mode,omitempty"`
	DryRun       bool                        `json:"dryRun,omitempty"`
	BuildVersion string                      `json:"buildVersion,omitempty"`
	BuildNumber  string                      `json:"buildNumber,omitempty"`
	BuildID      string                      `json:"buildId"`
	VersionID    string                      `json:"versionId"`
	SubmissionID string                      `json:"submissionId,omitempty"`
	Uploaded     bool                        `json:"uploaded"`
	Attached     bool                        `json:"attached"`
	Submitted    bool                        `json:"submitted"`
	Plan         []PublishPlanStep           `json:"plan,omitempty"`
	Archive      *PublishArchiveStageResult  `json:"archive,omitempty"`
	Export       *PublishExportStageResult   `json:"export,omitempty"`
	Publish      *AppStorePublishStageResult `json:"publish,omitempty"`
}

// Build processing states to poll for.
const (
	BuildProcessingStateProcessing = "PROCESSING"
	BuildProcessingStateFailed     = "FAILED"
	BuildProcessingStateValid      = "VALID"
	BuildProcessingStateInvalid    = "INVALID"
)
