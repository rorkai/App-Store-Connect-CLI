package asc

// ProfileDownloadSkippedItemResult identifies a profile omitted from a bulk download.
type ProfileDownloadSkippedItemResult struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}
