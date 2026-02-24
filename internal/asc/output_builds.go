package asc

import (
	"fmt"
	"strings"
)

// BuildUploadResult represents CLI output for build upload operations.
type BuildUploadResult struct {
	UploadID            string            `json:"uploadId"`
	FileID              string            `json:"fileId"`
	FileName            string            `json:"fileName"`
	FileSize            int64             `json:"fileSize"`
	Operations          []UploadOperation `json:"operations,omitempty"`
	Uploaded            *bool             `json:"uploaded,omitempty"`
	ChecksumVerified    *bool             `json:"checksumVerified,omitempty"`
	SourceFileChecksums *Checksums        `json:"sourceFileChecksums,omitempty"`
}

// BuildBetaGroupsUpdateResult represents CLI output for build beta group updates.
type BuildBetaGroupsUpdateResult struct {
	BuildID  string   `json:"buildId"`
	GroupIDs []string `json:"groupIds"`
	Action   string   `json:"action"`
}

// BuildIndividualTestersUpdateResult represents CLI output for build individual tester updates.
type BuildIndividualTestersUpdateResult struct {
	BuildID   string   `json:"buildId"`
	TesterIDs []string `json:"testerIds"`
	Action    string   `json:"action"`
}

// BuildUploadDeleteResult represents CLI output for build upload deletions.
type BuildUploadDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// BuildsLatestNextResult represents CLI output for next build number selection.
type BuildsLatestNextResult struct {
	LatestProcessedBuildNumber *string  `json:"latestProcessedBuildNumber"`
	LatestUploadBuildNumber    *string  `json:"latestUploadBuildNumber"`
	LatestObservedBuildNumber  *string  `json:"latestObservedBuildNumber"`
	NextBuildNumber            string   `json:"nextBuildNumber"`
	SourcesConsidered          []string `json:"sourcesConsidered"`
}

// BuildExpireAllItem represents a build selected for expiration.
type BuildExpireAllItem struct {
	ID           string `json:"id"`
	Version      string `json:"version"`
	UploadedDate string `json:"uploadedDate"`
	AgeDays      int    `json:"ageDays"`
	Expired      *bool  `json:"expired,omitempty"`
}

// BuildExpireAllFailure represents a failed expiration attempt.
type BuildExpireAllFailure struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// BuildExpireAllResult represents CLI output for batch build expiration.
type BuildExpireAllResult struct {
	DryRun              bool                    `json:"dryRun"`
	AppID               string                  `json:"appId"`
	OlderThan           *string                 `json:"olderThan,omitempty"`
	KeepLatest          *int                    `json:"keepLatest,omitempty"`
	SelectedCount       int                     `json:"selectedCount"`
	ExpiredCount        int                     `json:"expiredCount"`
	SkippedExpiredCount *int                    `json:"skippedExpiredCount,omitempty"`
	SkippedInvalidCount *int                    `json:"skippedInvalidCount,omitempty"`
	Builds              []BuildExpireAllItem    `json:"builds"`
	Failures            []BuildExpireAllFailure `json:"failures,omitempty"`
}

// formatEncryptionStatus formats the UsesNonExemptEncryption field for display.
// Returns "required" if true (needs encryption declaration), "exempt" if false,
// or "n/a" if null (no information available).
func formatEncryptionStatus(usesNonExempt *bool) string {
	if usesNonExempt == nil {
		return "n/a"
	}
	if *usesNonExempt {
		return "required"
	}
	return "exempt"
}

func buildsRows(resp *BuildsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "Uploaded", "Processing", "Expired", "Encryption"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Version,
			item.Attributes.UploadedDate,
			item.Attributes.ProcessingState,
			fmt.Sprintf("%t", item.Attributes.Expired),
			formatEncryptionStatus(item.Attributes.UsesNonExemptEncryption),
		})
	}
	return headers, rows
}

func buildIconAssetURL(attr BuildIconAttributes) string {
	if attr.IconAsset == nil {
		return ""
	}
	return attr.IconAsset.TemplateURL
}

func buildIconsRows(resp *BuildIconsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Type", "Masked", "Asset URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			string(item.Attributes.IconType),
			fmt.Sprintf("%t", item.Attributes.Masked),
			sanitizeTerminal(buildIconAssetURL(item.Attributes)),
		})
	}
	return headers, rows
}

func buildUploadState(attr BuildUploadAttributes) string {
	if attr.State == nil || attr.State.State == nil {
		return ""
	}
	return *attr.State.State
}

