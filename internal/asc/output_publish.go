package asc

import (
	"fmt"
	"os"
	"strings"
)

func printTestFlightPublishResultTable(result *TestFlightPublishResult) error {
	headers := []string{"Build ID", "Version", "Build Number", "Processing", "Groups", "Uploaded", "Notified"}
	rows := [][]string{{
		result.BuildID,
		result.BuildVersion,
		result.BuildNumber,
		result.ProcessingState,
		strings.Join(result.GroupIDs, ", "),
		fmt.Sprintf("%t", result.Uploaded),
		fmt.Sprintf("%t", result.Notified),
	}}
	RenderTable(headers, rows)
	return nil
}

func printTestFlightPublishResultMarkdown(result *TestFlightPublishResult) error {
	fmt.Fprintln(os.Stdout, "| Build ID | Version | Build Number | Processing | Groups | Uploaded | Notified |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %t | %t |\n",
		escapeMarkdown(result.BuildID),
		escapeMarkdown(result.BuildVersion),
		escapeMarkdown(result.BuildNumber),
		escapeMarkdown(result.ProcessingState),
		escapeMarkdown(strings.Join(result.GroupIDs, ", ")),
		result.Uploaded,
		result.Notified,
	)
	return nil
}

func printAppStorePublishResultTable(result *AppStorePublishResult) error {
	headers := []string{"Build ID", "Version ID", "Submission ID", "Uploaded", "Attached", "Submitted"}
	rows := [][]string{{
		result.BuildID,
		result.VersionID,
		result.SubmissionID,
		fmt.Sprintf("%t", result.Uploaded),
		fmt.Sprintf("%t", result.Attached),
		fmt.Sprintf("%t", result.Submitted),
	}}
	RenderTable(headers, rows)
	return nil
}

func printAppStorePublishResultMarkdown(result *AppStorePublishResult) error {
	fmt.Fprintln(os.Stdout, "| Build ID | Version ID | Submission ID | Uploaded | Attached | Submitted |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %t | %t | %t |\n",
		escapeMarkdown(result.BuildID),
		escapeMarkdown(result.VersionID),
		escapeMarkdown(result.SubmissionID),
		result.Uploaded,
		result.Attached,
		result.Submitted,
	)
	return nil
}
