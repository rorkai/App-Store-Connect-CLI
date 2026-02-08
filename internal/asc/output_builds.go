package asc

import (
	"fmt"
	"os"
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

func printBuildsTable(resp *BuildsResponse) error {
	headers := []string{"Version", "Uploaded", "Processing", "Expired", "Encryption"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.Attributes.Version,
			item.Attributes.UploadedDate,
			item.Attributes.ProcessingState,
			fmt.Sprintf("%t", item.Attributes.Expired),
			formatEncryptionStatus(item.Attributes.UsesNonExemptEncryption),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printBuildsMarkdown(resp *BuildsResponse) error {
	fmt.Fprintln(os.Stdout, "| Version | Uploaded | Processing | Expired | Encryption |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %t | %s |\n",
			escapeMarkdown(item.Attributes.Version),
			escapeMarkdown(item.Attributes.UploadedDate),
			escapeMarkdown(item.Attributes.ProcessingState),
			item.Attributes.Expired,
			formatEncryptionStatus(item.Attributes.UsesNonExemptEncryption),
		)
	}
	return nil
}

func buildIconAssetURL(attr BuildIconAttributes) string {
	if attr.IconAsset == nil {
		return ""
	}
	return attr.IconAsset.TemplateURL
}

func printBuildIconsTable(resp *BuildIconsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printBuildIconsMarkdown(resp *BuildIconsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Type | Masked | Asset URL |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %t | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(string(item.Attributes.IconType)),
			item.Attributes.Masked,
			escapeMarkdown(buildIconAssetURL(item.Attributes)),
		)
	}
	return nil
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

func printBuildUploadsTable(resp *BuildUploadsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printBuildUploadsMarkdown(resp *BuildUploadsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Version | Build | Platform | State | Uploaded |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.CFBundleShortVersionString),
			escapeMarkdown(item.Attributes.CFBundleVersion),
			escapeMarkdown(string(item.Attributes.Platform)),
			escapeMarkdown(buildUploadState(item.Attributes)),
			escapeMarkdown(buildUploadTimestamp(item.Attributes)),
		)
	}
	return nil
}

func buildUploadFileState(attr BuildUploadFileAttributes) string {
	if attr.AssetDeliveryState == nil || attr.AssetDeliveryState.State == nil {
		return ""
	}
	return *attr.AssetDeliveryState.State
}

func printBuildUploadFilesTable(resp *BuildUploadFilesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printBuildUploadFilesMarkdown(resp *BuildUploadFilesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | File Name | File Size | Asset Type | State | Uploaded |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		uploaded := ""
		if item.Attributes.Uploaded != nil {
			uploaded = fmt.Sprintf("%t", *item.Attributes.Uploaded)
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %d | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.FileName),
			item.Attributes.FileSize,
			escapeMarkdown(string(item.Attributes.AssetType)),
			escapeMarkdown(buildUploadFileState(item.Attributes)),
			escapeMarkdown(uploaded),
		)
	}
	return nil
}

func printBuildUploadResultTable(result *BuildUploadResult) error {
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
	RenderTable(headers, [][]string{values})
	if len(result.Operations) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stdout, "\nUpload Operations")
	opsHeaders := []string{"Method", "URL", "Length", "Offset"}
	opsRows := make([][]string, 0, len(result.Operations))
	for _, op := range result.Operations {
		opsRows = append(opsRows, []string{
			op.Method,
			op.URL,
			fmt.Sprintf("%d", op.Length),
			fmt.Sprintf("%d", op.Offset),
		})
	}
	RenderTable(opsHeaders, opsRows)
	return nil
}

func printBuildUploadResultMarkdown(result *BuildUploadResult) error {
	headers := []string{"Upload ID", "File ID", "File Name", "File Size"}
	values := []string{
		escapeMarkdown(result.UploadID),
		escapeMarkdown(result.FileID),
		escapeMarkdown(result.FileName),
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
	separator := make([]string, len(headers))
	for i := range separator {
		separator[i] = "---"
	}
	fmt.Fprintf(os.Stdout, "| %s |\n", strings.Join(headers, " | "))
	fmt.Fprintf(os.Stdout, "| %s |\n", strings.Join(separator, " | "))
	fmt.Fprintf(os.Stdout, "| %s |\n", strings.Join(values, " | "))
	if len(result.Operations) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stdout, "\n| Method | URL | Length | Offset |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, op := range result.Operations {
		fmt.Fprintf(os.Stdout, "| %s | %s | %d | %d |\n",
			escapeMarkdown(op.Method),
			escapeMarkdown(op.URL),
			op.Length,
			op.Offset,
		)
	}
	return nil
}

func printBuildExpireAllResultTable(result *BuildExpireAllResult) error {
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
	RenderTable(headers, rows)
	if len(result.Failures) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stdout, "\nFailures")
	failHeaders := []string{"ID", "Error"}
	failRows := make([][]string, 0, len(result.Failures))
	for _, failure := range result.Failures {
		failRows = append(failRows, []string{
			failure.ID,
			compactWhitespace(failure.Error),
		})
	}
	RenderTable(failHeaders, failRows)
	return nil
}

func printBuildExpireAllResultMarkdown(result *BuildExpireAllResult) error {
	status := "expired"
	if result.DryRun {
		status = "would-expire"
	}
	fmt.Fprintln(os.Stdout, "| ID | Version | Uploaded | Age Days | Status |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range result.Builds {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %d | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Version),
			escapeMarkdown(item.UploadedDate),
			item.AgeDays,
			status,
		)
	}
	if len(result.Failures) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stdout, "\nFailures")
	fmt.Fprintln(os.Stdout, "| ID | Error |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, failure := range result.Failures {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(failure.ID),
			escapeMarkdown(compactWhitespace(failure.Error)),
		)
	}
	return nil
}

func printBuildBetaGroupsUpdateTable(result *BuildBetaGroupsUpdateResult) error {
	headers := []string{"Build ID", "Group IDs", "Action"}
	rows := [][]string{{result.BuildID, strings.Join(result.GroupIDs, ", "), result.Action}}
	RenderTable(headers, rows)
	return nil
}

func printBuildBetaGroupsUpdateMarkdown(result *BuildBetaGroupsUpdateResult) error {
	fmt.Fprintln(os.Stdout, "| Build ID | Group IDs | Action |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
		escapeMarkdown(result.BuildID),
		escapeMarkdown(strings.Join(result.GroupIDs, ", ")),
		escapeMarkdown(result.Action),
	)
	return nil
}

func printBuildIndividualTestersUpdateTable(result *BuildIndividualTestersUpdateResult) error {
	headers := []string{"Build ID", "Tester IDs", "Action"}
	rows := [][]string{{result.BuildID, strings.Join(result.TesterIDs, ", "), result.Action}}
	RenderTable(headers, rows)
	return nil
}

func printBuildIndividualTestersUpdateMarkdown(result *BuildIndividualTestersUpdateResult) error {
	fmt.Fprintln(os.Stdout, "| Build ID | Tester IDs | Action |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
		escapeMarkdown(result.BuildID),
		escapeMarkdown(strings.Join(result.TesterIDs, ", ")),
		escapeMarkdown(result.Action),
	)
	return nil
}

func printBuildUploadDeleteResultTable(result *BuildUploadDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printBuildUploadDeleteResultMarkdown(result *BuildUploadDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}
