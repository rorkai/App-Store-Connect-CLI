package asc

import (
	"fmt"
)

func printNotarySubmissionStatusTable(resp *NotarySubmissionStatusResponse) error {
	headers := []string{"ID", "STATUS", "NAME", "CREATED"}
	rows := [][]string{{
		resp.Data.ID,
		string(resp.Data.Attributes.Status),
		compactWhitespace(resp.Data.Attributes.Name),
		resp.Data.Attributes.CreatedDate,
	}}
	RenderTable(headers, rows)
	return nil
}

func printNotarySubmissionsListTable(resp *NotarySubmissionsListResponse) error {
	headers := []string{"ID", "STATUS", "NAME", "CREATED"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			string(item.Attributes.Status),
			compactWhitespace(item.Attributes.Name),
			item.Attributes.CreatedDate,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printNotarySubmissionLogsTable(resp *NotarySubmissionLogsResponse) error {
	headers := []string{"ID", "DEVELOPER LOG URL"}
	rows := [][]string{{resp.Data.ID, resp.Data.Attributes.DeveloperLogURL}}
	RenderTable(headers, rows)
	return nil
}

func printNotarySubmissionStatusMarkdown(resp *NotarySubmissionStatusResponse) error {
	fmt.Println("| ID | Status | Name | Created |")
	fmt.Println("|----|--------|------|---------|")
	fmt.Printf("| %s | %s | %s | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(string(resp.Data.Attributes.Status)),
		escapeMarkdown(resp.Data.Attributes.Name),
		escapeMarkdown(resp.Data.Attributes.CreatedDate),
	)
	return nil
}

func printNotarySubmissionsListMarkdown(resp *NotarySubmissionsListResponse) error {
	fmt.Println("| ID | Status | Name | Created |")
	fmt.Println("|----|--------|------|---------|")
	for _, item := range resp.Data {
		fmt.Printf("| %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(string(item.Attributes.Status)),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.CreatedDate),
		)
	}
	return nil
}

func printNotarySubmissionLogsMarkdown(resp *NotarySubmissionLogsResponse) error {
	fmt.Println("| ID | Developer Log URL |")
	fmt.Println("|----|-------------------|")
	fmt.Printf("| %s | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(resp.Data.Attributes.DeveloperLogURL),
	)
	return nil
}
