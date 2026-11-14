package asc

// BetaFeedbackCrashSubmissionResponse is the response from crash submission detail endpoint.
type BetaFeedbackCrashSubmissionResponse = SingleResponse[CrashAttributes]

// BetaFeedbackScreenshotSubmissionResponse is the response from screenshot submission detail endpoint.
type BetaFeedbackScreenshotSubmissionResponse = SingleResponse[FeedbackAttributes]
