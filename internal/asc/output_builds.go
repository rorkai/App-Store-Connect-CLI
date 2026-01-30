package asc

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
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

func printBuildsTable(resp *BuildsResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Version\tUploaded\tProcessing\tExpired")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%t\n",
			item.Attributes.Version,
			item.Attributes.UploadedDate,
			item.Attributes.ProcessingState,
			item.Attributes.Expired,
		)
	}
	return w.Flush()
}

func printBuildsMarkdown(resp *BuildsResponse) error {
	fmt.Fprintln(os.Stdout, "| Version | Uploaded | Processing | Expired |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %t |\n",
			escapeMarkdown(item.Attributes.Version),
			escapeMarkdown(item.Attributes.UploadedDate),
			escapeMarkdown(item.Attributes.ProcessingState),
			item.Attributes.Expired,
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tVersion\tBuild\tPlatform\tState\tUploaded")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			item.ID,
			item.Attributes.CFBundleShortVersionString,
			item.Attributes.CFBundleVersion,
			string(item.Attributes.Platform),
			buildUploadState(item.Attributes),
			buildUploadTimestamp(item.Attributes),
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tFile Name\tFile Size\tAsset Type\tState\tUploaded")
	for _, item := range resp.Data {
		uploaded := ""
		if item.Attributes.Uploaded != nil {
			uploaded = fmt.Sprintf("%t", *item.Attributes.Uploaded)
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
			item.ID,
			item.Attributes.FileName,
			item.Attributes.FileSize,
			string(item.Attributes.AssetType),
			buildUploadFileState(item.Attributes),
			uploaded,
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
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
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Join(values, "\t"))
	if err := w.Flush(); err != nil {
		return err
	}
	if len(result.Operations) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stdout, "\nUpload Operations")
	opsWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(opsWriter, "Method\tURL\tLength\tOffset")
	for _, op := range result.Operations {
		fmt.Fprintf(opsWriter, "%s\t%s\t%d\t%d\n",
			op.Method,
			op.URL,
			op.Length,
			op.Offset,
		)
	}
	return opsWriter.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	status := "expired"
	if result.DryRun {
		status = "would-expire"
	}
	fmt.Fprintln(w, "ID\tVersion\tUploaded\tAge Days\tStatus")
	for _, item := range result.Builds {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			item.ID,
			item.Version,
			item.UploadedDate,
			item.AgeDays,
			status,
		)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if len(result.Failures) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stdout, "\nFailures")
	failuresWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(failuresWriter, "ID\tError")
	for _, failure := range result.Failures {
		fmt.Fprintf(failuresWriter, "%s\t%s\n",
			failure.ID,
			compactWhitespace(failure.Error),
		)
	}
	return failuresWriter.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Build ID\tGroup IDs\tAction")
	fmt.Fprintf(w, "%s\t%s\t%s\n",
		result.BuildID,
		strings.Join(result.GroupIDs, ", "),
		result.Action,
	)
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Build ID\tTester IDs\tAction")
	fmt.Fprintf(w, "%s\t%s\t%s\n",
		result.BuildID,
		strings.Join(result.TesterIDs, ", "),
		result.Action,
	)
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDeleted")
	fmt.Fprintf(w, "%s\t%t\n", result.ID, result.Deleted)
	return w.Flush()
}

func printBuildUploadDeleteResultMarkdown(result *BuildUploadDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}