func buildUploadTimestamp(attr BuildUploadAttributes) string {
	if attr.UploadedDate != nil {
		return *attr.UploadedDate
	}
	if attr.CreatedDate != nil {
		return *attr.CreatedDate
	}
	return ""
}

func buildUploadsRows(resp *BuildUploadsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "Build", "Platform", "State", "Uploaded"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.CFBundleShortVersionString,
			item.Attributes.CFBundleVersion,
			string(item.Attributes.Platform),
			buildUploadState(item.Attributes),
			buildUploadTimestamp(item.Attributes),
		})
	}
	return headers, rows
}

func buildUploadFileState(attr BuildUploadFileAttributes) string {
	if attr.AssetDeliveryState == nil || attr.AssetDeliveryState.State == nil {
		return ""
	}
	return *attr.AssetDeliveryState.State
}

func buildUploadFilesRows(resp *BuildUploadFilesResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "Asset Type", "State", "Uploaded"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		uploaded := ""
		if item.Attributes.Uploaded != nil {
			uploaded = fmt.Sprintf("%t", *item.Attributes.Uploaded)
		}
		rows = append(rows, []string{
			item.ID,
			item.Attributes.FileName,
			fmt.Sprintf("%d", item.Attributes.FileSize),
			string(item.Attributes.AssetType),
			buildUploadFileState(item.Attributes),
			uploaded,
		})
	}
	return headers, rows
}

func buildUploadOperationsRows(operations []UploadOperation) ([]string, [][]string) {
	headers := []string{"Method", "URL", "Length", "Offset"}
	rows := make([][]string, 0, len(operations))
	for _, op := range operations {
		rows = append(rows, []string{
			op.Method,
			op.URL,
			fmt.Sprintf("%d", op.Length),
			fmt.Sprintf("%d", op.Offset),
		})
	}
	return headers, rows
}

func buildUploadResultRows(result *BuildUploadResult) ([]string, [][]string) {
	headers := []string{"Upload ID", "File ID", "File Name", "File Size"}
	values := []string{
		result.UploadID,
		result.FileID,
		result.FileName,
		fmt.Sprintf("%d", result.FileSize),
	}
	if result.Uploaded != nil {
		headers = append(headers, "Uploaded")
		values = append(values, fmt.Sprintf("%t", *result.Uploaded))
	}
	if result.ChecksumVerified != nil {
		headers = append(headers, "Checksum Verified")
		values = append(values, fmt.Sprintf("%t", *result.ChecksumVerified))
	}
	return headers, [][]string{values}
}

func buildExpireAllResultRows(result *BuildExpireAllResult) ([]string, [][]string) {
	status := "expired"
	if result.DryRun {
		status = "would-expire"
	}
	headers := []string{"ID", "Version", "Uploaded", "Age Days", "Status"}
	rows := make([][]string, 0, len(result.Builds))
	for _, item := range result.Builds {
		rows = append(rows, []string{
			item.ID,
			item.Version,
			item.UploadedDate,
			fmt.Sprintf("%d", item.AgeDays),
			status,
		})
	}
	return headers, rows
}

func buildBetaGroupsUpdateRows(result *BuildBetaGroupsUpdateResult) ([]string, [][]string) {
	headers := []string{"Build ID", "Group IDs", "Action"}
	rows := [][]string{{result.BuildID, strings.Join(result.GroupIDs, ", "), result.Action}}
	return headers, rows
}

func buildIndividualTestersUpdateRows(result *BuildIndividualTestersUpdateResult) ([]string, [][]string) {
	headers := []string{"Build ID", "Tester IDs", "Action"}
	rows := [][]string{{result.BuildID, strings.Join(result.TesterIDs, ", "), result.Action}}
	return headers, rows
}

func buildUploadDeleteResultRows(result *BuildUploadDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func buildsLatestNextValue(value *string) string {
	if value == nil {
		return "n/a"
	}
	return *value
}

func buildsLatestNextSources(sources []string) string {
	if len(sources) == 0 {
		return "n/a"
	}
	return strings.Join(sources, ", ")
}

func buildsLatestNextRows(result *BuildsLatestNextResult) ([]string, [][]string) {
	headers := []string{"Latest Processed", "Latest Upload", "Latest Observed", "Next", "Sources"}
	rows := [][]string{{
		buildsLatestNextValue(result.LatestProcessedBuildNumber),
		buildsLatestNextValue(result.LatestUploadBuildNumber),
		buildsLatestNextValue(result.LatestObservedBuildNumber),
		result.NextBuildNumber,
		buildsLatestNextSources(result.SourcesConsidered),
	}}
	return headers, rows
}
